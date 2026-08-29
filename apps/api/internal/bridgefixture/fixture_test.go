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
		{name: "missing events", data: []byte(`{"initialState":{}}`)},
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
	events := []bridge.Event{}
	for _, actor := range []bridge.Seat{bridge.North, bridge.East, bridge.South, bridge.West} {
		decision, domainError := bridge.Decide(state, bridge.MakeCallCommand(actor, bridge.Pass()))
		if domainError != nil {
			t.Fatalf("Decide(Pass) error = %v", domainError)
		}
		events = append(events, decision.Events...)
		state = decision.NextState
	}
	return Fixture{InitialState: initial, Events: events}
}
