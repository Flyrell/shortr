package middleware

import (
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"
)

func RequestLogger(logger *slog.Logger) fiber.Handler {
	return func(c fiber.Ctx) error {
		start := time.Now()
		// Fiber runs the error handler only after the whole chain has unwound,
		// so it is called here to log the status the client actually receives;
		// nothing is returned afterwards so Fiber never handles it a second time.
		if err := c.Next(); err != nil {
			if handlerErr := c.App().ErrorHandler(c, err); handlerErr != nil {
				logger.LogAttrs(c.Context(), slog.LevelError, "the error handler failed",
					slog.String("error", handlerErr.Error()),
				)
			}
		}
		logger.LogAttrs(c.Context(), slog.LevelInfo, "request",
			slog.String("method", c.Method()),
			slog.String("path", c.Path()),
			slog.Int("status", c.Response().StatusCode()),
			slog.Duration("duration", time.Since(start)),
			slog.String("ip", c.IP()),
		)
		return nil
	}
}
