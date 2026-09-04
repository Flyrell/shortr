package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"

	"github.com/Flyrell/shortr/apps/api/server/servertest"
)

func TestSecurityHeadersAndRobotsTag(t *testing.T) {
	t.Parallel()

	app := servertest.NewApp(nil)
	app.Use(SecurityHeaders())
	app.Use(RobotsTag())
	app.Get("/", ok)
	app.Get("/boom", func(fiber.Ctx) error { return fiber.ErrTeapot })

	tests := []struct {
		header string
		want   string
	}{
		{header: "Content-Security-Policy", want: contentSecurityPolicy},
		{header: "X-Content-Type-Options", want: "nosniff"},
		{header: "X-Frame-Options", want: "DENY"},
		{header: "Referrer-Policy", want: "no-referrer"},
		{header: fiber.HeaderXRobotsTag, want: robotsTag},
	}

	for _, path := range []string{"/", "/boom"} {
		response := servertest.Do(t, app, httptest.NewRequest(http.MethodGet, path, http.NoBody))
		for _, test := range tests {
			if got := response.Header.Get(test.header); got != test.want {
				t.Errorf("%s %s = %q, want %q", path, test.header, got, test.want)
			}
		}
	}
}
