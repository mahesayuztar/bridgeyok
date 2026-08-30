//go:build integration

package database

import (
	"context"
	"crypto/rand"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mahesayuztar/bridgeyok/apps/api/internal/identity"
)

func TestIdentityRepositoryCredentialAndTicketLifecycle(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("TEST_DATABASE_URL is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	postgres, err := Open(ctx, databaseURL, 2)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(postgres.Close)

	service, err := identity.NewService(postgres, []byte(strings.Repeat("integration-pepper-", 2)), rand.Reader, time.Now)
	if err != nil {
		t.Fatalf("identity.NewService() error = %v", err)
	}
	credentials, err := service.CreateSession(ctx, "Integration Guest")
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if _, cleanupErr := postgres.Pool().Exec(cleanupCtx, "DELETE FROM bridgeyok.guest_sessions WHERE id = $1", credentials.Session.ID); cleanupErr != nil {
			t.Errorf("cleanup guest session: %v", cleanupErr)
		}
	})

	refreshed, err := service.Refresh(ctx, credentials.DeviceCredential)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if _, err := service.Refresh(ctx, credentials.DeviceCredential); err == nil {
		t.Fatal("rotated credential remained valid")
	}
	ticket, _, err := service.IssueTicket(ctx, refreshed.Session)
	if err != nil {
		t.Fatalf("IssueTicket() error = %v", err)
	}
	if _, err := service.ConsumeTicket(ctx, ticket); err != nil {
		t.Fatalf("ConsumeTicket() error = %v", err)
	}
	if _, err := service.ConsumeTicket(ctx, ticket); err == nil {
		t.Fatal("ticket replay succeeded")
	}
	if err := service.Revoke(ctx, refreshed.Session.ID); err != nil {
		t.Fatalf("Revoke() error = %v", err)
	}
	if _, err := service.Authenticate(ctx, refreshed.AccessToken); err == nil {
		t.Fatal("revoked access token authenticated")
	}
}
