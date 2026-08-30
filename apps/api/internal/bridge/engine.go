package bridge

import (
	"fmt"
	"reflect"
)

// Phase identifies the current board engine state.
type Phase string

const (
	PhaseAuction     Phase = "AUCTION"
	PhaseOpeningLead Phase = "OPENING_LEAD"
	PhasePlay        Phase = "PLAY"
	PhaseBoardScored Phase = "BOARD_SCORED"
)

// CommandName identifies a mutation understood by the pure board engine.
type CommandName string

const (
	CommandMakeCall CommandName = "MAKE_CALL"
	CommandPlayCard CommandName = "PLAY_CARD"
)

// Command is one typed game mutation request.
type Command struct {
	ActorSeat Seat        `json:"actorSeat"`
	Name      CommandName `json:"name"`
	Call      *Call       `json:"call,omitempty"`
	Card      *Card       `json:"card,omitempty"`
}

// EventType identifies one deterministic state transition.
type EventType string

const (
	EventCallMade         EventType = "CALL_MADE"
	EventAuctionPassedOut EventType = "AUCTION_PASSED_OUT"
	EventContractSet      EventType = "CONTRACT_SET"
	EventCardPlayed       EventType = "CARD_PLAYED"
	EventDummyRevealed    EventType = "DUMMY_REVEALED"
	EventTrickCompleted   EventType = "TRICK_COMPLETED"
	EventBoardScored      EventType = "BOARD_SCORED"
)

// Event is a complete replayable domain fact.
type Event struct {
	Type     EventType `json:"type"`
	Seat     Seat      `json:"seat,omitempty"`
	Call     *Call     `json:"call,omitempty"`
	Card     *Card     `json:"card,omitempty"`
	Contract *Contract `json:"contract,omitempty"`
	Trick    *Trick    `json:"trick,omitempty"`
	Result   *Result   `json:"result,omitempty"`
}

func (event Event) validateShape() error {
	switch event.Type {
	case EventCallMade:
		if !event.Seat.Valid() || event.Call == nil || event.Card != nil || event.Contract != nil || event.Trick != nil || event.Result != nil {
			return fmt.Errorf("invalid call-made event payload")
		}
	case EventAuctionPassedOut:
		if event.Seat != "" || event.Call != nil || event.Card != nil || event.Contract != nil || event.Trick != nil || event.Result != nil {
			return fmt.Errorf("invalid passed-out event payload")
		}
	case EventContractSet:
		if event.Seat != "" || event.Call != nil || event.Card != nil || event.Contract == nil || event.Trick != nil || event.Result != nil {
			return fmt.Errorf("invalid contract-set event payload")
		}
	case EventCardPlayed:
		if !event.Seat.Valid() || event.Call != nil || event.Card == nil || event.Contract != nil || event.Trick != nil || event.Result != nil {
			return fmt.Errorf("invalid card-played event payload")
		}
	case EventDummyRevealed:
		if event.Seat != "" || event.Call != nil || event.Card != nil || event.Contract != nil || event.Trick != nil || event.Result != nil {
			return fmt.Errorf("invalid dummy-revealed event payload")
		}
	case EventTrickCompleted:
		if event.Seat != "" || event.Call != nil || event.Card != nil || event.Contract != nil || event.Trick == nil || event.Result != nil {
			return fmt.Errorf("invalid trick-completed event payload")
		}
	case EventBoardScored:
		if event.Seat != "" || event.Call != nil || event.Card != nil || event.Contract != nil || event.Trick != nil || event.Result == nil {
			return fmt.Errorf("invalid board-scored event payload")
		}
	default:
		return fmt.Errorf("unknown event type %q", event.Type)
	}
	return nil
}

// State is the authoritative private state of one board.
type State struct {
	RulesetVersion  string        `json:"rulesetVersion"`
	Board           BoardMetadata `json:"board"`
	Phase           Phase         `json:"phase"`
	Deal            Deal          `json:"deal"`
	Auction         Auction       `json:"auction"`
	Turn            Seat          `json:"turn,omitempty"`
	DummyRevealed   bool          `json:"dummyRevealed"`
	CurrentTrick    Trick         `json:"currentTrick"`
	CompletedTricks []Trick       `json:"completedTricks"`
	TricksNS        int           `json:"tricksNS"`
	TricksEW        int           `json:"tricksEW"`
	Result          *Result       `json:"result,omitempty"`
}

// Decision contains the next immutable state and facts needed to replay it.
type Decision struct {
	NextState State   `json:"nextState"`
	Events    []Event `json:"events"`
}

// MakeCallCommand creates a structurally valid auction command.
func MakeCallCommand(actor Seat, call Call) Command {
	callCopy := call
	return Command{ActorSeat: actor, Name: CommandMakeCall, Call: &callCopy}
}

// PlayCardCommand creates a structurally valid card-play command.
func PlayCardCommand(actor Seat, card Card) Command {
	cardCopy := card
	return Command{ActorSeat: actor, Name: CommandPlayCard, Card: &cardCopy}
}

// NewBoard creates a board at the beginning of its auction.
func NewBoard(boardNumber int, deal Deal) (State, error) {
	if err := deal.Validate(); err != nil {
		return State{}, fmt.Errorf("deal: %w", err)
	}
	metadata, err := MetadataForBoard(boardNumber)
	if err != nil {
		return State{}, err
	}
	auction, err := NewAuction(metadata.Dealer)
	if err != nil {
		return State{}, err
	}
	state := State{
		RulesetVersion:  RulesetVersion,
		Board:           metadata,
		Phase:           PhaseAuction,
		Deal:            deal.clone(),
		Auction:         auction,
		Turn:            metadata.Dealer,
		CompletedTricks: []Trick{},
	}
	if err := state.ValidateInvariants(); err != nil {
		return State{}, err
	}
	return state, nil
}

// Decide validates a command and returns deterministic events plus the resulting state.
func Decide(state State, command Command) (Decision, *DomainError) {
	if err := state.ValidateInvariants(); err != nil {
		return Decision{}, reject(ErrorInvalidState, err.Error())
	}
	if err := state.validateCommandBoundary(); err != nil {
		return Decision{}, reject(ErrorInvalidState, err.Error())
	}

	var events []Event
	var domainError *DomainError
	switch command.Name {
	case CommandMakeCall:
		events, domainError = decideCall(state, command)
	case CommandPlayCard:
		events, domainError = decidePlay(state, command)
	default:
		domainError = reject(ErrorInvalidCommand, "unknown command")
	}
	if domainError != nil {
		return Decision{}, domainError
	}

	nextState, err := reduceFromValid(state, events)
	if err != nil {
		return Decision{}, reject(ErrorInvalidState, err.Error())
	}
	return Decision{NextState: nextState, Events: events}, nil
}

func (state State) validateCommandBoundary() error {
	switch state.Phase {
	case PhaseOpeningLead:
		if len(state.CurrentTrick.Plays) != 0 {
			return fmt.Errorf("opening lead transition is incomplete")
		}
	case PhasePlay:
		if len(state.CurrentTrick.Plays) == 4 || len(state.CompletedTricks) == 13 {
			return fmt.Errorf("derived play transition is incomplete")
		}
	}
	return nil
}

func decideCall(state State, command Command) ([]Event, *DomainError) {
	if state.Phase != PhaseAuction {
		return nil, reject(ErrorAuctionComplete, "board is no longer in auction")
	}
	if command.Call == nil || command.Card != nil {
		return nil, reject(ErrorInvalidCommand, "make call requires only a call payload")
	}

	nextAuction, domainError := state.Auction.MakeCall(command.ActorSeat, *command.Call)
	if domainError != nil {
		return nil, domainError
	}
	callCopy := *command.Call
	events := []Event{{Type: EventCallMade, Seat: command.ActorSeat, Call: &callCopy}}
	if nextAuction.PassedOut {
		events = append(events, Event{Type: EventAuctionPassedOut})
	} else if nextAuction.Complete {
		contract := *nextAuction.Contract
		events = append(events, Event{Type: EventContractSet, Contract: &contract})
	}
	return events, nil
}

func decidePlay(state State, command Command) ([]Event, *DomainError) {
	if state.Phase != PhaseOpeningLead && state.Phase != PhasePlay {
		return nil, reject(ErrorPlayComplete, "board is not accepting card play")
	}
	if command.Card == nil || command.Call != nil {
		return nil, reject(ErrorInvalidCommand, "play card requires only a card payload")
	}
	if err := command.Card.Validate(); err != nil {
		return nil, reject(ErrorInvalidCommand, err.Error())
	}
	if domainError := state.authorizePlay(command.ActorSeat); domainError != nil {
		return nil, domainError
	}

	hand := state.Deal.hand(state.Turn)
	if !hand.contains(*command.Card) {
		return nil, reject(ErrorCardNotHeld, "card is not in the active hand")
	}
	if len(state.CurrentTrick.Plays) > 0 {
		ledSuit := state.CurrentTrick.Plays[0].Card.Suit
		if command.Card.Suit != ledSuit && hand.hasSuit(ledSuit) {
			return nil, reject(ErrorMustFollowSuit, "active hand must follow the led suit")
		}
	}

	cardCopy := *command.Card
	events := []Event{{Type: EventCardPlayed, Seat: state.Turn, Card: &cardCopy}}
	if state.Phase == PhaseOpeningLead {
		events = append(events, Event{Type: EventDummyRevealed})
	}
	if len(state.CurrentTrick.Plays) != 3 {
		return events, nil
	}

	completedTrick := state.CurrentTrick.clone()
	completedTrick.Plays = append(completedTrick.Plays, PlayedCard{Seat: state.Turn, Card: cardCopy})
	winner, err := trickWinner(completedTrick, state.Auction.Contract.Strain)
	if err != nil {
		return nil, reject(ErrorInvalidState, err.Error())
	}
	completedTrick.Winner = winner
	events = append(events, Event{Type: EventTrickCompleted, Trick: &completedTrick})
	if len(state.CompletedTricks) != 12 {
		return events, nil
	}

	tricksNS := state.TricksNS
	tricksEW := state.TricksEW
	if winner.Partnership() == NorthSouth {
		tricksNS++
	} else {
		tricksEW++
	}
	tricksDeclarer := tricksNS
	if state.Auction.Contract.Declarer.Partnership() == EastWest {
		tricksDeclarer = tricksEW
	}
	result, err := ScoreContract(*state.Auction.Contract, state.Board.Vulnerability, tricksDeclarer)
	if err != nil {
		return nil, reject(ErrorInvalidState, err.Error())
	}
	events = append(events, Event{Type: EventBoardScored, Result: &result})
	return events, nil
}

func (state State) authorizePlay(actor Seat) *DomainError {
	contract := state.Auction.Contract
	if contract == nil {
		return reject(ErrorInvalidState, "play requires a contract")
	}
	if state.Turn == contract.Dummy() {
		if actor == contract.Dummy() {
			return reject(ErrorDeclarerControlsDummy, "declarer controls dummy")
		}
		if actor != contract.Declarer {
			return reject(ErrorNotYourTurn, "actor does not control the active hand")
		}
		return nil
	}
	if actor != state.Turn {
		return reject(ErrorNotYourTurn, "actor does not control the active hand")
	}
	return nil
}

// LegalCards returns the cards the actor may submit for the active hand.
func (state State) LegalCards(actor Seat) ([]Card, *DomainError) {
	if state.Phase != PhaseOpeningLead && state.Phase != PhasePlay {
		return nil, reject(ErrorPlayComplete, "board is not accepting card play")
	}
	if domainError := state.authorizePlay(actor); domainError != nil {
		return nil, domainError
	}
	hand := state.Deal.hand(state.Turn)
	if len(state.CurrentTrick.Plays) == 0 {
		return append([]Card{}, hand...), nil
	}
	ledSuit := state.CurrentTrick.Plays[0].Card.Suit
	if !hand.hasSuit(ledSuit) {
		return append([]Card{}, hand...), nil
	}
	legal := make([]Card, 0, len(hand))
	for _, card := range hand {
		if card.Suit == ledSuit {
			legal = append(legal, card)
		}
	}
	return legal, nil
}

// Reduce applies replayable events without consulting clocks, random sources, or external state.
func Reduce(state State, events []Event) (State, error) {
	if err := state.ValidateInvariants(); err != nil {
		return State{}, fmt.Errorf("initial state: %w", err)
	}
	return reduceFromValid(state, events)
}

func reduceFromValid(state State, events []Event) (State, error) {
	next := state.clone()
	for _index, event := range events {
		if err := next.apply(event); err != nil {
			return State{}, fmt.Errorf("event %d %s: %w", _index, event.Type, err)
		}
	}
	if err := next.ValidateInvariants(); err != nil {
		return State{}, fmt.Errorf("reduced state violates invariants: %w", err)
	}
	return next, nil
}

func (state *State) apply(event Event) error {
	if err := event.validateShape(); err != nil {
		return err
	}
	switch event.Type {
	case EventCallMade:
		if state.Phase != PhaseAuction {
			return fmt.Errorf("invalid call-made event")
		}
		nextAuction, domainError := state.Auction.MakeCall(event.Seat, *event.Call)
		if domainError != nil {
			return domainError
		}
		state.Auction = nextAuction
		state.Turn = nextAuction.Turn
		return nil
	case EventAuctionPassedOut:
		if state.Phase != PhaseAuction || !state.Auction.PassedOut {
			return fmt.Errorf("invalid passed-out event")
		}
		result, err := PassedOutResult(state.Board.Vulnerability)
		if err != nil {
			return err
		}
		state.Phase = PhaseBoardScored
		state.Turn = ""
		state.Result = &result
		return nil
	case EventContractSet:
		if state.Phase != PhaseAuction || !state.Auction.Complete || state.Auction.PassedOut || !reflect.DeepEqual(state.Auction.Contract, event.Contract) {
			return fmt.Errorf("invalid contract-set event")
		}
		state.Phase = PhaseOpeningLead
		state.Turn = event.Contract.OpeningLeader()
		state.CurrentTrick = Trick{Leader: state.Turn, Plays: []PlayedCard{}}
		return nil
	case EventCardPlayed:
		if (state.Phase != PhaseOpeningLead && state.Phase != PhasePlay) || event.Seat != state.Turn || len(state.CurrentTrick.Plays) >= 4 {
			return fmt.Errorf("invalid card-played event")
		}
		hand := state.Deal.hand(event.Seat)
		if len(state.CurrentTrick.Plays) > 0 {
			ledSuit := state.CurrentTrick.Plays[0].Card.Suit
			if event.Card.Suit != ledSuit && hand.hasSuit(ledSuit) {
				return fmt.Errorf("played card does not follow suit")
			}
		}
		if !state.Deal.removeCard(event.Seat, *event.Card) {
			return fmt.Errorf("played card is not in active hand")
		}
		state.CurrentTrick.Plays = append(state.CurrentTrick.Plays, PlayedCard{Seat: event.Seat, Card: *event.Card})
		state.Turn = event.Seat.Next()
		return nil
	case EventDummyRevealed:
		if state.Phase != PhaseOpeningLead || len(state.CurrentTrick.Plays) != 1 || state.DummyRevealed {
			return fmt.Errorf("invalid dummy-revealed event")
		}
		state.DummyRevealed = true
		state.Phase = PhasePlay
		return nil
	case EventTrickCompleted:
		if state.Phase != PhasePlay || len(state.CurrentTrick.Plays) != 4 || !reflect.DeepEqual(event.Trick.Plays, state.CurrentTrick.Plays) || event.Trick.Leader != state.CurrentTrick.Leader {
			return fmt.Errorf("invalid trick-completed event")
		}
		winner, err := trickWinner(state.CurrentTrick, state.Auction.Contract.Strain)
		if err != nil || event.Trick.Winner != winner {
			return fmt.Errorf("invalid trick winner")
		}
		completed := event.Trick.clone()
		state.CompletedTricks = append(state.CompletedTricks, completed)
		if winner.Partnership() == NorthSouth {
			state.TricksNS++
		} else {
			state.TricksEW++
		}
		state.Turn = winner
		state.CurrentTrick = Trick{Leader: winner, Plays: []PlayedCard{}}
		return nil
	case EventBoardScored:
		if state.Phase != PhasePlay || len(state.CompletedTricks) != 13 {
			return fmt.Errorf("invalid board-scored event")
		}
		if err := event.Result.Validate(); err != nil {
			return err
		}
		if event.Result.TricksNS != state.TricksNS || event.Result.TricksEW != state.TricksEW || event.Result.Vulnerability != state.Board.Vulnerability || !reflect.DeepEqual(event.Result.Contract, state.Auction.Contract) {
			return fmt.Errorf("score does not match completed play")
		}
		result := *event.Result
		contract := *event.Result.Contract
		result.Contract = &contract
		state.Result = &result
		state.Phase = PhaseBoardScored
		state.Turn = ""
		return nil
	}
	return fmt.Errorf("unknown event type %q", event.Type)
}

func (state State) clone() State {
	clone := state
	clone.Deal = state.Deal.clone()
	clone.Auction = state.Auction.clone()
	clone.CurrentTrick = state.CurrentTrick.clone()
	clone.CompletedTricks = make([]Trick, len(state.CompletedTricks))
	for _index, trick := range state.CompletedTricks {
		clone.CompletedTricks[_index] = trick.clone()
	}
	if state.Result != nil {
		result := *state.Result
		if state.Result.Contract != nil {
			contract := *state.Result.Contract
			result.Contract = &contract
		}
		clone.Result = &result
	}
	return clone
}
