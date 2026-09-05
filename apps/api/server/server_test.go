package server

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/Flyrell/shortr/apps/api/server/servertest"
	"github.com/Flyrell/shortr/apps/api/server/services"
)

const (
	shortenBody      = `{"url":"https://example.com/x"}`
	defaultRateLimit = 100
	redirectTarget   = "https://example.com/x"
)

func TestRoutes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		method       string
		path         string
		contentType  string
		body         string
		stubs        appStubs
		wantStatus   int
		wantLocation string
		wantError    string
	}{
		{name: "health", method: http.MethodGet, path: "/healthz", wantStatus: http.StatusOK},
		{name: "index", method: http.MethodGet, path: "/", wantStatus: http.StatusOK},
		{name: "robots", method: http.MethodGet, path: "/robots.txt", wantStatus: http.StatusOK},
		{name: "favicon", method: http.MethodGet, path: "/favicon.svg", wantStatus: http.StatusOK},
		{name: "favicon png", method: http.MethodGet, path: "/favicon.png", wantStatus: http.StatusOK},
		{name: "touch icon", method: http.MethodGet, path: "/apple-touch-icon.png", wantStatus: http.StatusOK},
		{name: "og image", method: http.MethodGet, path: "/og.png", wantStatus: http.StatusOK},
		{name: "asset", method: http.MethodGet, path: "/assets/main.js", wantStatus: http.StatusOK},
		{name: "missing asset", method: http.MethodGet, path: "/assets/missing.js", wantStatus: http.StatusNotFound},
		{name: "shorten", method: http.MethodPost, path: "/api/shorten", contentType: fiber.MIMEApplicationJSON, body: shortenBody, wantStatus: http.StatusCreated},
		{
			name:        "shorten rejects other media types",
			method:      http.MethodPost,
			path:        "/api/shorten",
			contentType: fiber.MIMETextPlain,
			body:        "https://example.com/x",
			wantStatus:  http.StatusUnsupportedMediaType,
			wantError:   "unsupported_media_type",
		},
		{
			name:        "shortener failure",
			method:      http.MethodPost,
			path:        "/api/shorten",
			contentType: fiber.MIMEApplicationJSON,
			body:        shortenBody,
			stubs:       appStubs{shortenErr: errors.New("backend down")},
			wantStatus:  http.StatusInternalServerError,
			wantError:   "internal",
		},
		{
			name:       "health without a reachable adapter",
			method:     http.MethodGet,
			path:       "/healthz",
			stubs:      appStubs{pingErr: errors.New("dial tcp: refused")},
			wantStatus: http.StatusServiceUnavailable,
			wantError:  "unavailable",
		},
		{name: "redirect", method: http.MethodGet, path: "/" + servertest.KnownCode, wantStatus: http.StatusFound, wantLocation: redirectTarget},
		{name: "unknown code", method: http.MethodGet, path: "/zzzzzzzzzzzz", wantStatus: http.StatusNotFound, wantError: "not_found"},
		{name: "unknown path", method: http.MethodGet, path: "/a/b/c", wantStatus: http.StatusNotFound, wantError: "not_found"},
		{name: "wrong method", method: http.MethodPost, path: "/healthz", wantStatus: http.StatusMethodNotAllowed, wantError: "method_not_allowed"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			if test.contentType != "" {
				request.Header.Set(fiber.HeaderContentType, test.contentType)
			}
			response := servertest.Do(t, newTestApp(t, test.stubs), request)

			if response.StatusCode != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.StatusCode, test.wantStatus)
			}
			if test.wantLocation != "" {
				if got := response.Header.Get(fiber.HeaderLocation); got != test.wantLocation {
					t.Errorf("Location = %q, want %q", got, test.wantLocation)
				}
			}
			assertSecurityHeaders(t, response)
			if test.wantError != "" {
				assertErrorBody(t, response, test.wantError)
			}
		})
	}
}

func TestBotBlocking(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
	}{
		{name: "index", method: http.MethodGet, path: "/", wantStatus: http.StatusForbidden},
		{name: "robots", method: http.MethodGet, path: "/robots.txt", wantStatus: http.StatusForbidden},
		{name: "favicon", method: http.MethodGet, path: "/favicon.svg", wantStatus: http.StatusForbidden},
		{name: "og image", method: http.MethodGet, path: "/og.png", wantStatus: http.StatusForbidden},
		{name: "asset", method: http.MethodGet, path: "/assets/main.js", wantStatus: http.StatusForbidden},
		{name: "shorten", method: http.MethodPost, path: "/api/shorten", wantStatus: http.StatusForbidden},
		{name: "redirect", method: http.MethodGet, path: "/" + servertest.KnownCode, wantStatus: http.StatusFound},
		{name: "health", method: http.MethodGet, path: "/healthz", wantStatus: http.StatusOK},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			request := httptest.NewRequest(test.method, test.path, strings.NewReader(shortenBody))
			request.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
			request.Header.Set(fiber.HeaderUserAgent, "Mozilla/5.0 (compatible; Googlebot/2.1)")
			response := servertest.Do(t, newTestApp(t, appStubs{}), request)

			if response.StatusCode != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.StatusCode, test.wantStatus)
			}
			if test.wantStatus == http.StatusForbidden {
				assertErrorBody(t, response, "forbidden")
			}
		})
	}
}

func TestRateLimitAppliesToShortenOnly(t *testing.T) {
	t.Parallel()

	app := newTestApp(t, appStubs{rateLimit: 1})
	shorten := func() *http.Response {
		request := httptest.NewRequest(http.MethodPost, "/api/shorten", strings.NewReader(shortenBody))
		request.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
		return servertest.Do(t, app, request)
	}

	if response := shorten(); response.StatusCode != http.StatusCreated {
		t.Fatalf("first shorten status = %d, want %d", response.StatusCode, http.StatusCreated)
	}
	limited := shorten()
	if limited.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("second shorten status = %d, want %d", limited.StatusCode, http.StatusTooManyRequests)
	}
	if limited.Header.Get("Retry-After") == "" {
		t.Error("Retry-After is missing")
	}
	assertErrorBody(t, limited, "rate_limited")

	for range 3 {
		if response := servertest.Do(t, app, httptest.NewRequest(http.MethodGet, "/"+servertest.KnownCode, http.NoBody)); response.StatusCode != http.StatusFound {
			t.Fatalf("redirect status = %d, want %d", response.StatusCode, http.StatusFound)
		}
	}
}

func TestConfig(t *testing.T) {
	t.Parallel()

	proxies := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8"), netip.MustParsePrefix("192.168.1.0/24")}
	got := newTestApp(t, appStubs{trustedProxies: proxies}).Config()

	if got.AppName != "shortr" || !got.StrictRouting || !got.CaseSensitive || !got.Immutable {
		t.Errorf("routing config = %+v", got)
	}
	if got.BodyLimit != bodyLimit || got.ReadTimeout != readTimeout || got.WriteTimeout != writeTimeout || got.IdleTimeout != idleTimeout {
		t.Errorf("limits = %+v", got)
	}
	if !got.TrustProxy || !got.EnableIPValidation || got.ProxyHeader != fiber.HeaderXForwardedFor {
		t.Errorf("proxy config = %+v", got)
	}
	if want := []string{"10.0.0.0/8", "192.168.1.0/24"}; !slices.Equal(got.TrustProxyConfig.Proxies, want) {
		t.Errorf("Proxies = %v, want %v", got.TrustProxyConfig.Proxies, want)
	}
}

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
		BaseURL:         "https://sho.rt",
		StaticDir:       staticDir(t),
		TrustedProxies:  stubs.trustedProxies,
		RateLimitWindow: time.Minute,
		RateLimitValue:  limit,
	})
}

func staticDir(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	for _, name := range append([]string{"index.html", "assets/main.js"}, rootFiles...) {
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
