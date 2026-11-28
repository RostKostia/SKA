package httpapi_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/RostKostia/SKA/backend/internal/ai"
	"github.com/RostKostia/SKA/backend/internal/app"
	"github.com/RostKostia/SKA/backend/internal/auth"
	"github.com/RostKostia/SKA/backend/internal/google"
	"github.com/RostKostia/SKA/backend/internal/httpapi"
	"github.com/RostKostia/SKA/backend/internal/jinius"
)

// fakeGoogle is Google's token endpoint: each code stands for one Google account.
type fakeGoogle struct {
	mu    sync.Mutex
	codes map[string]map[string]any
}

func (f *fakeGoogle) add(code, sub, email, name string, verified bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.codes[code] = map[string]any{"iss": "https://accounts.google.com", "aud": "test-client", "sub": sub, "email": email,
		"email_verified": verified, "name": name, "exp": time.Now().Add(time.Hour).Unix()}
}

func (f *fakeGoogle) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	f.mu.Lock()
	claims, ok := f.codes[r.PostForm.Get("code")]
	f.mu.Unlock()
	if !ok || r.PostForm.Get("code_verifier") == "" {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
		return
	}
	body, _ := json.Marshal(claims)
	tok := "e30." + base64.RawURLEncoding.EncodeToString(body) + ".sig"
	_ = json.NewEncoder(w).Encode(map[string]string{"id_token": tok})
}

// browser is one person's client: it keeps their session token.
type browser struct {
	t      *testing.T
	base   string
	token  string
	origin string
	http   *http.Client
}

func (b *browser) call(method, path string, body any, want int, out any) {
	b.t.Helper()
	var r io.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		r = bytes.NewReader(raw)
	}
	req, _ := http.NewRequest(method, b.base+path, r)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Kerdos-Return-Token", "1")
	if b.token != "" {
		req.Header.Set("Authorization", "Bearer "+b.token)
	}
	if b.origin != "" {
		req.Header.Set("Origin", b.origin)
	}
	res, err := b.http.Do(req)
	if err != nil {
		b.t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode != want {
		b.t.Fatalf("%s %s: status %d, want %d: %s", method, path, res.StatusCode, want, raw)
	}
	var session struct {
		Token string `json:"token"`
	}
	if json.Unmarshal(raw, &session) == nil && session.Token != "" {
		b.token = session.Token
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			b.t.Fatalf("%s %s: %v: %s", method, path, err, raw)
		}
	}
}

func errCode(t *testing.T, b *browser, method, path string, body any, want int) string {
	t.Helper()
	var e struct {
		Error struct{ Code, Message string } `json:"error"`
	}
	b.call(method, path, body, want, &e)
	return e.Error.Code
}

// googleSignIn walks "Continue with Google": start, (Google), callback. It returns where the
// browser lands.
func googleSignIn(t *testing.T, base string, start func() string, code string) *url.URL {
	t.Helper()
	authURL, err := url.Parse(start())
	if err != nil {
		t.Fatal(err)
	}
	state := authURL.Query().Get("state")
	q := url.Values{"state": {state}}
	if code == "cancel" {
		q.Set("error", "access_denied")
	} else {
		q.Set("code", code)
	}
	noFollow := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := noFollow.Get(base + "/api/auth/google/callback?" + q.Encode())
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusFound {
		t.Fatalf("callback status %d", res.StatusCode)
	}
	loc, _ := url.Parse(res.Header.Get("Location"))
	return loc
}

// TestAccounts walks sign-up, sign-in, the account page, the team and Google sign-in with
// KERDOS_AUTH=accounts.
func TestAccounts(t *testing.T) {
	dsn := os.Getenv("KERDOS_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set KERDOS_TEST_DATABASE_URL to a migrated database this test may reset")
	}
	auth.Iterations = 1000
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `TRUNCATE users, invites, oauth_states, login_codes, sessions CASCADE`); err != nil {
		t.Fatal(err)
	}

	fg := &fakeGoogle{codes: map[string]map[string]any{}}
	gsrv := httptest.NewServer(fg)
	defer gsrv.Close()
	var logOut io.Writer = io.Discard
	if os.Getenv("KERDOS_TEST_LOG") != "" {
		logOut = os.Stderr
	}
	quiet := slog.New(slog.NewTextHandler(logOut, nil))
	mailer := &switchMailer{}
	a := &app.App{DB: db, Log: quiet, Parser: ai.Rules{}, Deliverer: jinius.Mock{PublicURL: "http://front"}, Mailer: mailer,
		PublicURL: "http://front", Demo: true, SeedFile: "../../../db/seed/sample.sql", FeeBP: 150,
		Google: &google.Client{ClientID: "test-client", ClientSecret: "s", RedirectURI: "http://api/cb", AuthURL: "http://google.test/auth", TokenURL: gsrv.URL}}
	srv := httptest.NewServer((&httpapi.Server{App: a, Log: quiet, AuthMode: "accounts", PublicURL: "http://front"}).Handler())
	defer srv.Close()
	a.ReturnOrigins = []string{"http://front", srv.URL}
	if err := a.ResetDemo(ctx); err != nil {
		t.Fatal(err)
	}
	var seedOwner string
	_ = db.QueryRowContext(ctx, `SELECT id::text FROM users`).Scan(&seedOwner)

	newBrowser := func() *browser { return &browser{t: t, base: srv.URL, http: &http.Client{}} }
	anon := newBrowser()

	// ---- Before anyone has an account.
	var st struct {
		Mode       string       `json:"mode"`
		NeedsSetup bool         `json:"needs_setup"`
		Google     bool         `json:"google"`
		User       *app.Account `json:"user"`
	}
	anon.call("GET", "/api/auth/status", nil, 200, &st)
	if st.Mode != "accounts" || !st.NeedsSetup || !st.Google || st.User != nil {
		t.Fatalf("status before setup: %+v", st)
	}
	if code := errCode(t, anon, "GET", "/api/dashboard", nil, 401); code != "signed_out" {
		t.Errorf("no session: %s", code)
	}
	if code := errCode(t, anon, "POST", "/api/demo/reset", nil, 401); code != "signed_out" {
		t.Errorf("reset without session: %s", code)
	}

	// ---- The first account becomes the owner, taking over the seed's placeholder.
	owner := newBrowser()
	var signed struct {
		User app.Account `json:"user"`
	}
	if code := errCode(t, owner, "POST", "/api/auth/signup", map[string]string{"name": "Anna", "email": "anna@example.com", "password": "short"}, 400); code != "weak_password" {
		t.Errorf("weak password: %s", code)
	}
	owner.call("POST", "/api/auth/signup", map[string]string{"name": " Anna  Georgiou ", "email": "Anna@Example.com", "password": "anna-secret-1"}, 201, &signed)
	if signed.User.Role != "admin" || signed.User.Name != "Anna Georgiou" || signed.User.ID != seedOwner || !signed.User.HasPassword || owner.token == "" {
		t.Fatalf("owner: %+v (seed owner %s)", signed.User, seedOwner)
	}
	anon.call("GET", "/api/auth/status", nil, 200, &st)
	if st.NeedsSetup {
		t.Error("still needs setup after the owner signed up")
	}
	if code := errCode(t, newBrowser(), "POST", "/api/auth/signup", map[string]string{"name": "Eve", "email": "eve@example.com", "password": "eve-secret-1"}, 403); code != "invite_required" {
		t.Errorf("second sign-up without invite: %s", code)
	}
	owner.call("GET", "/api/dashboard", nil, 200, nil)

	// ---- Signing in, wrong passwords, rate limit, cookies and CSRF.
	if code := errCode(t, newBrowser(), "POST", "/api/auth/login", map[string]string{"email": "anna@example.com", "password": "wrong-password"}, 401); code != "wrong_login" {
		t.Errorf("wrong password: %s", code)
	}
	if code := errCode(t, newBrowser(), "POST", "/api/auth/login", map[string]string{"email": "nobody@example.com", "password": "whatever-1"}, 401); code != "wrong_login" {
		t.Errorf("unknown email: %s", code)
	}
	second := newBrowser()
	second.call("POST", "/api/auth/login", map[string]string{"email": "ANNA@example.com", "password": "anna-secret-1"}, 200, nil)
	if second.token == "" || second.token == owner.token {
		t.Fatal("login gave no new session")
	}
	// The cookie works on its own, as it does for the mockup.
	req, _ := http.NewRequest("GET", srv.URL+"/api/me", nil)
	req.AddCookie(&http.Cookie{Name: "kerdos_session", Value: second.token})
	if res, err := http.DefaultClient.Do(req); err != nil || res.StatusCode != 200 {
		t.Fatalf("cookie session: %v %v", res, err)
	}
	evil := *owner
	evil.origin = "http://evil.example"
	if code := errCode(t, &evil, "POST", "/api/chat", map[string]string{"text": "hi"}, 403); code != "bad_origin" {
		t.Errorf("cross-site POST: %s", code)
	}
	same := *owner
	same.origin = srv.URL
	same.call("POST", "/api/chat", map[string]string{"text": "Who owes me?"}, 200, nil)
	for i := 0; i < 8; i++ {
		errCode(t, newBrowser(), "POST", "/api/auth/login", map[string]string{"email": "target@example.com", "password": "guess-" + string(rune('a'+i))}, 401)
	}
	if code := errCode(t, newBrowser(), "POST", "/api/auth/login", map[string]string{"email": "target@example.com", "password": "guess-z"}, 429); code != "too_many_attempts" {
		t.Errorf("rate limit: %s", code)
	}

	// ---- The owner invites Maria. The link is emailed and returned, but hidden in the email log.
	var inv struct {
		Invite app.Invite `json:"invite"`
		Link   string     `json:"link"`
	}
	owner.call("POST", "/api/team/invites", map[string]string{"email": "maria@example.com", "name": "Maria"}, 201, &inv)
	token := strings.TrimPrefix(inv.Link, "http://front/signup?invite=")
	if !auth.ValidToken(token) || inv.Invite.EmailStatus != "sent" || inv.Invite.Email != "maria@example.com" {
		t.Fatalf("invite: %+v %s", inv.Invite, inv.Link)
	}
	if got := mailer.to("maria@example.com"); len(got) != 1 || !strings.Contains(got[0].Body, inv.Link) || !strings.Contains(got[0].Body, "Anna Georgiou invited you") {
		t.Fatalf("invite email: %+v", got)
	}
	var log struct{ Messages []app.Message }
	owner.call("GET", "/api/messages?kind=invite", nil, 200, &log)
	if len(log.Messages) != 1 || strings.Contains(log.Messages[0].Body, token) || !strings.Contains(log.Messages[0].Body, "invite=…") {
		t.Fatalf("email log shows the invite token: %+v", log.Messages)
	}
	var info app.InviteInfo
	anon.call("GET", "/api/auth/invites/"+token, nil, 200, &info)
	if info.Email != "maria@example.com" || info.InvitedBy != "Anna Georgiou" || info.Company != "Nicosia Studio" {
		t.Errorf("invite info %+v", info)
	}
	maria := newBrowser()
	if code := errCode(t, maria, "POST", "/api/auth/signup", map[string]string{"name": "Maria", "email": "eve@example.com", "password": "maria-secret-1", "invite": token}, 403); code != "invite_email_mismatch" {
		t.Errorf("invite for another address: %s", code)
	}
	maria.call("POST", "/api/auth/signup", map[string]string{"name": "Maria Ioannou", "email": "maria@example.com", "password": "maria-secret-1", "invite": token}, 201, &signed)
	if signed.User.Role != "user" {
		t.Errorf("member role %q", signed.User.Role)
	}
	mariaID := signed.User.ID
	if code := errCode(t, newBrowser(), "POST", "/api/auth/signup", map[string]string{"name": "X", "email": "maria@example.com", "password": "maria-secret-1", "invite": token}, 404); code != "invite_invalid" {
		t.Errorf("invite used twice: %s", code)
	}
	if code := errCode(t, maria, "GET", "/api/team", nil, 403); code != "owner_only" {
		t.Errorf("member on team page: %s", code)
	}
	if code := errCode(t, maria, "POST", "/api/demo/reset", nil, 403); code != "owner_only" {
		t.Errorf("member reset: %s", code)
	}
	owner.call("POST", "/api/demo/reset", nil, 200, nil)
	maria.call("GET", "/api/me", nil, 200, nil) // accounts and sessions survive a reset

	// ---- The account page.
	var acc app.Account
	maria.call("PATCH", "/api/me", map[string]string{"name": "Maria I."}, 200, &acc)
	if acc.Name != "Maria I." {
		t.Errorf("name %q", acc.Name)
	}
	if code := errCode(t, maria, "PATCH", "/api/me", map[string]string{"email": "maria.i@example.com"}, 400); code != "current_password_needed" {
		t.Errorf("email change without password: %s", code)
	}
	if code := errCode(t, maria, "PATCH", "/api/me", map[string]string{"email": "maria.i@example.com", "current_password": "nope-nope"}, 403); code != "wrong_password" {
		t.Errorf("email change, wrong password: %s", code)
	}
	if code := errCode(t, maria, "PATCH", "/api/me", map[string]string{"email": "anna@example.com", "current_password": "maria-secret-1"}, 409); code != "email_taken" {
		t.Errorf("email of another account: %s", code)
	}
	owner.call("POST", "/api/team/invites", map[string]string{"email": "zoe@example.com"}, 201, nil)
	if code := errCode(t, maria, "PATCH", "/api/me", map[string]string{"email": "Zoe@example.com", "current_password": "maria-secret-1"}, 409); code != "invited_address" {
		t.Errorf("taking an invited address: %s", code)
	}
	maria.call("PATCH", "/api/me", map[string]string{"email": "maria.i@example.com", "current_password": "maria-secret-1"}, 200, &acc)
	var openInvites struct{ Invites []app.Invite }
	owner.call("GET", "/api/team", nil, 200, &openInvites)
	for _, inv := range openInvites.Invites {
		owner.call("DELETE", "/api/team/invites/"+inv.ID, nil, 204, nil)
	}
	mariaPhone := newBrowser()
	mariaPhone.call("POST", "/api/auth/login", map[string]string{"email": "maria.i@example.com", "password": "maria-secret-1"}, 200, nil)
	if code := errCode(t, maria, "POST", "/api/me/password", map[string]string{"current_password": "wrong-one", "new_password": "maria-secret-2"}, 403); code != "wrong_password" {
		t.Errorf("password change, wrong current: %s", code)
	}
	maria.call("POST", "/api/me/password", map[string]string{"current_password": "maria-secret-1", "new_password": "maria-secret-2"}, 204, nil)
	maria.call("GET", "/api/me", nil, 200, nil) // this session stays signed in
	errCode(t, mariaPhone, "GET", "/api/me", nil, 401)
	errCode(t, newBrowser(), "POST", "/api/auth/login", map[string]string{"email": "maria.i@example.com", "password": "maria-secret-1"}, 401)
	mariaPhone.token = ""
	mariaPhone.call("POST", "/api/auth/login", map[string]string{"email": "maria.i@example.com", "password": "maria-secret-2"}, 200, nil)
	mariaPhone.call("POST", "/api/auth/logout", nil, 204, nil)
	errCode(t, mariaPhone, "GET", "/api/me", nil, 401)

	// ---- Google.
	startFor := func(b *browser, returnTo, invite string) func() string {
		return func() string {
			var out struct{ URL string }
			b.call("POST", "/api/auth/google/start", map[string]string{"return_to": returnTo, "invite": invite}, 200, &out)
			return out.URL
		}
	}
	if code := errCode(t, anon, "POST", "/api/auth/google/start", map[string]string{"return_to": "http://evil.example/x"}, 400); code != "bad_return_to" {
		t.Errorf("return_to elsewhere: %s", code)
	}
	exchange := func(loc *url.URL) *browser {
		t.Helper()
		code := loc.Query().Get("login_code")
		if code == "" {
			t.Fatalf("no login code: %s", loc)
		}
		b := newBrowser()
		b.call("POST", "/api/auth/exchange", map[string]string{"code": code}, 200, nil)
		if code := errCode(t, newBrowser(), "POST", "/api/auth/exchange", map[string]string{"code": code}, 400); code != "login_code_invalid" {
			t.Errorf("login code used twice: %s", code)
		}
		return b
	}

	// The owner's Google account has the same address: it links and signs in to her account.
	fg.add("anna-code", "g-anna", "anna@example.com", "Anna G", true)
	loc := googleSignIn(t, srv.URL, startFor(anon, "http://front/auth/callback?next=/invoices", ""), "anna-code")
	if loc.Host != "front" || loc.Path != "/auth/callback" || loc.Query().Get("next") != "/invoices" {
		t.Fatalf("landed on %s", loc)
	}
	annaG := exchange(loc)
	annaG.call("GET", "/api/me", nil, 200, &acc)
	if acc.ID != seedOwner || !acc.GoogleLinked || acc.GoogleEmail != "anna@example.com" || acc.Name != "Anna Georgiou" {
		t.Errorf("owner after Google: %+v", acc)
	}
	// A stranger with Google and no invitation is turned away.
	fg.add("stranger", "g-x", "x@example.com", "X", true)
	loc = googleSignIn(t, srv.URL, startFor(anon, srv.URL+"/mockup/", ""), "stranger")
	if loc.Query().Get("auth_error") != "invite_required" || !strings.HasPrefix(loc.String(), srv.URL+"/mockup/") {
		t.Errorf("stranger: %s", loc)
	}
	// Unverified addresses are not trusted.
	fg.add("unverified", "g-u", "anna@example.com", "U", false)
	if loc = googleSignIn(t, srv.URL, startFor(anon, "http://front/x", ""), "unverified"); loc.Query().Get("auth_error") != "google_unverified" {
		t.Errorf("unverified: %s", loc)
	}
	if loc = googleSignIn(t, srv.URL, startFor(anon, "http://front/x", ""), "cancel"); loc.Query().Get("auth_error") != "google_cancelled" {
		t.Errorf("cancelled: %s", loc)
	}
	if loc = googleSignIn(t, srv.URL, func() string { return "http://google.test/auth?state=made-up" }, "anna-code"); loc.Query().Get("auth_error") != "google_state" || loc.Host != "front" {
		t.Errorf("unknown state: %s", loc)
	}
	// Bob is invited and uses Google straight away, without the link: his verified address is enough.
	owner.call("POST", "/api/team/invites", map[string]string{"email": "bob@example.com"}, 201, &inv)
	fg.add("bob-code", "g-bob", "bob@example.com", "Bob Pappas", true)
	bob := exchange(googleSignIn(t, srv.URL, startFor(anon, "http://front/auth/callback", ""), "bob-code"))
	bob.call("GET", "/api/me", nil, 200, &acc)
	if acc.Name != "Bob Pappas" || acc.Role != "user" || acc.HasPassword || !acc.GoogleLinked {
		t.Errorf("bob: %+v", acc)
	}
	if code := errCode(t, bob, "DELETE", "/api/me/google", nil, 409); code != "needs_password" {
		t.Errorf("unlink the only way in: %s", code)
	}
	bob.call("POST", "/api/me/password", map[string]string{"new_password": "bob-first-pw"}, 204, nil) // a first password needs no current one
	bob.call("DELETE", "/api/me/google", nil, 200, &acc)
	if acc.GoogleLinked {
		t.Error("still linked")
	}
	// Maria links her Google account from the account page; Bob can't take the same one.
	fg.add("maria-g", "g-maria", "maria.personal@gmail.com", "Maria", true)
	linkFor := func(b *browser) func() string {
		return func() string {
			var out struct{ URL string }
			b.call("POST", "/api/me/google/link", map[string]string{"return_to": "http://front/account"}, 200, &out)
			return out.URL
		}
	}
	if loc = googleSignIn(t, srv.URL, linkFor(maria), "maria-g"); loc.Query().Get("google") != "linked" || loc.Path != "/account" {
		t.Fatalf("link: %s", loc)
	}
	if loc = googleSignIn(t, srv.URL, linkFor(bob), "maria-g"); loc.Query().Get("auth_error") != "google_in_use" {
		t.Errorf("same Google account twice: %s", loc)
	}
	mariaG := exchange(googleSignIn(t, srv.URL, startFor(anon, "http://front/auth/callback", ""), "maria-g"))
	mariaG.call("GET", "/api/me", nil, 200, &acc)
	if acc.ID != mariaID || acc.GoogleEmail != "maria.personal@gmail.com" {
		t.Errorf("maria via Google: %+v", acc)
	}

	// ---- The team: invitations and removing a member.
	var team struct {
		Members []app.Member
		Invites []app.Invite
	}
	owner.call("POST", "/api/team/invites", map[string]string{"email": "carl@example.com"}, 201, &inv)
	if code := errCode(t, owner, "POST", "/api/team/invites", map[string]string{"email": "Maria.I@example.com"}, 409); code != "already_member" {
		t.Errorf("invite a member: %s", code)
	}
	mailer.setFail(true)
	owner.call("POST", "/api/team/invites", map[string]string{"email": "dina@example.com"}, 201, &inv)
	mailer.setFail(false)
	if inv.Invite.EmailStatus != "failed" || !strings.Contains(inv.Invite.EmailError, "refused the login") {
		t.Errorf("failed invite email: %+v", inv.Invite)
	}
	owner.call("DELETE", "/api/team/invites/"+inv.Invite.ID, nil, 204, nil)
	var retried app.Message
	owner.call("POST", "/api/messages/"+inv.Invite.EmailMessageID+"/retry", nil, 200, &retried)
	if retried.Status != "cancelled" || !strings.Contains(retried.Error, "invitation was cancelled") || len(mailer.to("dina@example.com")) != 0 {
		t.Errorf("email for a cancelled invitation: %+v", retried)
	}
	owner.call("GET", "/api/team", nil, 200, &team)
	if len(team.Members) != 3 || !team.Members[0].You || team.Members[0].Role != "admin" || len(team.Invites) != 1 || team.Invites[0].Email != "carl@example.com" {
		t.Fatalf("team: %+v", team)
	}
	if code := errCode(t, owner, "DELETE", "/api/team/members/"+seedOwner, nil, 409); code != "cant_remove_self" {
		t.Errorf("remove self: %s", code)
	}
	owner.call("DELETE", "/api/team/members/"+mariaID, nil, 204, nil)
	errCode(t, maria, "GET", "/api/me", nil, 401)
	if code := errCode(t, newBrowser(), "POST", "/api/auth/login", map[string]string{"email": "maria.i@example.com", "password": "maria-secret-2"}, 403); code != "account_removed" {
		t.Errorf("removed member signs in: %s", code)
	}
	if loc = googleSignIn(t, srv.URL, startFor(anon, "http://front/x", ""), "maria-g"); loc.Query().Get("auth_error") != "account_removed" {
		t.Errorf("removed member with Google: %s", loc)
	}
	// Invited back, she gets her old account (and history) again.
	owner.call("POST", "/api/team/invites", map[string]string{"email": "maria.i@example.com"}, 201, &inv)
	back := newBrowser()
	back.call("POST", "/api/auth/signup", map[string]string{"name": "Maria", "email": "maria.i@example.com", "password": "maria-secret-3",
		"invite": strings.TrimPrefix(inv.Link, "http://front/signup?invite=")}, 201, &signed)
	if signed.User.ID != mariaID {
		t.Errorf("came back as a new user %s, was %s", signed.User.ID, mariaID)
	}

	// ---- Signing out everywhere.
	annaG.call("POST", "/api/auth/logout-everywhere", nil, 204, nil)
	for _, b := range []*browser{owner, second, annaG} {
		errCode(t, b, "GET", "/api/me", nil, 401)
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`).MatchString(back.token) {
		t.Errorf("session token %q", back.token)
	}
}

// Without Google configured, the plain "Continue with Google" link must still never send the
// browser to another site.
func TestGoogleStartNeverLeavesTheSite(t *testing.T) {
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	a := &app.App{Log: quiet, PublicURL: "http://front", ReturnOrigins: []string{"http://front"}}
	srv := httptest.NewServer((&httpapi.Server{App: a, Log: quiet, AuthMode: "accounts", PublicURL: "http://front"}).Handler())
	defer srv.Close()
	noFollow := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := noFollow.Get(srv.URL + "/api/auth/google/start?return_to=" + url.QueryEscape("https://evil.example/"))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusBadRequest || res.Header.Get("Location") != "" {
		t.Fatalf("status %d, location %q", res.StatusCode, res.Header.Get("Location"))
	}
	res, err = noFollow.Get(srv.URL + "/api/auth/google/start?return_to=" + url.QueryEscape("http://front/signin"))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if loc := res.Header.Get("Location"); res.StatusCode != http.StatusFound || !strings.HasPrefix(loc, "http://front/signin?") || !strings.Contains(loc, "auth_error=google_off") {
		t.Fatalf("status %d, location %q", res.StatusCode, loc)
	}
}
