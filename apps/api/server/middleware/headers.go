package middleware

import (
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/helmet"
)

const (
	contentSecurityPolicy = "default-src 'self'; img-src 'self' data: blob:; style-src 'self'; " +
		"script-src 'self'; connect-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'"
	robotsTag = "noindex, nofollow, noarchive"
)

func SecurityHeaders() fiber.Handler {
	return helmet.New(helmet.Config{
		ContentSecurityPolicy: contentSecurityPolicy,
		ContentTypeNosniff:    "nosniff",
		XFrameOptions:         "DENY",
		ReferrerPolicy:        "no-referrer",
	})
}

func RobotsTag() fiber.Handler {
	return func(c fiber.Ctx) error {
		c.Set(fiber.HeaderXRobotsTag, robotsTag)
		return c.Next()
	}
}
