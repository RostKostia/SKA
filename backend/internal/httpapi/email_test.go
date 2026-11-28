package httpapi_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
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

// switchMailer records what it sends, and fails like Gmail with a wrong password while fail is set.
type switchMailer struct {
	captureMailer
	mu   sync.Mutex
	fail bool
}

func (s *switchMailer) setFail(v bool)   { s.mu.Lock(); s.fail = v; s.mu.Unlock() }
func (s *switchMailer) Provider() string { return "gmail" }
func (s *switchMailer) Send(ctx context.Context, m mail.Message) (string, error) {
	s.mu.Lock()
	fail := s.fail
	s.mu.Unlock()
	if fail {
		return "", errors.New("mail: login as kerdosmail@gmail.com: 535 5.7.8 Username and Password not accepted.")
	}
	return s.captureMailer.Send(ctx, m)
}

// TestEmailOutcomesAreHonest: Kerdos says an email went out only when it did, a failed email
// can be retried, and an email for a cancelled invoice never goes out, even if requeued by hand.
func TestEmailOutcomesAreHonest(t *testing.T) {
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
	mailer := &switchMailer{}
	a := &app.App{DB: db, Log: quiet, Parser: ai.Rules{}, Deliverer: jinius.Mock{PublicURL: "http://front"}, Mailer: mailer,
		PublicURL: "http://front", Demo: true, SeedFile: "../../../db/seed/sample.sql", FeeBP: 150}
	srv := httptest.NewServer((&httpapi.Server{App: a, Log: quiet, AuthMode: "dev"}).Handler())
	defer srv.Close()
	c := api{t, srv.URL}
	ctx := context.Background()
	c.do("POST", "/api/demo/reset", nil, 200, nil)

	// ---- Wrong password: the invoice is sent, the email isn't, and the reply says so.
	mailer.setFail(true)
	var r chatReply
	c.do("POST", "/api/chat", map[string]string{"text": "Invoice Kostas €120"}, 200, &r)
	text, card, inv := r.last()
	if strings.Contains(text, "emailed a copy") || !strings.Contains(text, "didn't go out") || !strings.Contains(text, "refused the login") {
		t.Fatalf("reply should admit the email failed: %q", text)
	}
	if inv == nil || inv.Status != "sent" || card["email_status"] != "failed" || card["email_message_id"] == nil {
		t.Fatalf("card/invoice: %v %+v", card, inv)
	}
	msgID := card["email_message_id"].(string)
	var detail app.InvoiceDetail
	c.do("GET", "/api/invoices/"+inv.ID, nil, 200, &detail)
	if len(detail.Messages) != 1 || detail.Messages[0].Status != "failed" || !strings.Contains(detail.Messages[0].Error, "535") {
		t.Fatalf("email log: %+v", detail.Messages)
	}

	// ---- Fix the password, press Retry: now it goes out; a second Retry is refused.
	mailer.setFail(false)
	var m app.Message
	c.do("POST", "/api/messages/"+msgID+"/retry", nil, 200, &m)
	if m.Status != "sent" || m.SentAt == nil || m.Error != "" || len(mailer.to("kostas@example.com")) != 1 {
		t.Fatalf("after retry: %+v, sent=%d", m, len(mailer.to("kostas@example.com")))
	}
	c.do("POST", "/api/messages/"+msgID+"/retry", nil, 409, nil)
	c.do("POST", "/api/messages/not-a-uuid/retry", nil, 404, nil)

	// ---- With a working mailer the reply names the address it went to.
	c.do("POST", "/api/chat", map[string]string{"text": "Invoice Kostas €50"}, 200, &r)
	if text, _, _ := r.last(); !strings.Contains(text, "emailed a copy to kostas@example.com") {
		t.Errorf("success reply: %q", text)
	}

	// ---- A cancelled invoice's email never goes out: not via the worker after a manual
	// requeue (what happened on 28 Nov), and not via Retry.
	mailer.setFail(true)
	c.do("POST", "/api/chat", map[string]string{"text": "Invoice Kostas €70"}, 200, &r)
	_, card, inv = r.last()
	cancelID, cancelMsg := inv.ID, card["email_message_id"].(string)
	c.do("POST", "/api/invoices/"+cancelID+"/cancel", nil, 200, nil)
	mailer.setFail(false)
	before := len(mailer.to("kostas@example.com"))
	if _, err := db.Exec(`UPDATE outbound_messages SET status = 'queued', error = NULL WHERE status = 'failed'`); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SendQueued(ctx); err != nil {
		t.Fatal(err)
	}
	var cancelled app.InvoiceDetail // a fresh value: json.Unmarshal keeps old fields that a reply leaves out
	c.do("GET", "/api/invoices/"+cancelID, nil, 200, &cancelled)
	if got := cancelled.Messages[0]; got.Status != "cancelled" || !strings.Contains(got.Error, "cancelled") {
		t.Fatalf("requeued email of a cancelled invoice: %+v", got)
	}
	if after := len(mailer.to("kostas@example.com")); after != before {
		t.Fatalf("an email for a cancelled invoice was sent (%d → %d)", before, after)
	}
	c.do("POST", "/api/messages/"+cancelMsg+"/retry", nil, 409, nil) // not failed any more: cancelled

	// ---- Manual reminders: an honest answer, and "Remind now" again resends the failed one.
	var debtors struct{ Debtors []app.Debtor }
	c.do("GET", "/api/debtors", nil, 200, &debtors)
	var andreas string
	for _, d := range debtors.Debtors {
		if strings.HasPrefix(d.Name, "Andreas") {
			andreas = d.OldestInvoiceID
		}
	}
	mailer.setFail(true)
	m = app.Message{}
	c.do("POST", "/api/invoices/"+andreas+"/remind", nil, 201, &m)
	if m.Status != "failed" || m.Kind != "overdue" || m.Sequence == nil || *m.Sequence != 2 {
		t.Fatalf("failed manual reminder: %+v", m)
	}
	c.do("POST", "/api/chat", map[string]string{"text": "Remind Andreas"}, 200, &r)
	if text, _, _ := r.last(); !strings.Contains(text, "I tried to remind Andreas") || !strings.Contains(text, "refused the login") {
		t.Errorf("chat reminder reply: %q", text)
	}
	mailer.setFail(false)
	var again app.Message
	c.do("POST", "/api/invoices/"+andreas+"/remind", nil, 201, &again)
	if again.ID != m.ID || again.Status != "sent" || *again.Sequence != 2 || len(mailer.to("andreas@example.com")) != 1 {
		t.Fatalf("Remind now after fixing the password should resend reminder 2: %+v", again)
	}
	c.do("POST", "/api/invoices/"+andreas+"/remind", nil, 409, nil) // and now it was sent today

	// ---- Skip time narrates what really happened to automatic reminders.
	mailer.setFail(true)
	var skip app.SkipResult
	c.do("POST", "/api/demo/skip", map[string]int{"days": 4}, 200, &skip)
	var narration []string
	for _, msg := range skip.Messages {
		narration = append(narration, msg.Text)
	}
	joined := strings.Join(narration, "\n")
	if strings.Contains(joined, "I reminded ") || !strings.Contains(joined, "I tried to remind") {
		b, _ := json.Marshal(narration)
		t.Errorf("skip narration should report the failures: %s", b)
	}
}
