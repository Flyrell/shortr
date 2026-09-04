package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Flyrell/shortr/apps/api/server/servertest"
)

func TestRateLimitBlocksAfterTheLimit(t *testing.T) {
	t.Parallel()

	app := servertest.NewApp(nil, testProxy)
	app.Use(RateLimit(2, time.Minute))
	app.Post("/api/shorten", ok)

	send := func(ip string) *http.Response {
		request := httptest.NewRequest(http.MethodPost, "/api/shorten", http.NoBody)
		request.Header.Set("X-Forwarded-For", ip)
		return servertest.Do(t, app, request)
	}

	for attempt := 1; attempt <= 2; attempt++ {
		response := send("203.0.113.7")
		if response.StatusCode != http.StatusOK {
			t.Fatalf("attempt %d status = %d, want %d", attempt, response.StatusCode, http.StatusOK)
		}
		if got := response.Header.Get("X-RateLimit-Limit"); got != "2" {
			t.Errorf("attempt %d X-RateLimit-Limit = %q, want %q", attempt, got, "2")
		}
	}

	blocked := send("203.0.113.7")
	if blocked.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("third attempt status = %d, want %d", blocked.StatusCode, http.StatusTooManyRequests)
	}
	if blocked.Header.Get("Retry-After") == "" {
		t.Error("Retry-After is missing")
	}

	if other := send("198.51.100.4"); other.StatusCode != http.StatusOK {
		t.Errorf("other client status = %d, want %d", other.StatusCode, http.StatusOK)
	}

	for _, spoofed := range []string{"1.1.1.1, 203.0.113.7", "not-an-ip, 203.0.113.7"} {
		if response := send(spoofed); response.StatusCode != http.StatusTooManyRequests {
			t.Errorf("X-Forwarded-For %q status = %d, want %d", spoofed, response.StatusCode, http.StatusTooManyRequests)
		}
	}
}
