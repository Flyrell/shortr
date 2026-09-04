package adapters

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var (
	ErrNotFound  = errors.New("adapters: not found")
	ErrCodeTaken = errors.New("adapters: code taken")

	errInvalidTTL     = errors.New("adapters: ttl must be greater than zero")
	errClosed         = errors.New("adapters: closed")
	errUnknownAdapter = errors.New("adapters: unknown adapter")
)

type Env func(key string) (string, bool)

type Adapter interface {
	SaveURL(ctx context.Context, code, target string, ttl time.Duration) error
	FindURL(ctx context.Context, code string) (string, error)
	Ping(ctx context.Context) error
	Close() error
}

func New(name string, env Env) (Adapter, error) {
	switch name {
	case "memory":
		return NewMemory(), nil
	case "redis":
		// Returning NewRedis directly would hand back a non-nil Adapter
		// wrapping a nil *Redis whenever the environment is incomplete.
		adapter, err := NewRedis(env)
		if err != nil {
			return nil, err
		}
		return adapter, nil
	default:
		return nil, fmt.Errorf("%w: %q", errUnknownAdapter, name)
	}
}
