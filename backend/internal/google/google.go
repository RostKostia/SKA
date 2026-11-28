// Package google is "Sign in with Google": OpenID Connect with the authorization code flow and
// PKCE. Kerdos only asks for the user's identity (openid, email, profile); Gmail and Sheets
// access will be asked for separately, when those features exist.
package google

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	DefaultAuthURL  = "https://accounts.google.com/o/oauth2/v2/auth"
	DefaultTokenURL = "https://oauth2.googleapis.com/token"
)

type Client struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string // must point at GET /api/auth/google/callback, registered exactly like that
	AuthURL      string // DefaultAuthURL unless testing
	TokenURL     string // DefaultTokenURL unless testing
	HTTP         *http.Client
}

// Identity is who Google says signed in.
type Identity struct {
	Sub           string // stable Google account id
	Email         string
	EmailVerified bool
	Name          string
}

// AuthCodeURL is the Google page the browser is sent to. loginHint pre-fills the address.
func (c *Client) AuthCodeURL(state, codeChallenge, loginHint string) string {
	q := url.Values{
		"client_id":             {c.ClientID},
		"redirect_uri":          {c.RedirectURI},
		"response_type":         {"code"},
		"scope":                 {"openid email profile"},
		"state":                 {state},
		"code_challenge":        {codeChallenge},
		"code_challenge_method": {"S256"},
		"prompt":                {"select_account"},
	}
	if loginHint != "" {
		q.Set("login_hint", loginHint)
	}
	return c.authURL() + "?" + q.Encode()
}

// Exchange swaps the code from the callback for the user's identity. The ID token comes
// straight from Google's token endpoint over TLS, so (as Google's OpenID Connect guide allows)
// its claims are checked but its signature is not.
func (c *Client) Exchange(ctx context.Context, code, verifier string) (Identity, error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {c.ClientID},
		"client_secret": {c.ClientSecret},
		"redirect_uri":  {c.RedirectURI},
		"code_verifier": {verifier},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenURL(), strings.NewReader(form.Encode()))
	if err != nil {
		return Identity{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	httpc := c.HTTP
	if httpc == nil {
		httpc = &http.Client{Timeout: 15 * time.Second}
	}
	res, err := httpc.Do(req)
	if err != nil {
		return Identity{}, fmt.Errorf("google: token request: %w", err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	var body struct {
		IDToken          string `json:"id_token"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	_ = json.Unmarshal(raw, &body)
	if res.StatusCode != http.StatusOK || body.IDToken == "" {
		if body.Error != "" {
			return Identity{}, fmt.Errorf("google: token request: %s %s", body.Error, body.ErrorDescription)
		}
		return Identity{}, fmt.Errorf("google: token request: HTTP %d", res.StatusCode)
	}
	return c.parseIDToken(body.IDToken, time.Now())
}

func (c *Client) parseIDToken(token string, now time.Time) (Identity, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Identity{}, errors.New("google: malformed ID token")
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
	if err != nil {
		return Identity{}, errors.New("google: malformed ID token")
	}
	var claims struct {
		Iss           string          `json:"iss"`
		Aud           json.RawMessage `json:"aud"`
		Sub           string          `json:"sub"`
		Email         string          `json:"email"`
		EmailVerified any             `json:"email_verified"` // a bool, or "true" in older tokens
		Name          string          `json:"name"`
		Exp           int64           `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return Identity{}, errors.New("google: malformed ID token")
	}
	if claims.Iss != "https://accounts.google.com" && claims.Iss != "accounts.google.com" {
		return Identity{}, fmt.Errorf("google: ID token from %q", claims.Iss)
	}
	if !audienceIs(claims.Aud, c.ClientID) {
		return Identity{}, errors.New("google: ID token is for another app")
	}
	if claims.Exp == 0 || now.Unix() > claims.Exp+60 {
		return Identity{}, errors.New("google: ID token expired")
	}
	if claims.Sub == "" {
		return Identity{}, errors.New("google: ID token has no subject")
	}
	verified := claims.EmailVerified == true || claims.EmailVerified == "true"
	return Identity{Sub: claims.Sub, Email: claims.Email, EmailVerified: verified, Name: claims.Name}, nil
}

func audienceIs(raw json.RawMessage, clientID string) bool {
	var one string
	if json.Unmarshal(raw, &one) == nil {
		return one == clientID
	}
	var many []string
	if json.Unmarshal(raw, &many) == nil {
		for _, a := range many {
			if a == clientID {
				return true
			}
		}
	}
	return false
}

func (c *Client) authURL() string {
	if c.AuthURL != "" {
		return c.AuthURL
	}
	return DefaultAuthURL
}

func (c *Client) tokenURL() string {
	if c.TokenURL != "" {
		return c.TokenURL
	}
	return DefaultTokenURL
}
