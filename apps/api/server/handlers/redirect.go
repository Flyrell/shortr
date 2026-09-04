package handlers

import (
	"github.com/gofiber/fiber/v3"
)

func Redirect(shortener Shortener) fiber.Handler {
	return func(c fiber.Ctx) error {
		target, err := shortener.Resolve(c.Context(), c.Params("code"))
		if err != nil {
			return err
		}
		c.Set(fiber.HeaderCacheControl, "no-store")
		return c.Redirect().Status(fiber.StatusFound).To(target)
	}
}
