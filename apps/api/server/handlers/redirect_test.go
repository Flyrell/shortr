package handlers

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"

	"github.com/Flyrell/shortr/apps/api/server/servertest"
	"github.com/Flyrell/shortr/apps/api/server/services"
)

const notFoundMarker = "the link is gone"

func TestRedirect(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, notFoundPage), []byte(notFoundMarker), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	tests := []struct {
		name         string
		target       string
		resolveErr   error
		accept       string
		wantStatus   int
		wantLocation string
		wantCode     string
		wantHTML     bool
	}{
		{name: "known code", target: "https://example.com/x", wantStatus: http.StatusFound, wantLocation: "https://example.com/x"},
		{name: "unknown code", resolveErr: services.ErrNotFound, wantStatus: http.StatusNotFound, wantCode: "not_found"},
		{name: "unknown code in a browser", resolveErr: services.ErrNotFound, accept: fiber.MIMETextHTML, wantStatus: http.StatusNotFound, wantHTML: true},
		{name: "store failure", resolveErr: errors.New("store unavailable"), accept: fiber.MIMETextHTML, wantStatus: http.StatusInternalServerError, wantCode: "internal"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			shortener := &servertest.StubShortener{Target: test.target, ResolveErr: test.resolveErr}
			app := newApp()
			app.Get("/:code", Redirect(shortener, dir))

			request := httptest.NewRequest(http.MethodGet, "/"+servertest.KnownCode, http.NoBody)
			if test.accept != "" {
				request.Header.Set(fiber.HeaderAccept, test.accept)
			}
			response := servertest.Do(t, app, request)

			if response.StatusCode != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.StatusCode, test.wantStatus)
			}
			if shortener.GotCode != servertest.KnownCode {
				t.Errorf("Resolve() got %q, want %q", shortener.GotCode, servertest.KnownCode)
			}
			if test.wantHTML {
				assertNotFoundPage(t, response)
				return
			}
			if test.wantCode != "" {
				if got := decodeJSON[errorBody](t, response).Error; got != test.wantCode {
					t.Errorf("error = %q, want %q", got, test.wantCode)
				}
				return
			}
			if got := response.Header.Get(fiber.HeaderLocation); got != test.wantLocation {
				t.Errorf("Location = %q, want %q", got, test.wantLocation)
			}
			if got := response.Header.Get(fiber.HeaderCacheControl); got != "no-store" {
				t.Errorf("Cache-Control = %q, want %q", got, "no-store")
			}
		})
	}
}

func assertNotFoundPage(t *testing.T, response *http.Response) {
	t.Helper()

	if got := response.Header.Get(fiber.HeaderContentType); !strings.HasPrefix(got, fiber.MIMETextHTML) {
		t.Errorf("Content-Type = %q, want it to start with %q", got, fiber.MIMETextHTML)
	}
	if got := response.Header.Get(fiber.HeaderCacheControl); got != "no-store" {
		t.Errorf("Cache-Control = %q, want %q", got, "no-store")
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if !strings.Contains(string(body), notFoundMarker) {
		t.Errorf("body = %q, want it to contain the not-found page", body)
	}
}

func TestRedirectFallsBackToJSONWhenPageMissing(t *testing.T) {
	t.Parallel()

	shortener := &servertest.StubShortener{ResolveErr: services.ErrNotFound}
	app := newApp()
	app.Get("/:code", Redirect(shortener, t.TempDir()))

	request := httptest.NewRequest(http.MethodGet, "/"+servertest.KnownCode, http.NoBody)
	request.Header.Set(fiber.HeaderAccept, fiber.MIMETextHTML)
	response := servertest.Do(t, app, request)

	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusNotFound)
	}
	if got := decodeJSON[errorBody](t, response).Error; got != "not_found" {
		t.Errorf("error = %q, want %q", got, "not_found")
	}
}
