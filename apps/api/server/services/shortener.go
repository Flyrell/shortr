package services

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"strings"
	"time"
	"unicode"

	"github.com/Flyrell/shortr/apps/api/adapters"
)

const (
	maxSaveAttempts = 5
	maxURLLength    = 2048
	codeLength      = 12
	codeAlphabet    = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
)

var (
	ErrInvalidURL    = errors.New("shortener: invalid url")
	ErrNotFound      = fmt.Errorf("shortener: %w", adapters.ErrNotFound)
	ErrCodeExhausted = errors.New("shortener: could not allocate a free code")
)

type clock func() time.Time

type urlStore interface {
	SaveURL(ctx context.Context, code, target string, ttl time.Duration) error
	FindURL(ctx context.Context, code string) (string, error)
}

type ShortURL struct {
	Code      string
	ShortURL  string
	ExpiresAt time.Time
}

type Shortener struct {
	store   urlStore
	baseURL string
	ttl     time.Duration
	now     clock
}

func NewShortener(store urlStore, baseURL string, ttl time.Duration) *Shortener {
	return newShortener(store, baseURL, ttl, time.Now)
}

func newShortener(store urlStore, baseURL string, ttl time.Duration, now clock) *Shortener {
	return &Shortener{store: store, baseURL: baseURL, ttl: ttl, now: now}
}

func (s *Shortener) Shorten(ctx context.Context, rawURL string) (ShortURL, error) {
	if err := validateURL(rawURL); err != nil {
		return ShortURL{}, err
	}
	expiresAt := s.now().Add(s.ttl)

	for range maxSaveAttempts {
		code, err := generateCode()
		if err != nil {
			return ShortURL{}, fmt.Errorf("shortener: generate code: %w", err)
		}
		err = s.store.SaveURL(ctx, code, rawURL, s.ttl)
		if errors.Is(err, adapters.ErrCodeTaken) {
			continue
		}
		if err != nil {
			return ShortURL{}, fmt.Errorf("shortener: save %q: %w", code, err)
		}
		return ShortURL{Code: code, ShortURL: s.baseURL + "/" + code, ExpiresAt: expiresAt}, nil
	}
	return ShortURL{}, ErrCodeExhausted
}

func (s *Shortener) Resolve(ctx context.Context, code string) (string, error) {
	if !validCode(code) {
		return "", ErrNotFound
	}
	target, err := s.store.FindURL(ctx, code)
	if errors.Is(err, adapters.ErrNotFound) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("shortener: resolve %q: %w", code, err)
	}
	return target, nil
}

func validateURL(rawURL string) error {
	if rawURL == "" {
		return fmt.Errorf("%w: must not be empty", ErrInvalidURL)
	}
	if len(rawURL) > maxURLLength {
		return fmt.Errorf("%w: must be at most %d characters", ErrInvalidURL, maxURLLength)
	}
	if strings.ContainsFunc(rawURL, unicode.IsSpace) {
		return fmt.Errorf("%w: must not contain whitespace", ErrInvalidURL)
	}
	// net/url only guards the ASCII controls, leaving the C1 controls, the zero
	// width space and the byte order mark free to disguise a host.
	if strings.ContainsFunc(rawURL, func(r rune) bool { return !unicode.IsPrint(r) }) {
		return fmt.Errorf("%w: must not contain control characters", ErrInvalidURL)
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("%w: must be a valid URL", ErrInvalidURL)
	}
	if !parsed.IsAbs() {
		return fmt.Errorf("%w: must be absolute", ErrInvalidURL)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("%w: must use the http or https scheme", ErrInvalidURL)
	}
	if parsed.Host == "" {
		return fmt.Errorf("%w: must include a host", ErrInvalidURL)
	}
	// Userinfo lets a link read as a trusted brand while pointing elsewhere,
	// for example https://bank.example@evil.example/.
	if parsed.User != nil {
		return fmt.Errorf("%w: must not contain user information", ErrInvalidURL)
	}
	return nil
}

// Every character is drawn with crypto/rand.Int, so there is no modulo bias.
func generateCode() (string, error) {
	size := big.NewInt(int64(len(codeAlphabet)))
	code := make([]byte, codeLength)
	for i := range code {
		index, err := rand.Int(rand.Reader, size)
		if err != nil {
			return "", fmt.Errorf("read random source: %w", err)
		}
		code[i] = codeAlphabet[index.Int64()]
	}
	return string(code), nil
}

func validCode(code string) bool {
	if len(code) != codeLength {
		return false
	}
	for i := range len(code) {
		if strings.IndexByte(codeAlphabet, code[i]) < 0 {
			return false
		}
	}
	return true
}
