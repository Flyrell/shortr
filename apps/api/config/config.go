package config

import (
	"log/slog"
	"math"
	"net/netip"
	"time"
)

const (
	defaultPort           = 8080
	defaultBaseURL        = "http://localhost:8080"
	defaultStaticDir      = "./apps/client/dist"
	defaultURLTTL         = "30d"
	defaultURLMaxLength   = 4096
	defaultPersister      = "memory"
	defaultRateLimitValue = 30
	defaultLogLevel       = "info"
	defaultRateLimitMode  = RateLimitDay
)

type Lookup func(key string) (string, bool)

type Config struct {
	Port           int
	BaseURL        string
	StaticDir      string
	URLTTL         time.Duration
	URLMaxLength   int
	Persister      string
	RateLimitMode  RateLimitMode
	RateLimitValue int
	TrustedProxies []netip.Prefix
	LogLevel       slog.Level
}

func Load(lookup Lookup) (Config, error) {
	var (
		cfg Config
		err error
	)
	if cfg.StaticDir, err = dirVar(lookup, "STATIC_DIR", defaultStaticDir); err != nil {
		return Config{}, err
	}
	if cfg.Port, err = boundedIntVar(lookup, "PORT", defaultPort, 1, 65535); err != nil {
		return Config{}, err
	}
	if cfg.BaseURL, err = baseURLVar(lookup, "BASE_URL", defaultBaseURL); err != nil {
		return Config{}, err
	}
	if cfg.URLTTL, err = ttlVar(lookup, "URL_TTL", defaultURLTTL); err != nil {
		return Config{}, err
	}
	if cfg.URLMaxLength, err = boundedIntVar(lookup, "URL_MAX_LENGTH", defaultURLMaxLength, 256, 65536); err != nil {
		return Config{}, err
	}
	// The set of adapter names belongs to the adapters package, which validates
	// it, so any non-empty name is accepted here.
	cfg.Persister = stringVar(lookup, "URL_PERSISTER", defaultPersister)
	if cfg.RateLimitMode, err = rateLimitModeVar(lookup, "RATE_LIMIT_MODE", defaultRateLimitMode); err != nil {
		return Config{}, err
	}
	if cfg.RateLimitValue, err = boundedIntVar(lookup, "RATE_LIMIT_VALUE", defaultRateLimitValue, 1, math.MaxInt); err != nil {
		return Config{}, err
	}
	if cfg.TrustedProxies, err = cidrListVar(lookup, "TRUSTED_PROXIES"); err != nil {
		return Config{}, err
	}
	if cfg.LogLevel, err = logLevelVar(lookup, "LOG_LEVEL", defaultLogLevel); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
