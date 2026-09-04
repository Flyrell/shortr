package adapters

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

const urlPrefix = "url:"

var _ Adapter = (*Redis)(nil)

type Redis struct {
	client    *goredis.Client
	closeOnce sync.Once
	closeErr  error
}

func NewRedis(env Env) (*Redis, error) {
	host, err := redisVar(env, "REDIS_HOST")
	if err != nil {
		return nil, err
	}
	rawPort, err := redisVar(env, "REDIS_PORT")
	if err != nil {
		return nil, err
	}
	port, err := redisIntVar("REDIS_PORT", rawPort, 1, 65535)
	if err != nil {
		return nil, err
	}
	user, err := redisVar(env, "REDIS_USER")
	if err != nil {
		return nil, err
	}
	// The password is opaque, so surrounding whitespace belongs to it: it is
	// the one variable kept untrimmed, and it is never rendered anywhere.
	password, err := redisVar(env, "REDIS_PASSWORD")
	if err != nil {
		return nil, err
	}
	database := 0
	if raw, ok := env("REDIS_DB"); ok && strings.TrimSpace(raw) != "" {
		if database, err = redisIntVar("REDIS_DB", raw, 0, math.MaxInt); err != nil {
			return nil, err
		}
	}
	return &Redis{client: goredis.NewClient(&goredis.Options{
		Addr:     net.JoinHostPort(strings.TrimSpace(host), strconv.Itoa(port)),
		Username: strings.TrimSpace(user),
		Password: password,
		DB:       database,
	})}, nil
}

func (r *Redis) SaveURL(ctx context.Context, code, target string, ttl time.Duration) error {
	if ttl <= 0 {
		return errInvalidTTL
	}
	stored, err := r.client.SetNX(ctx, urlPrefix+code, target, ttl).Result()
	if err != nil {
		return fmt.Errorf("redis: save %q: %w", code, redisErr(err))
	}
	if !stored {
		return ErrCodeTaken
	}
	return nil
}

func (r *Redis) FindURL(ctx context.Context, code string) (string, error) {
	target, err := r.client.Get(ctx, urlPrefix+code).Result()
	if errors.Is(err, goredis.Nil) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("redis: find %q: %w", code, redisErr(err))
	}
	return target, nil
}

func (r *Redis) Ping(ctx context.Context) error {
	if err := r.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis: ping: %w", redisErr(err))
	}
	return nil
}

func (r *Redis) Close() error {
	r.closeOnce.Do(func() {
		if err := r.client.Close(); err != nil {
			r.closeErr = fmt.Errorf("redis: close: %w", err)
		}
	})
	return r.closeErr
}

func redisVar(env Env, name string) (string, error) {
	value, ok := env(name)
	if !ok || strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("redis: %s is required", name)
	}
	return value, nil
}

func redisIntVar(name, raw string, minimum, maximum int) (int, error) {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf("redis: %s must be an integer, got %q", name, raw)
	}
	if value < minimum || value > maximum {
		return 0, fmt.Errorf("redis: %s must be between %d and %d, got %d", name, minimum, maximum, value)
	}
	return value, nil
}

func redisErr(err error) error {
	if errors.Is(err, goredis.ErrClosed) {
		return errClosed
	}
	return err
}
