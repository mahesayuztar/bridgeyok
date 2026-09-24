package database

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/bridge"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/database/dbgen"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/history"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/table"
)

func (postgres *Postgres) ListHistoryBoards(ctx context.Context, sessionID string, cursor history.Cursor, limit int) ([]history.Board, error) {
	var cursorAt pgtype.Timestamptz
	var cursorID pgtype.UUID
	if !cursor.CompletedAt.IsZero() {
		parsed, err := uuid.Parse(cursor.BoardID)
		if err != nil {
			return nil, history.ErrInvalidCursor
		}
		cursorAt = timestamptz(cursor.CompletedAt)
		cursorID = pgtype.UUID{Bytes: [16]byte(parsed), Valid: true}
	}
	rows, err := postgres.queries.ListHistoryBoards(ctx, dbgen.ListHistoryBoardsParams{
		SessionID: sessionID,
		CursorAt:  cursorAt,
		CursorID:  cursorID,
		PageLimit: int32(limit),
	})
	if err != nil {
		return nil, err
	}
	items := make([]history.Board, 0, len(rows))
	for _, row := range rows {
		if row.ViewerSeat == "" || !row.CompletedAt.Valid {
			return nil, fmt.Errorf("history row is missing viewer or completion metadata")
		}
		var result bridge.Result
		if err := json.Unmarshal(row.Result, &result); err != nil {
			return nil, fmt.Errorf("decode history result: %w", err)
		}
		var participants map[bridge.Seat]history.Participant
		if err := json.Unmarshal(row.Lineup, &participants); err != nil {
			return nil, fmt.Errorf("decode history lineup: %w", err)
		}
		lineup, err := historyLineup(participants)
		if err != nil {
			return nil, err
		}
		item := history.Board{
			BoardID: row.ID, TableID: row.TableID, BoardNumber: int(row.BoardNumber), Result: result,
			Lineup: lineup, ViewerSeat: bridge.Seat(row.ViewerSeat), CompletedAt: row.CompletedAt.Time,
			ReplayAllowed: row.MatchStatus == "" || row.MatchStatus == "COMPLETE",
		}
		if row.MatchID.Valid {
			team, _ := row.Team.(string)
			item.MatchID = row.MatchID.String()
			item.MatchStatus = row.MatchStatus
			item.MatchRoom = row.MatchRoom
			item.Team = team
			if row.TeamAImp != nil {
				value := int(*row.TeamAImp)
				if item.Team == "B" {
					value = -value
				}
				item.TeamAIMP = &value
			}
		}
		items = append(items, item)
	}
	return items, nil
}

func historyLineup(participants map[bridge.Seat]history.Participant) (history.BoardLineup, error) {
	seats := map[bridge.Seat]history.Participant{}
	for _, seat := range []bridge.Seat{bridge.North, bridge.East, bridge.South, bridge.West} {
		participant, ok := participants[seat]
		if !ok || participant.ID == "" || participant.Nickname == "" {
			return history.BoardLineup{}, fmt.Errorf("history lineup is missing seat %s", seat)
		}
		seats[seat] = participant
	}
	pair := func(first, second bridge.Seat) table.ScorePair {
		members := [2]table.ScoreParticipant{
			{ID: seats[first].ID, Nickname: seats[first].Nickname, IsBot: seats[first].IsBot},
			{ID: seats[second].ID, Nickname: seats[second].Nickname, IsBot: seats[second].IsBot},
		}
		if members[1].ID < members[0].ID {
			members[0], members[1] = members[1], members[0]
		}
		return table.ScorePair{ID: members[0].ID + ":" + members[1].ID, Members: members}
	}
	return history.BoardLineup{
		Seats:      seats,
		NorthSouth: pair(bridge.North, bridge.South),
		EastWest:   pair(bridge.East, bridge.West),
	}, nil
}
