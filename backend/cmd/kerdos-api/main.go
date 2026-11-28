// Command kerdos-api is the Kerdos backend prototype: a JSON API over the PostgreSQL schema
// in db/, with mock adapters (bank, Jinius, email, rule-based chat) so the demo runs without
// any keys. See backend/README.md.
package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/RostKostia/SKA/backend/internal/ai"
	"github.com/RostKostia/SKA/backend/internal/app"
	"github.com/RostKostia/SKA/backend/internal/boc"
	"github.com/RostKostia/SKA/backend/internal/config"
	"github.com/RostKostia/SKA/backend/internal/httpapi"
	"github.com/RostKostia/SKA/backend/internal/jinius"
	"github.com/RostKostia/SKA/backend/internal/mail"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if err := run(log); err != nil {
		log.Error("kerdos-api stopped", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg := config.Load()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()
	db.SetMaxOpenConns(10)
	if err := waitForDB(ctx, db, log); err != nil {
		return err
	}

	httpClient := &http.Client{Timeout: 20 * time.Second}
	a := &app.App{
		DB:        db,
		Log:       log,
		Parser:    ai.Rules{},
		Deliverer: jinius.Mock{PublicURL: cfg.PublicURL},
		Mailer:    mail.Log{Log: log},
		PublicURL: cfg.PublicURL,
		Demo:      cfg.Demo,
		SeedFile:  cfg.SeedFile,
		FeeBP:     cfg.FinancingFeeBP,
	}
	if cfg.AnthropicAPIKey != "" {
		a.Parser = &ai.Claude{APIKey: cfg.AnthropicAPIKey, Model: cfg.AnthropicModel, BaseURL: cfg.AnthropicBaseURL, HTTP: httpClient, Log: log}
	}
	if cfg.SMTPHost != "" {
		a.Mailer = mail.SMTP{Host: cfg.SMTPHost, Port: cfg.SMTPPort, User: cfg.SMTPUser, Password: cfg.SMTPPassword, From: cfg.SMTPFrom}
	}
	if cfg.BankMode == "boc" {
		if cfg.BoCClientID == "" || cfg.BoCClientSecret == "" || cfg.BoCRedirectURI == "" {
			return errors.New("KERDOS_BANK=boc needs BOC_CLIENT_ID, BOC_CLIENT_SECRET and BOC_REDIRECT_URI")
		}
		a.BoC = &boc.Client{BaseURL: cfg.BoCBaseURL, ClientID: cfg.BoCClientID, ClientSecret: cfg.BoCClientSecret, RedirectURI: cfg.BoCRedirectURI, HTTP: httpClient}
	}
	parser := "rules"
	if cfg.AnthropicAPIKey != "" {
		parser = "claude (" + cfg.AnthropicModel + ")"
	}
	log.Info("kerdos-api starting", "addr", cfg.HTTPAddr, "bank", cfg.BankMode, "mailer", a.Mailer.Provider(), "parser", parser, "demo", cfg.Demo, "auth", cfg.AuthMode)

	mockupDir := ""
	if cfg.MockupDir != "off" {
		if st, err := os.Stat(cfg.MockupDir); err == nil && st.IsDir() {
			mockupDir = cfg.MockupDir
			log.Info("mockup on the API", "url", "http://localhost"+portOf(cfg.HTTPAddr)+"/mockup/")
		}
	}

	go a.Run(ctx, cfg.WorkerInterval)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           (&httpapi.Server{App: a, Log: log, AuthMode: cfg.AuthMode, PublicURL: cfg.PublicURL, MockupDir: mockupDir}).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
	}
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdown)
}

// portOf turns ":8080" or "127.0.0.1:8080" into ":8080" for the log line.
func portOf(addr string) string {
	if i := strings.LastIndex(addr, ":"); i >= 0 {
		return addr[i:]
	}
	return ":" + addr
}

// waitForDB retries for up to 30 seconds, so `docker compose up` and the API can start together.
func waitForDB(ctx context.Context, db *sql.DB, log *slog.Logger) error {
	deadline := time.Now().Add(30 * time.Second)
	for {
		pctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		err := db.PingContext(pctx)
		cancel()
		if err == nil {
			return nil
		}
		if time.Now().After(deadline) || ctx.Err() != nil {
			return errors.New("database not reachable at KERDOS_DATABASE_URL: " + err.Error())
		}
		log.Info("waiting for the database", "err", err)
		time.Sleep(time.Second)
	}
}
