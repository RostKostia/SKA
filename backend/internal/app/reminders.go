package app

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
)

type QueuedReminder struct {
	MessageID string `json:"message_id"`
	InvoiceID string `json:"invoice_id"`
	Number    string `json:"invoice_number"`
	Client    string `json:"client"`
	Kind      string `json:"kind"` // upcoming or overdue
	Tone      string `json:"tone,omitempty"`
	Sequence  int64  `json:"sequence,omitempty"`
}

// QueueDueReminders turns every row of the reminders_due view into a queued email. The view
// already applies the team rules: timed from the due date, every N days after the last one
// actually sent, polite → firm → final, never for paid or financed invoices.
func (a *App) QueueDueReminders(ctx context.Context) ([]QueuedReminder, error) {
	rows, err := a.DB.QueryContext(ctx, `SELECT d.kind, d.invoice_id::text, d.number, d.policy_id::text, d.sequence, coalesce(d.tone, ''),
			d.dedupe_key, o.counterparty_name
		FROM reminders_due d JOIN invoice_overview o ON o.id = d.invoice_id ORDER BY d.due_on, d.number`)
	if err != nil {
		return nil, err
	}
	type due struct {
		r      QueuedReminder
		policy string
		dedupe string
	}
	var list []due
	for rows.Next() {
		var d due
		var seq sql.NullInt64
		if err := rows.Scan(&d.r.Kind, &d.r.InvoiceID, &d.r.Number, &d.policy, &seq, &d.r.Tone, &d.dedupe, &d.r.Client); err != nil {
			rows.Close()
			return nil, err
		}
		d.r.Sequence = seq.Int64
		list = append(list, d)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := []QueuedReminder{}
	for _, d := range list {
		id, err := a.queueMessage(ctx, a.DB, msgSpec{Kind: d.r.Kind, Tone: d.r.Tone, Sequence: d.r.Sequence, PolicyID: d.policy,
			Mode: "automated", InvoiceID: d.r.InvoiceID, Dedupe: d.dedupe})
		var appErr *Error
		if errors.As(err, &appErr) {
			a.Log.Warn("reminder skipped", "invoice", d.r.Number, "reason", appErr.Message)
			continue
		}
		if err != nil {
			return out, err
		}
		if id != "" {
			d.r.MessageID = id
			out = append(out, d.r)
		}
	}
	if len(out) > 0 {
		a.Kick()
	}
	return out, nil
}

// RemindNow sends a manual reminder. After the due date it is the next overdue reminder (and
// counts for the schedule); before it, a friendly nudge. At most one per invoice per day.
func (a *App) RemindNow(ctx context.Context, u User, invoiceID string) (Message, error) {
	inv, err := a.getInvoice(ctx, a.DB, invoiceID)
	if err != nil {
		return Message{}, err
	}
	switch {
	case inv.Direction != "receivable" || inv.Status != "sent" || inv.OutstandingCents <= 0:
		return Message{}, conflict("not_open", "invoice "+inv.Number+" isn't open, so there's nothing to chase")
	case inv.IsFinanced:
		return Message{}, conflict("financed", "invoice "+inv.Number+" is financed, so Kerdos doesn't chase it")
	}
	var id, retryID string
	err = a.tx(ctx, func(tx *sql.Tx) error {
		var today, live int64
		if err := tx.QueryRowContext(ctx, `SELECT
				(SELECT count(*) FROM outbound_messages WHERE invoice_id = $1 AND kind IN ('upcoming', 'overdue')
				   AND status IN ('queued', 'sent') AND business_date = kerdos_today()),
				(SELECT count(*) FROM outbound_messages WHERE invoice_id = $1 AND kind = 'overdue' AND status IN ('queued', 'sent'))`,
			invoiceID).Scan(&today, &live); err != nil {
			return err
		}
		if today > 0 {
			return conflict("already_reminded_today", inv.CounterpartyName+" was already reminded today")
		}
		spec := msgSpec{Mode: "manual", InvoiceID: invoiceID, CreatedBy: u.ID}
		if inv.DaysOverdue > 0 {
			spec.Kind, spec.Sequence = "overdue", live+1
			if err := tx.QueryRowContext(ctx, `SELECT rp.tone_steps[least($2::int, cardinality(rp.tone_steps))], rp.id::text
				FROM reminder_policies rp
				WHERE rp.id = coalesce((SELECT reminder_policy_id FROM counterparties WHERE id = $1),
				                       (SELECT id FROM reminder_policies WHERE is_default))`, inv.CounterpartyID, spec.Sequence).
				Scan(&spec.Tone, &spec.PolicyID); err != nil {
				return err
			}
			spec.Dedupe = "invoice:" + invoiceID + ":overdue:" + strconv.FormatInt(spec.Sequence, 10)
		} else {
			var day string
			if err := tx.QueryRowContext(ctx, `SELECT kerdos_today()::text`).Scan(&day); err != nil {
				return err
			}
			spec.Kind, spec.Dedupe = "upcoming", "invoice:"+invoiceID+":manual:"+day
		}
		// The same reminder may already exist because it failed earlier (it keeps its dedupe key):
		// then "Remind now" sends that one again rather than being refused as a duplicate.
		var existing, status string
		err := tx.QueryRowContext(ctx, `SELECT id::text, status FROM outbound_messages WHERE dedupe_key = $1`, spec.Dedupe).Scan(&existing, &status)
		switch {
		case err == nil && status == "failed":
			retryID = existing
			return nil
		case err == nil:
			return conflict("already_reminded", inv.CounterpartyName+" already has this reminder ("+status+")")
		case !errors.Is(err, sql.ErrNoRows):
			return err
		}
		if id, err = a.queueMessage(ctx, tx, spec); err != nil {
			return err
		}
		if id == "" {
			return conflict("already_reminded", inv.CounterpartyName+" already has this reminder")
		}
		a.audit(ctx, tx, u.ID, "reminder.manual", "invoice", invoiceID, map[string]any{"kind": spec.Kind, "tone": spec.Tone})
		return nil
	})
	if err != nil {
		return Message{}, err
	}
	if retryID != "" {
		return a.RetryMessage(ctx, u, retryID)
	}
	return a.SendNow(ctx, id)
}

// emailProblem turns a send error into a short reason the owner can act on.
func emailProblem(errText string) string {
	e := strings.ToLower(errText)
	switch {
	case strings.Contains(e, "535") || strings.Contains(e, "username and password not accepted") || strings.Contains(e, "login as"):
		return "the mail server refused the login (check SMTP_USER and the app password in SMTP_PASSWORD)"
	case strings.Contains(e, "connect to") || strings.Contains(e, "no such host") || strings.Contains(e, "connection refused"):
		return "the mail server couldn't be reached"
	case strings.Contains(e, "timeout") || strings.Contains(e, "deadline exceeded"):
		return "the mail server didn't answer in time"
	case strings.Contains(e, "recipient"):
		return "the mail server refused the address"
	}
	line, _, _ := strings.Cut(strings.TrimPrefix(errText, "mail: "), "\n")
	if len(line) > 120 {
		line = line[:120] + "…"
	}
	return line
}

// outcome describes what happened to an email, for the chat: "" when it was sent.
func outcome(m Message) string {
	switch m.Status {
	case "sent":
		return ""
	case "failed":
		return "the email failed: " + emailProblem(m.Error)
	case "cancelled":
		return "it wasn't sent: " + strings.TrimPrefix(m.Error, "not sent: ")
	default:
		return "the email is still on its way"
	}
}

// reminderLine is the chat line for a reminder, worded by what really happened to its email.
func reminderLine(client string, m Message) string {
	what := client + " about " + m.InvoiceNumber + " (" + toneLabel(m.Kind, m.Tone) + ")"
	switch m.Status {
	case "sent":
		return "I reminded " + what + "."
	case "failed", "cancelled":
		return "I tried to remind " + what + ", but " + outcome(m) + "."
	default:
		return "I'm reminding " + what + "; the email is still on its way."
	}
}

func toneLabel(kind, tone string) string {
	switch {
	case kind == "upcoming":
		return "friendly nudge"
	case tone == "final":
		return "final notice"
	default:
		return tone + " reminder"
	}
}
