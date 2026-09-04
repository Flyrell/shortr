package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v3"
)

var botTokens = []string{
	"googlebot", "bingbot", "slurp", "duckduckbot", "baiduspider", "yandex", "sogou", "exabot",
	"facebookexternalhit", "facebot", "twitterbot", "linkedinbot", "pinterest", "ahrefsbot",
	"semrushbot", "mj12bot", "dotbot", "petalbot", "applebot", "ia_archiver", "gptbot",
	"chatgpt-user", "claudebot", "anthropic-ai", "ccbot", "bytespider", "perplexitybot",
	"amazonbot", "headlesschrome", "phantomjs", "python-requests", "scrapy", "wget", "curl",
}

func BlockBots() fiber.Handler {
	return func(c fiber.Ctx) error {
		agent := strings.ToLower(c.Get(fiber.HeaderUserAgent))
		for _, token := range botTokens {
			if strings.Contains(agent, token) {
				return fiber.ErrForbidden
			}
		}
		return c.Next()
	}
}
