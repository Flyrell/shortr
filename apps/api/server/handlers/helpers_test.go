package handlers

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v3"

	"github.com/Flyrell/shortr/apps/api/server/servertest"
)

func newApp() *fiber.App {
	return servertest.NewApp(ErrorHandler(slog.New(slog.NewTextHandler(io.Discard, nil))))
}

func decodeJSON[T any](t *testing.T, response *http.Response) T {
	t.Helper()

	var decoded T
	if err := json.NewDecoder(response.Body).Decode(&decoded); err != nil {
		t.Fatalf("decode %T error = %v", decoded, err)
	}
	return decoded
}
