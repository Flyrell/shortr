package adapters

import (
	"context"
	"maps"
	"sync"
	"time"
)

const sweepInterval = time.Minute

var _ Adapter = (*Memory)(nil)

type clock func() time.Time

type Memory struct {
	mu     sync.Mutex
	urls   map[string]urlEntry
	closed bool

	now      clock
	stop     chan struct{}
	stopOnce sync.Once
	sweeper  sync.WaitGroup
}

type urlEntry struct {
	target    string
	expiresAt time.Time
}

func NewMemory() *Memory { return newMemory(time.Now) }

func newMemory(now clock) *Memory {
	m := &Memory{
		urls: make(map[string]urlEntry),
		now:  now,
		stop: make(chan struct{}),
	}
	m.sweeper.Add(1)
	go m.sweep()
	return m
}

func (m *Memory) SaveURL(ctx context.Context, code, target string, ttl time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if ttl <= 0 {
		return errInvalidTTL
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return errClosed
	}
	now := m.now()
	if existing, ok := m.urls[code]; ok && !expired(existing.expiresAt, now) {
		return ErrCodeTaken
	}
	m.urls[code] = urlEntry{target: target, expiresAt: now.Add(ttl)}
	return nil
}

func (m *Memory) FindURL(ctx context.Context, code string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return "", errClosed
	}
	entry, ok := m.urls[code]
	if !ok {
		return "", ErrNotFound
	}
	if expired(entry.expiresAt, m.now()) {
		delete(m.urls, code)
		return "", ErrNotFound
	}
	return entry.target, nil
}

func (m *Memory) Ping(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return errClosed
	}
	return nil
}

func (m *Memory) Close() error {
	m.stopOnce.Do(func() {
		close(m.stop)
	})
	m.sweeper.Wait()

	m.mu.Lock()
	defer m.mu.Unlock()

	m.closed = true
	return nil
}

func (m *Memory) sweep() {
	defer m.sweeper.Done()

	ticker := time.NewTicker(sweepInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stop:
			return
		case <-ticker.C:
			m.purge()
		}
	}
}

func (m *Memory) purge() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := m.now()
	maps.DeleteFunc(m.urls, func(_ string, entry urlEntry) bool {
		return expired(entry.expiresAt, now)
	})
}

func expired(expiresAt, now time.Time) bool {
	return !expiresAt.After(now)
}
