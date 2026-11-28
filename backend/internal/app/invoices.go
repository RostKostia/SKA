package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/RostKostia/SKA/backend/internal/jinius"
	"github.com/RostKostia/SKA/backend/internal/money"
	"github.com/RostKostia/SKA/backend/internal/parse"
)

type Invoice struct {
	ID               string     `json:"id"`
	Number           string     `json:"number"`
	Direction        string     `json:"direction"` // receivable or payable
	CounterpartyID   string     `json:"counterparty_id"`
	CounterpartyName string     `json:"counterparty_name"`
	Description      string     `json:"description"`
	AmountCents      int64      `json:"amount_cents"`
	VATCents         int64      `json:"vat_cents"`
	PaidCents        int64      `json:"paid_cents"`
	OutstandingCents int64      `json:"outstanding_cents"`
	Currency         string     `json:"currency"`
	IssueDate        Date       `json:"issue_date"`
	DueDate          Date       `json:"due_date"`
	Status           string     `json:"status"`         // draft, sent, paid, cancelled
	DisplayStatus    string     `json:"display_status"` // + unpaid, overdue, partially_paid, financed
	DaysOverdue      int64      `json:"days_overdue"`
	IsFinanced       bool       `json:"is_financed"`
	Source           string     `json:"source"`
	SourceText       *string    `json:"source_text"`
	PaymentLinkURL   *string    `json:"payment_link_url"`
	PayToken         string     `json:"pay_token,omitempty"`
	SentAt           *time.Time `json:"sent_at"`
	PaidAt           *time.Time `json:"paid_at"`
}

const invoiceSelect = `SELECT id::text, number, direction, counterparty_id::text, counterparty_name, coalesce(description, ''),
	amount_cents, vat_cents, paid_cents, outstanding_cents, currency::text, issue_date, due_date, status, display_status,
	days_overdue, is_financed, source, source_text, payment_link_url, coalesce(pay_token, ''), sent_at, paid_at
	FROM invoice_overview `

type scanner interface{ Scan(dest ...any) error }

func scanInvoice(s scanner) (Invoice, error) {
	var i Invoice
	var srcText, link sql.NullString
	var sent, paid sql.NullTime
	err := s.Scan(&i.ID, &i.Number, &i.Direction, &i.CounterpartyID, &i.CounterpartyName, &i.Description,
		&i.AmountCents, &i.VATCents, &i.PaidCents, &i.OutstandingCents, &i.Currency, &i.IssueDate.Time, &i.DueDate.Time,
		&i.Status, &i.DisplayStatus, &i.DaysOverdue, &i.IsFinanced, &i.Source, &srcText, &link, &i.PayToken, &sent, &paid)
	i.SourceText, i.PaymentLinkURL, i.SentAt, i.PaidAt = strPtr(srcText), strPtr(link), timePtr(sent), timePtr(paid)
	return i, err
}

func (a *App) ListInvoices(ctx context.Context, direction, displayStatus string) ([]Invoice, error) {
	rows, err := a.DB.QueryContext(ctx, invoiceSelect+`WHERE ($1 = '' OR direction = $1) AND ($2 = '' OR display_status = $2)
		ORDER BY issue_date DESC, number DESC`, direction, displayStatus)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Invoice{}
	for rows.Next() {
		i, err := scanInvoice(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

func (a *App) getInvoice(ctx context.Context, q querier, id string) (Invoice, error) {
	if !ValidID(id) {
		return Invoice{}, notFound("invoice")
	}
	inv, err := scanInvoice(q.QueryRowContext(ctx, invoiceSelect+`WHERE id = $1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return inv, notFound("invoice")
	}
	return inv, err
}

type InvoiceLine struct {
	Position       int64  `json:"position"`
	Description    string `json:"description"`
	Quantity       string `json:"quantity"`
	Unit           string `json:"unit"`
	UnitPriceCents int64  `json:"unit_price_cents"`
	VATRatePercent string `json:"vat_rate_percent"`
	NetCents       int64  `json:"net_cents"`
	VATCents       int64  `json:"vat_cents"`
}

type Payment struct {
	ID          string `json:"id"`
	AmountCents int64  `json:"amount_cents"`
	ReceivedOn  Date   `json:"received_on"`
	Method      string `json:"method"`
	Provider    string `json:"provider"`
	ExternalID  string `json:"external_id"`
}

type InvoiceDetail struct {
	Invoice
	Lines    []InvoiceLine `json:"lines"`
	Payments []Payment     `json:"payments"`
	Messages []Message     `json:"messages"`
}

func (a *App) GetInvoice(ctx context.Context, id string) (InvoiceDetail, error) {
	inv, err := a.getInvoice(ctx, a.DB, id)
	if err != nil {
		return InvoiceDetail{}, err
	}
	d := InvoiceDetail{Invoice: inv, Lines: []InvoiceLine{}, Payments: []Payment{}}
	rows, err := a.DB.QueryContext(ctx, `SELECT position, description, quantity::text, unit, unit_price_cents, vat_rate_percent::text,
		net_cents, vat_cents FROM invoice_lines WHERE invoice_id = $1 ORDER BY position`, id)
	if err != nil {
		return d, err
	}
	for rows.Next() {
		var l InvoiceLine
		if err := rows.Scan(&l.Position, &l.Description, &l.Quantity, &l.Unit, &l.UnitPriceCents, &l.VATRatePercent, &l.NetCents, &l.VATCents); err != nil {
			rows.Close()
			return d, err
		}
		d.Lines = append(d.Lines, l)
	}
	rows.Close()
	rows, err = a.DB.QueryContext(ctx, `SELECT id::text, amount_cents, received_on, method, coalesce(provider, ''), coalesce(external_id, '')
		FROM payments WHERE invoice_id = $1 ORDER BY received_on, created_at`, id)
	if err != nil {
		return d, err
	}
	for rows.Next() {
		var p Payment
		if err := rows.Scan(&p.ID, &p.AmountCents, &p.ReceivedOn.Time, &p.Method, &p.Provider, &p.ExternalID); err != nil {
			rows.Close()
			return d, err
		}
		d.Payments = append(d.Payments, p)
	}
	rows.Close()
	d.Messages, err = a.ListMessages(ctx, id, "", 100)
	return d, err
}

// NewInvoice is a manual invoice from the Invoices page.
type NewInvoice struct {
	CounterpartyID string       `json:"counterparty_id"`
	Lines          []parse.Line `json:"lines"`
}

// SentInvoice is an invoice that was just sent, with what happened to its email copy.
type SentInvoice struct {
	Invoice
	Email *Message `json:"email"` // nil only if the email copy had been queued before
}

func (a *App) CreateInvoice(ctx context.Context, u User, in NewInvoice) (SentInvoice, error) {
	if !ValidID(in.CounterpartyID) {
		return SentInvoice{}, badRequest("bad_counterparty", "counterparty_id is required")
	}
	if len(in.Lines) == 0 {
		return SentInvoice{}, badRequest("no_lines", "an invoice needs at least one line")
	}
	for _, l := range in.Lines {
		if l.QuantityMilli <= 0 || l.UnitPriceCents <= 0 || strings.TrimSpace(l.Description) == "" {
			return SentInvoice{}, badRequest("bad_line", "each line needs a description, a quantity and a price above zero")
		}
	}
	return a.createAndSend(ctx, u, in.CounterpartyID, in.Lines, "manual", "")
}

// createAndSend writes the invoice and its lines, then delivers it at once (automatic
// invoicing) and sends the email copy. If delivery fails the invoice stays a draft and the
// error says why. The email's real outcome is in SentInvoice.Email.
func (a *App) createAndSend(ctx context.Context, u User, counterpartyID string, lines []parse.Line, source, sourceText string) (SentInvoice, error) {
	var id string
	err := a.tx(ctx, func(tx *sql.Tx) error {
		var vatRate string
		var terms int64
		if err := tx.QueryRowContext(ctx, `SELECT (CASE WHEN vat_registered THEN vat_rate_percent ELSE 0 END)::text, payment_terms_days FROM company`).
			Scan(&vatRate, &terms); err != nil {
			return err
		}
		desc := lines[0].Description
		if len(lines) > 1 {
			desc = fmt.Sprintf("%s and %d more", desc, len(lines)-1)
		}
		if err := tx.QueryRowContext(ctx, `INSERT INTO invoices (number, direction, counterparty_id, description, amount_cents,
				issue_date, due_date, status, source, source_text)
			VALUES (next_invoice_number(extract(year FROM kerdos_today())::smallint), 'receivable', $1, $2, 1,
				kerdos_today(), kerdos_today() + $3::int, 'draft', $4, $5)
			RETURNING id::text`, counterpartyID, desc, terms, source, nullable(sourceText)).Scan(&id); err != nil {
			if sqlState(err) == "23503" {
				return notFound("counterparty")
			}
			return err
		}
		for i, l := range lines {
			if _, err := tx.ExecContext(ctx, `INSERT INTO invoice_lines (invoice_id, position, description, quantity, unit, unit_price_cents, vat_rate_percent)
				VALUES ($1, $2, $3, $4::numeric, $5, $6, $7::numeric)`, id, i+1, l.Description, l.Quantity(), l.Unit, l.UnitPriceCents, vatRate); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, `UPDATE invoices i SET amount_cents = s.net + s.vat, vat_cents = s.vat
			FROM (SELECT sum(net_cents) AS net, sum(vat_cents) AS vat FROM invoice_lines WHERE invoice_id = $1) s WHERE i.id = $1`, id); err != nil {
			return err
		}
		a.audit(ctx, tx, u.ID, "invoice.created", "invoice", id, map[string]any{"source": source, "source_text": sourceText})
		return nil
	})
	if err != nil {
		return SentInvoice{}, err
	}
	return a.sendInvoice(ctx, u, id)
}

func (a *App) sendInvoice(ctx context.Context, u User, id string) (SentInvoice, error) {
	inv, err := a.getInvoice(ctx, a.DB, id)
	if err != nil {
		return SentInvoice{Invoice: inv}, err
	}
	var email string
	if err := a.DB.QueryRowContext(ctx, `SELECT coalesce(email, '') FROM counterparties WHERE id = $1`, inv.CounterpartyID).Scan(&email); err != nil {
		return SentInvoice{Invoice: inv}, err
	}
	if email == "" {
		return SentInvoice{Invoice: inv}, conflict("no_email", inv.CounterpartyName+" has no email address, so the invoice wasn't sent")
	}
	d, err := a.Deliverer.Deliver(ctx, jinius.Invoice{ID: inv.ID, Number: inv.Number, ClientName: inv.CounterpartyName, ClientEmail: email,
		AmountCents: inv.AmountCents, DueDate: inv.DueDate.Time, PayToken: inv.PayToken})
	if err != nil {
		a.Log.Error("invoice delivery failed", "invoice", inv.Number, "err", err)
		return SentInvoice{Invoice: inv}, conflict("delivery_failed", "invoice "+inv.Number+" was created but not sent: "+err.Error())
	}
	var msgID string
	err = a.tx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `UPDATE invoices SET status = 'sent', sent_at = now(), provider = $2, external_id = $3, payment_link_url = $4
			WHERE id = $1 AND status = 'draft'`, id, d.Provider, d.ExternalID, d.PaymentLinkURL); err != nil {
			return err
		}
		var err error
		if msgID, err = a.queueMessage(ctx, tx, msgSpec{Kind: "invoice", Mode: "automated", InvoiceID: id, Dedupe: "invoice:" + id + ":sent", CreatedBy: u.ID}); err != nil {
			return err
		}
		a.audit(ctx, tx, u.ID, "invoice.sent", "invoice", id, map[string]any{"provider": d.Provider, "external_id": d.ExternalID})
		return nil
	})
	if err != nil {
		return SentInvoice{Invoice: inv}, err
	}
	out := SentInvoice{}
	if msgID != "" {
		// Send the copy now, so the answer says what really happened to it.
		m, err := a.SendNow(ctx, msgID)
		if err != nil {
			return out, err
		}
		out.Email = &m
	}
	out.Invoice, err = a.getInvoice(ctx, a.DB, id)
	return out, err
}

func (a *App) CancelInvoice(ctx context.Context, u User, id string) (Invoice, error) {
	inv, err := a.getInvoice(ctx, a.DB, id)
	if err != nil {
		return inv, err
	}
	if inv.Status == "paid" || inv.Status == "cancelled" || inv.IsFinanced || inv.PaidCents > 0 {
		return inv, conflict("not_cancellable", "only open invoices with no payments and no financing can be cancelled")
	}
	err = a.tx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `UPDATE invoices SET status = 'cancelled' WHERE id = $1`, id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE outbound_messages SET status = 'cancelled' WHERE invoice_id = $1 AND status = 'queued'`, id); err != nil {
			return err
		}
		a.audit(ctx, tx, u.ID, "invoice.cancelled", "invoice", id, nil)
		return nil
	})
	if err != nil {
		return inv, err
	}
	return a.getInvoice(ctx, a.DB, id)
}

func describeLines(lines []parse.Line) string {
	if len(lines) == 1 {
		return lines[0].Description
	}
	parts := make([]string, len(lines))
	for i, l := range lines {
		parts[i] = l.Description + " (" + money.Format(l.NetCents()) + ")"
	}
	return strings.Join(parts, ", ")
}
