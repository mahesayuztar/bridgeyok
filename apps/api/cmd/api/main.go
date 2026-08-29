package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mahesayuztar/bridgeyok/apps/api/internal/config"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/database"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/httpapi"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/httpserver"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/observability"
)

func main() {
	appConfig, err := config.Load()
	if err != nil {
		slog.Error("invalid application configuration", "error", err)
		os.Exit(1)
	}

	logger := observability.NewLogger(appConfig.LogLevel)
	databaseCtx, cancelDatabase := context.WithTimeout(context.Background(), 10*time.Second)
	postgres, err := database.Open(databaseCtx, appConfig.DatabaseURL, appConfig.DatabaseMaxConns)
	cancelDatabase()
	if err != nil {
		logger.Error("database initialization failed", "error", err)
		os.Exit(1)
	}
	defer postgres.Close()

	handler := httpapi.NewRouter(httpapi.Options{
		Logger:         logger,
		AllowedOrigins: appConfig.AllowedOrigins,
		Readiness:      postgres,
	})
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := httpserver.Run(ctx, appConfig, handler, logger); err != nil {
		logger.Error("http server failed", "error", err)
		os.Exit(1)
	}
}
