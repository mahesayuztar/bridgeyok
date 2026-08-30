package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/bridge"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/database/dbgen"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/table"
)

const maxRecoveryEvents = 256

// ProcessCommand atomically persists an idempotent outcome, ordered events, and private snapshot.
func (postgres *Postgres) ProcessCommand(ctx context.Context, request table.CommandRequest, processedAt time.Time, expiresAt time.Time) (table.CommandResult, error) {
	var result table.CommandResult
	err := pgx.BeginFunc(ctx, postgres.pool, func(tx pgx.Tx) error {
		queries := postgres.queries.WithTx(tx)
		row, err := queries.LockTableByID(ctx, request.TableID)
		if errors.Is(err, pgx.ErrNoRows) {
			return table.ErrTableNotFound
		}
		if err != nil {
			return fmt.Errorf("lock table for command: %w", err)
		}
		aggregate, err := loadTableAggregate(ctx, queries, tableRow{
			id: row.ID, ownerSessionID: row.OwnerSessionID, state: row.State, locked: row.Locked,
			revision: row.Revision, lastSeq: int64(row.LastSeq),
		})
		if err != nil {
			return err
		}

		storedOutcome, err := queries.FindProcessedCommand(ctx, dbgen.FindProcessedCommandParams{
			TableID: request.TableID, SessionID: request.SessionID, RequestID: request.RequestID,
		})
		if err == nil {
			if err := json.Unmarshal(storedOutcome, &result.Outcome); err != nil {
				return fmt.Errorf("decode processed command outcome: %w", err)
			}
			result.Aggregate = aggregate
			result.Duplicate = true
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("find processed command: %w", err)
		}

		if request.ExpectedRevision != aggregate.Revision {
			result = rejectedResult(request, aggregate, table.ErrorStateChanged)
			return insertProcessedOutcome(ctx, queries, request, result.Outcome, processedAt, expiresAt)
		}
		decision, domainError := table.Decide(aggregate, request.Command)
		if domainError != nil {
			result = rejectedResult(request, aggregate, domainError.Code)
			return insertProcessedOutcome(ctx, queries, request, result.Outcome, processedAt, expiresAt)
		}
		result, err = persistAcceptedDecision(ctx, queries, request, aggregate, decision, processedAt)
		if err != nil {
			return err
		}
		return insertProcessedOutcome(ctx, queries, request, result.Outcome, processedAt, expiresAt)
	})
	if err != nil {
		return table.CommandResult{}, fmt.Errorf("process command transaction: %w", err)
	}
	return result, nil
}

// ListEventsAfter returns a bounded ordered event gap for reconnect recovery.
func (postgres *Postgres) ListEventsAfter(ctx context.Context, tableID string, afterSeq int64, limit int) ([]table.PersistedEvent, error) {
	if limit < 1 || limit > maxRecoveryEvents {
		return nil, fmt.Errorf("event limit must be between 1 and %d", maxRecoveryEvents)
	}
	rows, err := postgres.queries.ListGameEventsAfter(ctx, dbgen.ListGameEventsAfterParams{
		TableID: tableID, AfterSeq: afterSeq, EventLimit: int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list game events: %w", err)
	}
	events := make([]table.PersistedEvent, 0, len(rows))
	for _, row := range rows {
		events = append(events, table.PersistedEvent{
			TableID: row.TableID, Seq: row.Seq, Revision: row.Revision, Type: row.EventType,
			Payload: json.RawMessage(append([]byte(nil), row.Payload...)), OccurredAt: row.OccurredAt.Time,
		})
	}
	return events, nil
}

func rejectedResult(request table.CommandRequest, aggregate table.Aggregate, code table.ErrorCode) table.CommandResult {
	return table.CommandResult{
		Outcome: table.CommandOutcome{
			RequestID: request.RequestID, CommandName: request.Command.Name, Status: table.CommandStatusRejected,
			ErrorCode: code, Revision: aggregate.Revision, LastSeq: aggregate.LastSeq,
		},
		Aggregate: aggregate,
	}
}

func persistAcceptedDecision(
	ctx context.Context,
	queries *dbgen.Queries,
	request table.CommandRequest,
	current table.Aggregate,
	decision table.Decision,
	occurredAt time.Time,
) (table.CommandResult, error) {
	if len(decision.Events) == 0 {
		return table.CommandResult{}, fmt.Errorf("accepted command produced no events")
	}
	next := decision.NextState
	next.Revision = current.Revision + 1
	next.LastSeq = current.LastSeq + int64(len(decision.Events))
	if err := next.Validate(); err != nil {
		return table.CommandResult{}, fmt.Errorf("validate next command state: %w", err)
	}
	rows, err := queries.UpdateTableAfterCommand(ctx, dbgen.UpdateTableAfterCommandParams{
		State: string(next.State), Locked: next.Locked, NextRevision: next.Revision, NextSeq: next.LastSeq + 1,
		OccurredAt: timestamptz(occurredAt), ID: next.ID, ExpectedRevision: current.Revision,
	})
	if err != nil {
		return table.CommandResult{}, fmt.Errorf("update table command state: %w", err)
	}
	if rows != 1 {
		return table.CommandResult{}, fmt.Errorf("update table command state: affected %d rows", rows)
	}
	if err := syncRelationalAggregate(ctx, queries, next, occurredAt); err != nil {
		return table.CommandResult{}, err
	}

	events := make([]table.PersistedEvent, 0, len(decision.Events))
	for _index, event := range decision.Events {
		payload, err := json.Marshal(event.Payload)
		if err != nil {
			return table.CommandResult{}, fmt.Errorf("encode event %s: %w", event.Type, err)
		}
		persisted := table.PersistedEvent{
			TableID: next.ID, Seq: current.LastSeq + int64(_index) + 1, Revision: next.Revision,
			Type: event.Type, Payload: event.Payload, OccurredAt: occurredAt,
		}
		if err := queries.InsertGameEvent(ctx, dbgen.InsertGameEventParams{
			TableID: persisted.TableID, Seq: persisted.Seq, Revision: persisted.Revision,
			EventType: persisted.Type, Payload: payload, OccurredAt: timestamptz(persisted.OccurredAt),
		}); err != nil {
			return table.CommandResult{}, fmt.Errorf("insert event %s: %w", event.Type, err)
		}
		events = append(events, persisted)
	}
	if err := upsertPrivateSnapshot(ctx, queries, next, occurredAt); err != nil {
		return table.CommandResult{}, err
	}
	return table.CommandResult{
		Outcome: table.CommandOutcome{
			RequestID: request.RequestID, CommandName: request.Command.Name, Status: table.CommandStatusAccepted,
			Revision: next.Revision, FirstSeq: events[0].Seq, LastSeq: next.LastSeq,
		},
		Aggregate: next,
		Events:    events,
	}, nil
}

func insertProcessedOutcome(
	ctx context.Context,
	queries *dbgen.Queries,
	request table.CommandRequest,
	outcome table.CommandOutcome,
	processedAt time.Time,
	expiresAt time.Time,
) error {
	encoded, err := json.Marshal(outcome)
	if err != nil {
		return fmt.Errorf("encode command outcome: %w", err)
	}
	if err := queries.InsertProcessedCommand(ctx, dbgen.InsertProcessedCommandParams{
		TableID: request.TableID, SessionID: request.SessionID, RequestID: request.RequestID,
		CommandName: string(request.Command.Name), Outcome: encoded, Revision: outcome.Revision, LastSeq: outcome.LastSeq,
		ProcessedAt: timestamptz(processedAt), ExpiresAt: timestamptz(expiresAt),
	}); err != nil {
		return fmt.Errorf("insert processed command: %w", err)
	}
	return nil
}

func syncRelationalAggregate(ctx context.Context, queries *dbgen.Queries, aggregate table.Aggregate, occurredAt time.Time) error {
	for _, participant := range aggregate.Participants {
		if participant.LeftAt == nil {
			continue
		}
		if err := queries.SyncParticipantLeftAt(ctx, dbgen.SyncParticipantLeftAtParams{
			LeftAt: timestamptz(*participant.LeftAt), TableID: aggregate.ID, ParticipantID: participant.ID,
		}); err != nil {
			return fmt.Errorf("sync participant leave: %w", err)
		}
	}
	recoveryRows, err := queries.ListSeatRecoveryHashes(ctx, aggregate.ID)
	if err != nil {
		return fmt.Errorf("list seat recovery hashes: %w", err)
	}
	recoveryHashes := make(map[string][]byte, len(recoveryRows))
	for _, recoveryRow := range recoveryRows {
		recoveryHashes[recoveryRow.ParticipantID] = append([]byte(nil), recoveryRow.RecoveryHash...)
	}
	if err := queries.DeleteTableSeatsForSync(ctx, aggregate.ID); err != nil {
		return fmt.Errorf("clear table seats: %w", err)
	}
	for seat, assignment := range aggregate.Seats {
		if err := queries.InsertTableSeatForSync(ctx, dbgen.InsertTableSeatForSyncParams{
			TableID: aggregate.ID, Seat: string(seat), ParticipantID: assignment.ParticipantID,
			Ready: assignment.Ready, ControllerEpoch: assignment.ControllerEpoch,
			RecoveryHash: recoveryHashes[assignment.ParticipantID], UpdatedAt: timestamptz(occurredAt),
		}); err != nil {
			return fmt.Errorf("sync seat %s: %w", seat, err)
		}
	}
	if aggregate.Game != nil {
		if err := upsertBoard(ctx, queries, aggregate, occurredAt); err != nil {
			return err
		}
	}
	return nil
}

func upsertBoard(ctx context.Context, queries *dbgen.Queries, aggregate table.Aggregate, occurredAt time.Time) error {
	game := aggregate.Game
	status := "AUCTION"
	if game.Phase == bridge.PhaseOpeningLead || game.Phase == bridge.PhasePlay {
		status = "PLAY"
	} else if game.Phase == bridge.PhaseBoardScored && game.Result != nil && game.Result.PassedOut {
		status = "PASSED_OUT"
	} else if game.Phase == bridge.PhaseBoardScored {
		status = "SCORED"
	}
	var scoreNS *int32
	var resultJSON []byte
	completedAt := pgtype.Timestamptz{}
	if game.Result != nil {
		score := int32(game.Result.ScoreNS)
		scoreNS = &score
		var err error
		resultJSON, err = json.Marshal(game.Result)
		if err != nil {
			return fmt.Errorf("encode board result: %w", err)
		}
		completedAt = timestamptz(occurredAt)
	}
	if err := queries.UpsertBoard(ctx, dbgen.UpsertBoardParams{
		ID: aggregate.BoardID, TableID: aggregate.ID, BoardNumber: int32(aggregate.BoardNumber),
		Dealer: string(game.Board.Dealer), Vulnerability: string(game.Board.Vulnerability), RulesetVersion: game.RulesetVersion,
		Status: status, ScoreNs: scoreNS, Result: resultJSON, CreatedAt: timestamptz(occurredAt), CompletedAt: completedAt,
	}); err != nil {
		return fmt.Errorf("upsert board: %w", err)
	}
	return nil
}

func upsertPrivateSnapshot(ctx context.Context, queries *dbgen.Queries, aggregate table.Aggregate, updatedAt time.Time) error {
	privateState, err := json.Marshal(aggregate)
	if err != nil {
		return fmt.Errorf("encode private snapshot: %w", err)
	}
	boardID := pgtype.UUID{}
	if aggregate.BoardID != "" {
		parsed, err := uuid.Parse(aggregate.BoardID)
		if err != nil {
			return fmt.Errorf("parse snapshot board id: %w", err)
		}
		boardID = pgtype.UUID{Bytes: [16]byte(parsed), Valid: true}
	}
	if err := queries.UpsertGameSnapshot(ctx, dbgen.UpsertGameSnapshotParams{
		TableID: aggregate.ID, BoardID: boardID, Revision: aggregate.Revision, LastSeq: aggregate.LastSeq,
		PrivateState: privateState, UpdatedAt: timestamptz(updatedAt),
	}); err != nil {
		return fmt.Errorf("upsert private snapshot: %w", err)
	}
	return nil
}
