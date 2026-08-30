//go:build testfixture

package bridgefixture

import (
	"reflect"
	"testing"

	"github.com/mahesayuztar/bridgeyok/apps/api/internal/bridge"
)

type fixedReader byte

func (reader fixedReader) Read(target []byte) (int, error) {
	for _index := range target {
		target[_index] = byte(reader)
	}
	return len(target), nil
}

func TestFixtureRoundTrip(t *testing.T) {
	t.Parallel()

	fixture := passedOutFixture(t)
	first, err := Marshal(fixture)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	second, err := Marshal(fixture)
	if err != nil {
		t.Fatalf("Marshal() second error = %v", err)
	}
	if string(first) != string(second) {
		t.Fatal("Marshal() output is not deterministic")
	}

	decoded, err := Unmarshal(first)
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if !reflect.DeepEqual(decoded, fixture) {
		t.Fatal("fixture changed during round trip")
	}
	finalState, err := decoded.Replay()
	if err != nil {
		t.Fatalf("Replay() error = %v", err)
	}
	if finalState.Phase != bridge.PhaseBoardScored || finalState.Result == nil || !finalState.Result.PassedOut {
		t.Fatalf("Replay() state = %+v", finalState)
	}
}

func TestFullBoardFixtureRoundTrip(t *testing.T) {
	t.Parallel()

	fixture := playedFixture(t)
	data, err := Marshal(fixture)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	decoded, err := Unmarshal(data)
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	finalState, err := decoded.Replay()
	if err != nil {
		t.Fatalf("Replay() error = %v", err)
	}
	if finalState.Phase != bridge.PhaseBoardScored || finalState.Result == nil || finalState.Result.PassedOut {
		t.Fatalf("Replay() state = %+v", finalState)
	}
	if finalState.Auction.Contract == nil || finalState.Auction.Contract.Doubling != bridge.Redoubled {
		t.Fatalf("Replay() contract = %+v", finalState.Auction.Contract)
	}
	if len(finalState.CompletedTricks) != 13 || finalState.TricksNS+finalState.TricksEW != 13 {
		t.Fatalf("Replay() tricks = %d + %d", finalState.TricksNS, finalState.TricksEW)
	}
}

func TestFixtureReplayRejectsTampering(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(Fixture) Fixture
	}{
		{name: "schema version", mutate: func(fixture Fixture) Fixture { fixture.SchemaVersion++; return fixture }},
		{name: "ruleset version", mutate: func(fixture Fixture) Fixture { fixture.RulesetVersion = "v2"; return fixture }},
		{name: "missing steps", mutate: func(fixture Fixture) Fixture { fixture.Steps = nil; return fixture }},
		{name: "missing expected result", mutate: func(fixture Fixture) Fixture { fixture.ExpectedResult = nil; return fixture }},
		{name: "missing step events", mutate: func(fixture Fixture) Fixture { fixture.Steps[0].Events = nil; return fixture }},
		{name: "invalid command", mutate: func(fixture Fixture) Fixture { fixture.Steps[0].Command.Name = "UNKNOWN"; return fixture }},
		{name: "mismatched events", mutate: func(fixture Fixture) Fixture { fixture.Steps[0].Events[0].Seat = bridge.East; return fixture }},
		{name: "incomplete scenario", mutate: func(fixture Fixture) Fixture { fixture.Steps = fixture.Steps[:1]; return fixture }},
		{name: "wrong expected result", mutate: func(fixture Fixture) Fixture { fixture.ExpectedResult.ScoreNS++; return fixture }},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			fixture := test.mutate(passedOutFixture(t))
			if _, err := fixture.Replay(); err == nil {
				t.Fatal("Replay() error = nil")
			}
		})
	}
}

func TestUnmarshalRejectsInvalidDocuments(t *testing.T) {
	t.Parallel()

	valid, err := Marshal(passedOutFixture(t))
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	tests := []struct {
		name string
		data []byte
	}{
		{name: "empty", data: nil},
		{name: "malformed", data: []byte(`{"initialState":`)},
		{name: "unknown field", data: []byte(`{"unknown":true}`)},
		{name: "multiple values", data: append(append([]byte{}, valid...), []byte(`{}`)...)},
		{name: "missing version", data: []byte(`{"initialState":{}}`)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, err := Unmarshal(test.data); err == nil {
				t.Fatal("Unmarshal() error = nil")
			}
		})
	}
}

func FuzzUnmarshal(f *testing.F) {
	fixture := passedOutFixture(f)
	valid, err := Marshal(fixture)
	if err != nil {
		f.Fatalf("Marshal() error = %v", err)
	}
	for _, seed := range [][]byte{nil, []byte(`{}`), []byte(`null`), valid} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		fixture, err := Unmarshal(data)
		if err != nil {
			return
		}
		first, err := Marshal(fixture)
		if err != nil {
			t.Fatalf("valid decoded fixture failed marshal: %v", err)
		}
		decoded, err := Unmarshal(first)
		if err != nil {
			t.Fatalf("canonical fixture failed decode: %v", err)
		}
		second, err := Marshal(decoded)
		if err != nil {
			t.Fatalf("decoded canonical fixture failed marshal: %v", err)
		}
		if string(first) != string(second) {
			t.Fatal("canonical fixture is not stable")
		}
	})
}

type testingT interface {
	Helper()
	Fatalf(string, ...any)
}

func passedOutFixture(t testingT) Fixture {
	t.Helper()
	deal, err := bridge.GenerateDeal(fixedReader(0x80))
	if err != nil {
		t.Fatalf("GenerateDeal() error = %v", err)
	}
	state, err := bridge.NewBoard(1, deal)
	if err != nil {
		t.Fatalf("NewBoard() error = %v", err)
	}
	initial := state
	steps := []Step{}
	for _, actor := range []bridge.Seat{bridge.North, bridge.East, bridge.South, bridge.West} {
		command := bridge.MakeCallCommand(actor, bridge.Pass())
		decision, domainError := bridge.Decide(state, command)
		if domainError != nil {
			t.Fatalf("Decide(Pass) error = %v", domainError)
		}
		steps = append(steps, Step{Command: command, Events: decision.Events})
		state = decision.NextState
	}
	return Fixture{
		SchemaVersion:  SchemaVersion,
		RulesetVersion: bridge.RulesetVersion,
		InitialState:   initial,
		Steps:          steps,
		ExpectedResult: state.Result,
	}
}

func playedFixture(t testingT) Fixture {
	t.Helper()
	deal, err := bridge.GenerateDeal(fixedReader(0x42))
	if err != nil {
		t.Fatalf("GenerateDeal() error = %v", err)
	}
	state, err := bridge.NewBoard(2, deal)
	if err != nil {
		t.Fatalf("NewBoard() error = %v", err)
	}
	initial := state
	steps := []Step{}
	commands := []bridge.Command{
		bridge.MakeCallCommand(bridge.East, bridge.Bid(1, bridge.StrainNoTrump)),
		bridge.MakeCallCommand(bridge.South, bridge.Double()),
		bridge.MakeCallCommand(bridge.West, bridge.Redouble()),
		bridge.MakeCallCommand(bridge.North, bridge.Pass()),
		bridge.MakeCallCommand(bridge.East, bridge.Pass()),
		bridge.MakeCallCommand(bridge.South, bridge.Pass()),
	}
	for _, command := range commands {
		decision, domainError := bridge.Decide(state, command)
		if domainError != nil {
			t.Fatalf("Decide(call) error = %v", domainError)
		}
		steps = append(steps, Step{Command: command, Events: decision.Events})
		state = decision.NextState
	}
	for _playIndex := 0; state.Phase != bridge.PhaseBoardScored; _playIndex++ {
		if _playIndex >= 52 {
			t.Fatalf("board did not finish in 52 plays")
		}
		actor := state.Turn
		if state.Auction.Contract != nil && state.Turn == state.Auction.Contract.Dummy() {
			actor = state.Auction.Contract.Declarer
		}
		legalCards, domainError := state.LegalCards(actor)
		if domainError != nil || len(legalCards) == 0 {
			t.Fatalf("LegalCards(%s) cards = %d, error = %v", actor, len(legalCards), domainError)
		}
		command := bridge.PlayCardCommand(actor, legalCards[0])
		decision, domainError := bridge.Decide(state, command)
		if domainError != nil {
			t.Fatalf("Decide(play %d) error = %v", _playIndex, domainError)
		}
		steps = append(steps, Step{Command: command, Events: decision.Events})
		state = decision.NextState
	}
	return Fixture{
		SchemaVersion:  SchemaVersion,
		RulesetVersion: bridge.RulesetVersion,
		InitialState:   initial,
		Steps:          steps,
		ExpectedResult: state.Result,
	}
}
