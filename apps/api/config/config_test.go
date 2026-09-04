package config

import (
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	// The default STATIC_DIR is relative, so the test runs from a directory
	// that contains it. That rules out t.Parallel.
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "apps", "client", "dist"), 0o750); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	t.Chdir(root)

	cfg, err := Load(lookupFrom(nil))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	want := Config{
		Port:           8080,
		BaseURL:        "http://localhost:8080",
		StaticDir:      "./apps/client/dist",
		URLTTL:         30 * 24 * time.Hour,
		Persister:      "memory",
		RateLimitMode:  RateLimitDay,
		RateLimitValue: 30,
		LogLevel:       slog.LevelInfo,
	}
	if cfg.Port != want.Port || cfg.BaseURL != want.BaseURL || cfg.StaticDir != want.StaticDir {
		t.Errorf("Load() = %+v, want %+v", cfg, want)
	}
	if cfg.URLTTL != want.URLTTL || cfg.Persister != want.Persister {
		t.Errorf("Load() ttl/persister = %v/%v, want %v/%v", cfg.URLTTL, cfg.Persister, want.URLTTL, want.Persister)
	}
	if cfg.RateLimitMode != want.RateLimitMode || cfg.RateLimitValue != want.RateLimitValue {
		t.Errorf("Load() rate limit = %v/%d, want %v/%d", cfg.RateLimitMode, cfg.RateLimitValue, want.RateLimitMode, want.RateLimitValue)
	}
	if cfg.LogLevel != want.LogLevel || len(cfg.TrustedProxies) != 0 {
		t.Errorf("Load() log level/proxies = %v/%v", cfg.LogLevel, cfg.TrustedProxies)
	}
}

func TestLoadOverrides(t *testing.T) {
	t.Parallel()

	staticDir := t.TempDir()
	// PORT is padded on purpose: numbers follow the same whitespace policy as
	// strings.
	cfg, err := Load(lookupFrom(map[string]string{
		"PORT":             "  9000  ",
		"BASE_URL":         "  https://sho.rt/  ",
		"STATIC_DIR":       staticDir,
		"URL_TTL":          "12h",
		"RATE_LIMIT_MODE":  "minute",
		"RATE_LIMIT_VALUE": "5",
		"TRUSTED_PROXIES":  "10.0.0.0/8, 192.168.1.0/24",
		"LOG_LEVEL":        "debug",
	}))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Port != 9000 || cfg.BaseURL != "https://sho.rt" || cfg.StaticDir != staticDir {
		t.Errorf("Load() = %+v", cfg)
	}
	if cfg.URLTTL != 12*time.Hour || cfg.RateLimitMode != RateLimitMinute || cfg.RateLimitValue != 5 {
		t.Errorf("Load() ttl/rate = %v/%v/%d", cfg.URLTTL, cfg.RateLimitMode, cfg.RateLimitValue)
	}
	if len(cfg.TrustedProxies) != 2 || cfg.TrustedProxies[0].String() != "10.0.0.0/8" {
		t.Errorf("Load() trusted proxies = %v", cfg.TrustedProxies)
	}
	if cfg.LogLevel != slog.LevelDebug {
		t.Errorf("Load() log level = %v, want debug", cfg.LogLevel)
	}
}

func TestLoadFallsBackOnBlankValues(t *testing.T) {
	t.Parallel()

	cfg, err := Load(lookupFrom(map[string]string{
		"STATIC_DIR":       t.TempDir(),
		"PORT":             "   ",
		"BASE_URL":         "  ",
		"RATE_LIMIT_VALUE": " ",
	}))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Port != 8080 || cfg.BaseURL != "http://localhost:8080" || cfg.RateLimitValue != 30 {
		t.Errorf("Load() = %+v, want the defaults for whitespace only values", cfg)
	}
}

func TestLoadPersister(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "unset falls back to memory", want: "memory"},
		{name: "memory", value: "memory", want: "memory"},
		{name: "redis", value: "redis", want: "redis"},
		{name: "blank falls back to memory", value: "  ", want: "memory"},
		// The adapters package owns the list of names, so an unknown one loads
		// here and fails when the adapter is opened.
		{name: "unknown name is accepted", value: "postgres", want: "postgres"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg, err := Load(lookupFrom(withStaticDir(t, map[string]string{"URL_PERSISTER": test.value})))
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if cfg.Persister != test.want {
				t.Errorf("Persister = %q, want %q", cfg.Persister, test.want)
			}
		})
	}
}

func TestLoadValidationFailures(t *testing.T) {
	t.Parallel()

	staticFile := filepath.Join(t.TempDir(), "dist")
	if err := os.WriteFile(staticFile, nil, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	tests := []struct {
		name     string
		values   map[string]string
		variable string
	}{
		{"static dir missing", map[string]string{"STATIC_DIR": filepath.Join(t.TempDir(), "absent")}, "STATIC_DIR"},
		{"static dir is a file", map[string]string{"STATIC_DIR": staticFile}, "STATIC_DIR"},
		{"port not a number", map[string]string{"PORT": "http"}, "PORT"},
		{"port too low", map[string]string{"PORT": "0"}, "PORT"},
		{"port too high", map[string]string{"PORT": "65536"}, "PORT"},
		{"base url relative", map[string]string{"BASE_URL": "/relative"}, "BASE_URL"},
		{"base url wrong scheme", map[string]string{"BASE_URL": "ftp://example.com"}, "BASE_URL"},
		{"base url without host", map[string]string{"BASE_URL": "http://"}, "BASE_URL"},
		{"base url unparsable", map[string]string{"BASE_URL": "http://a b.com/%zz"}, "BASE_URL"},
		{"ttl malformed", map[string]string{"URL_TTL": "30days"}, "URL_TTL"},
		{"ttl zero", map[string]string{"URL_TTL": "0s"}, "URL_TTL"},
		{"ttl negative", map[string]string{"URL_TTL": "-1h"}, "URL_TTL"},
		{"rate limit mode unknown", map[string]string{"RATE_LIMIT_MODE": "week"}, "RATE_LIMIT_MODE"},
		{"rate limit value zero", map[string]string{"RATE_LIMIT_VALUE": "0"}, "RATE_LIMIT_VALUE"},
		{"rate limit value malformed", map[string]string{"RATE_LIMIT_VALUE": "many"}, "RATE_LIMIT_VALUE"},
		{"trusted proxies malformed", map[string]string{"TRUSTED_PROXIES": "10.0.0.1"}, "TRUSTED_PROXIES"},
		{"log level unknown", map[string]string{"LOG_LEVEL": "trace"}, "LOG_LEVEL"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := Load(lookupFrom(withStaticDir(t, test.values)))
			assertVarError(t, err, test.variable)
		})
	}
}

func TestLoadKeepsTheUnderlyingCause(t *testing.T) {
	t.Parallel()

	missing := filepath.Join(t.TempDir(), "absent")
	_, err := Load(lookupFrom(map[string]string{"STATIC_DIR": missing}))
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Load() error = %v, want it to wrap fs.ErrNotExist", err)
	}
	var pathErr *os.PathError
	if !errors.As(err, &pathErr) {
		t.Fatalf("Load() error = %v, want it to wrap *os.PathError", err)
	}
	if !strings.Contains(err.Error(), "STATIC_DIR") {
		t.Errorf("Error() = %q, want it to name STATIC_DIR", err)
	}
}

func assertVarError(t *testing.T, err error, variable string) {
	t.Helper()

	var varErr *varError
	if !errors.As(err, &varErr) {
		t.Fatalf("Load() error = %v, want *varError", err)
	}
	if varErr.Variable != variable {
		t.Errorf("Variable = %q, want %q", varErr.Variable, variable)
	}
	if !strings.Contains(varErr.Error(), variable) {
		t.Errorf("Error() = %q, want it to name %q", varErr.Error(), variable)
	}
}
