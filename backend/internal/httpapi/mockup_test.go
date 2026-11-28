package httpapi_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RostKostia/SKA/backend/internal/app"
	"github.com/RostKostia/SKA/backend/internal/httpapi"
)

// The repo no longer ships a mockup, but KERDOS_MOCKUP_DIR still serves one from any folder.
func TestMockupIsServed(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(`<!doctype html><script src="app.js"></script>`), 0o644); err != nil {
		t.Fatal(err)
	}
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := httptest.NewServer((&httpapi.Server{App: &app.App{}, Log: quiet, AuthMode: "dev", MockupDir: dir}).Handler())
	defer srv.Close()
	noRedirect := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

	for _, path := range []string{"/", "/mockup"} {
		resp, err := noRedirect.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusFound || resp.Header.Get("Location") != "/mockup/" {
			t.Errorf("GET %s: %d %s, want a redirect to /mockup/", path, resp.StatusCode, resp.Header.Get("Location"))
		}
	}
	resp, err := http.Get(srv.URL + "/mockup/")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !strings.Contains(string(body), "app.js") {
		t.Errorf("GET /mockup/: %d %.200s", resp.StatusCode, body)
	}
	resp, _ = http.Get(srv.URL + "/api/nope")
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("unknown API path: %d, want 404", resp.StatusCode)
	}
}
