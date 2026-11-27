package ai

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/RostKostia/SKA/backend/internal/parse"
)

func fakeAnthropic(t *testing.T, status int, answer string) *Claude {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" || r.Header.Get("x-api-key") != "test-key" || r.Header.Get("anthropic-version") == "" {
			t.Errorf("unexpected request %s %v", r.URL.Path, r.Header)
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["model"] != "claude-haiku-5-5" || body["tool_choice"] == nil {
			t.Errorf("unexpected body %v", body)
		}
		w.WriteHeader(status)
		io.WriteString(w, answer)
	}))
	t.Cleanup(srv.Close)
	return &Claude{APIKey: "test-key", Model: "claude-haiku-5-5", BaseURL: srv.URL, HTTP: srv.Client(), Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

func TestClaudeToolAnswer(t *testing.T) {
	c := fakeAnthropic(t, 200, `{"content":[{"type":"tool_use","id":"t1","name":"record_intent","input":
		{"intent":"create_invoice","client":"Kostas","lines":[{"description":"Consulting","quantity":3,"unit":"hour","unit_price_cents":4000}]}}]}`)
	r := c.Parse(context.Background(), "invoice kostas for three hours of consulting at forty")
	if r.Source != "claude" || r.Intent != parse.CreateInvoice || r.Draft.Client != "Kostas" || r.Draft.TotalCents() != 12000 || r.Draft.Lines[0].Unit != "hour" {
		t.Fatalf("got %+v", r)
	}
}

func TestClaudeFallsBackToRules(t *testing.T) {
	for _, answer := range []struct {
		status int
		body   string
	}{
		{500, `{"error":"boom"}`},
		{200, `{"content":[{"type":"text","text":"hi"}]}`},
		{200, `{"content":[{"type":"tool_use","name":"record_intent","input":{"intent":"create_invoice","client":"Kostas","lines":[{"description":"x","quantity":-1,"unit":"hour","unit_price_cents":4000}]}}]}`},
	} {
		c := fakeAnthropic(t, answer.status, answer.body)
		r := c.Parse(context.Background(), "Invoice Kostas €120")
		if r.Source != "rules" || r.Draft == nil || r.Draft.TotalCents() != 12000 {
			t.Errorf("answer %q: got %+v", answer.body, r)
		}
	}
}
