//go:build integration

package database

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/bridge"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/identity"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/match"
)

func TestMatchLobbyCapacityRollbackAndDiscovery(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()
	postgres, err := Open(ctx, os.Getenv("TEST_DATABASE_URL"), 12)
	if err != nil {
		t.Fatal(err)
	}
	defer postgres.Close()
	identities, err := identity.NewService(postgres, []byte(strings.Repeat("capacity-test-", 4)), rand.Reader, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	accounts := make([]identity.AccountLogin, 8)
	sessions := make([]string, 0, 8)
	defer func() {
		cleanup, done := context.WithTimeout(context.Background(), 10*time.Second)
		defer done()
		for _, query := range []string{"DELETE FROM bridgeyok.team_matches WHERE owner_session_id=ANY($1::uuid[])", "DELETE FROM bridgeyok.tables WHERE owner_session_id=ANY($1::uuid[])", "DELETE FROM bridgeyok.users WHERE session_id=ANY($1::uuid[])", "DELETE FROM bridgeyok.guest_sessions WHERE id=ANY($1::uuid[])"} {
			if _, err := postgres.pool.Exec(cleanup, query, sessions); err != nil {
				t.Error(err)
			}
		}
	}()
	request := match.CreateRequest{BoardCount: 1}
	suffix := strings.ToLower(rand.Text()[:8])
	for _index := range accounts {
		accounts[_index], err = identities.Register(ctx, fmt.Sprintf("cap%d_%s", _index, suffix), "hardening-test-password", fmt.Sprintf("Capacity %d", _index), "spade")
		if err != nil {
			t.Fatal(err)
		}
		sessions = append(sessions, accounts[_index].SessionID)
		room := match.Open
		if _index >= 4 {
			room = match.Closed
		}
		request.Assignments = append(request.Assignments, match.SeatRequest{UserID: accounts[_index].Profile.ID, Room: room, Seat: []bridge.Seat{bridge.North, bridge.East, bridge.South, bridge.West}[_index%4]})
	}
	type outcome struct {
		stored match.Stored
		err    error
	}
	outcomes := make(chan outcome, 8)
	var group sync.WaitGroup
	for _index := 0; _index < 8; _index++ {
		group.Go(func() {
			candidate := request
			candidate.RequestID = uuid.NewString()
			stored, err := postgres.CreateMatchLobby(ctx, sessions[0], candidate, time.Now())
			outcomes <- outcome{stored, err}
		})
	}
	group.Wait()
	close(outcomes)
	matches := make([]match.Stored, 0, 4)
	rejected := 0
	for result := range outcomes {
		if errors.Is(result.err, match.ErrCapacity) {
			rejected++
			continue
		}
		if result.err != nil {
			t.Fatal(result.err)
		}
		matches = append(matches, result.stored)
	}
	if len(matches) != 4 || rejected != 4 {
		t.Fatalf("accepted=%d capacity_rejected=%d", len(matches), rejected)
	}
	for _, session := range sessions {
		ids, err := postgres.ListMatchIDs(ctx, session)
		if err != nil || len(ids) != 4 {
			t.Fatal("inconsistent capacity", len(ids), err)
		}
	}
	victim := matches[0]
	before := victim.Match.PrivateSnapshot()
	openBefore, err := postgres.FindTable(ctx, before.OpenTableID)
	if err != nil {
		t.Fatal(err)
	}
	closedBefore, err := postgres.FindTable(ctx, before.ClosedTableID)
	if err != nil {
		t.Fatal(err)
	}
	constraint := "hardening_" + suffix
	query := fmt.Sprintf("ALTER TABLE bridgeyok.team_matches ADD CONSTRAINT %s CHECK (id <> '%s' OR status <> 'CANCELLED') NOT VALID", constraint, before.ID)
	if _, err := postgres.pool.Exec(ctx, query); err != nil {
		t.Fatal(err)
	}
	_, cancelErr := postgres.CancelMatch(ctx, before.ID, sessions[7], victim.Revision, time.Now())
	if _, err := postgres.pool.Exec(ctx, "ALTER TABLE bridgeyok.team_matches DROP CONSTRAINT "+constraint); err != nil {
		t.Fatal(err)
	}
	if cancelErr == nil {
		t.Fatal("fault injection did not reject cancel")
	}
	openAfter, err := postgres.FindTable(ctx, before.OpenTableID)
	if err != nil {
		t.Fatal(err)
	}
	closedAfter, err := postgres.FindTable(ctx, before.ClosedTableID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(openBefore, openAfter) || !reflect.DeepEqual(closedBefore, closedAfter) {
		t.Fatal("failed cancel changed a room")
	}
	if _, err := postgres.CancelMatch(ctx, before.ID, sessions[7], victim.Revision, time.Now()); err != nil {
		t.Fatal("cancel retry", err)
	}
	var tableCount int
	if err := postgres.pool.QueryRow(ctx, "SELECT count(*) FROM bridgeyok.tables WHERE owner_session_id=ANY($1::uuid[])", sessions).Scan(&tableCount); err != nil {
		t.Fatal(err)
	}
	query = fmt.Sprintf("ALTER TABLE bridgeyok.match_create_requests ADD CONSTRAINT %s CHECK (owner_session_id <> '%s') NOT VALID", constraint, sessions[0])
	if _, err := postgres.pool.Exec(ctx, query); err != nil {
		t.Fatal(err)
	}
	request.RequestID = uuid.NewString()
	_, createErr := postgres.CreateMatchLobby(ctx, sessions[0], request, time.Now())
	if _, err := postgres.pool.Exec(ctx, "ALTER TABLE bridgeyok.match_create_requests DROP CONSTRAINT "+constraint); err != nil {
		t.Fatal(err)
	}
	if createErr == nil {
		t.Fatal("fault injection did not reject create")
	}
	var afterCount int
	if err := postgres.pool.QueryRow(ctx, "SELECT count(*) FROM bridgeyok.tables WHERE owner_session_id=ANY($1::uuid[])", sessions).Scan(&afterCount); err != nil {
		t.Fatal(err)
	}
	if afterCount != tableCount {
		t.Fatal("failed create left orphan rooms")
	}
	for _index := 0; _index < 22; _index++ {
		request.RequestID = uuid.NewString()
		created, err := postgres.CreateMatchLobby(ctx, sessions[0], request, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := postgres.CancelMatch(ctx, created.Match.PrivateSnapshot().ID, sessions[7], created.Revision, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	ids, err := postgres.ListMatchIDs(ctx, sessions[0])
	if err != nil || len(ids) != 20 {
		t.Fatal("bounded list", err)
	}
	for _, stored := range matches[1:] {
		found := false
		for _, id := range ids[:3] {
			if id == stored.Match.PrivateSnapshot().ID {
				found = true
			}
		}
		if !found {
			t.Fatal("unfinished match hidden behind history")
		}
	}
	t.Log("8 concurrent creates: 4 accepted, 4 capacity rejections; create/cancel rollback and 22-history discovery passed")
}
