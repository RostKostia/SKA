package app

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/RostKostia/SKA/backend/internal/mail"
	"github.com/RostKostia/SKA/backend/internal/money"
)

// Message is a row of outbound_messages: every email Kerdos sends or will send.
type Message struct {
	ID            string     `json:"id"`
	Kind          string     `json:"kind"` // invoice, upcoming, overdue, receipt, owner_summary, budget
	Mode          string     `json:"mode"` // manual or automated
	InvoiceID     string     `json:"invoice_id,omitempty"`
	InvoiceNumber string     `json:"invoice_number,omitempty"`
	Sequence      *int64     `json:"sequence,omitempty"`
	Tone          string     `json:"tone,omitempty"`
	Recipient     string     `json:"recipient"`
	Subject       string     `json:"subject"`
	Body          string     `json:"body"`
	Status        string     `json:"status"`
	BusinessDate  Date       `json:"business_date"`
	SentAt        *time.Time `json:"sent_at"`
	Provider      string     `json:"provider,omitempty"`
	Error         string     `json:"error,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

const messageSelect = `SELECT m.id::text, m.kind, m.mode, coalesce(m.invoice_id::text, ''), coalesce(i.number, ''), m.sequence,
	coalesce(m.tone, ''), m.recipient, m.subject, m.body, m.status, m.business_date, m.sent_at, coalesce(m.provider, ''),
	coalesce(m.error, ''), m.created_at
	FROM outbound_messages m LEFT JOIN invoices i ON i.id = m.invoice_id `

func scanMessage(s scanner) (Message, error) {
	var m Message
	var seq sql.NullInt64
	var sent sql.NullTime
	err := s.Scan(&m.ID, &m.Kind, &m.Mode, &m.InvoiceID, &m.InvoiceNumber, &seq, &m.Tone, &m.Recipient, &m.Subject, &m.Body,
		&m.Status, &m.BusinessDate.Time, &sent, &m.Provider, &m.Error, &m.CreatedAt)
	if seq.Valid {
		m.Sequence = &seq.Int64
	}
	m.SentAt = timePtr(sent)
	return m, err
}

func (a *App) ListMessages(ctx context.Context, invoiceID, kind string, limit int) ([]Message, error) {
	if invoiceID != "" && !ValidID(invoiceID) {
		return []Message{}, nil
	}
	rows, err := a.DB.QueryContext(ctx, messageSelect+`WHERE ($1 = '' OR m.invoice_id::text = $1) AND ($2 = '' OR m.kind = $2)
		ORDER BY m.created_at DESC, m.business_date DESC LIMIT $3`, invoiceID, kind, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Message{}
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (a *App) getMessage(ctx context.Context, q querier, id string) (Message, error) {
	m, err := scanMessage(q.QueryRowContext(ctx, messageSelect+`WHERE m.id = $1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return m, notFound("message")
	}
	return m, err
}

type msgSpec struct {
	Kind      string // invoice, upcoming, overdue, receipt
	Tone      string // overdue only
	Sequence  int64  // overdue only
	PolicyID  string
	Mode      string // manual or automated
	InvoiceID string
	Dedupe    string
	CreatedBy string
}

// queueMessage renders the template for spec and queues the email. It returns "" when the
// dedupe key (or the one-live-reminder-number rule) says it was already queued or sent.
func (a *App) queueMessage(ctx context.Context, q querier, s msgSpec) (string, error) {
	var number, client, email, lang, link, company string
	var outstanding, amount, daysOverdue int64
	var due time.Time
	err := q.QueryRowContext(ctx, `SELECT o.number, o.counterparty_name, coalesce(c.email, ''), coalesce(c.language, co.language),
			o.outstanding_cents, o.amount_cents, o.due_date, o.days_overdue, coalesce(o.payment_link_url, ''), co.name
		FROM invoice_overview o JOIN counterparties c ON c.id = o.counterparty_id CROSS JOIN company co WHERE o.id = $1`, s.InvoiceID).
		Scan(&number, &client, &email, &lang, &outstanding, &amount, &due, &daysOverdue, &link, &company)
	if err != nil {
		return "", err
	}
	if email == "" {
		return "", conflict("no_email", client+" has no email address")
	}
	if strings.HasPrefix(link, "/") {
		link = a.PublicURL + link
	}
	shown := outstanding
	if s.Kind == "invoice" || s.Kind == "receipt" {
		shown = amount
	}
	vars := map[string]string{
		"{company_name}": company, "{client_name}": client, "{invoice_number}": number, "{amount}": money.Format(shown),
		"{due_date}": longDate(due), "{days_overdue}": strconv.FormatInt(daysOverdue, 10), "{payment_link}": link,
	}
	var tplID, subject, body string
	err = q.QueryRowContext(ctx, `SELECT id::text, subject, body FROM message_templates
		WHERE kind = $1 AND coalesce(tone, '') = $2 AND enabled AND language IN ($3, 'en')
		ORDER BY (language = $3) DESC LIMIT 1`, s.Kind, s.Tone, lang).Scan(&tplID, &subject, &body)
	if errors.Is(err, sql.ErrNoRows) {
		subject, body = "Invoice {invoice_number}", "Hi {client_name}, invoice {invoice_number} for {amount} is due on {due_date}. {payment_link}"
	} else if err != nil {
		return "", err
	}
	subject, body = render(subject, vars), render(body, vars)
	var seq, tone any
	if s.Kind == "overdue" {
		seq, tone = s.Sequence, s.Tone
	}
	var id string
	err = q.QueryRowContext(ctx, `INSERT INTO outbound_messages (kind, mode, invoice_id, sequence, tone, policy_id, template_id,
			recipient, subject, body, language, dedupe_key, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		ON CONFLICT DO NOTHING RETURNING id::text`,
		s.Kind, s.Mode, s.InvoiceID, seq, tone, nullable(s.PolicyID), nullable(tplID), email, subject, body, lang, nullable(s.Dedupe), nullable(s.CreatedBy)).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return id, err
}

func render(tpl string, vars map[string]string) string {
	pairs := make([]string, 0, len(vars)*2)
	for k, v := range vars {
		pairs = append(pairs, k, v)
	}
	return strings.NewReplacer(pairs...).Replace(tpl)
}

// SendQueued sends every queued email that is due, one row at a time. Rows are claimed with
// FOR UPDATE SKIP LOCKED, so several workers can run at once.
func (a *App) SendQueued(ctx context.Context) (int, error) {
	sent := 0
	for i := 0; i < 100; i++ {
		done := false
		err := a.tx(ctx, func(tx *sql.Tx) error {
			var id, to, subject, body string
			err := tx.QueryRowContext(ctx, `SELECT id::text, recipient, subject, body FROM outbound_messages
				WHERE status = 'queued' AND scheduled_for <= now() ORDER BY scheduled_for, created_at LIMIT 1 FOR UPDATE SKIP LOCKED`).
				Scan(&id, &to, &subject, &body)
			if errors.Is(err, sql.ErrNoRows) {
				done = true
				return nil
			}
			if err != nil {
				return err
			}
			providerID, sendErr := a.Mailer.Send(ctx, mail.Message{To: to, Subject: subject, Body: body})
			if sendErr != nil {
				a.Log.Warn("email failed", "to", to, "err", sendErr)
				_, err = tx.ExecContext(ctx, `UPDATE outbound_messages SET status = 'failed', error = $2, provider = $3 WHERE id = $1`, id, sendErr.Error(), a.Mailer.Provider())
				return err
			}
			sent++
			_, err = tx.ExecContext(ctx, `UPDATE outbound_messages SET status = 'sent', sent_at = now(), provider = $2, provider_message_id = $3 WHERE id = $1`,
				id, a.Mailer.Provider(), providerID)
			return err
		})
		if err != nil {
			return sent, err
		}
		if done {
			break
		}
	}
	return sent, nil
}

// Run is the background worker: queue reminders that are due and send queued emails, on
// every tick and whenever Kick is called.
func (a *App) Run(ctx context.Context, every time.Duration) {
	a.init()
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		if _, err := a.QueueDueReminders(ctx); err != nil && ctx.Err() == nil {
			a.Log.Error("queue reminders", "err", err)
		}
		if _, err := a.SendQueued(ctx); err != nil && ctx.Err() == nil {
			a.Log.Error("send emails", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		case <-a.kick:
		}
	}
}
