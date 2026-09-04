package handlers

import (
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/static"
)

const assetsMaxAge = 3600

func File(path string) fiber.Handler {
	return static.New(path, static.Config{Browse: false})
}

func Assets(dir string) fiber.Handler {
	return static.New(dir, static.Config{Browse: false, MaxAge: assetsMaxAge})
}
