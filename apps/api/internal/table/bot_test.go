package table

import (
	"testing"
	"time"

	"github.com/mahesayuztar/bridgeyok/apps/api/internal/bridge"
)

func TestDecideBotSeatLifecycle(t *testing.T) {
	t.Parallel()

	aggregate := testAggregateWithGuests(t, 2)
	guest := aggregate.Participants[1]
	aggregate = acceptedDecision(t, aggregate, Command{Name: CommandTakeSeat, SessionID: guest.SessionID, Seat: bridge.East}).NextState

	_, domainError := Decide(aggregate, Command{Name: CommandAddBot, SessionID: guest.SessionID, Seat: bridge.North, BotID: "bot-north"})
	assertDomainError(t, domainError, ErrorOwnerRequired)

	aggregate = acceptedDecision(t, aggregate, Command{Name: CommandAddBot, SessionID: aggregate.OwnerSessionID, Seat: bridge.North, BotID: "bot-north"}).NextState
	if assignment := aggregate.Seats[bridge.North]; !assignment.IsBot || !assignment.Ready {
		t.Fatalf("bot assignment = %+v", assignment)
	}

	aggregate = acceptedDecision(t, aggregate, Command{
		Name: CommandReplaceWithBot, SessionID: aggregate.OwnerSessionID, ParticipantID: guest.ID,
		BotID: "bot-east", OccurredAt: testJoinedAt.Add(time.Minute),
	}).NextState
	if assignment := aggregate.Seats[bridge.East]; !assignment.IsBot || assignment.ParticipantID != "bot-east" {
		t.Fatalf("replacement bot assignment = %+v", assignment)
	}
	if _, active := aggregate.activeParticipant(guest.SessionID); active {
		t.Fatal("replaced participant remained active")
	}

	aggregate = acceptedDecision(t, aggregate, Command{Name: CommandRemoveBot, SessionID: aggregate.OwnerSessionID, Seat: bridge.North}).NextState
	if _, occupied := aggregate.Seats[bridge.North]; occupied {
		t.Fatal("removed bot retained its seat")
	}
}

func TestNextBotCommandUsesFirstLegalCall(t *testing.T) {
	t.Parallel()

	aggregate := testAggregateWithGuests(t, 2)
	aggregate = acceptedDecision(t, aggregate, Command{Name: CommandAddBot, SessionID: aggregate.OwnerSessionID, Seat: bridge.North, BotID: "bot-north"}).NextState
	for _index, participant := range aggregate.Participants {
		seat := []bridge.Seat{bridge.East, bridge.South, bridge.West}[_index]
		aggregate = acceptedDecision(t, aggregate, Command{Name: CommandTakeSeat, SessionID: participant.SessionID, Seat: seat}).NextState
		aggregate = acceptedDecision(t, aggregate, Command{Name: CommandSetReady, SessionID: participant.SessionID, Ready: true}).NextState
	}
	deal := testDeal(t)
	aggregate = acceptedDecision(t, aggregate, Command{Name: CommandStartGame, SessionID: aggregate.OwnerSessionID, Deal: &deal, BoardID: "board-one"}).NextState

	command, ready := nextBotCommand(aggregate)
	if !ready || command.Name != CommandMakeCall || command.BotSeat != bridge.North || command.Call == nil || *command.Call != bridge.Pass() {
		t.Fatalf("nextBotCommand() = %+v, %t", command, ready)
	}
}

func TestNextBotCommandUsesFirstLegalCard(t *testing.T) {
	t.Parallel()

	aggregate := testStartedAggregate(t)
	for _, call := range []bridge.Call{bridge.Bid(1, bridge.StrainClubs), bridge.Pass(), bridge.Pass(), bridge.Pass()} {
		aggregate = acceptedDecision(t, aggregate, Command{Name: CommandMakeCall, SessionID: sessionForSeat(t, aggregate, aggregate.Game.Turn), Call: &call}).NextState
	}
	openingLeader := aggregate.Game.Turn
	target := aggregate.Seats[openingLeader]
	aggregate = acceptedDecision(t, aggregate, Command{
		Name: CommandReplaceWithBot, SessionID: aggregate.OwnerSessionID, ParticipantID: target.ParticipantID,
		BotID: "bot-opening-leader", OccurredAt: testJoinedAt.Add(time.Minute),
	}).NextState
	legalCards, domainError := aggregate.Game.LegalCards(openingLeader)
	if domainError != nil || len(legalCards) == 0 {
		t.Fatalf("LegalCards() cards = %d, error = %v", len(legalCards), domainError)
	}

	command, ready := nextBotCommand(aggregate)
	if !ready || command.Name != CommandPlayCard || command.BotSeat != openingLeader || command.Card == nil || *command.Card != legalCards[0] {
		t.Fatalf("nextBotCommand() = %+v, %t", command, ready)
	}
}

func TestBotSeatDisablesConsensusActions(t *testing.T) {
	t.Parallel()

	aggregate := testStartedAggregate(t)
	call := bridge.Bid(1, bridge.StrainClubs)
	aggregate = acceptedDecision(t, aggregate, Command{
		Name: CommandMakeCall, SessionID: aggregate.OwnerSessionID, Call: &call,
	}).NextState
	eastParticipantID := aggregate.Seats[bridge.East].ParticipantID
	aggregate = acceptedDecision(t, aggregate, Command{
		Name: CommandReplaceWithBot, SessionID: aggregate.OwnerSessionID, ParticipantID: eastParticipantID,
		BotID: "bot-east", OccurredAt: testJoinedAt.Add(time.Minute),
	}).NextState

	_, domainError := Decide(aggregate, Command{Name: CommandRequestUndo, SessionID: aggregate.OwnerSessionID})
	assertDomainError(t, domainError, ErrorInvalidState)
	projection, domainError := Project(aggregate, aggregate.OwnerSessionID)
	if domainError != nil {
		t.Fatalf("Project() error = %v", domainError)
	}
	if projection.CanRequestUndo {
		t.Fatal("bot table exposed undo action")
	}
}
