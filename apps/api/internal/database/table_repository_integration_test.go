//go:build integration

package database

import (
	"context"
	"crypto/rand"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mahesayuztar/bridgeyok/apps/api/internal/identity"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/table"
)

func TestTableRepositoryLifecycleAndJoinCapacity(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("TEST_DATABASE_URL is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	postgres, err := Open(ctx, databaseURL, 5)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(postgres.Close)
	identityService, err := identity.NewService(postgres, []byte(strings.Repeat("identity-integration-pepper", 2)), rand.Reader, time.Now)
	if err != nil {
		t.Fatalf("identity.NewService() error = %v", err)
	}
	tableService, err := table.NewService(postgres, []byte(strings.Repeat("table-integration-pepper", 2)), rand.Reader, time.Now)
	if err != nil {
		t.Fatalf("table.NewService() error = %v", err)
	}

	sessions := make([]identity.Session, 5)
	for _index := range sessions {
		credentials, createErr := identityService.CreateSession(ctx, "Guest "+string(rune('A'+_index)))
		if createErr != nil {
			t.Fatalf("CreateSession(%d) error = %v", _index, createErr)
		}
		sessions[_index] = credentials.Session
	}
	created, err := tableService.Create(ctx, sessions[0])
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if _, cleanupErr := postgres.Pool().Exec(cleanupCtx, "DELETE FROM bridgeyok.tables WHERE id = $1", created.Projection.TableID); cleanupErr != nil {
			t.Errorf("cleanup table: %v", cleanupErr)
		}
		for _, session := range sessions {
			if _, cleanupErr := postgres.Pool().Exec(cleanupCtx, "DELETE FROM bridgeyok.guest_sessions WHERE id = $1", session.ID); cleanupErr != nil {
				t.Errorf("cleanup guest session: %v", cleanupErr)
			}
		}
	})

	preview, err := tableService.Preview(ctx, strings.ToLower(created.InviteCode))
	if err != nil {
		t.Fatalf("Preview() error = %v", err)
	}
	if preview.ParticipantCount != 1 || preview.Capacity != 4 {
		t.Fatalf("initial preview = %+v", preview)
	}

	type joinResult struct {
		projection table.Projection
		err        error
	}
	results := make(chan joinResult, 4)
	var waitGroup sync.WaitGroup
	for _, session := range sessions[1:] {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			projection, joinErr := tableService.Join(ctx, created.InviteCode, session)
			results <- joinResult{projection: projection, err: joinErr}
		}()
	}
	waitGroup.Wait()
	close(results)

	joined := make([]identity.Session, 0, 3)
	successCount := 0
	fullCount := 0
	for result := range results {
		if result.err == nil {
			successCount++
			for _, session := range sessions[1:] {
				if result.projection.ViewerParticipantID != "" {
					projection, getErr := tableService.Get(ctx, created.Projection.TableID, session)
					if getErr == nil && projection.ViewerParticipantID == result.projection.ViewerParticipantID {
						joined = append(joined, session)
						break
					}
				}
			}
			continue
		}
		var domainError *table.DomainError
		if errors.As(result.err, &domainError) && domainError.Code == table.ErrorTableFull {
			fullCount++
			continue
		}
		t.Fatalf("Join() unexpected error = %v", result.err)
	}
	if successCount != 3 || fullCount != 1 || len(joined) != 3 {
		t.Fatalf("join results: success=%d full=%d identified=%d", successCount, fullCount, len(joined))
	}

	if err := tableService.Leave(ctx, created.Projection.TableID, joined[0]); err != nil {
		t.Fatalf("Leave() error = %v", err)
	}
	preview, err = tableService.Preview(ctx, created.InviteCode)
	if err != nil {
		t.Fatalf("Preview() after leave error = %v", err)
	}
	if preview.ParticipantCount != 3 {
		t.Fatalf("participant count after leave = %d, want 3", preview.ParticipantCount)
	}

	ownerLeaveError := tableService.Leave(ctx, created.Projection.TableID, sessions[0])
	var domainError *table.DomainError
	if !errors.As(ownerLeaveError, &domainError) || domainError.Code != table.ErrorOwnerCannotLeave {
		t.Fatalf("owner Leave() error = %v, want %s", ownerLeaveError, table.ErrorOwnerCannotLeave)
	}
}
