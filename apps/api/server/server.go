package server

import (
	"log/slog"
	"net/netip"
	"path/filepath"
	"time"

	"github.com/gofiber/fiber/v3"
	recovery "github.com/gofiber/fiber/v3/middleware/recover"

	"github.com/Flyrell/shortr/apps/api/server/handlers"
	"github.com/Flyrell/shortr/apps/api/server/middleware"
)

const (
	bodyLimitHeadroom = 1024
	readTimeout       = 10 * time.Second
	writeTimeout      = 15 * time.Second
	idleTimeout       = 60 * time.Second
)

var rootFiles = []string{"robots.txt", "favicon.svg", "favicon.png", "apple-touch-icon.png", "og.png"}

type Deps struct {
	Logger          *slog.Logger
	Shortener       handlers.Shortener
	Adapter         handlers.Pinger
	BaseURL         string
	StaticDir       string
	URLMaxLength    int
	TrustedProxies  []netip.Prefix
	RateLimitWindow time.Duration
	RateLimitValue  int
}

func New(deps *Deps) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:       "shortr",
		StrictRouting: true,
		CaseSensitive: true,
		// The shorten body carries the target url inside a JSON object, so the
		// longest accepted url would be refused before validation ran if the
		// limit did not clear the url cap by the wrapper and its escaping.
		BodyLimit:    deps.URLMaxLength + bodyLimitHeadroom,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
		// Fiber hands request bytes out of a pool and reuses them once the
		// handler returns, so every value read from a request would have to be
		// copied by hand; this trades that footgun for one copy per read.
		Immutable:        true,
		TrustProxy:       len(deps.TrustedProxies) > 0,
		TrustProxyConfig: fiber.TrustProxyConfig{Proxies: proxyList(deps.TrustedProxies)},
		ProxyHeader:      fiber.HeaderXForwardedFor,
		// Without this Fiber hands the whole X-Forwarded-For header to c.IP()
		// instead of walking the chain, which lets a client forge its own key.
		EnableIPValidation: true,
		ErrorHandler:       handlers.ErrorHandler(deps.Logger),
	})

	// A panic unwinds past the request log, so the stack trace is the only
	// record left of what produced the 500.
	app.Use(recovery.New(recovery.Config{EnableStackTrace: true}))
	app.Use(middleware.SecurityHeaders())
	app.Use(middleware.RobotsTag())
	app.Use(middleware.RequestLogger(deps.Logger))

	app.Get("/healthz", handlers.Health(deps.Adapter))

	bots := middleware.BlockBots()
	app.Get("/", bots, handlers.Index(deps.StaticDir, deps.BaseURL))
	for _, name := range rootFiles {
		app.Get("/"+name, bots, handlers.File(filepath.Join(deps.StaticDir, name)))
	}
	app.Get("/assets/*", bots, handlers.Assets(filepath.Join(deps.StaticDir, "assets")))

	api := app.Group("/api", bots, middleware.RateLimit(deps.RateLimitValue, deps.RateLimitWindow))
	api.Post("/shorten", handlers.Shorten(deps.Shortener))

	app.Get("/:code", handlers.Redirect(deps.Shortener))

	return app
}

func proxyList(networks []netip.Prefix) []string {
	proxies := make([]string, len(networks))
	for i, network := range networks {
		proxies[i] = network.String()
	}
	return proxies
}
