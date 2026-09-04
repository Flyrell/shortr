package middleware

import (
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/limiter"
)

func RateLimit(limit int, window time.Duration) fiber.Handler {
	return limiter.New(limiter.Config{
		Max:               limit,
		Expiration:        window,
		KeyGenerator:      func(c fiber.Ctx) string { return c.IP() },
		LimitReached:      func(_ fiber.Ctx) error { return fiber.ErrTooManyRequests },
		LimiterMiddleware: limiter.FixedWindow{},
	})
}
