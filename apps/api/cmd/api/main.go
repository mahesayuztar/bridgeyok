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
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/realtime"
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
	commandProcessor, err := table.NewCommandProcessor(postgres, nil, logger, time.Now)
	if err != nil {
		logger.Error("command processor initialization failed", "error", err)
		os.Exit(1)
	}
	actorRegistry, err := table.NewActorRegistry(postgres, commandProcessor, table.ActorRegistryOptions{
		QueueCapacity: appConfig.TableActorQueueCapacity,
		IdleTimeout:   appConfig.TableActorIdleTimeout,
		Logger:        logger,
		Now:           time.Now,
	})
	if err != nil {
		logger.Error("table actor initialization failed", "error", err)
		os.Exit(1)
	}
	realtimeServer, err := realtime.NewServer(realtime.Options{
		Logger:                   logger,
		AllowedOrigins:           appConfig.AllowedOrigins,
		Identity:                 identityService,
		Tables:                   actorRegistry,
		Events:                   postgres,
		Random:                   rand.Reader,
		Now:                      time.Now,
		ReadLimitBytes:           appConfig.RealtimeReadLimitBytes,
		OutboundQueueCapacity:    appConfig.RealtimeOutboundQueueCapacity,
		OutboundQueueBytes:       appConfig.RealtimeOutboundQueueBytes,
		WriteTimeout:             appConfig.RealtimeWriteTimeout,
		PingInterval:             appConfig.RealtimePingInterval,
		PongTimeout:              appConfig.RealtimePongTimeout,
		PresenceGracePeriod:      appConfig.RealtimePresenceGracePeriod,
		MaxConnections:           appConfig.RealtimeMaxConnections,
		MaxConnectionsPerSession: appConfig.RealtimeMaxConnectionsPerSession,
		MessageRate:              appConfig.RealtimeMessageRate,
		MessageBurst:             appConfig.RealtimeMessageBurst,
		RecoveryLimit:            appConfig.RealtimeRecoveryLimit,
	})
	if err != nil {
		logger.Error("realtime initialization failed", "error", err)
		os.Exit(1)
	}

	handler := httpapi.NewRouter(httpapi.Options{
		Logger:         logger,
		AllowedOrigins: appConfig.AllowedOrigins,
		Readiness:      postgres,
		Identity:       identityService,
		Table:          tableService,
		Realtime:       realtimeServer,
	})
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	serverErr := httpserver.Run(ctx, appConfig, handler, logger, realtimeServer.Drain)
	drainCtx, cancelDrain := context.WithTimeout(context.Background(), appConfig.ShutdownTimeout)
	drainErr := actorRegistry.Drain(drainCtx)
	cancelDrain()
	if serverErr != nil {
		logger.Error("http server failed", "error", serverErr)
		os.Exit(1)
	}
	if drainErr != nil {
		logger.Error("table actor drain failed", "error", drainErr)
		os.Exit(1)
	}
}
