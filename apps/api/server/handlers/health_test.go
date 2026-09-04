package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Flyrell/shortr/apps/api/server/servertest"
)

func TestHealth(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		pingErr    error
		wantStatus int
		wantState  string
	}{
		{name: "adapter reachable", wantStatus: http.StatusOK, wantState: "ok"},
		{name: "adapter unreachable", pingErr: errors.New("dial tcp: refused"), wantStatus: http.StatusServiceUnavailable},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			app := newApp()
			app.Get("/healthz", Health(servertest.StubPinger{Err: test.pingErr}))

			response := servertest.Do(t, app, httptest.NewRequest(http.MethodGet, "/healthz", http.NoBody))
			if response.StatusCode != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.StatusCode, test.wantStatus)
			}
			if test.wantState != "" {
				if got := decodeJSON[healthResponse](t, response).Status; got != test.wantState {
					t.Errorf("status = %q, want %q", got, test.wantState)
				}
				return
			}
			if got := decodeJSON[errorBody](t, response).Error; got != "unavailable" {
				t.Errorf("error = %q, want %q", got, "unavailable")
			}
		})
	}
}
