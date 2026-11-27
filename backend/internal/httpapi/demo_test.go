package httpapi_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/RostKostia/SKA/backend/internal/ai"
	"github.com/RostKostia/SKA/backend/internal/app"
	"github.com/RostKostia/SKA/backend/internal/httpapi"
	"github.com/RostKostia/SKA/backend/internal/jinius"
	"github.com/RostKostia/SKA/backend/internal/mail"
)

type captureMailer struct {
	mu   sync.Mutex
	sent []mail.Message
}

func (c *captureMailer) Provider() string { return "mock" }
func (c *captureMailer) Send(_ context.Context, m mail.Message) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sent = append(c.sent, m)
	return fmt.Sprintf("test-%d", len(c.sent)), nil
}
func (c *captureMailer) to(addr string) []mail.Message {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []mail.Message
	for _, m := range c.sent {
		if m.To == addr {
			out = append(out, m)
		}
	}
	return out
}

type api struct {
	t   *testing.T
	url string
}

func (c api) do(method, path string, body any, want int, out any) {
	c.t.Helper()
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, c.url+path, r)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != want {
		c.t.Fatalf("%s %s: status %d, want %d: %s", method, path, resp.StatusCode, want, raw)
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			c.t.Fatalf("%s %s: decode: %v: %s", method, path, err, raw)
		}
	}
}

type chatReply struct {
	Messages []struct {
		Role    string          `json:"role"`
		Text    string          `json:"text"`
		Card    json.RawMessage `json:"card"`
		Invoice *app.Invoice    `json:"invoice"`
	} `json:"messages"`
}

func (r chatReply) last() (string, map[string]any, *app.Invoice) {
	m := r.Messages[len(r.Messages)-1]
	var card map[string]any
	_ = json.Unmarshal(m.Card, &card)
	return m.Text, card, m.Invoice
}

// TestDemoStory walks the 2-minute demo through the HTTP API. It needs a migrated database
// that it may reset: KERDOS_TEST_DATABASE_URL=postgres://kerdos:…@127.0.0.1:5432/kerdos_test
func TestDemoStory(t *testing.T) {
	dsn := os.Getenv("KERDOS_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set KERDOS_TEST_DATABASE_URL to a migrated database this test may reset")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	mailer := &captureMailer{}
	a := &app.App{DB: db, Log: quiet, Parser: ai.Rules{}, Deliverer: jinius.Mock{PublicURL: "http://front"}, Mailer: mailer,
		PublicURL: "http://front", Demo: true, SeedFile: "../../../db/seed/sample.sql", FeeBP: 150}
	srv := httptest.NewServer((&httpapi.Server{App: a, Log: quiet, AuthMode: "dev", PublicURL: "http://front"}).Handler())
	defer srv.Close()
	c := api{t, srv.URL}
	ctx := context.Background()

	c.do("POST", "/api/demo/reset", nil, 200, nil)

	// ---- Day 0: the warning is already there.
	var f app.Forecast
	c.do("GET", "/api/forecast", nil, 200, &f)
	if f.StartBalanceCents != 100000 || f.Shortfall == nil || f.Shortfall.LowestDay != 12 || f.Shortfall.ShortfallCents != 80000 {
		t.Fatalf("day 0 forecast: %+v", f.Shortfall)
	}
	if f.Headline != "In 12 days you'll be €800 negative." {
		t.Errorf("headline %q", f.Headline)
	}

	// ---- "Invoice Kostas €120": sent at once, with a payment link and an email.
	var r chatReply
	c.do("POST", "/api/chat", map[string]string{"text": "Invoice Kostas €120"}, 200, &r)
	text, card, inv := r.last()
	if card["type"] != "invoice" || inv == nil {
		t.Fatalf("chat reply: %q %v", text, card)
	}
	wantNumber := fmt.Sprintf("KRD-%d-0006", f.Today.Year())
	if inv.Number != wantNumber || inv.Status != "sent" || inv.AmountCents != 12000 || inv.CounterpartyName != "Kostas Hadjis (sample)" ||
		inv.PaymentLinkURL == nil || !strings.HasPrefix(*inv.PaymentLinkURL, "http://front/pay/") || inv.Source != "chat" {
		t.Fatalf("invoice: %+v", inv)
	}
	if !strings.Contains(text, wantNumber) || !strings.Contains(text, "€120") {
		t.Errorf("reply text %q", text)
	}
	if _, err := a.SendQueued(ctx); err != nil {
		t.Fatal(err)
	}
	if got := mailer.to("kostas@example.com"); len(got) != 1 || !strings.Contains(got[0].Subject, wantNumber) || !strings.Contains(got[0].Body, *inv.PaymentLinkURL) {
		t.Fatalf("invoice email: %+v", got)
	}
	kostasID, kostasToken := inv.ID, strings.TrimPrefix(*inv.PaymentLinkURL, "http://front/pay/")
	c.do("GET", "/api/forecast", nil, 200, &f)
	if f.Shortfall.LowestDay != 12 || f.Shortfall.ShortfallCents != 80000 {
		t.Errorf("after Kostas: %+v", f.Shortfall)
	}

	// The same command without an amount asks instead of guessing.
	c.do("POST", "/api/chat", map[string]string{"text": "Invoice Kostas"}, 200, &r)
	if text, _, _ := r.last(); !strings.Contains(text, "How much") {
		t.Errorf("missing amount reply %q", text)
	}

	// ---- Skip time +3 days: deposit arrives, Andreas gets reminder 2 (firm), nobody else.
	var skip app.SkipResult
	c.do("POST", "/api/demo/skip", map[string]int{"days": 3}, 200, &skip)
	var reminders []app.QueuedReminder
	var arrived int64
	for _, rep := range skip.Reports {
		reminders = append(reminders, rep.Reminders...)
		for _, m := range rep.Arrivals {
			arrived += m.AmountCents
		}
	}
	if len(reminders) != 1 || reminders[0].Client != "Andreas Electric (sample)" || reminders[0].Tone != "firm" || reminders[0].Sequence != 2 {
		t.Fatalf("reminders during skip: %+v", reminders)
	}
	if arrived != 30000 || skip.Forecast.StartBalanceCents != 130000 {
		t.Errorf("arrivals %d, balance %d", arrived, skip.Forecast.StartBalanceCents)
	}
	if skip.Forecast.Headline != "In 9 days you'll be €800 negative." {
		t.Errorf("headline after skip %q", skip.Forecast.Headline)
	}
	if got := mailer.to("andreas@example.com"); len(got) != 1 || !strings.Contains(got[0].Subject, "overdue") {
		t.Errorf("Andreas's email: %+v", got)
	}
	lastMsg := skip.Messages[len(skip.Messages)-1]
	if !strings.Contains(lastMsg.Text, "In 9 days you'll be €800 negative") || !strings.Contains(lastMsg.Text, "€950") {
		t.Errorf("warning message %q", lastMsg.Text)
	}

	// A manual reminder for Elena (not due yet: a friendly nudge), then a second one is refused.
	var debtors struct{ Debtors []app.Debtor }
	c.do("GET", "/api/debtors", nil, 200, &debtors)
	var elenaInv string
	for _, d := range debtors.Debtors {
		if strings.HasPrefix(d.Name, "Elena") {
			elenaInv = d.OldestInvoiceID
		}
		if strings.HasPrefix(d.Name, "Andreas") && (d.LastTone == nil || *d.LastTone != "firm" || d.RemindersSent != 2) {
			t.Errorf("Andreas debtor row: %+v", d)
		}
	}
	var m app.Message
	c.do("POST", "/api/invoices/"+elenaInv+"/remind", nil, 201, &m)
	if m.Kind != "upcoming" || m.Mode != "manual" {
		t.Errorf("Elena's manual reminder: %+v", m)
	}
	c.do("POST", "/api/invoices/"+elenaInv+"/remind", nil, 409, nil)

	// ---- "Get €800 now": offer backed by Kostas, Elena and Andreas; accept once.
	var offer app.Offer
	c.do("POST", "/api/financing/offers", nil, 201, &offer)
	var backing int64
	for _, oi := range offer.Invoices {
		backing += oi.OutstandingCents
	}
	if offer.AmountCents != 80000 || offer.Product != "invoice_financing" || len(offer.Invoices) != 3 || backing != 95000 || offer.FeeCents != 1200 {
		t.Fatalf("offer: %+v", offer)
	}
	var accepted struct {
		Offer    app.Offer    `json:"offer"`
		Forecast app.Forecast `json:"forecast"`
	}
	c.do("POST", "/api/financing/offers/"+offer.ID+"/accept", nil, 200, &accepted)
	if accepted.Offer.Status != "disbursed" || accepted.Forecast.Shortfall != nil || accepted.Forecast.StartBalanceCents != 210000 {
		t.Fatalf("after accept: %+v %+v", accepted.Offer, accepted.Forecast.Shortfall)
	}
	c.do("POST", "/api/financing/offers/"+offer.ID+"/accept", nil, 409, nil)
	c.do("POST", "/api/invoices/"+kostasID+"/remind", nil, 409, nil) // financed: never chased
	c.do("GET", "/api/debtors", nil, 200, &debtors)
	if len(debtors.Debtors) != 0 {
		t.Errorf("debtors after financing: %+v", debtors.Debtors)
	}
	c.do("POST", "/api/financing/offers", nil, 409, nil) // no shortfall any more

	// ---- The second phone: Kostas opens the pay page and pays.
	var pub app.PublicInvoice
	c.do("GET", "/api/pay/"+kostasToken, nil, 200, &pub)
	if !pub.CanPay || pub.AmountCents != 12000 || pub.CompanyName != "Nicosia Studio (sample)" || pub.Status != "unpaid" {
		t.Fatalf("pay page: %+v", pub)
	}
	c.do("POST", "/api/pay/"+kostasToken, nil, 200, &pub)
	if pub.CanPay || pub.Status != "paid" {
		t.Fatalf("after paying: %+v", pub)
	}
	c.do("POST", "/api/pay/"+kostasToken, nil, 409, nil)
	c.do("GET", "/api/pay/not-a-token", nil, 404, nil)
	var detail app.InvoiceDetail
	c.do("GET", "/api/invoices/"+kostasID, nil, 200, &detail)
	if detail.Status != "paid" || len(detail.Payments) != 1 || detail.Payments[0].Method != "pay_by_bank" {
		t.Errorf("Kostas's invoice after paying: %+v", detail.Invoice)
	}

	// ---- A new client: Kerdos asks for the email, then sends.
	c.do("POST", "/api/chat", map[string]string{"text": "Invoice Nikos 2 hours at €30"}, 200, &r)
	if text, card, _ := r.last(); card["type"] != "need_details" || !strings.Contains(text, "email") {
		t.Fatalf("new client reply: %q %v", text, card)
	}
	c.do("POST", "/api/chat", map[string]string{"text": "nikos@example.com"}, 200, &r)
	_, card, inv = r.last()
	if card["type"] != "invoice" || inv == nil || inv.AmountCents != 6000 || inv.CounterpartyName != "Nikos" || inv.Status != "sent" {
		t.Fatalf("after giving the email: %v %+v", card, inv)
	}
	c.do("POST", "/api/chat", map[string]string{"text": "Who owes me?"}, 200, &r)
	if text, _, _ := r.last(); !strings.Contains(text, "Nikos owes €60") {
		t.Errorf("debtors reply %q", text)
	}

	// ---- Odds and ends.
	var dash app.Dashboard
	c.do("GET", "/api/dashboard", nil, 200, &dash)
	if dash.ClockOffsetDays != 3 || dash.OwedCents != 6000 {
		t.Errorf("dashboard: offset %d owed %d", dash.ClockOffsetDays, dash.OwedCents)
	}
	var chat struct{ Messages []app.ChatMessage }
	c.do("GET", "/api/chat", nil, 200, &chat)
	if len(chat.Messages) < 15 {
		t.Errorf("chat history has %d messages", len(chat.Messages))
	}
	c.do("GET", "/api/invoices/not-a-uuid", nil, 404, nil)
	c.do("POST", "/api/chat", map[string]string{"text": "   "}, 400, nil)
	c.do("GET", "/api/reminders/settings", nil, 200, nil)
	c.do("POST", "/api/bank/connect", nil, 409, nil) // mock bank
	c.do("GET", "/api/nope", nil, 404, nil)
}
