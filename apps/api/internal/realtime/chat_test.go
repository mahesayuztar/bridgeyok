package realtime

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mahesayuztar/bridgeyok/apps/api/internal/chat"
)

type blockedChat struct {
	started chan struct{}
	release chan struct{}
}

func (store *blockedChat) SendChat(ctx context.Context, sessionID string, target chat.Target, requestID, content string) (chat.Message, bool, error) {
	select {
	case store.started <- struct{}{}:
	default:
	}
	select {
	case <-ctx.Done():
		return chat.Message{}, false, ctx.Err()
	case <-store.release:
		return chat.Message{MessageID: "accepted-message", Scope: target.Scope, ConversationID: target.ID, ClientRequestID: requestID, Content: content}, false, nil
	}
}
func (*blockedChat) ChatHistory(context.Context, string, chat.Target, string, int) (chat.Page, error) {
	return chat.Page{}, nil
}
func (*blockedChat) ChatRecipientSessions(context.Context, chat.Message) ([]string, error) {
	return []string{realtimeSessionID}, nil
}

func TestChatFloodDoesNotBlockGameCommand(t *testing.T) {
	server, _, runtime := scriptedServer(t, realtimeAggregate(t), nil, "chat-flood")
	store := &blockedChat{started: make(chan struct{}, 1), release: make(chan struct{})}
	server.options.Chat = store
	httpServer := httptest.NewServer(server)
	defer httpServer.Close()
	client := mustDialScripted(t, httpServer.URL, "chat-flood")
	defer func() { _ = client.CloseNow() }()
	writeScripted(t, client, map[string]any{"v": 1, "kind": "command", "name": "table.subscribe", "request_id": "subscribe-chat", "table_id": realtimeTableID, "payload": map[string]any{"last_seen_seq": 0}})
	for _index := 0; _index < 3; _index++ {
		readScripted(t, client)
	}
	for _index := 0; _index < 50; _index++ {
		writeScripted(t, client, map[string]any{"v": 1, "kind": "command", "name": "chat.private.send", "request_id": "request-flood", "payload": map[string]any{"target": map[string]any{"scope": "private", "id": realtimeTableID}, "content": "hello"}})
	}
	select {
	case <-store.started:
	case <-time.After(time.Second):
		t.Fatal("chat worker not started")
	}
	writeScripted(t, client, map[string]any{"v": 1, "kind": "command", "name": "table.take_seat", "request_id": "game-during-chat", "table_id": realtimeTableID, "expected_revision": 0, "payload": map[string]any{"seat": "N"}})
	frame, _ := readScripted(t, client)
	if frame.Kind != "ack" || frame.Revision != 1 {
		t.Fatalf("game delayed or rejected: %v", frame)
	}
	runtime.mutex.Lock()
	revision := runtime.aggregate.Revision
	runtime.mutex.Unlock()
	if revision != 1 {
		t.Fatalf("chat altered revision: %d", revision)
	}
	close(store.release)
}

func TestChatProtocolCannotSpoofSenderOrRevision(t *testing.T) {
	base := map[string]any{"v": 1, "kind": "command", "name": "chat.private.send", "request_id": "request-valid", "payload": map[string]any{"target": map[string]any{"scope": "private", "id": realtimeTableID}, "content": "👩🏽‍💻"}}
	raw, _ := json.Marshal(base)
	if _, err := decodeClientEnvelope(raw); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"sender_user_id", "table_id", "expected_revision", "controller_epoch"} {
		t.Run(field, func(t *testing.T) {
			copy := map[string]any{}
			for key, value := range base {
				copy[key] = value
			}
			copy[field] = 1
			raw, _ := json.Marshal(copy)
			if _, err := decodeClientEnvelope(raw); err == nil {
				t.Fatal("accepted spoofed field")
			}
		})
	}
}

func TestChatPriorityQueuesReserveGameplayCapacity(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	connection := &connection{ctx: ctx, server: &Server{options: Options{OutboundQueueBytes: 4096}}, outbound: make(chan outboundFrame, 4), lifecycleOutbound: make(chan outboundFrame, 2), socialOutbound: make(chan outboundFrame, 2), presenceOutbound: make(chan outboundFrame, 2)}
	for _, encoded := range []string{`{"kind":"control","name":"presence.changed"}`, `{"kind":"control","name":"social.invite"}`, `{"kind":"control","name":"table.access_revoked"}`, `{"kind":"event","name":"table.updated"}`} {
		if !connection.enqueue(outboundFrame{message: []byte(encoded)}) {
			t.Fatal("enqueue failed")
		}
	}
	for _, name := range []string{"table.updated", "table.access_revoked", "social.invite"} {
		frame, ok := connection.takeHigherPriority(3)
		if !ok || !strings.Contains(string(frame.message), name) {
			t.Fatalf("priority order: %s", frame.message)
		}
		connection.releaseQueuedBytes(len(frame.message))
	}
	if len(connection.presenceOutbound) != 1 {
		t.Fatal("transient frame overtook game")
	}
	for _index := 0; _index < 2; _index++ {
		if !connection.enqueue(outboundFrame{message: []byte(`{"name":"chat.accepted"}`)}) {
			t.Fatal("chat queue unexpectedly full")
		}
	}
	if connection.enqueue(outboundFrame{message: []byte(`{"name":"chat.accepted"}`)}) {
		t.Fatal("unbounded chat queue")
	}
	if !connection.enqueue(outboundFrame{message: []byte(`{"name":"game.play_card"}`)}) {
		t.Fatal("chat consumed game capacity")
	}
}
