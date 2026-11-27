// Package boc is a client for the Bank of Cyprus PSD2 Accounts API, following the
// "BOC API Documentation" (APIC portals, Sep 2024):
//
//   - app token: POST /oauth2/token, client_credentials, scope TPPOAuth2Security (60 minutes)
//   - consent: POST /v1/subscriptions → redirect the owner to /oauth2/authorize?…&subscriptionid=…
//     → code → POST /oauth2/token, authorization_code, scope UserOAuth2Security
//     → GET then PATCH /v1/subscriptions/{id} → status ACTV with selectedAccounts
//   - data: GET /v1/accounts, /v1/accounts/{id}/balance, /v1/accounts/{id}/statement
//     with the app token plus the subscriptionId, journeyId and timestamp headers.
//
// BoC sends dates as dd/MM/yyyy and amounts as euro numbers; this package returns cents and
// time.Time. Which token the account calls take is read from the documentation's note
// ("pass the Token and the SubscriptionId"); confirm it against the sandbox.
package boc

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Client struct {
	BaseURL      string // e.g. https://sandbox-apis.bankofcyprus.com/df-boc-org-sb/sb/psd2
	ClientID     string
	ClientSecret string
	RedirectURI  string
	HTTP         *http.Client

	mu       sync.Mutex
	appToken string
	expires  time.Time
}

// AccountPermissions is the "accounts" object of a subscription request.
type AccountPermissions struct {
	TransactionHistory     bool `json:"transactionHistory"`
	Balance                bool `json:"balance"`
	Details                bool `json:"details"`
	CheckFundsAvailability bool `json:"checkFundsAvailability"`
}

type PaymentPermissions struct {
	Limit    float64 `json:"limit"`
	Currency string  `json:"currency"`
	Amount   float64 `json:"amount"`
}

type Permissions struct {
	Accounts AccountPermissions  `json:"accounts"`
	Payments *PaymentPermissions `json:"payments,omitempty"`
}

type Subscription struct {
	SubscriptionID   string              `json:"subscriptionId"`
	Status           string              `json:"status"` // PENDING or ACTV
	Accounts         AccountPermissions  `json:"accounts"`
	Payments         *PaymentPermissions `json:"payments,omitempty"`
	SelectedAccounts []struct {
		AccountID string `json:"accountId"`
	} `json:"selectedAccounts"`
	StartsOn  *time.Time `json:"-"`
	ExpiresOn *time.Time `json:"-"`
}

type Account struct {
	BankID      string `json:"bankId"`
	AccountID   string `json:"accountId"`
	Alias       string `json:"accountAlias"`
	Type        string `json:"accountType"`
	Name        string `json:"accountName"`
	IBAN        string `json:"IBAN"`
	Currency    string `json:"currency"`
	AvailableCt *int64 `json:"-"`
	CurrentCt   *int64 `json:"-"`
}

type Transaction struct {
	ID          string
	DCInd       string // "debit", "credit" or "" when BoC leaves dcInd out
	AmountCents int64  // signed: negative for debit; positive (unknown sign) when DCInd is ""
	Currency    string
	Description string
	Type        string
	PostingDate time.Time
	ValueDate   *time.Time
	Raw         json.RawMessage
}

// ---------------------------------------------------------------- tokens

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

func (c *Client) token(ctx context.Context, form url.Values) (tokenResponse, error) {
	form.Set("client_id", c.ClientID)
	form.Set("client_secret", c.ClientSecret)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/oauth2/token", strings.NewReader(form.Encode()))
	if err != nil {
		return tokenResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	var t tokenResponse
	if err := c.do(req, &t); err != nil {
		return t, err
	}
	if t.AccessToken == "" {
		return t, fmt.Errorf("boc: token response without access_token")
	}
	return t, nil
}

// AppToken returns a cached client-credentials token, renewed a minute before it expires.
func (c *Client) AppToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.appToken != "" && time.Now().Before(c.expires) {
		return c.appToken, nil
	}
	t, err := c.token(ctx, url.Values{"grant_type": {"client_credentials"}, "scope": {"TPPOAuth2Security"}})
	if err != nil {
		return "", err
	}
	life := time.Duration(t.ExpiresIn) * time.Second
	if life <= 0 {
		life = 5 * time.Minute
	}
	c.appToken, c.expires = t.AccessToken, time.Now().Add(life-time.Minute)
	return c.appToken, nil
}

// ExchangeCode turns the code from the 1Bank redirect into a user token.
func (c *Client) ExchangeCode(ctx context.Context, code string) (string, error) {
	t, err := c.token(ctx, url.Values{"grant_type": {"authorization_code"}, "code": {code}, "scope": {"UserOAuth2Security"}})
	return t.AccessToken, err
}

// ---------------------------------------------------------------- subscriptions

func (c *Client) CreateSubscription(ctx context.Context, p Permissions) (Subscription, error) {
	tok, err := c.AppToken(ctx)
	if err != nil {
		return Subscription{}, err
	}
	var s Subscription
	err = c.json(ctx, http.MethodPost, "/v1/subscriptions", tok, "", p, &s)
	return s, err
}

// AuthorizeURL is where the owner's browser goes to log into 1Bank and consent.
func (c *Client) AuthorizeURL(subscriptionID string) string {
	q := url.Values{
		"response_type":  {"code"},
		"redirect_uri":   {c.RedirectURI},
		"scope":          {"UserOAuth2Security"},
		"client_id":      {c.ClientID},
		"subscriptionid": {subscriptionID},
	}
	return c.BaseURL + "/oauth2/authorize?" + q.Encode()
}

// Activate completes consent: GET the subscription as the owner approved it, then PATCH it back.
func (c *Client) Activate(ctx context.Context, userToken, subscriptionID string) (Subscription, error) {
	var got Subscription
	if err := c.json(ctx, http.MethodGet, "/v1/subscriptions/"+url.PathEscape(subscriptionID), userToken, "", nil, &got); err != nil {
		return got, err
	}
	var raw struct {
		Subscription
		Duration struct {
			StartDate string `json:"startDate"`
			EndDate   string `json:"endDate"`
		} `json:"duration"`
		ExpirationDate string `json:"expirationDate"`
	}
	body := Permissions{Accounts: got.Accounts, Payments: got.Payments}
	if err := c.json(ctx, http.MethodPatch, "/v1/subscriptions/"+url.PathEscape(subscriptionID), userToken, "", body, &raw); err != nil {
		return raw.Subscription, err
	}
	s := raw.Subscription
	if d, err := ParseDate(raw.Duration.StartDate); err == nil {
		s.StartsOn = &d
	}
	end := raw.Duration.EndDate
	if end == "" {
		end = raw.ExpirationDate
	}
	if d, err := ParseDate(end); err == nil {
		s.ExpiresOn = &d
	}
	return s, nil
}

// ---------------------------------------------------------------- accounts

type rawBalance struct {
	Amount      json.Number `json:"amount"`
	BalanceType string      `json:"balanceType"`
}

type rawAccount struct {
	Account
	Balances []rawBalance `json:"balances"`
}

func (c *Client) Accounts(ctx context.Context, subscriptionID string) ([]Account, error) {
	tok, err := c.AppToken(ctx)
	if err != nil {
		return nil, err
	}
	var raw []rawAccount
	if err := c.json(ctx, http.MethodGet, "/v1/accounts", tok, subscriptionID, nil, &raw); err != nil {
		return nil, err
	}
	out := make([]Account, 0, len(raw))
	for _, a := range raw {
		out = append(out, withBalances(a))
	}
	return out, nil
}

// Balance returns the account with AvailableCt and CurrentCt filled in.
func (c *Client) Balance(ctx context.Context, subscriptionID, accountID string) (Account, error) {
	tok, err := c.AppToken(ctx)
	if err != nil {
		return Account{}, err
	}
	var raw []rawAccount
	if err := c.json(ctx, http.MethodGet, "/v1/accounts/"+url.PathEscape(accountID)+"/balance", tok, subscriptionID, nil, &raw); err != nil {
		return Account{}, err
	}
	if len(raw) == 0 {
		return Account{}, fmt.Errorf("boc: no balance for account %s", accountID)
	}
	a := withBalances(raw[0])
	if a.AvailableCt == nil {
		return a, fmt.Errorf("boc: no AVAILABLE balance for account %s", accountID)
	}
	return a, nil
}

func withBalances(a rawAccount) Account {
	acc := a.Account
	for _, b := range a.Balances {
		cents, err := Cents(b.Amount)
		if err != nil {
			continue
		}
		switch strings.ToUpper(b.BalanceType) {
		case "AVAILABLE":
			acc.AvailableCt = &cents
		case "CURRENT":
			acc.CurrentCt = &cents
		}
	}
	return acc
}

// Statement returns the account's transactions between from and to (inclusive), at most max.
func (c *Client) Statement(ctx context.Context, subscriptionID, accountID string, from, to time.Time, max int) ([]Transaction, error) {
	tok, err := c.AppToken(ctx)
	if err != nil {
		return nil, err
	}
	q := url.Values{"startDate": {FormatDate(from)}, "endDate": {FormatDate(to)}, "maxCount": {strconv.Itoa(max)}}
	var raw struct {
		Transaction []json.RawMessage `json:"transaction"`
	}
	if err := c.json(ctx, http.MethodGet, "/v1/accounts/"+url.PathEscape(accountID)+"/statement?"+q.Encode(), tok, subscriptionID, nil, &raw); err != nil {
		return nil, err
	}
	out := make([]Transaction, 0, len(raw.Transaction))
	for _, r := range raw.Transaction {
		var t struct {
			ID     string `json:"id"`
			DCInd  string `json:"dcInd"`
			Amount struct {
				Amount   json.Number `json:"amount"`
				Currency string      `json:"currency"`
			} `json:"transactionAmount"`
			Description string `json:"description"`
			PostingDate string `json:"postingDate"`
			ValueDate   string `json:"valueDate"`
			Type        string `json:"transactionType"`
		}
		if err := json.Unmarshal(r, &t); err != nil {
			return nil, err
		}
		posted, err := ParseDate(t.PostingDate)
		if err != nil || t.ID == "" {
			return nil, fmt.Errorf("boc: bad transaction %s", string(r))
		}
		cents, err := Cents(t.Amount.Amount)
		if err != nil {
			return nil, err
		}
		if cents < 0 {
			cents = -cents
		}
		tx := Transaction{ID: t.ID, Currency: t.Amount.Currency, Description: t.Description, Type: t.Type, PostingDate: posted, Raw: r}
		switch strings.ToUpper(t.DCInd) {
		case "DEBIT":
			tx.DCInd, tx.AmountCents = "debit", -cents
		case "CREDIT":
			tx.DCInd, tx.AmountCents = "credit", cents
		default:
			tx.AmountCents = cents // sign unknown: callers must not match or classify it
		}
		if tx.Currency == "" {
			tx.Currency = "EUR"
		}
		if v, err := ParseDate(t.ValueDate); err == nil {
			tx.ValueDate = &v
		}
		out = append(out, tx)
	}
	return out, nil
}

// ---------------------------------------------------------------- helpers

func (c *Client) json(ctx context.Context, method, path, token, subscriptionID string, in, out any) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = strings.NewReader(string(b))
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("journeyId", journeyID())
	req.Header.Set("timeStamp", strconv.FormatInt(time.Now().Unix(), 10))
	if subscriptionID != "" {
		req.Header.Set("subscriptionId", subscriptionID)
	}
	return c.do(req, out)
}

func (c *Client) do(req *http.Request, out any) error {
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("boc: %s %s: %s: %.300s", req.Method, req.URL.Path, resp.Status, raw)
	}
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	return dec.Decode(out)
}

func journeyID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6], b[8] = b[6]&0x0f|0x40, b[8]&0x3f|0x80
	h := hex.EncodeToString(b[:])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
}

// ParseDate reads BoC's dd/MM/yyyy.
func ParseDate(s string) (time.Time, error) { return time.Parse("02/01/2006", strings.TrimSpace(s)) }

// FormatDate writes BoC's dd/MM/yyyy.
func FormatDate(t time.Time) string { return t.Format("02/01/2006") }

// Cents converts a BoC euro amount (a JSON number such as 30, 10.5 or "1000.25") to cents exactly.
func Cents(n json.Number) (int64, error) {
	s := strings.TrimSpace(n.String())
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	whole, frac, _ := strings.Cut(s, ".")
	w, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("boc: bad amount %q", n)
	}
	frac = (frac + "000")[:3]
	f, err := strconv.ParseInt(frac, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("boc: bad amount %q", n)
	}
	c := w*100 + (f+5)/10
	if neg {
		c = -c
	}
	return c, nil
}
