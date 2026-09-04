package handlers

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"

	"github.com/Flyrell/shortr/apps/api/server/servertest"
	"github.com/Flyrell/shortr/apps/api/server/services"
)

func TestErrorHandler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		err         error
		wantStatus  int
		wantCode    string
		wantMessage string
	}{
		{name: "api error", err: errInvalidBody, wantStatus: http.StatusBadRequest, wantCode: "invalid_body"},
		{name: "wrapped api error", err: fmt.Errorf("bind: %w", errInvalidBody), wantStatus: http.StatusBadRequest, wantCode: "invalid_body"},
		{name: "invalid url", err: fmt.Errorf("%w: must be absolute", services.ErrInvalidURL), wantStatus: http.StatusBadRequest, wantCode: "invalid_url"},
		{name: "not found", err: services.ErrNotFound, wantStatus: http.StatusNotFound, wantCode: "not_found"},
		{name: "fiber not found", err: fiber.ErrNotFound, wantStatus: http.StatusNotFound, wantCode: "not_found"},
		{name: "fiber method not allowed", err: fiber.ErrMethodNotAllowed, wantStatus: http.StatusMethodNotAllowed, wantCode: "method_not_allowed"},
		{name: "fiber body too large", err: fiber.ErrRequestEntityTooLarge, wantStatus: http.StatusRequestEntityTooLarge, wantCode: "body_too_large"},
		{name: "fiber too many requests", err: fiber.ErrTooManyRequests, wantStatus: http.StatusTooManyRequests, wantCode: "rate_limited"},
		{name: "unmapped fiber error", err: fiber.ErrTeapot, wantStatus: http.StatusTeapot, wantCode: "request_failed"},
		{name: "unknown error", err: errors.New("boom"), wantStatus: http.StatusInternalServerError, wantCode: "internal"},
		{
			name:        "internal detail is not leaked",
			err:         errors.New("dial tcp 10.0.0.1:6379: connection refused"),
			wantStatus:  http.StatusInternalServerError,
			wantCode:    "internal",
			wantMessage: errInternal.message,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			app := newApp()
			app.Get("/boom", func(fiber.Ctx) error { return test.err })

			response := servertest.Do(t, app, httptest.NewRequest(http.MethodGet, "/boom", http.NoBody))
			if response.StatusCode != test.wantStatus {
				t.Errorf("status = %d, want %d", response.StatusCode, test.wantStatus)
			}
			body := decodeJSON[errorBody](t, response)
			if body.Error != test.wantCode {
				t.Errorf("error = %q, want %q", body.Error, test.wantCode)
			}
			if body.Message == "" {
				t.Error("message is empty")
			}
			if test.wantMessage != "" && body.Message != test.wantMessage {
				t.Errorf("message = %q, want %q", body.Message, test.wantMessage)
			}
		})
	}
}

func TestErrorHandlerMapsEveryFiberStatus(t *testing.T) {
	t.Parallel()

	for status, mapped := range statusErrors {
		t.Run(mapped.code, func(t *testing.T) {
			t.Parallel()

			app := newApp()
			app.Get("/boom", func(fiber.Ctx) error { return fiber.NewError(status) })

			response := servertest.Do(t, app, httptest.NewRequest(http.MethodGet, "/boom", http.NoBody))
			if response.StatusCode != status {
				t.Fatalf("status = %d, want %d", response.StatusCode, status)
			}
			if got := decodeJSON[errorBody](t, response).Error; got != mapped.code {
				t.Errorf("error = %q, want %q", got, mapped.code)
			}
		})
	}
}

func TestErrorHandlerLogsServerErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		err     error
		wantLog string
	}{
		{name: "unknown error", err: errors.New("dial tcp 10.0.0.1:6379: refused"), wantLog: "dial tcp 10.0.0.1:6379: refused"},
		{name: "unavailable adapter", err: fmt.Errorf("%w: %w", errUnavailable, errors.New("ping timeout")), wantLog: "ping timeout"},
		{name: "fiber internal error", err: fiber.ErrInternalServerError, wantLog: "request failed"},
		{name: "client error", err: errInvalidBody},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var logs bytes.Buffer
			app := fiber.New(fiber.Config{ErrorHandler: ErrorHandler(slog.New(slog.NewTextHandler(&logs, nil)))})
			app.Get("/boom", func(fiber.Ctx) error { return test.err })
			servertest.Do(t, app, httptest.NewRequest(http.MethodGet, "/boom", http.NoBody))

			if test.wantLog == "" && logs.Len() != 0 {
				t.Fatalf("logs = %q, want nothing", logs.String())
			}
			if test.wantLog != "" && !strings.Contains(logs.String(), test.wantLog) {
				t.Errorf("logs = %q, want them to contain %q", logs.String(), test.wantLog)
			}
		})
	}
}
