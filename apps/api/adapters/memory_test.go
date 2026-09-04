package adapters

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"testing/synctest"
	"time"
)

func TestMemorySaveAndFindURL(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	clock := newTestClock()
	adapter := newMemoryAdapter(t, clock)

	if err := adapter.SaveURL(ctx, "abc1234", "https://example.com", time.Hour); err != nil {
		t.Fatalf("SaveURL() error = %v", err)
	}
	got, err := adapter.FindURL(ctx, "abc1234")
	if err != nil {
		t.Fatalf("FindURL() error = %v", err)
	}
	if want := "https://example.com"; got != want {
		t.Errorf("FindURL() = %q, want %q", got, want)
	}

	if err := adapter.SaveURL(ctx, "abc1234", "https://other.example", time.Hour); !errors.Is(err, ErrCodeTaken) {
		t.Fatalf("SaveURL() error = %v, want ErrCodeTaken", err)
	}
	clock.Advance(2 * time.Hour)
	if err := adapter.SaveURL(ctx, "abc1234", "https://other.example", time.Hour); err != nil {
		t.Fatalf("SaveURL() after expiry error = %v", err)
	}
}

func TestMemoryFindURLErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		code    string
		save    bool
		advance time.Duration
	}{
		{name: "unknown code", code: "missing"},
		{name: "expired code", code: "abc1234", save: true, advance: 2 * time.Hour},
		{name: "expired exactly at boundary", code: "abc1234", save: true, advance: time.Hour},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()
			clock := newTestClock()
			adapter := newMemoryAdapter(t, clock)
			if test.save {
				if err := adapter.SaveURL(ctx, test.code, "https://example.com", time.Hour); err != nil {
					t.Fatalf("SaveURL() error = %v", err)
				}
			}
			clock.Advance(test.advance)

			if _, err := adapter.FindURL(ctx, test.code); !errors.Is(err, ErrNotFound) {
				t.Fatalf("FindURL() error = %v, want ErrNotFound", err)
			}
		})
	}
}

func TestMemoryRejectsCancelledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	adapter := newMemoryAdapter(t, newTestClock())

	for _, test := range adapterCalls(ctx, adapter) {
		if err := test.call(); !errors.Is(err, context.Canceled) {
			t.Errorf("%s() error = %v, want context.Canceled", test.name, err)
		}
	}
}

func TestMemorySweepsExpiredEntriesOnItsTicker(t *testing.T) {
	// The bubble gives the test a fake clock, so the sweep ticker and the entry
	// expiry advance together without waiting on the wall clock.
	synctest.Test(t, func(t *testing.T) {
		ctx := t.Context()
		adapter := NewMemory()
		t.Cleanup(func() {
			if err := adapter.Close(); err != nil {
				t.Errorf("Close() error = %v", err)
			}
		})

		if err := adapter.SaveURL(ctx, "abc1234", "https://example.com", 30*time.Second); err != nil {
			t.Fatalf("SaveURL() error = %v", err)
		}

		time.Sleep(2 * sweepInterval)
		synctest.Wait()

		adapter.mu.Lock()
		defer adapter.mu.Unlock()

		if len(adapter.urls) != 0 {
			t.Errorf("urls = %d, want the ticker to have emptied them", len(adapter.urls))
		}
	})
}

func TestMemoryConcurrentAccess(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	adapter := newMemoryAdapter(t, newTestClock())

	const workers = 8
	var group sync.WaitGroup
	group.Add(workers)
	for worker := range workers {
		go func() {
			defer group.Done()

			code := "code" + strconv.Itoa(worker)
			for range 50 {
				if err := adapter.SaveURL(ctx, code, "https://example.com", time.Minute); err != nil && !errors.Is(err, ErrCodeTaken) {
					t.Errorf("SaveURL() error = %v", err)
					return
				}
				if _, err := adapter.FindURL(ctx, code); err != nil {
					t.Errorf("FindURL() error = %v", err)
					return
				}
			}
		}()
	}
	group.Wait()
}
