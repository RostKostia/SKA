// Package config reads the backend's settings from the environment. A .env file in the
// working directory or the repo root is loaded first; real environment variables win.
package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTPAddr    string // KERDOS_HTTP_ADDR, default ":8080"
	DatabaseURL string // KERDOS_DATABASE_URL
	PublicURL   string // KERDOS_PUBLIC_URL: the frontend, used for pay links. Default http://localhost:3000
	Demo        bool   // KERDOS_DEMO=1 enables /api/demo/* (skip time, reset)
	SeedFile    string // KERDOS_SEED_FILE, default ../db/seed/sample.sql
	MockupDir   string // KERDOS_MOCKUP_DIR, default ../mockup: served at /mockup/ when the folder exists. "off" disables it
	AuthMode    string // KERDOS_AUTH: "dev" (default) signs every request in as the first admin

	WorkerInterval time.Duration // KERDOS_WORKER_INTERVAL, default 1m: reminder and email worker
	FinancingFeeBP int64         // KERDOS_MOCK_FINANCING_FEE_BP, default 150 (1.5%): sample fee on simulated offers

	AnthropicAPIKey  string // ANTHROPIC_API_KEY: empty means the rule-based parser
	AnthropicModel   string // ANTHROPIC_MODEL, default claude-haiku-5-5
	AnthropicBaseURL string // ANTHROPIC_BASE_URL, default https://api.anthropic.com

	SMTPHost     string // SMTP_HOST: empty means emails are logged, not sent
	SMTPPort     string // SMTP_PORT, default 587
	SMTPUser     string
	SMTPPassword string
	SMTPFrom     string

	BankMode        string // KERDOS_BANK: "mock" (default) or "boc"
	BoCBaseURL      string // BOC_BASE_URL
	BoCClientID     string // BOC_CLIENT_ID
	BoCClientSecret string // BOC_CLIENT_SECRET
	BoCRedirectURI  string // BOC_REDIRECT_URI: must point at GET /api/bank/callback
}

func Load() Config {
	for _, f := range []string{".env", "../.env"} {
		loadDotEnv(f)
	}
	return Config{
		HTTPAddr:         get("KERDOS_HTTP_ADDR", ":8080"),
		DatabaseURL:      get("KERDOS_DATABASE_URL", "postgres://kerdos:kerdos@127.0.0.1:5432/kerdos?sslmode=disable"),
		PublicURL:        strings.TrimRight(get("KERDOS_PUBLIC_URL", "http://localhost:3000"), "/"),
		Demo:             get("KERDOS_DEMO", "1") == "1",
		SeedFile:         get("KERDOS_SEED_FILE", "../db/seed/sample.sql"),
		MockupDir:        get("KERDOS_MOCKUP_DIR", "../mockup"),
		AuthMode:         get("KERDOS_AUTH", "dev"),
		WorkerInterval:   duration("KERDOS_WORKER_INTERVAL", time.Minute),
		FinancingFeeBP:   int64(number("KERDOS_MOCK_FINANCING_FEE_BP", 150)),
		AnthropicAPIKey:  get("ANTHROPIC_API_KEY", ""),
		AnthropicModel:   get("ANTHROPIC_MODEL", "claude-haiku-5-5"),
		AnthropicBaseURL: strings.TrimRight(get("ANTHROPIC_BASE_URL", "https://api.anthropic.com"), "/"),
		SMTPHost:         get("SMTP_HOST", ""),
		SMTPPort:         get("SMTP_PORT", "587"),
		SMTPUser:         get("SMTP_USER", ""),
		SMTPPassword:     get("SMTP_PASSWORD", ""),
		SMTPFrom:         get("SMTP_FROM", ""),
		BankMode:         get("KERDOS_BANK", "mock"),
		BoCBaseURL:       strings.TrimRight(get("BOC_BASE_URL", "https://sandbox-apis.bankofcyprus.com/df-boc-org-sb/sb/psd2"), "/"),
		BoCClientID:      get("BOC_CLIENT_ID", ""),
		BoCClientSecret:  get("BOC_CLIENT_SECRET", ""),
		BoCRedirectURI:   get("BOC_REDIRECT_URI", ""),
	}
}

func get(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func number(key string, def int) int {
	if n, err := strconv.Atoi(get(key, "")); err == nil {
		return n
	}
	return def
}

func duration(key string, def time.Duration) time.Duration {
	if d, err := time.ParseDuration(get(key, "")); err == nil && d > 0 {
		return d
	}
	return def
}

// loadDotEnv sets KEY=VALUE lines from path that are not already set. Missing files are ignored.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(strings.TrimPrefix(k, "export "))
		v = strings.Trim(strings.TrimSpace(v), `"'`)
		if _, exists := os.LookupEnv(k); !exists {
			os.Setenv(k, v)
		}
	}
}
