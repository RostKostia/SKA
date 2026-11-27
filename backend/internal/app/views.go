package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
)

// ---------------------------------------------------------------- counterparties

type Counterparty struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Email    string   `json:"email"`
	Phone    string   `json:"phone"`
	Aliases  []string `json:"aliases"`
	Language string   `json:"language,omitempty"`
}

const counterpartySelect = `SELECT id::text, name, coalesce(email, ''), coalesce(phone, ''), array_to_json(aliases)::text, coalesce(language, '') FROM counterparties `

func scanCounterparty(s scanner) (Counterparty, error) {
	var c Counterparty
	var aliases string
	err := s.Scan(&c.ID, &c.Name, &c.Email, &c.Phone, &aliases, &c.Language)
	if err == nil {
		_ = json.Unmarshal([]byte(aliases), &c.Aliases)
	}
	if c.Aliases == nil {
		c.Aliases = []string{}
	}
	return c, err
}

func (a *App) ListCounterparties(ctx context.Context) ([]Counterparty, error) {
	rows, err := a.DB.QueryContext(ctx, counterpartySelect+`ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Counterparty{}
	for rows.Next() {
		c, err := scanCounterparty(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

type NewCounterparty struct {
	Name    string   `json:"name"`
	Email   string   `json:"email"`
	Phone   string   `json:"phone"`
	Aliases []string `json:"aliases"`
}

func (a *App) CreateCounterparty(ctx context.Context, q querier, u User, in NewCounterparty) (Counterparty, error) {
	in.Name, in.Email = strings.TrimSpace(in.Name), strings.TrimSpace(in.Email)
	if in.Name == "" {
		return Counterparty{}, badRequest("no_name", "a name is required")
	}
	if in.Aliases == nil {
		in.Aliases = []string{}
	}
	c, err := scanCounterparty(q.QueryRowContext(ctx, `INSERT INTO counterparties (name, email, phone, aliases)
		VALUES ($1, $2, $3, ARRAY(SELECT jsonb_array_elements_text($4::jsonb)))
		RETURNING id::text, name, coalesce(email, ''), coalesce(phone, ''), array_to_json(aliases)::text, coalesce(language, '')`,
		in.Name, nullable(in.Email), nullable(in.Phone), jsonArg(in.Aliases)))
	if sqlState(err) == "23505" {
		return c, conflict("exists", in.Name+" already exists")
	}
	if err == nil {
		a.audit(ctx, q, u.ID, "counterparty.created", "counterparty", c.ID, nil)
	}
	return c, err
}

// matchCounterparty finds who the owner means by name: an exact name or alias wins, then a
// unique name prefix ("Kostas" → "Kostas Hadjis"). Several prefix matches are ambiguous.
func (a *App) matchCounterparty(ctx context.Context, name string) (*Counterparty, []Counterparty, error) {
	like := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(strings.ToLower(strings.TrimSpace(name))) + "%"
	rows, err := a.DB.QueryContext(ctx, `SELECT id::text, name, coalesce(email, ''), coalesce(phone, ''), array_to_json(aliases)::text, coalesce(language, ''),
			CASE WHEN lower(name) = lower($1) THEN 3
			     WHEN EXISTS (SELECT 1 FROM unnest(aliases) x WHERE lower(x) = lower($1)) THEN 2 ELSE 1 END AS score
		FROM counterparties
		WHERE lower(name) = lower($1) OR EXISTS (SELECT 1 FROM unnest(aliases) x WHERE lower(x) = lower($1)) OR lower(name) LIKE $2
		ORDER BY score DESC, name LIMIT 5`, strings.TrimSpace(name), like)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var found []Counterparty
	var scores []int64
	for rows.Next() {
		var c Counterparty
		var aliases string
		var score int64
		if err := rows.Scan(&c.ID, &c.Name, &c.Email, &c.Phone, &aliases, &c.Language, &score); err != nil {
			return nil, nil, err
		}
		_ = json.Unmarshal([]byte(aliases), &c.Aliases)
		found, scores = append(found, c), append(scores, score)
	}
	switch {
	case len(found) == 0:
		return nil, nil, rows.Err()
	case scores[0] >= 2 || len(found) == 1:
		return &found[0], nil, rows.Err()
	default:
		return nil, found, rows.Err()
	}
}

// ---------------------------------------------------------------- debtors and dashboard

type Debtor struct {
	CounterpartyID   string  `json:"counterparty_id"`
	Name             string  `json:"name"`
	OpenInvoices     int64   `json:"open_invoices"`
	OutstandingCents int64   `json:"outstanding_cents"`
	OldestAgeDays    int64   `json:"oldest_age_days"`
	MaxDaysOverdue   int64   `json:"max_days_overdue"`
	RemindersSent    int64   `json:"reminders_sent"`
	LastTone         *string `json:"last_tone"`
	OldestInvoiceID  string  `json:"oldest_invoice_id"`
}

func (a *App) Debtors(ctx context.Context) ([]Debtor, error) {
	rows, err := a.DB.QueryContext(ctx, `SELECT d.counterparty_id::text, d.counterparty_name, d.open_invoices, d.outstanding_cents,
			d.oldest_age_days, d.max_days_overdue, d.reminders_sent, d.last_tone,
			(SELECT o.id::text FROM invoice_overview o WHERE o.counterparty_id = d.counterparty_id AND o.direction = 'receivable'
			   AND o.status = 'sent' AND NOT o.is_financed AND o.outstanding_cents > 0 ORDER BY o.issue_date LIMIT 1)
		FROM debtors d ORDER BY d.outstanding_cents DESC, d.counterparty_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Debtor{}
	for rows.Next() {
		var d Debtor
		var tone sql.NullString
		if err := rows.Scan(&d.CounterpartyID, &d.Name, &d.OpenInvoices, &d.OutstandingCents, &d.OldestAgeDays, &d.MaxDaysOverdue,
			&d.RemindersSent, &tone, &d.OldestInvoiceID); err != nil {
			return nil, err
		}
		d.LastTone = strPtr(tone)
		out = append(out, d)
	}
	return out, rows.Err()
}

type Action struct {
	Type        string `json:"type"` // financing or remind
	Label       string `json:"label"`
	AmountCents int64  `json:"amount_cents,omitempty"`
	InvoiceID   string `json:"invoice_id,omitempty"`
}

type Dashboard struct {
	Company         string   `json:"company"`
	Today           Date     `json:"today"`
	ClockOffsetDays int64    `json:"clock_offset_days"`
	Demo            bool     `json:"demo"`
	BalanceCents    *int64   `json:"balance_cents"`
	OwedCents       int64    `json:"owed_cents"`
	Forecast        Forecast `json:"forecast"`
	Debtors         []Debtor `json:"debtors"`
	Actions         []Action `json:"actions"`
}

// Dashboard is the right-hand panel: cash outlook, who owes you, and the action plan.
func (a *App) Dashboard(ctx context.Context) (Dashboard, error) {
	d := Dashboard{Demo: a.Demo, Actions: []Action{}}
	var bal sql.NullInt64
	if err := a.DB.QueryRowContext(ctx, `SELECT name, kerdos_today(), clock_offset_days, (SELECT available_cents FROM current_balance) FROM company`).
		Scan(&d.Company, &d.Today.Time, &d.ClockOffsetDays, &bal); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return d, conflict("empty", "no company yet: load db/seed/sample.sql")
		}
		return d, err
	}
	if bal.Valid {
		d.BalanceCents = &bal.Int64
	}
	var err error
	if d.Debtors, err = a.Debtors(ctx); err != nil {
		return d, err
	}
	for _, x := range d.Debtors {
		d.OwedCents += x.OutstandingCents
	}
	if d.Forecast, err = a.Forecast(ctx, 30); err != nil {
		var appErr *Error
		if !errors.As(err, &appErr) {
			return d, err
		}
		d.Forecast.Headline = appErr.Message
	}
	if s := d.Forecast.Shortfall; s != nil {
		d.Actions = append(d.Actions, Action{Type: "financing", Label: "Get " + moneyFormat(s.ShortfallCents) + " now", AmountCents: s.ShortfallCents})
	}
	for _, x := range d.Debtors {
		d.Actions = append(d.Actions, Action{Type: "remind", Label: "Remind " + x.Name + " now", InvoiceID: x.OldestInvoiceID})
	}
	return d, nil
}

// ---------------------------------------------------------------- reminder settings

type ReminderPolicy struct {
	ID                string     `json:"id"`
	Name              string     `json:"name"`
	BeforeDueDays     []int      `json:"before_due_days"`
	FirstAfterDueDays int64      `json:"first_after_due_days"`
	EveryDays         int64      `json:"every_days"`
	ToneSteps         []string   `json:"tone_steps"`
	MaxReminders      *int64     `json:"max_reminders"`
	Enabled           bool       `json:"enabled"`
	Templates         []Template `json:"templates"`
}

type Template struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Tone     string `json:"tone,omitempty"`
	Language string `json:"language"`
	Subject  string `json:"subject"`
	Body     string `json:"body"`
}

func (a *App) ReminderSettings(ctx context.Context) (ReminderPolicy, error) {
	var p ReminderPolicy
	var before, tones string
	var max sql.NullInt64
	err := a.DB.QueryRowContext(ctx, `SELECT id::text, name, array_to_json(before_due_days)::text, first_after_due_days, every_days,
		array_to_json(tone_steps)::text, max_reminders, enabled FROM reminder_policies WHERE is_default`).
		Scan(&p.ID, &p.Name, &before, &p.FirstAfterDueDays, &p.EveryDays, &tones, &max, &p.Enabled)
	if errors.Is(err, sql.ErrNoRows) {
		return p, notFound("default reminder policy")
	}
	if err != nil {
		return p, err
	}
	_ = json.Unmarshal([]byte(before), &p.BeforeDueDays)
	_ = json.Unmarshal([]byte(tones), &p.ToneSteps)
	if max.Valid {
		p.MaxReminders = &max.Int64
	}
	rows, err := a.DB.QueryContext(ctx, `SELECT id::text, kind, coalesce(tone, ''), language, subject, body FROM message_templates
		ORDER BY kind, array_position(ARRAY['polite', 'firm', 'final'], tone), language`)
	if err != nil {
		return p, err
	}
	defer rows.Close()
	p.Templates = []Template{}
	for rows.Next() {
		var t Template
		if err := rows.Scan(&t.ID, &t.Kind, &t.Tone, &t.Language, &t.Subject, &t.Body); err != nil {
			return p, err
		}
		p.Templates = append(p.Templates, t)
	}
	return p, rows.Err()
}
