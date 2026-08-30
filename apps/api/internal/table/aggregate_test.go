package table

import (
	"bytes"
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

	_, domainError = Decide(aggregate, Command{Name: CommandLeaveTable, SessionID: "session-owner", OccurredAt: removedAt})
	assertDomainError(t, domainError, ErrorOwnerCannotLeave)
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
