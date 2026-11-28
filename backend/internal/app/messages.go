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
	if m.Kind == "invite" {
		// The link in an invitation is a credential for the invitee only; the log doesn't show it.
		m.Body = inviteLinkRe.ReplaceAllString(m.Body, "invite=…")
	}
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
		status, err := a.sendNext(ctx, "")
		if err != nil {
			return sent, err
		}
		if status == "" {
			break
		}
		if status == "sent" {
			sent++
		}
	}
	return sent, nil
}

// sendNext claims one queued email (only messageID, if given), checks that it should still
// go out, and sends it. It returns the email's new status, or "" when nothing was claimed.
func (a *App) sendNext(ctx context.Context, messageID string) (string, error) {
	status := ""
	err := a.tx(ctx, func(tx *sql.Tx) error {
		var id, kind, invoiceID, inviteID, to, subject, body string
		err := tx.QueryRowContext(ctx, `SELECT id::text, kind, coalesce(invoice_id::text, ''), coalesce(invite_id::text, ''), recipient, subject, body
			FROM outbound_messages WHERE status = 'queued' AND scheduled_for <= now() AND ($1 = '' OR id::text = $1)
			ORDER BY scheduled_for, created_at LIMIT 1 FOR UPDATE SKIP LOCKED`, messageID).
			Scan(&id, &kind, &invoiceID, &inviteID, &to, &subject, &body)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		// Check again right before sending: the invoice may have been cancelled, paid or financed
		// since the email was queued (or the row may have been requeued by hand).
		if reason, err := a.staleReason(ctx, tx, kind, invoiceID, inviteID); err != nil {
			return err
		} else if reason != "" {
			a.Log.Info("email dropped", "to", to, "kind", kind, "reason", reason)
			status = "cancelled"
			_, err = tx.ExecContext(ctx, `UPDATE outbound_messages SET status = 'cancelled', error = $2 WHERE id = $1`, id, "not sent: "+reason)
			return err
		}
		providerID, sendErr := a.Mailer.Send(ctx, mail.Message{To: to, Subject: subject, Body: body})
		if sendErr != nil {
			a.Log.Warn("email failed", "to", to, "err", sendErr)
			status = "failed"
			_, err = tx.ExecContext(ctx, `UPDATE outbound_messages SET status = 'failed', error = $2, provider = $3 WHERE id = $1`, id, sendErr.Error(), a.Mailer.Provider())
			return err
		}
		status = "sent"
		_, err = tx.ExecContext(ctx, `UPDATE outbound_messages SET status = 'sent', sent_at = now(), error = NULL, provider = $2, provider_message_id = $3 WHERE id = $1`,
			id, a.Mailer.Provider(), providerID)
		return err
	})
	return status, err
}

// staleReason says why an email about an invoice (or an invitation) must not go out any more,
// or "" if it may.
func (a *App) staleReason(ctx context.Context, q querier, kind, invoiceID, inviteID string) (string, error) {
	if kind == "invite" {
		var accepted, revoked, expired bool
		err := q.QueryRowContext(ctx, `SELECT accepted_at IS NOT NULL, revoked_at IS NOT NULL, expires_at <= now() FROM invites WHERE id = $1`,
			nullable(inviteID)).Scan(&accepted, &revoked, &expired)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return "the invitation no longer exists", nil
		case err != nil:
			return "", err
		case accepted:
			return "the invitation was already accepted", nil
		case revoked:
			return "the invitation was cancelled", nil
		case expired:
			return "the invitation expired", nil
		}
		return "", nil
	}
	if invoiceID == "" {
		return "", nil
	}
	var status string
	var financed bool
	var outstanding int64
	err := q.QueryRowContext(ctx, `SELECT status, is_financed, outstanding_cents FROM invoice_overview WHERE id = $1`, invoiceID).
		Scan(&status, &financed, &outstanding)
	if errors.Is(err, sql.ErrNoRows) {
		return "the invoice no longer exists", nil
	}
	if err != nil {
		return "", err
	}
	switch {
	case status == "cancelled":
		return "the invoice was cancelled", nil
	case kind != "upcoming" && kind != "overdue":
		return "", nil // invoice and receipt emails still go out once the invoice is paid
	case status == "paid" || outstanding <= 0:
		return "the invoice is paid", nil
	case financed:
		return "the invoice is financed, so it isn't chased", nil
	}
	return "", nil
}

// SendNow sends one queued email immediately and returns it with its real status: "sent",
// "failed" (with the error) or "cancelled". If the worker is sending it at that moment, this
// waits a few seconds for the outcome; it can come back still "queued" if the server is slow.
func (a *App) SendNow(ctx context.Context, messageID string) (Message, error) {
	if _, err := a.sendNext(ctx, messageID); err != nil {
		return Message{}, err
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		m, err := a.getMessage(ctx, a.DB, messageID)
		if err != nil || m.Status != "queued" || time.Now().After(deadline) {
			return m, err
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// RetryMessage sends a failed email again, at once, with the same text.
func (a *App) RetryMessage(ctx context.Context, u User, id string) (Message, error) {
	if !ValidID(id) {
		return Message{}, notFound("email")
	}
	m, err := a.getMessage(ctx, a.DB, id)
	if err != nil {
		return m, err
	}
	if m.Status != "failed" {
		return m, conflict("not_failed", "only failed emails can be retried; this one is "+m.Status)
	}
	res, err := a.DB.ExecContext(ctx, `UPDATE outbound_messages SET status = 'queued', error = NULL, scheduled_for = now() WHERE id = $1 AND status = 'failed'`, id)
	if sqlState(err) == "23505" {
		return m, conflict("superseded", "a later reminder with the same number was already sent, so this one isn't needed")
	}
	if err != nil {
		return m, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return m, conflict("not_failed", "this email is no longer failed")
	}
	a.audit(ctx, a.DB, u.ID, "email.retried", "outbound_message", id, nil)
	return a.SendNow(ctx, id)
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
