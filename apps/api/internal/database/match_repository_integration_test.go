//go:build integration

package database

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/bridge"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/deal"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/match"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/table"
)

type matchTestEnvironment struct {
	open    commandTestEnvironment
	closed  commandTestEnvironment
	initial match.Snapshot
}

type matchTestSource struct {
	calls  atomic.Int32
	failAt int32
}

func (source *matchTestSource) Generate(ctx context.Context) (deal.Result, error) {
	if source.calls.Add(1) == source.failAt {
		return deal.Result{}, deal.ErrUnavailable
	}
	return (deal.Deterministic{Seed: [32]byte{42}}).Generate(ctx)
}

func newMatchTestEnvironment(t *testing.T, boardCount int) matchTestEnvironment {
	t.Helper()
	open := newCommandTestEnvironment(t, 4)
	closed := newCommandTestEnvironment(t, 4)
	openTable := readyCommandTable(t, open)
	closedTable := readyCommandTable(t, closed)
	boardIDs := make([]string, boardCount)
	for _index := range boardIDs {
		boardIDs[_index] = uuid.NewString()
	}
	candidate, err := match.New(uuid.NewString(), openTable.OwnerSessionID, openTable.ID, closedTable.ID, boardIDs)
	if err != nil {
		t.Fatal(err)
	}
	assignments := make([]match.Assignment, 0, 8)
	for _index, aggregate := range []table.Aggregate{openTable, closedTable} {
		room := match.Open
		if _index == 1 {
			room = match.Closed
		}
		for _, seat := range []bridge.Seat{bridge.North, bridge.East, bridge.South, bridge.West} {
			assignments = append(assignments, match.Assignment{ParticipantID: commandSessionForSeat(t, aggregate, seat), Room: room, Seat: seat})
		}
	}
	if err := candidate.Assign(openTable.OwnerSessionID, assignments); err != nil {
		t.Fatal(err)
	}
	for _, assignment := range assignments {
		if err := candidate.SetReady(assignment.ParticipantID, true); err != nil {
			t.Fatal(err)
		}
	}
	state := candidate.PrivateSnapshot()
	if err := open.postgres.CreateMatch(t.Context(), state, time.Now()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := open.postgres.pool.Exec(ctx, "DELETE FROM bridgeyok.team_matches WHERE id=$1", state.ID); err != nil {
			t.Error(err)
		}
	})
	return matchTestEnvironment{open: open, closed: closed, initial: state}
}

func (environment matchTestEnvironment) start(t *testing.T) match.Started {
	t.Helper()
	started, err := environment.open.postgres.StartMatch(t.Context(), environment.initial.ID, environment.initial.OwnerID, 0, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return started
}

func completeMatchTestBoard(t *testing.T, environment commandTestEnvironment, passedOut bool) table.Aggregate {
	t.Helper()
	aggregate, err := environment.postgres.FindTable(t.Context(), environment.tableID)
	if err != nil {
		t.Fatal(err)
	}
	process := func(sessionID string, command table.Command) {
		result := environment.process(t, table.CommandRequest{TableID: aggregate.ID, SessionID: sessionID, RequestID: uuid.NewString(), ExpectedRevision: aggregate.Revision, Command: command})
		if result.Outcome.Status != table.CommandStatusAccepted {
			t.Fatalf("%s: %+v", command.Name, result.Outcome)
		}
		aggregate = result.Aggregate
	}
	if !passedOut {
		bid := bridge.Bid(1, bridge.StrainClubs)
		process(commandSessionForSeat(t, aggregate, aggregate.Game.Turn), table.Command{Name: table.CommandMakeCall, Call: &bid})
	}
	pass := bridge.Pass()
	for aggregate.Game.Phase == bridge.PhaseAuction {
		process(commandSessionForSeat(t, aggregate, aggregate.Game.Turn), table.Command{Name: table.CommandMakeCall, Call: &pass})
	}
	for aggregate.Game.Phase != bridge.PhaseBoardScored {
		seat := aggregate.Game.Turn
		if seat == aggregate.Game.Auction.Contract.Dummy() {
			seat = aggregate.Game.Auction.Contract.Declarer
		}
		cards, domainError := aggregate.Game.LegalCards(seat)
		if domainError != nil || len(cards) == 0 {
			t.Fatal(domainError)
		}
		process(commandSessionForSeat(t, aggregate, seat), table.Command{Name: table.CommandPlayCard, Card: &cards[0]})
	}
	return aggregate
}

func closeMatchTestBoard(t *testing.T, environment commandTestEnvironment, aggregate table.Aggregate, finish bool) table.CommandRequest {
	t.Helper()
	command := table.Command{Name: table.CommandRequestNextBoard}
	if finish {
		command.Name = table.CommandFinishTable
	}
	request := table.CommandRequest{TableID: aggregate.ID, SessionID: aggregate.OwnerSessionID, RequestID: uuid.NewString(), ExpectedRevision: aggregate.Revision, Command: command}
	result := environment.process(t, request)
	if result.Outcome.Status != table.CommandStatusAccepted {
		t.Fatalf("close board: %+v", result.Outcome)
	}
	return request
}

func TestMatchRepositoryCreateAndReadiness(t *testing.T) {
	environment := newMatchTestEnvironment(t, 2)
	postgres := environment.open.postgres
	if err := postgres.CreateMatch(t.Context(), environment.initial, time.Now()); err != nil {
		t.Fatal("idempotent creation", err)
	}
	conflicting := environment.initial
	conflicting.BoardIDs = append([]string(nil), conflicting.BoardIDs...)
	conflicting.BoardIDs[0] = uuid.NewString()
	if err := postgres.CreateMatch(t.Context(), conflicting, time.Now()); !errors.Is(err, match.ErrState) {
		t.Fatal("conflicting create accepted", err)
	}
	conflicting.ID = uuid.NewString()
	if err := postgres.CreateMatch(t.Context(), conflicting, time.Now()); !errors.Is(err, match.ErrState) {
		t.Fatal("room in multiple matches", err)
	}
	stored, err := postgres.LoadMatch(t.Context(), environment.initial.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Revision != 0 || stored.Match.PrivateSnapshot().Status != match.Waiting {
		t.Fatal("unexpected initial state")
	}
	if _, err := postgres.StartMatch(t.Context(), environment.initial.ID, environment.closed.sessions[0].ID, 0, time.Now()); !errors.Is(err, match.ErrForbidden) {
		t.Fatal("nonowner started", err)
	}
	aggregate, err := postgres.FindTable(t.Context(), environment.open.tableID)
	if err != nil {
		t.Fatal(err)
	}
	result := environment.open.process(t, table.CommandRequest{TableID: aggregate.ID, SessionID: aggregate.OwnerSessionID, RequestID: uuid.NewString(), ExpectedRevision: aggregate.Revision, Command: table.Command{Name: table.CommandSetReady, Ready: false}})
	if result.Outcome.Status != table.CommandStatusAccepted {
		t.Fatal(result.Outcome)
	}
	stored, err = postgres.LoadMatch(t.Context(), environment.initial.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Revision != 1 || len(stored.Match.PrivateSnapshot().Ready) != 7 {
		t.Fatal("readiness not persisted")
	}
	if _, err := postgres.StartMatch(t.Context(), environment.initial.ID, environment.initial.OwnerID, 0, time.Now()); !errors.Is(err, match.ErrState) {
		t.Fatal("stale revision accepted", err)
	}
	if _, err := postgres.StartMatch(t.Context(), environment.initial.ID, environment.initial.OwnerID, 1, time.Now()); !errors.Is(err, match.ErrState) {
		t.Fatal("unready match started", err)
	}
	request := table.CommandRequest{TableID: aggregate.ID, SessionID: aggregate.OwnerSessionID, RequestID: uuid.NewString(), ExpectedRevision: result.Aggregate.Revision, Command: table.Command{Name: table.CommandSetReady, Ready: true}}
	result = environment.open.process(t, request)
	if result.Outcome.Status != table.CommandStatusAccepted {
		t.Fatal(result.Outcome)
	}
	if duplicate := environment.open.process(t, request); !duplicate.Duplicate {
		t.Fatal("ready retry not deduplicated")
	}
	stored, err = postgres.LoadMatch(t.Context(), environment.initial.ID)
	if err != nil || stored.Revision != 2 {
		t.Fatal("ready retry changed match revision", err)
	}
	if _, err := postgres.StartMatch(t.Context(), environment.initial.ID, environment.initial.OwnerID, 2, time.Now()); err != nil {
		t.Fatal(err)
	}
}

func TestMatchRepositoryStartAtomicity(t *testing.T) {
	environment := newMatchTestEnvironment(t, 2)
	postgres := environment.open.postgres
	source := &matchTestSource{failAt: 2}
	postgres.dealSource = source
	if _, err := postgres.StartMatch(t.Context(), environment.initial.ID, environment.initial.OwnerID, 0, time.Now()); !errors.Is(err, deal.ErrUnavailable) {
		t.Fatal(err)
	}
	stored, err := postgres.LoadMatch(t.Context(), environment.initial.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Revision != 0 || len(stored.Match.PrivateSnapshot().Boards) != 0 {
		t.Fatal("partial shared set")
	}
	tableIDs := []string{environment.open.tableID, environment.closed.tableID}
	slices.Sort(tableIDs)
	if _, err := postgres.pool.Exec(t.Context(), fmt.Sprintf("ALTER TABLE bridgeyok.boards ADD CONSTRAINT phase5_start_failure CHECK (table_id <> '%s'::uuid) NOT VALID", tableIDs[1])); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = postgres.pool.Exec(context.Background(), "ALTER TABLE bridgeyok.boards DROP CONSTRAINT IF EXISTS phase5_start_failure")
	})
	if result, err := postgres.StartMatch(t.Context(), environment.initial.ID, environment.initial.OwnerID, 0, time.Now()); err == nil || len(result.Tables) != 0 {
		t.Fatal("failed transaction returned publishable batches", err)
	}
	for _, tableID := range tableIDs {
		aggregate, err := postgres.FindTable(t.Context(), tableID)
		if err != nil {
			t.Fatal(err)
		}
		if aggregate.BoardID != "" || aggregate.State != table.StateWaiting {
			t.Fatal("partial table start")
		}
	}
	var sources int
	if err := postgres.pool.QueryRow(t.Context(), "SELECT count(*) FROM bridgeyok.match_boards WHERE match_id=$1 AND source_record IS NOT NULL", environment.initial.ID).Scan(&sources); err != nil || sources != 0 {
		t.Fatal("source transaction not rolled back", err)
	}
	if _, err := postgres.pool.Exec(t.Context(), "ALTER TABLE bridgeyok.boards DROP CONSTRAINT phase5_start_failure"); err != nil {
		t.Fatal(err)
	}
	started := environment.start(t)
	if len(started.Tables) != 2 || started.Revision != 1 || source.calls.Load() != 6 {
		t.Fatal("start not atomic", source.calls.Load())
	}
	open, err := postgres.FindTable(t.Context(), environment.open.tableID)
	if err != nil {
		t.Fatal(err)
	}
	closed, err := postgres.FindTable(t.Context(), environment.closed.tableID)
	if err != nil {
		t.Fatal(err)
	}
	if open.BoardID == closed.BoardID || !reflect.DeepEqual(open.Game.Deal, closed.Game.Deal) || open.Game.Board != closed.Game.Board {
		t.Fatal("shared board identity/metadata mismatch")
	}
	for _, tableID := range tableIDs {
		var count int
		if err := postgres.pool.QueryRow(t.Context(), "SELECT count(*) FROM bridgeyok.match_room_boards WHERE match_id=$1 AND table_id=$2 AND board_id=$3", environment.initial.ID, tableID, environment.initial.BoardIDs[0]).Scan(&count); err != nil || count != 1 {
			t.Fatal("shared binding missing", err)
		}
	}
	duplicate, err := postgres.StartMatch(t.Context(), environment.initial.ID, environment.initial.OwnerID, 0, time.Now())
	if err != nil || !duplicate.Duplicate || len(duplicate.Tables) != 0 || source.calls.Load() != 6 {
		t.Fatal("retry regenerated start", err)
	}
	if _, err := postgres.pool.Exec(t.Context(), "UPDATE bridgeyok.match_boards SET source_record='{}' WHERE match_id=$1", environment.initial.ID); err == nil {
		t.Fatal("mutable shared source")
	}
}

func TestMatchRepositoryIndependentProgressAndRestart(t *testing.T) {
	environment := newMatchTestEnvironment(t, 2)
	environment.open.postgres.dealSource = &matchTestSource{}
	environment.start(t)
	openFirst := completeMatchTestBoard(t, environment.open, true)
	stored, err := environment.open.postgres.LoadMatch(t.Context(), environment.initial.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(stored.Match.PrivateSnapshot().Results) != 0 {
		t.Fatal("unsealed score collected")
	}
	closeMatchTestBoard(t, environment.open, openFirst, false)
	openSecond := completeMatchTestBoard(t, environment.open, false)
	finalRequest := closeMatchTestBoard(t, environment.open, openSecond, true)
	restarted, err := Open(t.Context(), os.Getenv("TEST_DATABASE_URL"), 4)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	stored, err = restarted.LoadMatch(t.Context(), environment.initial.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Match.PrivateSnapshot().Status != match.Active || len(stored.Match.PrivateSnapshot().Results) != 2 {
		t.Fatal("fast room not durably independent")
	}
	if comparisons, total := stored.Match.Comparisons(); len(comparisons) != 0 || total != 0 {
		t.Fatal("unpaired result compared")
	}
	if view, err := stored.Match.Project(environment.closed.sessions[0].ID); err != nil || len(view.Comparisons) != 0 || view.TeamAIMP != 0 {
		t.Fatal("partial result disclosure", err)
	}
	closed, err := restarted.FindTable(t.Context(), environment.closed.tableID)
	if err != nil {
		t.Fatal(err)
	}
	if closed.BoardNumber != 1 || closed.Game.Phase != bridge.PhaseAuction {
		t.Fatal("slow room advanced")
	}
	closedFirst := completeMatchTestBoard(t, environment.closed, false)
	closeMatchTestBoard(t, environment.closed, closedFirst, false)
	closedSecond := completeMatchTestBoard(t, environment.closed, true)
	closeMatchTestBoard(t, environment.closed, closedSecond, true)
	stored, err = restarted.LoadMatch(t.Context(), environment.initial.ID)
	if err != nil {
		t.Fatal(err)
	}
	comparisons, total := stored.Match.Comparisons()
	firstIMP, err := match.CompareScores(openFirst.Game.Result.ScoreNS, closedFirst.Game.Result.ScoreNS)
	if err != nil {
		t.Fatal(err)
	}
	secondIMP, err := match.CompareScores(openSecond.Game.Result.ScoreNS, closedSecond.Game.Result.ScoreNS)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Match.PrivateSnapshot().Status != match.Complete || len(comparisons) != 2 || total != firstIMP+secondIMP || firstIMP == 0 || secondIMP == 0 {
		t.Fatalf("wrong final %v %d, expected %d+%d", comparisons, total, firstIMP, secondIMP)
	}
	beforeRevision := stored.Revision
	result, err := restarted.ProcessCommand(t.Context(), finalRequest, time.Now(), time.Now().Add(time.Hour))
	if err != nil || !result.Duplicate {
		t.Fatal("post-restart retry", err)
	}
	stored, err = restarted.LoadMatch(t.Context(), environment.initial.ID)
	if err != nil || stored.Revision != beforeRevision {
		t.Fatal("retry changed result", err)
	}
	for _, boardID := range []string{openFirst.BoardID, openSecond.BoardID, closedFirst.BoardID, closedSecond.BoardID} {
		err := pgx.BeginFunc(t.Context(), restarted.pool, func(tx pgx.Tx) error {
			binding, err := restarted.queries.FindMatchRoomBoard(t.Context(), boardID)
			if err != nil {
				return err
			}
			return collectFinalMatchBoard(t.Context(), restarted.queries.WithTx(tx), binding.TableID, boardID, time.Now())
		})
		if err != nil {
			t.Fatal("duplicate archive delivery", err)
		}
	}
	stored, err = restarted.LoadMatch(t.Context(), environment.initial.ID)
	if err != nil || stored.Revision != beforeRevision {
		t.Fatal("archive retry changed total", err)
	}
	if _, err := restarted.pool.Exec(t.Context(), "UPDATE bridgeyok.boards SET score_ns=score_ns+10 WHERE id=$1", openFirst.BoardID); err == nil {
		t.Fatal("sealed score changed")
	}
	if _, err := restarted.pool.Exec(t.Context(), "UPDATE bridgeyok.team_matches SET total_imp=total_imp+1 WHERE id=$1", environment.initial.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := restarted.LoadMatch(t.Context(), environment.initial.ID); err == nil {
		t.Fatal("corrupt aggregate accepted")
	}
}

func TestMatchRepositoryConcurrentStartAndFinalization(t *testing.T) {
	environment := newMatchTestEnvironment(t, 1)
	postgres := environment.open.postgres
	source := &matchTestSource{}
	postgres.dealSource = source
	var group sync.WaitGroup
	starts := make(chan match.Started, 8)
	failures := make(chan error, 8)
	for _index := 0; _index < 8; _index++ {
		group.Go(func() {
			result, err := postgres.StartMatch(t.Context(), environment.initial.ID, environment.initial.OwnerID, 0, time.Now())
			starts <- result
			failures <- err
		})
	}
	group.Wait()
	close(starts)
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	committed := 0
	for result := range starts {
		if !result.Duplicate {
			committed++
		}
	}
	if committed != 1 || source.calls.Load() != 1 {
		t.Fatal("concurrent start generated multiple sets")
	}
	open := completeMatchTestBoard(t, environment.open, true)
	closed := completeMatchTestBoard(t, environment.closed, true)
	requests := []table.CommandRequest{}
	for _, aggregate := range []table.Aggregate{open, closed} {
		requests = append(requests, table.CommandRequest{TableID: aggregate.ID, SessionID: aggregate.OwnerSessionID, RequestID: uuid.NewString(), ExpectedRevision: aggregate.Revision, Command: table.Command{Name: table.CommandFinishTable}})
	}
	failures = make(chan error, 16)
	for _index := 0; _index < 16; _index++ {
		request := requests[_index%2]
		group.Go(func() {
			request.Command.SessionID = request.SessionID
			result, err := postgres.ProcessCommand(t.Context(), request, time.Now(), time.Now().Add(time.Hour))
			if err == nil && result.Outcome.Status != table.CommandStatusAccepted {
				err = fmt.Errorf("rejected result: %s", result.Outcome.ErrorCode)
			}
			failures <- err
		})
	}
	group.Wait()
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	stored, err := postgres.LoadMatch(t.Context(), environment.initial.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Revision != 3 || stored.Match.PrivateSnapshot().Status != match.Complete {
		t.Fatal("duplicate finalization changed revision")
	}
	comparisons, total := stored.Match.Comparisons()
	if len(comparisons) != 1 || total != 0 {
		t.Fatal("passed-out comparison")
	}
}

func TestMatchRepositoryFinalizationRollback(t *testing.T) {
	environment := newMatchTestEnvironment(t, 1)
	environment.start(t)
	open := completeMatchTestBoard(t, environment.open, true)
	closeMatchTestBoard(t, environment.open, open, true)
	closed := completeMatchTestBoard(t, environment.closed, true)
	postgres := environment.open.postgres
	if _, err := postgres.pool.Exec(t.Context(), fmt.Sprintf("ALTER TABLE bridgeyok.match_comparisons ADD CONSTRAINT phase5_comparison_failure CHECK (match_id <> '%s'::uuid) NOT VALID", environment.initial.ID)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = postgres.pool.Exec(context.Background(), "ALTER TABLE bridgeyok.match_comparisons DROP CONSTRAINT IF EXISTS phase5_comparison_failure")
	})
	request := table.CommandRequest{TableID: closed.ID, SessionID: closed.OwnerSessionID, RequestID: uuid.NewString(), ExpectedRevision: closed.Revision, Command: table.Command{Name: table.CommandFinishTable, SessionID: closed.OwnerSessionID}}
	if result, err := postgres.ProcessCommand(t.Context(), request, time.Now(), time.Now().Add(time.Hour)); err == nil || len(result.Events) != 0 {
		t.Fatal("failed finalization returned success", err)
	}
	restored, err := postgres.FindTable(t.Context(), closed.ID)
	if err != nil || !reflect.DeepEqual(closed, restored) {
		t.Fatal("closing snapshot not rolled back", err)
	}
	if _, err := postgres.queries.LoadBoardRecord(t.Context(), closed.BoardID); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatal("archive survived rollback", err)
	}
	stored, err := postgres.LoadMatch(t.Context(), environment.initial.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Revision != 2 || len(stored.Match.PrivateSnapshot().Results) != 1 {
		t.Fatal("partial finalization")
	}
	if _, err := postgres.pool.Exec(t.Context(), "ALTER TABLE bridgeyok.match_comparisons DROP CONSTRAINT phase5_comparison_failure"); err != nil {
		t.Fatal(err)
	}
	result, err := postgres.ProcessCommand(t.Context(), request, time.Now(), time.Now().Add(time.Hour))
	if err != nil || result.Outcome.Status != table.CommandStatusAccepted {
		t.Fatal("retry after rollback", err)
	}
	stored, err = postgres.LoadMatch(t.Context(), environment.initial.ID)
	if err != nil || stored.Match.PrivateSnapshot().Status != match.Complete {
		t.Fatal("recovery incomplete", err)
	}
}

func TestMatchRepositoryRejectsMissingBoardBinding(t *testing.T) {
	environment := newMatchTestEnvironment(t, 1)
	environment.start(t)
	aggregate := completeMatchTestBoard(t, environment.open, true)
	postgres := environment.open.postgres
	if _, err := postgres.pool.Exec(t.Context(), "DELETE FROM bridgeyok.match_room_boards WHERE table_board_id=$1", aggregate.BoardID); err != nil {
		t.Fatal(err)
	}
	request := table.CommandRequest{TableID: aggregate.ID, SessionID: aggregate.OwnerSessionID, RequestID: uuid.NewString(), ExpectedRevision: aggregate.Revision, Command: table.Command{Name: table.CommandFinishTable}}
	if _, err := environment.open.processor.Process(t.Context(), request); !errors.Is(err, match.ErrInvalid) {
		t.Fatal("missing shared binding silently discarded result", err)
	}
	recovered, err := postgres.FindTable(t.Context(), aggregate.ID)
	if err != nil || recovered.Revision != aggregate.Revision || recovered.State != table.StateBetweenBoards {
		t.Fatal("corrupt binding did not roll back", err)
	}
}
