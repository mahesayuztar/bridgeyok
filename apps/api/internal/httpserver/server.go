package httpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/mahesayuztar/bridgeyok/apps/api/internal/config"
)

func Run(ctx context.Context, appConfig config.Config, handler http.Handler, logger *slog.Logger) error {
	server := &http.Server{
		Addr:              appConfig.Address(),
		Handler:           handler,
		ReadHeaderTimeout: appConfig.ReadHeaderTimeout,
		ReadTimeout:       appConfig.ReadTimeout,
		WriteTimeout:      appConfig.WriteTimeout,
		IdleTimeout:       appConfig.IdleTimeout,
		MaxHeaderBytes:    1 << 20,
	}

	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.ListenAndServe()
	}()

	logger.Info("http server started", "address", appConfig.Address(), "environment", appConfig.Environment)
	select {
	case err := <-serverErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve HTTP: %w", err)
	case <-ctx.Done():
	}

	logger.Info("http server shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), appConfig.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		_ = server.Close()
		return fmt.Errorf("shutdown HTTP server: %w", err)
	}

	if err := <-serverErrors; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("stop HTTP server: %w", err)
	}
	logger.Info("http server stopped")
	return nil
}
