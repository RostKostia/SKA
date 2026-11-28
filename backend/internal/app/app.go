// Package app holds Kerdos's business operations. The database does the heavy lifting
// (views and functions in db/migrations); this package orchestrates it, talks to the outside
// adapters (Claude, Jinius, BoC, email) and narrates what happened in the chat.
package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"github.com/RostKostia/SKA/backend/internal/ai"
	"github.com/RostKostia/SKA/backend/internal/boc"
	"github.com/RostKostia/SKA/backend/internal/google"
	"github.com/RostKostia/SKA/backend/internal/jinius"
	"github.com/RostKostia/SKA/backend/internal/mail"
)

type App struct {
	DB        *sql.DB
	Log       *slog.Logger
	Parser    ai.Parser
	Deliverer jinius.Deliverer
	Mailer    mail.Mailer
	BoC       *boc.Client    // nil when KERDOS_BANK=mock
	Google    *google.Client // nil when GOOGLE_CLIENT_ID is unset: no "Continue with Google"
	// ReturnOrigins are where a Google sign-in may send the browser back to: the frontend and
	// this server (the mockup), as scheme://host[:port].
	ReturnOrigins []string
	PublicURL     string // the frontend, for absolute pay links in emails
	Demo          bool   // enables skip time and reset
	SeedFile      string
	FeeBP         int64 // sample fee on simulated financing offers, in basis points

	kick chan struct{}
}

func (a *App) init() {
	if a.kick == nil {
		a.kick = make(chan struct{}, 1)
	}
}

// Kick wakes the worker so queued emails go out now rather than at the next tick.
func (a *App) Kick() {
	a.init()
	select {
	case a.kick <- struct{}{}:
	default:
	}
}

// Error is an expected failure with an HTTP status and a stable code for the frontend.
type Error struct {
	Status  int    `json:"-"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

func notFound(what string) error {
	return &Error{http.StatusNotFound, "not_found", what + " not found"}
}
func conflict(code, msg string) error {
	return &Error{http.StatusConflict, code, msg}
}
func badRequest(code, msg string) error { return &Error{http.StatusBadRequest, code, msg} }
func forbidden(code, msg string) error  { return &Error{http.StatusForbidden, code, msg} }

// querier is satisfied by *sql.DB and *sql.Tx.
type querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func (a *App) tx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := a.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func sqlState(err error) string {
	var s interface{ SQLState() string }
	if errors.As(err, &s) {
		return s.SQLState()
	}
	return ""
}

var uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// ValidID reports whether s looks like a uuid, so bad ids give 404 instead of a SQL error.
func ValidID(s string) bool { return uuidRe.MatchString(s) }

// Date is a calendar date that marshals as "2006-01-02".
type Date struct{ time.Time }

func (d Date) MarshalJSON() ([]byte, error) { return json.Marshal(d.Format("2006-01-02")) }
func (d *Date) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	t, err := time.Parse("2006-01-02", s)
	d.Time = t
	return err
}

func datePtr(t sql.NullTime) *Date {
	if !t.Valid {
		return nil
	}
	return &Date{t.Time}
}

func timePtr(t sql.NullTime) *time.Time {
	if !t.Valid {
		return nil
	}
	return &t.Time
}

func strPtr(s sql.NullString) *string {
	if !s.Valid {
		return nil
	}
	return &s.String
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func jsonArg(v any) any {
	if v == nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return string(b)
}

func longDate(t time.Time) string { return t.Format("2 Jan 2006") }

func plural(n int64, word string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}

type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}

// DevUser is the first admin: in KERDOS_AUTH=dev every request acts as this user.
func (a *App) DevUser(ctx context.Context) (User, error) {
	var u User
	err := a.DB.QueryRowContext(ctx, `SELECT id::text, email, name, role FROM users WHERE role = 'admin' ORDER BY created_at LIMIT 1`).
		Scan(&u.ID, &u.Email, &u.Name, &u.Role)
	if errors.Is(err, sql.ErrNoRows) {
		return u, &Error{http.StatusUnauthorized, "no_user", "no admin user yet: load db/seed/sample.sql or create one"}
	}
	return u, err
}

func (a *App) audit(ctx context.Context, q querier, userID, action, entity, entityID string, details any) {
	if _, err := q.ExecContext(ctx, `INSERT INTO audit_log (user_id, action, entity, entity_id, details) VALUES ($1, $2, $3, $4, $5::jsonb)`,
		nullable(userID), action, entity, nullable(entityID), jsonArg(details)); err != nil {
		a.Log.Warn("audit log write failed", "action", action, "err", err)
	}
}

// Health is what GET /healthz reports.
type Health struct {
	OK      bool   `json:"ok"`
	DB      string `json:"db"`
	Bank    string `json:"bank"`
	Mailer  string `json:"mailer"`
	Demo    bool   `json:"demo"`
	Version string `json:"version"`
}

func (a *App) Health(ctx context.Context) Health {
	h := Health{OK: true, DB: "ok", Bank: "mock", Mailer: a.Mailer.Provider(), Demo: a.Demo, Version: "prototype"}
	if a.BoC != nil {
		h.Bank = "boc"
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	var n int
	if err := a.DB.QueryRowContext(ctx, `SELECT count(*) FROM company`).Scan(&n); err != nil {
		h.OK, h.DB = false, err.Error()
	} else if n == 0 {
		h.DB = "empty: load db/seed/sample.sql"
	}
	return h
}
