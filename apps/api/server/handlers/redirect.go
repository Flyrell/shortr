package handlers

import (
	"errors"
	"io/fs"
	"os"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/Flyrell/shortr/apps/api/server/services"
)

const notFoundPage = "not-found.html"

func Redirect(shortener Shortener, staticDir string) fiber.Handler {
	return func(c fiber.Ctx) error {
		target, err := shortener.Resolve(c.Context(), c.Params("code"))
		if err != nil {
			if errors.Is(err, services.ErrNotFound) {
				return notFound(c, staticDir)
			}
			return err
		}
		c.Set(fiber.HeaderCacheControl, "no-store")
		return c.Redirect().Status(fiber.StatusFound).To(target)
	}
}

// A browser gets the styled page; anything asking for JSON, or unable to be
// served the page, gets the same error shape as the rest of the API.
func notFound(c fiber.Ctx, staticDir string) error {
	if !strings.Contains(c.Get(fiber.HeaderAccept), fiber.MIMETextHTML) {
		return errNotFound
	}
	page, err := fs.ReadFile(os.DirFS(staticDir), notFoundPage)
	if err != nil {
		return errNotFound
	}
	c.Set(fiber.HeaderCacheControl, "no-store")
	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
	return c.Status(fiber.StatusNotFound).Send(page)
}
