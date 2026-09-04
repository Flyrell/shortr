package middleware

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"

	"github.com/Flyrell/shortr/apps/api/server/servertest"
)

type logRecord struct {
	Method   string `json:"method"`
	Path     string `json:"path"`
	Status   int    `json:"status"`
	Duration int64  `json:"duration"`
	IP       string `json:"ip"`
}

func TestRequestLogger(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		path       string
		wantStatus int
	}{
		{name: "successful request", path: "/", wantStatus: http.StatusOK},
		{name: "failed request", path: "/boom", wantStatus: http.StatusTeapot},
		{name: "unmatched path", path: "/missing", wantStatus: http.StatusNotFound},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var logs bytes.Buffer
			app := servertest.NewApp(nil, testProxy)
			app.Use(RequestLogger(slog.New(slog.NewJSONHandler(&logs, nil))))
			app.Get("/", ok)
			app.Get("/boom", func(fiber.Ctx) error { return fiber.ErrTeapot })

			request := httptest.NewRequest(http.MethodGet, test.path, http.NoBody)
			request.Header.Set("X-Forwarded-For", "203.0.113.7")
			if response := servertest.Do(t, app, request); response.StatusCode != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.StatusCode, test.wantStatus)
			}

			var record logRecord
			if err := json.Unmarshal(logs.Bytes(), &record); err != nil {
				t.Fatalf("unmarshal %q error = %v", logs.String(), err)
			}
			if record.Duration <= 0 {
				t.Errorf("duration = %d, want a positive value", record.Duration)
			}
			record.Duration = 0
			want := logRecord{Method: http.MethodGet, Path: test.path, Status: test.wantStatus, IP: "203.0.113.7"}
			if record != want {
				t.Errorf("record = %+v, want %+v", record, want)
			}
		})
	}
}

func TestRequestLoggerRunsTheErrorHandlerOnce(t *testing.T) {
	t.Parallel()

	runs := 0
	var logs bytes.Buffer
	app := servertest.NewApp(func(fiber.Ctx, error) error {
		runs++
		return errors.New("write failed")
	})
	app.Use(RequestLogger(slog.New(slog.NewJSONHandler(&logs, nil))))
	app.Get("/boom", func(fiber.Ctx) error { return fiber.ErrTeapot })

	servertest.Do(t, app, httptest.NewRequest(http.MethodGet, "/boom", http.NoBody))

	if runs != 1 {
		t.Errorf("error handler runs = %d, want 1", runs)
	}
	if !strings.Contains(logs.String(), "the error handler failed") {
		t.Errorf("logs = %q, want them to report the failing error handler", logs.String())
	}
}
