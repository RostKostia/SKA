package google

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func idToken(claims map[string]any) string {
	head := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	body, _ := json.Marshal(claims)
	return head + "." + base64.RawURLEncoding.EncodeToString(body) + ".c2ln"
}

func goodClaims() map[string]any {
	return map[string]any{"iss": "https://accounts.google.com", "aud": "cid", "sub": "1234567890",
		"email": "maria@example.com", "email_verified": true, "name": "Maria Ioannou", "exp": time.Now().Add(time.Hour).Unix()}
}

func TestAuthCodeURL(t *testing.T) {
	c := &Client{ClientID: "cid", RedirectURI: "http://localhost:8080/api/auth/google/callback"}
	u, err := url.Parse(c.AuthCodeURL("st", "ch", "maria@example.com"))
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if u.Host != "accounts.google.com" || q.Get("state") != "st" || q.Get("code_challenge") != "ch" ||
		q.Get("code_challenge_method") != "S256" || q.Get("scope") != "openid email profile" ||
		q.Get("login_hint") != "maria@example.com" || q.Get("redirect_uri") != c.RedirectURI {
		t.Fatalf("auth URL %s", u)
	}
}

func TestExchange(t *testing.T) {
	var claims map[string]any
	var form url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		form = r.PostForm
		if r.PostForm.Get("code") == "bad" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"Bad Request"}`))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"id_token": idToken(claims), "access_token": "x"})
	}))
	defer srv.Close()
	c := &Client{ClientID: "cid", ClientSecret: "sec", RedirectURI: "http://cb", TokenURL: srv.URL}
	ctx := context.Background()

	claims = goodClaims()
	id, err := c.Exchange(ctx, "code1", "verifier1")
	if err != nil {
		t.Fatal(err)
	}
	if id != (Identity{Sub: "1234567890", Email: "maria@example.com", EmailVerified: true, Name: "Maria Ioannou"}) {
		t.Errorf("identity %+v", id)
	}
	if form.Get("code_verifier") != "verifier1" || form.Get("client_secret") != "sec" || form.Get("redirect_uri") != "http://cb" ||
		form.Get("grant_type") != "authorization_code" {
		t.Errorf("form %v", form)
	}

	claims = goodClaims()
	claims["email_verified"] = "true"
	claims["aud"] = []string{"other", "cid"}
	claims["iss"] = "accounts.google.com"
	if id, err := c.Exchange(ctx, "c", "v"); err != nil || !id.EmailVerified {
		t.Errorf("string email_verified and list audience: %+v %v", id, err)
	}

	bad := map[string]func(map[string]any){
		"another app": func(m map[string]any) { m["aud"] = "someone-else" },
		"expired":     func(m map[string]any) { m["exp"] = time.Now().Add(-time.Hour).Unix() },
		"from":        func(m map[string]any) { m["iss"] = "https://evil.example" },
		"no subject":  func(m map[string]any) { delete(m, "sub") },
	}
	for want, change := range bad {
		claims = goodClaims()
		change(claims)
		if _, err := c.Exchange(ctx, "c", "v"); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: got %v", want, err)
		}
	}
	claims = goodClaims()
	claims["email_verified"] = false
	if id, err := c.Exchange(ctx, "c", "v"); err != nil || id.EmailVerified {
		t.Errorf("unverified: %+v %v", id, err)
	}
	if _, err := c.Exchange(ctx, "bad", "v"); err == nil || !strings.Contains(err.Error(), "invalid_grant") {
		t.Errorf("error response: %v", err)
	}
}
