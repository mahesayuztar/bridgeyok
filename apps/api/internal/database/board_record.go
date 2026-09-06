package database

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/bridge"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/database/dbgen"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/deal"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/table"
)

type BoardRecord struct {
	Version     int                `json:"version"`
	BoardID     string             `json:"boardId"`
	BoardNumber int                `json:"boardNumber"`
	Source      deal.Result        `json:"source"`
	FinalHash   string             `json:"finalHash"`
	Batches     []boardRecordBatch `json:"batches"`
}

type boardRecordBatch struct {
	Seq      int64         `json:"s"`
	Revision int64         `json:"r"`
	At       time.Time     `json:"t"`
	Events   []table.Event `json:"e"`
}

// Replay validates the archive and reconstructs the final engine state without active-table recovery.
func (record BoardRecord) Replay() (bridge.State, error) {
	if record.Version != 1 || record.BoardID == "" || len(record.Batches) == 0 || record.Batches[0].Seq < 1 {
		return bridge.State{}, fmt.Errorf("invalid board record header")
	}
	if err := record.Source.Validate(); err != nil {
		return bridge.State{}, err
	}
	state, err := bridge.NewBoard(record.BoardNumber, record.Source.Deal)
	if err != nil {
		return bridge.State{}, err
	}
	var undo *bridge.State
	nextSeq := record.Batches[0].Seq
	previousRevision := int64(0)
	for _batchIndex, batch := range record.Batches {
		if batch.Seq != nextSeq || batch.Revision <= previousRevision || len(batch.Events) == 0 {
			return bridge.State{}, fmt.Errorf("non-contiguous board history")
		}
		nextSeq += int64(len(batch.Events))
		previousRevision = batch.Revision
		engineEvents := []bridge.Event{}
		for _eventIndex, event := range batch.Events {
			payload, err := json.Marshal(event.Payload)
			if err != nil {
				return bridge.State{}, err
			}
			switch event.Type {
			case "BOARD_STARTED":
				var started struct {
					BoardID     string `json:"boardId"`
					BoardNumber int    `json:"boardNumber"`
				}
				if err := json.Unmarshal(payload, &started); err != nil {
					return bridge.State{}, err
				}
				if _batchIndex != 0 || _eventIndex != 0 || started.BoardID != record.BoardID || started.BoardNumber != record.BoardNumber {
					return bridge.State{}, fmt.Errorf("board start mismatch")
				}
			case "UNDO_ACCEPTED":
				if undo == nil || len(engineEvents) != 0 {
					return bridge.State{}, fmt.Errorf("undo without prior action")
				}
				state = undo.Clone()
				undo = nil
			case string(bridge.EventCallMade), string(bridge.EventCardPlayed), string(bridge.EventAuctionPassedOut), string(bridge.EventContractSet), string(bridge.EventDummyRevealed), string(bridge.EventTrickCompleted), string(bridge.EventBoardScored), "CLAIM_ACCEPTED":
				var engineEvent bridge.Event
				if err := json.Unmarshal(payload, &engineEvent); err != nil {
					return bridge.State{}, err
				}
				expectedType := event.Type
				if expectedType == "CLAIM_ACCEPTED" {
					expectedType = string(bridge.EventBoardClaimed)
				}
				if string(engineEvent.Type) != expectedType {
					return bridge.State{}, fmt.Errorf("event payload type mismatch")
				}
				switch engineEvent.Type {
				case bridge.EventCallMade, bridge.EventCardPlayed:
					previous := state.Clone()
					undo = &previous
				case bridge.EventBoardClaimed:
					undo = nil
				}
				engineEvents = append(engineEvents, engineEvent)
			}
		}
		if len(engineEvents) > 0 {
			state, err = bridge.Reduce(state, engineEvents)
			if err != nil {
				return bridge.State{}, fmt.Errorf("replay revision %d: %w", batch.Revision, err)
			}
		}
	}
	if record.Batches[0].Events[0].Type != "BOARD_STARTED" || state.Phase != bridge.PhaseBoardScored || state.Result == nil {
		return bridge.State{}, fmt.Errorf("board record is not complete")
	}
	encoded, err := json.Marshal(state)
	if err != nil {
		return bridge.State{}, err
	}
	if fmt.Sprintf("%x", sha256.Sum256(encoded)) != record.FinalHash {
		return bridge.State{}, fmt.Errorf("board replay differs from final snapshot")
	}
	return state, nil
}

// CompletedBoardRecord reads a private historical board for an existing table participant.
func (postgres *Postgres) CompletedBoardRecord(ctx context.Context, boardID, sessionID string) (BoardRecord, error) {
	encoded, err := postgres.queries.FindBoardRecord(ctx, dbgen.FindBoardRecordParams{BoardID: boardID, SessionID: sessionID})
	if err != nil {
		return BoardRecord{}, fmt.Errorf("find board record: %w", err)
	}
	var record BoardRecord
	if err := json.Unmarshal(encoded, &record); err != nil {
		return BoardRecord{}, err
	}
	if _, err := record.Replay(); err != nil {
		return BoardRecord{}, err
	}
	return record, nil
}

func compactFinalBoard(ctx context.Context, queries *dbgen.Queries, current table.Aggregate, next table.Aggregate, occurredAt time.Time) error {
	if current.Game == nil || current.Game.Phase != bridge.PhaseBoardScored || current.Game.Result == nil ||
		(current.BoardID == next.BoardID && next.State != table.StateFinished) {
		return nil
	}
	finalState, err := json.Marshal(current.Game)
	if err != nil {
		return err
	}
	finalHash := fmt.Sprintf("%x", sha256.Sum256(finalState))
	stored, err := queries.LoadBoardRecord(ctx, current.BoardID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if errors.Is(err, pgx.ErrNoRows) {
		source, err := queries.LoadBoardSource(ctx, current.BoardID)
		if err != nil {
			return fmt.Errorf("load archive deal: %w", err)
		}
		record := BoardRecord{Version: 1, BoardID: current.BoardID, BoardNumber: current.BoardNumber, FinalHash: finalHash}
		if err := json.Unmarshal(source, &record.Source); err != nil {
			return err
		}
		rows, err := queries.ListBoardRecordEvents(ctx, dbgen.ListBoardRecordEventsParams{TableID: current.ID, BoardID: current.BoardID, LastSeq: current.LastSeq})
		if err != nil {
			return err
		}
		for _, row := range rows {
			if len(record.Batches) == 0 || record.Batches[len(record.Batches)-1].Revision != row.Revision {
				record.Batches = append(record.Batches, boardRecordBatch{Seq: row.Seq, Revision: row.Revision, At: row.OccurredAt.Time})
			}
			batch := &record.Batches[len(record.Batches)-1]
			if row.Seq != batch.Seq+int64(len(batch.Events)) {
				return fmt.Errorf("missing board event")
			}
			batch.Events = append(batch.Events, table.Event{Type: row.EventType, Payload: json.RawMessage(row.Payload)})
		}
		if len(rows) == 0 || rows[len(rows)-1].Seq != current.LastSeq {
			return fmt.Errorf("incomplete board event range")
		}
		if _, err := record.Replay(); err != nil {
			return fmt.Errorf("validate board compaction: %w", err)
		}
		encoded, err := json.Marshal(record)
		if err != nil {
			return err
		}
		affected, err := queries.InsertBoardRecord(ctx, dbgen.InsertBoardRecordParams{
			BoardID: current.BoardID, TableID: current.ID, FirstSeq: rows[0].Seq, LastSeq: current.LastSeq,
			FinalRevision: current.Revision, Record: encoded, CompactedAt: timestamptz(occurredAt),
		})
		if err != nil {
			return fmt.Errorf("persist board record: %w", err)
		}
		if affected != 1 {
			return fmt.Errorf("conflicting permanent board record")
		}
		stored, err = queries.LoadBoardRecord(ctx, current.BoardID)
		if err != nil {
			return err
		}
	}
	var saved BoardRecord
	if err := json.Unmarshal(stored.Record, &saved); err != nil {
		return err
	}
	if _, err := saved.Replay(); err != nil {
		return err
	}
	lastBatch := saved.Batches[len(saved.Batches)-1]
	if saved.BoardID != current.BoardID || saved.FinalHash != finalHash || stored.TableID != current.ID ||
		stored.LastSeq != current.LastSeq || stored.FinalRevision != current.Revision ||
		saved.Batches[0].Seq != stored.FirstSeq || lastBatch.Seq+int64(len(lastBatch.Events))-1 != stored.LastSeq ||
		lastBatch.Revision != stored.FinalRevision {
		return fmt.Errorf("conflicting permanent board record")
	}
	return queries.DeleteCompactedBoardEvents(ctx, current.BoardID)
}
