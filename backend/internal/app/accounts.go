package app

// Accounts: sign-up and sign-in with a password or with Google, sessions, the account page and
// the team. There is one business: the first account created becomes its owner (role admin),
// and everyone else joins through an invitation the owner sends.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/RostKostia/SKA/backend/internal/auth"
	"github.com/RostKostia/SKA/backend/internal/google"
)

const (
	SessionTTL   = 30 * 24 * time.Hour
	InviteTTL    = 7 * 24 * time.Hour
	stateTTL     = 10 * time.Minute
	loginCodeTTL = 2 * time.Minute
)

var (
	errSignedOut  = &Error{http.StatusUnauthorized, "signed_out", "Sign in to continue."}
	errWrongLogin = &Error{http.StatusUnauthorized, "wrong_login",
		"Wrong email or password. If you signed up with Google, use Continue with Google."}
	errRemoved       = forbidden("account_removed", "The owner removed this account from the business.")
	errInviteInvalid = &Error{http.StatusNotFound, "invite_invalid", "This invitation has expired or was already used. Ask the owner for a new one."}
	errGoogleInUse   = conflict("google_in_use", "That Google account is already linked to another Kerdos account.")
	errEmailTaken    = conflict("email_taken", "An account with this email already exists. Sign in instead.")
)

// Account is a user as the account and team pages show them.
type Account struct {
	ID           string     `json:"id"`
	Email        string     `json:"email"`
	Name         string     `json:"name"`
	Role         string     `json:"role"` // admin (the owner) or user
	HasPassword  bool       `json:"has_password"`
	GoogleLinked bool       `json:"google_linked"`
	GoogleEmail  string     `json:"google_email,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	LastLoginAt  *time.Time `json:"last_login_at"`
}

func (acc Account) User() User {
	return User{ID: acc.ID, Email: acc.Email, Name: acc.Name, Role: acc.Role}
}

const accountSelect = `SELECT id::text, email, name, role, password_hash IS NOT NULL, google_sub IS NOT NULL,
	coalesce(google_email, ''), created_at, last_login_at FROM users `

func scanAccount(s scanner) (Account, error) {
	var acc Account
	err := s.Scan(&acc.ID, &acc.Email, &acc.Name, &acc.Role, &acc.HasPassword, &acc.GoogleLinked, &acc.GoogleEmail, &acc.CreatedAt, &acc.LastLoginAt)
	return acc, err
}

// Account loads an active user.
func (a *App) Account(ctx context.Context, id string) (Account, error) {
	acc, err := scanAccount(a.DB.QueryRowContext(ctx, accountSelect+`WHERE id = $1 AND disabled_at IS NULL`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return acc, errSignedOut
	}
	return acc, err
}

// NeedsSetup is true while nobody can sign in yet: the next account created becomes the owner.
func (a *App) NeedsSetup(ctx context.Context) (bool, error) {
	return needsSetup(ctx, a.DB)
}

func needsSetup(ctx context.Context, q querier) (bool, error) {
	var setup bool
	err := q.QueryRowContext(ctx, `SELECT NOT EXISTS (SELECT 1 FROM users WHERE disabled_at IS NULL
		AND (password_hash IS NOT NULL OR google_sub IS NOT NULL))`).Scan(&setup)
	return setup, err
}

// ---------------------------------------------------------------- validation

var emailRe = regexp.MustCompile(`^[^\s@<>(),;:"\[\]\\]+@[^\s@<>(),;:"\[\]\\]+\.[^\s@<>(),;:"\[\]\\]+$`)

func cleanEmail(s string) (string, error) {
	s = strings.TrimSpace(s)
	if len(s) > 254 || !emailRe.MatchString(s) {
		return "", badRequest("bad_email", "Enter a valid email address.")
	}
	return s, nil
}

func cleanName(s string) (string, error) {
	s = strings.Join(strings.Fields(s), " ")
	if s == "" || utf8.RuneCountInString(s) > 100 || strings.IndexFunc(s, unicode.IsControl) >= 0 {
		return "", badRequest("bad_name", "Enter your name (up to 100 characters).")
	}
	return s, nil
}

func hashNewPassword(password string) (string, error) {
	h, err := auth.HashPassword(password)
	if errors.Is(err, auth.ErrWeakPassword) {
		return "", badRequest("weak_password", "Choose a password of at least 8 characters.")
	}
	return h, err
}

// ---------------------------------------------------------------- creating accounts

type newAccount struct {
	Name, Email           string
	PasswordHash          string
	GoogleSub, GoogleMail string
	InviteToken           string // from the invitation link
	InviteID              string // from a Google sign-in that started on the invitation link
	EmailVerified         bool   // Google vouches for the address: an open invitation to it is enough
}

// createAccount is the shared end of sign-up, by password or by Google. While NeedsSetup, the
// first account takes over the placeholder owner row from the seed (or becomes the owner).
// After that it needs an open invitation to the same address. An advisory lock makes sure two
// sign-ups at once can't both become the owner.
func (a *App) createAccount(ctx context.Context, tx *sql.Tx, n newAccount) (string, error) {
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext('kerdos:accounts'))`); err != nil {
		return "", err
	}
	setup, err := needsSetup(ctx, tx)
	if err != nil {
		return "", err
	}
	var existingID string
	var existingDisabled bool
	err = tx.QueryRowContext(ctx, `SELECT id::text, disabled_at IS NOT NULL FROM users WHERE lower(email) = lower($1)`, n.Email).
		Scan(&existingID, &existingDisabled)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}

	role, inviteID := "admin", ""
	if !setup {
		var inviteEmail string
		inviteID, inviteEmail, role, err = a.openInvite(ctx, tx, n)
		if err != nil {
			return "", err
		}
		if !strings.EqualFold(inviteEmail, n.Email) {
			return "", forbidden("invite_email_mismatch", "This invitation is for "+inviteEmail+". Use that address, or ask the owner to invite "+n.Email+".")
		}
		if existingID != "" && !existingDisabled {
			return "", errEmailTaken
		}
	}

	id := existingID
	switch {
	case setup:
		// Take over the placeholder owner, or the row that already has this address.
		var placeholder string
		err := tx.QueryRowContext(ctx, `SELECT id::text FROM users WHERE role = 'admin' AND disabled_at IS NULL
			AND password_hash IS NULL AND google_sub IS NULL ORDER BY created_at LIMIT 1 FOR UPDATE`).Scan(&placeholder)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return "", err
		}
		if existingID != "" && existingID != placeholder {
			return "", errEmailTaken
		}
		id = placeholder
	}
	// (A removed member who is invited back gets their old row, so their history stays theirs.)
	if id == "" {
		err = tx.QueryRowContext(ctx, `INSERT INTO users (email, name, role, password_hash, google_sub, google_email, last_login_at)
			VALUES ($1, $2, $3, $4, $5, $6, now()) RETURNING id::text`,
			n.Email, n.Name, role, nullable(n.PasswordHash), nullable(n.GoogleSub), nullable(n.GoogleMail)).Scan(&id)
	} else {
		_, err = tx.ExecContext(ctx, `UPDATE users SET email = $2, name = $3, role = $4, password_hash = $5, google_sub = $6,
			google_email = $7, disabled_at = NULL, last_login_at = now() WHERE id = $1`,
			id, n.Email, n.Name, role, nullable(n.PasswordHash), nullable(n.GoogleSub), nullable(n.GoogleMail))
	}
	if err != nil {
		return "", uniqueErr(err)
	}
	if inviteID != "" {
		if _, err := tx.ExecContext(ctx, `UPDATE invites SET accepted_at = now(), accepted_by = $2 WHERE id = $1`, inviteID, id); err != nil {
			return "", err
		}
	}
	a.audit(ctx, tx, id, "account.created", "user", id, map[string]any{"role": role, "google": n.GoogleSub != "", "invite": inviteID != ""})
	return id, nil
}

// openInvite finds the invitation a sign-up relies on: by the link's token, by the id carried
// through Google, or (when Google vouches for the address) by the address alone.
func (a *App) openInvite(ctx context.Context, tx *sql.Tx, n newAccount) (id, email, role string, err error) {
	const open = ` AND accepted_at IS NULL AND revoked_at IS NULL AND expires_at > now() FOR UPDATE`
	var row *sql.Row
	switch {
	case n.InviteToken != "":
		if !auth.ValidToken(n.InviteToken) {
			return "", "", "", errInviteInvalid
		}
		row = tx.QueryRowContext(ctx, `SELECT id::text, email, role FROM invites WHERE token_hash = $1`+open, auth.HashToken(n.InviteToken))
	case n.InviteID != "":
		row = tx.QueryRowContext(ctx, `SELECT id::text, email, role FROM invites WHERE id = $1`+open, n.InviteID)
	case n.EmailVerified:
		row = tx.QueryRowContext(ctx, `SELECT id::text, email, role FROM invites WHERE lower(email) = lower($1)`+open, n.Email)
	default:
		return "", "", "", forbidden("invite_required", "Kerdos is invite-only. Ask the owner of the business to invite you.")
	}
	err = row.Scan(&id, &email, &role)
	if errors.Is(err, sql.ErrNoRows) {
		if n.InviteToken == "" && n.InviteID == "" {
			return "", "", "", forbidden("invite_required", "Kerdos is invite-only. Ask the owner of the business to invite you.")
		}
		return "", "", "", errInviteInvalid
	}
	return id, email, role, err
}

func uniqueErr(err error) error {
	if sqlState(err) != "23505" {
		return err
	}
	if strings.Contains(err.Error(), "google_sub") {
		return errGoogleInUse
	}
	return errEmailTaken
}

// SignUp creates an account with a password. It needs an invitation, except for the very
// first account, which becomes the owner.
func (a *App) SignUp(ctx context.Context, name, email, password, inviteToken string) (User, error) {
	name, err := cleanName(name)
	if err != nil {
		return User{}, err
	}
	if email, err = cleanEmail(email); err != nil {
		return User{}, err
	}
	hash, err := hashNewPassword(password)
	if err != nil {
		return User{}, err
	}
	var id string
	err = a.tx(ctx, func(tx *sql.Tx) error {
		id, err = a.createAccount(ctx, tx, newAccount{Name: name, Email: email, PasswordHash: hash, InviteToken: inviteToken})
		return err
	})
	if err != nil {
		return User{}, err
	}
	acc, err := a.Account(ctx, id)
	return acc.User(), err
}

// LogIn checks an email and password.
func (a *App) LogIn(ctx context.Context, email, password string) (User, error) {
	email = strings.TrimSpace(email)
	var id, hash string
	var disabled bool
	err := a.DB.QueryRowContext(ctx, `SELECT id::text, coalesce(password_hash, ''), disabled_at IS NOT NULL FROM users
		WHERE lower(email) = lower($1)`, email).Scan(&id, &hash, &disabled)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return User{}, err
	}
	if hash == "" {
		auth.BurnPasswordCheck(password) // same time as a real check, so this doesn't reveal which emails exist
		return User{}, errWrongLogin
	}
	if !auth.CheckPassword(hash, password) {
		return User{}, errWrongLogin
	}
	if disabled {
		return User{}, errRemoved
	}
	acc, err := a.Account(ctx, id)
	return acc.User(), err
}

// ---------------------------------------------------------------- sessions

// NewSession signs a user in and returns the session token (only its hash is stored).
func (a *App) NewSession(ctx context.Context, userID string) (string, error) {
	token, err := auth.NewToken()
	if err != nil {
		return "", err
	}
	err = a.tx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at < now()`); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO sessions (token_hash, user_id, expires_at) VALUES ($1, $2, $3)`,
			auth.HashToken(token), userID, time.Now().Add(SessionTTL)); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `UPDATE users SET last_login_at = now() WHERE id = $1`, userID)
		return err
	})
	return token, err
}

// SessionUser is the signed-in user for a session token.
func (a *App) SessionUser(ctx context.Context, token string) (User, error) {
	if !auth.ValidToken(token) {
		return User{}, errSignedOut
	}
	var u User
	err := a.DB.QueryRowContext(ctx, `SELECT u.id::text, u.email, u.name, u.role FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = $1 AND s.expires_at > now() AND u.disabled_at IS NULL`, auth.HashToken(token)).
		Scan(&u.ID, &u.Email, &u.Name, &u.Role)
	if errors.Is(err, sql.ErrNoRows) {
		return u, errSignedOut
	}
	return u, err
}

// EndSession signs one session out.
func (a *App) EndSession(ctx context.Context, token string) error {
	if !auth.ValidToken(token) {
		return nil
	}
	_, err := a.DB.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = $1`, auth.HashToken(token))
	return err
}

// EndSessions signs a user out everywhere, except the session with keepToken (if any).
func (a *App) EndSessions(ctx context.Context, userID, keepToken string) error {
	keep := []byte{}
	if keepToken != "" {
		keep = auth.HashToken(keepToken)
	}
	_, err := a.DB.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = $1 AND token_hash <> $2`, userID, keep)
	return err
}

// ExchangeLoginCode swaps the one-time code from a finished Google sign-in for the user. The
// caller then starts a session.
func (a *App) ExchangeLoginCode(ctx context.Context, code string) (User, error) {
	bad := badRequest("login_code_invalid", "This sign-in has expired. Sign in again.")
	if !auth.ValidToken(code) {
		return User{}, bad
	}
	var id string
	err := a.DB.QueryRowContext(ctx, `UPDATE login_codes SET used_at = now()
		WHERE code_hash = $1 AND used_at IS NULL AND expires_at > now() RETURNING user_id::text`, auth.HashToken(code)).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, bad
	}
	if err != nil {
		return User{}, err
	}
	acc, err := a.Account(ctx, id)
	return acc.User(), err
}

// ---------------------------------------------------------------- Google

// GoogleEnabled says whether "Continue with Google" can be offered.
func (a *App) GoogleEnabled() bool { return a.Google != nil }

// safeReturn accepts only addresses on the frontend (KERDOS_PUBLIC_URL) or on this server
// (where the mockup lives), so the sign-in can't be bounced to someone else's site.
func (a *App) safeReturn(raw string) (string, error) {
	bad := badRequest("bad_return_to", "return_to must be a page of the Kerdos frontend or the mockup")
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || u.Fragment != "" || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", bad
	}
	origin := u.Scheme + "://" + u.Host
	for _, base := range a.ReturnOrigins {
		if strings.EqualFold(origin, base) {
			return u.String(), nil
		}
	}
	return "", bad
}

// GoogleStart begins "Continue with Google" and returns the Google page to send the browser
// to. purpose is "signin" (with an optional invitation token) or "link" (userID set).
func (a *App) GoogleStart(ctx context.Context, purpose, userID, inviteToken, returnTo string) (string, error) {
	// The return address is checked first: the plain-link variant sends the browser back to it
	// with any later error, so it must never be somewhere else.
	rt, err := a.safeReturn(returnTo)
	if err != nil {
		return "", err
	}
	if a.Google == nil {
		return "", conflict("google_off", "Google sign-in isn't set up on this server: set GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET.")
	}
	var inviteID, hint string
	if inviteToken != "" {
		if !auth.ValidToken(inviteToken) {
			return "", errInviteInvalid
		}
		err := a.DB.QueryRowContext(ctx, `SELECT id::text, email FROM invites WHERE token_hash = $1
			AND accepted_at IS NULL AND revoked_at IS NULL AND expires_at > now()`, auth.HashToken(inviteToken)).Scan(&inviteID, &hint)
		if errors.Is(err, sql.ErrNoRows) {
			return "", errInviteInvalid
		}
		if err != nil {
			return "", err
		}
	}
	state, err := auth.NewToken()
	if err != nil {
		return "", err
	}
	verifier, challenge, err := auth.PKCE()
	if err != nil {
		return "", err
	}
	err = a.tx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM oauth_states WHERE expires_at < now()`); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO oauth_states (state_hash, purpose, user_id, invite_id, code_verifier, return_to, expires_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`, auth.HashToken(state), purpose, nullable(userID), nullable(inviteID), verifier, rt, time.Now().Add(stateTTL))
		return err
	})
	if err != nil {
		return "", err
	}
	return a.Google.AuthCodeURL(state, challenge, hint), nil
}

// GoogleCallback finishes "Continue with Google" and always returns where to send the browser:
// the return_to page with ?login_code=… (sign-in), ?google=linked (link), or ?auth_error=…
// and a readable auth_message.
func (a *App) GoogleCallback(ctx context.Context, state, code, googleErr string) string {
	fallback := a.PublicURL + "/login"
	var purpose, userID, inviteID, verifier, returnTo string
	err := errors.New("no state")
	if auth.ValidToken(state) {
		err = a.DB.QueryRowContext(ctx, `DELETE FROM oauth_states WHERE state_hash = $1 AND expires_at > now()
			RETURNING purpose, coalesce(user_id::text, ''), coalesce(invite_id::text, ''), code_verifier, return_to`,
			auth.HashToken(state)).Scan(&purpose, &userID, &inviteID, &verifier, &returnTo)
	}
	if err != nil {
		return withQuery(fallback, "auth_error", "google_state", "auth_message", "That Google sign-in expired. Try again.")
	}
	fail := func(e error) string {
		var appErr *Error
		if !errors.As(e, &appErr) {
			a.Log.Error("google sign-in failed", "err", e)
			appErr = &Error{Code: "google_failed", Message: "Google sign-in didn't work. Try again."}
		}
		return withQuery(returnTo, "auth_error", appErr.Code, "auth_message", appErr.Message)
	}
	switch {
	case googleErr == "access_denied":
		return fail(&Error{Code: "google_cancelled", Message: "Google sign-in was cancelled."})
	case googleErr != "" || code == "":
		return fail(fmt.Errorf("google returned error %q", googleErr))
	}
	id, err := a.Google.Exchange(ctx, code, verifier)
	if err != nil {
		return fail(err)
	}
	if purpose == "link" {
		if err := a.linkGoogle(ctx, userID, id); err != nil {
			return fail(err)
		}
		return withQuery(returnTo, "google", "linked")
	}
	uid, err := a.googleSignIn(ctx, id, inviteID)
	if err != nil {
		return fail(err)
	}
	loginCode, err := auth.NewToken()
	if err != nil {
		return fail(err)
	}
	if _, err := a.DB.ExecContext(ctx, `INSERT INTO login_codes (code_hash, user_id, expires_at) VALUES ($1, $2, $3)`,
		auth.HashToken(loginCode), uid, time.Now().Add(loginCodeTTL)); err != nil {
		return fail(err)
	}
	_, _ = a.DB.ExecContext(ctx, `DELETE FROM login_codes WHERE expires_at < now() - interval '1 day'`)
	return withQuery(returnTo, "login_code", loginCode)
}

// googleSignIn finds or creates the account for a Google identity: by Google id, then by
// verified email (which links Google to an existing password account), else a new account.
func (a *App) googleSignIn(ctx context.Context, id google.Identity, inviteID string) (string, error) {
	var uid string
	err := a.tx(ctx, func(tx *sql.Tx) error {
		var disabled bool
		err := tx.QueryRowContext(ctx, `SELECT id::text, disabled_at IS NOT NULL FROM users WHERE google_sub = $1`, id.Sub).Scan(&uid, &disabled)
		removed := err == nil && disabled
		if err == nil && !disabled {
			_, err = tx.ExecContext(ctx, `UPDATE users SET google_email = $2 WHERE id = $1`, uid, nullable(id.Email))
			return err
		}
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		// A removed member can come back only through a new invitation (handled below).
		if !id.EmailVerified || id.Email == "" {
			return forbidden("google_unverified", "Your Google account's email address isn't verified, so Kerdos can't use it.")
		}
		setup, err := needsSetup(ctx, tx)
		if err != nil {
			return err
		}
		err = tx.QueryRowContext(ctx, `SELECT id::text, disabled_at IS NOT NULL FROM users WHERE lower(email) = lower($1)
			AND (password_hash IS NOT NULL OR NOT $2)`, id.Email, setup).Scan(&uid, &disabled)
		if err == nil && !disabled {
			// The same address already has an account: Google now signs in to it too.
			_, err = tx.ExecContext(ctx, `UPDATE users SET google_sub = $2, google_email = $3 WHERE id = $1`, uid, id.Sub, id.Email)
			if err == nil {
				a.audit(ctx, tx, uid, "account.google_linked", "user", uid, map[string]any{"by": "email"})
			}
			return uniqueErr(err)
		}
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		name := id.Name
		if strings.TrimSpace(name) == "" {
			name, _, _ = strings.Cut(id.Email, "@")
		}
		if name, err = cleanName(name); err != nil {
			name = "New member"
		}
		uid, err = a.createAccount(ctx, tx, newAccount{Name: name, Email: id.Email, GoogleSub: id.Sub, GoogleMail: id.Email,
			InviteID: inviteID, EmailVerified: true})
		var appErr *Error
		if removed && errors.As(err, &appErr) && appErr.Code == "invite_required" {
			return errRemoved
		}
		return err
	})
	return uid, err
}

func (a *App) linkGoogle(ctx context.Context, userID string, id google.Identity) error {
	var owner string
	err := a.DB.QueryRowContext(ctx, `SELECT id::text FROM users WHERE google_sub = $1`, id.Sub).Scan(&owner)
	switch {
	case err == nil && owner != userID:
		return errGoogleInUse
	case err != nil && !errors.Is(err, sql.ErrNoRows):
		return err
	}
	res, err := a.DB.ExecContext(ctx, `UPDATE users SET google_sub = $2, google_email = $3 WHERE id = $1 AND disabled_at IS NULL`,
		userID, id.Sub, nullable(id.Email))
	if err != nil {
		return uniqueErr(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errSignedOut
	}
	a.audit(ctx, a.DB, userID, "account.google_linked", "user", userID, nil)
	return nil
}

func withQuery(raw string, kv ...string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	q := u.Query()
	for _, k := range []string{"login_code", "auth_error", "auth_message", "google"} {
		q.Del(k)
	}
	for i := 0; i+1 < len(kv); i += 2 {
		q.Set(kv[i], kv[i+1])
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// ---------------------------------------------------------------- the account page

// UpdateProfile changes the name and/or email. Changing the email of an account with a
// password needs that password.
func (a *App) UpdateProfile(ctx context.Context, u User, name, email *string, currentPassword string) (Account, error) {
	acc, err := a.Account(ctx, u.ID)
	if err != nil {
		return acc, err
	}
	newName, newEmail := acc.Name, acc.Email
	if name != nil {
		if newName, err = cleanName(*name); err != nil {
			return acc, err
		}
	}
	if email != nil {
		if newEmail, err = cleanEmail(*email); err != nil {
			return acc, err
		}
	}
	if !strings.EqualFold(newEmail, acc.Email) {
		if acc.HasPassword {
			if err := a.checkCurrentPassword(ctx, u.ID, currentPassword); err != nil {
				return acc, err
			}
		}
		// An invited address belongs to the person invited: taking it would let their Google
		// sign-in land in this account.
		var invited bool
		if err := a.DB.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM invites WHERE lower(email) = lower($1)
			AND accepted_at IS NULL AND revoked_at IS NULL AND expires_at > now())`, newEmail).Scan(&invited); err != nil {
			return acc, err
		}
		if invited {
			return acc, conflict("invited_address", newEmail+" has an open invitation, so it can't be used for another account.")
		}
	}
	if _, err := a.DB.ExecContext(ctx, `UPDATE users SET name = $2, email = $3 WHERE id = $1`, u.ID, newName, newEmail); err != nil {
		return acc, uniqueErr(err)
	}
	a.audit(ctx, a.DB, u.ID, "account.updated", "user", u.ID, map[string]any{"email_changed": !strings.EqualFold(newEmail, acc.Email)})
	return a.Account(ctx, u.ID)
}

func (a *App) checkCurrentPassword(ctx context.Context, userID, password string) error {
	var hash string
	if err := a.DB.QueryRowContext(ctx, `SELECT coalesce(password_hash, '') FROM users WHERE id = $1`, userID).Scan(&hash); err != nil {
		return err
	}
	if password == "" {
		return badRequest("current_password_needed", "Enter your current password.")
	}
	if !auth.CheckPassword(hash, password) {
		return forbidden("wrong_password", "Your current password is wrong.")
	}
	return nil
}

// ChangePassword sets a new password (or a first one, for an account that only used Google)
// and signs out every other session.
func (a *App) ChangePassword(ctx context.Context, u User, current, next, keepToken string) error {
	acc, err := a.Account(ctx, u.ID)
	if err != nil {
		return err
	}
	if acc.HasPassword {
		if err := a.checkCurrentPassword(ctx, u.ID, current); err != nil {
			return err
		}
	}
	hash, err := hashNewPassword(next)
	if err != nil {
		return err
	}
	if _, err := a.DB.ExecContext(ctx, `UPDATE users SET password_hash = $2 WHERE id = $1`, u.ID, hash); err != nil {
		return err
	}
	a.audit(ctx, a.DB, u.ID, "account.password_changed", "user", u.ID, nil)
	return a.EndSessions(ctx, u.ID, keepToken)
}

// UnlinkGoogle disconnects Google. Only possible with a password, so nobody locks themselves out.
func (a *App) UnlinkGoogle(ctx context.Context, u User) (Account, error) {
	acc, err := a.Account(ctx, u.ID)
	if err != nil {
		return acc, err
	}
	if !acc.HasPassword {
		return acc, conflict("needs_password", "Set a password first, so you can still sign in without Google.")
	}
	if _, err := a.DB.ExecContext(ctx, `UPDATE users SET google_sub = NULL, google_email = NULL WHERE id = $1`, u.ID); err != nil {
		return acc, err
	}
	a.audit(ctx, a.DB, u.ID, "account.google_unlinked", "user", u.ID, nil)
	return a.Account(ctx, u.ID)
}

// ---------------------------------------------------------------- the team (owner only)

type Member struct {
	Account
	You bool `json:"you"`
}

type Invite struct {
	ID             string    `json:"id"`
	Email          string    `json:"email"`
	Name           string    `json:"name,omitempty"`
	Role           string    `json:"role"`
	InvitedBy      string    `json:"invited_by,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	ExpiresAt      time.Time `json:"expires_at"`
	Expired        bool      `json:"expired"`
	EmailMessageID string    `json:"email_message_id,omitempty"`
	EmailStatus    string    `json:"email_status,omitempty"` // queued, sent, failed or cancelled
	EmailError     string    `json:"email_error,omitempty"`
}

// Team lists the active members and the open invitations.
func (a *App) Team(ctx context.Context, u User) ([]Member, []Invite, error) {
	rows, err := a.DB.QueryContext(ctx, accountSelect+`WHERE disabled_at IS NULL ORDER BY role = 'admin' DESC, created_at`)
	if err != nil {
		return nil, nil, err
	}
	members := []Member{}
	for rows.Next() {
		acc, err := scanAccount(rows)
		if err != nil {
			rows.Close()
			return nil, nil, err
		}
		if !acc.HasPassword && !acc.GoogleLinked {
			continue // the seed's placeholder owner, before anyone took it over
		}
		members = append(members, Member{Account: acc, You: acc.ID == u.ID})
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	invites, err := a.openInvites(ctx, "")
	return members, invites, err
}

func (a *App) openInvites(ctx context.Context, id string) ([]Invite, error) {
	rows, err := a.DB.QueryContext(ctx, `SELECT i.id::text, i.email, coalesce(i.name, ''), i.role, coalesce(u.name, ''), i.created_at,
			i.expires_at, i.expires_at <= now(), coalesce(m.id::text, ''), coalesce(m.status, ''), coalesce(m.error, '')
		FROM invites i
		LEFT JOIN users u ON u.id = i.invited_by
		LEFT JOIN LATERAL (SELECT id, status, error FROM outbound_messages WHERE invite_id = i.id ORDER BY created_at DESC LIMIT 1) m ON true
		WHERE i.accepted_at IS NULL AND i.revoked_at IS NULL AND ($1 = '' OR i.id::text = $1)
		ORDER BY i.created_at DESC`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Invite{}
	for rows.Next() {
		var inv Invite
		if err := rows.Scan(&inv.ID, &inv.Email, &inv.Name, &inv.Role, &inv.InvitedBy, &inv.CreatedAt, &inv.ExpiresAt, &inv.Expired,
			&inv.EmailMessageID, &inv.EmailStatus, &inv.EmailError); err != nil {
			return nil, err
		}
		if inv.EmailError != "" {
			inv.EmailError = emailProblem(inv.EmailError)
		}
		out = append(out, inv)
	}
	return out, rows.Err()
}

// InviteMember invites someone by email. It replaces any open invitation to the same address,
// emails the link and returns it too, so the owner can also send it another way.
func (a *App) InviteMember(ctx context.Context, u User, email, name string) (Invite, string, error) {
	email, err := cleanEmail(email)
	if err != nil {
		return Invite{}, "", err
	}
	if strings.TrimSpace(name) != "" {
		if name, err = cleanName(name); err != nil {
			return Invite{}, "", err
		}
	}
	token, err := auth.NewToken()
	if err != nil {
		return Invite{}, "", err
	}
	link := a.PublicURL + "/signup?invite=" + token
	var inviteID, msgID string
	err = a.tx(ctx, func(tx *sql.Tx) error {
		var member bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE lower(email) = lower($1) AND disabled_at IS NULL
			AND (password_hash IS NOT NULL OR google_sub IS NOT NULL))`, email).Scan(&member); err != nil {
			return err
		}
		if member {
			return conflict("already_member", email+" already has an account here.")
		}
		if _, err := tx.ExecContext(ctx, `UPDATE invites SET revoked_at = now()
			WHERE lower(email) = lower($1) AND accepted_at IS NULL AND revoked_at IS NULL`, email); err != nil {
			return err
		}
		if err := tx.QueryRowContext(ctx, `INSERT INTO invites (email, name, role, token_hash, invited_by, expires_at)
			VALUES ($1, $2, 'user', $3, $4, $5) RETURNING id::text`,
			email, nullable(name), auth.HashToken(token), u.ID, time.Now().Add(InviteTTL)).Scan(&inviteID); err != nil {
			return err
		}
		var company string
		if err := tx.QueryRowContext(ctx, `SELECT coalesce((SELECT name FROM company), 'your business')`).Scan(&company); err != nil {
			return err
		}
		hello := "Hi,"
		if name != "" {
			hello = "Hi " + name + ","
		}
		subject := "Join " + company + " on Kerdos"
		body := fmt.Sprintf("%s\n\n%s invited you to %s on Kerdos, the assistant that sends the invoices and chases late payments.\n\n"+
			"Create your account with this link (it works for 7 days):\n%s\n\n"+
			"You can choose a password, or use Continue with Google with this address (%s).\n",
			hello, u.Name, company, link, email)
		if err := tx.QueryRowContext(ctx, `INSERT INTO outbound_messages (kind, mode, invite_id, recipient, subject, body, created_by)
			VALUES ('invite', 'manual', $1, $2, $3, $4, $5) RETURNING id::text`, inviteID, email, subject, body, u.ID).Scan(&msgID); err != nil {
			return err
		}
		a.audit(ctx, tx, u.ID, "team.invited", "invite", inviteID, map[string]any{"email": email})
		return nil
	})
	if err != nil {
		return Invite{}, "", err
	}
	if _, err := a.SendNow(ctx, msgID); err != nil {
		a.Log.Warn("invite email", "err", err)
	}
	invites, err := a.openInvites(ctx, inviteID)
	if err != nil || len(invites) == 0 {
		return Invite{ID: inviteID, Email: email}, link, err
	}
	return invites[0], link, nil
}

// RevokeInvite cancels an open invitation; its link stops working.
func (a *App) RevokeInvite(ctx context.Context, u User, id string) error {
	if !ValidID(id) {
		return notFound("invitation")
	}
	res, err := a.DB.ExecContext(ctx, `UPDATE invites SET revoked_at = now() WHERE id = $1 AND accepted_at IS NULL AND revoked_at IS NULL`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return notFound("invitation")
	}
	a.audit(ctx, a.DB, u.ID, "team.invite_revoked", "invite", id, nil)
	return nil
}

// RemoveMember takes someone out of the business: they can't sign in and their sessions end.
// Their past actions stay attributed to them.
func (a *App) RemoveMember(ctx context.Context, u User, id string) error {
	if !ValidID(id) {
		return notFound("member")
	}
	if id == u.ID {
		return conflict("cant_remove_self", "You can't remove yourself.")
	}
	return a.tx(ctx, func(tx *sql.Tx) error {
		var role string
		err := tx.QueryRowContext(ctx, `SELECT role FROM users WHERE id = $1 AND disabled_at IS NULL FOR UPDATE`, id).Scan(&role)
		if errors.Is(err, sql.ErrNoRows) {
			return notFound("member")
		}
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE users SET disabled_at = now() WHERE id = $1`, id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = $1`, id); err != nil {
			return err
		}
		a.audit(ctx, tx, u.ID, "team.member_removed", "user", id, nil)
		return nil
	})
}

// InviteInfo is what the sign-up page shows for an invitation link.
type InviteInfo struct {
	Email     string    `json:"email"`
	Name      string    `json:"name,omitempty"`
	Company   string    `json:"company"`
	InvitedBy string    `json:"invited_by,omitempty"`
	ExpiresAt time.Time `json:"expires_at"`
}

func (a *App) InviteInfo(ctx context.Context, token string) (InviteInfo, error) {
	var info InviteInfo
	if !auth.ValidToken(token) {
		return info, errInviteInvalid
	}
	err := a.DB.QueryRowContext(ctx, `SELECT i.email, coalesce(i.name, ''), coalesce((SELECT name FROM company), ''), coalesce(u.name, ''), i.expires_at
		FROM invites i LEFT JOIN users u ON u.id = i.invited_by
		WHERE i.token_hash = $1 AND i.accepted_at IS NULL AND i.revoked_at IS NULL AND i.expires_at > now()`, auth.HashToken(token)).
		Scan(&info.Email, &info.Name, &info.Company, &info.InvitedBy, &info.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return info, errInviteInvalid
	}
	return info, err
}

// inviteLinkRe finds the token in an invitation email, to hide it in the email log.
var inviteLinkRe = regexp.MustCompile(`invite=[A-Za-z0-9_-]{43}`)
