package match

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/mahesayuztar/bridgeyok/apps/api/internal/bridge"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/deal"
)

type testSource struct {
	calls   int
	failAt  int
	invalid bool
}

func (source *testSource) Generate(context.Context) (deal.Result, error) {
	source.calls++
	if source.calls == source.failAt {
		return deal.Result{}, deal.ErrUnavailable
	}
	deck := bridge.FullDeck()
	cards := bridge.Deal{North: deck[:13], East: deck[13:26], South: deck[26:39], West: deck[39:]}
	if source.invalid {
		cards.North = nil
	}
	return deal.Result{Deal: cards, Provenance: deal.Provenance{Type: "prepared", Version: "v1"}}, nil
}

func waitingMatch(t *testing.T) *Match {
	t.Helper()
	match, err := New("match", "p0", "open", "closed", []string{"b1", "b2"})
	if err != nil {
		t.Fatal(err)
	}
	assignments := make([]Assignment, 0, 8)
	for _roomIndex, room := range []Room{Open, Closed} {
		for _seatIndex, seat := range []bridge.Seat{bridge.North, bridge.East, bridge.South, bridge.West} {
			assignments = append(assignments, Assignment{ParticipantID: fmt.Sprintf("p%d", 4*_roomIndex+_seatIndex), Room: room, Seat: seat})
		}
	}
	if err := match.Assign("p0", assignments); err != nil {
		t.Fatal(err)
	}
	for _, assignment := range assignments {
		if err := match.SetReady(assignment.ParticipantID, true); err != nil {
			t.Fatal(err)
		}
	}
	return match
}

func activeMatch(t *testing.T) *Match {
	t.Helper()
	match := waitingMatch(t)
	if err := match.Start(t.Context(), "p0", &testSource{}); err != nil {
		t.Fatal(err)
	}
	return match
}

func TestNew(t *testing.T) {
	t.Parallel()
	cases := []struct {
		id, owner, open, closed string
		boards                  []string
	}{
		{"", "owner", "open", "closed", []string{"b"}},
		{"m", "", "open", "closed", []string{"b"}},
		{"m", "owner", "", "closed", []string{"b"}},
		{"m", "owner", "open", "", []string{"b"}},
		{"m", "owner", "open", "open", []string{"b"}},
		{"m", "owner", "open", "closed", nil},
		{"m", "owner", "open", "closed", []string{""}},
		{"m", "owner", "open", "closed", []string{"b", "b"}},
		{"m", "owner", "open", "closed", make([]string, MaxBoards+1)},
	}
	for _, test := range cases {
		if _, err := New(test.id, test.owner, test.open, test.closed, test.boards); !errors.Is(err, ErrInvalid) {
			t.Fatalf("%+v: %v", test, err)
		}
	}
}

func TestAssign(t *testing.T) {
	t.Parallel()
	match := waitingMatch(t)
	original := match.PrivateSnapshot()
	if err := match.Assign("p1", original.Assignments); !errors.Is(err, ErrForbidden) {
		t.Fatal(err)
	}
	changes := []func([]Assignment){
		func(a []Assignment) { a[1].ParticipantID = a[0].ParticipantID },
		func(a []Assignment) { a[1].Seat = a[0].Seat },
		func(a []Assignment) { a[1].Room = "other" },
		func(a []Assignment) { a[1].Seat = "other" },
		func(a []Assignment) { a[0].ParticipantID = "other" },
		func(a []Assignment) { a[1].ParticipantID = "" },
	}
	for _, change := range changes {
		assignments := match.PrivateSnapshot().Assignments
		change(assignments)
		if err := match.Assign("p0", assignments); !errors.Is(err, ErrInvalid) {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(original, match.PrivateSnapshot()) {
			t.Fatal("invalid assignment mutated state")
		}
	}
	if err := match.Assign("p0", nil); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if err := match.Assign("p0", original.Assignments); err != nil {
		t.Fatal(err)
	}
	if len(match.state.Ready) != 0 {
		t.Fatal("lineup change retained readiness")
	}
	match = activeMatch(t)
	if err := match.Assign("p0", original.Assignments); !errors.Is(err, ErrState) {
		t.Fatal(err)
	}
}

func TestSetReady(t *testing.T) {
	t.Parallel()
	match := waitingMatch(t)
	if err := match.SetReady("outsider", true); !errors.Is(err, ErrForbidden) {
		t.Fatal(err)
	}
	if err := match.SetReady("p0", true); err != nil || len(match.state.Ready) != 8 {
		t.Fatal("ready not idempotent", err)
	}
	if err := match.SetReady("p0", false); err != nil || len(match.state.Ready) != 7 {
		t.Fatal("unready", err)
	}
	if err := match.SetReady("p0", false); err != nil || len(match.state.Ready) != 7 {
		t.Fatal("unready not idempotent", err)
	}
	match = activeMatch(t)
	if err := match.SetReady("p0", false); !errors.Is(err, ErrState) {
		t.Fatal(err)
	}
}

func TestStart(t *testing.T) {
	t.Parallel()
	match := waitingMatch(t)
	initial := match.PrivateSnapshot()
	source := &testSource{failAt: 2}
	if err := match.Start(t.Context(), "p1", source); !errors.Is(err, ErrForbidden) || source.calls != 0 {
		t.Fatal("unauthorized generation", err)
	}
	if err := match.Start(t.Context(), "p0", nil); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if err := match.Start(t.Context(), "p0", source); !errors.Is(err, deal.ErrUnavailable) {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(initial, match.PrivateSnapshot()) {
		t.Fatal("partial start")
	}
	if err := match.Start(t.Context(), "p0", &testSource{invalid: true}); err == nil {
		t.Fatal("accepted invalid deal")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := match.Start(ctx, "p0", source); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(initial, match.PrivateSnapshot()) {
		t.Fatal("failed generation mutated match")
	}
	source = &testSource{}
	if err := match.Start(t.Context(), "p0", source); err != nil || source.calls != 2 {
		t.Fatal(err)
	}
	if err := match.Start(t.Context(), "p0", source); !errors.Is(err, ErrState) || source.calls != 2 {
		t.Fatal("restarted match", err)
	}
	match = waitingMatch(t)
	if err := match.SetReady("p7", false); err != nil {
		t.Fatal(err)
	}
	if err := match.Start(t.Context(), "p0", source); !errors.Is(err, ErrState) {
		t.Fatal("started without eight ready", err)
	}
}

func TestNextBoard(t *testing.T) {
	t.Parallel()
	match := activeMatch(t)
	open, ok := match.NextBoard(Open)
	if !ok {
		t.Fatal("missing open board")
	}
	closed, ok := match.NextBoard(Closed)
	if !ok || !reflect.DeepEqual(open, closed) {
		t.Fatal("different room boards")
	}
	open.Source.Deal.North[0] = bridge.Card{}
	if !reflect.DeepEqual(closed, match.state.Boards[0]) {
		t.Fatal("board aliases private deal")
	}
	if _, ok := match.NextBoard("invalid"); ok {
		t.Fatal("invalid room")
	}
	if _, ok := waitingMatch(t).NextBoard(Open); ok {
		t.Fatal("waiting exposes board")
	}
}

func TestRecordResult(t *testing.T) {
	t.Parallel()
	match := activeMatch(t)
	invalid := []struct {
		table  string
		result Result
	}{
		{"closed", Result{"b1", Open, 620}},
		{"open", Result{"b1", Closed, 620}},
		{"open", Result{"b1", "invalid", 620}},
		{"open", Result{"unknown", Open, 620}},
		{"open", Result{"b2", Open, 620}},
		{"open", Result{"b1", Open, 15}},
	}
	initial := match.PrivateSnapshot()
	for _, test := range invalid {
		if err := match.RecordResult(test.table, test.result); err == nil {
			t.Fatalf("accepted %+v", test)
		}
		if !reflect.DeepEqual(initial, match.PrivateSnapshot()) {
			t.Fatal("rejection mutated match")
		}
	}
	results := []Result{{"b1", Open, 620}, {"b2", Open, 0}, {"b1", Closed, 170}, {"b2", Closed, 0}}
	for _index, result := range results {
		table := "open"
		if result.Room == Closed {
			table = "closed"
		}
		if err := match.RecordResult(table, result); err != nil {
			t.Fatal(err)
		}
		state := match.PrivateSnapshot()
		if err := match.RecordResult(table, result); err != nil || !reflect.DeepEqual(state, match.PrivateSnapshot()) {
			t.Fatal("duplicate changed state", err)
		}
		conflicting := result
		conflicting.ScoreNS += 10
		if err := match.RecordResult(table, conflicting); !errors.Is(err, ErrResult) {
			t.Fatal("accepted changed result", err)
		}
		if _index == 1 {
			if _, ok := match.NextBoard(Open); ok {
				t.Fatal("open should have finished independently")
			}
			if board, ok := match.NextBoard(Closed); !ok || board.ID != "b1" {
				t.Fatal("closed lost its board")
			}
			if comparisons, total := match.Comparisons(); len(comparisons) != 0 || total != 0 {
				t.Fatal("compared unpaired result")
			}
		}
	}
	comparisons, total := match.Comparisons()
	if match.state.Status != Complete || len(comparisons) != 2 || total != 10 || comparisons[1].TeamAIMP != 0 {
		t.Fatalf("incorrect completion: %+v %d", comparisons, total)
	}
	if _, ok := match.NextBoard(Closed); ok {
		t.Fatal("complete exposes next board")
	}
}

func TestPrivateSnapshot(t *testing.T) {
	t.Parallel()
	match := activeMatch(t)
	before := match.PrivateSnapshot()
	copy := match.PrivateSnapshot()
	copy.BoardIDs[0] = "changed"
	copy.Assignments[0].ParticipantID = "changed"
	copy.Ready[0] = "changed"
	copy.Boards[0].Source.Deal.North[0] = bridge.Card{}
	if !reflect.DeepEqual(before, match.PrivateSnapshot()) {
		t.Fatal("snapshot aliases state")
	}
}

func TestRestore(t *testing.T) {
	t.Parallel()
	match := waitingMatch(t)
	for _step := 0; _step < 6; _step++ {
		state := match.PrivateSnapshot()
		encoded, err := json.Marshal(state)
		if err != nil {
			t.Fatal(err)
		}
		var decoded Snapshot
		if err := json.Unmarshal(encoded, &decoded); err != nil {
			t.Fatal(err)
		}
		recovered, err := Restore(decoded)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(state, recovered.PrivateSnapshot()) {
			t.Fatalf("roundtrip changed state at %d", _step)
		}
		if len(decoded.Boards) > 0 {
			decoded.Boards[0].Source.Deal.North[0] = bridge.Card{}
		}
		if !reflect.DeepEqual(state, recovered.PrivateSnapshot()) {
			t.Fatal("hydration aliases caller")
		}
		match = recovered
		if _step == 0 {
			if err := match.Start(t.Context(), "p0", &testSource{}); err != nil {
				t.Fatal(err)
			}
		}
		if _step >= 1 && _step <= 4 {
			results := []Result{{"b1", Open, 620}, {"b1", Closed, 170}, {"b2", Closed, 420}, {"b2", Open, 170}}
			result := results[_step-1]
			table := "open"
			if result.Room == Closed {
				table = "closed"
			}
			if err := match.RecordResult(table, result); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, total := match.Comparisons(); total != 4 {
		t.Fatalf("aggregate %d", total)
	}
	empty, err := New("m", "p0", "o", "c", []string{"b"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Restore(empty.PrivateSnapshot()); err != nil {
		t.Fatal(err)
	}
	corruptions := []func(*Snapshot){
		func(s *Snapshot) { s.ID = "" },
		func(s *Snapshot) { s.Assignments[0].Seat = "invalid" },
		func(s *Snapshot) { s.Ready = append(s.Ready, "p0") },
		func(s *Snapshot) { s.Ready[0] = "outsider" },
		func(s *Snapshot) { s.Ready = s.Ready[:7] },
		func(s *Snapshot) { s.Status = Waiting },
		func(s *Snapshot) { s.Status = "invalid" },
		func(s *Snapshot) { s.Status = Complete },
		func(s *Snapshot) { s.Boards = nil },
		func(s *Snapshot) { s.Boards[0].ID = "different" },
		func(s *Snapshot) { s.Boards[0].Metadata.Dealer = bridge.East },
		func(s *Snapshot) { s.Boards[0].Source.Deal.North = nil },
		func(s *Snapshot) { s.Results = []Result{{"b2", Open, 0}} },
		func(s *Snapshot) { s.Results = []Result{{"b1", Open, 0}, {"b1", Open, 0}} },
	}
	for _index, corrupt := range corruptions {
		snapshot := activeMatch(t).PrivateSnapshot()
		corrupt(&snapshot)
		if _, err := Restore(snapshot); err == nil {
			t.Fatalf("accepted corruption %d", _index)
		}
	}
}

func TestProject(t *testing.T) {
	t.Parallel()
	match := activeMatch(t)
	if _, err := match.Project("outsider"); !errors.Is(err, ErrForbidden) {
		t.Fatal(err)
	}
	for _index, assignment := range match.state.Assignments {
		view, err := match.Project(assignment.ParticipantID)
		if err != nil {
			t.Fatal(err)
		}
		expectedTeam := []string{"A", "B", "A", "B", "B", "A", "B", "A"}[_index]
		expectedTable := "open"
		if assignment.Room == Closed {
			expectedTable = "closed"
		}
		if view.Team != expectedTeam || view.TableID != expectedTable || view.Assignment != assignment || view.BoardCount != 2 || view.ReadyCount != 8 {
			t.Fatalf("incorrect projection %+v", view)
		}
		raw, err := json.Marshal(view)
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			t.Fatal(err)
		}
		allowed := map[string]bool{"id": true, "status": true, "ownerId": true, "tableId": true, "assignment": true, "team": true, "boardCount": true, "readyCount": true, "openCompleted": true, "closedCompleted": true, "teamAIMP": true}
		for key := range fields {
			if !allowed[key] {
				t.Fatalf("unexpected active projection field %s", key)
			}
		}
	}
	for _index, result := range []Result{{"b1", Open, 620}, {"b1", Closed, 170}, {"b2", Open, 0}, {"b2", Closed, 0}} {
		table := "open"
		if result.Room == Closed {
			table = "closed"
		}
		if err := match.RecordResult(table, result); err != nil {
			t.Fatal(err)
		}
		view, err := match.Project("p0")
		if err != nil {
			t.Fatal(err)
		}
		if view.OpenCompleted+view.ClosedCompleted != _index+1 {
			t.Fatal("incorrect progress")
		}
		if _index < 3 && (len(view.Comparisons) != 0 || view.TeamAIMP != 0) {
			t.Fatal("early result disclosure")
		}
		if _index == 3 && (len(view.Comparisons) != 2 || view.TeamAIMP != 10) {
			t.Fatal("missing final results")
		}
	}
}

func TestCancel(t *testing.T) {
	t.Parallel()
	waiting := waitingMatch(t)
	if err := waiting.Cancel("outsider"); !errors.Is(err, ErrForbidden) {
		t.Fatal(err)
	}
	if err := waiting.Cancel("p7"); err != nil {
		t.Fatal(err)
	}
	recovered, err := Restore(waiting.PrivateSnapshot())
	if err != nil || recovered.PrivateSnapshot().Status != Cancelled {
		t.Fatal("cancel recovery", err)
	}
	if err := recovered.Cancel("p7"); err != nil {
		t.Fatal("duplicate cancel", err)
	}
	if err := recovered.Start(t.Context(), "p0", &testSource{}); !errors.Is(err, ErrState) {
		t.Fatal("cancelled start", err)
	}
	if err := activeMatch(t).Cancel("p7"); !errors.Is(err, ErrState) {
		t.Fatal("active cancellation", err)
	}
}
