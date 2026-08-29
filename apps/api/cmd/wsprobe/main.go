package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/mahesayuztar/bridgeyok/apps/api/internal/config"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/httpserver"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/observability"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/wsprobe"
)

func main() {
	logger := observability.NewLogger(slog.LevelInfo)
	allowedOrigin := strings.TrimSpace(os.Getenv("WS_PROBE_ALLOWED_ORIGIN"))
	if allowedOrigin == "" {
		logger.Error("invalid websocket probe configuration", "error", "WS_PROBE_ALLOWED_ORIGIN is required")
		os.Exit(1)
	}
	port := 8090
	if rawPort := strings.TrimSpace(os.Getenv("PORT")); rawPort != "" {
		parsedPort, err := strconv.Atoi(rawPort)
		if err != nil || parsedPort < 1 || parsedPort > 65535 {
			logger.Error("invalid websocket probe configuration", "error", "PORT must be an integer between 1 and 65535")
			os.Exit(1)
		}
		port = parsedPort
	}

	handler, err := wsprobe.NewHandler(allowedOrigin, logger)
	if err != nil {
		logger.Error("invalid websocket probe configuration", "error", err)
		os.Exit(1)
	}
	appConfig := config.Config{
		Environment:       "test",
		Host:              "0.0.0.0",
		Port:              port,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		ShutdownTimeout:   10 * time.Second,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := httpserver.Run(ctx, appConfig, handler, logger); err != nil {
		logger.Error("websocket probe failed", "error", err)
		os.Exit(1)
	}
}
