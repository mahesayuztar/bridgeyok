//go:build integration

package database

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/mahesayuztar/bridgeyok/apps/api/internal/database/dbgen"
)

func TestPostgresPing(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("TEST_DATABASE_URL is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	postgres, err := Open(ctx, databaseURL, 2)
	if err != nil {
		t.Fatalf("Open() unexpected error: %v", err)
	}
	t.Cleanup(postgres.Close)

	if err := postgres.Ping(ctx); err != nil {
		t.Fatalf("Ping() unexpected error: %v", err)
	}
}

func TestSchemaReadinessRejectsMissingChat(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	postgres, err := Open(ctx, os.Getenv("TEST_DATABASE_URL"), 2)
	if err != nil {
		t.Fatal(err)
	}
	defer postgres.Close()
	tx, err := postgres.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, "ALTER TABLE bridgeyok.chat_messages RENAME TO chat_messages_readiness_test"); err != nil {
		t.Fatal(err)
	}
	ready, err := dbgen.New(tx).IsSchemaReady(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if ready {
		t.Fatal("schema without chat messages must not be ready")
	}
}
