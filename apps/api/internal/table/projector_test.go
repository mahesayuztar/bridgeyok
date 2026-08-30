package table

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/mahesayuztar/bridgeyok/apps/api/internal/bridge"
)

func TestProjectWaitingTable(t *testing.T) {
	t.Parallel()

	aggregate := testReadyAggregate(t)
	projection, domainError := Project(aggregate, "session-owner")
	if domainError != nil {
		t.Fatalf("Project() error = %v", domainError)
	}
	if projection.ViewerParticipantID != "participant-owner" || projection.ViewerRole != RoleOwner || projection.ViewerSeat != bridge.North {
		t.Fatalf("viewer projection = %+v", projection)
	}
	if projection.Game != nil || len(projection.Participants) != 4 || len(projection.Seats) != 4 {
		t.Fatalf("waiting projection = %+v", projection)
	}

	encoded, err := json.Marshal(projection)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	for _, participant := range aggregate.Participants {
		if strings.Contains(string(encoded), participant.SessionID) {
			t.Fatalf("projection contains private session id %q", participant.SessionID)
		}
	}
}

func TestProjectHidesOpponentHands(t *testing.T) {
	t.Parallel()

	aggregate := testStartedAggregate(t)
	for _, participant := range aggregate.Participants {
		projection, domainError := Project(aggregate, participant.SessionID)
		if domainError != nil {
			t.Fatalf("Project(%s) error = %v", participant.SessionID, domainError)
		}
		if projection.Game == nil {
			t.Fatal("active projection omitted game")
		}
		if len(projection.Game.LegalCalls) == 0 {
			t.Fatal("auction projection omitted engine-authoritative legal calls")
		}
		wantHand := aggregate.Game.Deal.Hand(projection.ViewerSeat)
		if !reflect.DeepEqual(projection.Game.OwnHand, wantHand) {
			t.Fatalf("own hand = %+v, want %+v", projection.Game.OwnHand, wantHand)
		}
		if len(projection.Game.DummyHand) != 0 || projection.Game.FullDeal != nil {
			t.Fatalf("pre-lead projection exposed hidden cards: %+v", projection.Game)
		}
	}
}

func TestProjectEncodesEmptyCollectionsAsArrays(t *testing.T) {
	t.Parallel()

	projection, domainError := Project(testStartedAggregate(t), "session-owner")
	if domainError != nil {
		t.Fatalf("Project() error = %v", domainError)
	}
	encoded, err := json.Marshal(projection)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	for _, expected := range []string{`"calls":[]`, `"plays":[]`, `"completedTricks":[]`} {
		if !strings.Contains(string(encoded), expected) {
			t.Fatalf("projection JSON does not contain %s: %s", expected, encoded)
		}
	}
}

func TestProjectRevealsOnlyDummyAfterOpeningLead(t *testing.T) {
	t.Parallel()

	aggregate := testStartedAggregate(t)
	calls := []bridge.Call{bridge.Bid(1, bridge.StrainClubs), bridge.Pass(), bridge.Pass(), bridge.Pass()}
	for _, call := range calls {
		sessionID := sessionForSeat(t, aggregate, aggregate.Game.Turn)
		aggregate = acceptedDecision(t, aggregate, Command{Name: CommandMakeCall, SessionID: sessionID, Call: &call}).NextState
	}
	openingLeader := aggregate.Game.Turn
	legalCards, domainError := aggregate.Game.LegalCards(openingLeader)
	if domainError != nil || len(legalCards) == 0 {
		t.Fatalf("LegalCards() = %+v, error = %v", legalCards, domainError)
	}
	aggregate = acceptedDecision(t, aggregate, Command{
		Name:      CommandPlayCard,
		SessionID: sessionForSeat(t, aggregate, openingLeader),
		Card:      &legalCards[0],
	}).NextState

	projection, projectionError := Project(aggregate, sessionForSeat(t, aggregate, bridge.West))
	if projectionError != nil {
		t.Fatalf("Project() error = %v", projectionError)
	}
	dummy := aggregate.Game.Auction.Contract.Dummy()
	if !reflect.DeepEqual(projection.Game.DummyHand, aggregate.Game.Deal.Hand(dummy)) {
		t.Fatalf("dummy hand = %+v, want seat %s hand", projection.Game.DummyHand, dummy)
	}
	if projection.Game.FullDeal != nil {
		t.Fatal("opening lead projection exposed full deal")
	}
}

func TestProjectRevealsFullDealOnlyAfterScore(t *testing.T) {
	t.Parallel()

	aggregate := testStartedAggregate(t)
	for aggregate.State == StateActive {
		call := bridge.Pass()
		aggregate = acceptedDecision(t, aggregate, Command{
			Name:      CommandMakeCall,
			SessionID: sessionForSeat(t, aggregate, aggregate.Game.Turn),
			Call:      &call,
		}).NextState
	}
	projection, domainError := Project(aggregate, "session-owner")
	if domainError != nil {
		t.Fatalf("Project() error = %v", domainError)
	}
	if projection.Game.FullDeal == nil || !reflect.DeepEqual(*projection.Game.FullDeal, aggregate.Game.Deal) {
		t.Fatal("scored projection omitted full deal")
	}
}

func TestProjectReturnsDefensiveCopies(t *testing.T) {
	t.Parallel()

	aggregate := testStartedAggregate(t)
	call := bridge.Bid(1, bridge.StrainClubs)
	aggregate = acceptedDecision(t, aggregate, Command{Name: CommandMakeCall, SessionID: "session-owner", Call: &call}).NextState
	before := aggregate.clone()
	projection, domainError := Project(aggregate, "session-owner")
	if domainError != nil {
		t.Fatalf("Project() error = %v", domainError)
	}
	projection.Game.OwnHand[0] = bridge.Card{}
	projection.Game.Auction.Calls[0].Call = bridge.Pass()
	projection.Game.LegalCalls[0] = bridge.Double()
	if !reflect.DeepEqual(aggregate, before) {
		t.Fatal("mutating projection changed authoritative aggregate")
	}
}

func TestProjectRejectsNonParticipant(t *testing.T) {
	t.Parallel()

	_, domainError := Project(testAggregate(t), "session-unknown")
	assertDomainError(t, domainError, ErrorNotParticipant)
}

func testStartedAggregate(t *testing.T) Aggregate {
	t.Helper()
	aggregate := testReadyAggregate(t)
	deal := testDeal(t)
	return acceptedDecision(t, aggregate, Command{
		Name:      CommandStartGame,
		SessionID: "session-owner",
		Deal:      &deal,
		BoardID:   "board-one",
	}).NextState
}

func sessionForSeat(t *testing.T, aggregate Aggregate, seat bridge.Seat) string {
	t.Helper()
	assignment, exists := aggregate.Seats[seat]
	if !exists {
		t.Fatalf("seat %s is not occupied", seat)
	}
	for _, participant := range aggregate.Participants {
		if participant.ID == assignment.ParticipantID {
			return participant.SessionID
		}
	}
	t.Fatalf("seat %s references missing participant", seat)
	return ""
}
