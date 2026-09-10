package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"slices"

	"github.com/mahesayuztar/bridgeyok/apps/api/internal/chat"
)

type ChatRepository interface {
	chat.Repository
	ChatRecipientSessions(context.Context, chat.Message) ([]string, error)
}

type chatPayload struct {
	Target  chat.Target `json:"target"`
	Content string      `json:"content"`
}

func (connection *connection) handleChat(envelope ClientEnvelope) {
	var payload chatPayload
	if err := decodeStrict(envelope.Payload, &payload); err != nil {
		connection.sendError(envelope, "INVALID_CHAT_INPUT", false, nil, nil)
		return
	}
	repository := connection.server.options.Chat
	if repository == nil {
		connection.sendError(envelope, "CHAT_UNAVAILABLE", true, nil, nil)
		return
	}
	message, duplicate, err := repository.SendChat(connection.ctx, connection.session.ID, payload.Target, envelope.RequestID, payload.Content)
	if err != nil {
		code := "CHAT_UNAVAILABLE"
		switch {
		case errors.Is(err, chat.ErrInput):
			code = "INVALID_CHAT_INPUT"
		case errors.Is(err, chat.ErrAccess):
			code = "CHAT_ACCESS_DENIED"
		case errors.Is(err, chat.ErrConflict):
			code = "CHAT_REQUEST_CONFLICT"
		}
		connection.sendError(envelope, code, code == "CHAT_UNAVAILABLE", nil, nil)
		connection.server.options.Logger.InfoContext(connection.ctx, "chat_send_rejected", "result_code", code)
		return
	}
	connection.sendControl("chat.accepted", envelope.RequestID, "", map[string]any{"message": message, "duplicate": duplicate})
	if duplicate {
		return
	}
	sessions, err := repository.ChatRecipientSessions(connection.ctx, message)
	if err != nil {
		connection.server.options.Logger.WarnContext(connection.ctx, "chat_delivery_failed", "result_code", "RECIPIENT_LOOKUP_FAILED")
		return
	}
	connection.server.publishChat(sessions, message)
}

func (server *Server) publishChat(sessions []string, message chat.Message) {
	frame, err := json.Marshal(controlEnvelope{Version: 1, Kind: "control", Name: "chat." + message.Scope + ".received", Payload: map[string]any{"message": message}})
	if err != nil {
		return
	}
	server.mutex.Lock()
	recipients := []*connection{}
	for client := range server.connections {
		if slices.Contains(sessions, client.session.ID) {
			recipients = append(recipients, client)
		}
	}
	server.mutex.Unlock()
	for _, client := range recipients {
		if !client.enqueue(outboundFrame{message: frame}) {
			client.closeSlowConsumer()
		}
	}
}

func (connection *connection) chatLoop() {
	defer close(connection.socialDone)
	for {
		select {
		case <-connection.ctx.Done():
			return
		case envelope := <-connection.socialInbound:
			select {
			case connection.server.chatSlots <- struct{}{}:
			case <-connection.ctx.Done():
				return
			}
			if err := connection.validateIdentity(); err != nil {
				<-connection.server.chatSlots
				connection.sendError(envelope, "SESSION_INACTIVE", false, nil, nil)
				continue
			}
			connection.handleChat(envelope)
			<-connection.server.chatSlots
		}
	}
}

func (server *Server) NotifySession(sessionID, eventID, name string, payload map[string]any) {
	payload["eventId"] = eventID
	frame, err := json.Marshal(controlEnvelope{Version: 1, Kind: "control", Name: "social." + name, Payload: payload})
	if err != nil {
		return
	}
	server.mutex.Lock()
	recipients := []*connection{}
	for client := range server.connections {
		if client.session.ID == sessionID {
			recipients = append(recipients, client)
		}
	}
	server.mutex.Unlock()
	for _, client := range recipients {
		if !client.enqueue(outboundFrame{message: frame}) {
			client.closeSlowConsumer()
		}
	}
}
