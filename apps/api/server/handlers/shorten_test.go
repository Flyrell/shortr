package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/Flyrell/shortr/apps/api/server/servertest"
	"github.com/Flyrell/shortr/apps/api/server/services"
)

var expiresAt = time.Date(2026, time.September, 4, 12, 0, 0, 0, time.UTC)

func TestShortenRejectsBadRequests(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		contentType string
		body        string
		shortenErr  error
		wantStatus  int
		wantCode    string
		wantMessage string
	}{
		{name: "plain text", contentType: "text/plain", body: "https://example.com", wantStatus: http.StatusUnsupportedMediaType, wantCode: "unsupported_media_type"},
		{name: "no content type", body: `{"url":"https://example.com"}`, wantStatus: http.StatusUnsupportedMediaType, wantCode: "unsupported_media_type"},
		{name: "broken json", contentType: fiber.MIMEApplicationJSON, body: "{", wantStatus: http.StatusBadRequest, wantCode: "invalid_body"},
		{name: "empty body", contentType: fiber.MIMEApplicationJSON, body: "", wantStatus: http.StatusBadRequest, wantCode: "invalid_body"},
		{name: "missing url", contentType: fiber.MIMEApplicationJSON, body: `{}`, wantStatus: http.StatusBadRequest, wantCode: "invalid_body"},
		{name: "empty url", contentType: fiber.MIMEApplicationJSON, body: `{"url":""}`, wantStatus: http.StatusBadRequest, wantCode: "invalid_body"},
		{
			name:        "invalid url",
			contentType: fiber.MIMEApplicationJSON,
			body:        `{"url":"ftp://example.com"}`,
			shortenErr:  services.InvalidURLError{Message: "the url must use the http or https scheme"},
			wantStatus:  http.StatusBadRequest,
			wantCode:    "invalid_url",
			wantMessage: "the url must use the http or https scheme",
		},
		{
			name:        "url over the length limit",
			contentType: fiber.MIMEApplicationJSON,
			body:        `{"url":"https://example.com/too-long"}`,
			shortenErr:  services.InvalidURLError{Message: "the url must be at most 4096 characters"},
			wantStatus:  http.StatusBadRequest,
			wantCode:    "invalid_url",
			wantMessage: "the url must be at most 4096 characters",
		},
		{
			name:        "service failure",
			contentType: fiber.MIMEApplicationJSON,
			body:        `{"url":"https://example.com"}`,
			shortenErr:  errors.New("store unavailable"),
			wantStatus:  http.StatusInternalServerError,
			wantCode:    "internal",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			app := newApp()
			app.Post("/api/shorten", Shorten(&servertest.StubShortener{ShortenErr: test.shortenErr}))

			request := httptest.NewRequest(http.MethodPost, "/api/shorten", strings.NewReader(test.body))
			if test.contentType != "" {
				request.Header.Set(fiber.HeaderContentType, test.contentType)
			}
			response := servertest.Do(t, app, request)

			if response.StatusCode != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.StatusCode, test.wantStatus)
			}
			body := decodeJSON[errorBody](t, response)
			if body.Error != test.wantCode {
				t.Errorf("error = %q, want %q", body.Error, test.wantCode)
			}
			if test.wantMessage != "" && body.Message != test.wantMessage {
				t.Errorf("message = %q, want %q", body.Message, test.wantMessage)
			}
		})
	}
}

func TestShortenCreatesShortURL(t *testing.T) {
	t.Parallel()

	shortener := &servertest.StubShortener{Short: services.ShortURL{
		Code:      servertest.KnownCode,
		ShortURL:  "https://sho.rt/" + servertest.KnownCode,
		ExpiresAt: expiresAt,
	}}
	app := newApp()
	app.Post("/api/shorten", Shorten(shortener))

	request := httptest.NewRequest(http.MethodPost, "/api/shorten", strings.NewReader(`{"url":"https://example.com/x"}`))
	request.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	response := servertest.Do(t, app, request)

	if response.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusCreated)
	}
	if shortener.GotURL != "https://example.com/x" {
		t.Errorf("Shorten() got %q, want the request url", shortener.GotURL)
	}
	body := decodeJSON[shortenResponse](t, response)
	want := shortenResponse{
		Code:      servertest.KnownCode,
		ShortURL:  "https://sho.rt/" + servertest.KnownCode,
		ExpiresAt: "2026-09-04T12:00:00Z",
	}
	if body != want {
		t.Errorf("body = %+v, want %+v", body, want)
	}
}
