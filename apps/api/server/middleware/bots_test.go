package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"

	"github.com/Flyrell/shortr/apps/api/server/servertest"
)

func TestBlockBots(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		userAgent  string
		wantStatus int
	}{
		{name: "no user agent", wantStatus: http.StatusOK},
		{name: "browser", userAgent: "Mozilla/5.0 (Macintosh) Safari/605.1.15", wantStatus: http.StatusOK},
		{name: "googlebot", userAgent: "Googlebot/2.1 (+http://www.google.com/bot.html)", wantStatus: http.StatusForbidden},
		{name: "uppercase token", userAgent: "CURL/8.7.1", wantStatus: http.StatusForbidden},
		{name: "token inside a longer agent", userAgent: "Mozilla/5.0 (compatible; ClaudeBot/1.0)", wantStatus: http.StatusForbidden},
		{name: "underscore token", userAgent: "ia_archiver", wantStatus: http.StatusForbidden},
		{name: "headless browser", userAgent: "Mozilla/5.0 HeadlessChrome/131.0.0.0", wantStatus: http.StatusForbidden},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			app := servertest.NewApp(nil)
			app.Get("/", BlockBots(), ok)

			request := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			if test.userAgent != "" {
				request.Header.Set(fiber.HeaderUserAgent, test.userAgent)
			}
			if response := servertest.Do(t, app, request); response.StatusCode != test.wantStatus {
				t.Errorf("status = %d, want %d", response.StatusCode, test.wantStatus)
			}
		})
	}
}
