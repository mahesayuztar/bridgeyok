//go:build integration

package database

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/analysis"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/bridge"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/database/dbgen"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/table"
)

func TestMatchCommandsPreserveLineupAndPrivacy(t *testing.T) {
	environment := newMatchTestEnvironment(t, 2)
	postgres := environment.open.postgres
	aggregate, err := postgres.FindTable(t.Context(), environment.open.tableID)
	if err != nil {
		t.Fatal(err)
	}
	blocked := []table.CommandName{table.CommandStartGame, table.CommandLeaveTable, table.CommandTakeSeat, table.CommandLeaveSeat, table.CommandRemoveParticipant, table.CommandReplaceWithBot, table.CommandAddBot, table.CommandRemoveBot, table.CommandExpireParticipant, table.CommandExpireTable, table.CommandFinishTable, table.CommandLockTable}
	for _, name := range blocked {
		result := environment.open.process(t, table.CommandRequest{TableID: aggregate.ID, SessionID: aggregate.OwnerSessionID, RequestID: uuid.NewString(), ExpectedRevision: aggregate.Revision, Command: table.Command{Name: name, Seat: bridge.North}})
		if result.Outcome.Status != table.CommandStatusRejected || result.Aggregate.Revision != aggregate.Revision {
			t.Fatalf("casual mutation accepted %s", name)
		}
	}
	if err := postgres.LeaveTable(t.Context(), aggregate.ID, aggregate.OwnerSessionID, time.Now()); err == nil {
		t.Fatal("REST leave bypassed fixed lineup")
	}
	inactive, err := postgres.ListInactiveTables(t.Context(), time.Now().Add(time.Hour), 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range inactive {
		if row.ID == environment.open.tableID || row.ID == environment.closed.tableID {
			t.Fatal("casual expiry selected match room")
		}
	}
	var inviteHash []byte
	if err := postgres.pool.QueryRow(t.Context(), "SELECT invite_code_hash FROM bridgeyok.tables WHERE id=$1", aggregate.ID).Scan(&inviteHash); err != nil {
		t.Fatal(err)
	}
	if _, err := postgres.JoinTable(t.Context(), inviteHash, table.Participant{ID: uuid.NewString(), SessionID: environment.closed.sessions[0].ID, Nickname: "Wrong room", Role: table.RoleParticipant, JoinedAt: time.Now()}); !errors.Is(err, table.ErrTableUnavailable) {
		t.Fatal("cross-room join", err)
	}
	environment.start(t)
	aggregate, err = postgres.FindTable(t.Context(), aggregate.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := postgres.AnalysisPosition(t.Context(), aggregate.BoardID, aggregate.OwnerSessionID, fmt.Sprint(aggregate.Revision), nil); !errors.Is(err, analysis.ErrNotFound) {
		t.Fatal("match live DDS available", err)
	}
	scored := completeMatchTestBoard(t, environment.open, true)
	lastCaller := commandSessionForSeat(t, scored, bridge.West)
	requestUndo := environment.open.process(t, table.CommandRequest{TableID: scored.ID, SessionID: lastCaller, RequestID: uuid.NewString(), ExpectedRevision: scored.Revision, Command: table.Command{Name: table.CommandRequestUndo}})
	if requestUndo.Outcome.Status != table.CommandStatusAccepted {
		t.Fatal("undo unavailable before seal", requestUndo.Outcome)
	}
	aggregate = requestUndo.Aggregate
	for _, session := range environment.open.sessions {
		if session.ID == lastCaller {
			continue
		}
		response := environment.open.process(t, table.CommandRequest{TableID: aggregate.ID, SessionID: session.ID, RequestID: uuid.NewString(), ExpectedRevision: aggregate.Revision, Command: table.Command{Name: table.CommandRespondUndo, Accepted: true}})
		if response.Outcome.Status != table.CommandStatusAccepted {
			t.Fatal(response.Outcome)
		}
		aggregate = response.Aggregate
	}
	if aggregate.Game.Phase != bridge.PhaseAuction {
		t.Fatal("undo did not restore auction")
	}
	stored, err := postgres.LoadMatch(t.Context(), environment.initial.ID)
	if err != nil || len(stored.Match.PrivateSnapshot().Results) != 0 {
		t.Fatal("undo contaminated results", err)
	}
	pass := bridge.Pass()
	result := environment.open.process(t, table.CommandRequest{TableID: aggregate.ID, SessionID: lastCaller, RequestID: uuid.NewString(), ExpectedRevision: aggregate.Revision, Command: table.Command{Name: table.CommandMakeCall, Call: &pass}})
	if result.Outcome.Status != table.CommandStatusAccepted {
		t.Fatal(result.Outcome)
	}
	scored = result.Aggregate
	earlyFinish := environment.open.process(t, table.CommandRequest{TableID: scored.ID, SessionID: scored.OwnerSessionID, RequestID: uuid.NewString(), ExpectedRevision: scored.Revision, Command: table.Command{Name: table.CommandFinishTable}})
	if earlyFinish.Outcome.Status != table.CommandStatusRejected {
		t.Fatal("truncated match board set")
	}
	if _, err := postgres.CompletedBoardReplay(t.Context(), scored.BoardID, scored.OwnerSessionID); !errors.Is(err, analysis.ErrNotFound) {
		t.Fatal("early replay", err)
	}
	if _, err := postgres.CompletedAnalysisBoard(t.Context(), scored.BoardID, scored.OwnerSessionID); !errors.Is(err, analysis.ErrNotFound) {
		t.Fatal("early analysis", err)
	}
	closeMatchTestBoard(t, environment.open, scored, false)
	if _, err := postgres.CompletedBoardRecord(t.Context(), scored.BoardID, scored.OwnerSessionID); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatal("early archive", err)
	}
	for _, room := range []commandTestEnvironment{environment.open, environment.closed} {
		current, err := postgres.FindTable(t.Context(), room.tableID)
		if err != nil {
			t.Fatal(err)
		}
		if current.BoardNumber == 1 {
			first := completeMatchTestBoard(t, room, true)
			closeMatchTestBoard(t, room, first, false)
		}
		last := completeMatchTestBoard(t, room, true)
		extra := room.process(t, table.CommandRequest{TableID: last.ID, SessionID: last.OwnerSessionID, RequestID: uuid.NewString(), ExpectedRevision: last.Revision, Command: table.Command{Name: table.CommandRequestNextBoard}})
		if extra.Outcome.Status != table.CommandStatusRejected {
			t.Fatal("generated outside shared set")
		}
		closeMatchTestBoard(t, room, last, true)
	}
	if _, err := postgres.CompletedBoardReplay(t.Context(), scored.BoardID, scored.OwnerSessionID); err != nil {
		t.Fatal("final authorized replay unavailable", err)
	}
	if _, err := postgres.CompletedAnalysisBoard(t.Context(), scored.BoardID, scored.OwnerSessionID); err != nil {
		t.Fatal("final authorized analysis unavailable", err)
	}
	if _, err := postgres.CompletedBoardReplay(t.Context(), scored.BoardID, environment.closed.sessions[0].ID); !errors.Is(err, analysis.ErrNotFound) {
		t.Fatal("cross-room replay exposed", err)
	}
	if _, err := postgres.pool.Exec(t.Context(), "UPDATE bridgeyok.match_results SET score_ns=score_ns+10 WHERE match_id=$1", environment.initial.ID); err == nil {
		t.Fatal("final result mutable")
	}
	if _, err := postgres.pool.Exec(t.Context(), "UPDATE bridgeyok.match_comparisons SET team_a_imp=1 WHERE match_id=$1", environment.initial.ID); err == nil {
		t.Fatal("final comparison mutable")
	}
}

func TestMatchSchemaPrivacyAndReadiness(t *testing.T) {
	environment := newMatchTestEnvironment(t, 1)
	postgres := environment.open.postgres
	var count int
	if err := postgres.pool.QueryRow(t.Context(), "SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='bridgeyok' AND c.relname=ANY($1) AND c.relrowsecurity", []string{"team_matches", "match_rooms", "match_boards", "match_assignments", "match_room_boards", "match_results", "match_comparisons"}).Scan(&count); err != nil || count != 7 {
		t.Fatal("match RLS missing", err)
	}
	tx, err := postgres.pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(t.Context(), "ALTER TABLE bridgeyok.match_results RENAME TO match_results_readiness_test"); err != nil {
		t.Fatal(err)
	}
	ready, err := dbgen.New(tx).IsSchemaReady(t.Context())
	if err != nil || ready {
		t.Fatal("missing match schema accepted", err)
	}
}
