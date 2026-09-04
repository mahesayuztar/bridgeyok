package table

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/mahesayuztar/bridgeyok/apps/api/internal/bridge"
)

var testJoinedAt = time.Date(2026, 8, 30, 9, 0, 0, 0, time.UTC)

func TestNewAggregate(t *testing.T) {
	t.Parallel()

	owner := testParticipant("participant-owner", "session-owner", "Owner", RoleOwner)
	aggregate, err := NewAggregate("table-one", owner)
	if err != nil {
		t.Fatalf("NewAggregate() error = %v", err)
	}
	if aggregate.State != StateWaiting || aggregate.OwnerSessionID != owner.SessionID || len(aggregate.Participants) != 1 || aggregate.Seats == nil {
		t.Fatalf("NewAggregate() = %+v", aggregate)
	}
}

func TestDomainError(t *testing.T) {
	t.Parallel()

	var nilError *DomainError
	if nilError.Error() != "" {
		t.Fatalf("nil DomainError.Error() = %q", nilError.Error())
	}
	domainError := &DomainError{Code: ErrorInvalidState, Message: "invalid state"}
	if domainError.Error() != "invalid state" {
		t.Fatalf("DomainError.Error() = %q", domainError.Error())
	}
}

func TestDecideParticipantLifecycle(t *testing.T) {
	t.Parallel()

	aggregate := testAggregate(t)
	guest := testParticipant("participant-east", "session-east", "East", RoleParticipant)
	aggregate = acceptedDecision(t, aggregate, Command{Name: CommandJoinTable, Participant: &guest}).NextState

	beforeDuplicate := aggregate.clone()
	_, domainError := Decide(aggregate, Command{Name: CommandJoinTable, Participant: &guest})
	assertDomainError(t, domainError, ErrorAlreadyParticipant)
	if !reflect.DeepEqual(aggregate, beforeDuplicate) {
		t.Fatal("rejected duplicate join mutated aggregate")
	}

	aggregate = acceptedDecision(t, aggregate, Command{Name: CommandTakeSeat, SessionID: guest.SessionID, Seat: bridge.East}).NextState
	aggregate = acceptedDecision(t, aggregate, Command{Name: CommandSetReady, SessionID: guest.SessionID, Ready: true}).NextState
	aggregate = acceptedDecision(t, aggregate, Command{Name: CommandTakeSeat, SessionID: guest.SessionID, Seat: bridge.South}).NextState
	if _, occupied := aggregate.Seats[bridge.East]; occupied {
		t.Fatal("old seat remained occupied after move")
	}
	if aggregate.Seats[bridge.South].Ready {
		t.Fatal("ready state remained set after seat move")
	}

	leftAt := testJoinedAt.Add(time.Minute)
	decision := acceptedDecision(t, aggregate, Command{Name: CommandLeaveTable, SessionID: guest.SessionID, OccurredAt: leftAt})
	if _, occupied := decision.NextState.Seats[bridge.South]; occupied {
		t.Fatal("seat remained occupied after leaving table")
	}
	if _, active := decision.NextState.activeParticipant(guest.SessionID); active {
		t.Fatal("participant remained active after leaving table")
	}
}

func TestDecideJoinBoundaries(t *testing.T) {
	t.Parallel()

	guest := testParticipant("participant-new", "session-new", "New Guest", RoleParticipant)
	full := testAggregateWithGuests(t, 3)
	locked := testAggregate(t)
	locked.Locked = true
	active := testAggregate(t)
	active.State = StateFinished

	tests := []struct {
		name      string
		aggregate Aggregate
		command   Command
		wantCode  ErrorCode
	}{
		{name: "locked", aggregate: locked, command: Command{Name: CommandJoinTable, Participant: &guest}, wantCode: ErrorTableLocked},
		{name: "full", aggregate: full, command: Command{Name: CommandJoinTable, Participant: &guest}, wantCode: ErrorTableFull},
		{name: "finished", aggregate: active, command: Command{Name: CommandJoinTable, Participant: &guest}, wantCode: ErrorInvalidState},
		{name: "missing participant", aggregate: testAggregate(t), command: Command{Name: CommandJoinTable}, wantCode: ErrorInvalidCommand},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			before := test.aggregate.clone()
			_, domainError := Decide(test.aggregate, test.command)
			assertDomainError(t, domainError, test.wantCode)
			if !reflect.DeepEqual(test.aggregate, before) {
				t.Fatal("rejected join mutated aggregate")
			}
		})
	}
}

func TestDecideSeatRaceAndOwnerControls(t *testing.T) {
	t.Parallel()

	aggregate := testAggregateWithGuests(t, 2)
	first := aggregate.Participants[1]
	second := aggregate.Participants[2]
	aggregate = acceptedDecision(t, aggregate, Command{Name: CommandTakeSeat, SessionID: first.SessionID, Seat: bridge.North}).NextState

	beforeRaceLoss := aggregate.clone()
	_, domainError := Decide(aggregate, Command{Name: CommandTakeSeat, SessionID: second.SessionID, Seat: bridge.North})
	assertDomainError(t, domainError, ErrorSeatTaken)
	if !reflect.DeepEqual(aggregate, beforeRaceLoss) {
		t.Fatal("losing seat command mutated aggregate")
	}

	_, domainError = Decide(aggregate, Command{Name: CommandLockTable, SessionID: first.SessionID, Locked: true})
	assertDomainError(t, domainError, ErrorOwnerRequired)
	aggregate = acceptedDecision(t, aggregate, Command{Name: CommandLockTable, SessionID: "session-owner", Locked: true}).NextState
	if !aggregate.Locked {
		t.Fatal("owner lock command did not lock table")
	}
	aggregate = acceptedDecision(t, aggregate, Command{Name: CommandLockTable, SessionID: "session-owner", Locked: false}).NextState

	removedAt := testJoinedAt.Add(2 * time.Minute)
	aggregate = acceptedDecision(t, aggregate, Command{
		Name:          CommandRemoveParticipant,
		SessionID:     "session-owner",
		ParticipantID: first.ID,
		OccurredAt:    removedAt,
	}).NextState
	if _, occupied := aggregate.Seats[bridge.North]; occupied {
		t.Fatal("removed participant retained seat")
	}
	if _, active := aggregate.activeParticipant(first.SessionID); active {
		t.Fatal("removed participant remained active")
	}

	decision := acceptedDecision(t, aggregate, Command{Name: CommandLeaveTable, SessionID: "session-owner", OccurredAt: removedAt})
	if decision.NextState.OwnerSessionID != second.SessionID || decision.NextState.Participants[2].Role != RoleOwner {
		t.Fatalf("owner leave replacement = %+v", decision.NextState.Participants)
	}
}

func TestDecideWaitingSeatAndFinishCommands(t *testing.T) {
	t.Parallel()

	aggregate := testAggregateWithGuests(t, 1)
	ownerSessionID := aggregate.OwnerSessionID
	aggregate = acceptedDecision(t, aggregate, Command{Name: CommandTakeSeat, SessionID: ownerSessionID, Seat: bridge.North}).NextState
	_, domainError := Decide(aggregate, Command{Name: CommandTakeSeat, SessionID: ownerSessionID, Seat: bridge.North})
	assertDomainError(t, domainError, ErrorAlreadySeated)
	aggregate = acceptedDecision(t, aggregate, Command{Name: CommandSetReady, SessionID: ownerSessionID, Ready: true}).NextState
	aggregate = acceptedDecision(t, aggregate, Command{Name: CommandLeaveSeat, SessionID: ownerSessionID}).NextState
	if _, occupied := aggregate.Seats[bridge.North]; occupied {
		t.Fatal("leave seat retained assignment")
	}
	_, domainError = Decide(aggregate, Command{Name: CommandSetReady, SessionID: ownerSessionID, Ready: true})
	assertDomainError(t, domainError, ErrorSeatRequired)

	guest := aggregate.Participants[1]
	_, domainError = Decide(aggregate, Command{
		Name:          CommandRemoveParticipant,
		SessionID:     ownerSessionID,
		ParticipantID: guest.ID,
	})
	assertDomainError(t, domainError, ErrorInvalidCommand)
	_, domainError = Decide(aggregate, Command{
		Name:          CommandRemoveParticipant,
		SessionID:     ownerSessionID,
		ParticipantID: "participant-missing",
		OccurredAt:    testJoinedAt.Add(time.Minute),
	})
	assertDomainError(t, domainError, ErrorParticipantMissing)

	aggregate = acceptedDecision(t, aggregate, Command{Name: CommandFinishTable, SessionID: ownerSessionID}).NextState
	if aggregate.State != StateFinished {
		t.Fatalf("state = %s, want %s", aggregate.State, StateFinished)
	}
}

func TestDecideControllerTakeoverFencesPreviousController(t *testing.T) {
	t.Parallel()

	aggregate := testAggregate(t)
	aggregate = acceptedDecision(t, aggregate, Command{Name: CommandTakeSeat, SessionID: "session-owner", Seat: bridge.North}).NextState
	takeover := acceptedDecision(t, aggregate, Command{Name: CommandTakeoverControl, SessionID: "session-owner", ControllerEpoch: 1})
	assignment := takeover.NextState.Seats[bridge.North]
	if assignment.ControllerEpoch != 2 || takeover.Events[0].Type != "CONTROLLER_REPLACED" {
		t.Fatalf("takeover = %+v", takeover)
	}
	_, domainError := Decide(takeover.NextState, Command{Name: CommandSetReady, SessionID: "session-owner", ControllerEpoch: 1, Ready: true})
	assertDomainError(t, domainError, ErrorStaleController)
	acceptedDecision(t, takeover.NextState, Command{Name: CommandSetReady, SessionID: "session-owner", ControllerEpoch: 2, Ready: true})
}

func TestDecideStartPassedOutAndFinish(t *testing.T) {
	t.Parallel()

	aggregate := testReadyAggregate(t)
	deal := testDeal(t)
	aggregate = acceptedDecision(t, aggregate, Command{
		Name:      CommandStartGame,
		SessionID: "session-owner",
		Deal:      &deal,
		BoardID:   "board-one",
	}).NextState
	if aggregate.State != StateActive || aggregate.Game == nil || aggregate.BoardNumber != 1 {
		t.Fatalf("started aggregate = %+v", aggregate)
	}

	sessionsBySeat := map[bridge.Seat]string{}
	for seat, assignment := range aggregate.Seats {
		for _, participant := range aggregate.Participants {
			if participant.ID == assignment.ParticipantID {
				sessionsBySeat[seat] = participant.SessionID
			}
		}
	}
	for aggregate.State == StateActive {
		turn := aggregate.Game.Turn
		call := bridge.Pass()
		aggregate = acceptedDecision(t, aggregate, Command{Name: CommandMakeCall, SessionID: sessionsBySeat[turn], Call: &call}).NextState
	}
	if aggregate.State != StateBetweenBoards || aggregate.Game.Result == nil || !aggregate.Game.Result.PassedOut {
		t.Fatalf("passed-out aggregate = %+v", aggregate)
	}

	aggregate = acceptedDecision(t, aggregate, Command{Name: CommandFinishTable, SessionID: "session-owner"}).NextState
	if aggregate.State != StateFinished {
		t.Fatalf("state = %s, want %s", aggregate.State, StateFinished)
	}
}

func TestDecideClaimConsensus(t *testing.T) {
	t.Parallel()

	aggregate := testStartedAggregate(t)
	for _, call := range []bridge.Call{bridge.Bid(1, bridge.StrainClubs), bridge.Pass(), bridge.Pass(), bridge.Pass()} {
		aggregate = acceptedDecision(t, aggregate, Command{Name: CommandMakeCall, SessionID: sessionForSeat(t, aggregate, aggregate.Game.Turn), Call: &call}).NextState
	}
	aggregate = playTableCards(t, aggregate, 4)
	requester := aggregate.Game.Auction.Contract.Declarer
	claimTricks := 7
	aggregate = acceptedDecision(t, aggregate, Command{Name: CommandRequestClaim, SessionID: sessionForSeat(t, aggregate, requester), ClaimTricks: claimTricks}).NextState
	if aggregate.ActionRequest == nil || aggregate.ActionRequest.Kind != ActionRequestClaim || aggregate.ActionRequest.ClaimTricks != claimTricks {
		t.Fatalf("claim request = %+v", aggregate.ActionRequest)
	}
	legalCards, bridgeError := aggregate.Game.LegalCards(aggregate.Game.Turn)
	if bridgeError != nil {
		t.Fatalf("LegalCards() error = %v", bridgeError)
	}
	_, domainError := Decide(aggregate, Command{Name: CommandPlayCard, SessionID: sessionForSeat(t, aggregate, aggregate.Game.Turn), Card: &legalCards[0]})
	assertDomainError(t, domainError, ErrorActionPending)

	opponents := []bridge.Seat{requester.Next(), requester.Next().Next().Next()}
	aggregate = acceptedDecision(t, aggregate, Command{Name: CommandRespondClaim, SessionID: sessionForSeat(t, aggregate, opponents[0]), Accepted: true}).NextState
	if aggregate.State != StateActive || len(aggregate.ActionRequest.ApprovedBy) != 1 {
		t.Fatalf("first claim response state = %+v", aggregate)
	}
	aggregate = acceptedDecision(t, aggregate, Command{Name: CommandRespondClaim, SessionID: sessionForSeat(t, aggregate, opponents[1]), Accepted: true}).NextState
	if aggregate.State != StateBetweenBoards || aggregate.ActionRequest != nil || aggregate.Game == nil || !aggregate.Game.Claimed {
		t.Fatalf("accepted claim state = %+v", aggregate)
	}
}

func TestDecideClaimRejectionResumesPlay(t *testing.T) {
	t.Parallel()

	aggregate := testStartedAggregate(t)
	for _, call := range []bridge.Call{bridge.Bid(1, bridge.StrainClubs), bridge.Pass(), bridge.Pass(), bridge.Pass()} {
		aggregate = acceptedDecision(t, aggregate, Command{Name: CommandMakeCall, SessionID: sessionForSeat(t, aggregate, aggregate.Game.Turn), Call: &call}).NextState
	}
	aggregate = playTableCards(t, aggregate, 4)
	requester := aggregate.Game.Auction.Contract.Declarer
	aggregate = acceptedDecision(t, aggregate, Command{Name: CommandRequestClaim, SessionID: sessionForSeat(t, aggregate, requester), ClaimTricks: 5}).NextState
	aggregate = acceptedDecision(t, aggregate, Command{Name: CommandRespondClaim, SessionID: sessionForSeat(t, aggregate, requester.Next()), Accepted: false}).NextState
	if aggregate.State != StateActive || aggregate.ActionRequest != nil || aggregate.Game.Claimed {
		t.Fatalf("rejected claim state = %+v", aggregate)
	}
}

func TestDecideUndoConsensus(t *testing.T) {
	t.Parallel()

	aggregate := testStartedAggregate(t)
	before := aggregate.Game.Clone()
	call := bridge.Bid(1, bridge.StrainClubs)
	aggregate = acceptedDecision(t, aggregate, Command{Name: CommandMakeCall, SessionID: sessionForSeat(t, aggregate, bridge.North), Call: &call}).NextState
	aggregate = acceptedDecision(t, aggregate, Command{Name: CommandRequestUndo, SessionID: sessionForSeat(t, aggregate, bridge.North)}).NextState
	for _, seat := range []bridge.Seat{bridge.East, bridge.South, bridge.West} {
		aggregate = acceptedDecision(t, aggregate, Command{Name: CommandRespondUndo, SessionID: sessionForSeat(t, aggregate, seat), Accepted: true}).NextState
	}
	if aggregate.ActionRequest != nil || aggregate.UndoableAction != nil || !reflect.DeepEqual(*aggregate.Game, before) {
		t.Fatalf("accepted undo state = %+v", aggregate)
	}
	_, domainError := Decide(aggregate, Command{Name: CommandRequestUndo, SessionID: sessionForSeat(t, aggregate, bridge.North)})
	assertDomainError(t, domainError, ErrorUndoUnavailable)
}

func TestAggregateConsensusSnapshotRoundTrip(t *testing.T) {
	t.Parallel()

	aggregate := testStartedAggregate(t)
	call := bridge.Bid(1, bridge.StrainClubs)
	aggregate = acceptedDecision(t, aggregate, Command{Name: CommandMakeCall, SessionID: sessionForSeat(t, aggregate, bridge.North), Call: &call}).NextState
	aggregate = acceptedDecision(t, aggregate, Command{Name: CommandRequestUndo, SessionID: sessionForSeat(t, aggregate, bridge.North)}).NextState

	encoded, err := json.Marshal(aggregate)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	var decoded Aggregate
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if err := decoded.Validate(); err != nil {
		t.Fatalf("decoded.Validate() error = %v", err)
	}
	if !reflect.DeepEqual(decoded, aggregate) {
		t.Fatal("consensus snapshot changed during JSON round trip")
	}
}

func TestAggregatePlayHistorySnapshotRoundTrip(t *testing.T) {
	t.Parallel()

	aggregate := testStartedAggregate(t)
	for _, call := range []bridge.Call{bridge.Bid(1, bridge.StrainClubs), bridge.Pass(), bridge.Pass(), bridge.Pass()} {
		aggregate = acceptedDecision(t, aggregate, Command{Name: CommandMakeCall, SessionID: sessionForSeat(t, aggregate, aggregate.Game.Turn), Call: &call}).NextState
	}
	aggregate = playTableCards(t, aggregate, 8)

	encoded, err := json.Marshal(aggregate)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	var decoded Aggregate
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if err := decoded.Validate(); err != nil {
		t.Fatalf("decoded.Validate() error = %v", err)
	}
	if !reflect.DeepEqual(decoded.Game.CompletedTricks, aggregate.Game.CompletedTricks) {
		t.Fatal("completed trick history changed during JSON round trip")
	}
}

func TestDecideRejectsUnauthorizedConsensusResponses(t *testing.T) {
	t.Parallel()

	aggregate := testStartedAggregate(t)
	call := bridge.Bid(1, bridge.StrainClubs)
	aggregate = acceptedDecision(t, aggregate, Command{Name: CommandMakeCall, SessionID: sessionForSeat(t, aggregate, bridge.North), Call: &call}).NextState
	aggregate = acceptedDecision(t, aggregate, Command{Name: CommandRequestUndo, SessionID: sessionForSeat(t, aggregate, bridge.North)}).NextState

	tests := []struct {
		name      string
		sessionID string
	}{
		{name: "requester", sessionID: sessionForSeat(t, aggregate, bridge.North)},
		{name: "duplicate responder", sessionID: sessionForSeat(t, aggregate, bridge.East)},
	}
	_, domainError := Decide(aggregate, Command{Name: CommandRespondUndo, SessionID: tests[0].sessionID, Accepted: true})
	assertDomainError(t, domainError, ErrorResponseNotAllowed)
	aggregate = acceptedDecision(t, aggregate, Command{Name: CommandRespondUndo, SessionID: tests[1].sessionID, Accepted: true}).NextState
	_, domainError = Decide(aggregate, Command{Name: CommandRespondUndo, SessionID: tests[1].sessionID, Accepted: true})
	assertDomainError(t, domainError, ErrorResponseNotAllowed)
}

func TestDecideRejectsMechanicalIrregularitiesWithoutMutation(t *testing.T) {
	t.Parallel()

	auction := testStartedAggregate(t)
	openingPass := bridge.Pass()
	openingDouble := bridge.Double()
	openingBid := bridge.Bid(1, bridge.StrainSpades)
	openingCard := auction.Game.Deal.Hand(bridge.North)[0]
	afterBid := acceptedDecision(t, auction, Command{
		Name: CommandMakeCall, SessionID: sessionForSeat(t, auction, bridge.North), Call: &openingBid,
	}).NextState
	insufficientBid := bridge.Bid(1, bridge.StrainHearts)

	play := testStartedAggregate(t)
	for _, call := range []bridge.Call{
		bridge.Bid(1, bridge.StrainClubs), bridge.Pass(), bridge.Pass(), bridge.Pass(),
	} {
		play = acceptedDecision(t, play, Command{
			Name: CommandMakeCall, SessionID: sessionForSeat(t, play, play.Game.Turn), Call: &call,
		}).NextState
	}
	openingLeader := play.Game.Turn
	wrongHandCard := play.Game.Deal.Hand(bridge.North)[0]
	legalOpeningCards, domainError := play.Game.LegalCards(openingLeader)
	if domainError != nil || len(legalOpeningCards) == 0 {
		t.Fatalf("LegalCards(%s) cards = %d, error = %v", openingLeader, len(legalOpeningCards), domainError)
	}
	legalOpeningCard := legalOpeningCards[0]
	afterLead := acceptedDecision(t, play, Command{
		Name: CommandPlayCard, SessionID: sessionForSeat(t, play, openingLeader), Card: &legalOpeningCard,
	}).NextState
	dummy := afterLead.Game.Auction.Contract.Dummy()
	dummyCard := afterLead.Game.Deal.Hand(dummy)[0]

	revokeState, revokeCard := aggregateWithDummyRevokeAttempt(t)
	tests := []struct {
		name      string
		aggregate Aggregate
		command   Command
		wantCode  ErrorCode
	}{
		{
			name: "call out of turn", aggregate: auction,
			command:  Command{Name: CommandMakeCall, SessionID: sessionForSeat(t, auction, bridge.East), Call: &openingPass},
			wantCode: ErrorCode(bridge.ErrorNotYourTurn),
		},
		{
			name: "double without prior bid", aggregate: auction,
			command:  Command{Name: CommandMakeCall, SessionID: sessionForSeat(t, auction, bridge.North), Call: &openingDouble},
			wantCode: ErrorCode(bridge.ErrorIllegalCall),
		},
		{
			name: "insufficient bid", aggregate: afterBid,
			command:  Command{Name: CommandMakeCall, SessionID: sessionForSeat(t, afterBid, bridge.East), Call: &insufficientBid},
			wantCode: ErrorCode(bridge.ErrorIllegalCall),
		},
		{
			name: "play during auction", aggregate: auction,
			command:  Command{Name: CommandPlayCard, SessionID: sessionForSeat(t, auction, bridge.North), Card: &openingCard},
			wantCode: ErrorCode(bridge.ErrorPlayComplete),
		},
		{
			name: "call with card payload", aggregate: auction,
			command:  Command{Name: CommandMakeCall, SessionID: sessionForSeat(t, auction, bridge.North), Call: &openingPass, Card: &openingCard},
			wantCode: ErrorInvalidCommand,
		},
		{
			name: "opening lead out of turn", aggregate: play,
			command:  Command{Name: CommandPlayCard, SessionID: sessionForSeat(t, play, openingLeader.Next()), Card: &legalOpeningCard},
			wantCode: ErrorCode(bridge.ErrorNotYourTurn),
		},
		{
			name: "card from wrong hand", aggregate: play,
			command:  Command{Name: CommandPlayCard, SessionID: sessionForSeat(t, play, openingLeader), Card: &wrongHandCard},
			wantCode: ErrorCode(bridge.ErrorCardNotHeld),
		},
		{
			name: "dummy acts for own hand", aggregate: afterLead,
			command:  Command{Name: CommandPlayCard, SessionID: sessionForSeat(t, afterLead, dummy), Card: &dummyCard},
			wantCode: ErrorCode(bridge.ErrorDeclarerControlsDummy),
		},
		{
			name: "failure to follow suit", aggregate: revokeState,
			command: Command{
				Name:      CommandPlayCard,
				SessionID: sessionForSeat(t, revokeState, revokeState.Game.Auction.Contract.Declarer),
				Card:      &revokeCard,
			},
			wantCode: ErrorCode(bridge.ErrorMustFollowSuit),
		},
		{
			name: "play with call payload", aggregate: play,
			command:  Command{Name: CommandPlayCard, SessionID: sessionForSeat(t, play, openingLeader), Call: &openingPass, Card: &legalOpeningCard},
			wantCode: ErrorInvalidCommand,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			before := test.aggregate.clone()
			decision, domainError := Decide(test.aggregate, test.command)
			assertDomainError(t, domainError, test.wantCode)
			if !reflect.DeepEqual(decision, Decision{}) {
				t.Fatalf("rejected command returned decision %+v", decision)
			}
			if !reflect.DeepEqual(test.aggregate, before) || test.aggregate.Revision != before.Revision || test.aggregate.LastSeq != before.LastSeq {
				t.Fatal("rejected mechanical irregularity changed aggregate, revision, or sequence")
			}
		})
	}
}

func TestDecideActiveTableParticipantReplacement(t *testing.T) {
	t.Parallel()

	aggregate := testReadyAggregate(t)
	aggregate.Locked = true
	deal := testDeal(t)
	aggregate = acceptedDecision(t, aggregate, Command{
		Name: CommandStartGame, SessionID: "session-owner", Deal: &deal, BoardID: "board-one",
	}).NextState
	east := aggregate.Participants[1]
	removedAt := testJoinedAt.Add(5 * time.Minute)
	aggregate = acceptedDecision(t, aggregate, Command{
		Name: CommandRemoveParticipant, SessionID: "session-owner", ParticipantID: east.ID, OccurredAt: removedAt,
	}).NextState
	if _, occupied := aggregate.Seats[bridge.East]; occupied {
		t.Fatal("removed active participant retained seat")
	}

	replacement := testParticipant("participant-replacement", "session-replacement", "Replacement", RoleParticipant)
	replacement.JoinedAt = removedAt.Add(time.Second)
	aggregate = acceptedDecision(t, aggregate, Command{Name: CommandJoinTable, Participant: &replacement}).NextState
	aggregate = acceptedDecision(t, aggregate, Command{
		Name: CommandTakeSeat, SessionID: replacement.SessionID, Seat: bridge.East,
	}).NextState
	if assignment := aggregate.Seats[bridge.East]; assignment.ParticipantID != replacement.ID || !assignment.Ready {
		t.Fatalf("replacement seat = %+v", assignment)
	}

	pass := bridge.Pass()
	aggregate = acceptedDecision(t, aggregate, Command{Name: CommandMakeCall, SessionID: "session-owner", Call: &pass}).NextState
	acceptedDecision(t, aggregate, Command{Name: CommandMakeCall, SessionID: replacement.SessionID, Call: &pass})
}

func TestDecideOfflineParticipantTimeout(t *testing.T) {
	t.Parallel()

	aggregate := testAggregateWithGuests(t, 1)
	guest := aggregate.Participants[1]
	decision := acceptedDecision(t, aggregate, Command{
		Name: CommandExpireParticipant, SessionID: "session-owner", ParticipantID: guest.ID, OccurredAt: testJoinedAt.Add(10 * time.Minute),
	})
	if decision.Events[0].Type != "PARTICIPANT_TIMED_OUT" {
		t.Fatalf("timeout event = %s", decision.Events[0].Type)
	}
}

func TestDecideSoleOwnerLeaveClosesTable(t *testing.T) {
	t.Parallel()

	aggregate := testAggregate(t)
	owner := aggregate.Participants[0]
	decision := acceptedDecision(t, aggregate, Command{
		Name: CommandLeaveTable, SessionID: owner.SessionID, OccurredAt: testJoinedAt.Add(time.Minute),
	})
	if decision.NextState.State != StateFinished || decision.Events[0].Type != "TABLE_CLOSED" {
		t.Fatalf("owner leave decision = %+v", decision)
	}
	if _, active := decision.NextState.activeParticipant(owner.SessionID); active {
		t.Fatal("sole owner remained active after leaving")
	}
}

func TestDecideParticipantCanLeaveActiveTable(t *testing.T) {
	t.Parallel()

	aggregate := testReadyAggregate(t)
	deal := testDeal(t)
	aggregate = acceptedDecision(t, aggregate, Command{
		Name: CommandStartGame, SessionID: aggregate.OwnerSessionID, Deal: &deal, BoardID: "board-one",
	}).NextState
	guest := aggregate.Participants[1]
	decision := acceptedDecision(t, aggregate, Command{
		Name: CommandLeaveTable, SessionID: guest.SessionID, OccurredAt: testJoinedAt.Add(time.Minute),
	})
	if decision.NextState.State != StateActive {
		t.Fatalf("state after active leave = %s, want %s", decision.NextState.State, StateActive)
	}
	if _, seated := decision.NextState.seatForParticipant(guest.ID); seated {
		t.Fatal("active participant retained seat after leaving")
	}
}

func TestDecideTableExpiryClosesWithoutRemovingParticipants(t *testing.T) {
	t.Parallel()

	aggregate := testAggregateWithGuests(t, 1)
	decision := acceptedDecision(t, aggregate, Command{
		Name: CommandExpireTable, SessionID: aggregate.OwnerSessionID, OccurredAt: testJoinedAt.Add(5 * time.Minute),
	})
	if decision.NextState.State != StateFinished || decision.Events[0].Type != "TABLE_EXPIRED" {
		t.Fatalf("expiry decision = %+v", decision)
	}
	if decision.NextState.activeParticipantCount() != 2 {
		t.Fatalf("expiry active participants = %d, want historical membership preserved", decision.NextState.activeParticipantCount())
	}
}

func TestDecideOwnerTimeoutTransfersOwnership(t *testing.T) {
	t.Parallel()

	aggregate := testAggregateWithGuests(t, 2)
	owner := aggregate.Participants[0]
	replacement := aggregate.Participants[2]
	aggregate = acceptedDecision(t, aggregate, Command{Name: CommandTakeSeat, SessionID: replacement.SessionID, Seat: bridge.South}).NextState
	decision := acceptedDecision(t, aggregate, Command{
		Name: CommandExpireParticipant, SessionID: owner.SessionID, ParticipantID: owner.ID,
		ReplacementParticipantID: replacement.ID, OccurredAt: testJoinedAt.Add(time.Minute),
	})
	if decision.NextState.OwnerSessionID != replacement.SessionID || decision.NextState.Participants[2].Role != RoleOwner {
		t.Fatalf("replacement owner = %+v", decision.NextState.Participants[2])
	}
	if _, active := decision.NextState.activeParticipant(owner.SessionID); active {
		t.Fatal("expired owner remained active")
	}
	payload, ok := decision.Events[0].Payload.(map[string]any)
	if !ok || payload["ownerParticipantId"] != replacement.ID {
		t.Fatalf("timeout payload = %#v", decision.Events[0].Payload)
	}
}

func TestDecideOwnerRemovalRequiresReplacement(t *testing.T) {
	t.Parallel()

	aggregate := testAggregate(t)
	owner := aggregate.Participants[0]
	_, domainError := Decide(aggregate, Command{
		Name: CommandRemoveParticipant, SessionID: owner.SessionID, ParticipantID: owner.ID, OccurredAt: testJoinedAt.Add(time.Minute),
	})
	assertDomainError(t, domainError, ErrorOwnerCannotLeave)
}

func TestDecideStartRequiresFourReadySeats(t *testing.T) {
	t.Parallel()

	deal := testDeal(t)
	tests := []struct {
		name      string
		aggregate Aggregate
	}{
		{name: "missing seats", aggregate: testAggregate(t)},
		{name: "one unready", aggregate: func() Aggregate {
			aggregate := testReadyAggregate(t)
			assignment := aggregate.Seats[bridge.West]
			assignment.Ready = false
			aggregate.Seats[bridge.West] = assignment
			return aggregate
		}()},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			before := test.aggregate.clone()
			_, domainError := Decide(test.aggregate, Command{Name: CommandStartGame, SessionID: "session-owner", Deal: &deal, BoardID: "board-one"})
			assertDomainError(t, domainError, ErrorNotReady)
			if !reflect.DeepEqual(test.aggregate, before) {
				t.Fatal("rejected start mutated aggregate")
			}
		})
	}
}

func TestAggregateValidateRejectsInvalidRelationships(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(Aggregate) Aggregate
	}{
		{name: "inactive seated participant", mutate: func(aggregate Aggregate) Aggregate {
			guest := testParticipant("participant-east", "session-east", "East", RoleParticipant)
			leftAt := guest.JoinedAt.Add(time.Minute)
			guest.LeftAt = &leftAt
			aggregate.Participants = append(aggregate.Participants, guest)
			aggregate.Seats[bridge.East] = SeatAssignment{ParticipantID: guest.ID, ControllerEpoch: 1}
			return aggregate
		}},
		{name: "participant left before join", mutate: func(aggregate Aggregate) Aggregate {
			guest := testParticipant("participant-east", "session-east", "East", RoleParticipant)
			leftAt := guest.JoinedAt.Add(-time.Minute)
			guest.LeftAt = &leftAt
			aggregate.Participants = append(aggregate.Participants, guest)
			return aggregate
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if err := test.mutate(testAggregate(t)).Validate(); err == nil {
				t.Fatal("Validate() error = nil")
			}
		})
	}
}

func testAggregate(t *testing.T) Aggregate {
	t.Helper()
	aggregate, err := NewAggregate("table-one", testParticipant("participant-owner", "session-owner", "Owner", RoleOwner))
	if err != nil {
		t.Fatalf("NewAggregate() error = %v", err)
	}
	return aggregate
}

func testAggregateWithGuests(t *testing.T, guestCount int) Aggregate {
	t.Helper()
	aggregate := testAggregate(t)
	for _index := 0; _index < guestCount; _index++ {
		guest := testParticipant(
			"participant-"+string(rune('a'+_index)),
			"session-"+string(rune('a'+_index)),
			"Guest "+string(rune('A'+_index)),
			RoleParticipant,
		)
		aggregate = acceptedDecision(t, aggregate, Command{Name: CommandJoinTable, Participant: &guest}).NextState
	}
	return aggregate
}

func testReadyAggregate(t *testing.T) Aggregate {
	t.Helper()
	aggregate := testAggregateWithGuests(t, 3)
	seats := []bridge.Seat{bridge.North, bridge.East, bridge.South, bridge.West}
	for _index, participant := range aggregate.Participants {
		aggregate = acceptedDecision(t, aggregate, Command{Name: CommandTakeSeat, SessionID: participant.SessionID, Seat: seats[_index]}).NextState
		aggregate = acceptedDecision(t, aggregate, Command{Name: CommandSetReady, SessionID: participant.SessionID, Ready: true}).NextState
	}
	return aggregate
}

func testParticipant(id string, sessionID string, nickname string, role Role) Participant {
	return Participant{ID: id, SessionID: sessionID, Nickname: nickname, Role: role, JoinedAt: testJoinedAt}
}

func testDeal(t *testing.T) bridge.Deal {
	t.Helper()
	deal, err := bridge.GenerateDeal(bytes.NewReader(bytes.Repeat([]byte{0xff}, 1024)))
	if err != nil {
		t.Fatalf("bridge.GenerateDeal() error = %v", err)
	}
	return deal
}

func aggregateWithDummyRevokeAttempt(t *testing.T) (Aggregate, bridge.Card) {
	t.Helper()
	aggregate := testStartedAggregate(t)
	for _, call := range []bridge.Call{
		bridge.Bid(1, bridge.StrainClubs), bridge.Pass(), bridge.Pass(), bridge.Pass(),
	} {
		aggregate = acceptedDecision(t, aggregate, Command{
			Name: CommandMakeCall, SessionID: sessionForSeat(t, aggregate, aggregate.Game.Turn), Call: &call,
		}).NextState
	}
	dummy := aggregate.Game.Auction.Contract.Dummy()
	for _, openingCard := range aggregate.Game.Deal.Hand(aggregate.Game.Turn) {
		hasLedSuit := false
		var offSuitCard bridge.Card
		for _, card := range aggregate.Game.Deal.Hand(dummy) {
			if card.Suit == openingCard.Suit {
				hasLedSuit = true
			} else {
				offSuitCard = card
			}
		}
		if !hasLedSuit || !offSuitCard.Suit.Valid() {
			continue
		}
		aggregate = acceptedDecision(t, aggregate, Command{
			Name: CommandPlayCard, SessionID: sessionForSeat(t, aggregate, aggregate.Game.Turn), Card: &openingCard,
		}).NextState
		return aggregate, offSuitCard
	}
	t.Fatal("test deal has no dummy revoke scenario")
	return Aggregate{}, bridge.Card{}
}

func playTableCards(t *testing.T, aggregate Aggregate, count int) Aggregate {
	t.Helper()
	for _index := 0; _index < count; _index++ {
		actor := aggregate.Game.Turn
		if actor == aggregate.Game.Auction.Contract.Dummy() {
			actor = aggregate.Game.Auction.Contract.Declarer
		}
		legalCards, domainError := aggregate.Game.LegalCards(actor)
		if domainError != nil || len(legalCards) == 0 {
			t.Fatalf("LegalCards(%s) cards = %d, error = %v", actor, len(legalCards), domainError)
		}
		aggregate = acceptedDecision(t, aggregate, Command{Name: CommandPlayCard, SessionID: sessionForSeat(t, aggregate, actor), Card: &legalCards[0]}).NextState
	}
	return aggregate
}

func acceptedDecision(t *testing.T, aggregate Aggregate, command Command) Decision {
	t.Helper()
	decision, domainError := Decide(aggregate, command)
	if domainError != nil {
		t.Fatalf("Decide(%s) error = %v", command.Name, domainError)
	}
	return decision
}

func assertDomainError(t *testing.T, domainError *DomainError, wantCode ErrorCode) {
	t.Helper()
	if domainError == nil || domainError.Code != wantCode {
		t.Fatalf("domain error = %+v, want code %s", domainError, wantCode)
	}
}
