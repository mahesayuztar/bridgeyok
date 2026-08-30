package realtime

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/bridge"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/identity"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/table"
)

const (
	realtimeTableID       = "0f4b9a5b-0ea8-4ad6-a866-e576ccd8be31"
	realtimeSessionID     = "fe752a3c-61e8-4f08-867e-e24945101aa8"
	realtimeParticipantID = "99ef3682-3ba8-42db-9c33-17238bfb2207"
	realtimeOrigin        = "http://client.example"
)

type scriptedIdentity struct {
	mutex       sync.Mutex
	session     identity.Session
	tickets     map[string]struct{}
	active      bool
	validations int
}

func (service *scriptedIdentity) ConsumeTicket(_ context.Context, ticket string) (identity.Session, error) {
	service.mutex.Lock()
	defer service.mutex.Unlock()
	if _, exists := service.tickets[ticket]; !exists || !service.active {
		return identity.Session{}, identity.ErrInvalidTicket
	}
	delete(service.tickets, ticket)
	return service.session, nil
}

func (service *scriptedIdentity) ValidateSession(_ context.Context, sessionID string) (identity.Session, error) {
	service.mutex.Lock()
	defer service.mutex.Unlock()
	service.validations++
	if !service.active || sessionID != service.session.ID {
		return identity.Session{}, identity.ErrSessionInactive
	}
	return service.session, nil
}

func (service *scriptedIdentity) setActive(active bool) {
	service.mutex.Lock()
	service.active = active
	service.mutex.Unlock()
}

func (service *scriptedIdentity) validationCount() int {
	service.mutex.Lock()
	defer service.mutex.Unlock()
	return service.validations
}

type scriptedRuntime struct {
	mutex     sync.Mutex
	aggregate table.Aggregate
	events    []table.PersistedEvent
	processed map[string]table.CommandResult
}

func (runtime *scriptedRuntime) Submit(_ context.Context, request table.CommandRequest) (table.CommandResult, error) {
	runtime.mutex.Lock()
	defer runtime.mutex.Unlock()
	if stored, exists := runtime.processed[request.RequestID]; exists {
		stored.Aggregate = runtime.aggregate
		stored.Events = nil
		stored.Duplicate = true
		return stored, nil
	}
	if request.ExpectedRevision != runtime.aggregate.Revision {
		result := table.CommandResult{
			Aggregate: runtime.aggregate,
			Outcome: table.CommandOutcome{
				RequestID: request.RequestID, CommandName: request.Command.Name, Status: table.CommandStatusRejected,
				ErrorCode: table.ErrorStateChanged, Revision: runtime.aggregate.Revision, LastSeq: runtime.aggregate.LastSeq,
			},
		}
		runtime.processed[request.RequestID] = result
		return result, nil
	}
	decision, domainError := table.Decide(runtime.aggregate, request.Command)
	if domainError != nil {
		result := table.CommandResult{
			Aggregate: runtime.aggregate,
			Outcome: table.CommandOutcome{
				RequestID: request.RequestID, CommandName: request.Command.Name, Status: table.CommandStatusRejected,
				ErrorCode: domainError.Code, Revision: runtime.aggregate.Revision, LastSeq: runtime.aggregate.LastSeq,
			},
		}
		runtime.processed[request.RequestID] = result
		return result, nil
	}
	next := decision.NextState
	next.Revision = runtime.aggregate.Revision + 1
	next.LastSeq = runtime.aggregate.LastSeq + int64(len(decision.Events))
	persisted := make([]table.PersistedEvent, 0, len(decision.Events))
	for _index, event := range decision.Events {
		persisted = append(persisted, table.PersistedEvent{
			TableID: next.ID, Seq: runtime.aggregate.LastSeq + int64(_index) + 1, Revision: next.Revision,
			Type: event.Type, Payload: event.Payload, OccurredAt: request.Command.OccurredAt,
		})
	}
	runtime.aggregate = next
	runtime.events = append(runtime.events, persisted...)
	result := table.CommandResult{
		Aggregate: next,
		Events:    persisted,
		Outcome: table.CommandOutcome{
			RequestID: request.RequestID, CommandName: request.Command.Name, Status: table.CommandStatusAccepted,
			Revision: next.Revision, FirstSeq: persisted[0].Seq, LastSeq: next.LastSeq,
		},
	}
	runtime.processed[request.RequestID] = result
	return result, nil
}

func (runtime *scriptedRuntime) Snapshot(context.Context, string) (table.Aggregate, error) {
	runtime.mutex.Lock()
	defer runtime.mutex.Unlock()
	return runtime.aggregate, nil
}

func (runtime *scriptedRuntime) Refresh(context.Context, string) (table.Aggregate, error) {
	runtime.mutex.Lock()
	defer runtime.mutex.Unlock()
	return runtime.aggregate, nil
}

func (runtime *scriptedRuntime) ListEventsAfter(_ context.Context, _ string, afterSeq int64, limit int) ([]table.PersistedEvent, error) {
	runtime.mutex.Lock()
	defer runtime.mutex.Unlock()
	events := make([]table.PersistedEvent, 0, limit)
	for _, event := range runtime.events {
		if event.Seq > afterSeq {
			events = append(events, event)
			if len(events) == limit {
				break
			}
		}
	}
	return events, nil
}

type wireEnvelope struct {
	Kind     string          `json:"kind"`
	Name     string          `json:"name"`
	Code     string          `json:"code"`
	Revision int64           `json:"revision"`
	Seq      int64           `json:"seq"`
	Payload  json.RawMessage `json:"payload"`
}

func TestNewServerRejectsNonExactOrigins(t *testing.T) {
	t.Parallel()

	valid, _, _ := scriptedServer(t, realtimeAggregate(t), nil)
	for _, origin := range []string{"*", "https://*.example", "https://client.example/path"} {
		options := valid.options
		options.AllowedOrigins = []string{origin}
		if _, err := NewServer(options); err == nil {
			t.Fatalf("NewServer() accepted origin %q", origin)
		}
	}
}

func TestServerRequiresExactOriginAndSingleUseTicket(t *testing.T) {
	t.Parallel()

	server, identityService, _ := scriptedServer(t, realtimeAggregate(t), nil, "ticket-one")
	httpServer := httptest.NewServer(server)
	defer httpServer.Close()

	_, response, err := dialScripted(t, httpServer.URL, "ticket-one", "http://attacker.example")
	if err == nil || response == nil || response.StatusCode != http.StatusForbidden {
		t.Fatalf("wrong-origin Dial() error = %v, response = %+v", err, response)
	}
	closeResponse(t, response)

	client, response, err := dialScripted(t, httpServer.URL, "ticket-one", realtimeOrigin)
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	closeResponse(t, response)
	defer closeClient(t, client)

	_, response, err = dialScripted(t, httpServer.URL, "ticket-one", realtimeOrigin)
	if err == nil || response == nil || response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("reused-ticket Dial() error = %v, response = %+v", err, response)
	}
	closeResponse(t, response)
	if identityService.validationCount() != 0 {
		t.Fatalf("validation count = %d before messages, want 0", identityService.validationCount())
	}
}

func TestServerLogsDoNotExposeTicketOrGuestIdentity(t *testing.T) {
	t.Parallel()

	const ticket = "ticket-sensitive-value"
	server, _, _ := scriptedServer(t, realtimeAggregate(t), nil, ticket)
	logs := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(logs, nil))
	server.options.Logger = logger
	server.broker.logger = logger
	httpServer := httptest.NewServer(server)
	defer httpServer.Close()
	client := mustDialScripted(t, httpServer.URL, ticket)
	writeScripted(t, client, map[string]any{"v": 1, "kind": "control", "name": "client.heartbeat", "payload": map[string]any{}})
	readScripted(t, client)
	closeClient(t, client)
	drainCtx, cancelDrain := context.WithTimeout(t.Context(), time.Second)
	defer cancelDrain()
	if err := server.Drain(drainCtx); err != nil {
		t.Fatalf("Drain() error = %v", err)
	}
	encodedLogs := logs.String()
	for _, privateValue := range []string{ticket, realtimeSessionID, "Owner"} {
		if strings.Contains(encodedLogs, privateValue) {
			t.Fatalf("realtime logs exposed private value %q: %s", privateValue, encodedLogs)
		}
	}
}

func TestServerSubscribesAndBroadcastsAckBeforeProjectedEvent(t *testing.T) {
	t.Parallel()

	server, identityService, _ := scriptedServer(t, realtimeAggregate(t), nil, "ticket-command")
	httpServer := httptest.NewServer(server)
	defer httpServer.Close()
	client := mustDialScripted(t, httpServer.URL, "ticket-command")
	defer closeClient(t, client)

	writeScripted(t, client, map[string]any{
		"v": 1, "kind": "command", "name": "table.subscribe", "request_id": "subscribe_01",
		"table_id": realtimeTableID, "payload": map[string]any{"last_seen_seq": 0},
	})
	ack, _ := readScripted(t, client)
	snapshot, _ := readScripted(t, client)
	presence, _ := readScripted(t, client)
	if ack.Kind != "ack" || ack.Name != "table.subscribed" || snapshot.Kind != "snapshot" || presence.Name != "presence.snapshot" {
		t.Fatalf("subscription frames = %+v, %+v, %+v", ack, snapshot, presence)
	}

	writeScripted(t, client, map[string]any{
		"v": 1, "kind": "command", "name": "table.lock", "request_id": "command_0001",
		"table_id": realtimeTableID, "expected_revision": 0, "payload": map[string]any{"locked": true},
	})
	commandAck, _ := readScripted(t, client)
	event, encodedEvent := readScripted(t, client)
	if commandAck.Kind != "ack" || commandAck.Name != "command.accepted" || event.Kind != "event" || event.Name != "table.locked" {
		t.Fatalf("mutation frames = %+v then %+v", commandAck, event)
	}
	if commandAck.Revision != 1 || event.Revision != 1 || event.Seq != 1 {
		t.Fatalf("mutation versions = ack %+v, event %+v", commandAck, event)
	}
	if strings.Contains(string(encodedEvent), realtimeSessionID) || strings.Contains(string(encodedEvent), "sessionId") {
		t.Fatalf("projected event exposed private session data: %s", encodedEvent)
	}
	if identityService.validationCount() != 2 {
		t.Fatalf("validation count = %d, want one per message", identityService.validationCount())
	}

	writeScripted(t, client, map[string]any{
		"v": 1, "kind": "command", "name": "table.lock", "request_id": "command_0001",
		"table_id": realtimeTableID, "expected_revision": 0, "payload": map[string]any{"locked": true},
	})
	duplicate, _ := readScripted(t, client)
	var duplicatePayload struct {
		Duplicate bool `json:"duplicate"`
	}
	if err := json.Unmarshal(duplicate.Payload, &duplicatePayload); err != nil {
		t.Fatalf("decode duplicate ack: %v", err)
	}
	if duplicate.Kind != "ack" || !duplicatePayload.Duplicate || duplicate.Revision != 1 {
		t.Fatalf("duplicate ack = %+v with payload %+v", duplicate, duplicatePayload)
	}

	writeScripted(t, client, map[string]any{
		"v": 1, "kind": "command", "name": "table.lock", "request_id": "command_stale_01",
		"table_id": realtimeTableID, "expected_revision": 0, "payload": map[string]any{"locked": false},
	})
	stale, _ := readScripted(t, client)
	if stale.Kind != "error" || stale.Code != string(table.ErrorStateChanged) || stale.Revision != 1 || stale.Seq != 1 {
		t.Fatalf("out-of-order response = %+v", stale)
	}
}

func TestServerReconnectUsesGapEventsOrSnapshotFallback(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		events   []table.PersistedEvent
		wantKind string
		wantSeqs []int64
		wantMode string
	}{
		{
			name: "contiguous gap",
			events: []table.PersistedEvent{
				{TableID: realtimeTableID, Seq: 2, Revision: 2, Type: "READY_CHANGED"},
				{TableID: realtimeTableID, Seq: 3, Revision: 3, Type: "TABLE_LOCKED"},
			},
			wantKind: "event", wantSeqs: []int64{2, 3}, wantMode: "events",
		},
		{
			name: "event gap fallback",
			events: []table.PersistedEvent{
				{TableID: realtimeTableID, Seq: 3, Revision: 3, Type: "TABLE_LOCKED"},
			},
			wantKind: "snapshot", wantSeqs: []int64{3}, wantMode: "snapshot",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			aggregate := realtimeAggregate(t)
			aggregate.Revision = 3
			aggregate.LastSeq = 3
			server, _, _ := scriptedServer(t, aggregate, test.events, "ticket-resume")
			httpServer := httptest.NewServer(server)
			defer httpServer.Close()
			client := mustDialScripted(t, httpServer.URL, "ticket-resume")
			defer closeClient(t, client)

			writeScripted(t, client, map[string]any{
				"v": 1, "kind": "command", "name": "table.resume", "request_id": "resume_001",
				"table_id": realtimeTableID, "payload": map[string]any{"last_seen_seq": 1},
			})
			ack, _ := readScripted(t, client)
			var ackPayload struct {
				SyncMode string `json:"syncMode"`
			}
			if err := json.Unmarshal(ack.Payload, &ackPayload); err != nil {
				t.Fatalf("decode ack payload: %v", err)
			}
			if ack.Kind != "ack" || ackPayload.SyncMode != test.wantMode {
				t.Fatalf("resume ack = %+v with payload %+v", ack, ackPayload)
			}
			for _, wantSeq := range test.wantSeqs {
				frame, _ := readScripted(t, client)
				if frame.Kind != test.wantKind || frame.Seq != wantSeq {
					t.Fatalf("recovery frame = %+v, want kind %s seq %d", frame, test.wantKind, wantSeq)
				}
			}
		})
	}
}

func TestServerRejectsInactiveSessionOnEveryMessage(t *testing.T) {
	t.Parallel()

	server, identityService, _ := scriptedServer(t, realtimeAggregate(t), nil, "ticket-revoked")
	httpServer := httptest.NewServer(server)
	defer httpServer.Close()
	client := mustDialScripted(t, httpServer.URL, "ticket-revoked")
	identityService.setActive(false)

	writeScripted(t, client, map[string]any{"v": 1, "kind": "control", "name": "client.heartbeat", "payload": map[string]any{}})
	frame, _ := readScripted(t, client)
	if frame.Kind != "error" || frame.Code != "SESSION_INACTIVE" {
		t.Fatalf("inactive session frame = %+v", frame)
	}
	readCtx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	_, _, err := client.Read(readCtx)
	if websocket.CloseStatus(err) != websocket.StatusPolicyViolation {
		t.Fatalf("close status = %d, error = %v", websocket.CloseStatus(err), err)
	}
}

func TestServerRejectsTableSubscriptionForNonParticipant(t *testing.T) {
	t.Parallel()

	aggregate, err := table.NewAggregate(realtimeTableID, table.Participant{
		ID: "ec0d4071-bdc2-49f0-a27d-7855c10ce19c", SessionID: "c2a67adc-cfb2-4dc2-a46c-cbab1fb16291",
		Nickname: "Different Owner", Role: table.RoleOwner, JoinedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("NewAggregate() error = %v", err)
	}
	server, _, _ := scriptedServer(t, aggregate, nil, "ticket-denied")
	httpServer := httptest.NewServer(server)
	defer httpServer.Close()
	client := mustDialScripted(t, httpServer.URL, "ticket-denied")
	defer closeClient(t, client)
	writeScripted(t, client, map[string]any{
		"v": 1, "kind": "command", "name": "table.subscribe", "request_id": "subscribe_denied",
		"table_id": realtimeTableID, "payload": map[string]any{"last_seen_seq": 0},
	})
	frame, _ := readScripted(t, client)
	if frame.Kind != "error" || frame.Code != "TABLE_ACCESS_DENIED" {
		t.Fatalf("subscription response = %+v", frame)
	}
}

func TestServerOwnerExpiryPromotesOnlineParticipant(t *testing.T) {
	t.Parallel()

	aggregate := realtimeAggregate(t)
	guest := table.Participant{
		ID: "91eeb013-54a1-4287-92cc-715904206f65", SessionID: "482f6524-66c1-4313-886c-e6bfd07fe58f",
		Nickname: "Replacement", Role: table.RoleParticipant, JoinedAt: time.Now().UTC(),
	}
	decision, domainError := table.Decide(aggregate, table.Command{Name: table.CommandJoinTable, Participant: &guest})
	if domainError != nil {
		t.Fatalf("join setup error = %v", domainError)
	}
	aggregate = decision.NextState
	server, _, runtime := scriptedServer(t, aggregate, nil)
	ownerConnection := projectedConnection(server, realtimeSessionID)
	guestConnection := projectedConnection(server, guest.SessionID)
	server.broker.subscribe(ownerConnection, aggregate.ID, nil, aggregate.Participants, realtimeParticipantID)
	server.broker.subscribe(guestConnection, aggregate.ID, nil, aggregate.Participants, guest.ID)
	server.broker.unsubscribe(ownerConnection)
	server.broker.mutex.Lock()
	generation := server.broker.presence[aggregate.ID][realtimeParticipantID].generation
	server.broker.mutex.Unlock()

	server.expireParticipant(t.Context(), aggregate.ID, realtimeParticipantID, generation)
	updated, err := runtime.Snapshot(t.Context(), aggregate.ID)
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if updated.OwnerSessionID != guest.SessionID {
		t.Fatalf("owner session = %s, want %s", updated.OwnerSessionID, guest.SessionID)
	}
	owner, exists := activeParticipantByID(updated, guest.ID)
	if !exists || owner.Role != table.RoleOwner {
		t.Fatalf("replacement owner = %+v", owner)
	}
}

func TestServerTakeoverFencesOldControllerEpoch(t *testing.T) {
	t.Parallel()

	aggregate := realtimeAggregate(t)
	decision, domainError := table.Decide(aggregate, table.Command{Name: table.CommandTakeSeat, SessionID: realtimeSessionID, Seat: bridge.North})
	if domainError != nil {
		t.Fatalf("take seat setup error = %v", domainError)
	}
	aggregate = decision.NextState
	aggregate.Revision = 1
	aggregate.LastSeq = 1
	server, _, _ := scriptedServer(t, aggregate, nil, "ticket-fence")
	httpServer := httptest.NewServer(server)
	defer httpServer.Close()
	client := mustDialScripted(t, httpServer.URL, "ticket-fence")
	defer closeClient(t, client)

	writeScripted(t, client, map[string]any{
		"v": 1, "kind": "command", "name": "table.subscribe", "request_id": "subscribe_02",
		"table_id": realtimeTableID, "payload": map[string]any{"last_seen_seq": 1},
	})
	readScripted(t, client)
	readScripted(t, client)
	readScripted(t, client)

	writeScripted(t, client, map[string]any{
		"v": 1, "kind": "command", "name": "table.set_ready", "request_id": "stale_missing",
		"table_id": realtimeTableID, "expected_revision": 1, "payload": map[string]any{"ready": true},
	})
	stale, _ := readScripted(t, client)
	if stale.Code != string(table.ErrorStaleController) {
		t.Fatalf("missing epoch response = %+v", stale)
	}

	writeScripted(t, client, map[string]any{
		"v": 1, "kind": "command", "name": "table.takeover", "request_id": "takeover_01",
		"table_id": realtimeTableID, "expected_revision": 1, "controller_epoch": 1, "payload": map[string]any{},
	})
	ack, _ := readScripted(t, client)
	event, _ := readScripted(t, client)
	if ack.Kind != "ack" || event.Name != "controller.replaced" {
		t.Fatalf("takeover frames = %+v then %+v", ack, event)
	}
	writeScripted(t, client, map[string]any{
		"v": 1, "kind": "command", "name": "table.takeover", "request_id": "takeover_01",
		"table_id": realtimeTableID, "expected_revision": 1, "controller_epoch": 1, "payload": map[string]any{},
	})
	duplicate, _ := readScripted(t, client)
	var duplicatePayload struct {
		Duplicate bool `json:"duplicate"`
	}
	if err := json.Unmarshal(duplicate.Payload, &duplicatePayload); err != nil || !duplicatePayload.Duplicate {
		t.Fatalf("duplicate takeover ack = %+v, payload = %+v, error = %v", duplicate, duplicatePayload, err)
	}

	writeScripted(t, client, map[string]any{
		"v": 1, "kind": "command", "name": "table.set_ready", "request_id": "stale_old_01",
		"table_id": realtimeTableID, "expected_revision": 2, "controller_epoch": 1, "payload": map[string]any{"ready": true},
	})
	stale, _ = readScripted(t, client)
	if stale.Code != string(table.ErrorStaleController) || stale.Revision != 2 || stale.Seq != 2 {
		t.Fatalf("old epoch response = %+v", stale)
	}
}

func TestServerDrainSignalsServiceRestartAndRejectsAdmission(t *testing.T) {
	t.Parallel()

	server, _, _ := scriptedServer(t, realtimeAggregate(t), nil, "ticket-drain", "ticket-after-drain")
	httpServer := httptest.NewServer(server)
	defer httpServer.Close()
	client := mustDialScripted(t, httpServer.URL, "ticket-drain")

	drainCtx, cancelDrain := context.WithTimeout(t.Context(), time.Second)
	defer cancelDrain()
	drainResult := make(chan error, 1)
	go func() {
		drainResult <- server.Drain(drainCtx)
	}()
	control, _ := readScripted(t, client)
	if control.Kind != "control" || control.Name != "server.draining" {
		t.Fatalf("drain control = %+v", control)
	}
	readCtx, cancelRead := context.WithTimeout(t.Context(), time.Second)
	defer cancelRead()
	_, _, err := client.Read(readCtx)
	if websocket.CloseStatus(err) != websocket.StatusServiceRestart {
		t.Fatalf("drain close status = %d, error = %v", websocket.CloseStatus(err), err)
	}
	if err := <-drainResult; err != nil {
		t.Fatalf("Drain() error = %v", err)
	}

	_, response, err := dialScripted(t, httpServer.URL, "ticket-after-drain", realtimeOrigin)
	if err == nil || response == nil || response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("post-drain Dial() error = %v, response = %+v", err, response)
	}
	closeResponse(t, response)
}

func TestServerEnforcesFrameSafetyAndProtocolStrikes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		write     func(*testing.T, *websocket.Conn)
		errorCode string
		closeCode websocket.StatusCode
	}{
		{
			name: "binary frame",
			write: func(t *testing.T, client *websocket.Conn) {
				writeRawScripted(t, client, websocket.MessageBinary, []byte{0x01})
			},
			errorCode: "UNSUPPORTED_DATA", closeCode: websocket.StatusUnsupportedData,
		},
		{
			name: "invalid UTF-8",
			write: func(t *testing.T, client *websocket.Conn) {
				writeRawScripted(t, client, websocket.MessageText, []byte{0xff})
			},
			errorCode: "INVALID_TEXT", closeCode: websocket.StatusInvalidFramePayloadData,
		},
		{
			name: "repeated malformed envelopes",
			write: func(t *testing.T, client *websocket.Conn) {
				for _attempt := 0; _attempt < 3; _attempt++ {
					writeRawScripted(t, client, websocket.MessageText, []byte(`{"v":`))
					frame, _ := readScripted(t, client)
					if frame.Code != "INVALID_MESSAGE" {
						t.Fatalf("malformed response = %+v", frame)
					}
				}
			},
			closeCode: websocket.StatusProtocolError,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server, _, _ := scriptedServer(t, realtimeAggregate(t), nil, "ticket-safety")
			httpServer := httptest.NewServer(server)
			defer httpServer.Close()
			client := mustDialScripted(t, httpServer.URL, "ticket-safety")
			test.write(t, client)
			if test.errorCode != "" {
				frame, _ := readScripted(t, client)
				if frame.Code != test.errorCode {
					t.Fatalf("safety response = %+v, want %s", frame, test.errorCode)
				}
			}
			assertCloseStatus(t, client, test.closeCode)
		})
	}

	t.Run("oversized message", func(t *testing.T) {
		server, _, _ := scriptedServer(t, realtimeAggregate(t), nil, "ticket-oversized")
		httpServer := httptest.NewServer(server)
		defer httpServer.Close()
		client := mustDialScripted(t, httpServer.URL, "ticket-oversized")
		writeRawScripted(t, client, websocket.MessageText, []byte(strings.Repeat("x", (8<<10)+1)))
		assertCloseStatus(t, client, websocket.StatusMessageTooBig)
	})
}

func scriptedServer(t *testing.T, aggregate table.Aggregate, events []table.PersistedEvent, tickets ...string) (*Server, *scriptedIdentity, *scriptedRuntime) {
	t.Helper()
	now := time.Now().UTC()
	identityService := &scriptedIdentity{
		session: identity.Session{ID: realtimeSessionID, Nickname: "Owner", Status: "ACTIVE", ExpiresAt: now.Add(time.Hour)},
		tickets: make(map[string]struct{}),
		active:  true,
	}
	for _, ticket := range tickets {
		identityService.tickets[ticket] = struct{}{}
	}
	runtime := &scriptedRuntime{
		aggregate: aggregate,
		events:    append([]table.PersistedEvent(nil), events...),
		processed: make(map[string]table.CommandResult),
	}
	server, err := NewServer(Options{
		Logger: slog.New(slog.NewJSONHandler(io.Discard, nil)), AllowedOrigins: []string{realtimeOrigin},
		Identity: identityService, Tables: runtime, Events: runtime, Random: rand.Reader, Now: time.Now,
		ReadLimitBytes: 8 << 10, OutboundQueueCapacity: 32, OutboundQueueBytes: 128 << 10,
		WriteTimeout: time.Second, PingInterval: time.Hour, PongTimeout: time.Second,
		PresenceGracePeriod: time.Hour,
		MaxConnections:      8, MaxConnectionsPerSession: 3, MessageRate: 100, MessageBurst: 100, RecoveryLimit: 16,
	})
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	t.Cleanup(server.broker.drain)
	return server, identityService, runtime
}

func realtimeAggregate(t *testing.T) table.Aggregate {
	t.Helper()
	aggregate, err := table.NewAggregate(realtimeTableID, table.Participant{
		ID: realtimeParticipantID, SessionID: realtimeSessionID, Nickname: "Owner", Role: table.RoleOwner, JoinedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("NewAggregate() error = %v", err)
	}
	return aggregate
}

func mustDialScripted(t *testing.T, serverURL string, ticket string) *websocket.Conn {
	t.Helper()
	client, response, err := dialScripted(t, serverURL, ticket, realtimeOrigin)
	if err != nil {
		closeResponse(t, response)
		t.Fatalf("Dial() error = %v", err)
	}
	closeResponse(t, response)
	return client
}

func dialScripted(t *testing.T, serverURL string, ticket string, origin string) (*websocket.Conn, *http.Response, error) {
	t.Helper()
	url := "ws" + strings.TrimPrefix(serverURL, "http") + "?ticket=" + ticket
	return websocket.Dial(t.Context(), url, &websocket.DialOptions{HTTPHeader: http.Header{"Origin": []string{origin}}})
}

func writeScripted(t *testing.T, client *websocket.Conn, message any) {
	t.Helper()
	encoded, err := json.Marshal(message)
	if err != nil {
		t.Fatalf("encode scripted message: %v", err)
	}
	writeRawScripted(t, client, websocket.MessageText, encoded)
}

func writeRawScripted(t *testing.T, client *websocket.Conn, messageType websocket.MessageType, encoded []byte) {
	t.Helper()
	writeCtx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if err := client.Write(writeCtx, messageType, encoded); err != nil {
		t.Fatalf("write scripted message: %v", err)
	}
}

func readScripted(t *testing.T, client *websocket.Conn) (wireEnvelope, []byte) {
	t.Helper()
	readCtx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	messageType, encoded, err := client.Read(readCtx)
	if err != nil {
		t.Fatalf("read scripted frame: %v", err)
	}
	if messageType != websocket.MessageText {
		t.Fatalf("message type = %v, want text", messageType)
	}
	var envelope wireEnvelope
	if err := json.Unmarshal(encoded, &envelope); err != nil {
		t.Fatalf("decode scripted frame: %v", err)
	}
	return envelope, encoded
}

func closeClient(t *testing.T, client *websocket.Conn) {
	t.Helper()
	if client == nil {
		return
	}
	if err := client.Close(websocket.StatusNormalClosure, "test complete"); err != nil && !errors.Is(err, context.Canceled) {
		t.Logf("close WebSocket client: %v", err)
	}
}

func assertCloseStatus(t *testing.T, client *websocket.Conn, want websocket.StatusCode) {
	t.Helper()
	readCtx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	_, _, err := client.Read(readCtx)
	if websocket.CloseStatus(err) != want {
		t.Fatalf("close status = %d, want %d; error = %v", websocket.CloseStatus(err), want, err)
	}
}

func closeResponse(t *testing.T, response *http.Response) {
	t.Helper()
	if response != nil && response.Body != nil {
		if err := response.Body.Close(); err != nil {
			t.Fatalf("close response body: %v", err)
		}
	}
}
