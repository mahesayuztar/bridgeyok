package realtime

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/bridge"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/table"
)

const protocolVersion = 1

const (
	kindCommand = "command"
	kindControl = "control"
)

const (
	nameSubscribe         = "table.subscribe"
	nameResume            = "table.resume"
	nameHeartbeat         = "client.heartbeat"
	nameTakeSeat          = "table.take_seat"
	nameLeaveSeat         = "table.leave_seat"
	nameSetReady          = "table.set_ready"
	nameLockTable         = "table.lock"
	nameRemoveParticipant = "table.remove_participant"
	nameStartGame         = "table.start_game"
	nameMakeCall          = "game.make_call"
	namePlayCard          = "game.play_card"
	nameRequestClaim      = "game.request_claim"
	nameRespondClaim      = "game.respond_claim"
	nameRequestUndo       = "game.request_undo"
	nameRespondUndo       = "game.respond_undo"
	nameNextBoard         = "table.next_board"
	nameFinishTable       = "table.finish"
	nameLeaveTable        = "table.leave"
	nameTakeover          = "table.takeover"
)

// ClientEnvelope is one strictly validated inbound realtime message.
type ClientEnvelope struct {
	Version          int             `json:"v"`
	Kind             string          `json:"kind"`
	Name             string          `json:"name"`
	RequestID        string          `json:"request_id,omitempty"`
	TableID          string          `json:"table_id,omitempty"`
	ExpectedRevision *int64          `json:"expected_revision,omitempty"`
	ControllerEpoch  *int64          `json:"controller_epoch,omitempty"`
	Payload          json.RawMessage `json:"payload"`
}

type ackEnvelope struct {
	Version   int        `json:"v"`
	Kind      string     `json:"kind"`
	Name      string     `json:"name"`
	RequestID string     `json:"request_id"`
	TableID   string     `json:"table_id"`
	Revision  int64      `json:"revision"`
	Seq       int64      `json:"seq"`
	Payload   ackPayload `json:"payload"`
}

type ackPayload struct {
	Status    string `json:"status"`
	Duplicate bool   `json:"duplicate"`
	SyncMode  string `json:"syncMode,omitempty"`
}

type errorEnvelope struct {
	Version   int            `json:"v"`
	Kind      string         `json:"kind"`
	Name      string         `json:"name"`
	RequestID string         `json:"request_id,omitempty"`
	TableID   string         `json:"table_id,omitempty"`
	Code      string         `json:"code"`
	Retryable bool           `json:"retryable"`
	Revision  *int64         `json:"revision,omitempty"`
	Seq       *int64         `json:"seq,omitempty"`
	Payload   map[string]any `json:"payload"`
}

type eventEnvelope struct {
	Version  int                   `json:"v"`
	Kind     string                `json:"kind"`
	Name     string                `json:"name"`
	TableID  string                `json:"table_id"`
	Revision int64                 `json:"revision"`
	Seq      int64                 `json:"seq"`
	Payload  projectedEventPayload `json:"payload"`
}

type projectedEventPayload struct {
	EventType string           `json:"eventType"`
	Table     table.Projection `json:"table"`
}

type snapshotEnvelope struct {
	Version  int              `json:"v"`
	Kind     string           `json:"kind"`
	Name     string           `json:"name"`
	TableID  string           `json:"table_id"`
	Revision int64            `json:"revision"`
	Seq      int64            `json:"seq"`
	Payload  table.Projection `json:"payload"`
}

type controlEnvelope struct {
	Version   int            `json:"v"`
	Kind      string         `json:"kind"`
	Name      string         `json:"name"`
	RequestID string         `json:"request_id,omitempty"`
	TableID   string         `json:"table_id,omitempty"`
	Payload   map[string]any `json:"payload"`
}

type subscriptionPayload struct {
	LastSeenSeq int64 `json:"last_seen_seq"`
}

func decodeClientEnvelope(message []byte) (ClientEnvelope, error) {
	var envelope ClientEnvelope
	if err := decodeStrict(message, &envelope); err != nil {
		return ClientEnvelope{}, fmt.Errorf("decode envelope: %w", err)
	}
	if envelope.Version != protocolVersion || !validMessageName(envelope.Name) {
		return ClientEnvelope{}, fmt.Errorf("unsupported protocol envelope")
	}
	trimmedPayload := bytes.TrimSpace(envelope.Payload)
	if len(trimmedPayload) == 0 || trimmedPayload[0] != '{' {
		return ClientEnvelope{}, fmt.Errorf("payload must be an object")
	}
	switch envelope.Kind {
	case kindControl:
		if envelope.Name != nameHeartbeat || envelope.RequestID != "" || envelope.TableID != "" || envelope.ExpectedRevision != nil || envelope.ControllerEpoch != nil {
			return ClientEnvelope{}, fmt.Errorf("invalid control envelope")
		}
		if err := decodeEmptyPayload(envelope.Payload); err != nil {
			return ClientEnvelope{}, err
		}
	case kindCommand:
		if !validRequestID(envelope.RequestID) {
			return ClientEnvelope{}, fmt.Errorf("request id is invalid")
		}
		if _, err := uuid.Parse(envelope.TableID); err != nil {
			return ClientEnvelope{}, fmt.Errorf("table id is invalid")
		}
		if envelope.Name == nameSubscribe || envelope.Name == nameResume {
			if envelope.ExpectedRevision != nil || envelope.ControllerEpoch != nil {
				return ClientEnvelope{}, fmt.Errorf("subscription envelope contains mutation fields")
			}
			if _, err := decodeSubscriptionPayload(envelope.Payload); err != nil {
				return ClientEnvelope{}, err
			}
			return envelope, nil
		}
		if envelope.ExpectedRevision == nil || *envelope.ExpectedRevision < 0 {
			return ClientEnvelope{}, fmt.Errorf("expected revision is required")
		}
		if envelope.ControllerEpoch != nil && *envelope.ControllerEpoch < 1 {
			return ClientEnvelope{}, fmt.Errorf("controller epoch is invalid")
		}
		if err := validateMutationPayload(envelope); err != nil {
			return ClientEnvelope{}, err
		}
	default:
		return ClientEnvelope{}, fmt.Errorf("client message kind is unsupported")
	}
	return envelope, nil
}

func decodeSubscriptionPayload(payload json.RawMessage) (subscriptionPayload, error) {
	var decoded subscriptionPayload
	if err := decodeStrict(payload, &decoded); err != nil || decoded.LastSeenSeq < 0 {
		return subscriptionPayload{}, fmt.Errorf("subscription payload is invalid")
	}
	return decoded, nil
}

func validateMutationPayload(envelope ClientEnvelope) error {
	switch envelope.Name {
	case nameTakeSeat:
		var payload struct {
			Seat bridge.Seat `json:"seat"`
		}
		if err := decodeStrict(envelope.Payload, &payload); err != nil || !payload.Seat.Valid() {
			return fmt.Errorf("take seat payload is invalid")
		}
	case nameSetReady:
		var payload struct {
			Ready *bool `json:"ready"`
		}
		if err := decodeStrict(envelope.Payload, &payload); err != nil || payload.Ready == nil {
			return fmt.Errorf("ready payload is invalid")
		}
	case nameLockTable:
		var payload struct {
			Locked *bool `json:"locked"`
		}
		if err := decodeStrict(envelope.Payload, &payload); err != nil || payload.Locked == nil {
			return fmt.Errorf("lock payload is invalid")
		}
	case nameRemoveParticipant:
		var payload struct {
			ParticipantID string `json:"participant_id"`
		}
		if err := decodeStrict(envelope.Payload, &payload); err != nil {
			return fmt.Errorf("remove participant payload is invalid")
		}
		if _, err := uuid.Parse(payload.ParticipantID); err != nil {
			return fmt.Errorf("participant id is invalid")
		}
	case nameMakeCall:
		var payload struct {
			Call bridge.Call `json:"call"`
		}
		if err := decodeStrict(envelope.Payload, &payload); err != nil || payload.Call.Validate() != nil {
			return fmt.Errorf("call payload is invalid")
		}
	case namePlayCard:
		var payload struct {
			Card bridge.Card `json:"card"`
		}
		if err := decodeStrict(envelope.Payload, &payload); err != nil || payload.Card.Validate() != nil {
			return fmt.Errorf("card payload is invalid")
		}
	case nameRequestClaim:
		var payload struct {
			Tricks *int `json:"tricks"`
		}
		if err := decodeStrict(envelope.Payload, &payload); err != nil || payload.Tricks == nil || *payload.Tricks < 0 || *payload.Tricks > 13 {
			return fmt.Errorf("claim payload is invalid")
		}
	case nameRespondClaim, nameRespondUndo:
		var payload struct {
			Accepted *bool `json:"accepted"`
		}
		if err := decodeStrict(envelope.Payload, &payload); err != nil || payload.Accepted == nil {
			return fmt.Errorf("response payload is invalid")
		}
	case nameLeaveSeat, nameStartGame, nameNextBoard, nameFinishTable, nameLeaveTable, nameTakeover, nameRequestUndo:
		if err := decodeEmptyPayload(envelope.Payload); err != nil {
			return err
		}
	default:
		return fmt.Errorf("command name is unsupported")
	}
	return nil
}

func tableCommand(envelope ClientEnvelope, random io.Reader, now time.Time) (table.Command, error) {
	command := table.Command{OccurredAt: now.UTC()}
	if envelope.ControllerEpoch != nil {
		command.ControllerEpoch = *envelope.ControllerEpoch
	}
	switch envelope.Name {
	case nameTakeSeat:
		var payload struct {
			Seat bridge.Seat `json:"seat"`
		}
		if err := decodeStrict(envelope.Payload, &payload); err != nil {
			return table.Command{}, fmt.Errorf("decode take seat command: %w", err)
		}
		command.Name, command.Seat = table.CommandTakeSeat, payload.Seat
	case nameLeaveSeat:
		command.Name = table.CommandLeaveSeat
	case nameSetReady:
		var payload struct {
			Ready bool `json:"ready"`
		}
		if err := decodeStrict(envelope.Payload, &payload); err != nil {
			return table.Command{}, fmt.Errorf("decode ready command: %w", err)
		}
		command.Name, command.Ready = table.CommandSetReady, payload.Ready
	case nameLockTable:
		var payload struct {
			Locked bool `json:"locked"`
		}
		if err := decodeStrict(envelope.Payload, &payload); err != nil {
			return table.Command{}, fmt.Errorf("decode lock command: %w", err)
		}
		command.Name, command.Locked = table.CommandLockTable, payload.Locked
	case nameRemoveParticipant:
		var payload struct {
			ParticipantID string `json:"participant_id"`
		}
		if err := decodeStrict(envelope.Payload, &payload); err != nil {
			return table.Command{}, fmt.Errorf("decode remove participant command: %w", err)
		}
		command.Name, command.ParticipantID = table.CommandRemoveParticipant, payload.ParticipantID
	case nameStartGame, nameNextBoard:
		deal, err := bridge.GenerateDeal(random)
		if err != nil {
			return table.Command{}, fmt.Errorf("generate board deal: %w", err)
		}
		boardID, err := uuid.NewRandomFromReader(random)
		if err != nil {
			return table.Command{}, fmt.Errorf("generate board id: %w", err)
		}
		command.Deal, command.BoardID = &deal, boardID.String()
		if envelope.Name == nameStartGame {
			command.Name = table.CommandStartGame
		} else {
			command.Name = table.CommandRequestNextBoard
		}
	case nameMakeCall:
		var payload struct {
			Call bridge.Call `json:"call"`
		}
		if err := decodeStrict(envelope.Payload, &payload); err != nil {
			return table.Command{}, fmt.Errorf("decode call command: %w", err)
		}
		command.Name, command.Call = table.CommandMakeCall, &payload.Call
	case namePlayCard:
		var payload struct {
			Card bridge.Card `json:"card"`
		}
		if err := decodeStrict(envelope.Payload, &payload); err != nil {
			return table.Command{}, fmt.Errorf("decode card command: %w", err)
		}
		command.Name, command.Card = table.CommandPlayCard, &payload.Card
	case nameRequestClaim:
		var payload struct {
			Tricks int `json:"tricks"`
		}
		if err := decodeStrict(envelope.Payload, &payload); err != nil {
			return table.Command{}, fmt.Errorf("decode claim command: %w", err)
		}
		command.Name, command.ClaimTricks = table.CommandRequestClaim, payload.Tricks
	case nameRespondClaim, nameRespondUndo:
		var payload struct {
			Accepted bool `json:"accepted"`
		}
		if err := decodeStrict(envelope.Payload, &payload); err != nil {
			return table.Command{}, fmt.Errorf("decode response command: %w", err)
		}
		command.Accepted = payload.Accepted
		if envelope.Name == nameRespondClaim {
			command.Name = table.CommandRespondClaim
		} else {
			command.Name = table.CommandRespondUndo
		}
	case nameRequestUndo:
		command.Name = table.CommandRequestUndo
	case nameFinishTable:
		command.Name = table.CommandFinishTable
	case nameLeaveTable:
		command.Name = table.CommandLeaveTable
	case nameTakeover:
		command.Name = table.CommandTakeoverControl
	default:
		return table.Command{}, fmt.Errorf("command name is unsupported")
	}
	return command, nil
}

func decodeEmptyPayload(payload json.RawMessage) error {
	var decoded struct{}
	if err := decodeStrict(payload, &decoded); err != nil {
		return fmt.Errorf("payload must be empty: %w", err)
	}
	return nil
}

func decodeStrict(encoded []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("message must contain one JSON value")
	}
	return nil
}

func validRequestID(requestID string) bool {
	if len(requestID) < 8 || len(requestID) > 64 {
		return false
	}
	for _, character := range requestID {
		if (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') && (character < '0' || character > '9') && character != '-' && character != '_' {
			return false
		}
	}
	return true
}

func validMessageName(name string) bool {
	if len(name) < 1 || len(name) > 64 || name[0] < 'a' || name[0] > 'z' {
		return false
	}
	for _, character := range name[1:] {
		if (character < 'a' || character > 'z') && (character < '0' || character > '9') && character != '_' && character != '.' && character != '-' {
			return false
		}
	}
	return true
}
