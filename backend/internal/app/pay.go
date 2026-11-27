package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/RostKostia/SKA/backend/internal/money"
)

// PublicInvoice is what the client sees on the pay page (the demo's second phone). It never
// shows internal state such as financing.
type PublicInvoice struct {
	CompanyName      string          `json:"company_name"`
	Number           string          `json:"number"`
	ClientName       string          `json:"client_name"`
	Description      string          `json:"description"`
	AmountCents      int64           `json:"amount_cents"`
	OutstandingCents int64           `json:"outstanding_cents"`
	Currency         string          `json:"currency"`
	IssueDate        Date            `json:"issue_date"`
	DueDate          Date            `json:"due_date"`
	Status           string          `json:"status"` // unpaid, overdue or paid
	CanPay           bool            `json:"can_pay"`
	PaidAt           *time.Time      `json:"paid_at"`
	Reminder         *PublicReminder `json:"latest_reminder"`
}

type PublicReminder struct {
	Kind string `json:"kind"`
	Tone string `json:"tone,omitempty"`
	Text string `json:"text"`
}

func (a *App) PublicInvoice(ctx context.Context, token string) (PublicInvoice, error) {
	var p PublicInvoice
	var id, status string
	var daysOverdue int64
	var paid sql.NullTime
	err := a.DB.QueryRowContext(ctx, `SELECT o.id::text, co.name, o.number, o.counterparty_name, coalesce(o.description, ''), o.amount_cents,
			o.outstanding_cents, o.currency::text, o.issue_date, o.due_date, o.status, o.days_overdue, o.paid_at
		FROM invoice_overview o CROSS JOIN company co
		WHERE o.pay_token = $1 AND o.direction = 'receivable' AND o.status IN ('sent', 'paid')`, token).
		Scan(&id, &p.CompanyName, &p.Number, &p.ClientName, &p.Description, &p.AmountCents, &p.OutstandingCents, &p.Currency,
			&p.IssueDate.Time, &p.DueDate.Time, &status, &daysOverdue, &paid)
	if errors.Is(err, sql.ErrNoRows) {
		return p, notFound("invoice")
	}
	if err != nil {
		return p, err
	}
	p.PaidAt = timePtr(paid)
	switch {
	case status == "paid" || p.OutstandingCents <= 0:
		p.Status = "paid"
	case daysOverdue > 0:
		p.Status = "overdue"
	default:
		p.Status = "unpaid"
	}
	p.CanPay = p.Status != "paid"
	var r PublicReminder
	err = a.DB.QueryRowContext(ctx, `SELECT kind, coalesce(tone, ''), body FROM outbound_messages
		WHERE invoice_id = $1 AND kind IN ('upcoming', 'overdue') AND status = 'sent'
		ORDER BY business_date DESC, created_at DESC LIMIT 1`, id).Scan(&r.Kind, &r.Tone, &r.Text)
	if err == nil && p.CanPay {
		p.Reminder = &r
	}
	return p, nil
}

// Pay is the pay page's "Pay now", simulated as a BoC pay-by-bank payment. The real flow
// (signed payment initiation, the client's 1Bank approval, execute) is described in
// docs/data-model.md under PaymentInitiation.
func (a *App) Pay(ctx context.Context, token string) (PublicInvoice, error) {
	err := a.tx(ctx, func(tx *sql.Tx) error {
		var id string
		if err := tx.QueryRowContext(ctx, `SELECT id::text FROM invoices WHERE pay_token = $1 AND direction = 'receivable' FOR UPDATE`, token).Scan(&id); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return notFound("invoice")
			}
			return err
		}
		inv, err := a.getInvoice(ctx, tx, id)
		if err != nil {
			return err
		}
		if inv.Status != "sent" || inv.OutstandingCents <= 0 {
			return conflict("not_payable", "this invoice is already paid or no longer open")
		}
		amount := inv.OutstandingCents
		// The mock bank credits the business, unless the invoice was financed: then the money repays the advance.
		var txID any
		if !inv.IsFinanced {
			var acct string
			err := tx.QueryRowContext(ctx, `SELECT bank_account_id::text FROM current_balance`).Scan(&acct)
			if err == nil {
				var t string
				if err := tx.QueryRowContext(ctx, `INSERT INTO bank_transactions (bank_account_id, boc_transaction_id, booked_date, dc_ind, amount_cents,
						description, counterparty_name, category)
					VALUES ($1, $2, kerdos_today(), 'credit', $3, $4, $5, 'Income') RETURNING id::text`,
					acct, "sim-pay-"+id, amount, inv.Number+" "+inv.CounterpartyName, inv.CounterpartyName).Scan(&t); err != nil {
					return err
				}
				txID = t
				if _, err := tx.ExecContext(ctx, `INSERT INTO balance_snapshots (bank_account_id, available_cents, current_cents, source)
					SELECT bank_account_id, available_cents + $1, current_cents + $1, 'mock' FROM current_balance`, amount); err != nil {
					return err
				}
			} else if !errors.Is(err, sql.ErrNoRows) {
				return err
			}
		}
		ref := fmt.Sprintf("mock-%d", time.Now().UnixNano())
		var payID string
		if err := tx.QueryRowContext(ctx, `INSERT INTO payments (invoice_id, amount_cents, method, bank_transaction_id, provider, external_id)
			VALUES ($1, $2, 'pay_by_bank', $3, 'mock', $4) RETURNING id::text`, id, amount, txID, ref).Scan(&payID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO payment_initiations (invoice_id, provider, payment_id, creditor_iban, amount_cents,
				payment_details, status, ref_number, executed_at, resulting_payment_id)
			VALUES ($1, 'mock', $2, (SELECT coalesce(iban, 'unknown') FROM company), $3, $4, 'executed', $2, now(), $5)`,
			id, ref, amount, inv.Number+" "+inv.CounterpartyName, payID); err != nil {
			return err
		}
		if _, err := a.queueMessage(ctx, tx, msgSpec{Kind: "receipt", Mode: "automated", InvoiceID: id, Dedupe: "invoice:" + id + ":receipt"}); err != nil {
			return err
		}
		text := fmt.Sprintf("%s paid %s for %s.", inv.CounterpartyName, money.Format(amount), inv.Number)
		if inv.IsFinanced {
			text += " It was financed, so this repays the advance."
		}
		if _, err := a.addChat(ctx, tx, "", "assistant", text, "", nil, map[string]any{"type": "paid", "invoice_id": id}); err != nil {
			return err
		}
		a.audit(ctx, tx, "", "invoice.paid", "invoice", id, map[string]any{"method": "pay_by_bank", "amount_cents": amount})
		return nil
	})
	if err != nil {
		return PublicInvoice{}, err
	}
	a.Kick()
	return a.PublicInvoice(ctx, token)
}
