package httpserver

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/mahesayuztar/bridgeyok/apps/api/internal/config"
)

func TestRunStopsOnCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	appConfig := config.Config{
		Environment:       "test",
		Host:              "127.0.0.1",
		Port:              0,
		ReadHeaderTimeout: time.Second,
		ReadTimeout:       time.Second,
		WriteTimeout:      time.Second,
		IdleTimeout:       time.Second,
		ShutdownTimeout:   time.Second,
	}
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	if err := Run(ctx, appConfig, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), logger); err != nil {
		t.Fatalf("Run() unexpected error: %v", err)
	}
}
