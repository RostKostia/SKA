package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/RostKostia/SKA/backend/internal/money"
	"github.com/RostKostia/SKA/backend/internal/parse"
)

func moneyFormat(c int64) string { return money.Format(c) }

type ChatMessage struct {
	ID        string          `json:"id"`
	Role      string          `json:"role"` // owner, assistant or system
	Text      string          `json:"text"`
	Intent    string          `json:"intent,omitempty"`
	Card      json.RawMessage `json:"card,omitempty"`
	Invoice   *Invoice        `json:"invoice,omitempty"` // current state of the card's invoice, if any
	CreatedAt time.Time       `json:"created_at"`
}

func (a *App) addChat(ctx context.Context, q querier, userID, role, text, intent string, parsed, card any) (ChatMessage, error) {
	m := ChatMessage{Role: role, Text: text, Intent: intent}
	err := q.QueryRowContext(ctx, `INSERT INTO chat_messages (user_id, role, text, intent, parsed, card)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6::jsonb) RETURNING id::text, created_at`,
		nullable(userID), role, text, nullable(intent), jsonArg(parsed), jsonArg(card)).Scan(&m.ID, &m.CreatedAt)
	if card != nil {
		m.Card, _ = json.Marshal(card)
	}
	return m, err
}

// ListChat returns the last limit messages, oldest first, with each card's invoice resolved.
func (a *App) ListChat(ctx context.Context, limit int) ([]ChatMessage, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := a.DB.QueryContext(ctx, `SELECT id::text, role, text, coalesce(intent, ''), coalesce(card::text, ''), created_at FROM (
		SELECT * FROM chat_messages ORDER BY created_at DESC, id DESC LIMIT $1) m ORDER BY created_at, id`, limit)
	if err != nil {
		return nil, err
	}
	var out []ChatMessage
	for rows.Next() {
		var m ChatMessage
		var card string
		if err := rows.Scan(&m.ID, &m.Role, &m.Text, &m.Intent, &card, &m.CreatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		if card != "" {
			m.Card = json.RawMessage(card)
		}
		out = append(out, m)
	}
	rows.Close()
	if out == nil {
		out = []ChatMessage{}
	}
	for i := range out {
		a.resolveCard(ctx, &out[i])
	}
	return out, rows.Err()
}

func (a *App) resolveCard(ctx context.Context, m *ChatMessage) {
	if len(m.Card) == 0 {
		return
	}
	var c struct {
		InvoiceID string `json:"invoice_id"`
	}
	if json.Unmarshal(m.Card, &c) == nil && c.InvoiceID != "" {
		if inv, err := a.getInvoice(ctx, a.DB, c.InvoiceID); err == nil {
			m.Invoice = &inv
		}
	}
}

// pendingCard is the "need_details" card Kerdos leaves when it asked for a missing email.
type pendingCard struct {
	Type           string       `json:"type"`
	Missing        string       `json:"missing"`
	Draft          *parse.Draft `json:"draft"`
	CounterpartyID string       `json:"counterparty_id,omitempty"`
	SourceText     string       `json:"source_text,omitempty"`
}

func (a *App) pending(ctx context.Context) *pendingCard {
	var card sql.NullString
	err := a.DB.QueryRowContext(ctx, `SELECT card::text FROM chat_messages WHERE role = 'assistant' ORDER BY created_at DESC, id DESC LIMIT 1`).Scan(&card)
	if err != nil || !card.Valid {
		return nil
	}
	var p pendingCard
	if json.Unmarshal([]byte(card.String), &p) != nil || p.Type != "need_details" || p.Draft == nil {
		return nil
	}
	return &p
}

// Chat handles one owner message: read it (Claude or rules), act, and answer.
func (a *App) Chat(ctx context.Context, u User, text string) ([]ChatMessage, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, badRequest("empty", "say something first")
	}
	if len([]rune(text)) > 1000 {
		return nil, badRequest("too_long", "keep it under 1,000 characters")
	}
	res := a.Parser.Parse(ctx, text)
	owner, err := a.addChat(ctx, a.DB, u.ID, "owner", text, string(res.Intent), res, nil)
	if err != nil {
		return nil, err
	}
	out := []ChatMessage{owner}
	reply := func(t string, card any) error {
		m, err := a.addChat(ctx, a.DB, "", "assistant", t, "", nil, card)
		if err != nil {
			return err
		}
		a.resolveCard(ctx, &m)
		out = append(out, m)
		return nil
	}
	if err := a.respond(ctx, u, text, res, reply); err != nil {
		return out, err
	}
	return out, nil
}

type replyFunc func(text string, card any) error

func (a *App) respond(ctx context.Context, u User, text string, res parse.Result, reply replyFunc) error {
	switch res.Intent {
	case parse.ProvideDetails:
		p := a.pending(ctx)
		if p == nil || p.Missing != "email" {
			return reply("Got the address, but I wasn't waiting for one. To invoice someone, try: Invoice Kostas €120", nil)
		}
		cpID := p.CounterpartyID
		if cpID != "" {
			if _, err := a.DB.ExecContext(ctx, `UPDATE counterparties SET email = $2 WHERE id = $1`, cpID, res.Email); err != nil {
				return err
			}
		} else {
			c, err := a.CreateCounterparty(ctx, a.DB, u, NewCounterparty{Name: titleCase(p.Draft.Client), Email: res.Email, Aliases: []string{p.Draft.Client}})
			if err != nil {
				return err
			}
			cpID = c.ID
		}
		return a.chatSend(ctx, u, cpID, p.Draft, p.SourceText, reply)

	case parse.CreateInvoice:
		d := res.Draft
		switch res.Missing {
		case "client":
			return reply("Who should I invoice? Try: Invoice Kostas €120", nil)
		case "amount":
			return reply(fmt.Sprintf("How much should I invoice %[1]s? For example: Invoice %[1]s €120, or Invoice %[1]s, 3 hours at €40", d.Client), nil)
		}
		cp, candidates, err := a.matchCounterparty(ctx, d.Client)
		if err != nil {
			return err
		}
		if len(candidates) > 0 {
			names := make([]string, len(candidates))
			for i, c := range candidates {
				names[i] = c.Name
			}
			return reply(fmt.Sprintf("I know more than one %s: %s. Which one? Use the full name.", d.Client, strings.Join(names, ", ")), nil)
		}
		if cp == nil || cp.Email == "" {
			who, id := d.Client, ""
			if cp != nil {
				who, id = cp.Name, cp.ID
			}
			return reply(fmt.Sprintf("I don't have an email for %s yet. What's their email? I'll send the invoice for %s as soon as I have it.", who, money.Format(d.TotalCents())),
				pendingCard{Type: "need_details", Missing: "email", Draft: d, CounterpartyID: id, SourceText: text})
		}
		return a.chatSend(ctx, u, cp.ID, d, text, reply)

	case parse.Remind:
		return a.chatRemind(ctx, u, res, reply)

	case parse.Forecast:
		f, err := a.Forecast(ctx, 30)
		if err != nil {
			return replyErr(err, reply)
		}
		if f.Shortfall == nil {
			return reply(fmt.Sprintf("You're fine: your balance is %s and it stays above zero for the next 30 days.", money.Format(f.StartBalanceCents)), map[string]any{"type": "ok"})
		}
		card, t, err := a.warningCard(ctx, f)
		if err != nil {
			return err
		}
		return reply(t, card)

	case parse.Financing:
		o, err := a.CreateOffer(ctx, u)
		if err != nil {
			return replyErr(err, reply)
		}
		what := "against your unpaid invoices"
		if o.Product == "overdraft" {
			what = "as an overdraft"
		}
		return reply(fmt.Sprintf("I can get you %s now %s. Sample fee: %s. Accept?", money.Format(o.AmountCents), what, money.Format(o.FeeCents)),
			map[string]any{"type": "offer", "offer_id": o.ID, "amount_cents": o.AmountCents, "fee_cents": o.FeeCents, "product": o.Product})

	case parse.Debtors:
		ds, err := a.Debtors(ctx)
		if err != nil {
			return err
		}
		if len(ds) == 0 {
			return reply("Nobody owes you anything right now.", nil)
		}
		parts := make([]string, len(ds))
		var total int64
		for i, d := range ds {
			total += d.OutstandingCents
			parts[i] = d.Name + " owes " + money.Format(d.OutstandingCents)
			if d.MaxDaysOverdue > 0 {
				parts[i] += fmt.Sprintf(" (%s overdue)", plural(d.MaxDaysOverdue, "day"))
			}
		}
		return reply(fmt.Sprintf("%s in all. %s.", money.Format(total), strings.Join(parts, "; ")), map[string]any{"type": "debtors"})
	}
	return reply(`I can invoice a client ("Invoice Kostas €120"), chase a payment ("Remind Andreas"), tell you how your cash looks ("Will I go negative?") or get you money now against unpaid invoices ("Get money now").`,
		map[string]any{"type": "suggestions", "chips": []string{"Invoice Kostas €120", "Invoice Kostas, 3 hours at €40", "Will I go negative?", "Who owes me?"}})
}

func (a *App) chatSend(ctx context.Context, u User, cpID string, d *parse.Draft, sourceText string, reply replyFunc) error {
	sent, err := a.createAndSend(ctx, u, cpID, d.Lines, "chat", sourceText)
	inv := sent.Invoice
	if err != nil {
		var appErr *Error
		if errors.As(err, &appErr) && inv.ID != "" {
			return reply("I created invoice "+inv.Number+" but couldn't send it: "+appErr.Message, map[string]any{"type": "invoice", "invoice_id": inv.ID})
		}
		return replyErr(err, reply)
	}
	via := "through Jinius"
	if inv.PaymentLinkURL != nil && strings.Contains(*inv.PaymentLinkURL, "/pay/") {
		via = "through Jinius (simulated)"
	}
	card := map[string]any{"type": "invoice", "invoice_id": inv.ID}
	head := fmt.Sprintf("I sent invoice %s for %s to %s %s with a payment link", inv.Number, money.Format(inv.AmountCents), inv.CounterpartyName, via)
	due := fmt.Sprintf("It's due on %s; I'll start reminding them if it's still unpaid after that.", longDate(inv.DueDate.Time))
	m := sent.Email
	switch {
	case m == nil:
		return reply("Done. "+head+". "+due, card)
	case m.Status == "sent":
		return reply(fmt.Sprintf("Done. %s, and emailed a copy to %s. %s", head, m.Recipient, due), card)
	default:
		card["email_message_id"], card["email_status"] = m.ID, m.Status
		if m.Status == "failed" {
			return reply(fmt.Sprintf("%s, but the email copy to %s didn't go out: %s. Fix that, then press Retry in the email log. %s",
				head, m.Recipient, emailProblem(m.Error), due), card)
		}
		return reply(fmt.Sprintf("%s. The email copy to %s: %s. %s", head, m.Recipient, outcome(*m), due), card)
	}
}

func (a *App) chatRemind(ctx context.Context, u User, res parse.Result, reply replyFunc) error {
	var targets []string
	if res.All {
		ds, err := a.Debtors(ctx)
		if err != nil {
			return err
		}
		for _, d := range ds {
			targets = append(targets, d.OldestInvoiceID)
		}
		if len(targets) == 0 {
			return reply("Nobody needs chasing: there are no open invoices.", nil)
		}
	} else {
		cp, candidates, err := a.matchCounterparty(ctx, res.Client)
		if err != nil {
			return err
		}
		if cp == nil {
			if len(candidates) > 0 {
				return reply("I know more than one "+res.Client+". Which one? Use the full name.", nil)
			}
			return reply("I don't know "+res.Client+".", nil)
		}
		var id string
		err = a.DB.QueryRowContext(ctx, `SELECT id::text FROM invoice_overview WHERE counterparty_id = $1 AND direction = 'receivable'
			AND status = 'sent' AND outstanding_cents > 0 ORDER BY is_financed, issue_date LIMIT 1`, cp.ID).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) {
			return reply(cp.Name+" doesn't owe you anything.", nil)
		}
		if err != nil {
			return err
		}
		targets = []string{id}
	}
	for _, id := range targets {
		m, err := a.RemindNow(ctx, u, id)
		if err != nil {
			var appErr *Error
			if errors.As(err, &appErr) {
				if err := reply(capitalize(appErr.Message)+".", nil); err != nil {
					return err
				}
				continue
			}
			return err
		}
		var client string
		_ = a.DB.QueryRowContext(ctx, `SELECT counterparty_name FROM invoice_overview WHERE id = $1`, id).Scan(&client)
		var seq int64
		if m.Sequence != nil {
			seq = *m.Sequence
		}
		if err := reply(reminderLine(client, m), map[string]any{"type": "reminder", "message_id": m.ID, "invoice_id": id, "tone": m.Tone, "sequence": seq}); err != nil {
			return err
		}
	}
	return nil
}

// warningCard is "In 9 days you'll be €800 negative. Get €800 against your unpaid invoices now?"
func (a *App) warningCard(ctx context.Context, f Forecast) (map[string]any, string, error) {
	s := f.Shortfall
	var unpaid int64
	if err := a.DB.QueryRowContext(ctx, `SELECT coalesce(sum(outstanding_cents), 0) FROM invoice_overview
		WHERE direction = 'receivable' AND status = 'sent' AND NOT is_financed AND outstanding_cents > 0`).Scan(&unpaid); err != nil {
		return nil, "", err
	}
	var firstOut string
	for _, e := range f.Events {
		if e.Day == s.FirstNegativeDay && e.AmountCents < 0 {
			label := e.Label
			if _, after, ok := strings.Cut(label, " · "); ok {
				label = after
			}
			firstOut = fmt.Sprintf("%s (%s)", label, money.Format(-e.AmountCents))
			break
		}
	}
	text := "Heads up. " + f.Headline
	if firstOut != "" {
		text += fmt.Sprintf(" The first dip comes on %s, when %s goes out.", longDate(f.Today.AddDate(0, 0, int(s.FirstNegativeDay))), firstOut)
	}
	if unpaid >= s.ShortfallCents {
		text += fmt.Sprintf(" Your unpaid invoices total %s, so I can get you %s against them now.", money.Format(unpaid), money.Format(s.ShortfallCents))
	} else {
		text += fmt.Sprintf(" I can get you %s now as an overdraft.", money.Format(s.ShortfallCents))
	}
	return map[string]any{"type": "warning", "lowest_day": s.LowestDay, "first_negative_day": s.FirstNegativeDay,
		"shortfall_cents": s.ShortfallCents, "unpaid_cents": unpaid, "first_out": firstOut}, text, nil
}

func replyErr(err error, reply replyFunc) error {
	var appErr *Error
	if errors.As(err, &appErr) {
		return reply(capitalize(appErr.Message)+".", nil)
	}
	return err
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	return strings.ToUpper(string(r[0])) + string(r[1:])
}

func titleCase(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		words[i] = capitalize(w)
	}
	return strings.Join(words, " ")
}
