package database

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/analysis"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/bridge"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/database/dbgen"
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
