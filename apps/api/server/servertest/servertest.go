package servertest

import (
	"context"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v3"

	"github.com/Flyrell/shortr/apps/api/server/services"
)

const KnownCode = "abc1234defgh"

type StubShortener struct {
	Short      services.ShortURL
	ShortenErr error
	Target     string
	ResolveErr error
	GotURL     string
	GotCode    string
}

func (s *StubShortener) Shorten(_ context.Context, rawURL string) (services.ShortURL, error) {
	s.GotURL = rawURL
	return s.Short, s.ShortenErr
}

func (s *StubShortener) Resolve(_ context.Context, code string) (string, error) {
	s.GotCode = code
	if s.ResolveErr != nil {
		return "", s.ResolveErr
	}
	if code != KnownCode {
		return "", services.ErrNotFound
	}
	return s.Target, nil
}

type StubPinger struct {
	Err error
}

func (p StubPinger) Ping(context.Context) error { return p.Err }

func NewApp(errorHandler fiber.ErrorHandler, trustedProxies ...string) *fiber.App {
	return fiber.New(fiber.Config{
		StrictRouting:      true,
		CaseSensitive:      true,
		Immutable:          true,
		TrustProxy:         len(trustedProxies) > 0,
		TrustProxyConfig:   fiber.TrustProxyConfig{Proxies: trustedProxies},
		ProxyHeader:        fiber.HeaderXForwardedFor,
		EnableIPValidation: true,
		ErrorHandler:       errorHandler,
	})
}

func Do(t *testing.T, app *fiber.App, request *http.Request) *http.Response {
	t.Helper()

	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("Test() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := response.Body.Close(); closeErr != nil {
			t.Errorf("Close() error = %v", closeErr)
		}
	})
	return response
}
