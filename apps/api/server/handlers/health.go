package handlers

import (
	"context"
	"fmt"

	"github.com/gofiber/fiber/v3"
)

type Pinger interface {
	Ping(ctx context.Context) error
}

type healthResponse struct {
	Status string `json:"status"`
}

func Health(pinger Pinger) fiber.Handler {
	return func(c fiber.Ctx) error {
		if err := pinger.Ping(c.Context()); err != nil {
			return fmt.Errorf("%w: %w", errUnavailable, err)
		}
		return c.JSON(healthResponse{Status: "ok"})
	}
}
