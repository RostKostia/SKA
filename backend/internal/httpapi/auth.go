package httpapi

// Sign-in, the account page and the team. With KERDOS_AUTH=accounts every /api route except
// the public ones needs a session: the kerdos_session cookie (the mockup, same origin) or
// "Authorization: Bearer <token>". The web app's server forwards the browser's cookie.

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/RostKostia/SKA/backend/internal/app"
)

const sessionCookie = "kerdos_session"

type tokenKey struct{}

// sessionToken reads the session from the Authorization header or the cookie.
func sessionToken(r *http.Request) string {
	if h := r.Header.Get("Authorization"); len(h) > 7 && strings.EqualFold(h[:7], "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	if c, err := r.Cookie(sessionCookie); err == nil {
		return c.Value
	}
	return ""
}

func (s *Server) currentUser(r *http.Request) (app.User, string, error) {
	if s.AuthMode == "dev" {
		u, err := s.App.DevUser(r.Context())
		return u, "", err
	}
	token := sessionToken(r)
	u, err := s.App.SessionUser(r.Context(), token)
	return u, token, err
}

func (s *Server) authed(h handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, token, err := s.currentUser(r)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		ctx := context.WithValue(context.WithValue(r.Context(), ctxKey{}, u), tokenKey{}, token)
		if err := h(w, r.WithContext(ctx), u); err != nil {
			s.fail(w, r, err)
		}
	})
}

// ownerOnly lets only the owner (role admin) through.
func ownerOnly(h handler) handler {
	return func(w http.ResponseWriter, r *http.Request, u app.User) error {
		if u.Role != "admin" {
			return &app.Error{Status: http.StatusForbidden, Code: "owner_only", Message: "Only the owner of the business can do this."}
		}
		return h(w, r, u)
	}
}

// checkOrigin refuses state-changing requests that a browser sent from another site (CSRF).
// Server-to-server calls send no Origin header and are not affected.
func (s *Server) checkOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" || r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		allowed := []string{requestOrigin(r), originOf(s.PublicURL)}
		allowed = append(allowed, s.App.ReturnOrigins...)
		for _, a := range allowed {
			if a != "" && strings.EqualFold(origin, a) {
				next.ServeHTTP(w, r)
				return
			}
		}
		writeJSON(w, http.StatusForbidden, errBody("bad_origin", "this request came from another website ("+origin+")"))
	})
}

func isHTTPS(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func requestOrigin(r *http.Request) string {
	scheme := "http"
	if isHTTPS(r) {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

func originOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

func (s *Server) setSessionCookie(w http.ResponseWriter, r *http.Request, token string) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode,
		Secure: isHTTPS(r), MaxAge: int(app.SessionTTL.Seconds())})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode,
		Secure: isHTTPS(r), MaxAge: -1})
}

// startSession signs u in: it sets the cookie and returns the account. A server-side frontend
// (the web app) asks for the token itself with "X-Kerdos-Return-Token: 1", to set its own cookie.
func (s *Server) startSession(w http.ResponseWriter, r *http.Request, status int, u app.User) error {
	token, err := s.App.NewSession(r.Context(), u.ID)
	if err != nil {
		return err
	}
	acc, err := s.App.Account(r.Context(), u.ID)
	if err != nil {
		return err
	}
	s.setSessionCookie(w, r, token)
	body := map[string]any{"user": acc}
	if r.Header.Get("X-Kerdos-Return-Token") == "1" {
		body["token"] = token
	}
	writeJSON(w, status, body)
	return nil
}

// ---------------------------------------------------------------- sign-in attempts

// attempts limits failed sign-ins: per address and per client IP, in a sliding window.
type attempts struct {
	mu   sync.Mutex
	hits map[string][]time.Time
}

const (
	attemptWindow   = 15 * time.Minute
	maxPerEmail     = 8
	maxPerIP        = 30
	attemptsMaxKeys = 10_000
)

func (l *attempts) recent(key string, now time.Time) []time.Time {
	kept := l.hits[key][:0]
	for _, t := range l.hits[key] {
		if now.Sub(t) < attemptWindow {
			kept = append(kept, t)
		}
	}
	if len(kept) == 0 {
		delete(l.hits, key)
		return nil
	}
	l.hits[key] = kept
	return kept
}

func (l *attempts) blocked(keys map[string]int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.hits == nil {
		l.hits = map[string][]time.Time{}
	}
	now := time.Now()
	for key, max := range keys {
		if len(l.recent(key, now)) >= max {
			return true
		}
	}
	return false
}

func (l *attempts) failed(keys ...string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.hits == nil || len(l.hits) > attemptsMaxKeys {
		l.hits = map[string][]time.Time{}
	}
	for _, key := range keys {
		l.hits[key] = append(l.hits[key], time.Now())
	}
}

func (l *attempts) succeeded(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.hits, key)
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// ---------------------------------------------------------------- handlers: signing in

type authStatus struct {
	Mode       string       `json:"mode"` // dev (everyone is the owner) or accounts
	NeedsSetup bool         `json:"needs_setup"`
	Google     bool         `json:"google"`
	User       *app.Account `json:"user"`
}

func (s *Server) authStatus(w http.ResponseWriter, r *http.Request) error {
	st := authStatus{Mode: s.AuthMode, Google: s.App.GoogleEnabled()}
	var err error
	if st.NeedsSetup, err = s.App.NeedsSetup(r.Context()); err != nil {
		return err
	}
	if u, _, err := s.currentUser(r); err == nil {
		if acc, err := s.App.Account(r.Context(), u.ID); err == nil {
			st.User = &acc
		}
	}
	writeJSON(w, http.StatusOK, st)
	return nil
}

func (s *Server) signUp(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
		Invite   string `json:"invite"`
	}
	if err := readJSON(w, r, &in); err != nil {
		return err
	}
	u, err := s.App.SignUp(r.Context(), in.Name, in.Email, in.Password, in.Invite)
	if err != nil {
		return err
	}
	return s.startSession(w, r, http.StatusCreated, u)
}

func (s *Server) logIn(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := readJSON(w, r, &in); err != nil {
		return err
	}
	emailKey := "email:" + strings.ToLower(strings.TrimSpace(in.Email))
	ipKey := "ip:" + clientIP(r)
	if s.attempts.blocked(map[string]int{emailKey: maxPerEmail, ipKey: maxPerIP}) {
		return &app.Error{Status: http.StatusTooManyRequests, Code: "too_many_attempts",
			Message: "Too many wrong passwords. Wait 15 minutes, or use Continue with Google."}
	}
	u, err := s.App.LogIn(r.Context(), in.Email, in.Password)
	if err != nil {
		if e, ok := err.(*app.Error); ok && e.Code == "wrong_login" {
			s.attempts.failed(emailKey, ipKey)
		}
		return err
	}
	s.attempts.succeeded(emailKey)
	return s.startSession(w, r, http.StatusOK, u)
}

func (s *Server) logOut(w http.ResponseWriter, r *http.Request) error {
	if err := s.App.EndSession(r.Context(), sessionToken(r)); err != nil {
		return err
	}
	s.clearSessionCookie(w, r)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) logOutEverywhere(w http.ResponseWriter, r *http.Request, u app.User) error {
	if err := s.App.EndSessions(r.Context(), u.ID, ""); err != nil {
		return err
	}
	s.clearSessionCookie(w, r)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) exchangeLoginCode(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Code string `json:"code"`
	}
	if err := readJSON(w, r, &in); err != nil {
		return err
	}
	u, err := s.App.ExchangeLoginCode(r.Context(), in.Code)
	if err != nil {
		return err
	}
	return s.startSession(w, r, http.StatusOK, u)
}

func (s *Server) inviteInfo(w http.ResponseWriter, r *http.Request) error {
	info, err := s.App.InviteInfo(r.Context(), r.PathValue("token"))
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, info)
	return nil
}

// googleStartRedirect is for a plain link ("Continue with Google" in the mockup).
func (s *Server) googleStartRedirect(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	target, err := s.App.GoogleStart(r.Context(), "signin", "", q.Get("invite"), q.Get("return_to"))
	if err != nil {
		e, ok := err.(*app.Error)
		if !ok || e.Code == "bad_return_to" {
			s.fail(w, r, err)
			return
		}
		target = withAuthError(q.Get("return_to"), e)
	}
	http.Redirect(w, r, target, http.StatusFound)
}

func withAuthError(returnTo string, e *app.Error) string {
	u, _ := url.Parse(returnTo)
	v := u.Query()
	v.Set("auth_error", e.Code)
	v.Set("auth_message", e.Message)
	u.RawQuery = v.Encode()
	return u.String()
}

// googleStart is for a frontend that redirects itself: it returns {"url": …}.
func (s *Server) googleStart(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		ReturnTo string `json:"return_to"`
		Invite   string `json:"invite"`
	}
	if err := readJSON(w, r, &in); err != nil {
		return err
	}
	target, err := s.App.GoogleStart(r.Context(), "signin", "", in.Invite, in.ReturnTo)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]string{"url": target})
	return nil
}

func (s *Server) googleCallback(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	http.Redirect(w, r, s.App.GoogleCallback(r.Context(), q.Get("state"), q.Get("code"), q.Get("error")), http.StatusFound)
}

// ---------------------------------------------------------------- handlers: the account page

func (s *Server) me(w http.ResponseWriter, r *http.Request, u app.User) error {
	acc, err := s.App.Account(r.Context(), u.ID)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, acc)
	return nil
}

func (s *Server) updateMe(w http.ResponseWriter, r *http.Request, u app.User) error {
	var in struct {
		Name            *string `json:"name"`
		Email           *string `json:"email"`
		CurrentPassword string  `json:"current_password"`
	}
	if err := readJSON(w, r, &in); err != nil {
		return err
	}
	acc, err := s.App.UpdateProfile(r.Context(), u, in.Name, in.Email, in.CurrentPassword)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, acc)
	return nil
}

func (s *Server) changePassword(w http.ResponseWriter, r *http.Request, u app.User) error {
	var in struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := readJSON(w, r, &in); err != nil {
		return err
	}
	token, _ := r.Context().Value(tokenKey{}).(string)
	if err := s.App.ChangePassword(r.Context(), u, in.CurrentPassword, in.NewPassword, token); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) linkGoogle(w http.ResponseWriter, r *http.Request, u app.User) error {
	var in struct {
		ReturnTo string `json:"return_to"`
	}
	if err := readJSON(w, r, &in); err != nil {
		return err
	}
	target, err := s.App.GoogleStart(r.Context(), "link", u.ID, "", in.ReturnTo)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]string{"url": target})
	return nil
}

func (s *Server) unlinkGoogle(w http.ResponseWriter, r *http.Request, u app.User) error {
	acc, err := s.App.UnlinkGoogle(r.Context(), u)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, acc)
	return nil
}

// ---------------------------------------------------------------- handlers: the team

func (s *Server) team(w http.ResponseWriter, r *http.Request, u app.User) error {
	members, invites, err := s.App.Team(r.Context(), u)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"members": members, "invites": invites})
	return nil
}

func (s *Server) invite(w http.ResponseWriter, r *http.Request, u app.User) error {
	var in struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := readJSON(w, r, &in); err != nil {
		return err
	}
	inv, link, err := s.App.InviteMember(r.Context(), u, in.Email, in.Name)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, map[string]any{"invite": inv, "link": link})
	return nil
}

func (s *Server) revokeInvite(w http.ResponseWriter, r *http.Request, u app.User) error {
	if err := s.App.RevokeInvite(r.Context(), u, r.PathValue("id")); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) removeMember(w http.ResponseWriter, r *http.Request, u app.User) error {
	if err := s.App.RemoveMember(r.Context(), u, r.PathValue("id")); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
