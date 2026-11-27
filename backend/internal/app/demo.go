package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/RostKostia/SKA/backend/internal/money"
)

type Movement struct {
	Label       string `json:"label"`
	AmountCents int64  `json:"amount_cents"`
}

type DayReport struct {
	Date      Date             `json:"date"`
	Arrivals  []Movement       `json:"arrivals"`
	BillsPaid []Movement       `json:"bills_paid"`
	Reminders []QueuedReminder `json:"reminders"`
}

type SkipResult struct {
	Days     int           `json:"days"`
	Today    Date          `json:"today"`
	Reports  []DayReport   `json:"reports"`
	Forecast Forecast      `json:"forecast"`
	Messages []ChatMessage `json:"messages"`
}

// SkipDays is the demo's "Skip time": for each day it moves the clock, lets the mock bank
// post that day's money (cash flows and bills due), and sends the reminders now due. It then
// narrates the result in the chat, ending with the cash warning if there is one.
func (a *App) SkipDays(ctx context.Context, u User, days int) (SkipResult, error) {
	if !a.Demo {
		return SkipResult{}, forbidden("demo_off", "skip time is only available with KERDOS_DEMO=1")
	}
	if days < 1 || days > 60 {
		return SkipResult{}, badRequest("bad_days", "days must be between 1 and 60")
	}
	res := SkipResult{Days: days}
	for i := 0; i < days; i++ {
		rep, err := a.skipOneDay(ctx)
		if err != nil {
			return res, err
		}
		if rep.Reminders, err = a.QueueDueReminders(ctx); err != nil {
			return res, err
		}
		res.Reports = append(res.Reports, rep)
	}
	if _, err := a.SendQueued(ctx); err != nil {
		a.Log.Warn("send after skip", "err", err)
	}
	f, err := a.Forecast(ctx, 30)
	if err != nil {
		return res, err
	}
	res.Today, res.Forecast = f.Today, f

	say := func(role, text string, card any) error {
		m, err := a.addChat(ctx, a.DB, u.ID, role, text, "", nil, card)
		if err == nil {
			res.Messages = append(res.Messages, m)
		}
		return err
	}
	if err := say("system", fmt.Sprintf("%s later · %s", plural(int64(days), "day"), longDate(f.Today.Time)), nil); err != nil {
		return res, err
	}
	for _, rep := range res.Reports {
		for _, m := range rep.Arrivals {
			_ = say("assistant", fmt.Sprintf("%s of %s arrived.", m.Label, money.Format(m.AmountCents)), nil)
		}
		for _, m := range rep.BillsPaid {
			_ = say("assistant", fmt.Sprintf("Paid %s: %s.", m.Label, money.Format(m.AmountCents)), nil)
		}
		for _, r := range rep.Reminders {
			_ = say("assistant", fmt.Sprintf("I reminded %s about %s (%s).", r.Client, r.Number, toneLabel(r.Kind, r.Tone)),
				map[string]any{"type": "reminder", "message_id": r.MessageID, "invoice_id": r.InvoiceID, "tone": r.Tone, "sequence": r.Sequence})
		}
	}
	if f.Shortfall != nil {
		if card, text, err := a.warningCard(ctx, f); err == nil {
			_ = say("assistant", text, card)
		}
	}
	return res, nil
}

func (a *App) skipOneDay(ctx context.Context) (DayReport, error) {
	rep := DayReport{Arrivals: []Movement{}, BillsPaid: []Movement{}, Reminders: []QueuedReminder{}}
	err := a.tx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `UPDATE company SET clock_offset_days = clock_offset_days + 1`); err != nil {
			return err
		}
		if err := tx.QueryRowContext(ctx, `SELECT kerdos_today()`).Scan(&rep.Date.Time); err != nil {
			return err
		}
		day := rep.Date.Format("2006-01-02")
		var acct string
		var avail, current int64
		err := tx.QueryRowContext(ctx, `SELECT bank_account_id::text, available_cents, current_cents FROM current_balance`).Scan(&acct, &avail, &current)
		if errors.Is(err, sql.ErrNoRows) {
			return nil // no bank account: only the clock moves
		}
		if err != nil {
			return err
		}
		delta := int64(0)

		type flow struct {
			id, label string
			amount    int64
		}
		var flows []flow
		rows, err := tx.QueryContext(ctx, `SELECT cashflow_id::text, label, amount_cents FROM forecast_events(0) WHERE kind = 'cashflow'`)
		if err != nil {
			return err
		}
		for rows.Next() {
			var f flow
			if err := rows.Scan(&f.id, &f.label, &f.amount); err != nil {
				rows.Close()
				return err
			}
			flows = append(flows, f)
		}
		rows.Close()
		for _, f := range flows {
			dc := "credit"
			if f.amount < 0 {
				dc = "debit"
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO bank_transactions (bank_account_id, boc_transaction_id, booked_date, dc_ind, amount_cents,
					description, category, cashflow_id)
				VALUES ($1, $2, $3::date, $4, $5, $6, (SELECT category FROM cashflows WHERE id = $7), $7)`,
				acct, "sim-"+f.id+"-"+day, day, dc, f.amount, f.label, f.id); err != nil {
				return err
			}
			delta += f.amount
			if f.amount > 0 {
				rep.Arrivals = append(rep.Arrivals, Movement{f.label, f.amount})
			} else {
				rep.BillsPaid = append(rep.BillsPaid, Movement{f.label, -f.amount})
			}
		}

		type bill struct {
			id, label string
			amount    int64
		}
		var bills []bill
		rows, err = tx.QueryContext(ctx, `SELECT id::text, coalesce(description, number), outstanding_cents FROM invoice_overview
			WHERE direction = 'payable' AND status = 'sent' AND outstanding_cents > 0 AND due_date <= kerdos_today() ORDER BY due_date`)
		if err != nil {
			return err
		}
		for rows.Next() {
			var b bill
			if err := rows.Scan(&b.id, &b.label, &b.amount); err != nil {
				rows.Close()
				return err
			}
			bills = append(bills, b)
		}
		rows.Close()
		for _, b := range bills {
			var txID string
			if err := tx.QueryRowContext(ctx, `INSERT INTO bank_transactions (bank_account_id, boc_transaction_id, booked_date, dc_ind, amount_cents, description)
				VALUES ($1, $2, $3::date, 'debit', $4, $5) RETURNING id::text`, acct, "sim-bill-"+b.id, day, -b.amount, b.label).Scan(&txID); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO payments (invoice_id, amount_cents, method, bank_transaction_id, provider)
				VALUES ($1, $2, 'bank_transfer', $3, 'mock')`, b.id, b.amount, txID); err != nil {
				return err
			}
			delta -= b.amount
			rep.BillsPaid = append(rep.BillsPaid, Movement{b.label, b.amount})
		}
		if len(flows) > 0 || len(bills) > 0 {
			_, err = tx.ExecContext(ctx, `INSERT INTO balance_snapshots (bank_account_id, available_cents, current_cents, source)
				VALUES ($1, $2, $3, 'mock')`, acct, avail+delta, current+delta)
		}
		return err
	})
	return rep, err
}

const resetSQL = `TRUNCATE audit_log, chat_messages, outbound_messages, payment_initiations, payments, bank_transactions,
	cashflows, balance_snapshots, bank_accounts, bank_connections, financing_offer_invoices, invoice_lines, invoices,
	financing_offers, forecast_runs, budgets, invoice_counters, counterparties, message_templates, reminder_policies,
	sessions, google_tokens, users, company RESTART IDENTITY CASCADE;
`

// ResetDemo empties every table and loads the sample data again (db/seed/sample.sql), in one
// transaction. For rehearsals: the demo always starts from the same story.
func (a *App) ResetDemo(ctx context.Context) error {
	if !a.Demo {
		return forbidden("demo_off", "reset is only available with KERDOS_DEMO=1")
	}
	seed, err := os.ReadFile(a.SeedFile)
	if err != nil {
		return fmt.Errorf("read seed file %s (set KERDOS_SEED_FILE): %w", a.SeedFile, err)
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	// No arguments, so the driver sends it as one simple query: Postgres runs it all in one transaction.
	if _, err := a.DB.ExecContext(ctx, resetSQL+string(seed)); err != nil {
		return err
	}
	a.Kick()
	return nil
}
