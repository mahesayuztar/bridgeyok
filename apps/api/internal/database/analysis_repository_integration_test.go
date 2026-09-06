//go:build integration

package database

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"context"
	"github.com/google/uuid"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/analysis"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/bridge"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/deal"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/table"
)

type countedSource struct {
	source deal.Source
	calls  int
	fail   bool
}

func (source *countedSource) Generate(ctx context.Context) (deal.Result, error) {
	source.calls++
	if source.fail {
		return deal.Result{}, deal.ErrUnavailable
	}
	return source.source.Generate(ctx)
}

func TestBoardSourcesPersistAtomicallyAndAuthorizeAnalysis(t *testing.T) {
	generated, err := (deal.Deterministic{Seed: [32]byte{7}}).Generate(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := deal.NewPrepared(generated.Deal, uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	constrained, err := deal.NewConstraint(map[bridge.Seat]deal.HandConstraint{bridge.North: {HCP: &deal.Range{Min: 0, Max: 37}}}, 1, uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name   string
		source deal.Source
	}{{"secure_random", deal.SecureRandom{}}, {"deterministic", deal.Deterministic{Seed: [32]byte{7}}}, {"prepared", prepared}, {"constraint", constrained}} {
		t.Run(test.name, func(t *testing.T) {
			environment := newCommandTestEnvironment(t, 4)
			aggregate := readyCommandTable(t, environment)
			source := &countedSource{source: test.source}
			environment.postgres.dealSource = source
			request := table.CommandRequest{TableID: environment.tableID, SessionID: environment.sessions[0].ID, RequestID: "source_start_01", ExpectedRevision: aggregate.Revision, Command: table.Command{Name: table.CommandStartGame}}
			unauthorized := request
			unauthorized.SessionID = environment.sessions[1].ID
			unauthorized.RequestID = "source_unauthorized"
			if result := environment.process(t, unauthorized); result.Outcome.Status != table.CommandStatusRejected || source.calls != 0 {
				t.Fatal("unauthorized request generated a deal")
			}
			stale := request
			stale.ExpectedRevision--
			stale.RequestID = "source_stale_request"
			if result := environment.process(t, stale); result.Outcome.Status != table.CommandStatusRejected || source.calls != 0 {
				t.Fatal("stale request generated a deal")
			}
			source.fail = true
			if _, err := environment.processor.Process(environment.ctx, request); !errors.Is(err, deal.ErrUnavailable) {
				t.Fatalf("failed source: %v", err)
			}
			restored, err := environment.postgres.FindTable(environment.ctx, environment.tableID)
			if err != nil {
				t.Fatal(err)
			}
			if restored.Revision != aggregate.Revision || restored.LastSeq != aggregate.LastSeq || restored.BoardID != "" {
				t.Fatal("source failure changed authoritative state")
			}
			var count int
			if err := environment.postgres.pool.QueryRow(environment.ctx, "SELECT count(*) FROM bridgeyok.boards WHERE table_id = $1", environment.tableID).Scan(&count); err != nil || count != 0 {
				t.Fatal("source failure persisted partial board")
			}
			source.fail = false
			started := environment.process(t, request)
			if started.Outcome.Status != table.CommandStatusAccepted || source.calls != 2 {
				t.Fatal("board not generated once after successful retry")
			}
			duplicate := environment.process(t, request)
			if !duplicate.Duplicate || duplicate.Aggregate.BoardID != started.Aggregate.BoardID || source.calls != 2 {
				t.Fatal("duplicate regenerated a board")
			}
			boardID := started.Aggregate.BoardID
			if _, err := environment.postgres.CompletedAnalysisBoard(environment.ctx, boardID, environment.sessions[0].ID); !errors.Is(err, analysis.ErrNotCompleted) {
				t.Fatal("active board exposed to analysis")
			}
			if _, err := environment.postgres.CompletedAnalysisBoard(environment.ctx, boardID, uuid.NewString()); !errors.Is(err, analysis.ErrNotFound) {
				t.Fatal("outsider discovered board")
			}
			aggregate = started.Aggregate
			for _callIndex := 0; _callIndex < 4; _callIndex++ {
				call := bridge.Pass()
				result := environment.process(t, table.CommandRequest{TableID: environment.tableID, SessionID: commandSessionForSeat(t, aggregate, aggregate.Game.Turn), RequestID: "source_pass_" + string(rune('a'+_callIndex)), ExpectedRevision: aggregate.Revision, Command: table.Command{Name: table.CommandMakeCall, Call: &call}})
				aggregate = result.Aggregate
			}
			board, err := environment.postgres.CompletedAnalysisBoard(environment.ctx, boardID, environment.sessions[0].ID)
			if err != nil {
				t.Fatal(err)
			}
			if board.Source.Provenance.Type != test.name || board.Metadata != started.Aggregate.Game.Board || !reflect.DeepEqual(board.Source.Deal, started.Aggregate.Game.Deal) {
				t.Fatal("persisted identity, deal, or provenance changed")
			}
			reopened := &Postgres{pool: environment.postgres.pool, queries: environment.postgres.queries}
			recovered, err := reopened.CompletedAnalysisBoard(environment.ctx, boardID, environment.sessions[0].ID)
			if err != nil || !reflect.DeepEqual(recovered, board) {
				t.Fatal("repository restart changed board source")
			}
			next := environment.process(t, table.CommandRequest{TableID: environment.tableID, SessionID: environment.sessions[0].ID, RequestID: "source_next_board", ExpectedRevision: aggregate.Revision, Command: table.Command{Name: table.CommandRequestNextBoard}})
			if next.Aggregate.BoardID == boardID || next.Aggregate.Game.Board.Number != 2 {
				t.Fatal("next board reused identity")
			}
			recovered, err = reopened.CompletedAnalysisBoard(environment.ctx, boardID, environment.sessions[0].ID)
			if err != nil || !reflect.DeepEqual(recovered, board) {
				t.Fatal("next board overwrote historical source")
			}
			projection, domainError := table.Project(next.Aggregate, environment.sessions[0].ID)
			if domainError != nil {
				t.Fatal(domainError)
			}
			encoded, err := json.Marshal(projection)
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(encoded, &fields); err != nil {
				t.Fatal(err)
			}
			if fields["sourceRecord"] != nil || fields["provenance"] != nil {
				t.Fatal("private source projected to realtime")
			}
		})
	}
}
