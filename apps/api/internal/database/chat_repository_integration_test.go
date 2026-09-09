//go:build integration

package database

import (
	"context"
	"crypto/rand"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/chat"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/identity"
)

func TestChatAuthorizationPersistenceAndRetention(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	postgres, err := Open(ctx, os.Getenv("TEST_DATABASE_URL"), 4)
	if err != nil {
		t.Fatal(err)
	}
	defer postgres.Close()
	service, err := identity.NewService(postgres, []byte(strings.Repeat("chat-test-", 4)), rand.Reader, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	accounts := []identity.AccountLogin{}
	for _, name := range []string{"alice", "bob", "eve"} {
		account, err := service.Register(ctx, name+strings.ToLower(rand.Text()[:8]), "correct horse bridge", name, "heart")
		if err != nil {
			t.Fatal(err)
		}
		accounts = append(accounts, account)
	}
	alice, bob, eve := accounts[0], accounts[1], accounts[2]
	defer func() {
		for _, account := range accounts {
			_, _ = postgres.pool.Exec(context.Background(), `DELETE FROM bridgeyok.users WHERE id=$1`, account.Profile.ID)
			_, _ = postgres.pool.Exec(context.Background(), `DELETE FROM bridgeyok.guest_sessions WHERE id=$1`, account.SessionID)
		}
	}()
	target := chat.Target{Scope: "private", ID: bob.Profile.ID}
	if _, _, err := postgres.SendChat(ctx, alice.SessionID, target, "request-001", "hello"); !errors.Is(err, chat.ErrAccess) {
		t.Fatalf("non friend: %v", err)
	}
	if err := postgres.Follow(ctx, alice.Profile.ID, bob.Profile.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, _, err := postgres.SendChat(ctx, alice.SessionID, target, "request-001", "hello"); !errors.Is(err, chat.ErrAccess) {
		t.Fatalf("one way follow: %v", err)
	}
	if err := postgres.Follow(ctx, bob.Profile.ID, alice.Profile.ID, true); err != nil {
		t.Fatal(err)
	}
	message, duplicate, err := postgres.SendChat(ctx, alice.SessionID, target, "request-001", "👩🏽‍💻 ♠️")
	if err != nil || duplicate {
		t.Fatalf("offline send: %v", err)
	}
	retry, duplicate, err := postgres.SendChat(ctx, alice.SessionID, target, "request-001", message.Content)
	if err != nil || !duplicate || retry.MessageID != message.MessageID {
		t.Fatalf("duplicate: %v", err)
	}
	if _, _, err := postgres.SendChat(ctx, alice.SessionID, target, "request-001", "changed"); !errors.Is(err, chat.ErrConflict) {
		t.Fatalf("identity reuse: %v", err)
	}
	page, err := postgres.ChatHistory(ctx, bob.SessionID, chat.Target{Scope: "private", ID: alice.Profile.ID}, "", 1)
	if err != nil || len(page.Messages) != 1 || page.Messages[0].MessageID != message.MessageID {
		t.Fatalf("offline history: %+v %v", page, err)
	}
	if _, err := postgres.ChatHistory(ctx, eve.SessionID, target, "", 50); !errors.Is(err, chat.ErrAccess) {
		t.Fatalf("private isolation: %v", err)
	}
	_, _, err = postgres.SendChat(ctx, alice.SessionID, target, "request-002", "second")
	if err != nil {
		t.Fatal(err)
	}
	page, err = postgres.ChatHistory(ctx, alice.SessionID, target, "", 1)
	if err != nil || page.NextCursor == "" {
		t.Fatalf("pagination: %+v %v", page, err)
	}
	older, err := postgres.ChatHistory(ctx, alice.SessionID, target, page.NextCursor, 1)
	if err != nil || len(older.Messages) != 1 || older.Messages[0].MessageID != message.MessageID {
		t.Fatalf("older: %+v %v", older, err)
	}
	session, err := service.Authenticate(ctx, alice.Token)
	if err != nil {
		t.Fatal(err)
	}
	tableID := uuid.NewString()
	_, err = postgres.pool.Exec(ctx, `INSERT INTO bridgeyok.tables(id,owner_session_id,invite_code_hash,state,created_at,meaningful_at) VALUES($1,$2,$3,'WAITING',now(),now())`, tableID, session.ID, []byte(strings.Repeat("x", 32)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = postgres.pool.Exec(context.Background(), `DELETE FROM bridgeyok.tables WHERE id=$1`, tableID)
	}()
	_, err = postgres.pool.Exec(ctx, `INSERT INTO bridgeyok.table_participants(id,table_id,session_id,role,joined_at) VALUES($1,$2,$3,'OWNER',now())`, uuid.NewString(), tableID, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	tableTarget := chat.Target{Scope: "table", ID: tableID}
	tableMessage, _, err := postgres.SendChat(ctx, alice.SessionID, tableTarget, "table-request-01", "table hello")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := postgres.SendChat(ctx, bob.SessionID, tableTarget, "table-request-02", "leak"); !errors.Is(err, chat.ErrAccess) {
		t.Fatalf("table send isolation: %v", err)
	}
	if _, err := postgres.ChatHistory(ctx, bob.SessionID, tableTarget, "", 50); !errors.Is(err, chat.ErrAccess) {
		t.Fatalf("table history isolation: %v", err)
	}
	_, err = postgres.pool.Exec(ctx, `UPDATE bridgeyok.chat_messages SET created_at=now()-CASE WHEN scope='private' THEN interval '91 days' ELSE interval '15 days' END WHERE message_id IN ($1,$2)`, message.MessageID, tableMessage.MessageID)
	if err != nil {
		t.Fatal(err)
	}
	page, err = postgres.ChatHistory(ctx, alice.SessionID, target, "", 50)
	if err != nil || len(page.Messages) != 1 {
		t.Fatalf("retention before cron: %+v %v", page, err)
	}
	if _, err = postgres.pool.Exec(ctx, `SELECT bridgeyok.cleanup_chat_messages()`); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = postgres.pool.QueryRow(ctx, `SELECT count(*) FROM bridgeyok.chat_messages WHERE message_id IN ($1,$2)`, message.MessageID, tableMessage.MessageID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("cleanup: %d %v", count, err)
	}
}
