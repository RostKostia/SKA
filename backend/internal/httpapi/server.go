// Package httpapi is the JSON API in front of package app. Routes use Go 1.22 patterns.
// Every /api route except the public pay page and the bank callback needs a signed-in user;
// in KERDOS_AUTH=dev that is the first admin.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/RostKostia/SKA/backend/internal/app"
)

type Server struct {
	App       *app.App
	Log       *slog.Logger
	AuthMode  string // "dev"
	PublicURL string // the frontend, for the redirect after BoC consent
	MockupDir string // if set, the clickable mockup is served at /mockup/ (same origin, so no CORS needed)
}

type handler func(w http.ResponseWriter, r *http.Request, u app.User) error

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		h := s.App.Health(r.Context())
		status := http.StatusOK
		if !h.OK {
			status = http.StatusServiceUnavailable
		}
		writeJSON(w, status, h)
	})

	route := func(pattern string, h handler) { mux.Handle(pattern, s.authed(h)) }
	route("GET /api/me", s.me)
	route("GET /api/dashboard", s.dashboard)
	route("GET /api/chat", s.listChat)
	route("POST /api/chat", s.postChat)
	route("GET /api/invoices", s.listInvoices)
	route("POST /api/invoices", s.createInvoice)
	route("GET /api/invoices/{id}", s.getInvoice)
	route("POST /api/invoices/{id}/remind", s.remindInvoice)
	route("POST /api/invoices/{id}/cancel", s.cancelInvoice)
	route("GET /api/counterparties", s.listCounterparties)
	route("POST /api/counterparties", s.createCounterparty)
	route("GET /api/debtors", s.debtors)
	route("GET /api/forecast", s.forecast)
	route("POST /api/financing/offers", s.createOffer)
	route("GET /api/financing/offers/{id}", s.getOffer)
	route("POST /api/financing/offers/{id}/accept", s.acceptOffer)
	route("GET /api/messages", s.listMessages)
	route("POST /api/messages/{id}/retry", s.retryMessage)
	route("GET /api/reminders/settings", s.reminderSettings)
	route("POST /api/demo/skip", s.skip)
	route("POST /api/bank/connect", s.bankConnect)
	route("POST /api/bank/sync", s.bankSync)

	// No user needed: reset must work on an empty database (it is refused unless KERDOS_DEMO=1).
	mux.HandleFunc("POST /api/demo/reset", s.public(s.reset))

	// Public: the client's pay page and the browser coming back from 1Bank.
	mux.HandleFunc("GET /api/pay/{token}", s.public(s.getPay))
	mux.HandleFunc("POST /api/pay/{token}", s.public(s.postPay))
	mux.HandleFunc("GET /api/bank/callback", s.bankCallback)

	if s.MockupDir != "" {
		files := http.StripPrefix("/mockup/", http.FileServer(http.Dir(s.MockupDir)))
		mux.Handle("GET /mockup/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-cache")
			files.ServeHTTP(w, r)
		}))
		toMockup := func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/mockup/", http.StatusFound) }
		mux.HandleFunc("GET /mockup", toMockup)
		mux.HandleFunc("GET /{$}", toMockup)
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusNotFound, errBody("not_found", "no such endpoint: "+r.Method+" "+r.URL.Path))
	})
	return s.logRequests(s.recoverPanics(mux))
}

// ---------------------------------------------------------------- middleware and helpers

type ctxKey struct{}

func (s *Server) authed(h handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.AuthMode != "dev" {
			writeJSON(w, http.StatusNotImplemented, errBody("auth_not_implemented", "only KERDOS_AUTH=dev exists in this prototype; Google sign-in is next"))
			return
		}
		u, err := s.App.DevUser(r.Context())
		if err != nil {
			s.fail(w, r, err)
			return
		}
		if err := h(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, u)), u); err != nil {
			s.fail(w, r, err)
		}
	})
}

func (s *Server) public(h func(w http.ResponseWriter, r *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := h(w, r); err != nil {
			s.fail(w, r, err)
		}
	}
}

func (s *Server) fail(w http.ResponseWriter, r *http.Request, err error) {
	var appErr *app.Error
	if errors.As(err, &appErr) {
		writeJSON(w, appErr.Status, map[string]any{"error": appErr})
		return
	}
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) {
		writeJSON(w, http.StatusRequestEntityTooLarge, errBody("too_large", "request body too large"))
		return
	}
	var syntax *json.SyntaxError
	var typ *json.UnmarshalTypeError
	if errors.As(err, &syntax) || errors.As(err, &typ) {
		writeJSON(w, http.StatusBadRequest, errBody("bad_json", err.Error()))
		return
	}
	s.Log.Error("request failed", "method", r.Method, "path", r.URL.Path, "err", err)
	writeJSON(w, http.StatusInternalServerError, errBody("internal", "something went wrong; see the server log"))
}

func errBody(code, msg string) map[string]any {
	return map[string]any{"error": map[string]string{"code": code, "message": msg}}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

func readJSON(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(v); err != nil {
		if err.Error() == "EOF" {
			return &app.Error{Status: http.StatusBadRequest, Code: "bad_json", Message: "a JSON body is required"}
		}
		return err
	}
	return nil
}

func (s *Server) recoverPanics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if p := recover(); p != nil {
				s.Log.Error("panic", "path", r.URL.Path, "panic", p)
				writeJSON(w, http.StatusInternalServerError, errBody("internal", "something went wrong"))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) { r.status = code; r.ResponseWriter.WriteHeader(code) }

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		s.Log.Info("http", "method", r.Method, "path", r.URL.Path, "status", rec.status, "ms", time.Since(start).Milliseconds())
	})
}

func intQuery(r *http.Request, key string, def int) int {
	if n, err := strconv.Atoi(r.URL.Query().Get(key)); err == nil {
		return n
	}
	return def
}

// ---------------------------------------------------------------- handlers

func (s *Server) me(w http.ResponseWriter, _ *http.Request, u app.User) error {
	writeJSON(w, http.StatusOK, u)
	return nil
}

func (s *Server) dashboard(w http.ResponseWriter, r *http.Request, _ app.User) error {
	d, err := s.App.Dashboard(r.Context())
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, d)
	return nil
}

func (s *Server) listChat(w http.ResponseWriter, r *http.Request, _ app.User) error {
	msgs, err := s.App.ListChat(r.Context(), intQuery(r, "limit", 100))
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"messages": msgs})
	return nil
}

func (s *Server) postChat(w http.ResponseWriter, r *http.Request, u app.User) error {
	var in struct {
		Text string `json:"text"`
	}
	if err := readJSON(w, r, &in); err != nil {
		return err
	}
	msgs, err := s.App.Chat(r.Context(), u, in.Text)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"messages": msgs})
	return nil
}

func (s *Server) listInvoices(w http.ResponseWriter, r *http.Request, _ app.User) error {
	list, err := s.App.ListInvoices(r.Context(), r.URL.Query().Get("direction"), r.URL.Query().Get("status"))
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"invoices": list})
	return nil
}

func (s *Server) createInvoice(w http.ResponseWriter, r *http.Request, u app.User) error {
	var in app.NewInvoice
	if err := readJSON(w, r, &in); err != nil {
		return err
	}
	inv, err := s.App.CreateInvoice(r.Context(), u, in)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, inv)
	return nil
}

func (s *Server) getInvoice(w http.ResponseWriter, r *http.Request, _ app.User) error {
	d, err := s.App.GetInvoice(r.Context(), r.PathValue("id"))
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, d)
	return nil
}

func (s *Server) remindInvoice(w http.ResponseWriter, r *http.Request, u app.User) error {
	m, err := s.App.RemindNow(r.Context(), u, r.PathValue("id"))
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, m)
	return nil
}

func (s *Server) cancelInvoice(w http.ResponseWriter, r *http.Request, u app.User) error {
	inv, err := s.App.CancelInvoice(r.Context(), u, r.PathValue("id"))
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, inv)
	return nil
}

func (s *Server) listCounterparties(w http.ResponseWriter, r *http.Request, _ app.User) error {
	list, err := s.App.ListCounterparties(r.Context())
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"counterparties": list})
	return nil
}

func (s *Server) createCounterparty(w http.ResponseWriter, r *http.Request, u app.User) error {
	var in app.NewCounterparty
	if err := readJSON(w, r, &in); err != nil {
		return err
	}
	c, err := s.App.CreateCounterparty(r.Context(), s.App.DB, u, in)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, c)
	return nil
}

func (s *Server) debtors(w http.ResponseWriter, r *http.Request, _ app.User) error {
	list, err := s.App.Debtors(r.Context())
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"debtors": list})
	return nil
}

func (s *Server) forecast(w http.ResponseWriter, r *http.Request, _ app.User) error {
	f, err := s.App.Forecast(r.Context(), intQuery(r, "horizon", 30))
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, f)
	return nil
}

func (s *Server) createOffer(w http.ResponseWriter, r *http.Request, u app.User) error {
	o, err := s.App.CreateOffer(r.Context(), u)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, o)
	return nil
}

func (s *Server) getOffer(w http.ResponseWriter, r *http.Request, _ app.User) error {
	o, err := s.App.GetOffer(r.Context(), r.PathValue("id"))
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, o)
	return nil
}

func (s *Server) acceptOffer(w http.ResponseWriter, r *http.Request, u app.User) error {
	o, err := s.App.AcceptOffer(r.Context(), u, r.PathValue("id"))
	if err != nil {
		return err
	}
	f, err := s.App.Forecast(r.Context(), 30)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"offer": o, "forecast": f})
	return nil
}

func (s *Server) listMessages(w http.ResponseWriter, r *http.Request, _ app.User) error {
	list, err := s.App.ListMessages(r.Context(), r.URL.Query().Get("invoice_id"), r.URL.Query().Get("kind"), intQuery(r, "limit", 100))
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"messages": list})
	return nil
}

func (s *Server) retryMessage(w http.ResponseWriter, r *http.Request, u app.User) error {
	m, err := s.App.RetryMessage(r.Context(), u, r.PathValue("id"))
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, m)
	return nil
}

func (s *Server) reminderSettings(w http.ResponseWriter, r *http.Request, _ app.User) error {
	p, err := s.App.ReminderSettings(r.Context())
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, p)
	return nil
}

func (s *Server) skip(w http.ResponseWriter, r *http.Request, u app.User) error {
	var in struct {
		Days int `json:"days"`
	}
	if r.ContentLength != 0 {
		if err := readJSON(w, r, &in); err != nil {
			return err
		}
	}
	if in.Days == 0 {
		in.Days = 3
	}
	res, err := s.App.SkipDays(r.Context(), u, in.Days)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, res)
	return nil
}

func (s *Server) reset(w http.ResponseWriter, r *http.Request) error {
	if err := s.App.ResetDemo(r.Context()); err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	return nil
}

func (s *Server) bankConnect(w http.ResponseWriter, r *http.Request, u app.User) error {
	url, err := s.App.BankConnect(r.Context(), u)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"authorize_url": url})
	return nil
}

func (s *Server) bankSync(w http.ResponseWriter, r *http.Request, _ app.User) error {
	res, err := s.App.BankSync(r.Context())
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, res)
	return nil
}

func (s *Server) bankCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	target := s.PublicURL + "/settings?bank=connected"
	if code == "" {
		target = s.PublicURL + "/settings?bank=error"
	} else if err := s.App.BankCallback(r.Context(), code); err != nil {
		s.Log.Error("bank callback", "err", err)
		target = s.PublicURL + "/settings?bank=error"
	}
	http.Redirect(w, r, target, http.StatusFound)
}

func (s *Server) getPay(w http.ResponseWriter, r *http.Request) error {
	p, err := s.App.PublicInvoice(r.Context(), r.PathValue("token"))
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, p)
	return nil
}

func (s *Server) postPay(w http.ResponseWriter, r *http.Request) error {
	p, err := s.App.Pay(r.Context(), r.PathValue("token"))
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, p)
	return nil
}
