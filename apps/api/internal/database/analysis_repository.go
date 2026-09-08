package database

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/analysis"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/bridge"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/database/dbgen"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/table"
)

func (postgres *Postgres) CompletedAnalysisBoard(ctx context.Context, boardID string, sessionID string) (analysis.Board, error) {
	row, err := postgres.queries.FindBoardDeal(ctx, dbgen.FindBoardDealParams{BoardID: boardID, SessionID: sessionID})
	if errors.Is(err, pgx.ErrNoRows) {
		return analysis.Board{}, analysis.ErrNotFound
	}
	if err != nil {
		return analysis.Board{}, analysis.ErrUnavailable
	}
	if row.Status != "SCORED" && row.Status != "PASSED_OUT" {
		return analysis.Board{}, analysis.ErrNotCompleted
	}
	board := analysis.Board{ID: row.ID, Metadata: bridge.BoardMetadata{Number: int(row.BoardNumber), Dealer: bridge.Seat(row.Dealer), Vulnerability: bridge.Vulnerability(row.Vulnerability)}}
	if err := json.Unmarshal(row.SourceRecord, &board.Source); err != nil {
		return analysis.Board{}, analysis.ErrUnavailable
	}
	if err := board.Source.Validate(); err != nil {
		return analysis.Board{}, analysis.ErrUnavailable
	}
	return board, nil
}

func (postgres *Postgres) AnalysisPosition(ctx context.Context, boardID, sessionID, positionKey string, step *int) (bridge.State, error) {
	if step != nil {
		replay, err := postgres.CompletedBoardReplay(ctx, boardID, sessionID)
		if err != nil {
			return bridge.State{}, err
		}
		return analysis.ReplayPosition(replay.Game, replay.FullDeal, *step)
	}
	row, err := postgres.queries.FindBoardDeal(ctx, dbgen.FindBoardDealParams{BoardID: boardID, SessionID: sessionID})
	if errors.Is(err, pgx.ErrNoRows) {
		return bridge.State{}, analysis.ErrNotFound
	}
	if err != nil {
		return bridge.State{}, err
	}
	snapshot, err := postgres.queries.LoadGameSnapshot(ctx, row.TableID)
	if err != nil {
		return bridge.State{}, analysis.ErrUnavailable
	}
	var aggregate table.Aggregate
	if err := json.Unmarshal(snapshot.PrivateState, &aggregate); err != nil {
		return bridge.State{}, err
	}
	if aggregate.BoardID != boardID || aggregate.Game == nil {
		return bridge.State{}, analysis.ErrPositionChanged
	}
	if strconv.FormatInt(aggregate.Revision, 10) != positionKey {
		return bridge.State{}, analysis.ErrPositionChanged
	}
	projection, projectionError := table.Project(aggregate, sessionID)
	if projectionError != nil {
		return bridge.State{}, analysis.ErrNotFound
	}
	visibleTurn := projection.ViewerSeat == aggregate.Game.Turn
	if aggregate.Game.Auction.Contract != nil && aggregate.Game.Turn == aggregate.Game.Auction.Contract.Dummy() && projection.Game != nil && len(projection.Game.DummyHand) > 0 {
		visibleTurn = true
	}
	if !visibleTurn {
		return bridge.State{}, analysis.ErrNotFound
	}
	if err := aggregate.Game.ValidateInvariants(); err != nil {
		return bridge.State{}, err
	}
	return *aggregate.Game, nil
}
