//go:build integration

package database

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/analysis"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/bridge"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/database/dbgen"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/table"
)

func TestBoardRecordFinalityAndRecovery(t *testing.T) {
	tests := []struct {
		name      string
		claim     bool
		passedOut bool
		undo      bool
		finish    bool
	}{
		{name: "passed out then next", passedOut: true},
		{name: "passed out undo then finish", passedOut: true, undo: true, finish: true},
		{name: "thirteen tricks then next"},
		{name: "accepted claim then finish", claim: true, finish: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			environment := newCommandTestEnvironment(t, 4)
			aggregate := readyCommandTable(t, environment)
			process := func(sessionID string, command table.Command) table.CommandResult {
				result := environment.process(t, table.CommandRequest{TableID: aggregate.ID, SessionID: sessionID, RequestID: uuid.NewString(), ExpectedRevision: aggregate.Revision, Command: command})
				if result.Outcome.Status != table.CommandStatusAccepted {
					t.Fatalf("%s: %+v", command.Name, result.Outcome)
				}
				aggregate = result.Aggregate
				return result
			}
			started := process(aggregate.OwnerSessionID, table.Command{Name: table.CommandStartGame})
			firstSeq := started.Events[0].Seq
			if _, err := environment.postgres.CompletedBoardReplay(environment.ctx, aggregate.BoardID, aggregate.OwnerSessionID); !errors.Is(err, analysis.ErrNotCompleted) {
				t.Fatalf("unfinished replay: %v", err)
			}
			boardID := aggregate.BoardID
			pass := bridge.Pass()
			if !test.passedOut {
				bid := bridge.Bid(1, bridge.StrainClubs)
				process(commandSessionForSeat(t, aggregate, aggregate.Game.Turn), table.Command{Name: table.CommandMakeCall, Call: &bid})
			}
			process(commandSessionForSeat(t, aggregate, aggregate.Game.Turn), table.Command{Name: table.CommandMakeCall, Call: &pass})
			restarted, err := Open(environment.ctx, os.Getenv("TEST_DATABASE_URL"), 2)
			if err != nil {
				t.Fatal(err)
			}
			defer restarted.Close()
			recovered, err := restarted.FindTable(environment.ctx, aggregate.ID)
			if err != nil || !reflect.DeepEqual(recovered, aggregate) {
				t.Fatalf("mid-board restart differs: %v", err)
			}
			gap, err := restarted.ListEventsAfter(environment.ctx, aggregate.ID, firstSeq-1, maxRecoveryEvents)
			if err != nil || len(gap) != int(aggregate.LastSeq-firstSeq+1) {
				t.Fatalf("active event gap missing: %v", err)
			}
			for _index, event := range gap {
				if event.Seq != firstSeq+int64(_index) {
					t.Fatal("active event sequence gap")
				}
			}
			var lastCaller string
			for aggregate.Game.Phase == bridge.PhaseAuction {
				lastCaller = commandSessionForSeat(t, aggregate, aggregate.Game.Turn)
				process(lastCaller, table.Command{Name: table.CommandMakeCall, Call: &pass})
			}
			if test.undo {
				process(lastCaller, table.Command{Name: table.CommandRequestUndo})
				for _, session := range environment.sessions {
					if session.ID != lastCaller {
						process(session.ID, table.Command{Name: table.CommandRespondUndo, Accepted: true})
					}
				}
				if aggregate.Game.Phase != bridge.PhaseAuction {
					t.Fatal("scored board could not be undone")
				}
				process(lastCaller, table.Command{Name: table.CommandMakeCall, Call: &pass})
			}
			for aggregate.Game.Phase != bridge.PhaseBoardScored {
				if test.claim && len(aggregate.Game.CompletedTricks) == 1 {
					process(commandSessionForSeat(t, aggregate, bridge.North), table.Command{Name: table.CommandRequestClaim, ClaimTricks: 5})
					process(commandSessionForSeat(t, aggregate, bridge.East), table.Command{Name: table.CommandRespondClaim, Accepted: false})
					process(commandSessionForSeat(t, aggregate, bridge.North), table.Command{Name: table.CommandRequestClaim, ClaimTricks: 6})
					for _, seat := range []bridge.Seat{bridge.East, bridge.West} {
						process(commandSessionForSeat(t, aggregate, seat), table.Command{Name: table.CommandRespondClaim, Accepted: true})
					}
					break
				}
				seat := aggregate.Game.Turn
				if seat == aggregate.Game.Auction.Contract.Dummy() {
					seat = aggregate.Game.Auction.Contract.Declarer
				}
				cards, domainError := aggregate.Game.LegalCards(seat)
				if domainError != nil || len(cards) == 0 {
					t.Fatal("no legal play")
				}
				process(commandSessionForSeat(t, aggregate, seat), table.Command{Name: table.CommandPlayCard, Card: &cards[0]})
				if len(aggregate.Game.CompletedTricks) == 1 && len(aggregate.Game.CurrentTrick.Plays) == 0 {
					recovered, err := restarted.FindTable(environment.ctx, aggregate.ID)
					if err != nil || !reflect.DeepEqual(recovered, aggregate) {
						t.Fatalf("mid-play recovery differs: %v", err)
					}
				}
			}
			before := aggregate
			snapshotReplay, err := environment.postgres.CompletedBoardReplay(environment.ctx, boardID, aggregate.OwnerSessionID)
			if err != nil || !reflect.DeepEqual(snapshotReplay.Game, *aggregate.Game) {
				t.Fatalf("scored snapshot replay: %v", err)
			}
			if err := snapshotReplay.FullDeal.Validate(); err != nil {
				t.Fatal(err)
			}
			if _, err := environment.postgres.CompletedBoardReplay(environment.ctx, boardID, uuid.NewString()); !errors.Is(err, analysis.ErrNotFound) {
				t.Fatalf("outsider replay: %v", err)
			}
			if _, err := environment.postgres.queries.LoadBoardRecord(environment.ctx, boardID); !errors.Is(err, pgx.ErrNoRows) {
				t.Fatalf("premature compaction: %v", err)
			}
			retained, err := environment.postgres.ListEventsAfter(environment.ctx, aggregate.ID, firstSeq-1, maxRecoveryEvents)
			if err != nil || len(retained) != int(aggregate.LastSeq-firstSeq+1) {
				t.Fatalf("scored events removed before finality: %v", err)
			}
			command := table.Command{Name: table.CommandRequestNextBoard}
			if test.finish {
				command.Name = table.CommandFinishTable
			}
			request := table.CommandRequest{TableID: aggregate.ID, SessionID: aggregate.OwnerSessionID, RequestID: uuid.NewString(), ExpectedRevision: aggregate.Revision, Command: command}
			final := environment.process(t, request)
			if final.Outcome.Status != table.CommandStatusAccepted {
				t.Fatalf("finalize: %+v", final.Outcome)
			}
			record, err := environment.postgres.CompletedBoardRecord(environment.ctx, boardID, aggregate.OwnerSessionID)
			if err != nil {
				t.Fatal(err)
			}
			archiveReplay, err := environment.postgres.CompletedBoardReplay(environment.ctx, boardID, aggregate.OwnerSessionID)
			if err != nil || !reflect.DeepEqual(archiveReplay, snapshotReplay) {
				t.Fatalf("archived replay changed: %v", err)
			}
			replayed, err := record.Replay()
			if err != nil || !reflect.DeepEqual(replayed, *before.Game) {
				t.Fatalf("archive replay differs from snapshot: %v", err)
			}
			if _, err := environment.postgres.CompletedBoardRecord(environment.ctx, boardID, uuid.NewString()); !errors.Is(err, pgx.ErrNoRows) {
				t.Fatal("private record exposed to outsider")
			}
			remaining, err := restarted.ListEventsAfter(environment.ctx, aggregate.ID, firstSeq-1, maxRecoveryEvents)
			if err != nil || len(remaining) != len(final.Events) || remaining[0].Seq != before.LastSeq+1 {
				t.Fatalf("incorrect cleanup range: %v", err)
			}
			recovered, err = restarted.FindTable(environment.ctx, aggregate.ID)
			if err != nil || !reflect.DeepEqual(recovered, final.Aggregate) {
				t.Fatalf("post-compaction restart differs: %v", err)
			}
			duplicate := environment.process(t, request)
			if !duplicate.Duplicate || len(duplicate.Events) != 0 || duplicate.Outcome != final.Outcome {
				t.Fatal("finalization retry not idempotent")
			}
			stored, err := environment.postgres.queries.LoadBoardRecord(environment.ctx, boardID)
			if err != nil {
				t.Fatal(err)
			}
			if err := pgx.BeginFunc(environment.ctx, environment.postgres.pool, func(tx pgx.Tx) error {
				return compactFinalBoard(environment.ctx, environment.postgres.queries.WithTx(tx), before, final.Aggregate, time.Now())
			}); err != nil {
				t.Fatalf("compaction retry: %v", err)
			}
			retried, err := environment.postgres.queries.LoadBoardRecord(environment.ctx, boardID)
			if err != nil || !reflect.DeepEqual(stored, retried) {
				t.Fatalf("retry changed permanent row: %v", err)
			}
			if !test.finish {
				aggregate = final.Aggregate
				process(commandSessionForSeat(t, aggregate, aggregate.Game.Turn), table.Command{Name: table.CommandMakeCall, Call: &pass})
				activeGap, err := restarted.ListEventsAfter(environment.ctx, aggregate.ID, before.LastSeq, maxRecoveryEvents)
				if err != nil || len(activeGap) != int(aggregate.LastSeq-before.LastSeq) {
					t.Fatalf("next board gap broken: %v", err)
				}
			}
		})
	}
}

func TestBoardRecordCompactionRollback(t *testing.T) {
	for _, stage := range []string{"validation", "insert", "readback", "outcome"} {
		t.Run(stage, func(t *testing.T) {
			environment := newCommandTestEnvironment(t, 4)
			aggregate := readyCommandTable(t, environment)
			process := func(command table.Command, sessionID string) {
				result := environment.process(t, table.CommandRequest{TableID: aggregate.ID, SessionID: sessionID, RequestID: uuid.NewString(), ExpectedRevision: aggregate.Revision, Command: command})
				if result.Outcome.Status != table.CommandStatusAccepted {
					t.Fatalf("%s: %+v", command.Name, result.Outcome)
				}
				aggregate = result.Aggregate
			}
			process(table.Command{Name: table.CommandStartGame}, aggregate.OwnerSessionID)
			pass := bridge.Pass()
			for aggregate.Game.Phase == bridge.PhaseAuction {
				process(table.Command{Name: table.CommandMakeCall, Call: &pass}, commandSessionForSeat(t, aggregate, aggregate.Game.Turn))
			}
			beforeEvents, err := environment.postgres.ListEventsAfter(environment.ctx, aggregate.ID, 0, maxRecoveryEvents)
			if err != nil {
				t.Fatal(err)
			}
			request := table.CommandRequest{TableID: aggregate.ID, SessionID: aggregate.OwnerSessionID, RequestID: uuid.NewString(), ExpectedRevision: aggregate.Revision, Command: table.Command{Name: table.CommandFinishTable}}
			var restore func()
			if stage == "validation" {
				source, err := environment.postgres.queries.LoadBoardSource(environment.ctx, aggregate.BoardID)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := environment.postgres.pool.Exec(environment.ctx, "UPDATE bridgeyok.board_deals SET source_record = jsonb_set(source_record, '{deal}', '{}') WHERE board_id = $1", aggregate.BoardID); err != nil {
					t.Fatal(err)
				}
				restore = func() {
					if _, err := environment.postgres.pool.Exec(environment.ctx, "UPDATE bridgeyok.board_deals SET source_record = $2 WHERE board_id = $1", aggregate.BoardID, source); err != nil {
						t.Fatal(err)
					}
				}
			} else if stage == "readback" {
				triggerName := "compaction_corruption_" + fmt.Sprint(time.Now().UnixNano())
				statement := fmt.Sprintf("CREATE FUNCTION bridgeyok.%s() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.board_id = '%s' THEN NEW.record = jsonb_set(NEW.record, '{finalHash}', '\"corrupted\"'); END IF; RETURN NEW; END $$", triggerName, aggregate.BoardID)
				if _, err := environment.postgres.pool.Exec(environment.ctx, statement); err != nil {
					t.Fatal(err)
				}
				if _, err := environment.postgres.pool.Exec(environment.ctx, fmt.Sprintf("CREATE TRIGGER %s BEFORE INSERT ON bridgeyok.board_records FOR EACH ROW EXECUTE FUNCTION bridgeyok.%s()", triggerName, triggerName)); err != nil {
					t.Fatal(err)
				}
				restore = func() {
					if _, err := environment.postgres.pool.Exec(environment.ctx, fmt.Sprintf("DROP TRIGGER %s ON bridgeyok.board_records", triggerName)); err != nil {
						t.Fatal(err)
					}
					if _, err := environment.postgres.pool.Exec(environment.ctx, fmt.Sprintf("DROP FUNCTION bridgeyok.%s()", triggerName)); err != nil {
						t.Fatal(err)
					}
				}
			} else {
				relation, field, value := "board_records", "board_id", aggregate.BoardID
				if stage == "outcome" {
					relation, field, value = "processed_commands", "request_id", request.RequestID
				}
				constraint := "compaction_failure_" + fmt.Sprint(time.Now().UnixNano())
				if _, err := environment.postgres.pool.Exec(environment.ctx, fmt.Sprintf("ALTER TABLE bridgeyok.%s ADD CONSTRAINT %s CHECK (%s <> '%s') NOT VALID", relation, constraint, field, value)); err != nil {
					t.Fatal(err)
				}
				restore = func() {
					if _, err := environment.postgres.pool.Exec(environment.ctx, fmt.Sprintf("ALTER TABLE bridgeyok.%s DROP CONSTRAINT %s", relation, constraint)); err != nil {
						t.Fatal(err)
					}
				}
			}
			if _, err := environment.processor.Process(environment.ctx, request); err == nil {
				t.Fatal("injected failure accepted")
			}
			restore()
			after, err := environment.postgres.FindTable(environment.ctx, aggregate.ID)
			if err != nil || !reflect.DeepEqual(after, aggregate) {
				t.Fatalf("snapshot changed on failure: %v", err)
			}
			afterEvents, err := environment.postgres.ListEventsAfter(environment.ctx, aggregate.ID, 0, maxRecoveryEvents)
			if err != nil || !reflect.DeepEqual(afterEvents, beforeEvents) {
				t.Fatalf("cleanup escaped rollback: %v", err)
			}
			if _, err := environment.postgres.queries.LoadBoardRecord(environment.ctx, aggregate.BoardID); !errors.Is(err, pgx.ErrNoRows) {
				t.Fatalf("archive escaped rollback: %v", err)
			}
			if _, err := environment.postgres.queries.FindProcessedCommand(environment.ctx, dbgen.FindProcessedCommandParams{TableID: aggregate.ID, SessionID: aggregate.OwnerSessionID, RequestID: request.RequestID}); !errors.Is(err, pgx.ErrNoRows) {
				t.Fatalf("outcome escaped rollback: %v", err)
			}
			final := environment.process(t, request)
			if final.Outcome.Status != table.CommandStatusAccepted {
				t.Fatal("retry after failure rejected")
			}
			record, err := environment.postgres.CompletedBoardRecord(environment.ctx, aggregate.BoardID, aggregate.OwnerSessionID)
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(record)
			if err != nil || len(encoded) == 0 {
				t.Fatal("retry archive missing")
			}
		})
	}
}

func TestBoardRecordRetainsAbandonedUnfinishedBoard(t *testing.T) {
	environment := newCommandTestEnvironment(t, 4)
	aggregate := readyCommandTable(t, environment)
	started := environment.process(t, table.CommandRequest{TableID: aggregate.ID, SessionID: aggregate.OwnerSessionID, RequestID: uuid.NewString(), ExpectedRevision: aggregate.Revision, Command: table.Command{Name: table.CommandStartGame}})
	aggregate = started.Aggregate
	beforeEvents, err := environment.postgres.ListEventsAfter(environment.ctx, aggregate.ID, 0, maxRecoveryEvents)
	if err != nil {
		t.Fatal(err)
	}
	expired := environment.process(t, table.CommandRequest{TableID: aggregate.ID, SessionID: aggregate.OwnerSessionID, RequestID: uuid.NewString(), ExpectedRevision: aggregate.Revision, Command: table.Command{Name: table.CommandExpireTable, OccurredAt: time.Now()}})
	if expired.Outcome.Status != table.CommandStatusAccepted || expired.Aggregate.State != table.StateFinished {
		t.Fatal("expiry failed")
	}
	if _, err := environment.postgres.queries.LoadBoardRecord(environment.ctx, aggregate.BoardID); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("unfinished archive created: %v", err)
	}
	afterEvents, err := environment.postgres.ListEventsAfter(environment.ctx, aggregate.ID, 0, maxRecoveryEvents)
	if err != nil || len(afterEvents) != len(beforeEvents)+len(expired.Events) || !reflect.DeepEqual(afterEvents[:len(beforeEvents)], beforeEvents) {
		t.Fatalf("unfinished events lost: %v", err)
	}
	recovered, err := environment.postgres.FindTable(environment.ctx, aggregate.ID)
	if err != nil || !reflect.DeepEqual(recovered, expired.Aggregate) {
		t.Fatalf("expired snapshot lost: %v", err)
	}
}
