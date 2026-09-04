package adapters

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	t.Parallel()

	server := newRedisServer(t)
	env := envFrom(redisValues(server, nil))

	tests := []struct {
		name    string
		matches func(Adapter) bool
	}{
		{name: "memory", matches: func(a Adapter) bool { _, ok := a.(*Memory); return ok }},
		{name: "redis", matches: func(a Adapter) bool { _, ok := a.(*Redis); return ok }},
		{name: "postgres"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			adapter, err := New(test.name, env)
			if test.matches == nil {
				if !errors.Is(err, errUnknownAdapter) {
					t.Fatalf("New() error = %v, want errUnknownAdapter", err)
				}
				if !strings.Contains(err.Error(), test.name) {
					t.Errorf("New() error = %q, want it to name %q", err, test.name)
				}
				if adapter != nil {
					t.Errorf("New() = %v, want nil", adapter)
				}
				return
			}
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			t.Cleanup(func() {
				if closeErr := adapter.Close(); closeErr != nil {
					t.Errorf("Close() error = %v", closeErr)
				}
			})
			if !test.matches(adapter) {
				t.Fatalf("New() = %T, want the %s adapter", adapter, test.name)
			}
			if err := adapter.Ping(t.Context()); err != nil {
				t.Errorf("Ping() error = %v", err)
			}
		})
	}
}

func TestNewRedisReportsEnvironmentErrors(t *testing.T) {
	t.Parallel()

	adapter, err := New("redis", envFrom(nil))
	if err == nil {
		t.Fatalf("New() = %v, want an error", adapter)
	}
	if !strings.Contains(err.Error(), "REDIS_HOST") {
		t.Errorf("New() error = %q, want it to name REDIS_HOST", err)
	}
	if adapter != nil {
		t.Errorf("New() = %v, want nil", adapter)
	}
}

func TestAdapterRejectsNonPositiveTTL(t *testing.T) {
	t.Parallel()

	ttls := map[string]time.Duration{"zero": 0, "negative": -time.Minute}
	for name, adapter := range contractAdapters(t) {
		for ttlName, ttl := range ttls {
			t.Run(name+" "+ttlName, func(t *testing.T) {
				if err := adapter.SaveURL(t.Context(), "abc1234defgh", "https://example.com", ttl); !errors.Is(err, errInvalidTTL) {
					t.Errorf("SaveURL() error = %v, want errInvalidTTL", err)
				}
			})
		}
	}
}

func TestAdapterRejectsUseAfterClose(t *testing.T) {
	t.Parallel()

	for name, adapter := range contractAdapters(t) {
		t.Run(name, func(t *testing.T) {
			for i := range 3 {
				if err := adapter.Close(); err != nil {
					t.Fatalf("Close() call %d error = %v", i+1, err)
				}
			}
			for _, test := range adapterCalls(t.Context(), adapter) {
				if err := test.call(); !errors.Is(err, errClosed) {
					t.Errorf("%s() error = %v, want errClosed", test.name, err)
				}
			}
		})
	}
}

func contractAdapters(t *testing.T) map[string]Adapter {
	t.Helper()

	memory := NewMemory()
	t.Cleanup(func() {
		if err := memory.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})
	redis, _ := newRedisAdapter(t)
	return map[string]Adapter{"memory": memory, "redis": redis}
}
