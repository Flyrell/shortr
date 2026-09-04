package config

import (
	"errors"
	"log/slog"
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    time.Duration
		wantErr bool
	}{
		{name: "days", input: "30d", want: 30 * 24 * time.Hour},
		{name: "single day", input: "1d", want: 24 * time.Hour},
		{name: "zero days", input: "0d", want: 0},
		{name: "negative days", input: "-2d", want: -48 * time.Hour},
		{name: "hours", input: "12h", want: 12 * time.Hour},
		{name: "minutes", input: "90m", want: 90 * time.Minute},
		{name: "seconds", input: "30s", want: 30 * time.Second},
		{name: "compound", input: "1h30m", want: 90 * time.Minute},
		{name: "empty", input: "", wantErr: true},
		{name: "days suffix only", input: "d", wantErr: true},
		{name: "fractional days", input: "1.5d", wantErr: true},
		{name: "unknown unit", input: "30days", wantErr: true},
		{name: "not a duration", input: "soon", wantErr: true},
		{name: "days out of range", input: "999999999d", wantErr: true},
		{name: "day and hour", input: "1d12h", wantErr: true},
		{name: "hour and day", input: "12h1d", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseDuration(test.input)
			if test.wantErr {
				if err == nil {
					t.Fatalf("parseDuration(%q) = %v, want error", test.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseDuration(%q) error = %v", test.input, err)
			}
			if got != test.want {
				t.Errorf("parseDuration(%q) = %v, want %v", test.input, got, test.want)
			}
		})
	}
}

func TestRateLimitModeWindow(t *testing.T) {
	t.Parallel()

	tests := []struct {
		mode RateLimitMode
		want time.Duration
	}{
		{mode: RateLimitSecond, want: time.Second},
		{mode: RateLimitMinute, want: time.Minute},
		{mode: RateLimitHour, want: time.Hour},
		{mode: RateLimitDay, want: 24 * time.Hour},
		{mode: RateLimitMode("week")},
		{mode: RateLimitMode("")},
	}

	for _, test := range tests {
		t.Run(string(test.mode), func(t *testing.T) {
			t.Parallel()

			if got := test.mode.Window(); got != test.want {
				t.Errorf("Window() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestLoadLogLevels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
		want  slog.Level
	}{
		{name: "debug", value: "debug", want: slog.LevelDebug},
		{name: "info", value: "info", want: slog.LevelInfo},
		{name: "warn", value: "warn", want: slog.LevelWarn},
		{name: "error", value: "error", want: slog.LevelError},
		{name: "padded", value: "  warn  ", want: slog.LevelWarn},
		{name: "unset falls back to info", value: "", want: slog.LevelInfo},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg, err := Load(lookupFrom(withStaticDir(t, map[string]string{"LOG_LEVEL": test.value})))
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if cfg.LogLevel != test.want {
				t.Errorf("LogLevel = %v, want %v", cfg.LogLevel, test.want)
			}
		})
	}
}

func TestLoadTrustedProxies(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
		want  []string
	}{
		{name: "unset", value: "", want: nil},
		{name: "whitespace only", value: "   ", want: nil},
		{name: "single block", value: "10.0.0.0/8", want: []string{"10.0.0.0/8"}},
		{name: "several blocks", value: "10.0.0.0/8, 192.168.1.0/24", want: []string{"10.0.0.0/8", "192.168.1.0/24"}},
		{name: "host bits are masked", value: "10.0.0.5/8", want: []string{"10.0.0.0/8"}},
		{name: "ipv6 host bits are masked", value: "2001:db8::1/32", want: []string{"2001:db8::/32"}},
		{name: "empty entries are skipped", value: "10.0.0.0/8,,192.168.1.0/24,", want: []string{"10.0.0.0/8", "192.168.1.0/24"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg, err := Load(lookupFrom(withStaticDir(t, map[string]string{"TRUSTED_PROXIES": test.value})))
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if len(cfg.TrustedProxies) != len(test.want) {
				t.Fatalf("TrustedProxies = %v, want %v", cfg.TrustedProxies, test.want)
			}
			for i, want := range test.want {
				if got := cfg.TrustedProxies[i].String(); got != want {
					t.Errorf("TrustedProxies[%d] = %q, want %q", i, got, want)
				}
			}
		})
	}
}

func TestVarErrorMessage(t *testing.T) {
	t.Parallel()

	cause := errors.New("boom")

	tests := []struct {
		name       string
		err        *varError
		want       string
		wantUnwrap error
	}{
		{
			name: "without a cause",
			err:  &varError{Variable: "PORT", Reason: "is required"},
			want: "config: PORT: is required",
		},
		{
			name:       "with a cause",
			err:        &varError{Variable: "PORT", Reason: "is required", Cause: cause},
			want:       "config: PORT: is required: boom",
			wantUnwrap: cause,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if got := test.err.Error(); got != test.want {
				t.Errorf("Error() = %q, want %q", got, test.want)
			}
			// wantUnwrap comes from the table, so the row without a cause
			// pins Unwrap to nil: errors.Is(err, nil) only holds for a nil err.
			if got := errors.Unwrap(test.err); !errors.Is(got, test.wantUnwrap) {
				t.Errorf("Unwrap() = %v, want %v", got, test.wantUnwrap)
			}
		})
	}
}
