package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/RostKostia/SKA/backend/internal/money"
)

type ForecastPoint struct {
	Day          int64 `json:"day"`
	Date         Date  `json:"date"`
	BalanceCents int64 `json:"balance_cents"`
}

type ForecastEvent struct {
	Day         int64   `json:"day"`
	Date        Date    `json:"date"`
	AmountCents int64   `json:"amount_cents"`
	Kind        string  `json:"kind"` // receivable, payable or cashflow
	Label       string  `json:"label"`
	InvoiceID   *string `json:"invoice_id"`
	CashflowID  *string `json:"cashflow_id"`
}

type Shortfall struct {
	FirstNegativeDay   int64 `json:"first_negative_day"`
	LowestDay          int64 `json:"lowest_day"`
	LowestBalanceCents int64 `json:"lowest_balance_cents"`
	ShortfallCents     int64 `json:"shortfall_cents"`
}

type Forecast struct {
	Today             Date            `json:"today"`
	HorizonDays       int             `json:"horizon_days"`
	StartBalanceCents int64           `json:"start_balance_cents"`
	Points            []ForecastPoint `json:"points"`
	Events            []ForecastEvent `json:"events"`
	Shortfall         *Shortfall      `json:"shortfall"`
	Headline          string          `json:"headline"`
	Method            string          `json:"method"`
}

const forecastMethod = "Starts from the bank balance. Adds each unpaid invoice on the day that client usually pays (due date plus their average delay), subtracts bills on their due date, and applies known cash flows. Financed invoices are left out."

// Forecast is the cash outlook for the next horizon days, computed by the database
// (forecast_points, forecast_events, forecast_shortfall).
func (a *App) Forecast(ctx context.Context, horizon int) (Forecast, error) {
	if horizon <= 0 || horizon > 365 {
		horizon = 30
	}
	f := Forecast{HorizonDays: horizon, Points: []ForecastPoint{}, Events: []ForecastEvent{}, Method: forecastMethod}
	var start sql.NullInt64
	if err := a.DB.QueryRowContext(ctx, `SELECT kerdos_today(), (SELECT available_cents FROM current_balance)`).Scan(&f.Today.Time, &start); err != nil {
		return f, err
	}
	if !start.Valid {
		return f, conflict("no_balance", "no bank balance yet: connect the bank or load the sample data")
	}
	f.StartBalanceCents = start.Int64
	rows, err := a.DB.QueryContext(ctx, `SELECT day, on_date, balance_cents FROM forecast_points($1)`, horizon)
	if err != nil {
		return f, err
	}
	for rows.Next() {
		var p ForecastPoint
		if err := rows.Scan(&p.Day, &p.Date.Time, &p.BalanceCents); err != nil {
			rows.Close()
			return f, err
		}
		f.Points = append(f.Points, p)
	}
	rows.Close()
	rows, err = a.DB.QueryContext(ctx, `SELECT day, on_date, amount_cents, kind, label, invoice_id::text, cashflow_id::text FROM forecast_events($1)`, horizon)
	if err != nil {
		return f, err
	}
	for rows.Next() {
		var e ForecastEvent
		var inv, cf sql.NullString
		if err := rows.Scan(&e.Day, &e.Date.Time, &e.AmountCents, &e.Kind, &e.Label, &inv, &cf); err != nil {
			rows.Close()
			return f, err
		}
		e.InvoiceID, e.CashflowID = strPtr(inv), strPtr(cf)
		f.Events = append(f.Events, e)
	}
	rows.Close()
	s, err := a.shortfall(ctx, a.DB, horizon)
	if err != nil {
		return f, err
	}
	f.Shortfall = s
	f.Headline = headline(s, horizon)
	return f, nil
}

func (a *App) shortfall(ctx context.Context, q querier, horizon int) (*Shortfall, error) {
	var s Shortfall
	err := q.QueryRowContext(ctx, `SELECT first_negative_day, lowest_day, lowest_balance_cents, shortfall_cents FROM forecast_shortfall($1)`, horizon).
		Scan(&s.FirstNegativeDay, &s.LowestDay, &s.LowestBalanceCents, &s.ShortfallCents)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func headline(s *Shortfall, horizon int) string {
	if s == nil {
		return fmt.Sprintf("No shortfall in the next %d days.", horizon)
	}
	return fmt.Sprintf("In %s you'll be %s negative.", plural(s.LowestDay, "day"), money.Format(s.ShortfallCents))
}
