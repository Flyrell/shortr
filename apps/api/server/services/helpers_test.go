package services

import (
	"context"
	"time"

	"github.com/Flyrell/shortr/apps/api/adapters"
)

const knownCode = "abc1234defgh"

var shortenerTime = time.Date(2026, time.September, 4, 12, 0, 0, 0, time.UTC)

type saveCall struct {
	code   string
	target string
	ttl    time.Duration
}

type stubStore struct {
	takenSaves int
	saveErr    error
	target     string
	findErr    error
	saves      []saveCall
	findCalls  int
}

func (s *stubStore) SaveURL(_ context.Context, code, target string, ttl time.Duration) error {
	s.saves = append(s.saves, saveCall{code: code, target: target, ttl: ttl})
	if len(s.saves) <= s.takenSaves {
		return adapters.ErrCodeTaken
	}
	return s.saveErr
}

func (s *stubStore) FindURL(_ context.Context, _ string) (string, error) {
	s.findCalls++
	return s.target, s.findErr
}

func newTestShortener(store urlStore) *Shortener {
	return newShortener(store, "https://sho.rt", 24*time.Hour, func() time.Time { return shortenerTime })
}
