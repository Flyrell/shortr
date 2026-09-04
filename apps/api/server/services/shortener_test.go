package services

import (
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

	live, err := NewShortener(&stubStore{}, "https://sho.rt", time.Hour).Shorten(t.Context(), "https://example.com")
	if err != nil || !live.ExpiresAt.After(time.Now()) {
		t.Errorf("Shorten() = %v, %v, want an expiry measured from the wall clock", live.ExpiresAt, err)
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
		{name: "code that is too short", code: "nope", store: &stubStore{findErr: adapters.ErrNotFound}, wantErr: ErrNotFound},
		{name: "code that is too long", code: "abc1234567890", store: &stubStore{findErr: adapters.ErrNotFound}, wantErr: ErrNotFound},
		{name: "code with a slash", code: "abc/12345678", store: &stubStore{findErr: adapters.ErrNotFound}, wantErr: ErrNotFound},
		{name: "code with a non ascii letter", code: "abc1234567é", store: &stubStore{findErr: adapters.ErrNotFound}, wantErr: ErrNotFound},
		{name: "empty code", code: "", store: &stubStore{findErr: adapters.ErrNotFound}, wantErr: ErrNotFound},
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

	accepted := []string{
		"https://example.com",
		"http://example.com/a/b?c=d#e",
		"https://example.com:8443/x",
		"https://exämple.com/café",
		"https://example.com/" + strings.Repeat("a", 2028),
	}
	rejected := []string{
		"", "/relative", "https://", "ftp://example.com", "javascript:alert(1)",
		"data:text/html,<script>alert(1)</script>", "file:///etc/passwd",
		"https://example.com/a b", "https://example.com/\n", "https://example.com/\ta",
		"https://example.com/" + strings.Repeat("a", 2029), "https://example.com/%zz",
		"http://example.com/\x7f", "https://exam\u0080ple.com/", "https://exa\u200bmple.com/",
		"https://example.com/\ufeff", "https://accounts.example.com@evil.example/login",
		"https://user:pass@example.com/",
	}

	for _, rawURL := range accepted {
		if err := validateURL(rawURL); err != nil {
			t.Errorf("validateURL(%q) error = %v", rawURL, err)
		}
	}
	for _, rawURL := range rejected {
		if err := validateURL(rawURL); !errors.Is(err, ErrInvalidURL) {
			t.Errorf("validateURL(%q) error = %v, want ErrInvalidURL", rawURL, err)
		}
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
