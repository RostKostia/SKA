// Package mail sends Kerdos's emails: logged only (mock) or through any SMTP server
// (for example Gmail with an app password), so the demo's second phone gets a real email.
package mail

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"mime"
	"net"
	"net/smtp"
	"strings"
	"time"
)

type Message struct {
	To, Subject, Body string
}

type Mailer interface {
	Send(ctx context.Context, m Message) (providerMessageID string, err error)
	Provider() string // stored in outbound_messages.provider: "mock", "smtp" or "gmail"
}

type Log struct{ Log *slog.Logger }

func (l Log) Provider() string { return "mock" }
func (l Log) Send(_ context.Context, m Message) (string, error) {
	l.Log.Info("email (not sent: no SMTP_HOST)", "to", m.To, "subject", m.Subject, "body", m.Body)
	return "logged-" + randomID(), nil
}

type SMTP struct {
	Host, Port, User, Password, From string
}

func (s SMTP) Provider() string {
	if strings.EqualFold(s.Host, "smtp.gmail.com") {
		return "gmail"
	}
	return "smtp"
}

func (s SMTP) Send(_ context.Context, m Message) (string, error) {
	if strings.ContainsAny(m.To, "\r\n") || strings.ContainsAny(m.Subject, "\r\n") {
		return "", fmt.Errorf("mail: header injection attempt")
	}
	from := s.From
	if from == "" {
		from = s.User
	}
	domain := "kerdos.local"
	if _, d, ok := strings.Cut(from, "@"); ok {
		domain = strings.Trim(d, "> ")
	}
	id := "<" + randomID() + "@" + domain + ">"
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\nTo: %s\r\nSubject: %s\r\nDate: %s\r\nMessage-ID: %s\r\n", from, m.To,
		mime.QEncoding.Encode("utf-8", m.Subject), time.Now().Format(time.RFC1123Z), id)
	b.WriteString("MIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\n")
	b.WriteString(strings.ReplaceAll(m.Body, "\n", "\r\n"))
	var auth smtp.Auth
	if s.User != "" {
		auth = smtp.PlainAuth("", s.User, s.Password, s.Host)
	}
	if err := smtp.SendMail(net.JoinHostPort(s.Host, s.Port), auth, s.User, []string{m.To}, []byte(b.String())); err != nil {
		return "", err
	}
	return id, nil
}

func randomID() string {
	var b [12]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
