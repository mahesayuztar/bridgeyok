package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/database/dbgen"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/deal"
)

type Postgres struct {
	pool       *pgxpool.Pool
	queries    *dbgen.Queries
	dealSource deal.Source
}

func Open(ctx context.Context, databaseURL string, maxConns int32, sources ...deal.Source) (*Postgres, error) {
	var source deal.Source = deal.SecureRandom{}
	if len(sources) > 1 {
		return nil, fmt.Errorf("only one deal source is supported")
	}
	if len(sources) == 1 {
		if sources[0] == nil {
			return nil, fmt.Errorf("deal source is required")
		}
		source = sources[0]
	}
	poolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse PostgreSQL configuration")
	}
	poolConfig.MaxConns = maxConns
	poolConfig.MinConns = 0
	poolConfig.MaxConnIdleTime = 5 * time.Minute
	poolConfig.MaxConnLifetime = 30 * time.Minute
	poolConfig.HealthCheckPeriod = time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create PostgreSQL pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("connect to PostgreSQL: %w", err)
	}
	return &Postgres{pool: pool, queries: dbgen.New(pool), dealSource: source}, nil
}

func (postgres *Postgres) Ping(ctx context.Context) error {
	ready, err := postgres.queries.IsSchemaReady(ctx)
	if err != nil {
		return fmt.Errorf("check database schema readiness: %w", err)
	}
	if !ready {
		return errors.New("database schema is not migrated")
	}
	return nil
}

func (postgres *Postgres) Close() {
	postgres.pool.Close()
}

func (postgres *Postgres) Pool() *pgxpool.Pool {
	return postgres.pool
}
