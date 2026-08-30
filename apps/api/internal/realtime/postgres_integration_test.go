//go:build integration

package realtime

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mahesayuztar/bridgeyok/apps/api/internal/database"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/identity"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/table"
)

func TestPostgresWebSocketCommandAndResume(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("TEST_DATABASE_URL is required")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	postgres, err := database.Open(ctx, databaseURL, 4)
	if err != nil {
		t.Fatalf("database.Open() error = %v", err)
	}
	t.Cleanup(postgres.Close)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	identityService, err := identity.NewService(postgres, []byte(strings.Repeat("realtime-integration-pepper", 2)), rand.Reader, time.Now)
	if err != nil {
		t.Fatalf("identity.NewService() error = %v", err)
	}
	tableService, err := table.NewService(postgres, []byte(strings.Repeat("table-integration-pepper", 2)), rand.Reader, time.Now)
	if err != nil {
		t.Fatalf("table.NewService() error = %v", err)
	}
	credentials, err := identityService.CreateSession(ctx, "Realtime Owner")
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	created, err := tableService.Create(ctx, credentials.Session)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if _, cleanupErr := postgres.Pool().Exec(cleanupCtx, "DELETE FROM bridgeyok.tables WHERE id = $1", created.Projection.TableID); cleanupErr != nil {
			t.Errorf("cleanup table: %v", cleanupErr)
		}
		if _, cleanupErr := postgres.Pool().Exec(cleanupCtx, "DELETE FROM bridgeyok.guest_sessions WHERE id = $1", credentials.Session.ID); cleanupErr != nil {
			t.Errorf("cleanup guest session: %v", cleanupErr)
		}
	})
	processor, err := table.NewCommandProcessor(postgres, nil, logger, time.Now)
	if err != nil {
		t.Fatalf("NewCommandProcessor() error = %v", err)
	}
	actors, err := table.NewActorRegistry(postgres, processor, table.ActorRegistryOptions{
		QueueCapacity: 8, IdleTimeout: time.Hour, Logger: logger, Now: time.Now,
	})
	if err != nil {
		t.Fatalf("NewActorRegistry() error = %v", err)
	}
	t.Cleanup(func() {
		drainCtx, drainCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer drainCancel()
		if drainErr := actors.Drain(drainCtx); drainErr != nil {
			t.Errorf("Drain() error = %v", drainErr)
		}
	})
	realtimeServer, err := NewServer(Options{
		Logger: logger, AllowedOrigins: []string{realtimeOrigin}, Identity: identityService, Tables: actors, Events: postgres,
		Random: rand.Reader, Now: time.Now, ReadLimitBytes: 8 << 10, OutboundQueueCapacity: 32, OutboundQueueBytes: 128 << 10,
		WriteTimeout: time.Second, PingInterval: time.Hour, PongTimeout: time.Second,
		PresenceGracePeriod: time.Hour,
		MaxConnections:      8, MaxConnectionsPerSession: 3, MessageRate: 100, MessageBurst: 100, RecoveryLimit: 16,
	})
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	httpServer := httptest.NewServer(realtimeServer)
	defer httpServer.Close()

	firstTicket, _, err := identityService.IssueTicket(ctx, credentials.Session)
	if err != nil {
		t.Fatalf("IssueTicket() error = %v", err)
	}
	first := mustDialScripted(t, httpServer.URL, firstTicket)
	writeScripted(t, first, map[string]any{
		"v": 1, "kind": "command", "name": "table.subscribe", "request_id": "subscribe_db_01",
		"table_id": created.Projection.TableID, "payload": map[string]any{"last_seen_seq": 0},
	})
	readScripted(t, first)
	readScripted(t, first)
	readScripted(t, first)
	writeScripted(t, first, map[string]any{
		"v": 1, "kind": "command", "name": "table.lock", "request_id": "lock_db_0001",
		"table_id": created.Projection.TableID, "expected_revision": 0, "payload": map[string]any{"locked": true},
	})
	ack, _ := readScripted(t, first)
	event, _ := readScripted(t, first)
	if ack.Kind != "ack" || ack.Revision != 1 || event.Kind != "event" || event.Seq != 1 {
		t.Fatalf("durable mutation frames = %+v then %+v", ack, event)
	}
	closeClient(t, first)

	secondTicket, _, err := identityService.IssueTicket(ctx, credentials.Session)
	if err != nil {
		t.Fatalf("second IssueTicket() error = %v", err)
	}
	second := mustDialScripted(t, httpServer.URL, secondTicket)
	defer closeClient(t, second)
	writeScripted(t, second, map[string]any{
		"v": 1, "kind": "command", "name": "table.resume", "request_id": "resume_db_001",
		"table_id": created.Projection.TableID, "payload": map[string]any{"last_seen_seq": 0},
	})
	resumeAck, _ := readScripted(t, second)
	resumedEvent, _ := readScripted(t, second)
	var payload struct {
		SyncMode string `json:"syncMode"`
	}
	if err := json.Unmarshal(resumeAck.Payload, &payload); err != nil {
		t.Fatalf("decode resume ack: %v", err)
	}
	if payload.SyncMode != "events" || resumedEvent.Kind != "event" || resumedEvent.Seq != 1 {
		t.Fatalf("resume frames = %+v then %+v", resumeAck, resumedEvent)
	}
}
