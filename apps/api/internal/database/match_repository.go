package database

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/bridge"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/database/dbgen"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/deal"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/match"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/table"
)

type StoredMatch struct {
	Match    *match.Match
	Revision int64
}

type StartedMatch struct {
	StoredMatch
	Tables    []table.CommandResult
	Duplicate bool
}

// CreateMatch binds two prepared waiting tables. Assignment participant IDs are stable session IDs, not table-local participant IDs.
func (postgres *Postgres) CreateMatch(ctx context.Context, state match.Snapshot, occurredAt time.Time) error {
	candidate, err := match.Restore(state)
	if err != nil {
		return err
	}
	if state.Status != match.Waiting || len(state.Assignments) != 8 {
		return match.ErrInvalid
	}
	state = candidate.PrivateSnapshot()
	identifiers := append([]string{state.ID, state.OwnerID, state.OpenTableID, state.ClosedTableID}, state.BoardIDs...)
	for _, assignment := range state.Assignments {
		identifiers = append(identifiers, assignment.ParticipantID)
	}
	for _, id := range identifiers {
		if _, err := uuid.Parse(id); err != nil {
			return match.ErrInvalid
		}
	}
	slices.SortFunc(state.Assignments, func(a, b match.Assignment) int {
		return strings.Compare(string(a.Room)+string(a.Seat), string(b.Room)+string(b.Seat))
	})
	slices.Sort(state.Ready)
	encoded, err := json.Marshal(state)
	if err != nil {
		return err
	}
	creationHash := sha256.Sum256(encoded)
	err = pgx.BeginFunc(ctx, postgres.pool, func(tx pgx.Tx) error {
		queries := postgres.queries.WithTx(tx)
		tables, err := lockMatchTables(ctx, queries, []string{state.OpenTableID, state.ClosedTableID})
		if err != nil {
			return err
		}
		existing, err := queries.LoadMatch(ctx, state.ID)
		if err == nil {
			if !bytes.Equal(existing.CreationHash, creationHash[:]) {
				return match.ErrState
			}
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		for _, aggregate := range tables {
			if aggregate.State != table.StateWaiting || aggregate.BoardID != "" || len(aggregate.Seats) != 4 {
				return match.ErrState
			}
			activeCount := 0
			for _, participant := range aggregate.Participants {
				if participant.LeftAt == nil {
					activeCount++
				}
			}
			if activeCount != 4 {
				return match.ErrInvalid
			}
			room := match.Open
			if aggregate.ID == state.ClosedTableID {
				room = match.Closed
			}
			for _, assignment := range state.Assignments {
				if assignment.Room != room {
					continue
				}
				seat := aggregate.Seats[assignment.Seat]
				found := false
				for _, participant := range aggregate.Participants {
					if participant.ID == seat.ParticipantID && participant.SessionID == assignment.ParticipantID && participant.LeftAt == nil {
						found = true
					}
				}
				if !found || seat.IsBot || seat.Ready != slices.Contains(state.Ready, assignment.ParticipantID) {
					return match.ErrInvalid
				}
			}
		}
		ownerIsTableOwner := slices.ContainsFunc(tables, func(aggregate table.Aggregate) bool { return aggregate.OwnerSessionID == state.OwnerID })
		if !ownerIsTableOwner {
			return match.ErrForbidden
		}
		if err := queries.CreateMatch(ctx, dbgen.CreateMatchParams{ID: state.ID, OwnerSessionID: state.OwnerID, BoardCount: int32(len(state.BoardIDs)), CreationHash: creationHash[:], CreatedAt: timestamptz(occurredAt)}); err != nil {
			return err
		}
		for _, room := range []struct {
			name    match.Room
			tableID string
		}{{match.Open, state.OpenTableID}, {match.Closed, state.ClosedTableID}} {
			if err := queries.InsertMatchRoom(ctx, dbgen.InsertMatchRoomParams{MatchID: state.ID, Room: string(room.name), TableID: room.tableID}); err != nil {
				return err
			}
		}
		for _, assignment := range state.Assignments {
			if err := queries.InsertMatchAssignment(ctx, dbgen.InsertMatchAssignmentParams{MatchID: state.ID, Room: string(assignment.Room), Seat: string(assignment.Seat), SessionID: assignment.ParticipantID, Ready: slices.Contains(state.Ready, assignment.ParticipantID)}); err != nil {
				return err
			}
		}
		for _index, boardID := range state.BoardIDs {
			if err := queries.InsertMatchBoard(ctx, dbgen.InsertMatchBoardParams{MatchID: state.ID, ID: boardID, BoardNumber: int32(_index + 1)}); err != nil {
				return err
			}
		}
		return nil
	})
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) && postgresError.Code == "23505" {
		return match.ErrState
	}
	return err
}

// LoadMatch returns private recovery state from a consistent database snapshot, never an HTTP response.
func (postgres *Postgres) LoadMatch(ctx context.Context, matchID string) (StoredMatch, error) {
	tx, err := postgres.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return StoredMatch{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := postgres.queries.WithTx(tx)
	row, err := queries.LoadMatch(ctx, matchID)
	if err != nil {
		return StoredMatch{}, err
	}
	result, err := hydrateMatch(ctx, queries, row)
	if err != nil {
		return StoredMatch{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return StoredMatch{}, err
	}
	return result, nil
}

// StartMatch commits the shared deals and both first-board snapshots together. Returned batches may be published only after success.
func (postgres *Postgres) StartMatch(ctx context.Context, matchID, actorID string, expectedRevision int64, occurredAt time.Time) (StartedMatch, error) {
	var result StartedMatch
	err := pgx.BeginFunc(ctx, postgres.pool, func(tx pgx.Tx) error {
		queries := postgres.queries.WithTx(tx)
		rooms, err := queries.ListMatchRooms(ctx, matchID)
		if err != nil {
			return err
		}
		if len(rooms) != 2 {
			return match.ErrInvalid
		}
		tables, err := lockMatchTables(ctx, queries, []string{rooms[0].TableID, rooms[1].TableID})
		if err != nil {
			return err
		}
		row, err := queries.LockMatch(ctx, matchID)
		if err != nil {
			return err
		}
		if row.OwnerSessionID != actorID {
			return match.ErrForbidden
		}
		stored, err := hydrateMatch(ctx, queries, row)
		if err != nil {
			return err
		}
		if row.Status != string(match.Waiting) {
			result = StartedMatch{StoredMatch: stored, Duplicate: true}
			return nil
		}
		if row.Revision != expectedRevision {
			return match.ErrState
		}
		source := postgres.dealSource
		if source == nil {
			source = deal.SecureRandom{}
		}
		if err := stored.Match.Start(ctx, actorID, source); err != nil {
			return err
		}
		state := stored.Match.PrivateSnapshot()
		for _, board := range state.Boards {
			encoded, err := json.Marshal(board.Source)
			if err != nil {
				return err
			}
			affected, err := queries.SetMatchBoardSource(ctx, dbgen.SetMatchBoardSourceParams{MatchID: matchID, ID: board.ID, SourceRecord: encoded})
			if err != nil {
				return err
			}
			if affected != 1 {
				return match.ErrState
			}
		}
		first := state.Boards[0]
		for _, aggregate := range tables {
			command := table.Command{Name: table.CommandStartGame, SessionID: aggregate.OwnerSessionID, BoardID: uuid.NewString(), Deal: &first.Source.Deal, DealProvenance: &first.Source.Provenance}
			decision, domainError := table.Decide(aggregate, command)
			if domainError != nil {
				return domainError
			}
			started, err := persistAcceptedDecision(ctx, queries, table.CommandRequest{TableID: aggregate.ID, SessionID: aggregate.OwnerSessionID, RequestID: "match_start", Command: command}, aggregate, decision, occurredAt)
			if err != nil {
				return err
			}
			room := match.Open
			if aggregate.ID == state.ClosedTableID {
				room = match.Closed
			}
			if err := queries.InsertMatchRoomBoard(ctx, dbgen.InsertMatchRoomBoardParams{MatchID: matchID, BoardID: first.ID, Room: string(room), TableID: aggregate.ID, TableBoardID: started.Aggregate.BoardID}); err != nil {
				return err
			}
			result.Tables = append(result.Tables, started)
		}
		if err := saveMatchProgress(ctx, queries, stored, occurredAt); err != nil {
			return err
		}
		stored.Revision++
		result.StoredMatch = stored
		return nil
	})
	if err != nil {
		return StartedMatch{}, err
	}
	return result, nil
}

// lockMatchTables establishes the shared lock order: sorted table rows, then the match row.
func lockMatchTables(ctx context.Context, queries *dbgen.Queries, tableIDs []string) ([]table.Aggregate, error) {
	slices.Sort(tableIDs)
	aggregates := make([]table.Aggregate, 0, len(tableIDs))
	for _, tableID := range tableIDs {
		row, err := queries.LockTableByID(ctx, tableID)
		if err != nil {
			return nil, err
		}
		aggregate, err := loadTableAggregate(ctx, queries, tableRow{id: row.ID, ownerSessionID: row.OwnerSessionID, state: row.State, locked: row.Locked, revision: row.Revision, lastSeq: row.LastSeq})
		if err != nil {
			return nil, err
		}
		aggregates = append(aggregates, aggregate)
	}
	return aggregates, nil
}

func hydrateMatch(ctx context.Context, queries *dbgen.Queries, row dbgen.BridgeyokTeamMatch) (StoredMatch, error) {
	rooms, err := queries.ListMatchRooms(ctx, row.ID)
	if err != nil {
		return StoredMatch{}, err
	}
	if len(rooms) != 2 {
		return StoredMatch{}, match.ErrInvalid
	}
	state := match.Snapshot{ID: row.ID, OwnerID: row.OwnerSessionID, Status: match.Status(row.Status)}
	for _, room := range rooms {
		if room.Room == string(match.Open) {
			state.OpenTableID = room.TableID
		} else {
			state.ClosedTableID = room.TableID
		}
	}
	assignments, err := queries.ListMatchAssignments(ctx, row.ID)
	if err != nil {
		return StoredMatch{}, err
	}
	for _, assignment := range assignments {
		state.Assignments = append(state.Assignments, match.Assignment{ParticipantID: assignment.SessionID, Room: match.Room(assignment.Room), Seat: bridge.Seat(assignment.Seat)})
		if assignment.Ready {
			state.Ready = append(state.Ready, assignment.SessionID)
		}
	}
	boards, err := queries.ListMatchBoards(ctx, row.ID)
	if err != nil {
		return StoredMatch{}, err
	}
	if len(boards) != int(row.BoardCount) {
		return StoredMatch{}, match.ErrInvalid
	}
	for _index, board := range boards {
		if int(board.BoardNumber) != _index+1 {
			return StoredMatch{}, match.ErrInvalid
		}
		state.BoardIDs = append(state.BoardIDs, board.ID)
		if state.Status == match.Waiting {
			if board.SourceRecord != nil {
				return StoredMatch{}, match.ErrInvalid
			}
			continue
		}
		metadata, err := bridge.MetadataForBoard(int(board.BoardNumber))
		if err != nil {
			return StoredMatch{}, err
		}
		var source deal.Result
		if err := json.Unmarshal(board.SourceRecord, &source); err != nil {
			return StoredMatch{}, fmt.Errorf("decode match board source")
		}
		state.Boards = append(state.Boards, match.Board{ID: board.ID, Metadata: metadata, Source: source})
	}
	results, err := queries.ListMatchResults(ctx, row.ID)
	if err != nil {
		return StoredMatch{}, err
	}
	for _, result := range results {
		state.Results = append(state.Results, match.Result{BoardID: result.BoardID, Room: match.Room(result.Room), ScoreNS: int(result.ScoreNs)})
	}
	candidate, err := match.Restore(state)
	if err != nil {
		return StoredMatch{}, err
	}
	comparisons, total := candidate.Comparisons()
	persisted, err := queries.ListMatchComparisons(ctx, row.ID)
	if err != nil {
		return StoredMatch{}, err
	}
	if total != int(row.TotalImp) || len(comparisons) != len(persisted) {
		return StoredMatch{}, fmt.Errorf("match comparison totals mismatch")
	}
	for _index, comparison := range comparisons {
		if persisted[_index].BoardID != comparison.BoardID || int(persisted[_index].TeamAImp) != comparison.TeamAIMP {
			return StoredMatch{}, fmt.Errorf("match comparison mismatch")
		}
	}
	return StoredMatch{Match: candidate, Revision: row.Revision}, nil
}

func saveMatchProgress(ctx context.Context, queries *dbgen.Queries, stored StoredMatch, occurredAt time.Time) error {
	state := stored.Match.PrivateSnapshot()
	_, total := stored.Match.Comparisons()
	affected, err := queries.UpdateMatch(ctx, dbgen.UpdateMatchParams{ID: state.ID, Status: string(state.Status), TotalImp: int32(total), UpdatedAt: timestamptz(occurredAt), Revision: stored.Revision})
	if err != nil {
		return err
	}
	if affected != 1 {
		return match.ErrState
	}
	return nil
}

// collectFinalMatchBoard runs inside the table's closing transaction, after its validated archive is stored.
func collectFinalMatchBoard(ctx context.Context, queries *dbgen.Queries, tableID, tableBoardID string, occurredAt time.Time) error {
	binding, err := queries.FindMatchRoomBoard(ctx, tableBoardID)
	if errors.Is(err, pgx.ErrNoRows) {
		if _, roomError := queries.FindTableMatch(ctx, tableID); roomError == nil {
			return match.ErrInvalid
		} else if !errors.Is(roomError, pgx.ErrNoRows) {
			return roomError
		}
		return nil
	}
	if err != nil {
		return err
	}
	if binding.TableID != tableID {
		return match.ErrInvalid
	}
	row, err := queries.LockMatch(ctx, binding.MatchID)
	if err != nil {
		return err
	}
	stored, err := hydrateMatch(ctx, queries, row)
	if err != nil {
		return err
	}
	archive, err := queries.LoadBoardRecord(ctx, tableBoardID)
	if err != nil {
		return err
	}
	var record BoardRecord
	if err := json.Unmarshal(archive.Record, &record); err != nil {
		return fmt.Errorf("decode match board archive")
	}
	game, err := record.Replay()
	if err != nil {
		return err
	}
	state := stored.Match.PrivateSnapshot()
	_index := slices.Index(state.BoardIDs, binding.BoardID)
	if _index < 0 || _index >= len(state.Boards) || record.BoardID != tableBoardID || archive.TableID != binding.TableID || game.Board != state.Boards[_index].Metadata {
		return match.ErrInvalid
	}
	sourceJSON, err := json.Marshal(record.Source)
	if err != nil {
		return err
	}
	sharedJSON, err := json.Marshal(state.Boards[_index].Source)
	if err != nil {
		return err
	}
	if !bytes.Equal(sourceJSON, sharedJSON) {
		return fmt.Errorf("match board source mismatch")
	}
	result := match.Result{BoardID: binding.BoardID, Room: match.Room(binding.Room), ScoreNS: game.Result.ScoreNS}
	before := len(state.Results)
	if err := stored.Match.RecordResult(binding.TableID, result); err != nil {
		return err
	}
	if len(stored.Match.PrivateSnapshot().Results) == before {
		return nil
	}
	if err := queries.InsertMatchResult(ctx, dbgen.InsertMatchResultParams{MatchID: binding.MatchID, BoardID: binding.BoardID, Room: binding.Room, TableBoardID: tableBoardID, ScoreNs: int32(result.ScoreNS), FinalizedAt: timestamptz(occurredAt)}); err != nil {
		return err
	}
	comparisons, _ := stored.Match.Comparisons()
	for _, comparison := range comparisons {
		if _, err := queries.InsertMatchComparison(ctx, dbgen.InsertMatchComparisonParams{MatchID: binding.MatchID, BoardID: comparison.BoardID, TeamAImp: int32(comparison.TeamAIMP), ComparedAt: timestamptz(occurredAt)}); err != nil {
			return err
		}
	}
	return saveMatchProgress(ctx, queries, stored, occurredAt)
}
