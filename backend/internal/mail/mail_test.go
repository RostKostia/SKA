package mail

import (
	"bufio"
	"context"
	"encoding/base64"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeSMTP is a minimal SMTP server on 127.0.0.1 (no TLS, so net/smtp allows PLAIN auth to it).
// It accepts the password "right" and rejects anything else with Gmail's 535 answer.
type fakeSMTP struct {
	addr string
	mu   sync.Mutex
	got  []string // DATA bodies received
	from []string
	rcpt []string
}

func startFakeSMTP(t *testing.T, stall bool) *fakeSMTP {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	f := &fakeSMTP{addr: ln.Addr().String()}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go f.serve(conn, stall)
		}
	}()
	return f
}

func (f *fakeSMTP) serve(conn net.Conn, stall bool) {
	defer conn.Close()
	if stall {
		time.Sleep(3 * time.Second) // never greets in time
		return
	}
	r := bufio.NewReader(conn)
	say := func(s string) { conn.Write([]byte(s + "\r\n")) }
	say("220 fake ESMTP")
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		cmd := strings.TrimSpace(line)
		up := strings.ToUpper(cmd)
		switch {
		case strings.HasPrefix(up, "EHLO"):
			say("250-fake")
			say("250 AUTH PLAIN")
		case strings.HasPrefix(up, "AUTH PLAIN"):
			raw, _ := base64.StdEncoding.DecodeString(strings.TrimSpace(cmd[len("AUTH PLAIN"):]))
			parts := strings.Split(string(raw), "\x00")
			if len(parts) == 3 && parts[1] == "kerdosmail@gmail.com" && parts[2] == "right" {
				say("235 2.7.0 Accepted")
			} else {
				say("535 5.7.8 Username and Password not accepted.")
			}
		case strings.HasPrefix(up, "MAIL FROM:"):
			f.mu.Lock()
			f.from = append(f.from, cmd)
			f.mu.Unlock()
			say("250 OK")
		case strings.HasPrefix(up, "RCPT TO:"):
			f.mu.Lock()
			f.rcpt = append(f.rcpt, cmd)
			f.mu.Unlock()
			say("250 OK")
		case up == "DATA":
			say("354 go ahead")
			var body strings.Builder
			for {
				l, err := r.ReadString('\n')
				if err != nil {
					return
				}
				if l == ".\r\n" {
					break
				}
				body.WriteString(l)
			}
			f.mu.Lock()
			f.got = append(f.got, body.String())
			f.mu.Unlock()
			say("250 queued")
		case up == "QUIT":
			say("221 bye")
			return
		default:
			say("250 OK")
		}
	}
}

func smtpFor(f *fakeSMTP, password string) SMTP {
	host, port, _ := net.SplitHostPort(f.addr)
	_ = host
	return SMTP{Host: "localhost", Port: port, User: "kerdosmail@gmail.com", Password: password, From: "Kerdos <kerdosmail@gmail.com>",
		DialTimeout: time.Second, Timeout: 2 * time.Second}
}

func TestSMTPSends(t *testing.T) {
	f := startFakeSMTP(t, false)
	id, err := smtpFor(f, "right").Send(context.Background(), Message{To: "kostas@example.com", Subject: "Invoice KRD-2026-0006 · €120", Body: "Hi Kostas,\n.\nPay here."})
	if err != nil || id == "" {
		t.Fatalf("send: %q %v", id, err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.got) != 1 || !strings.Contains(f.from[0], "kerdosmail@gmail.com") || !strings.Contains(f.rcpt[0], "kostas@example.com") {
		t.Fatalf("server saw from=%v rcpt=%v bodies=%d", f.from, f.rcpt, len(f.got))
	}
	body := f.got[0]
	for _, want := range []string{"From: Kerdos <kerdosmail@gmail.com>", "To: kostas@example.com", "=?utf-8?q?", "Content-Type: text/plain; charset=UTF-8", "Hi Kostas,\r\n..\r\nPay here."} {
		if !strings.Contains(body, want) {
			t.Errorf("message lacks %q:\n%s", want, body)
		}
	}
}

func TestSMTPWrongPassword(t *testing.T) {
	f := startFakeSMTP(t, false)
	_, err := smtpFor(f, "kerdos").Send(context.Background(), Message{To: "kostas@example.com", Subject: "x", Body: "y"})
	if err == nil || !strings.Contains(err.Error(), "535") || !strings.Contains(err.Error(), "login as kerdosmail@gmail.com") {
		t.Fatalf("want a 535 login error, got %v", err)
	}
}

func TestSMTPTimesOut(t *testing.T) {
	f := startFakeSMTP(t, true)
	start := time.Now()
	_, err := smtpFor(f, "right").Send(context.Background(), Message{To: "kostas@example.com", Subject: "x", Body: "y"})
	if err == nil || time.Since(start) > 2500*time.Millisecond {
		t.Fatalf("want a timeout within ~2 s, got %v after %s", err, time.Since(start))
	}
}

func TestSMTPRefusesHeaderInjection(t *testing.T) {
	if _, err := (SMTP{Host: "localhost", Port: "1"}).Send(context.Background(), Message{To: "a@b.c\r\nBcc: x@y.z", Subject: "s"}); err == nil {
		t.Fatal("header injection was not refused")
	}
}
