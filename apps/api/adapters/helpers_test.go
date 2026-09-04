package adapters

import (
	"context"
	"maps"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

const (
	redisUser     = "default"
	redisPassword = "s3cret"
)

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
		{name: "SaveURL", call: func() error { return adapter.SaveURL(ctx, "abc1234defgh", "https://example.com", time.Hour) }},
		{name: "FindURL", call: func() error { _, err := adapter.FindURL(ctx, "abc1234defgh"); return err }},
		{name: "Ping", call: func() error { return adapter.Ping(ctx) }},
	}
}
