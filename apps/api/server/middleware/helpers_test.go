package middleware

import (
	"github.com/gofiber/fiber/v3"
)

// app.Test connects from 0.0.0.0, so trusting that address is what makes Fiber
// read the X-Forwarded-For headers these tests send.
const testProxy = "0.0.0.0/32"

func ok(c fiber.Ctx) error { return c.SendString("ok") }
