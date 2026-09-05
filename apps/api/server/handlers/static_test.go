package handlers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/gofiber/fiber/v3"

	"github.com/Flyrell/shortr/apps/api/server/servertest"
)

func TestFileAndAssets(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "index.html"), "<!doctype html><title>shortr</title>")
	writeFile(t, filepath.Join(dir, "robots.txt"), "User-agent: *\nDisallow: /\n")
	writeFile(t, filepath.Join(dir, "assets", "main.js"), "export const shortr = 1;")

	app := newApp()
	app.Get("/", File(filepath.Join(dir, "index.html")))
	app.Get("/robots.txt", File(filepath.Join(dir, "robots.txt")))
	app.Get("/favicon.svg", File(filepath.Join(dir, "favicon.svg")))
	app.Get("/assets/*", Assets(filepath.Join(dir, "assets")))

	tests := []struct {
		name         string
		path         string
		wantStatus   int
		wantBody     string
		wantCacheAge bool
	}{
		{name: "index", path: "/", wantStatus: http.StatusOK, wantBody: "<!doctype html><title>shortr</title>"},
		{name: "robots", path: "/robots.txt", wantStatus: http.StatusOK, wantBody: "User-agent: *\nDisallow: /\n"},
		{name: "asset", path: "/assets/main.js", wantStatus: http.StatusOK, wantBody: "export const shortr = 1;", wantCacheAge: true},
		{name: "missing file", path: "/favicon.svg", wantStatus: http.StatusNotFound},
		{name: "missing asset", path: "/assets/missing.js", wantStatus: http.StatusNotFound},
		{name: "asset traversal", path: "/assets/../index.html", wantStatus: http.StatusNotFound},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			response := servertest.Do(t, app, httptest.NewRequest(http.MethodGet, test.path, http.NoBody))
			if response.StatusCode != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.StatusCode, test.wantStatus)
			}
			if test.wantStatus != http.StatusOK {
				return
			}
			body, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatalf("ReadAll() error = %v", err)
			}
			if string(body) != test.wantBody {
				t.Errorf("body = %q, want %q", body, test.wantBody)
			}
			cacheControl := response.Header.Get(fiber.HeaderCacheControl)
			if want := "public, max-age=" + strconv.Itoa(assetsMaxAge); test.wantCacheAge && cacheControl != want {
				t.Errorf("Cache-Control = %q, want %q", cacheControl, want)
			}
		})
	}
}

func TestIndex(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "index.html"), `<meta property="og:image" content="__BASE_URL__/og.png">`)

	app := newApp()
	app.Get("/", Index(dir, "https://sho.rt"))
	app.Get("/missing", Index(t.TempDir(), "https://sho.rt"))

	response := servertest.Do(t, app, httptest.NewRequest(http.MethodGet, "/", http.NoBody))
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if want := `<meta property="og:image" content="https://sho.rt/og.png">`; string(body) != want {
		t.Errorf("body = %q, want %q", body, want)
	}
	if got := response.Header.Get(fiber.HeaderContentType); got != fiber.MIMETextHTMLCharsetUTF8 {
		t.Errorf("Content-Type = %q, want %q", got, fiber.MIMETextHTMLCharsetUTF8)
	}

	response = servertest.Do(t, app, httptest.NewRequest(http.MethodGet, "/missing", http.NoBody))
	if response.StatusCode != http.StatusNotFound {
		t.Errorf("missing page status = %d, want %d", response.StatusCode, http.StatusNotFound)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}
