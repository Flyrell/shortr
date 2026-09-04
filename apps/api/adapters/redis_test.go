package adapters

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

func TestNewRedisValidatesEnvironment(t *testing.T) {
	t.Parallel()

	server := miniredis.RunT(t)

	tests := []struct {
		name      string
		remove    string
		overrides map[string]string
		variable  string
	}{
		{name: "host missing", remove: "REDIS_HOST", variable: "REDIS_HOST"},
		{name: "host blank", overrides: map[string]string{"REDIS_HOST": "   "}, variable: "REDIS_HOST"},
		{name: "port missing", remove: "REDIS_PORT", variable: "REDIS_PORT"},
		{name: "port blank", overrides: map[string]string{"REDIS_PORT": " "}, variable: "REDIS_PORT"},
		{name: "port not a number", overrides: map[string]string{"REDIS_PORT": "http"}, variable: "REDIS_PORT"},
		{name: "port too low", overrides: map[string]string{"REDIS_PORT": "0"}, variable: "REDIS_PORT"},
		{name: "port too high", overrides: map[string]string{"REDIS_PORT": "65536"}, variable: "REDIS_PORT"},
		{name: "user missing", remove: "REDIS_USER", variable: "REDIS_USER"},
		{name: "user blank", overrides: map[string]string{"REDIS_USER": "  "}, variable: "REDIS_USER"},
		{name: "password missing", remove: "REDIS_PASSWORD", variable: "REDIS_PASSWORD"},
		{name: "password blank", overrides: map[string]string{"REDIS_PASSWORD": "  "}, variable: "REDIS_PASSWORD"},
		{name: "database not a number", overrides: map[string]string{"REDIS_DB": "main"}, variable: "REDIS_DB"},
		{name: "database negative", overrides: map[string]string{"REDIS_DB": "-1"}, variable: "REDIS_DB"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			values := redisValues(server, test.overrides)
			delete(values, test.remove)

			adapter, err := NewRedis(envFrom(values))
			if err == nil {
				t.Fatalf("NewRedis() = %v, want an error", adapter)
			}
			if !strings.Contains(err.Error(), test.variable) {
				t.Errorf("NewRedis() error = %q, want it to name %q", err, test.variable)
			}
			if adapter != nil {
				t.Errorf("NewRedis() = %v, want nil", adapter)
			}
		})
	}
}

func TestNewRedisUsesCredentialsAndDatabase(t *testing.T) {
	t.Parallel()

	// The password is padded on purpose: whitespace is part of a credential, so
	// the adapter must hand it to the server exactly as the environment gave it.
	const user, password = "shortr", " s3 cret "

	tests := []struct {
		name     string
		database string
		want     int
	}{
		{name: "explicit database", database: "3", want: 3},
		{name: "blank database falls back to zero", database: "  ", want: 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			server := miniredis.RunT(t)
			server.RequireUserAuth(user, password)
			adapter, err := NewRedis(envFrom(redisValues(server, map[string]string{
				"REDIS_USER":     user,
				"REDIS_PASSWORD": password,
				"REDIS_DB":       test.database,
			})))
			if err != nil {
				t.Fatalf("NewRedis() error = %v", err)
			}
			t.Cleanup(func() {
				if closeErr := adapter.Close(); closeErr != nil {
					t.Errorf("Close() error = %v", closeErr)
				}
			})

			if err = adapter.SaveURL(t.Context(), "abc1234defgh", "https://example.com", time.Hour); err != nil {
				t.Fatalf("SaveURL() error = %v", err)
			}
			got, err := server.DB(test.want).Get(urlPrefix + "abc1234defgh")
			if err != nil {
				t.Fatalf("DB(%d).Get() error = %v", test.want, err)
			}
			if want := "https://example.com"; got != want {
				t.Errorf("DB(%d).Get() = %q, want %q", test.want, got, want)
			}
			if test.want != 0 && server.DB(0).Exists(urlPrefix+"abc1234defgh") {
				t.Errorf("SaveURL() also wrote to database 0, want only %d", test.want)
			}
			for _, rendering := range []string{fmt.Sprintf("%v", adapter), fmt.Sprintf("%+v", adapter), fmt.Sprintf("%#v", adapter)} {
				if strings.Contains(rendering, password) {
					t.Errorf("rendering = %q, want it to keep the password out", rendering)
				}
			}
		})
	}
}

func TestRedisSaveAndFindURL(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	adapter, server := newRedisAdapter(t)

	if err := adapter.SaveURL(ctx, "abc1234defgh", "https://example.com", time.Minute); err != nil {
		t.Fatalf("SaveURL() error = %v", err)
	}
	if !server.Exists(urlPrefix + "abc1234defgh") {
		t.Errorf("SaveURL() did not write the %s prefixed key", urlPrefix)
	}
	if got, want := server.TTL(urlPrefix+"abc1234defgh"), time.Minute; got != want {
		t.Errorf("TTL = %v, want %v", got, want)
	}
	got, err := adapter.FindURL(ctx, "abc1234defgh")
	if err != nil {
		t.Fatalf("FindURL() error = %v", err)
	}
	if want := "https://example.com"; got != want {
		t.Errorf("FindURL() = %q, want %q", got, want)
	}

	if err := adapter.SaveURL(ctx, "abc1234defgh", "https://other.example", time.Minute); !errors.Is(err, ErrCodeTaken) {
		t.Fatalf("SaveURL() error = %v, want ErrCodeTaken", err)
	}
}

func TestRedisFindURLErrors(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	adapter, server := newRedisAdapter(t)

	if _, err := adapter.FindURL(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("FindURL() unknown code error = %v, want ErrNotFound", err)
	}
	if err := adapter.SaveURL(ctx, "abc1234defgh", "https://example.com", time.Minute); err != nil {
		t.Fatalf("SaveURL() error = %v", err)
	}
	server.FastForward(2 * time.Minute)

	if _, err := adapter.FindURL(ctx, "abc1234defgh"); !errors.Is(err, ErrNotFound) {
		t.Errorf("FindURL() expired code error = %v, want ErrNotFound", err)
	}
}

func TestRedisFailsWhenServerIsDown(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()
	adapter, server := newRedisAdapter(t)
	server.Close()

	prefixes := map[string]string{
		"SaveURL": "redis: save ",
		"FindURL": "redis: find ",
		"Ping":    "redis: ping",
	}
	for _, test := range adapterCalls(ctx, adapter) {
		err := test.call()
		if err == nil {
			t.Fatalf("%s() error = nil, want a connection error", test.name)
		}
		// A transport failure must never be reported as a missing or taken
		// code: callers would answer 404 or retry instead of failing loudly.
		if errors.Is(err, ErrNotFound) || errors.Is(err, ErrCodeTaken) {
			t.Fatalf("%s() error = %v, want a connection error, not a sentinel", test.name, err)
		}
		if !strings.HasPrefix(err.Error(), prefixes[test.name]) {
			t.Errorf("%s() error = %q, want it to start with %q", test.name, err, prefixes[test.name])
		}
	}
}
