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

func TestStateValidateInvariantsRejectsUnevenHistoricalHands(t *testing.T) {
	t.Parallel()

	state := stateAfterOpeningLead(t)
	movedCard := state.Deal.East[0]
	state.Deal.East = append(Hand{}, state.Deal.East[1:]...)
	state.Deal.North = append(state.Deal.North, movedCard)
	if err := state.ValidateInvariants(); err == nil {
		t.Fatal("ValidateInvariants() error = nil")
	}
}

func TestStateValidateInvariantsRejectsCurrentAndHistoricalRevoke(t *testing.T) {
	t.Parallel()

	state, followedCard, offSuitIndex := stateWithDummyFollowingLead(t)
	tests := []struct {
		name     string
		complete bool
	}{
		{name: "current trick"},
		{name: "completed trick", complete: true},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			tampered := state.clone()
			if test.complete {
				tampered = playLegalCards(t, tampered, 2)
			}

			offSuitCard := tampered.Deal.South[offSuitIndex]
			tampered.Deal.South[offSuitIndex] = followedCard
			if test.complete {
				tampered.CompletedTricks[0].Plays[1].Card = offSuitCard
				winner, err := trickWinner(tampered.CompletedTricks[0], tampered.Auction.Contract.Strain)
				if err != nil {
					t.Fatalf("trickWinner() error = %v", err)
				}
				tampered.CompletedTricks[0].Winner = winner
				tampered.CurrentTrick.Leader = winner
				tampered.Turn = winner
				tampered.TricksNS = 0
				tampered.TricksEW = 0
				if winner.Partnership() == NorthSouth {
					tampered.TricksNS = 1
				} else {
					tampered.TricksEW = 1
				}
			} else {
				tampered.CurrentTrick.Plays[1].Card = offSuitCard
			}
			if err := tampered.ValidateInvariants(); err == nil {
				t.Fatal("ValidateInvariants() error = nil")
			}
		})
	}
}

func TestStateValidateInvariantsRejectsInvalidPhaseHistory(t *testing.T) {
	t.Parallel()

	oneTrick := playLegalCards(t, contractedTestBoard(t), 4)
	wrongOpeningLeader := oneTrick.clone()
	wrongOpeningLeader.CompletedTricks[0].Leader = South
	for _playIndex := range wrongOpeningLeader.CompletedTricks[0].Plays {
		wrongOpeningLeader.CompletedTricks[0].Plays[_playIndex].Seat = []Seat{South, West, North, East}[_playIndex]
	}
	winner, err := trickWinner(wrongOpeningLeader.CompletedTricks[0], wrongOpeningLeader.Auction.Contract.Strain)
	if err != nil {
		t.Fatalf("trickWinner() error = %v", err)
	}
	wrongOpeningLeader.CompletedTricks[0].Winner = winner
	wrongOpeningLeader.CurrentTrick.Leader = winner
	wrongOpeningLeader.Turn = winner
	wrongOpeningLeader.TricksNS = 0
	wrongOpeningLeader.TricksEW = 0
	if winner.Partnership() == NorthSouth {
		wrongOpeningLeader.TricksNS = 1
	} else {
		wrongOpeningLeader.TricksEW = 1
	}

	openingWithCompletedTrick := oneTrick.clone()
	openingWithCompletedTrick.Phase = PhaseOpeningLead
	openingWithCompletedTrick.DummyRevealed = false

	completed := playLegalCards(t, contractedTestBoard(t), 52)
	hiddenDummy := completed.clone()
	hiddenDummy.DummyRevealed = false

	tests := []struct {
		name  string
		state State
	}{
		{name: "wrong opening leader", state: wrongOpeningLeader},
		{name: "opening phase with completed trick", state: openingWithCompletedTrick},
		{name: "hidden dummy after completed play", state: hiddenDummy},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if err := test.state.ValidateInvariants(); err == nil {
				t.Fatal("ValidateInvariants() error = nil")
			}
		})
	}
}

func stateWithDummyFollowingLead(t *testing.T) (State, Card, int) {
	t.Helper()
	state := contractedTestBoard(t)
	for _, openingCard := range state.Deal.East {
		for _followIndex, followedCard := range state.Deal.South {
			if followedCard.Suit != openingCard.Suit {
				continue
			}
			for _offSuitIndex, offSuitCard := range state.Deal.South {
				if offSuitCard.Suit == openingCard.Suit {
					continue
				}
				leadDecision, domainError := Decide(state, PlayCardCommand(East, openingCard))
				if domainError != nil {
					t.Fatalf("Decide(opening lead) error = %v", domainError)
				}
				followDecision, domainError := Decide(leadDecision.NextState, PlayCardCommand(North, followedCard))
				if domainError != nil {
					t.Fatalf("Decide(dummy follow) error = %v", domainError)
				}
				adjustedOffSuitIndex := _offSuitIndex
				if _offSuitIndex > _followIndex {
					adjustedOffSuitIndex--
				}
				return followDecision.NextState, followedCard, adjustedOffSuitIndex
			}
		}
	}
	t.Fatal("test deal has no dummy follow-suit and discard scenario")
	return State{}, Card{}, 0
}

func playLegalCards(t *testing.T, state State, count int) State {
	t.Helper()
	for _playIndex := 0; _playIndex < count; _playIndex++ {
		actor := state.Turn
		if state.Auction.Contract != nil && state.Turn == state.Auction.Contract.Dummy() {
			actor = state.Auction.Contract.Declarer
		}
		legalCards, domainError := state.LegalCards(actor)
		if domainError != nil || len(legalCards) == 0 {
			t.Fatalf("LegalCards(%s) cards = %d, error = %v", actor, len(legalCards), domainError)
		}
		decision, domainError := Decide(state, PlayCardCommand(actor, legalCards[0]))
		if domainError != nil {
			t.Fatalf("Decide(play %d) error = %v", _playIndex, domainError)
		}
		state = decision.NextState
	}
	return state
}
