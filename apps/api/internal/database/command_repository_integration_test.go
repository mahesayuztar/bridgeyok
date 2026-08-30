//go:build integration

package database

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"log/slog"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/bridge"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/database/dbgen"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/identity"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/observability"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/table"
)

type commandTestEnvironment struct {
	ctx          context.Context
	postgres     *Postgres
	tableService *table.Service
	processor    *table.CommandProcessor
	sessions     []identity.Session
	tableID      string
	logs         *lockedBuffer
}

type lockedBuffer struct {
	mutex   sync.Mutex
	builder strings.Builder
}

func (buffer *lockedBuffer) Write(value []byte) (int, error) {
	buffer.mutex.Lock()
	defer buffer.mutex.Unlock()
	return buffer.builder.Write(value)
}

func (buffer *lockedBuffer) String() string {
	buffer.mutex.Lock()
	defer buffer.mutex.Unlock()
	return buffer.builder.String()
}

func TestCommandRepositoryOrderingIdempotencyAndHydrate(t *testing.T) {
	environment := newCommandTestEnvironment(t, 2)
	aggregate, err := environment.postgres.FindTable(environment.ctx, environment.tableID)
	if err != nil {
		t.Fatalf("FindTable() error = %v", err)
	}
	if aggregate.Revision != 1 || aggregate.LastSeq != 1 {
		t.Fatalf("join revision/seq = %d/%d, want 1/1", aggregate.Revision, aggregate.LastSeq)
	}

	takeSeat := environment.process(t, table.CommandRequest{
		TableID: environment.tableID, SessionID: environment.sessions[0].ID, RequestID: "take_seat_01",
		ExpectedRevision: aggregate.Revision, Command: table.Command{Name: table.CommandTakeSeat, Seat: bridge.North},
	})
	if takeSeat.Outcome.Status != table.CommandStatusAccepted || takeSeat.Outcome.Revision != 2 || takeSeat.Outcome.FirstSeq != 2 || takeSeat.Outcome.LastSeq != 2 {
		t.Fatalf("take seat outcome = %+v", takeSeat.Outcome)
	}
	recoveryHash := bytes.Repeat([]byte{0x5a}, 32)
	if _, err := environment.postgres.Pool().Exec(environment.ctx,
		"UPDATE bridgeyok.table_seats SET recovery_hash = $1 WHERE table_id = $2 AND participant_id = $3",
		recoveryHash, environment.tableID, takeSeat.Aggregate.Seats[bridge.North].ParticipantID,
	); err != nil {
		t.Fatalf("seed recovery hash: %v", err)
	}

	duplicate := environment.process(t, table.CommandRequest{
		TableID: environment.tableID, SessionID: environment.sessions[0].ID, RequestID: "take_seat_01",
		ExpectedRevision: 1, Command: table.Command{Name: table.CommandTakeSeat, Seat: bridge.North},
	})
	if !duplicate.Duplicate || duplicate.Outcome != takeSeat.Outcome || len(duplicate.Events) != 0 {
		t.Fatalf("duplicate result = %+v", duplicate)
	}

	stale := environment.process(t, table.CommandRequest{
		TableID: environment.tableID, SessionID: environment.sessions[0].ID, RequestID: "stale_ready_01",
		ExpectedRevision: 1, Command: table.Command{Name: table.CommandSetReady, Ready: true},
	})
	if stale.Outcome.Status != table.CommandStatusRejected || stale.Outcome.ErrorCode != table.ErrorStateChanged || stale.Outcome.Revision != 2 {
		t.Fatalf("stale outcome = %+v", stale.Outcome)
	}

	seatTaken := environment.process(t, table.CommandRequest{
		TableID: environment.tableID, SessionID: environment.sessions[1].ID, RequestID: "seat_taken_01",
		ExpectedRevision: 2, Command: table.Command{Name: table.CommandTakeSeat, Seat: bridge.North},
	})
	if seatTaken.Outcome.Status != table.CommandStatusRejected || seatTaken.Outcome.ErrorCode != table.ErrorSeatTaken {
		t.Fatalf("seat-taken outcome = %+v", seatTaken.Outcome)
	}

	ready := environment.process(t, table.CommandRequest{
		TableID: environment.tableID, SessionID: environment.sessions[0].ID, RequestID: "set_ready_01",
		ExpectedRevision: 2, Command: table.Command{Name: table.CommandSetReady, Ready: true},
	})
	if ready.Outcome.Status != table.CommandStatusAccepted || ready.Outcome.Revision != 3 || ready.Outcome.LastSeq != 3 {
		t.Fatalf("ready outcome = %+v", ready.Outcome)
	}
	var persistedRecoveryHash []byte
	if err := environment.postgres.Pool().QueryRow(environment.ctx,
		"SELECT recovery_hash FROM bridgeyok.table_seats WHERE table_id = $1 AND participant_id = $2",
		environment.tableID, ready.Aggregate.Seats[bridge.North].ParticipantID,
	).Scan(&persistedRecoveryHash); err != nil {
		t.Fatalf("read recovery hash: %v", err)
	}
	if !bytes.Equal(persistedRecoveryHash, recoveryHash) {
		t.Fatal("seat sync did not preserve recovery hash")
	}

	staleDuplicate := environment.process(t, table.CommandRequest{
		TableID: environment.tableID, SessionID: environment.sessions[0].ID, RequestID: "stale_ready_01",
		ExpectedRevision: 3, Command: table.Command{Name: table.CommandSetReady, Ready: true},
	})
	if !staleDuplicate.Duplicate || staleDuplicate.Outcome != stale.Outcome || staleDuplicate.Aggregate.Revision != 3 {
		t.Fatalf("stale duplicate = %+v", staleDuplicate)
	}

	restarted, err := Open(environment.ctx, os.Getenv("TEST_DATABASE_URL"), 2)
	if err != nil {
		t.Fatalf("restart Open() error = %v", err)
	}
	t.Cleanup(restarted.Close)
	hydrated, err := restarted.FindTable(environment.ctx, environment.tableID)
	if err != nil {
		t.Fatalf("restart FindTable() error = %v", err)
	}
	if hydrated.Revision != 3 || hydrated.LastSeq != 3 || !hydrated.Seats[bridge.North].Ready {
		t.Fatalf("hydrated aggregate = %+v", hydrated)
	}
	events, err := restarted.ListEventsAfter(environment.ctx, environment.tableID, 0, 10)
	if err != nil {
		t.Fatalf("ListEventsAfter() error = %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("event count = %d, want 3", len(events))
	}
	for _index, event := range events {
		if event.Seq != int64(_index+1) {
			t.Fatalf("event %d seq = %d", _index, event.Seq)
		}
	}
	if strings.Contains(environment.logs.String(), environment.sessions[0].ID) || strings.Contains(environment.logs.String(), environment.sessions[0].Nickname) {
		t.Fatalf("command logs contain private identity: %s", environment.logs.String())
	}
}

func TestCommandRepositoryConcurrentRevisionFence(t *testing.T) {
	environment := newCommandTestEnvironment(t, 2)
	aggregate, err := environment.postgres.FindTable(environment.ctx, environment.tableID)
	if err != nil {
		t.Fatalf("FindTable() error = %v", err)
	}
	requests := []table.CommandRequest{
		{
			TableID: environment.tableID, SessionID: environment.sessions[0].ID, RequestID: "concurrent_lock_01",
			ExpectedRevision: aggregate.Revision, Command: table.Command{Name: table.CommandLockTable, Locked: true},
		},
		{
			TableID: environment.tableID, SessionID: environment.sessions[1].ID, RequestID: "concurrent_seat_01",
			ExpectedRevision: aggregate.Revision, Command: table.Command{Name: table.CommandTakeSeat, Seat: bridge.East},
		},
	}
	results := make(chan table.CommandResult, len(requests))
	errorsChannel := make(chan error, len(requests))
	var waitGroup sync.WaitGroup
	for _, request := range requests {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			result, processErr := environment.processor.Process(environment.ctx, request)
			results <- result
			errorsChannel <- processErr
		}()
	}
	waitGroup.Wait()
	close(results)
	close(errorsChannel)
	for processErr := range errorsChannel {
		if processErr != nil {
			t.Fatalf("Process() error = %v", processErr)
		}
	}
	accepted := 0
	stateChanged := 0
	for result := range results {
		if result.Outcome.Status == table.CommandStatusAccepted {
			accepted++
		} else if result.Outcome.ErrorCode == table.ErrorStateChanged {
			stateChanged++
		}
	}
	if accepted != 1 || stateChanged != 1 {
		t.Fatalf("concurrent outcomes: accepted=%d stateChanged=%d", accepted, stateChanged)
	}
	final, err := environment.postgres.FindTable(environment.ctx, environment.tableID)
	if err != nil {
		t.Fatalf("FindTable() error = %v", err)
	}
	if final.Revision != aggregate.Revision+1 || final.LastSeq != aggregate.LastSeq+1 {
		t.Fatalf("final revision/seq = %d/%d, initial %d/%d", final.Revision, final.LastSeq, aggregate.Revision, aggregate.LastSeq)
	}
}

func TestCommandRepositoryRejectsInvalidSnapshot(t *testing.T) {
	tests := []struct {
		name  string
		query string
	}{
		{name: "revision mismatch", query: "UPDATE bridgeyok.game_snapshots SET revision = revision + 1 WHERE table_id = $1"},
		{name: "invalid aggregate", query: "UPDATE bridgeyok.game_snapshots SET private_state = '{}'::jsonb WHERE table_id = $1"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			environment := newCommandTestEnvironment(t, 1)
			if _, err := environment.postgres.Pool().Exec(environment.ctx, test.query, environment.tableID); err != nil {
				t.Fatalf("tamper snapshot: %v", err)
			}
			if _, err := environment.postgres.FindTable(environment.ctx, environment.tableID); err == nil {
				t.Fatal("FindTable() error = nil")
			}
		})
	}
}

func TestCommandRepositoryRollsBackPartialPersistence(t *testing.T) {
	environment := newCommandTestEnvironment(t, 4)
	seats := []bridge.Seat{bridge.North, bridge.East, bridge.South, bridge.West}
	aggregate, err := environment.postgres.FindTable(environment.ctx, environment.tableID)
	if err != nil {
		t.Fatalf("FindTable() error = %v", err)
	}
	for _index, session := range environment.sessions {
		takeSeat := environment.process(t, table.CommandRequest{
			TableID: environment.tableID, SessionID: session.ID, RequestID: "rollback_seat_" + string(rune('a'+_index)),
			ExpectedRevision: aggregate.Revision, Command: table.Command{Name: table.CommandTakeSeat, Seat: seats[_index]},
		})
		aggregate = takeSeat.Aggregate
		ready := environment.process(t, table.CommandRequest{
			TableID: environment.tableID, SessionID: session.ID, RequestID: "rollback_ready_" + string(rune('a'+_index)),
			ExpectedRevision: aggregate.Revision, Command: table.Command{Name: table.CommandSetReady, Ready: true},
		})
		aggregate = ready.Aggregate
	}
	before := aggregate
	beforeEvents, err := environment.postgres.ListEventsAfter(environment.ctx, environment.tableID, 0, maxRecoveryEvents)
	if err != nil {
		t.Fatalf("ListEventsAfter() error = %v", err)
	}
	deal, err := bridge.GenerateDeal(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateDeal() error = %v", err)
	}
	request := table.CommandRequest{
		TableID: environment.tableID, SessionID: environment.sessions[0].ID, RequestID: "rollback_start_01",
		ExpectedRevision: before.Revision,
		Command:          table.Command{Name: table.CommandStartGame, Deal: &deal, BoardID: "invalid-board-id"},
	}
	if _, err := environment.processor.Process(environment.ctx, request); err == nil {
		t.Fatal("Process() error = nil")
	}
	if !strings.Contains(environment.logs.String(), `"msg":"table_command_persistence_failed"`) || !strings.Contains(environment.logs.String(), `"result_code":"DB_ERROR"`) {
		t.Fatalf("rollback logs = %s", environment.logs.String())
	}
	after, err := environment.postgres.FindTable(environment.ctx, environment.tableID)
	if err != nil {
		t.Fatalf("FindTable() after rollback error = %v", err)
	}
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("aggregate changed after rollback\nafter=%+v\nbefore=%+v", after, before)
	}
	afterEvents, err := environment.postgres.ListEventsAfter(environment.ctx, environment.tableID, 0, maxRecoveryEvents)
	if err != nil {
		t.Fatalf("ListEventsAfter() after rollback error = %v", err)
	}
	if !reflect.DeepEqual(afterEvents, beforeEvents) {
		t.Fatal("event log changed after rollback")
	}
	_, err = environment.postgres.queries.FindProcessedCommand(environment.ctx, dbgen.FindProcessedCommandParams{
		TableID: environment.tableID, SessionID: environment.sessions[0].ID, RequestID: request.RequestID,
	})
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("FindProcessedCommand() error = %v, want no rows", err)
	}
}

func newCommandTestEnvironment(t *testing.T, participantCount int) commandTestEnvironment {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("TEST_DATABASE_URL is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	t.Cleanup(cancel)
	postgres, err := Open(ctx, databaseURL, 6)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(postgres.Close)
	identityService, err := identity.NewService(postgres, []byte(strings.Repeat("command-identity-pepper", 2)), rand.Reader, time.Now)
	if err != nil {
		t.Fatalf("identity.NewService() error = %v", err)
	}
	tableService, err := table.NewService(postgres, []byte(strings.Repeat("command-table-pepper", 2)), rand.Reader, time.Now)
	if err != nil {
		t.Fatalf("table.NewService() error = %v", err)
	}
	sessions := make([]identity.Session, participantCount)
	for _index := range sessions {
		credentials, createErr := identityService.CreateSession(ctx, "Command Guest "+string(rune('A'+_index)))
		if createErr != nil {
			t.Fatalf("CreateSession(%d) error = %v", _index, createErr)
		}
		sessions[_index] = credentials.Session
	}
	created, err := tableService.Create(ctx, sessions[0])
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	for _, session := range sessions[1:] {
		if _, err := tableService.Join(ctx, created.InviteCode, session); err != nil {
			t.Fatalf("Join() error = %v", err)
		}
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		if _, cleanupErr := postgres.Pool().Exec(cleanupCtx, "DELETE FROM bridgeyok.tables WHERE id = $1", created.Projection.TableID); cleanupErr != nil {
			t.Errorf("cleanup table: %v", cleanupErr)
		}
		for _, session := range sessions {
			if _, cleanupErr := postgres.Pool().Exec(cleanupCtx, "DELETE FROM bridgeyok.guest_sessions WHERE id = $1", session.ID); cleanupErr != nil {
				t.Errorf("cleanup session: %v", cleanupErr)
			}
		}
	})
	logs := &lockedBuffer{}
	processor, err := table.NewCommandProcessor(postgres, nil, observability.NewLoggerWithWriter(slog.LevelDebug, logs), time.Now)
	if err != nil {
		t.Fatalf("NewCommandProcessor() error = %v", err)
	}
	return commandTestEnvironment{
		ctx: ctx, postgres: postgres, tableService: tableService, processor: processor,
		sessions: sessions, tableID: created.Projection.TableID, logs: logs,
	}
}

func (environment commandTestEnvironment) process(t *testing.T, request table.CommandRequest) table.CommandResult {
	t.Helper()
	result, err := environment.processor.Process(environment.ctx, request)
	if err != nil {
		t.Fatalf("Process(%s) error = %v", request.RequestID, err)
	}
	return result
}
