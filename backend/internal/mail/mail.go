// Package mail sends Kerdos's emails: logged only (mock) or through any SMTP server
// (for example Gmail with an app password), so the demo's second phone gets a real email.
package mail

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net"
	netmail "net/mail"
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
	// DialTimeout and Timeout bound the whole send, so a slow or unreachable server can't
	// hang a chat request. Zero means 10 s to connect and 30 s for the whole conversation.
	DialTimeout, Timeout time.Duration
}

func (s SMTP) Provider() string {
	if strings.EqualFold(s.Host, "smtp.gmail.com") {
		return "gmail"
	}
	return "smtp"
}

// Send delivers one email. Port 465 uses TLS from the start; any other port upgrades with
// STARTTLS when the server offers it. Authentication is PLAIN, which net/smtp only sends over
// TLS (or to localhost).
func (s SMTP) Send(ctx context.Context, m Message) (string, error) {
	if strings.ContainsAny(m.To, "\r\n") || strings.ContainsAny(m.Subject, "\r\n") {
		return "", fmt.Errorf("mail: header injection attempt")
	}
	from := s.From
	if from == "" {
		from = s.User
	}
	envelope := s.User
	if addr, err := netmail.ParseAddress(from); err == nil && envelope == "" {
		envelope = addr.Address
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

	dialTimeout, timeout := s.DialTimeout, s.Timeout
	if dialTimeout == 0 {
		dialTimeout = 10 * time.Second
	}
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	d := net.Dialer{Timeout: dialTimeout}
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(s.Host, s.Port))
	if err != nil {
		return "", fmt.Errorf("mail: connect to %s:%s: %w", s.Host, s.Port, err)
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	tlsConf := &tls.Config{ServerName: s.Host, MinVersion: tls.VersionTLS12}
	if s.Port == "465" {
		conn = tls.Client(conn, tlsConf)
	}
	c, err := smtp.NewClient(conn, s.Host)
	if err != nil {
		return "", fmt.Errorf("mail: %w", err)
	}
	defer c.Close()
	if ok, _ := c.Extension("STARTTLS"); ok && s.Port != "465" {
		if err := c.StartTLS(tlsConf); err != nil {
			return "", fmt.Errorf("mail: starttls: %w", err)
		}
	}
	if s.User != "" {
		if err := c.Auth(smtp.PlainAuth("", s.User, s.Password, s.Host)); err != nil {
			return "", fmt.Errorf("mail: login as %s: %w", s.User, err)
		}
	}
	if err := c.Mail(envelope); err != nil {
		return "", fmt.Errorf("mail: sender: %w", err)
	}
	if err := c.Rcpt(m.To); err != nil {
		return "", fmt.Errorf("mail: recipient %s: %w", m.To, err)
	}
	w, err := c.Data()
	if err != nil {
		return "", fmt.Errorf("mail: %w", err)
	}
	if _, err := io.WriteString(w, b.String()); err != nil {
		return "", fmt.Errorf("mail: %w", err)
	}
	if err := w.Close(); err != nil {
		return "", fmt.Errorf("mail: server refused the message: %w", err)
	}
	_ = c.Quit()
	return id, nil
}

func randomID() string {
	var b [12]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
