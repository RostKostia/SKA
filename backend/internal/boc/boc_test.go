package boc

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// Response bodies below are the samples from the BoC "APIC Portals Documentation" (Sep 2024).
const (
	docAccount = `{"bankId":"12345671","accountId":"351012345671","accountAlias":"ANDREAS","accountType":"CURRENT",
		"accountName":"ANDREAS MICHAEL","IBAN":"CY11002003510000000012345671","currency":"EUR","infoTimeStamp":"1511779237",
		"interestRate":0,"maturityDate":"19/11/2018","lastPaymentDate":"19/11/2017","nextPaymentDate":"19/12/2017","remainingInstallments":10`
	docStatement = `{"account":{"bankId":"12345671","accountId":"351012345671"},"transaction":[
		{"id":"663c9d26de9162079842ce59","dcInd":"DEBIT","transactionAmount":{"amount":30,"currency":"EUR"},
		 "description":"SWIFT Transfer","postingDate":"09/05/2024","valueDate":"09/05/2024","transactionType":"PAYMENT"},
		{"id":"664307a7de9162079842cef8","transactionAmount":{"amount":10,"currency":"EUR"},"postingDate":"14/05/2024","valueDate":"14/05/2024"},
		{"id":"6644a7c9d983f403982a0b1e","dcInd":"CREDIT","transactionAmount":{"amount":120.5,"currency":"EUR"},
		 "description":"KRD-2026-0006 Nicosia Studio","postingDate":"15/05/2024","valueDate":"15/05/2024"}]}`
	docSubscriptionPatched = `{"subscriptionId":"Subid000001-1725429256148","status":"ACTV","description":"SUBSCRIPTION",
		"selectedAccounts":[{"accountId":"351012345671"},{"accountId":"351092345672"}],
		"accounts":{"transactionHistory":true,"balance":true,"details":true,"checkFundsAvailability":true},
		"payments":{"limit":50,"currency":"string","amount":50},"duration":{"startDate":"04/09/2024","endDate":"03/12/2024"}}`
)

func fakeBoC(t *testing.T) (*Client, *int) {
	t.Helper()
	tokenCalls := 0
	mux := http.NewServeMux()
	mux.HandleFunc("POST /oauth2/token", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if r.Form.Get("client_id") != "cid" || r.Form.Get("client_secret") != "secret" {
			t.Errorf("token: bad client credentials %v", r.Form)
		}
		switch r.Form.Get("grant_type") {
		case "client_credentials":
			tokenCalls++
			if r.Form.Get("scope") != "TPPOAuth2Security" {
				t.Errorf("app token scope %q", r.Form.Get("scope"))
			}
			io.WriteString(w, `{"token_type":"bearer","access_token":"app-token","expires_in":3600,"consented_on":1542264419,"scope":"TPPOAuth2Security"}`)
		case "authorization_code":
			if r.Form.Get("code") != "AAIB7CW9gOFNa6c3B5l9GsfSHT" || r.Form.Get("scope") != "UserOAuth2Security" {
				t.Errorf("code exchange %v", r.Form)
			}
			io.WriteString(w, `{"token_type":"bearer","access_token":"user-token","expires_in":3600,"scope":"UserOAuth2Security"}`)
		}
	})
	check := func(r *http.Request, token string, wantSub bool) {
		if r.Header.Get("Authorization") != "Bearer "+token || r.Header.Get("journeyId") == "" || r.Header.Get("timeStamp") == "" {
			t.Errorf("%s %s: headers %v", r.Method, r.URL.Path, r.Header)
		}
		if wantSub && r.Header.Get("subscriptionId") != "Subid000001-1725429256148" {
			t.Errorf("%s: missing subscriptionId header", r.URL.Path)
		}
	}
	mux.HandleFunc("POST /v1/subscriptions", func(w http.ResponseWriter, r *http.Request) {
		check(r, "app-token", false)
		var p Permissions
		json.NewDecoder(r.Body).Decode(&p)
		if !p.Accounts.Balance || !p.Accounts.TransactionHistory {
			t.Errorf("subscription body %+v", p)
		}
		io.WriteString(w, `{"duration":{"startDate":"20/11/2017","endDate":"20/11/2018"},"subscriptionId":"Subid000001-1725429256148","status":"PENDING","selectedAccounts":[],
			"accounts":{"transactionHistory":true,"balance":true,"details":true,"checkFundsAvailability":false}}`)
	})
	mux.HandleFunc("GET /v1/subscriptions/{id}", func(w http.ResponseWriter, r *http.Request) {
		check(r, "user-token", false)
		io.WriteString(w, `{"SubscriptionId":"Subid000001-1725429256148","status":"PENDING","accounts":{"transactionHistory":true,"balance":true,"details":true,"checkFundsAvailability":true},"payments":{"limit":50,"currency":"string","amount":50},"expirationDate":"03/12/2024"}`)
	})
	mux.HandleFunc("PATCH /v1/subscriptions/{id}", func(w http.ResponseWriter, r *http.Request) {
		check(r, "user-token", false)
		var p Permissions
		json.NewDecoder(r.Body).Decode(&p)
		if !p.Accounts.CheckFundsAvailability || p.Payments == nil {
			t.Errorf("PATCH must send back the approved details, got %+v", p)
		}
		io.WriteString(w, docSubscriptionPatched)
	})
	mux.HandleFunc("GET /v1/accounts", func(w http.ResponseWriter, r *http.Request) {
		check(r, "app-token", true)
		io.WriteString(w, "["+docAccount+`,"balances":[]}]`)
	})
	mux.HandleFunc("GET /v1/accounts/{id}/balance", func(w http.ResponseWriter, r *http.Request) {
		check(r, "app-token", true)
		io.WriteString(w, "["+docAccount+`,"balances":[{"amount":1000,"balanceType":"AVAILABLE"},{"amount":1000.25,"balanceType":"CURRENT"}]}]`)
	})
	mux.HandleFunc("GET /v1/accounts/{id}/statement", func(w http.ResponseWriter, r *http.Request) {
		check(r, "app-token", true)
		q := r.URL.Query()
		if q.Get("startDate") != "16/04/2024" || q.Get("endDate") != "16/05/2024" || q.Get("maxCount") != "10" {
			t.Errorf("statement query %v", q)
		}
		io.WriteString(w, docStatement)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return &Client{BaseURL: srv.URL, ClientID: "cid", ClientSecret: "secret", RedirectURI: "http://localhost:8080/api/bank/callback", HTTP: srv.Client()}, &tokenCalls
}

func TestConsentFlow(t *testing.T) {
	c, _ := fakeBoC(t)
	ctx := context.Background()
	sub, err := c.CreateSubscription(ctx, Permissions{Accounts: AccountPermissions{TransactionHistory: true, Balance: true, Details: true}})
	if err != nil || sub.SubscriptionID != "Subid000001-1725429256148" || sub.Status != "PENDING" {
		t.Fatalf("create: %+v %v", sub, err)
	}
	u, _ := url.Parse(c.AuthorizeURL(sub.SubscriptionID))
	q := u.Query()
	if !strings.HasSuffix(u.Path, "/oauth2/authorize") || q.Get("subscriptionid") != sub.SubscriptionID || q.Get("scope") != "UserOAuth2Security" || q.Get("response_type") != "code" || q.Get("redirect_uri") == "" {
		t.Errorf("authorize URL %s", u)
	}
	userTok, err := c.ExchangeCode(ctx, "AAIB7CW9gOFNa6c3B5l9GsfSHT")
	if err != nil || userTok != "user-token" {
		t.Fatalf("exchange: %q %v", userTok, err)
	}
	act, err := c.Activate(ctx, userTok, sub.SubscriptionID)
	if err != nil || act.Status != "ACTV" || len(act.SelectedAccounts) != 2 || act.SelectedAccounts[0].AccountID != "351012345671" {
		t.Fatalf("activate: %+v %v", act, err)
	}
	if act.ExpiresOn == nil || act.ExpiresOn.Format("2006-01-02") != "2024-12-03" || act.StartsOn.Format("2006-01-02") != "2024-09-04" {
		t.Errorf("duration: %v %v", act.StartsOn, act.ExpiresOn)
	}
}

func TestAccountsBalanceStatement(t *testing.T) {
	c, tokenCalls := fakeBoC(t)
	ctx := context.Background()
	const sub = "Subid000001-1725429256148"
	accts, err := c.Accounts(ctx, sub)
	if err != nil || len(accts) != 1 || accts[0].IBAN != "CY11002003510000000012345671" || accts[0].Type != "CURRENT" {
		t.Fatalf("accounts: %+v %v", accts, err)
	}
	bal, err := c.Balance(ctx, sub, "351012345671")
	if err != nil || *bal.AvailableCt != 100000 || *bal.CurrentCt != 100025 {
		t.Fatalf("balance: %+v %v", bal, err)
	}
	from := time.Date(2024, 4, 16, 0, 0, 0, 0, time.UTC)
	txs, err := c.Statement(ctx, sub, "351012345671", from, from.AddDate(0, 1, 0), 10)
	if err != nil || len(txs) != 3 {
		t.Fatalf("statement: %+v %v", txs, err)
	}
	if txs[0].DCInd != "debit" || txs[0].AmountCents != -3000 || txs[0].PostingDate.Format("2006-01-02") != "2024-05-09" || txs[0].Type != "PAYMENT" {
		t.Errorf("debit row: %+v", txs[0])
	}
	if txs[1].DCInd != "" || txs[1].AmountCents != 1000 {
		t.Errorf("row without dcInd must keep a positive amount and no sign: %+v", txs[1])
	}
	if txs[2].DCInd != "credit" || txs[2].AmountCents != 12050 || txs[2].Description != "KRD-2026-0006 Nicosia Studio" {
		t.Errorf("credit row: %+v", txs[2])
	}
	if *tokenCalls != 1 {
		t.Errorf("app token should be cached, fetched %d times", *tokenCalls)
	}
}

func TestCents(t *testing.T) {
	for in, want := range map[string]int64{"30": 3000, "10.5": 1050, "1000.25": 100025, "0.005": 1, "-12.34": -1234} {
		if got, err := Cents(json.Number(in)); err != nil || got != want {
			t.Errorf("Cents(%s) = %d, %v; want %d", in, got, err, want)
		}
	}
}
