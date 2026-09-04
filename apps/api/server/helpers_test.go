package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/Flyrell/shortr/apps/api/server/servertest"
	"github.com/Flyrell/shortr/apps/api/server/services"
)

const (
	defaultRateLimit = 100
	redirectTarget   = "https://example.com/x"
)

type appStubs struct {
	rateLimit      int
	shortenErr     error
	pingErr        error
	trustedProxies []netip.Prefix
}

func newTestApp(t *testing.T, stubs appStubs) *fiber.App {
	t.Helper()

	limit := stubs.rateLimit
	if limit == 0 {
		limit = defaultRateLimit
	}
	return New(&Deps{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Shortener: &servertest.StubShortener{
			Short:      services.ShortURL{Code: servertest.KnownCode, ShortURL: "https://sho.rt/" + servertest.KnownCode},
			ShortenErr: stubs.shortenErr,
			Target:     redirectTarget,
		},
		Adapter:         servertest.StubPinger{Err: stubs.pingErr},
		StaticDir:       staticDir(t),
		TrustedProxies:  stubs.trustedProxies,
		RateLimitWindow: time.Minute,
		RateLimitValue:  limit,
	})
}

func staticDir(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	for _, name := range []string{"index.html", "robots.txt", "favicon.svg", "assets/main.js"} {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll() error = %v", err)
		}
		if err := os.WriteFile(path, []byte(name), 0o600); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
	}
	return dir
}

func postToListener(t *testing.T, app *fiber.App, path, body string) *http.Response {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	served := make(chan error, 1)
	go func() { served <- app.Listener(listener, fiber.ListenConfig{DisableStartupMessage: true}) }()
	t.Cleanup(func() {
		if shutdownErr := app.Shutdown(); shutdownErr != nil {
			t.Errorf("Shutdown() error = %v", shutdownErr)
		}
		if serveErr := <-served; serveErr != nil {
			t.Errorf("Listener() error = %v", serveErr)
		}
	})

	response, err := http.Post("http://"+listener.Addr().String()+path, fiber.MIMEApplicationJSON, strings.NewReader(body))
	if err != nil {
		t.Fatalf("Post() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := response.Body.Close(); closeErr != nil {
			t.Errorf("Close() error = %v", closeErr)
		}
	})
	return response
}

func assertSecurityHeaders(t *testing.T, response *http.Response) {
	t.Helper()

	headers := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "no-referrer",
		fiber.HeaderXRobotsTag:   "noindex, nofollow, noarchive",
	}
	for header, want := range headers {
		if got := response.Header.Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
	if response.Header.Get("Content-Security-Policy") == "" {
		t.Error("Content-Security-Policy is missing")
	}
}

func assertErrorBody(t *testing.T, response *http.Response, want string) {
	t.Helper()

	var body struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode error = %v", err)
	}
	if body.Error != want {
		t.Errorf("error = %q, want %q", body.Error, want)
	}
	if body.Message == "" {
		t.Error("message is empty")
	}
}
