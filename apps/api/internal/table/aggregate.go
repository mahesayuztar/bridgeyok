package table

import (
	"fmt"
	"slices"
	"time"

	"github.com/mahesayuztar/bridgeyok/apps/api/internal/bridge"
)

// State identifies the durable table lifecycle.
type State string

const (
	StateWaiting       State = "WAITING"
	StateActive        State = "ACTIVE"
	StateBetweenBoards State = "BETWEEN_BOARDS"
	StateFinished      State = "FINISHED"
	StatePaused        State = "PAUSED"
)

// Role identifies table-level participant permissions.
type Role string

const (
	RoleOwner       Role = "OWNER"
	RoleParticipant Role = "PARTICIPANT"
)

// CommandName identifies a table or game mutation.
type CommandName string

const (
	CommandJoinTable         CommandName = "JOIN_TABLE"
	CommandLeaveTable        CommandName = "LEAVE_TABLE"
	CommandTakeSeat          CommandName = "TAKE_SEAT"
	CommandLeaveSeat         CommandName = "LEAVE_SEAT"
	CommandSetReady          CommandName = "SET_READY"
	CommandLockTable         CommandName = "LOCK_TABLE"
	CommandRemoveParticipant CommandName = "REMOVE_PARTICIPANT"
	CommandExpireParticipant CommandName = "EXPIRE_PARTICIPANT"
	CommandStartGame         CommandName = "START_GAME"
	CommandMakeCall          CommandName = "MAKE_CALL"
	CommandPlayCard          CommandName = "PLAY_CARD"
	CommandRequestClaim      CommandName = "REQUEST_CLAIM"
	CommandRespondClaim      CommandName = "RESPOND_CLAIM"
	CommandRequestUndo       CommandName = "REQUEST_UNDO"
	CommandRespondUndo       CommandName = "RESPOND_UNDO"
	CommandRequestNextBoard  CommandName = "REQUEST_NEXT_BOARD"
	CommandFinishTable       CommandName = "FINISH_TABLE"
	CommandTakeoverControl   CommandName = "TAKEOVER_CONTROL"
)

// ErrorCode is a stable table rejection reason.
type ErrorCode string

const (
	ErrorInvalidState       ErrorCode = "INVALID_STATE"
	ErrorStateChanged       ErrorCode = "STATE_CHANGED"
	ErrorNotParticipant     ErrorCode = "NOT_PARTICIPANT"
	ErrorAlreadyParticipant ErrorCode = "ALREADY_PARTICIPANT"
	ErrorTableLocked        ErrorCode = "TABLE_LOCKED"
	ErrorTableFull          ErrorCode = "TABLE_FULL"
	ErrorOwnerRequired      ErrorCode = "OWNER_REQUIRED"
	ErrorOwnerCannotLeave   ErrorCode = "OWNER_CANNOT_LEAVE"
	ErrorSeatTaken          ErrorCode = "SEAT_TAKEN"
	ErrorAlreadySeated      ErrorCode = "ALREADY_SEATED"
	ErrorSeatRequired       ErrorCode = "SEAT_REQUIRED"
	ErrorNotReady           ErrorCode = "TABLE_NOT_READY"
	ErrorParticipantMissing ErrorCode = "PARTICIPANT_MISSING"
	ErrorInvalidCommand     ErrorCode = "INVALID_COMMAND"
	ErrorStaleController    ErrorCode = "STALE_CONTROLLER"
	ErrorActionPending      ErrorCode = "ACTION_PENDING"
	ErrorRequestMissing     ErrorCode = "REQUEST_MISSING"
	ErrorResponseNotAllowed ErrorCode = "RESPONSE_NOT_ALLOWED"
	ErrorUndoUnavailable    ErrorCode = "UNDO_UNAVAILABLE"
)

// ActionRequestKind identifies a consensus request that pauses gameplay.
type ActionRequestKind string

const (
	ActionRequestClaim ActionRequestKind = "CLAIM"
	ActionRequestUndo  ActionRequestKind = "UNDO"
)

// ActionRequest records the public state of one in-progress claim or undo vote.
type ActionRequest struct {
	Kind          ActionRequestKind `json:"kind"`
	RequesterSeat bridge.Seat       `json:"requesterSeat"`
	ClaimTricks   int               `json:"claimTricks,omitempty"`
	ApprovedBy    []bridge.Seat     `json:"approvedBy"`
}

type undoableAction struct {
	ActorSeat bridge.Seat  `json:"actorSeat"`
	Game      bridge.State `json:"game"`
}

// DomainError describes a rejected table command.
type DomainError struct {
	Code    ErrorCode
	Message string
}

func (domainError *DomainError) Error() string {
	if domainError == nil {
		return ""
	}
	return domainError.Message
}

// Participant is one guest joined to a table.
type Participant struct {
	ID        string     `json:"id"`
	SessionID string     `json:"sessionId"`
	Nickname  string     `json:"nickname"`
	Role      Role       `json:"role"`
	JoinedAt  time.Time  `json:"joinedAt"`
	LeftAt    *time.Time `json:"leftAt,omitempty"`
}

// SeatAssignment binds one participant to a bridge seat and controller epoch.
type SeatAssignment struct {
	ParticipantID   string `json:"participantId"`
	Ready           bool   `json:"ready"`
	ControllerEpoch int64  `json:"controllerEpoch"`
}

// Aggregate is the authoritative private state of one table.
type Aggregate struct {
	SchemaVersion  int                            `json:"schemaVersion"`
	ID             string                         `json:"id"`
	OwnerSessionID string                         `json:"ownerSessionId"`
	State          State                          `json:"state"`
	Locked         bool                           `json:"locked"`
	Revision       int64                          `json:"revision"`
	LastSeq        int64                          `json:"lastSeq"`
	BoardID        string                         `json:"boardId,omitempty"`
	BoardNumber    int                            `json:"boardNumber"`
	Participants   []Participant                  `json:"participants"`
	Seats          map[bridge.Seat]SeatAssignment `json:"seats"`
	Game           *bridge.State                  `json:"game,omitempty"`
	ActionRequest  *ActionRequest                 `json:"actionRequest,omitempty"`
	UndoableAction *undoableAction                `json:"undoableAction,omitempty"`
}

// Command contains one authenticated table mutation.
type Command struct {
	Name                     CommandName
	SessionID                string
	Participant              *Participant
	OccurredAt               time.Time
	Seat                     bridge.Seat
	Ready                    bool
	Locked                   bool
	ParticipantID            string
	Call                     *bridge.Call
	Card                     *bridge.Card
	ClaimTricks              int
	Accepted                 bool
	Deal                     *bridge.Deal
	BoardID                  string
	ControllerEpoch          int64
	ReplacementParticipantID string
}

// Event is a durable aggregate fact with a privacy-safe typed payload.
type Event struct {
	Type    string `json:"type"`
	Payload any    `json:"payload"`
}

// Decision is the immutable result of one accepted table command.
type Decision struct {
	NextState Aggregate
	Events    []Event
}

// NewAggregate constructs a waiting table with its owner joined.
func NewAggregate(tableID string, owner Participant) (Aggregate, error) {
	if tableID == "" || owner.ID == "" || owner.SessionID == "" || owner.Role != RoleOwner || owner.LeftAt != nil {
		return Aggregate{}, fmt.Errorf("invalid table owner")
	}
	aggregate := Aggregate{
		SchemaVersion:  1,
		ID:             tableID,
		OwnerSessionID: owner.SessionID,
		State:          StateWaiting,
		Participants:   []Participant{owner},
		Seats:          map[bridge.Seat]SeatAssignment{},
	}
	if err := aggregate.Validate(); err != nil {
		return Aggregate{}, err
	}
	return aggregate, nil
}

// Decide validates and applies one table command without external I/O.
func Decide(aggregate Aggregate, command Command) (Decision, *DomainError) {
	if err := aggregate.Validate(); err != nil {
		return Decision{}, reject(ErrorInvalidState, err.Error())
	}
	if command.Name == CommandJoinTable {
		return decideJoin(aggregate, command)
	}
	participant, exists := aggregate.activeParticipant(command.SessionID)
	if !exists {
		return Decision{}, reject(ErrorNotParticipant, "session is not an active participant")
	}
	if command.ControllerEpoch > 0 {
		seat, seated := aggregate.seatForParticipant(participant.ID)
		if !seated || aggregate.Seats[seat].ControllerEpoch != command.ControllerEpoch {
			return Decision{}, reject(ErrorStaleController, "seat controller has been replaced")
		}
	}

	next := aggregate.clone()
	var events []Event
	switch command.Name {
	case CommandLeaveTable:
		if next.State != StateWaiting {
			return Decision{}, reject(ErrorInvalidState, "leaving the table requires a waiting table")
		}
		if participant.Role == RoleOwner {
			return Decision{}, reject(ErrorOwnerCannotLeave, "owner must finish the table instead")
		}
		if command.OccurredAt.IsZero() {
			return Decision{}, reject(ErrorInvalidCommand, "leave time is required")
		}
		for _index := range next.Participants {
			if next.Participants[_index].ID == participant.ID {
				leftAt := command.OccurredAt.UTC()
				next.Participants[_index].LeftAt = &leftAt
				break
			}
		}
		if seat, seated := next.seatForParticipant(participant.ID); seated {
			delete(next.Seats, seat)
		}
		events = []Event{{Type: "PARTICIPANT_LEFT", Payload: map[string]any{"participantId": participant.ID}}}
	case CommandTakeSeat:
		if next.State != StateWaiting && next.State != StateActive && next.State != StateBetweenBoards {
			return Decision{}, reject(ErrorInvalidState, "seat changes require an open table")
		}
		if !command.Seat.Valid() {
			return Decision{}, reject(ErrorInvalidCommand, "seat is invalid")
		}
		if assignment, occupied := next.Seats[command.Seat]; occupied && assignment.ParticipantID != participant.ID {
			return Decision{}, reject(ErrorSeatTaken, "seat is already occupied")
		}
		if currentSeat, seated := next.seatForParticipant(participant.ID); seated {
			if currentSeat == command.Seat || next.State != StateWaiting {
				return Decision{}, reject(ErrorAlreadySeated, "participant already occupies a seat")
			}
		}
		for seat, assignment := range next.Seats {
			if assignment.ParticipantID == participant.ID && seat != command.Seat {
				delete(next.Seats, seat)
			}
		}
		epoch := int64(1)
		if current, occupied := next.Seats[command.Seat]; occupied {
			epoch = current.ControllerEpoch
		}
		next.Seats[command.Seat] = SeatAssignment{ParticipantID: participant.ID, Ready: next.State != StateWaiting, ControllerEpoch: epoch}
		events = []Event{{Type: "SEAT_CHANGED", Payload: map[string]any{"participantId": participant.ID, "seat": command.Seat}}}
	case CommandLeaveSeat:
		if next.State != StateWaiting {
			return Decision{}, reject(ErrorInvalidState, "leaving a seat requires a waiting table")
		}
		seat, seated := next.seatForParticipant(participant.ID)
		if !seated {
			return Decision{}, reject(ErrorSeatRequired, "participant is not seated")
		}
		delete(next.Seats, seat)
		events = []Event{{Type: "SEAT_CHANGED", Payload: map[string]any{"participantId": participant.ID, "seat": nil}}}
	case CommandSetReady:
		if next.State != StateWaiting {
			return Decision{}, reject(ErrorInvalidState, "ready changes require a waiting table")
		}
		seat, seated := next.seatForParticipant(participant.ID)
		if !seated {
			return Decision{}, reject(ErrorSeatRequired, "participant is not seated")
		}
		assignment := next.Seats[seat]
		assignment.Ready = command.Ready
		next.Seats[seat] = assignment
		events = []Event{{Type: "READY_CHANGED", Payload: map[string]any{"participantId": participant.ID, "ready": command.Ready}}}
	case CommandLockTable:
		if participant.Role != RoleOwner {
			return Decision{}, reject(ErrorOwnerRequired, "only the owner can lock the table")
		}
		if next.State != StateWaiting {
			return Decision{}, reject(ErrorInvalidState, "lock changes require a waiting table")
		}
		next.Locked = command.Locked
		events = []Event{{Type: "TABLE_LOCKED", Payload: map[string]any{"locked": command.Locked}}}
	case CommandRemoveParticipant, CommandExpireParticipant:
		if participant.Role != RoleOwner {
			return Decision{}, reject(ErrorOwnerRequired, "only the owner can remove a participant")
		}
		if next.State == StateFinished {
			return Decision{}, reject(ErrorInvalidState, "finished table participants cannot be removed")
		}
		targetIndex := slices.IndexFunc(next.Participants, func(candidate Participant) bool {
			return candidate.ID == command.ParticipantID && candidate.LeftAt == nil
		})
		if targetIndex < 0 {
			return Decision{}, reject(ErrorParticipantMissing, "participant cannot be removed")
		}
		if command.OccurredAt.IsZero() {
			return Decision{}, reject(ErrorInvalidCommand, "removal time is required")
		}
		replacementParticipantID := ""
		if next.Participants[targetIndex].Role == RoleOwner {
			replacementIndex := next.replacementOwnerIndex(command.ParticipantID, command.ReplacementParticipantID)
			if replacementIndex < 0 {
				return Decision{}, reject(ErrorOwnerCannotLeave, "owner cannot leave without a replacement")
			}
			next.Participants[replacementIndex].Role = RoleOwner
			next.OwnerSessionID = next.Participants[replacementIndex].SessionID
			replacementParticipantID = next.Participants[replacementIndex].ID
		}
		leftAt := command.OccurredAt.UTC()
		next.Participants[targetIndex].LeftAt = &leftAt
		if seat, seated := next.seatForParticipant(command.ParticipantID); seated {
			delete(next.Seats, seat)
		}
		next.ActionRequest = nil
		eventType := "PARTICIPANT_REMOVED"
		if command.Name == CommandExpireParticipant {
			eventType = "PARTICIPANT_TIMED_OUT"
		}
		payload := map[string]any{"participantId": command.ParticipantID}
		if replacementParticipantID != "" {
			payload["ownerParticipantId"] = replacementParticipantID
		}
		events = []Event{{Type: eventType, Payload: payload}}
	case CommandStartGame:
		if participant.Role != RoleOwner {
			return Decision{}, reject(ErrorOwnerRequired, "only the owner can start a board")
		}
		if next.State != StateWaiting || len(next.Seats) != 4 {
			return Decision{}, reject(ErrorNotReady, "four seats must be occupied")
		}
		for _, seat := range []bridge.Seat{bridge.North, bridge.East, bridge.South, bridge.West} {
			if !next.Seats[seat].Ready {
				return Decision{}, reject(ErrorNotReady, "every seat must be ready")
			}
		}
		if command.Deal == nil || command.BoardID == "" {
			return Decision{}, reject(ErrorInvalidCommand, "start requires a deal and board id")
		}
		game, err := bridge.NewBoard(1, *command.Deal)
		if err != nil {
			return Decision{}, reject(ErrorInvalidCommand, err.Error())
		}
		next.State = StateActive
		next.BoardNumber = 1
		next.BoardID = command.BoardID
		next.Game = &game
		next.ActionRequest = nil
		next.UndoableAction = nil
		events = []Event{{Type: "BOARD_STARTED", Payload: map[string]any{"boardId": command.BoardID, "boardNumber": 1}}}
	case CommandMakeCall, CommandPlayCard:
		if next.State != StateActive || next.Game == nil {
			return Decision{}, reject(ErrorInvalidState, "table has no active board")
		}
		if next.ActionRequest != nil {
			return Decision{}, reject(ErrorActionPending, "claim or undo response is pending")
		}
		seat, seated := next.seatForParticipant(participant.ID)
		if !seated {
			return Decision{}, reject(ErrorSeatRequired, "participant is not seated")
		}
		var gameCommand bridge.Command
		if command.Name == CommandMakeCall && command.Call != nil && command.Card == nil {
			gameCommand = bridge.MakeCallCommand(seat, *command.Call)
		} else if command.Name == CommandPlayCard && command.Card != nil && command.Call == nil {
			gameCommand = bridge.PlayCardCommand(seat, *command.Card)
		} else {
			return Decision{}, reject(ErrorInvalidCommand, "game command payload is invalid")
		}
		previousGame := next.Game.Clone()
		gameDecision, domainError := bridge.Decide(*next.Game, gameCommand)
		if domainError != nil {
			return Decision{}, reject(ErrorCode(domainError.Code), domainError.Error())
		}
		next.Game = &gameDecision.NextState
		next.UndoableAction = &undoableAction{ActorSeat: seat, Game: previousGame}
		for _, gameEvent := range gameDecision.Events {
			events = append(events, Event{Type: string(gameEvent.Type), Payload: gameEvent})
		}
		if gameDecision.NextState.Phase == bridge.PhaseBoardScored {
			next.State = StateBetweenBoards
		}
	case CommandRequestClaim:
		if next.State != StateActive || next.Game == nil {
			return Decision{}, reject(ErrorInvalidState, "claim requires an active board")
		}
		if next.ActionRequest != nil {
			return Decision{}, reject(ErrorActionPending, "another claim or undo request is pending")
		}
		seat, seated := next.seatForParticipant(participant.ID)
		if !seated {
			return Decision{}, reject(ErrorSeatRequired, "participant is not seated")
		}
		if _, domainError := bridge.Decide(*next.Game, bridge.ClaimCommand(seat, command.ClaimTricks)); domainError != nil {
			return Decision{}, reject(ErrorCode(domainError.Code), domainError.Error())
		}
		next.ActionRequest = &ActionRequest{Kind: ActionRequestClaim, RequesterSeat: seat, ClaimTricks: command.ClaimTricks, ApprovedBy: []bridge.Seat{}}
		events = []Event{{Type: "CLAIM_REQUESTED", Payload: map[string]any{"requesterSeat": seat, "claimTricks": command.ClaimTricks}}}
	case CommandRespondClaim:
		if next.State != StateActive || next.Game == nil || next.ActionRequest == nil || next.ActionRequest.Kind != ActionRequestClaim {
			return Decision{}, reject(ErrorRequestMissing, "there is no claim awaiting a response")
		}
		seat, seated := next.seatForParticipant(participant.ID)
		if !seated || seat.Partnership() == next.ActionRequest.RequesterSeat.Partnership() || slices.Contains(next.ActionRequest.ApprovedBy, seat) {
			return Decision{}, reject(ErrorResponseNotAllowed, "seat cannot respond to this claim")
		}
		if !command.Accepted {
			next.ActionRequest = nil
			events = []Event{{Type: "CLAIM_REJECTED", Payload: map[string]any{"responderSeat": seat}}}
			break
		}
		next.ActionRequest.ApprovedBy = append(next.ActionRequest.ApprovedBy, seat)
		if len(next.ActionRequest.ApprovedBy) < 2 {
			events = []Event{{Type: "CLAIM_RESPONSE_RECORDED", Payload: map[string]any{"responderSeat": seat, "accepted": true}}}
			break
		}
		gameDecision, domainError := bridge.Decide(*next.Game, bridge.ClaimCommand(next.ActionRequest.RequesterSeat, next.ActionRequest.ClaimTricks))
		if domainError != nil {
			return Decision{}, reject(ErrorCode(domainError.Code), domainError.Error())
		}
		next.Game = &gameDecision.NextState
		next.State = StateBetweenBoards
		next.ActionRequest = nil
		next.UndoableAction = nil
		events = []Event{{Type: "CLAIM_ACCEPTED", Payload: gameDecision.Events[0]}}
	case CommandRequestUndo:
		if (next.State != StateActive && next.State != StateBetweenBoards) || next.Game == nil {
			return Decision{}, reject(ErrorInvalidState, "undo requires a current board")
		}
		if next.ActionRequest != nil {
			return Decision{}, reject(ErrorActionPending, "another claim or undo request is pending")
		}
		seat, seated := next.seatForParticipant(participant.ID)
		if !seated || next.UndoableAction == nil || next.UndoableAction.ActorSeat != seat {
			return Decision{}, reject(ErrorUndoUnavailable, "only the actor of the latest game action can request undo")
		}
		next.ActionRequest = &ActionRequest{Kind: ActionRequestUndo, RequesterSeat: seat, ApprovedBy: []bridge.Seat{}}
		events = []Event{{Type: "UNDO_REQUESTED", Payload: map[string]any{"requesterSeat": seat}}}
	case CommandRespondUndo:
		if (next.State != StateActive && next.State != StateBetweenBoards) || next.Game == nil || next.ActionRequest == nil || next.ActionRequest.Kind != ActionRequestUndo || next.UndoableAction == nil {
			return Decision{}, reject(ErrorRequestMissing, "there is no undo awaiting a response")
		}
		seat, seated := next.seatForParticipant(participant.ID)
		if !seated || seat == next.ActionRequest.RequesterSeat || slices.Contains(next.ActionRequest.ApprovedBy, seat) {
			return Decision{}, reject(ErrorResponseNotAllowed, "seat cannot respond to this undo")
		}
		if !command.Accepted {
			next.ActionRequest = nil
			events = []Event{{Type: "UNDO_REJECTED", Payload: map[string]any{"responderSeat": seat}}}
			break
		}
		next.ActionRequest.ApprovedBy = append(next.ActionRequest.ApprovedBy, seat)
		if len(next.ActionRequest.ApprovedBy) < 3 {
			events = []Event{{Type: "UNDO_RESPONSE_RECORDED", Payload: map[string]any{"responderSeat": seat, "accepted": true}}}
			break
		}
		restoredGame := next.UndoableAction.Game.Clone()
		next.Game = &restoredGame
		next.State = StateActive
		next.ActionRequest = nil
		next.UndoableAction = nil
		events = []Event{{Type: "UNDO_ACCEPTED", Payload: map[string]any{}}}
	case CommandRequestNextBoard:
		if participant.Role != RoleOwner {
			return Decision{}, reject(ErrorOwnerRequired, "only the owner can start the next board")
		}
		if next.State != StateBetweenBoards || command.Deal == nil || command.BoardID == "" || next.ActionRequest != nil {
			return Decision{}, reject(ErrorInvalidState, "next board requires a completed board and new deal")
		}
		game, err := bridge.NewBoard(next.BoardNumber+1, *command.Deal)
		if err != nil {
			return Decision{}, reject(ErrorInvalidCommand, err.Error())
		}
		next.State = StateActive
		next.BoardNumber++
		next.BoardID = command.BoardID
		next.Game = &game
		next.ActionRequest = nil
		next.UndoableAction = nil
		events = []Event{{Type: "BOARD_STARTED", Payload: map[string]any{"boardId": command.BoardID, "boardNumber": next.BoardNumber}}}
	case CommandFinishTable:
		if participant.Role != RoleOwner {
			return Decision{}, reject(ErrorOwnerRequired, "only the owner can finish the table")
		}
		if next.State != StateWaiting && next.State != StateBetweenBoards || next.ActionRequest != nil {
			return Decision{}, reject(ErrorInvalidState, "table cannot finish during an active board")
		}
		next.State = StateFinished
		next.ActionRequest = nil
		next.UndoableAction = nil
		events = []Event{{Type: "TABLE_FINISHED", Payload: map[string]any{}}}
	case CommandTakeoverControl:
		if next.State == StateFinished {
			return Decision{}, reject(ErrorInvalidState, "finished table controller cannot be replaced")
		}
		if command.ControllerEpoch < 1 {
			return Decision{}, reject(ErrorInvalidCommand, "controller epoch is required")
		}
		seat, seated := next.seatForParticipant(participant.ID)
		if !seated {
			return Decision{}, reject(ErrorSeatRequired, "participant is not seated")
		}
		assignment := next.Seats[seat]
		assignment.ControllerEpoch++
		next.Seats[seat] = assignment
		events = []Event{{Type: "CONTROLLER_REPLACED", Payload: map[string]any{"participantId": participant.ID, "controllerEpoch": assignment.ControllerEpoch}}}
	default:
		return Decision{}, reject(ErrorInvalidCommand, "unknown table command")
	}
	if err := next.Validate(); err != nil {
		return Decision{}, reject(ErrorInvalidState, err.Error())
	}
	return Decision{NextState: next, Events: events}, nil
}

func decideJoin(aggregate Aggregate, command Command) (Decision, *DomainError) {
	if aggregate.State != StateWaiting && aggregate.State != StateActive && aggregate.State != StateBetweenBoards {
		return Decision{}, reject(ErrorInvalidState, "joining requires an open table")
	}
	if aggregate.State == StateWaiting && aggregate.Locked {
		return Decision{}, reject(ErrorTableLocked, "table is locked")
	}
	if command.Participant == nil {
		return Decision{}, reject(ErrorInvalidCommand, "participant is required")
	}
	participant := *command.Participant
	if participant.ID == "" || participant.SessionID == "" || participant.Nickname == "" || participant.Role != RoleParticipant || participant.JoinedAt.IsZero() || participant.LeftAt != nil {
		return Decision{}, reject(ErrorInvalidCommand, "participant is invalid")
	}
	if _, exists := aggregate.activeParticipant(participant.SessionID); exists {
		return Decision{}, reject(ErrorAlreadyParticipant, "session already joined the table")
	}
	if aggregate.activeParticipantCount() >= 4 {
		return Decision{}, reject(ErrorTableFull, "table already has four participants")
	}

	next := aggregate.clone()
	participant.JoinedAt = participant.JoinedAt.UTC()
	next.Participants = append(next.Participants, participant)
	if err := next.Validate(); err != nil {
		return Decision{}, reject(ErrorInvalidState, err.Error())
	}
	return Decision{
		NextState: next,
		Events:    []Event{{Type: "PARTICIPANT_JOINED", Payload: map[string]any{"participantId": participant.ID}}},
	}, nil
}

// Validate enforces aggregate structural and private game invariants.
func (aggregate Aggregate) Validate() error {
	if aggregate.SchemaVersion != 1 || aggregate.ID == "" || aggregate.OwnerSessionID == "" || aggregate.Revision < 0 || aggregate.LastSeq < 0 {
		return fmt.Errorf("invalid aggregate identity or version")
	}
	if aggregate.State != StateWaiting && aggregate.State != StateActive && aggregate.State != StateBetweenBoards && aggregate.State != StateFinished && aggregate.State != StatePaused {
		return fmt.Errorf("invalid table state %q", aggregate.State)
	}
	participantIDs := make(map[string]struct{}, len(aggregate.Participants))
	activeParticipantIDs := make(map[string]struct{}, len(aggregate.Participants))
	activeSessions := make(map[string]struct{}, len(aggregate.Participants))
	ownerCount := 0
	for _, participant := range aggregate.Participants {
		if participant.ID == "" || participant.SessionID == "" || participant.Nickname == "" || participant.JoinedAt.IsZero() || participant.Role != RoleOwner && participant.Role != RoleParticipant {
			return fmt.Errorf("invalid participant")
		}
		if participant.LeftAt != nil && participant.LeftAt.Before(participant.JoinedAt) {
			return fmt.Errorf("participant left before joining")
		}
		if _, exists := participantIDs[participant.ID]; exists {
			return fmt.Errorf("duplicate participant id")
		}
		participantIDs[participant.ID] = struct{}{}
		if participant.LeftAt == nil {
			if _, exists := activeSessions[participant.SessionID]; exists {
				return fmt.Errorf("duplicate active participant session")
			}
			activeSessions[participant.SessionID] = struct{}{}
			activeParticipantIDs[participant.ID] = struct{}{}
		}
		if participant.Role == RoleOwner && participant.LeftAt == nil && participant.SessionID == aggregate.OwnerSessionID {
			ownerCount++
		}
	}
	if ownerCount != 1 {
		return fmt.Errorf("table must have exactly one active owner")
	}
	if len(activeParticipantIDs) > 4 {
		return fmt.Errorf("table cannot have more than four active participants")
	}
	seatedParticipants := make(map[string]struct{}, len(aggregate.Seats))
	for seat, assignment := range aggregate.Seats {
		if !seat.Valid() || assignment.ControllerEpoch < 1 {
			return fmt.Errorf("invalid seat assignment")
		}
		if _, exists := activeParticipantIDs[assignment.ParticipantID]; !exists {
			return fmt.Errorf("seat references inactive participant")
		}
		if _, exists := seatedParticipants[assignment.ParticipantID]; exists {
			return fmt.Errorf("participant occupies multiple seats")
		}
		seatedParticipants[assignment.ParticipantID] = struct{}{}
	}
	if aggregate.State == StateActive || aggregate.State == StateBetweenBoards {
		if aggregate.Game == nil || aggregate.BoardID == "" || aggregate.BoardNumber < 1 {
			return fmt.Errorf("game state is required")
		}
		if err := aggregate.Game.ValidateInvariants(); err != nil {
			return fmt.Errorf("game invariant: %w", err)
		}
	}
	if aggregate.ActionRequest != nil {
		request := aggregate.ActionRequest
		if aggregate.Game == nil || aggregate.State != StateActive && aggregate.State != StateBetweenBoards || !request.RequesterSeat.Valid() || request.Kind != ActionRequestClaim && request.Kind != ActionRequestUndo {
			return fmt.Errorf("invalid action request")
		}
		requesterAssignment, requesterSeated := aggregate.Seats[request.RequesterSeat]
		if !requesterSeated || requesterAssignment.ParticipantID == "" {
			return fmt.Errorf("action requester is not seated")
		}
		seenApprovals := make(map[bridge.Seat]struct{}, len(request.ApprovedBy))
		for _, seat := range request.ApprovedBy {
			if !seat.Valid() || seat == request.RequesterSeat {
				return fmt.Errorf("invalid action approval")
			}
			if _, duplicate := seenApprovals[seat]; duplicate {
				return fmt.Errorf("duplicate action approval")
			}
			if _, seated := aggregate.Seats[seat]; !seated {
				return fmt.Errorf("action approver is not seated")
			}
			if request.Kind == ActionRequestClaim && seat.Partnership() == request.RequesterSeat.Partnership() {
				return fmt.Errorf("claim approval must come from an opponent")
			}
			seenApprovals[seat] = struct{}{}
		}
		if request.Kind == ActionRequestClaim && len(request.ApprovedBy) > 1 || request.Kind == ActionRequestUndo && len(request.ApprovedBy) > 2 {
			return fmt.Errorf("action request has too many pending approvals")
		}
		if request.Kind == ActionRequestClaim {
			if aggregate.State != StateActive {
				return fmt.Errorf("claim request requires an active board")
			}
			if _, domainError := bridge.Decide(*aggregate.Game, bridge.ClaimCommand(request.RequesterSeat, request.ClaimTricks)); domainError != nil {
				return fmt.Errorf("invalid claim request: %w", domainError)
			}
		} else if aggregate.UndoableAction == nil || aggregate.UndoableAction.ActorSeat != request.RequesterSeat {
			return fmt.Errorf("undo request does not match the latest action")
		}
	}
	if aggregate.UndoableAction != nil {
		if aggregate.Game == nil || !aggregate.UndoableAction.ActorSeat.Valid() {
			return fmt.Errorf("invalid undoable action")
		}
		if err := aggregate.UndoableAction.Game.ValidateInvariants(); err != nil {
			return fmt.Errorf("undo state invariant: %w", err)
		}
	}
	return nil
}

func (aggregate Aggregate) activeParticipant(sessionID string) (Participant, bool) {
	for _, participant := range aggregate.Participants {
		if participant.SessionID == sessionID && participant.LeftAt == nil {
			return participant, true
		}
	}
	return Participant{}, false
}

func (aggregate Aggregate) activeParticipantCount() int {
	count := 0
	for _, participant := range aggregate.Participants {
		if participant.LeftAt == nil {
			count++
		}
	}
	return count
}

func (aggregate Aggregate) seatForParticipant(participantID string) (bridge.Seat, bool) {
	for seat, assignment := range aggregate.Seats {
		if assignment.ParticipantID == participantID {
			return seat, true
		}
	}
	return "", false
}

func (aggregate Aggregate) replacementOwnerIndex(ownerParticipantID string, preferredParticipantID string) int {
	if preferredParticipantID != "" {
		preferredIndex := slices.IndexFunc(aggregate.Participants, func(participant Participant) bool {
			return participant.ID == preferredParticipantID && participant.ID != ownerParticipantID && participant.LeftAt == nil
		})
		if preferredIndex >= 0 {
			return preferredIndex
		}
	}
	for _index, participant := range aggregate.Participants {
		if participant.ID == ownerParticipantID || participant.LeftAt != nil {
			continue
		}
		if _, seated := aggregate.seatForParticipant(participant.ID); seated {
			return _index
		}
	}
	return slices.IndexFunc(aggregate.Participants, func(participant Participant) bool {
		return participant.ID != ownerParticipantID && participant.LeftAt == nil
	})
}

func (aggregate Aggregate) clone() Aggregate {
	clone := aggregate
	clone.Participants = append([]Participant(nil), aggregate.Participants...)
	for _index := range clone.Participants {
		if clone.Participants[_index].LeftAt != nil {
			leftAt := *clone.Participants[_index].LeftAt
			clone.Participants[_index].LeftAt = &leftAt
		}
	}
	clone.Seats = make(map[bridge.Seat]SeatAssignment, len(aggregate.Seats))
	for seat, assignment := range aggregate.Seats {
		clone.Seats[seat] = assignment
	}
	if aggregate.Game != nil {
		game := aggregate.Game.Clone()
		clone.Game = &game
	}
	if aggregate.ActionRequest != nil {
		request := *aggregate.ActionRequest
		request.ApprovedBy = append([]bridge.Seat(nil), aggregate.ActionRequest.ApprovedBy...)
		clone.ActionRequest = &request
	}
	if aggregate.UndoableAction != nil {
		undoable := *aggregate.UndoableAction
		undoable.Game = aggregate.UndoableAction.Game.Clone()
		clone.UndoableAction = &undoable
	}
	return clone
}

func reject(code ErrorCode, message string) *DomainError {
	return &DomainError{Code: code, Message: message}
}
