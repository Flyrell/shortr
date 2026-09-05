package handlers

import (
	"bytes"
	"errors"
	"io/fs"
	"os"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/static"
)

const (
	assetsMaxAge = 3600
	// The client page carries absolute Open Graph URLs and only the server knows its origin.
	baseURLPlaceholder = "__BASE_URL__"
)

func Index(dir, baseURL string) fiber.Handler {
	return func(c fiber.Ctx) error {
		page, err := fs.ReadFile(os.DirFS(dir), "index.html")
		if errors.Is(err, fs.ErrNotExist) {
			return fiber.ErrNotFound
		}
		if err != nil {
			return err
		}
		c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
		return c.Send(bytes.ReplaceAll(page, []byte(baseURLPlaceholder), []byte(baseURL)))
	}
}

func File(path string) fiber.Handler {
	return static.New(path, static.Config{Browse: false})
}

func Assets(dir string) fiber.Handler {
	return static.New(dir, static.Config{Browse: false, MaxAge: assetsMaxAge})
}
