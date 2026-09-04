package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/Flyrell/shortr/apps/api/adapters"
	"github.com/Flyrell/shortr/apps/api/config"
	"github.com/Flyrell/shortr/apps/api/server"
	"github.com/Flyrell/shortr/apps/api/server/services"
)

const shutdownTimeout = 10 * time.Second

func main() {
	if err := run(); err != nil {
		slog.Error("shortr stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load(os.LookupEnv)
	if err != nil {
		return err
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(logger)

	adapter, err := adapters.New(cfg.Persister, os.LookupEnv)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := adapter.Close(); closeErr != nil {
			logger.Error("closing the adapter failed", "error", closeErr)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	app := server.New(&server.Deps{
		Logger:          logger,
		Shortener:       services.NewShortener(adapter, cfg.BaseURL, cfg.URLTTL),
		Adapter:         adapter,
		StaticDir:       cfg.StaticDir,
		TrustedProxies:  cfg.TrustedProxies,
		RateLimitWindow: cfg.RateLimitMode.Window(),
		RateLimitValue:  cfg.RateLimitValue,
	})
	logger.Info("starting shortr", "port", cfg.Port, "persister", cfg.Persister)

	return app.Listen(":"+strconv.Itoa(cfg.Port), fiber.ListenConfig{
		DisableStartupMessage: true,
		GracefulContext:       ctx,
		ShutdownTimeout:       shutdownTimeout,
	})
}
