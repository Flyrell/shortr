package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"

	"github.com/Flyrell/shortr/apps/api/server/servertest"
	"github.com/Flyrell/shortr/apps/api/server/services"
)

func TestRedirect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		target       string
		resolveErr   error
		wantStatus   int
		wantLocation string
		wantCode     string
	}{
		{name: "known code", target: "https://example.com/x", wantStatus: http.StatusFound, wantLocation: "https://example.com/x"},
		{name: "unknown code", resolveErr: services.ErrNotFound, wantStatus: http.StatusNotFound, wantCode: "not_found"},
		{name: "store failure", resolveErr: errors.New("store unavailable"), wantStatus: http.StatusInternalServerError, wantCode: "internal"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			shortener := &servertest.StubShortener{Target: test.target, ResolveErr: test.resolveErr}
			app := newApp()
			app.Get("/:code", Redirect(shortener))

			response := servertest.Do(t, app, httptest.NewRequest(http.MethodGet, "/"+servertest.KnownCode, http.NoBody))

			if response.StatusCode != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.StatusCode, test.wantStatus)
			}
			if shortener.GotCode != servertest.KnownCode {
				t.Errorf("Resolve() got %q, want %q", shortener.GotCode, servertest.KnownCode)
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
