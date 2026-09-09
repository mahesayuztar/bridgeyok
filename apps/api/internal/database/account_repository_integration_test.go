//go:build integration

package database

import (
	"context"
	"crypto/rand"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mahesayuztar/bridgeyok/apps/api/internal/httpapi"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/identity"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/table"
)

func TestRegisteredAccountSocialAndInviteBoundary(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	postgres, err := Open(ctx, os.Getenv("TEST_DATABASE_URL"), 4)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(postgres.Close)
	now := time.Now().UTC()
	service, err := identity.NewService(postgres, []byte(strings.Repeat("account-integration-", 2)), rand.Reader, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	suffix := strings.ToLower(rand.Text()[:8])
	alice, err := service.Register(ctx, "alice_"+suffix, "correct horse bridge", "Alice", "heart")
	if err != nil {
		t.Fatal(err)
	}
	bob, err := service.Register(ctx, "bob_"+suffix, "correct horse bridge", "Bob", "fox")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = postgres.pool.Exec(context.Background(), `DELETE FROM bridgeyok.tables WHERE owner_session_id=$1`, alice.SessionID)
		_, _ = postgres.pool.Exec(context.Background(), `DELETE FROM bridgeyok.users WHERE id IN ($1,$2)`, alice.Profile.ID, bob.Profile.ID)
		_, _ = postgres.pool.Exec(context.Background(), `DELETE FROM bridgeyok.guest_sessions WHERE id IN ($1,$2)`, alice.SessionID, bob.SessionID)
	})
	if _, err = service.Register(ctx, alice.Profile.Username, "correct horse bridge", "Alice", "spade"); !errors.Is(err, identity.ErrUsernameTaken) {
		t.Fatalf("duplicate username: %v", err)
	}
	if _, err = service.Login(ctx, alice.Profile.Username, "incorrect password"); !errors.Is(err, identity.ErrInvalidCredential) {
		t.Fatalf("wrong password: %v", err)
	}
	login, err := service.Login(ctx, strings.ToUpper(alice.Profile.Username), "correct horse bridge")
	if err != nil || login.Profile.ID != alice.Profile.ID || login.SessionID != alice.SessionID || login.Token == alice.Token {
		t.Fatalf("stable login: %+v %v", login.Profile, err)
	}
	connected, err := service.Authenticate(ctx, login.Token)
	if err != nil {
		t.Fatal(err)
	}
	ticket, _, err := service.IssueTicket(ctx, connected)
	if err != nil {
		t.Fatal(err)
	}
	connected, err = service.ConsumeTicket(ctx, ticket)
	if err != nil || len(connected.AccountTokenHash) == 0 {
		t.Fatalf("account ticket binding: %v", err)
	}
	if err = service.ValidateConnection(ctx, connected); err != nil {
		t.Fatal(err)
	}
	unusedTicket, _, err := service.IssueTicket(ctx, connected)
	if err != nil {
		t.Fatal(err)
	}

	if err = service.LogoutAccount(ctx, login.Token); err != nil {
		t.Fatal(err)
	}
	if err = service.ValidateConnection(ctx, connected); err == nil {
		t.Fatal("revoked connection remains authorized")
	}
	if _, err = service.ConsumeTicket(ctx, unusedTicket); err == nil {
		t.Fatal("revoked unused ticket remains authorized")
	}

	if _, err = service.Account(ctx, login.Token); !errors.Is(err, identity.ErrInvalidCredential) {
		t.Fatalf("revoked token: %v", err)
	}
	if _, err = service.Authenticate(ctx, alice.Token); err != nil {
		t.Fatalf("other device revoked: %v", err)
	}
	if _, err = service.SaveProfile(ctx, alice.Profile.ID, "Updated Alice", "uploaded-url"); !errors.Is(err, identity.ErrAccountInput) {
		t.Fatalf("invalid avatar: %v", err)
	}
	if _, err = service.SaveProfile(ctx, alice.Profile.ID, "Updated Alice", "owl"); err != nil {
		t.Fatal(err)
	}
	for _attempt := 0; _attempt < 2; _attempt++ {
		if err = postgres.Follow(ctx, alice.Profile.ID, bob.Profile.ID, true); err != nil {
			t.Fatal(err)
		}
	}
	users, err := postgres.SearchUsers(ctx, alice.Profile.ID, bob.Profile.Username, false, now)
	if err != nil || len(users) != 1 || !users[0].Following || users[0].Friends || users[0].Online {
		t.Fatalf("one-way follow/offline: %+v %v", users, err)
	}
	if err = postgres.Follow(ctx, bob.Profile.ID, alice.Profile.ID, true); err != nil {
		t.Fatal(err)
	}
	users, err = postgres.SearchUsers(ctx, alice.Profile.ID, "", true, now)
	if err != nil || len(users) != 1 || !users[0].Friends {
		t.Fatalf("mutual friends: %+v %v", users, err)
	}
	if err = postgres.Follow(ctx, alice.Profile.ID, alice.Profile.ID, true); !errors.Is(err, identity.ErrAccountInput) {
		t.Fatalf("self-follow: %v", err)
	}
	tables, err := table.NewService(postgres, []byte(strings.Repeat("account-integration-", 2)), rand.Reader, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	aliceSession, err := service.Authenticate(ctx, alice.Token)
	if err != nil {
		t.Fatal(err)
	}
	created, err := tables.Create(ctx, aliceSession)
	if err != nil {
		t.Fatal(err)
	}
	tableID := created.Projection.TableID
	router := httpapi.NewRouter(httpapi.Options{Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Identity: service, Accounts: postgres, Table: tables})
	request := func(method, path, token, body string) int {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		return response.Code
	}
	profilePath := "/v1/account/tables/" + tableID + "/participants"
	if status := request("GET", profilePath, bob.Token, ""); status != 403 {
		t.Fatalf("nonparticipant profile access: %d", status)
	}
	if status := request("GET", profilePath, alice.Token, ""); status != 200 {
		t.Fatalf("participant profile access: %d", status)
	}
	if status := request("PUT", "/v1/account/users/"+bob.Profile.ID+"/follow", "", ""); status != 401 {
		t.Fatalf("unauthenticated follow: %d", status)
	}
	invitePath := "/v1/account/tables/" + tableID + "/invites"
	inviteBody := `{"userId":"` + bob.Profile.ID + `","inviteCode":"` + created.InviteCode + `"}`
	if status := request("POST", invitePath, alice.Token, inviteBody); status != 409 {
		t.Fatalf("offline invite: %d", status)
	}
	if err = service.AccountHeartbeat(ctx, bob.Token); err != nil {
		t.Fatal(err)
	}
	for _attempt := 0; _attempt < 2; _attempt++ {
		if status := request("POST", invitePath, alice.Token, inviteBody); status != 204 {
			t.Fatalf("online invite: %d", status)
		}
	}
	inbox, err := postgres.Invitations(ctx, bob.Profile.ID, now)
	if err != nil || len(inbox) != 1 || inbox[0].TableID != tableID {
		t.Fatalf("deduplicated inbox: %+v %v", inbox, err)
	}
	unchanged, err := tables.Get(ctx, tableID, aliceSession)
	if err != nil || unchanged.Revision != created.Projection.Revision || len(unchanged.Participants) != 1 || len(unchanged.Seats) != 0 {
		t.Fatalf("invite changed gameplay: %+v %v", unchanged, err)
	}
	users, err = postgres.SearchUsers(ctx, alice.Profile.ID, bob.Profile.Username, false, now.Add(46*time.Second))
	if err != nil || users[0].Online {
		t.Fatalf("expired lease: %+v %v", users, err)
	}
	for _attempt := 0; _attempt < 2; _attempt++ {
		if err = postgres.Follow(ctx, alice.Profile.ID, bob.Profile.ID, false); err != nil {
			t.Fatal(err)
		}
	}
	users, err = postgres.SearchUsers(ctx, bob.Profile.ID, alice.Profile.Username, false, now)
	if err != nil || !users[0].Following || users[0].Friends {
		t.Fatalf("unfollow semantics: %+v %v", users, err)
	}
	if status := request(http.MethodDelete, "/v1/guest-sessions/current", alice.Token, ""); status != 403 {
		t.Fatalf("account can revoke stable table identity: %d", status)
	}
}
