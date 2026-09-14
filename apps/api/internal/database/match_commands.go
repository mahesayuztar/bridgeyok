package database

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/database/dbgen"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/match"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/table"
)

// prepareMatchCommand prevents casual lifecycle changes and supplies only the persisted shared deal for progression.
func prepareMatchCommand(ctx context.Context, queries *dbgen.Queries, aggregate table.Aggregate, request *table.CommandRequest) (*dbgen.BridgeyokMatchRoomBoard, table.ErrorCode, error) {
	room, err := queries.FindTableMatch(ctx, aggregate.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", err
	}
	switch request.Command.Name {
	case table.CommandMakeCall, table.CommandPlayCard, table.CommandRequestClaim, table.CommandRespondClaim, table.CommandRequestUndo, table.CommandRespondUndo, table.CommandTakeoverControl, table.CommandSetReady:
		return nil, "", nil
	case table.CommandRequestNextBoard, table.CommandFinishTable:
		row, err := queries.LockMatch(ctx, room.MatchID)
		if err != nil {
			return nil, "", err
		}
		stored, err := hydrateMatch(ctx, queries, row)
		if err != nil {
			return nil, "", err
		}
		state := stored.Match.PrivateSnapshot()
		if state.Status != match.Active || aggregate.State != table.StateBetweenBoards || aggregate.BoardNumber < 1 {
			return nil, table.ErrorInvalidState, nil
		}
		if request.Command.Name == table.CommandFinishTable {
			if aggregate.BoardNumber != len(state.Boards) {
				return nil, table.ErrorInvalidState, nil
			}
			return nil, "", nil
		}
		if aggregate.BoardNumber >= len(state.Boards) {
			return nil, table.ErrorInvalidState, nil
		}
		board := state.Boards[aggregate.BoardNumber]
		request.Command.Deal = &board.Source.Deal
		request.Command.DealProvenance = &board.Source.Provenance
		request.Command.BoardID = uuid.NewString()
		return &dbgen.BridgeyokMatchRoomBoard{MatchID: room.MatchID, BoardID: board.ID, Room: room.Room, TableID: aggregate.ID, TableBoardID: request.Command.BoardID}, "", nil
	default:
		return nil, table.ErrorInvalidState, nil
	}
}

func syncMatchReadiness(ctx context.Context, queries *dbgen.Queries, request table.CommandRequest, occurredAt time.Time) error {
	room, err := queries.FindTableMatch(ctx, request.TableID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	row, err := queries.LockMatch(ctx, room.MatchID)
	if err != nil {
		return err
	}
	stored, err := hydrateMatch(ctx, queries, row)
	if err != nil {
		return err
	}
	if err := stored.Match.SetReady(request.SessionID, request.Command.Ready); err != nil {
		return err
	}
	affected, err := queries.SetMatchReady(ctx, dbgen.SetMatchReadyParams{MatchID: room.MatchID, SessionID: request.SessionID, Ready: request.Command.Ready})
	if err != nil {
		return err
	}
	if affected != 1 {
		return match.ErrInvalid
	}
	return saveMatchProgress(ctx, queries, stored, occurredAt)
}
