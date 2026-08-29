//go:build integration

package database

import (
	"context"
	"os"
	"testing"
	"time"
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
