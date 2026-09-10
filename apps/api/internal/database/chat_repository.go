package database

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/chat"
)

func authorizeChat(ctx context.Context, tx pgx.Tx, sessionID string, target chat.Target) (string, string, error) {
	var userID string
	if err := tx.QueryRow(ctx, `SELECT id::text FROM bridgeyok.users WHERE session_id=$1`, sessionID).Scan(&userID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", chat.ErrAccess
		}
		return "", "", err
	}
	if target.Scope == "private" {
		rows, err := tx.Query(ctx, `SELECT follower_id FROM bridgeyok.follows WHERE (follower_id=$1 AND followed_id=$2) OR (follower_id=$2 AND followed_id=$1) FOR SHARE`, userID, target.ID)
		if err != nil {
			return "", "", err
		}
		count := 0
		for rows.Next() {
			count++
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return "", "", err
		}
		if count != 2 || userID == target.ID {
			return "", "", chat.ErrAccess
		}
		pair := userID + ":" + target.ID
		if userID > target.ID {
			pair = target.ID + ":" + userID
		}
		return userID, pair, nil
	}
	var participantID string
	err := tx.QueryRow(ctx, `SELECT p.id::text FROM bridgeyok.table_participants p JOIN bridgeyok.tables t ON t.id=p.table_id WHERE p.table_id=$1 AND p.session_id=$2 AND p.left_at IS NULL AND t.state IN ('WAITING','ACTIVE','BETWEEN_BOARDS') FOR SHARE OF p,t`, target.ID, sessionID).Scan(&participantID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", chat.ErrAccess
	}
	return userID, target.ID, err
}

func (postgres *Postgres) SendChat(ctx context.Context, sessionID string, target chat.Target, requestID, content string) (chat.Message, bool, error) {
	if err := chat.Validate(target, requestID, content); err != nil {
		return chat.Message{}, false, err
	}
	tx, err := postgres.pool.Begin(ctx)
	if err != nil {
		return chat.Message{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	userID, conversationID, err := authorizeChat(ctx, tx, sessionID, target)
	if err != nil {
		return chat.Message{}, false, err
	}
	var tableID, recipientID *string
	if target.Scope == "private" {
		recipientID = &target.ID
	} else {
		tableID = &target.ID
	}
	result, err := tx.Exec(ctx, `INSERT INTO bridgeyok.chat_messages(message_id,scope,conversation_id,table_id,sender_user_id,recipient_user_id,content,client_request_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT(sender_user_id,client_request_id) DO NOTHING`, uuid.NewString(), target.Scope, conversationID, tableID, userID, recipientID, content, requestID)
	if err != nil {
		return chat.Message{}, false, err
	}
	var message chat.Message
	err = tx.QueryRow(ctx, `SELECT m.message_id::text,m.scope,m.conversation_id,m.sender_user_id::text,m.content,m.client_request_id,m.created_at,u.username,u.display_name,u.avatar FROM bridgeyok.chat_messages m JOIN bridgeyok.users u ON u.id=m.sender_user_id WHERE m.sender_user_id=$1 AND m.client_request_id=$2`, userID, requestID).Scan(&message.MessageID, &message.Scope, &message.ConversationID, &message.SenderUserID, &message.Content, &message.ClientRequestID, &message.CreatedAt, &message.Sender.Username, &message.Sender.DisplayName, &message.Sender.Avatar)
	if err != nil {
		return chat.Message{}, false, err
	}
	message.Sender.ID = message.SenderUserID
	if message.Scope != target.Scope || message.ConversationID != conversationID || message.Content != content {
		return chat.Message{}, false, chat.ErrConflict
	}
	return message, result.RowsAffected() == 0, tx.Commit(ctx)
}

type chatCursor struct {
	CreatedAt time.Time `json:"createdAt"`
	MessageID string    `json:"messageId"`
}

func (postgres *Postgres) ChatHistory(ctx context.Context, sessionID string, target chat.Target, cursor string, limit int) (chat.Page, error) {
	if err := chat.Validate(target, "history-request", "history"); err != nil || limit < 1 || limit > 100 {
		return chat.Page{}, chat.ErrInput
	}
	before := chatCursor{CreatedAt: time.Now().UTC().Add(time.Hour), MessageID: "ffffffff-ffff-ffff-ffff-ffffffffffff"}
	if len(cursor) > 512 {
		return chat.Page{}, chat.ErrInput
	}
	if cursor != "" {
		raw, err := base64.RawURLEncoding.DecodeString(cursor)
		if err != nil || len(raw) > 256 || json.Unmarshal(raw, &before) != nil || before.CreatedAt.IsZero() {
			return chat.Page{}, chat.ErrInput
		}
		if _, err := uuid.Parse(before.MessageID); err != nil {
			return chat.Page{}, chat.ErrInput
		}
	}
	tx, err := postgres.pool.Begin(ctx)
	if err != nil {
		return chat.Page{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	_, conversationID, err := authorizeChat(ctx, tx, sessionID, target)
	if err != nil {
		return chat.Page{}, err
	}
	days := 14
	if target.Scope == "private" {
		days = 90
	}
	rows, err := tx.Query(ctx, `SELECT m.message_id::text,m.scope,m.conversation_id,m.sender_user_id::text,m.content,m.client_request_id,m.created_at,u.username,u.display_name,u.avatar FROM bridgeyok.chat_messages m JOIN bridgeyok.users u ON u.id=m.sender_user_id WHERE m.scope=$1 AND m.conversation_id=$2 AND m.created_at >= now()-make_interval(days=>$3) AND (m.created_at,m.message_id)<($4,$5::uuid) ORDER BY m.created_at DESC,m.message_id DESC LIMIT $6`, target.Scope, conversationID, days, before.CreatedAt, before.MessageID, limit+1)
	if err != nil {
		return chat.Page{}, err
	}
	page := chat.Page{Messages: []chat.Message{}}
	for rows.Next() {
		var message chat.Message
		if err := rows.Scan(&message.MessageID, &message.Scope, &message.ConversationID, &message.SenderUserID, &message.Content, &message.ClientRequestID, &message.CreatedAt, &message.Sender.Username, &message.Sender.DisplayName, &message.Sender.Avatar); err != nil {
			rows.Close()
			return chat.Page{}, err
		}
		message.Sender.ID = message.SenderUserID
		page.Messages = append(page.Messages, message)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return chat.Page{}, err
	}
	if len(page.Messages) > limit {
		page.Messages = page.Messages[:limit]
		last := page.Messages[limit-1]
		raw, err := json.Marshal(chatCursor{CreatedAt: last.CreatedAt, MessageID: last.MessageID})
		if err != nil {
			return chat.Page{}, err
		}
		page.NextCursor = base64.RawURLEncoding.EncodeToString(raw)
	}
	return page, tx.Commit(ctx)
}

func (postgres *Postgres) ChatRecipientSessions(ctx context.Context, message chat.Message) ([]string, error) {
	var rows pgx.Rows
	var err error
	if message.Scope == "private" {
		rows, err = postgres.pool.Query(ctx, `SELECT session_id::text FROM bridgeyok.users WHERE id::text=ANY($1::text[])`, strings.Split(message.ConversationID, ":"))
	} else {
		rows, err = postgres.pool.Query(ctx, `SELECT session_id::text FROM bridgeyok.table_participants WHERE table_id=$1 AND left_at IS NULL AND session_id IS NOT NULL`, message.ConversationID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	sessions := []string{}
	for rows.Next() {
		var sessionID string
		if err := rows.Scan(&sessionID); err != nil {
			return nil, err
		}
		sessions = append(sessions, sessionID)
	}
	return sessions, rows.Err()
}
