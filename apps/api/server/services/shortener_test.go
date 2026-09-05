package services

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Flyrell/shortr/apps/api/adapters"
)

func TestShortenerShorten(t *testing.T) {
	t.Parallel()

	store := &stubStore{}
	got, err := newTestShortener(store).Shorten(t.Context(), "https://example.com/page")
	if err != nil {
		t.Fatalf("Shorten() error = %v", err)
	}
	if !validCode(got.Code) {
		t.Errorf("Code = %q, want a generated base62 code", got.Code)
	}
	if want := "https://sho.rt/" + got.Code; got.ShortURL != want {
		t.Errorf("ShortURL = %q, want %q", got.ShortURL, want)
	}
	if want := shortenerTime.Add(24 * time.Hour); got.ExpiresAt != want {
		t.Errorf("ExpiresAt = %v, want %v", got.ExpiresAt, want)
	}
	want := saveCall{code: got.Code, target: "https://example.com/page", ttl: 24 * time.Hour}
	if len(store.saves) != 1 || store.saves[0] != want {
		t.Errorf("SaveURL calls = %+v, want [%+v]", store.saves, want)
	}
}

func TestNewShortenerMeasuresExpiryFromTheWallClock(t *testing.T) {
	t.Parallel()

	got, err := NewShortener(&stubStore{}, "https://sho.rt", time.Hour, testMaxURLLength).Shorten(t.Context(), "https://example.com")
	if err != nil {
		t.Fatalf("Shorten() error = %v", err)
	}
	if !got.ExpiresAt.After(time.Now()) {
		t.Errorf("ExpiresAt = %v, want a time still in the future", got.ExpiresAt)
	}
}

func TestShortenerShortenRetriesOnTakenCode(t *testing.T) {
	t.Parallel()

	store := &stubStore{takenSaves: maxSaveAttempts - 1}
	got, err := newTestShortener(store).Shorten(t.Context(), "https://example.com")
	if err != nil {
		t.Fatalf("Shorten() error = %v", err)
	}
	if len(store.saves) != maxSaveAttempts {
		t.Fatalf("SaveURL called %d times, want %d", len(store.saves), maxSaveAttempts)
	}
	if got.Code != store.saves[maxSaveAttempts-1].code {
		t.Errorf("Code = %q, want the last code tried", got.Code)
	}
	for i, save := range store.saves[:maxSaveAttempts-1] {
		if save.code == got.Code {
			t.Errorf("save %d reused the code %q that was reported taken", i, save.code)
		}
	}
}

func TestShortenerShortenErrors(t *testing.T) {
	t.Parallel()

	backendErr := errors.New("backend down")

	tests := []struct {
		name       string
		rawURL     string
		store      *stubStore
		wantErr    error
		wantPrefix string
	}{
		{name: "invalid url", rawURL: "not a url", store: &stubStore{}, wantErr: ErrInvalidURL},
		{name: "every code collided", rawURL: "https://example.com", store: &stubStore{takenSaves: maxSaveAttempts}, wantErr: ErrCodeExhausted},
		{name: "store failure", rawURL: "https://example.com", store: &stubStore{saveErr: backendErr}, wantErr: backendErr, wantPrefix: "shortener: save "},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := newTestShortener(test.store).Shorten(t.Context(), test.rawURL)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("Shorten() error = %v, want %v", err, test.wantErr)
			}
			if test.wantPrefix != "" && !strings.HasPrefix(err.Error(), test.wantPrefix) {
				t.Errorf("Shorten() error = %q, want it to start with %q", err, test.wantPrefix)
			}
		})
	}
}

func TestShortenerResolve(t *testing.T) {
	t.Parallel()

	backendErr := errors.New("backend down")

	// A malformed code must be rejected before it can become a store key, so
	// each case also pins how often the store was asked.
	tests := []struct {
		name          string
		code          string
		store         *stubStore
		want          string
		wantErr       error
		wantPrefix    string
		wantFindCalls int
	}{
		{name: "known code", code: knownCode, store: &stubStore{target: "https://example.com/page"}, want: "https://example.com/page", wantFindCalls: 1},
		{name: "code that is too short", code: "nope", store: &stubStore{}, wantErr: ErrNotFound},
		{name: "code that is too long", code: "abc1234567890", store: &stubStore{}, wantErr: ErrNotFound},
		{name: "code with a slash", code: "abc/12345678", store: &stubStore{}, wantErr: ErrNotFound},
		{name: "code with a non ascii letter", code: "abc1234567é", store: &stubStore{}, wantErr: ErrNotFound},
		{name: "empty code", code: "", store: &stubStore{}, wantErr: ErrNotFound},
		{name: "unknown code", code: knownCode, store: &stubStore{findErr: adapters.ErrNotFound}, wantErr: ErrNotFound, wantFindCalls: 1},
		{name: "store failure", code: knownCode, store: &stubStore{findErr: backendErr}, wantErr: backendErr, wantPrefix: "shortener: resolve ", wantFindCalls: 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := newTestShortener(test.store).Resolve(t.Context(), test.code)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("Resolve() error = %v, want %v", err, test.wantErr)
			}
			if errors.Is(test.wantErr, ErrNotFound) && !errors.Is(err, adapters.ErrNotFound) {
				t.Errorf("Resolve() error = %v, want it to wrap adapters.ErrNotFound", err)
			}
			if test.wantPrefix != "" && !strings.HasPrefix(err.Error(), test.wantPrefix) {
				t.Errorf("Resolve() error = %q, want it to start with %q", err, test.wantPrefix)
			}
			if got != test.want {
				t.Errorf("Resolve() = %q, want %q", got, test.want)
			}
			if test.store.findCalls != test.wantFindCalls {
				t.Errorf("FindURL called %d times, want %d", test.store.findCalls, test.wantFindCalls)
			}
		})
	}
}

func TestValidateURL(t *testing.T) {
	t.Parallel()

	// The limit is small so a row can spell out the message it produces.
	const maxLength = 64
	const prefix = "https://example.com/"
	filler := func(length int) string { return prefix + strings.Repeat("a", length-len(prefix)) }

	tests := []struct {
		name        string
		rawURL      string
		wantMessage string
	}{
		{name: "host only", rawURL: "https://example.com"},
		{name: "path query and fragment", rawURL: "http://example.com/a/b?c=d#e"},
		{name: "explicit port", rawURL: "https://example.com:8443/x"},
		{name: "non ascii host and path", rawURL: "https://exämple.com/café"},
		{name: "exactly the limit", rawURL: filler(maxLength)},
		{name: "empty", wantMessage: "the url must not be empty"},
		{name: "one over the limit", rawURL: filler(maxLength + 1), wantMessage: "the url must be at most 64 characters"},
		{name: "space", rawURL: "https://example.com/a b", wantMessage: "the url must not contain whitespace"},
		{name: "newline", rawURL: "https://example.com/\n", wantMessage: "the url must not contain whitespace"},
		{name: "tab", rawURL: "https://example.com/\ta", wantMessage: "the url must not contain whitespace"},
		{name: "delete character", rawURL: "http://example.com/\x7f", wantMessage: "the url must not contain control characters"},
		{name: "c1 control", rawURL: "https://exam\u0080ple.com/", wantMessage: "the url must not contain control characters"},
		{name: "zero width space", rawURL: "https://exa\u200bmple.com/", wantMessage: "the url must not contain control characters"},
		{name: "byte order mark", rawURL: "https://example.com/\ufeff", wantMessage: "the url must not contain control characters"},
		{name: "broken escape", rawURL: "https://example.com/%zz", wantMessage: "the url must be a valid url"},
		{name: "relative", rawURL: "/relative", wantMessage: "the url must be absolute"},
		{name: "ftp scheme", rawURL: "ftp://example.com", wantMessage: "the url must use the http or https scheme"},
		{name: "javascript scheme", rawURL: "javascript:alert(1)", wantMessage: "the url must use the http or https scheme"},
		{name: "data scheme", rawURL: "data:text/html,<script>alert(1)</script>", wantMessage: "the url must use the http or https scheme"},
		{name: "file scheme", rawURL: "file:///etc/passwd", wantMessage: "the url must use the http or https scheme"},
		{name: "scheme without a host", rawURL: "https://", wantMessage: "the url must include a host"},
		{name: "host shaped userinfo", rawURL: "https://accounts.example.com@evil.example/login", wantMessage: "the url must not contain user information"},
		{name: "credentials", rawURL: "https://user:pass@example.com/", wantMessage: "the url must not contain user information"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := validateURL(test.rawURL, maxLength)
			if test.wantMessage == "" {
				if err != nil {
					t.Fatalf("validateURL(%q) error = %v", test.rawURL, err)
				}
				return
			}
			if !errors.Is(err, ErrInvalidURL) {
				t.Fatalf("validateURL(%q) error = %v, want it to wrap ErrInvalidURL", test.rawURL, err)
			}
			var invalid InvalidURLError
			if !errors.As(err, &invalid) {
				t.Fatalf("validateURL(%q) error = %v, want an InvalidURLError", test.rawURL, err)
			}
			if invalid.Message != test.wantMessage {
				t.Errorf("Message = %q, want %q", invalid.Message, test.wantMessage)
			}
			if want := "shortener: " + test.wantMessage; err.Error() != want {
				t.Errorf("Error() = %q, want %q", err, want)
			}
		})
	}
}

func TestShortenerShortenReportsTheConfiguredURLLimit(t *testing.T) {
	t.Parallel()

	const maxLength = 128
	shortener := newShortener(&stubStore{}, "https://sho.rt", time.Hour, maxLength, func() time.Time { return shortenerTime })

	_, err := shortener.Shorten(t.Context(), "https://example.com/"+strings.Repeat("a", maxLength))
	var invalid InvalidURLError
	if !errors.As(err, &invalid) {
		t.Fatalf("Shorten() error = %v, want an InvalidURLError", err)
	}
	if want := "the url must be at most 128 characters"; invalid.Message != want {
		t.Errorf("Message = %q, want %q", invalid.Message, want)
	}
}

func TestGenerateCode(t *testing.T) {
	t.Parallel()

	const samples = 5000
	codes := make(map[string]struct{}, samples)
	characters := make(map[byte]struct{}, len(codeAlphabet))
	for range samples {
		code, err := generateCode()
		if err != nil {
			t.Fatalf("generateCode() error = %v", err)
		}
		if !validCode(code) {
			t.Fatalf("generateCode() = %q, want a valid base62 code", code)
		}
		codes[code] = struct{}{}
		for i := range len(code) {
			characters[code[i]] = struct{}{}
		}
	}
	if len(codes) != samples || len(characters) != len(codeAlphabet) {
		t.Errorf("generateCode() produced %d of %d unique codes using %d of %d alphabet characters",
			len(codes), samples, len(characters), len(codeAlphabet))
	}
}

const (
	knownCode        = "abc1234defgh"
	testMaxURLLength = 4096
)

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
	return newShortener(store, "https://sho.rt", 24*time.Hour, testMaxURLLength, func() time.Time { return shortenerTime })
}
