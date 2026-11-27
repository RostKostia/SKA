package app

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/RostKostia/SKA/backend/internal/money"
)

type OfferInvoice struct {
	InvoiceID        string `json:"invoice_id"`
	Number           string `json:"number"`
	Client           string `json:"client"`
	OutstandingCents int64  `json:"outstanding_cents"`
}

type Offer struct {
	ID            string         `json:"id"`
	Product       string         `json:"product"` // invoice_financing or overdraft
	AmountCents   int64          `json:"amount_cents"`
	FeeCents      int64          `json:"fee_cents"`
	RepaymentDate *Date          `json:"repayment_date"`
	Status        string         `json:"status"`
	Provider      string         `json:"provider"`
	ForecastRunID string         `json:"forecast_run_id"`
	OfferedAt     time.Time      `json:"offered_at"`
	Invoices      []OfferInvoice `json:"invoices"`
	Note          string         `json:"note"`
}

const offerNote = "Simulated offer with a sample fee. A real one comes from the bank, with its own fee and repayment terms, before you accept."

// CreateOffer offers to cover the current shortfall: against the unpaid invoices (newest
// first until they cover it) or, if they can't, as an overdraft. Older open offers expire.
func (a *App) CreateOffer(ctx context.Context, u User) (Offer, error) {
	var id string
	err := a.tx(ctx, func(tx *sql.Tx) error {
		s, err := a.shortfall(ctx, tx, 30)
		if err != nil {
			return err
		}
		if s == nil {
			return conflict("no_shortfall", "no shortfall in the next 30 days, so there's nothing to cover")
		}
		if _, err := tx.ExecContext(ctx, `UPDATE financing_offers SET status = 'expired', decided_at = now() WHERE status = 'offered'`); err != nil {
			return err
		}
		var runID string
		if err := tx.QueryRowContext(ctx, `INSERT INTO forecast_runs (as_of_date, horizon_days, start_balance_cents, points,
				first_negative_date, lowest_date, lowest_balance_cents, shortfall_cents)
			SELECT kerdos_today(), 30, (SELECT available_cents FROM current_balance),
				(SELECT jsonb_agg(jsonb_build_object('date', on_date, 'balance_cents', balance_cents) ORDER BY day) FROM forecast_points(30)),
				kerdos_today() + $1::int, kerdos_today() + $2::int, $3, $4
			RETURNING id::text`, s.FirstNegativeDay, s.LowestDay, s.LowestBalanceCents, s.ShortfallCents).Scan(&runID); err != nil {
			return err
		}
		var open int64
		if err := tx.QueryRowContext(ctx, `SELECT coalesce(sum(outstanding_cents), 0) FROM invoice_overview
			WHERE direction = 'receivable' AND status = 'sent' AND NOT is_financed AND outstanding_cents > 0`).Scan(&open); err != nil {
			return err
		}
		product := "invoice_financing"
		if open < s.ShortfallCents {
			product = "overdraft"
		}
		fee := (s.ShortfallCents*a.FeeBP + 5000) / 10000
		if err := tx.QueryRowContext(ctx, `INSERT INTO financing_offers (forecast_run_id, product, amount_cents, fee_cents, repayment_date, provider)
			VALUES ($1, $2, $3, $4, CASE WHEN $2 = 'overdraft' THEN kerdos_today() + 30 END, 'mock') RETURNING id::text`,
			runID, product, s.ShortfallCents, fee).Scan(&id); err != nil {
			return err
		}
		if product == "invoice_financing" {
			if _, err := tx.ExecContext(ctx, `INSERT INTO financing_offer_invoices (financing_offer_id, invoice_id, outstanding_cents)
				SELECT $1, p.id, p.outstanding_cents FROM (
					SELECT o.id, o.outstanding_cents,
					       sum(o.outstanding_cents) OVER (ORDER BY o.issue_date DESC, o.id) - o.outstanding_cents AS covered_before
					FROM invoice_overview o
					WHERE o.direction = 'receivable' AND o.status = 'sent' AND NOT o.is_financed AND o.outstanding_cents > 0
				) p WHERE p.covered_before < $2`, id, s.ShortfallCents); err != nil {
				return err
			}
		}
		a.audit(ctx, tx, u.ID, "financing.offered", "financing_offer", id, map[string]any{"amount_cents": s.ShortfallCents, "product": product})
		return nil
	})
	if err != nil {
		return Offer{}, err
	}
	return a.GetOffer(ctx, id)
}

func (a *App) GetOffer(ctx context.Context, id string) (Offer, error) {
	if !ValidID(id) {
		return Offer{}, notFound("offer")
	}
	o := Offer{Invoices: []OfferInvoice{}, Note: offerNote}
	var repay sql.NullTime
	err := a.DB.QueryRowContext(ctx, `SELECT id::text, product, amount_cents, fee_cents, repayment_date, status, provider,
		coalesce(forecast_run_id::text, ''), offered_at FROM financing_offers WHERE id = $1`, id).
		Scan(&o.ID, &o.Product, &o.AmountCents, &o.FeeCents, &repay, &o.Status, &o.Provider, &o.ForecastRunID, &o.OfferedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return o, notFound("offer")
	}
	if err != nil {
		return o, err
	}
	o.RepaymentDate = datePtr(repay)
	rows, err := a.DB.QueryContext(ctx, `SELECT foi.invoice_id::text, i.number, c.name, foi.outstanding_cents
		FROM financing_offer_invoices foi JOIN invoices i ON i.id = foi.invoice_id JOIN counterparties c ON c.id = i.counterparty_id
		WHERE foi.financing_offer_id = $1 ORDER BY i.issue_date DESC`, id)
	if err != nil {
		return o, err
	}
	defer rows.Close()
	for rows.Next() {
		var oi OfferInvoice
		if err := rows.Scan(&oi.InvoiceID, &oi.Number, &oi.Client, &oi.OutstandingCents); err != nil {
			return o, err
		}
		o.Invoices = append(o.Invoices, oi)
	}
	return o, rows.Err()
}

// AcceptOffer accepts once (a double click gets 409), then simulates the bank paying out:
// the money lands in the primary account and the forecast updates.
func (a *App) AcceptOffer(ctx context.Context, u User, id string) (Offer, error) {
	if !ValidID(id) {
		return Offer{}, notFound("offer")
	}
	err := a.tx(ctx, func(tx *sql.Tx) error {
		var ok bool
		if err := tx.QueryRowContext(ctx, `SELECT accept_financing_offer($1, $2)`, id, nullable(u.ID)).Scan(&ok); err != nil {
			if sqlState(err) == "23514" {
				return conflict("stale_offer", "an invoice in this offer is no longer open; ask for a new offer")
			}
			return err
		}
		if !ok {
			var status string
			err := tx.QueryRowContext(ctx, `SELECT status FROM financing_offers WHERE id = $1`, id).Scan(&status)
			if errors.Is(err, sql.ErrNoRows) {
				return notFound("offer")
			}
			if err != nil {
				return err
			}
			return conflict("already_decided", "this offer is already "+status)
		}
		var amount int64
		if err := tx.QueryRowContext(ctx, `UPDATE financing_offers SET status = 'disbursed', disbursed_at = now() WHERE id = $1 RETURNING amount_cents`, id).Scan(&amount); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO balance_snapshots (bank_account_id, available_cents, current_cents, source)
			SELECT bank_account_id, available_cents + $1, current_cents + $1, 'simulated' FROM current_balance`, amount); err != nil {
			return err
		}
		if _, err := a.addChat(ctx, tx, "", "assistant", "Done. "+money.Format(amount)+" is in your account (simulated).", "",
			nil, map[string]any{"type": "financed", "offer_id": id, "amount_cents": amount}); err != nil {
			return err
		}
		a.audit(ctx, tx, u.ID, "financing.accepted", "financing_offer", id, map[string]any{"amount_cents": amount})
		return nil
	})
	if err != nil {
		return Offer{}, err
	}
	return a.GetOffer(ctx, id)
}
