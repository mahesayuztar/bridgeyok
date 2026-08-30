package realtime

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"reflect"
	"testing"
	"time"

	"github.com/mahesayuztar/bridgeyok/apps/api/internal/bridge"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/identity"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/table"
)

func TestBrokerProjectsEventsForEachRecipient(t *testing.T) {
	t.Parallel()

	aggregate, sessions := activeRealtimeAggregate(t)
	server := &Server{options: Options{OutboundQueueBytes: 128 << 10}}
	north := projectedConnection(server, sessions[bridge.North])
	east := projectedConnection(server, sessions[bridge.East])
	roomBroker := newBroker(slog.New(slog.NewJSONHandler(io.Discard, nil)), time.Hour, time.Now, nil)
	t.Cleanup(roomBroker.drain)
	northParticipant, _ := activeParticipantForSession(aggregate, sessions[bridge.North])
	eastParticipant, _ := activeParticipantForSession(aggregate, sessions[bridge.East])
	if !roomBroker.subscribe(north, aggregate.ID, nil, aggregate.Participants, northParticipant.ID) || !roomBroker.subscribe(east, aggregate.ID, nil, aggregate.Participants, eastParticipant.ID) {
		t.Fatal("subscribe() rejected test connections")
	}
	result := table.CommandResult{
		Aggregate: aggregate,
		Outcome:   table.CommandOutcome{Status: table.CommandStatusAccepted},
		Events: []table.PersistedEvent{{
			TableID: aggregate.ID, Seq: 1, Revision: 1, Type: "BOARD_STARTED",
		}},
	}
	if err := roomBroker.publishResult(t.Context(), result); err != nil {
		t.Fatalf("publishResult() error = %v", err)
	}
	northProjection := projectedTableFromQueue(t, north)
	eastProjection := projectedTableFromQueue(t, east)
	if northProjection.Game == nil || eastProjection.Game == nil {
		t.Fatal("projected event omitted active game")
	}
	if !reflect.DeepEqual(northProjection.Game.OwnHand, aggregate.Game.Deal.Hand(bridge.North)) {
		t.Fatal("north event did not contain north hand")
	}
	if !reflect.DeepEqual(eastProjection.Game.OwnHand, aggregate.Game.Deal.Hand(bridge.East)) {
		t.Fatal("east event did not contain east hand")
	}
	if reflect.DeepEqual(northProjection.Game.OwnHand, eastProjection.Game.OwnHand) || northProjection.Game.FullDeal != nil || eastProjection.Game.FullDeal != nil {
		t.Fatal("recipient projection exposed another hand or the full deal")
	}
}

func TestBrokerPresenceExpiresAfterLastConnection(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	owner := table.Participant{ID: "dd681310-2bad-4251-b9a8-c74d496a18f9", SessionID: "01d180b4-2dbd-4280-981b-c69c20da25cc", Nickname: "Owner", Role: table.RoleOwner, JoinedAt: now}
	guest := table.Participant{ID: "418e80c2-574e-45e9-bad9-a71267c1f69c", SessionID: "944d1d7b-425f-454f-82aa-dae40e000760", Nickname: "Guest", Role: table.RoleParticipant, JoinedAt: now}
	aggregate, err := table.NewAggregate(realtimeTableID, owner)
	if err != nil {
		t.Fatalf("NewAggregate() error = %v", err)
	}
	decision, domainError := table.Decide(aggregate, table.Command{Name: table.CommandJoinTable, Participant: &guest})
	if domainError != nil {
		t.Fatalf("join setup error = %v", domainError)
	}
	aggregate = decision.NextState
	expired := make(chan string, 2)
	roomBroker := newBroker(slog.New(slog.NewJSONHandler(io.Discard, nil)), 20*time.Millisecond, time.Now, func(_ context.Context, _ string, participantID string, _ uint64) {
		expired <- participantID
	})
	t.Cleanup(roomBroker.drain)
	server := &Server{options: Options{OutboundQueueBytes: 128 << 10}}
	ownerConnection := projectedConnection(server, owner.SessionID)
	firstGuestConnection := projectedConnection(server, guest.SessionID)
	secondGuestConnection := projectedConnection(server, guest.SessionID)
	roomBroker.subscribe(ownerConnection, aggregate.ID, nil, aggregate.Participants, owner.ID)
	roomBroker.subscribe(firstGuestConnection, aggregate.ID, nil, aggregate.Participants, guest.ID)
	roomBroker.subscribe(secondGuestConnection, aggregate.ID, nil, aggregate.Participants, guest.ID)

	roomBroker.unsubscribe(firstGuestConnection)
	select {
	case participantID := <-expired:
		t.Fatalf("participant %s expired while another connection remained", participantID)
	case <-time.After(40 * time.Millisecond):
	}
	roomBroker.unsubscribe(secondGuestConnection)
	select {
	case participantID := <-expired:
		if participantID != guest.ID {
			t.Fatalf("expired participant = %s, want %s", participantID, guest.ID)
		}
	case <-time.After(time.Second):
		t.Fatal("offline participant did not expire")
	}
}

func TestTokenBucketUsesBoundedBurstAndRefill(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 30, 12, 0, 0, 0, time.UTC)
	bucket := tokenBucket{tokens: 2, last: now, rate: 1, burst: 2}
	for _attempt := 0; _attempt < 2; _attempt++ {
		if !bucket.allow(now) {
			t.Fatalf("token bucket rejected initial burst token %d", _attempt+1)
		}
	}
	if bucket.allow(now) {
		t.Fatal("token bucket exceeded its initial burst")
	}
	if !bucket.allow(now.Add(time.Second)) {
		t.Fatal("token bucket did not refill at the configured rate")
	}
	if bucket.allow(now.Add(time.Second)) {
		t.Fatal("token bucket refilled more than the configured rate")
	}
}

func TestConnectionOutboundQueueBoundsCountAndBytes(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	connection := &connection{
		ctx: ctx, cancel: cancel, server: &Server{options: Options{OutboundQueueBytes: 4}},
		outbound: make(chan outboundFrame, 1),
	}
	if !connection.enqueue(outboundFrame{message: []byte("1234")}) {
		t.Fatal("enqueue() rejected frame within count and byte bounds")
	}
	if connection.enqueue(outboundFrame{message: []byte("5")}) {
		t.Fatal("enqueue() exceeded outbound queue count")
	}
	frame := <-connection.outbound
	connection.releaseQueuedBytes(len(frame.message))
	if connection.enqueue(outboundFrame{message: []byte("12345")}) {
		t.Fatal("enqueue() exceeded outbound queue byte bound")
	}
}

func activeRealtimeAggregate(t *testing.T) (table.Aggregate, map[bridge.Seat]string) {
	t.Helper()
	now := time.Date(2026, time.August, 30, 12, 0, 0, 0, time.UTC)
	participants := []table.Participant{
		{ID: "0ac0763e-6328-4d6c-bc96-001c1b428163", SessionID: "1c48f35b-2a33-4660-b2d3-1614fdf937c8", Nickname: "North", Role: table.RoleOwner, JoinedAt: now},
		{ID: "f1bbf004-2eb2-41b6-b86a-b695bf9f28b7", SessionID: "f1b8d9d8-698e-47cd-af2a-8a8951c4aa58", Nickname: "East", Role: table.RoleParticipant, JoinedAt: now},
		{ID: "5ac4f064-5a62-45f9-bb02-4e3c404599d4", SessionID: "ea273deb-26fa-43a7-853b-cb7c3c018902", Nickname: "South", Role: table.RoleParticipant, JoinedAt: now},
		{ID: "5f250850-e76c-4609-81d0-c1100919799c", SessionID: "3ea06ae2-d10c-4d5d-b43d-bb7de843cb95", Nickname: "West", Role: table.RoleParticipant, JoinedAt: now},
	}
	aggregate, err := table.NewAggregate(realtimeTableID, participants[0])
	if err != nil {
		t.Fatalf("NewAggregate() error = %v", err)
	}
	for _, participant := range participants[1:] {
		decision, domainError := table.Decide(aggregate, table.Command{Name: table.CommandJoinTable, Participant: &participant})
		if domainError != nil {
			t.Fatalf("join setup error = %v", domainError)
		}
		aggregate = decision.NextState
	}
	sessions := make(map[bridge.Seat]string, 4)
	seats := []bridge.Seat{bridge.North, bridge.East, bridge.South, bridge.West}
	for _index, participant := range participants {
		decision, domainError := table.Decide(aggregate, table.Command{Name: table.CommandTakeSeat, SessionID: participant.SessionID, Seat: seats[_index]})
		if domainError != nil {
			t.Fatalf("seat setup error = %v", domainError)
		}
		aggregate = decision.NextState
		decision, domainError = table.Decide(aggregate, table.Command{Name: table.CommandSetReady, SessionID: participant.SessionID, Ready: true})
		if domainError != nil {
			t.Fatalf("ready setup error = %v", domainError)
		}
		aggregate = decision.NextState
		sessions[seats[_index]] = participant.SessionID
	}
	deal, err := bridge.GenerateDeal(bytes.NewReader(bytes.Repeat([]byte{0xff}, 1024)))
	if err != nil {
		t.Fatalf("GenerateDeal() error = %v", err)
	}
	decision, domainError := table.Decide(aggregate, table.Command{
		Name: table.CommandStartGame, SessionID: participants[0].SessionID, Deal: &deal,
		BoardID: "0059f493-9c83-4d08-96ce-f7f15cc734c9",
	})
	if domainError != nil {
		t.Fatalf("start setup error = %v", domainError)
	}
	aggregate = decision.NextState
	aggregate.Revision = 1
	aggregate.LastSeq = 1
	return aggregate, sessions
}

func projectedConnection(server *Server, sessionID string) *connection {
	ctx, cancel := context.WithCancel(context.Background())
	return &connection{
		session: identity.Session{ID: sessionID}, server: server, ctx: ctx, cancel: cancel,
		outbound: make(chan outboundFrame, 4),
	}
}

func projectedTableFromQueue(t *testing.T, connection *connection) table.Projection {
	t.Helper()
	for {
		frame := <-connection.outbound
		connection.releaseQueuedBytes(len(frame.message))
		var kind struct {
			Kind string `json:"kind"`
		}
		if err := json.Unmarshal(frame.message, &kind); err != nil {
			t.Fatalf("decode queued envelope: %v", err)
		}
		if kind.Kind != "event" {
			continue
		}
		var envelope eventEnvelope
		if err := json.Unmarshal(frame.message, &envelope); err != nil {
			t.Fatalf("decode event envelope: %v", err)
		}
		return envelope.Payload.Table
	}
}
