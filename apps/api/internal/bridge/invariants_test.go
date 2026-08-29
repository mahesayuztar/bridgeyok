package bridge

import "testing"

func TestStateValidateInvariantsRejectsTampering(t *testing.T) {
	t.Parallel()

	valid := stateAfterOpeningLead(t)
	tests := []struct {
		name   string
		mutate func(State) State
	}{
		{name: "ruleset", mutate: func(state State) State { state.RulesetVersion = "v2"; return state }},
		{name: "board cycle", mutate: func(state State) State { state.Board.Dealer = West; return state }},
		{name: "turn", mutate: func(state State) State { state.Turn = West; return state }},
		{name: "duplicate card", mutate: func(state State) State { state.Deal.North[0] = state.Deal.West[0]; return state }},
		{name: "trick total", mutate: func(state State) State { state.TricksNS = 1; return state }},
		{name: "current trick winner", mutate: func(state State) State { state.CurrentTrick.Winner = East; return state }},
		{name: "dummy hidden", mutate: func(state State) State { state.DummyRevealed = false; return state }},
		{name: "terminal result", mutate: func(state State) State {
			result, _ := PassedOutResult(state.Board.Vulnerability)
			state.Result = &result
			return state
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if err := test.mutate(valid.clone()).ValidateInvariants(); err == nil {
				t.Fatal("ValidateInvariants() error = nil")
			}
		})
	}
}

func TestReduceRejectsTamperedEvents(t *testing.T) {
	t.Parallel()

	state := contractedTestBoard(t)
	card := state.Deal.East[0]
	tests := []struct {
		name  string
		event Event
	}{
		{name: "unknown", event: Event{Type: "UNKNOWN"}},
		{name: "wrong seat", event: Event{Type: EventCardPlayed, Seat: West, Card: &card}},
		{name: "missing card", event: Event{Type: EventCardPlayed, Seat: East}},
		{name: "premature dummy", event: Event{Type: EventDummyRevealed}},
		{name: "premature score", event: Event{Type: EventBoardScored}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, err := Reduce(state, []Event{test.event}); err == nil {
				t.Fatal("Reduce() error = nil")
			}
		})
	}
}
