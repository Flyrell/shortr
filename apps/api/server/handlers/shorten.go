package handlers

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/Flyrell/shortr/apps/api/server/services"
)

type Shortener interface {
	Shorten(ctx context.Context, rawURL string) (services.ShortURL, error)
	Resolve(ctx context.Context, code string) (string, error)
}

type shortenRequest struct {
	URL string `json:"url"`
}

type shortenResponse struct {
	Code      string `json:"code"`
	ShortURL  string `json:"shortUrl"`
	ExpiresAt string `json:"expiresAt"`
}

func Shorten(shortener Shortener) fiber.Handler {
	return func(c fiber.Ctx) error {
		if !c.Is("json") {
			return errUnsupportedMediaType
		}
		var request shortenRequest
		if err := c.Bind().JSON(&request); err != nil {
			return errInvalidBody
		}
		if request.URL == "" {
			return errInvalidBody
		}
		short, err := shortener.Shorten(c.Context(), request.URL)
		if err != nil {
			return err
		}
		return c.Status(fiber.StatusCreated).JSON(shortenResponse{
			Code:      short.Code,
			ShortURL:  short.ShortURL,
			ExpiresAt: short.ExpiresAt.Format(time.RFC3339),
		})
	}
}
