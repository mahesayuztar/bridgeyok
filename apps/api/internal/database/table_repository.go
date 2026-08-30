package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/bridge"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/database/dbgen"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/table"
)

// CreateTable persists a waiting table and its owner in one transaction.
func (postgres *Postgres) CreateTable(ctx context.Context, record table.CreateRecord) error {
	if err := record.Aggregate.Validate(); err != nil {
		return fmt.Errorf("validate table record: %w", err)
	}
	owner := table.Participant{}
	for _, participant := range record.Aggregate.Participants {
		if participant.SessionID == record.Aggregate.OwnerSessionID && participant.Role == table.RoleOwner {
			owner = participant
			break
		}
	}
	if owner.ID == "" {
		return fmt.Errorf("validate table record: owner participant is missing")
	}
	err := pgx.BeginFunc(ctx, postgres.pool, func(tx pgx.Tx) error {
		queries := postgres.queries.WithTx(tx)
		if err := queries.CreateTable(ctx, dbgen.CreateTableParams{
			ID:             record.Aggregate.ID,
			OwnerSessionID: record.Aggregate.OwnerSessionID,
			InviteCodeHash: record.InviteCodeHash,
			CreatedAt:      timestamptz(record.CreatedAt),
		}); err != nil {
			return fmt.Errorf("insert table: %w", err)
		}
		if err := queries.CreateTableParticipant(ctx, dbgen.CreateTableParticipantParams{
			ID:        owner.ID,
			TableID:   record.Aggregate.ID,
			SessionID: owner.SessionID,
			Role:      string(owner.Role),
			JoinedAt:  timestamptz(owner.JoinedAt),
		}); err != nil {
			return fmt.Errorf("insert owner participant: %w", err)
		}
		if err := upsertPrivateSnapshot(ctx, queries, record.Aggregate, record.CreatedAt); err != nil {
			return err
		}
		return nil
	})
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) && postgresError.Code == "23505" {
		return table.ErrInviteCollision
	}
	if err != nil {
		return fmt.Errorf("create table transaction: %w", err)
	}
	return nil
}

// PreviewTable returns non-identifying invite status.
func (postgres *Postgres) PreviewTable(ctx context.Context, inviteCodeHash []byte) (table.Preview, error) {
	row, err := postgres.queries.PreviewTable(ctx, inviteCodeHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return table.Preview{}, table.ErrTableNotFound
	}
	if err != nil {
		return table.Preview{}, fmt.Errorf("select table preview: %w", err)
	}
	return table.Preview{
		State:            table.State(row.State),
		Locked:           row.Locked,
		ParticipantCount: int(row.ParticipantCount),
		Capacity:         4,
	}, nil
}

// JoinTable serializes join decisions on the table row and persists one participant.
func (postgres *Postgres) JoinTable(ctx context.Context, inviteCodeHash []byte, participant table.Participant) (table.Aggregate, error) {
	var joined table.Aggregate
	err := pgx.BeginFunc(ctx, postgres.pool, func(tx pgx.Tx) error {
		queries := postgres.queries.WithTx(tx)
		row, err := queries.LockTableByInvite(ctx, inviteCodeHash)
		if errors.Is(err, pgx.ErrNoRows) {
			return table.ErrTableUnavailable
		}
		if err != nil {
			return fmt.Errorf("lock table by invite: %w", err)
		}
		aggregate, err := loadTableAggregate(ctx, queries, tableRow{
			id: row.ID, ownerSessionID: row.OwnerSessionID, state: row.State, locked: row.Locked,
			revision: row.Revision, lastSeq: int64(row.LastSeq),
		})
		if err != nil {
			return err
		}
		for _, current := range aggregate.Participants {
			if current.SessionID == participant.SessionID && current.LeftAt == nil {
				joined = aggregate
				return nil
			}
		}
		decision, domainError := table.Decide(aggregate, table.Command{Name: table.CommandJoinTable, Participant: &participant})
		if domainError != nil {
			return domainError
		}
		if err := queries.CreateTableParticipant(ctx, dbgen.CreateTableParticipantParams{
			ID:        participant.ID,
			TableID:   aggregate.ID,
			SessionID: participant.SessionID,
			Role:      string(participant.Role),
			JoinedAt:  timestamptz(participant.JoinedAt),
		}); err != nil {
			return fmt.Errorf("insert table participant: %w", err)
		}
		result, err := persistAcceptedDecision(ctx, queries, table.CommandRequest{
			TableID: aggregate.ID, SessionID: participant.SessionID,
			RequestID: "rest_join", Command: table.Command{Name: table.CommandJoinTable},
		}, aggregate, decision, participant.JoinedAt)
		if err != nil {
			return err
		}
		joined = result.Aggregate
		return nil
	})
	if err != nil {
		return table.Aggregate{}, fmt.Errorf("join table transaction: %w", err)
	}
	return joined, nil
}

// FindTable hydrates the current relational table lifecycle state.
func (postgres *Postgres) FindTable(ctx context.Context, tableID string) (table.Aggregate, error) {
	row, err := postgres.queries.FindTableByID(ctx, tableID)
	if errors.Is(err, pgx.ErrNoRows) {
		return table.Aggregate{}, table.ErrTableNotFound
	}
	if err != nil {
		return table.Aggregate{}, fmt.Errorf("select table: %w", err)
	}
	return loadTableAggregate(ctx, postgres.queries, tableRow{
		id: row.ID, ownerSessionID: row.OwnerSessionID, state: row.State, locked: row.Locked,
		revision: row.Revision, lastSeq: int64(row.LastSeq),
	})
}

// LeaveTable serializes a waiting-table leave and releases any occupied seat.
func (postgres *Postgres) LeaveTable(ctx context.Context, tableID string, sessionID string, occurredAt time.Time) error {
	return pgx.BeginFunc(ctx, postgres.pool, func(tx pgx.Tx) error {
		queries := postgres.queries.WithTx(tx)
		row, err := queries.LockTableByID(ctx, tableID)
		if errors.Is(err, pgx.ErrNoRows) {
			return table.ErrTableNotFound
		}
		if err != nil {
			return fmt.Errorf("lock table: %w", err)
		}
		aggregate, err := loadTableAggregate(ctx, queries, tableRow{
			id: row.ID, ownerSessionID: row.OwnerSessionID, state: row.State, locked: row.Locked,
			revision: row.Revision, lastSeq: int64(row.LastSeq),
		})
		if err != nil {
			return err
		}
		decision, domainError := table.Decide(aggregate, table.Command{Name: table.CommandLeaveTable, SessionID: sessionID, OccurredAt: occurredAt})
		if domainError != nil {
			return domainError
		}
		_, err = persistAcceptedDecision(ctx, queries, table.CommandRequest{
			TableID: tableID, SessionID: sessionID,
			RequestID: "rest_leave", Command: table.Command{Name: table.CommandLeaveTable},
		}, aggregate, decision, occurredAt)
		return err
	})
}

type tableRow struct {
	id             string
	ownerSessionID string
	state          string
	locked         bool
	revision       int64
	lastSeq        int64
}

func loadTableAggregate(ctx context.Context, queries *dbgen.Queries, row tableRow) (table.Aggregate, error) {
	snapshot, err := queries.LoadGameSnapshot(ctx, row.id)
	if err == nil {
		var aggregate table.Aggregate
		if err := json.Unmarshal(snapshot.PrivateState, &aggregate); err != nil {
			return table.Aggregate{}, fmt.Errorf("decode private snapshot: %w", err)
		}
		if snapshot.SchemaVersion != 1 || snapshot.Revision != row.revision || snapshot.LastSeq != row.lastSeq || aggregate.ID != row.id || aggregate.OwnerSessionID != row.ownerSessionID || aggregate.State != table.State(row.state) || aggregate.Locked != row.locked || aggregate.Revision != row.revision || aggregate.LastSeq != row.lastSeq {
			return table.Aggregate{}, fmt.Errorf("private snapshot does not match table row")
		}
		if err := aggregate.Validate(); err != nil {
			return table.Aggregate{}, fmt.Errorf("validate private snapshot: %w", err)
		}
		return aggregate, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return table.Aggregate{}, fmt.Errorf("load private snapshot: %w", err)
	}
	participantRows, err := queries.ListActiveTableParticipants(ctx, row.id)
	if err != nil {
		return table.Aggregate{}, fmt.Errorf("list table participants: %w", err)
	}
	seatRows, err := queries.ListTableSeats(ctx, row.id)
	if err != nil {
		return table.Aggregate{}, fmt.Errorf("list table seats: %w", err)
	}
	aggregate := table.Aggregate{
		SchemaVersion:  1,
		ID:             row.id,
		OwnerSessionID: row.ownerSessionID,
		State:          table.State(row.state),
		Locked:         row.locked,
		Revision:       row.revision,
		LastSeq:        row.lastSeq,
		Participants:   make([]table.Participant, 0, len(participantRows)),
		Seats:          make(map[bridge.Seat]table.SeatAssignment, len(seatRows)),
	}
	for _, participantRow := range participantRows {
		aggregate.Participants = append(aggregate.Participants, table.Participant{
			ID:        participantRow.ID,
			SessionID: participantRow.SessionID,
			Nickname:  participantRow.Nickname,
			Role:      table.Role(participantRow.Role),
			JoinedAt:  participantRow.JoinedAt.Time,
		})
	}
	for _, seatRow := range seatRows {
		aggregate.Seats[bridge.Seat(seatRow.Seat)] = table.SeatAssignment{
			ParticipantID:   seatRow.ParticipantID,
			Ready:           seatRow.Ready,
			ControllerEpoch: seatRow.ControllerEpoch,
		}
	}
	if err := aggregate.Validate(); err != nil {
		return table.Aggregate{}, fmt.Errorf("validate hydrated table: %w", err)
	}
	return aggregate, nil
}
