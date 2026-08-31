package bridge

import (
	"reflect"
	"testing"
)

func TestDecidePassedOutBoard(t *testing.T) {
	t.Parallel()

	initial := newTestBoard(t, 1)
	state := initial
	allEvents := []Event{}
	for _, actor := range []Seat{North, East, South, West} {
		decision, domainError := Decide(state, MakeCallCommand(actor, Pass()))
		if domainError != nil {
			t.Fatalf("Decide(Pass by %s) error = %v", actor, domainError)
		}
		allEvents = append(allEvents, decision.Events...)
		state = decision.NextState
	}
	if state.Phase != PhaseBoardScored || state.Result == nil || !state.Result.PassedOut || state.Result.ScoreNS != 0 {
		t.Fatalf("passed-out state = %+v", state)
	}
	replayed, err := Reduce(initial, allEvents)
	if err != nil {
		t.Fatalf("Reduce() error = %v", err)
	}
	if !reflect.DeepEqual(replayed, state) {
		t.Fatal("replayed state differs from decided state")
	}
}

func TestDecideContractAndOpeningLead(t *testing.T) {
	t.Parallel()

	state := contractedTestBoard(t)
	if state.Phase != PhaseOpeningLead || state.Auction.Contract == nil || state.Turn != East {
		t.Fatalf("contracted state = %+v", state)
	}

	openingCard := state.Deal.East[0]
	decision, domainError := Decide(state, PlayCardCommand(East, openingCard))
	if domainError != nil {
		t.Fatalf("Decide(opening lead) error = %v", domainError)
	}
	if len(decision.Events) != 2 || decision.Events[0].Type != EventCardPlayed || decision.Events[1].Type != EventDummyRevealed {
		t.Fatalf("opening events = %+v", decision.Events)
	}
	next := decision.NextState
	if next.Phase != PhasePlay || !next.DummyRevealed || next.Turn != South || len(next.Deal.East) != 12 || len(next.CurrentTrick.Plays) != 1 {
		t.Fatalf("state after opening lead = %+v", next)
	}
}

func TestDecideDeclarerControlsDummy(t *testing.T) {
	t.Parallel()

	state := stateAfterOpeningLead(t)
	dummyCard := state.Deal.South[0]
	before := state.clone()
	if _, domainError := Decide(state, PlayCardCommand(South, dummyCard)); domainError == nil || domainError.Code != ErrorDeclarerControlsDummy {
		t.Fatalf("dummy command error = %+v, want %s", domainError, ErrorDeclarerControlsDummy)
	}
	if !reflect.DeepEqual(state, before) {
		t.Fatal("rejected dummy command mutated input state")
	}
	if _, domainError := Decide(state, PlayCardCommand(North, dummyCard)); domainError != nil {
		t.Fatalf("declarer play from dummy error = %v", domainError)
	}
}

func TestDecideEnforcesFollowSuit(t *testing.T) {
	t.Parallel()

	state := contractedTestBoard(t)
	var openingCard Card
	var offSuit Card
	found := false
	for _, candidateLead := range state.Deal.East {
		var matching bool
		for _, candidateDummy := range state.Deal.South {
			if candidateDummy.Suit == candidateLead.Suit {
				matching = true
			} else {
				offSuit = candidateDummy
			}
		}
		if matching && offSuit.Suit != candidateLead.Suit {
			openingCard = candidateLead
			found = true
			break
		}
	}
	if !found {
		t.Fatal("test deal has no follow-suit scenario")
	}

	decision, domainError := Decide(state, PlayCardCommand(East, openingCard))
	if domainError != nil {
		t.Fatalf("opening lead error = %v", domainError)
	}
	state = decision.NextState
	if _, domainError := Decide(state, PlayCardCommand(North, offSuit)); domainError == nil || domainError.Code != ErrorMustFollowSuit {
		t.Fatalf("off-suit command error = %+v, want %s", domainError, ErrorMustFollowSuit)
	}
	legal, domainError := state.LegalCards(North)
	if domainError != nil {
		t.Fatalf("LegalCards() error = %v", domainError)
	}
	for _, card := range legal {
		if card.Suit != openingCard.Suit {
			t.Fatalf("LegalCards() returned off-suit card %s", card)
		}
	}
}

func TestDecideCompletesBoard(t *testing.T) {
	t.Parallel()

	state := contractedTestBoard(t)
	initial := state.clone()
	allEvents := []Event{}
	for _playIndex := 0; state.Phase != PhaseBoardScored; _playIndex++ {
		if _playIndex >= 52 {
			t.Fatal("board did not complete in 52 plays")
		}
		actor := state.Turn
		if state.Auction.Contract != nil && state.Turn == state.Auction.Contract.Dummy() {
			actor = state.Auction.Contract.Declarer
		}
		legalCards, domainError := state.LegalCards(actor)
		if domainError != nil {
			t.Fatalf("LegalCards(%s) error = %v", actor, domainError)
		}
		if len(legalCards) == 0 {
			t.Fatal("LegalCards() returned no card before board completion")
		}
		decision, domainError := Decide(state, PlayCardCommand(actor, legalCards[0]))
		if domainError != nil {
			t.Fatalf("Decide(play %d) error = %v", _playIndex, domainError)
		}
		allEvents = append(allEvents, decision.Events...)
		state = decision.NextState
		if err := state.ValidateInvariants(); err != nil {
			t.Fatalf("play %d invariants: %v", _playIndex, err)
		}
	}
	if len(state.CompletedTricks) != 13 || state.TricksNS+state.TricksEW != 13 || state.Result == nil {
		t.Fatalf("completed state = %+v", state)
	}
	for _, seat := range []Seat{North, East, South, West} {
		if got := len(state.Deal.Hand(seat)); got != 0 {
			t.Errorf("seat %s has %d cards after completion", seat, got)
		}
	}
	replayed, err := Reduce(initial, allEvents)
	if err != nil {
		t.Fatalf("Reduce(full play) error = %v", err)
	}
	if !reflect.DeepEqual(replayed, state) {
		t.Fatal("full event replay differs from decided state")
	}
}

func TestDecideClaimAtTrickBoundary(t *testing.T) {
	t.Parallel()

	state := playLegalCards(t, contractedTestBoard(t), 4)
	if len(state.CompletedTricks) != 1 || len(state.CurrentTrick.Plays) != 0 {
		t.Fatalf("claim boundary state = %+v", state)
	}
	claimTricks := 8
	decision, domainError := Decide(state, ClaimCommand(North, claimTricks))
	if domainError != nil {
		t.Fatalf("Decide(claim) error = %v", domainError)
	}
	if len(decision.Events) != 1 || decision.Events[0].Type != EventBoardClaimed {
		t.Fatalf("claim events = %+v", decision.Events)
	}
	if decision.NextState.Phase != PhaseBoardScored || !decision.NextState.Claimed || decision.NextState.Result == nil {
		t.Fatalf("claimed state = %+v", decision.NextState)
	}
	wantNS := state.TricksNS + claimTricks
	if decision.NextState.Result.TricksNS != wantNS || decision.NextState.Result.TricksEW != 13-wantNS {
		t.Fatalf("claim result = %+v, want NS=%d EW=%d", decision.NextState.Result, wantNS, 13-wantNS)
	}
	replayed, err := Reduce(state, decision.Events)
	if err != nil {
		t.Fatalf("Reduce(claim) error = %v", err)
	}
	if !reflect.DeepEqual(replayed, decision.NextState) {
		t.Fatal("replayed claim differs from decided state")
	}
}

func TestDecideRejectsInvalidClaims(t *testing.T) {
	t.Parallel()

	boundary := playLegalCards(t, contractedTestBoard(t), 4)
	partialTrick := playLegalCards(t, contractedTestBoard(t), 1)
	tests := []struct {
		name    string
		state   State
		command Command
		code    ErrorCode
	}{
		{name: "during auction", state: newTestBoard(t, 1), command: ClaimCommand(North, 1), code: ErrorInvalidState},
		{name: "during partial trick", state: partialTrick, command: ClaimCommand(North, 1), code: ErrorInvalidState},
		{name: "dummy requester", state: boundary, command: ClaimCommand(South, 1), code: ErrorInvalidCommand},
		{name: "negative tricks", state: boundary, command: ClaimCommand(North, -1), code: ErrorInvalidCommand},
		{name: "too many tricks", state: boundary, command: ClaimCommand(North, 13), code: ErrorInvalidCommand},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, domainError := Decide(test.state, test.command); domainError == nil || domainError.Code != test.code {
				t.Fatalf("Decide() error = %+v, want %s", domainError, test.code)
			}
		})
	}
}

func TestDecideIsDeterministicAndDoesNotMutateInput(t *testing.T) {
	t.Parallel()

	state := contractedTestBoard(t)
	before := state.clone()
	command := PlayCardCommand(East, state.Deal.East[0])
	first, firstError := Decide(state, command)
	second, secondError := Decide(state, command)
	if firstError != nil || secondError != nil {
		t.Fatalf("Decide() errors = %v, %v", firstError, secondError)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("same state and command produced different decisions")
	}
	if !reflect.DeepEqual(state, before) {
		t.Fatal("Decide() mutated input state")
	}
}

func TestEngineOutputsDoNotAliasInputsOrSiblings(t *testing.T) {
	t.Parallel()

	deal, err := GenerateDeal(constantReader(0x80))
	if err != nil {
		t.Fatalf("GenerateDeal() error = %v", err)
	}
	originalNorthCard := deal.North[0]
	state, err := NewBoard(1, deal)
	if err != nil {
		t.Fatalf("NewBoard() error = %v", err)
	}
	deal.North[0] = deal.East[0]
	if state.Deal.North[0] != originalNorthCard {
		t.Fatal("NewBoard() retained an alias to the source deal")
	}

	state = contractedTestBoard(t)
	decision, domainError := Decide(state, PlayCardCommand(East, state.Deal.East[0]))
	if domainError != nil {
		t.Fatalf("Decide(opening lead) error = %v", domainError)
	}
	eventCard := *decision.Events[0].Card
	decision.NextState.CurrentTrick.Plays[0].Card = state.Deal.North[0]
	if *decision.Events[0].Card != eventCard {
		t.Fatal("mutating next state changed decision events")
	}
	nextStateCard := decision.NextState.CurrentTrick.Plays[0].Card
	decision.Events[0].Card.Suit = "INVALID"
	if decision.NextState.CurrentTrick.Plays[0].Card != nextStateCard {
		t.Fatal("mutating decision events changed next state")
	}
}

func TestDecideRejectsInvalidCommands(t *testing.T) {
	t.Parallel()

	auctionState := newTestBoard(t, 1)
	playState := contractedTestBoard(t)
	notHeld := playState.Deal.North[0]
	tests := []struct {
		name    string
		state   State
		command Command
		code    ErrorCode
	}{
		{name: "unknown command", state: auctionState, command: Command{ActorSeat: North, Name: "UNKNOWN"}, code: ErrorInvalidCommand},
		{name: "call missing payload", state: auctionState, command: Command{ActorSeat: North, Name: CommandMakeCall}, code: ErrorInvalidCommand},
		{name: "play during auction", state: auctionState, command: PlayCardCommand(North, auctionState.Deal.North[0]), code: ErrorPlayComplete},
		{name: "call after auction", state: playState, command: MakeCallCommand(North, Pass()), code: ErrorAuctionComplete},
		{name: "wrong play actor", state: playState, command: PlayCardCommand(North, playState.Deal.East[0]), code: ErrorNotYourTurn},
		{name: "card not held", state: playState, command: PlayCardCommand(East, notHeld), code: ErrorCardNotHeld},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, domainError := Decide(test.state, test.command); domainError == nil || domainError.Code != test.code {
				t.Fatalf("Decide() error = %+v, want %s", domainError, test.code)
			}
		})
	}
}

func TestEventValidateShape(t *testing.T) {
	t.Parallel()

	call := Pass()
	card := Card{Suit: Spades, Rank: Ace}
	contract := Contract{Level: 1, Strain: StrainNoTrump, Doubling: Undoubled, Declarer: North}
	trick := Trick{Leader: East, Plays: []PlayedCard{}}
	result, err := ScoreContract(contract, VulnerabilityNone, 7)
	if err != nil {
		t.Fatalf("ScoreContract() error = %v", err)
	}
	valid := []Event{
		{Type: EventCallMade, Seat: North, Call: &call},
		{Type: EventAuctionPassedOut},
		{Type: EventContractSet, Contract: &contract},
		{Type: EventCardPlayed, Seat: East, Card: &card},
		{Type: EventDummyRevealed},
		{Type: EventTrickCompleted, Trick: &trick},
		{Type: EventBoardScored, Result: &result},
		{Type: EventBoardClaimed, Seat: North, Result: &result},
	}
	for _, event := range valid {
		event := event
		t.Run(string(event.Type)+" valid", func(t *testing.T) {
			t.Parallel()
			if err := event.validateShape(); err != nil {
				t.Fatalf("validateShape() error = %v", err)
			}
		})
	}

	tests := []struct {
		name  string
		event Event
	}{
		{name: "unknown event", event: Event{Type: "UNKNOWN"}},
		{name: "call without seat", event: Event{Type: EventCallMade, Call: &call}},
		{name: "passed out with seat", event: Event{Type: EventAuctionPassedOut, Seat: North}},
		{name: "contract with call", event: Event{Type: EventContractSet, Call: &call, Contract: &contract}},
		{name: "card with result", event: Event{Type: EventCardPlayed, Seat: East, Card: &card, Result: &result}},
		{name: "dummy with card", event: Event{Type: EventDummyRevealed, Card: &card}},
		{name: "trick with contract", event: Event{Type: EventTrickCompleted, Contract: &contract, Trick: &trick}},
		{name: "score with trick", event: Event{Type: EventBoardScored, Trick: &trick, Result: &result}},
		{name: "claim without seat", event: Event{Type: EventBoardClaimed, Result: &result}},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if err := test.event.validateShape(); err == nil {
				t.Fatal("validateShape() error = nil")
			}
		})
	}
}

func TestDecideRejectsIntermediateEventBoundaries(t *testing.T) {
	t.Parallel()

	openingState := contractedTestBoard(t)
	actor := openingState.Turn
	openingDecision, domainError := Decide(openingState, PlayCardCommand(actor, openingState.Deal.hand(actor)[0]))
	if domainError != nil {
		t.Fatalf("Decide(opening lead) error = %v", domainError)
	}
	partialOpening, err := Reduce(openingState, openingDecision.Events[:1])
	if err != nil {
		t.Fatalf("Reduce(partial opening) error = %v", err)
	}

	threeCards := playLegalCards(t, contractedTestBoard(t), 3)
	actor = threeCards.Turn
	legalCards, domainError := threeCards.LegalCards(actor)
	if domainError != nil {
		t.Fatalf("LegalCards(fourth card) error = %v", domainError)
	}
	fourthDecision, domainError := Decide(threeCards, PlayCardCommand(actor, legalCards[0]))
	if domainError != nil {
		t.Fatalf("Decide(fourth card) error = %v", domainError)
	}
	partialTrick, err := Reduce(threeCards, fourthDecision.Events[:1])
	if err != nil {
		t.Fatalf("Reduce(partial trick) error = %v", err)
	}

	fiftyOneCards := playLegalCards(t, contractedTestBoard(t), 51)
	actor = fiftyOneCards.Turn
	legalCards, domainError = fiftyOneCards.LegalCards(actor)
	if domainError != nil {
		t.Fatalf("LegalCards(final card) error = %v", domainError)
	}
	finalDecision, domainError := Decide(fiftyOneCards, PlayCardCommand(actor, legalCards[0]))
	if domainError != nil {
		t.Fatalf("Decide(final card) error = %v", domainError)
	}
	partialScore, err := Reduce(fiftyOneCards, finalDecision.Events[:2])
	if err != nil {
		t.Fatalf("Reduce(partial score) error = %v", err)
	}

	tests := []struct {
		name  string
		state State
	}{
		{name: "opening lead before dummy reveal", state: partialOpening},
		{name: "fourth card before trick completion", state: partialTrick},
		{name: "thirteenth trick before scoring", state: partialScore},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, domainError := Decide(test.state, Command{Name: "UNKNOWN"}); domainError == nil || domainError.Code != ErrorInvalidState {
				t.Fatalf("Decide() error = %+v, want %s", domainError, ErrorInvalidState)
			}
		})
	}
}

func newTestBoard(t *testing.T, boardNumber int) State {
	t.Helper()
	deal, err := GenerateDeal(constantReader(0x80))
	if err != nil {
		t.Fatalf("GenerateDeal() error = %v", err)
	}
	state, err := NewBoard(boardNumber, deal)
	if err != nil {
		t.Fatalf("NewBoard() error = %v", err)
	}
	return state
}

func contractedTestBoard(t *testing.T) State {
	t.Helper()
	state := newTestBoard(t, 1)
	for _, call := range []Call{Bid(1, StrainNoTrump), Pass(), Pass(), Pass()} {
		decision, domainError := Decide(state, MakeCallCommand(state.Turn, call))
		if domainError != nil {
			t.Fatalf("Decide(call %+v) error = %v", call, domainError)
		}
		state = decision.NextState
	}
	return state
}

func stateAfterOpeningLead(t *testing.T) State {
	t.Helper()
	state := contractedTestBoard(t)
	decision, domainError := Decide(state, PlayCardCommand(East, state.Deal.East[0]))
	if domainError != nil {
		t.Fatalf("Decide(opening lead) error = %v", domainError)
	}
	return decision.NextState
}
