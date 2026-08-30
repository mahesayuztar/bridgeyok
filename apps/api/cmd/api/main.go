package main

import (
	"context"
	"crypto/rand"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mahesayuztar/bridgeyok/apps/api/internal/config"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/database"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/httpapi"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/httpserver"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/identity"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/observability"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/table"
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
	identityService, err := identity.NewService(postgres, appConfig.AuthSecret, rand.Reader, time.Now)
	if err != nil {
		logger.Error("identity initialization failed", "error", err)
		os.Exit(1)
	}
	tableService, err := table.NewService(postgres, appConfig.AuthSecret, rand.Reader, time.Now)
	if err != nil {
		logger.Error("table initialization failed", "error", err)
		os.Exit(1)
	}

	handler := httpapi.NewRouter(httpapi.Options{
		Logger:         logger,
		AllowedOrigins: appConfig.AllowedOrigins,
		Readiness:      postgres,
		Identity:       identityService,
		Table:          tableService,
	})
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := httpserver.Run(ctx, appConfig, handler, logger); err != nil {
		logger.Error("http server failed", "error", err)
		os.Exit(1)
	}
}
