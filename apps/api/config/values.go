package config

import (
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type varError struct {
	Variable string
	Reason   string
	Cause    error
}

func (e *varError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("config: %s: %s: %v", e.Variable, e.Reason, e.Cause)
	}
	return fmt.Sprintf("config: %s: %s", e.Variable, e.Reason)
}

func (e *varError) Unwrap() error { return e.Cause }

func newVarError(variable, format string, args ...any) *varError {
	return &varError{Variable: variable, Reason: fmt.Sprintf(format, args...)}
}

func wrapVarError(cause error, variable, format string, args ...any) *varError {
	err := newVarError(variable, format, args...)
	err.Cause = cause
	return err
}

func stringVar(lookup Lookup, name, fallback string) string {
	if value, ok := lookup(name); ok {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return fallback
}

func boundedIntVar(lookup Lookup, name string, fallback, minimum, maximum int) (int, error) {
	raw := stringVar(lookup, name, "")
	if raw == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return 0, newVarError(name, "must be an integer, got %q", raw)
	}
	if parsed < minimum || parsed > maximum {
		return 0, newVarError(name, "must be between %d and %d, got %d", minimum, maximum, parsed)
	}
	return parsed, nil
}

type RateLimitMode string

const (
	RateLimitSecond RateLimitMode = "second"
	RateLimitMinute RateLimitMode = "minute"
	RateLimitHour   RateLimitMode = "hour"
	RateLimitDay    RateLimitMode = "day"
)

func (m RateLimitMode) Window() time.Duration {
	switch m {
	case RateLimitSecond:
		return time.Second
	case RateLimitMinute:
		return time.Minute
	case RateLimitHour:
		return time.Hour
	case RateLimitDay:
		return 24 * time.Hour
	default:
		return 0
	}
}

func rateLimitModeVar(lookup Lookup, name string, fallback RateLimitMode) (RateLimitMode, error) {
	mode := RateLimitMode(stringVar(lookup, name, string(fallback)))
	if mode.Window() <= 0 {
		return "", newVarError(name, "must be one of second, minute, hour, day, got %q", mode)
	}
	return mode, nil
}

var logLevels = map[string]slog.Level{
	"debug": slog.LevelDebug,
	"info":  slog.LevelInfo,
	"warn":  slog.LevelWarn,
	"error": slog.LevelError,
}

func logLevelVar(lookup Lookup, name, fallback string) (slog.Level, error) {
	raw := stringVar(lookup, name, fallback)
	level, ok := logLevels[raw]
	if !ok {
		return 0, newVarError(name, "must be one of debug, info, warn, error, got %q", raw)
	}
	return level, nil
}

func baseURLVar(lookup Lookup, name, fallback string) (string, error) {
	raw := stringVar(lookup, name, fallback)
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", wrapVarError(err, name, "must be a valid URL, got %q", raw)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", newVarError(name, "must use the http or https scheme, got %q", raw)
	}
	if parsed.Host == "" {
		return "", newVarError(name, "must include a host, got %q", raw)
	}
	return strings.TrimRight(raw, "/"), nil
}

// Every block is masked, so 10.0.0.5/8 is stored as 10.0.0.0/8 and matches.
func cidrListVar(lookup Lookup, name string) ([]netip.Prefix, error) {
	raw, ok := lookup(name)
	if !ok || strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	networks := make([]netip.Prefix, 0, len(parts))
	for _, part := range parts {
		entry := strings.TrimSpace(part)
		if entry == "" {
			continue
		}
		network, err := netip.ParsePrefix(entry)
		if err != nil {
			return nil, wrapVarError(err, name, "must be a comma separated list of CIDR blocks, got %q", entry)
		}
		networks = append(networks, network.Masked())
	}
	return networks, nil
}

const maxDurationDays = int(math.MaxInt64 / int64(24*time.Hour))

func ttlVar(lookup Lookup, name, fallback string) (time.Duration, error) {
	raw := stringVar(lookup, name, fallback)
	ttl, err := parseDuration(raw)
	if err != nil {
		return 0, newVarError(name, "must be a duration such as 30d, 12h or 90m, got %q", raw)
	}
	if ttl <= 0 {
		return 0, newVarError(name, "must be greater than zero, got %q", raw)
	}
	return ttl, nil
}

func parseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, errors.New("must not be empty")
	}
	digits, isDays := strings.CutSuffix(s, "d")
	if !isDays {
		return time.ParseDuration(s)
	}
	days, err := strconv.Atoi(digits)
	if err != nil {
		return 0, fmt.Errorf("invalid day count %q", digits)
	}
	if days > maxDurationDays || days < -maxDurationDays {
		return 0, fmt.Errorf("day count %d is out of range", days)
	}
	return time.Duration(days) * 24 * time.Hour, nil
}

func dirVar(lookup Lookup, name, fallback string) (string, error) {
	path := stringVar(lookup, name, fallback)
	info, err := os.Stat(path)
	if err != nil {
		return "", wrapVarError(err, name, "must be an existing directory, got %q", path)
	}
	if !info.IsDir() {
		return "", newVarError(name, "must be a directory, got %q", path)
	}
	return path, nil
}
