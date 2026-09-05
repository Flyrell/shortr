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
	codeLength      = 12
	codeAlphabet    = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
)

var (
	ErrInvalidURL    = errors.New("shortener: invalid url")
	ErrNotFound      = fmt.Errorf("shortener: %w", adapters.ErrNotFound)
	ErrCodeExhausted = errors.New("shortener: could not allocate a free code")
)

type InvalidURLError struct {
	Message string
}

func (e InvalidURLError) Error() string { return "shortener: " + e.Message }

func (e InvalidURLError) Unwrap() error { return ErrInvalidURL }

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
	store        urlStore
	baseURL      string
	ttl          time.Duration
	maxURLLength int
	now          clock
}

func NewShortener(store urlStore, baseURL string, ttl time.Duration, maxURLLength int) *Shortener {
	return newShortener(store, baseURL, ttl, maxURLLength, time.Now)
}

func newShortener(store urlStore, baseURL string, ttl time.Duration, maxURLLength int, now clock) *Shortener {
	return &Shortener{store: store, baseURL: baseURL, ttl: ttl, maxURLLength: maxURLLength, now: now}
}

func (s *Shortener) Shorten(ctx context.Context, rawURL string) (ShortURL, error) {
	if err := validateURL(rawURL, s.maxURLLength); err != nil {
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

func validateURL(rawURL string, maxLength int) error {
	if rawURL == "" {
		return InvalidURLError{Message: "the url must not be empty"}
	}
	if len(rawURL) > maxLength {
		return InvalidURLError{Message: fmt.Sprintf("the url must be at most %d characters", maxLength)}
	}
	if strings.ContainsFunc(rawURL, unicode.IsSpace) {
		return InvalidURLError{Message: "the url must not contain whitespace"}
	}
	// net/url only guards the ASCII controls, leaving the C1 controls, the zero
	// width space and the byte order mark free to disguise a host.
	if strings.ContainsFunc(rawURL, func(r rune) bool { return !unicode.IsPrint(r) }) {
		return InvalidURLError{Message: "the url must not contain control characters"}
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return InvalidURLError{Message: "the url must be a valid url"}
	}
	if !parsed.IsAbs() {
		return InvalidURLError{Message: "the url must be absolute"}
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return InvalidURLError{Message: "the url must use the http or https scheme"}
	}
	if parsed.Host == "" {
		return InvalidURLError{Message: "the url must include a host"}
	}
	// Userinfo lets a link read as a trusted brand while pointing elsewhere,
	// for example https://bank.example@evil.example/.
	if parsed.User != nil {
		return InvalidURLError{Message: "the url must not contain user information"}
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
