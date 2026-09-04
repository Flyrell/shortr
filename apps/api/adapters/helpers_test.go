package adapters

import (
	"context"
	"maps"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

const (
	redisUser     = "default"
	redisPassword = "s3cret"
)

var baseTime = time.Date(2026, time.September, 4, 12, 0, 0, 0, time.UTC)

type testClock struct {
	mu  sync.Mutex
	now time.Time
}

func newTestClock() *testClock {
	return &testClock{now: baseTime}
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.now
}

func (c *testClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.now = c.now.Add(d)
}

func newMemoryAdapter(t *testing.T, clock *testClock) *Memory {
	t.Helper()

	adapter := newMemory(clock.Now)
	t.Cleanup(func() {
		if err := adapter.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})
	return adapter
}

func envFrom(values map[string]string) Env {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}

func redisValues(server *miniredis.Miniredis, overrides map[string]string) map[string]string {
	values := map[string]string{
		"REDIS_HOST":     server.Host(),
		"REDIS_PORT":     server.Port(),
		"REDIS_USER":     redisUser,
		"REDIS_PASSWORD": redisPassword,
	}
	maps.Copy(values, overrides)
	return values
}

func newRedisServer(t *testing.T) *miniredis.Miniredis {
	t.Helper()

	server := miniredis.RunT(t)
	server.RequireUserAuth(redisUser, redisPassword)
	return server
}

func newRedisAdapter(t *testing.T) (*Redis, *miniredis.Miniredis) {
	t.Helper()

	server := newRedisServer(t)
	adapter, err := NewRedis(envFrom(redisValues(server, nil)))
	if err != nil {
		t.Fatalf("NewRedis() error = %v", err)
	}
	t.Cleanup(func() {
		if err := adapter.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})
	return adapter, server
}

type adapterCall struct {
	name string
	call func() error
}

func adapterCalls(ctx context.Context, adapter Adapter) []adapterCall {
	return []adapterCall{
		{name: "SaveURL", call: func() error { return adapter.SaveURL(ctx, "abc1234", "https://example.com", time.Hour) }},
		{name: "FindURL", call: func() error { _, err := adapter.FindURL(ctx, "abc1234"); return err }},
		{name: "Ping", call: func() error { return adapter.Ping(ctx) }},
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
