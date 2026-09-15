//go:build integration

package httpapi

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/bridge"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/database"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/identity"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/match"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/realtime"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/table"
)

type matchWireFrame struct {
	Kind      string          `json:"kind"`
	RequestID string          `json:"request_id"`
	TableID   string          `json:"table_id"`
	Revision  int64           `json:"revision"`
	Code      string          `json:"code"`
	Payload   json.RawMessage `json:"payload"`
}

type matchNotificationGate struct {
	server *realtime.Server
	fail   bool
}

func (gate *matchNotificationGate) RefreshTables(ctx context.Context, tables []string) error {
	if gate.fail {
		return fmt.Errorf("simulated lost post-commit notification")
	}
	return gate.server.RefreshTables(ctx, tables)
}

func TestMatchHTTPEightClientActorRecovery(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()
	postgres, err := database.Open(ctx, os.Getenv("TEST_DATABASE_URL"), 12)
	if err != nil {
		t.Fatal(err)
	}
	defer postgres.Close()
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	identities, err := identity.NewService(postgres, []byte(strings.Repeat("match-api-pepper", 3)), rand.Reader, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	accounts := make([]identity.AccountLogin, 9)
	suffix := strings.ToLower(rand.Text()[:8])
	for _index := range accounts {
		accounts[_index], err = identities.Register(ctx, fmt.Sprintf("m%d_%s", _index, suffix), "bridge-match-test-password", fmt.Sprintf("Player %d", _index), "spade")
		if err != nil {
			t.Fatal(err)
		}
	}
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		sessions := make([]string, 0, len(accounts))
		for _, account := range accounts {
			sessions = append(sessions, account.SessionID)
		}
		if _, err := postgres.Pool().Exec(cleanupCtx, "DELETE FROM bridgeyok.team_matches WHERE owner_session_id=ANY($1::uuid[])", sessions); err != nil {
			t.Error(err)
		}
		if _, err := postgres.Pool().Exec(cleanupCtx, "DELETE FROM bridgeyok.tables WHERE owner_session_id=ANY($1::uuid[])", sessions); err != nil {
			t.Error(err)
		}
		if _, err := postgres.Pool().Exec(cleanupCtx, "DELETE FROM bridgeyok.users WHERE session_id=ANY($1::uuid[])", sessions); err != nil {
			t.Error(err)
		}
		if _, err := postgres.Pool().Exec(cleanupCtx, "DELETE FROM bridgeyok.guest_sessions WHERE id=ANY($1::uuid[])", sessions); err != nil {
			t.Error(err)
		}
	}()
	processor, err := table.NewCommandProcessor(postgres, nil, logger, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	actors, err := table.NewActorRegistry(postgres, processor, table.ActorRegistryOptions{QueueCapacity: 32, IdleTimeout: time.Hour, Logger: logger, Now: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		drainCtx, drainCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer drainCancel()
		if err := actors.Drain(drainCtx); err != nil {
			t.Error(err)
		}
	}()
	realtimeServer, err := realtime.NewServer(realtime.Options{Logger: logger, AllowedOrigins: []string{"http://match.test"}, Identity: identities, Tables: actors, Events: postgres, Random: rand.Reader, Now: time.Now, ReadLimitBytes: 32768, OutboundQueueCapacity: 256, OutboundQueueBytes: 1 << 20, WriteTimeout: 3 * time.Second, PingInterval: time.Hour, PongTimeout: time.Second, MaxConnections: 16, MaxConnectionsPerSession: 3, MessageRate: 200, MessageBurst: 200, RecoveryLimit: 64})
	if err != nil {
		t.Fatal(err)
	}
	gate := &matchNotificationGate{server: realtimeServer}
	service, err := match.NewService(postgres, actors, gate, logger, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	tableService, err := table.NewService(postgres, []byte(strings.Repeat("match-table-pepper", 3)), rand.Reader, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(NewRouter(Options{Logger: logger, AllowedOrigins: []string{"http://match.test"}, Identity: identities, Match: service, Table: tableService, Realtime: realtimeServer, Replay: postgres}))
	defer server.Close()
	sendHTTP := func(_accountIndex int, method, path string, body any, wantStatus int) []byte {
		t.Helper()
		var reader io.Reader
		if body != nil {
			encoded, err := json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
			reader = bytes.NewReader(encoded)
		}
		request, err := http.NewRequestWithContext(ctx, method, server.URL+path, reader)
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Authorization", "Bearer "+accounts[_accountIndex].Token)
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		raw, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != wantStatus {
			t.Fatalf("%s %s got %d: %s", method, path, response.StatusCode, raw)
		}
		if response.Header.Get("Cache-Control") != "private, no-store" {
			t.Fatal("cacheable response")
		}
		for _, account := range accounts {
			if bytes.Contains(raw, []byte(account.SessionID)) {
				t.Fatal("session ID leaked in API")
			}
		}
		for _, field := range []string{`"source"`, `"private_state"`, `"credential"`} {
			if bytes.Contains(raw, []byte(field)) {
				t.Fatal("private field in API")
			}
		}
		return raw
	}
	decodeView := func(raw []byte) match.PublicView {
		t.Helper()
		var view match.PublicView
		if err := json.Unmarshal(raw, &view); err != nil {
			t.Fatal(err)
		}
		return view
	}
	create := match.CreateRequest{RequestID: uuid.NewString(), BoardCount: 2}
	for _index := 0; _index < 8; _index++ {
		room := match.Open
		if _index >= 4 {
			room = match.Closed
		}
		create.Assignments = append(create.Assignments, match.SeatRequest{UserID: accounts[_index].Profile.ID, Room: room, Seat: []bridge.Seat{bridge.North, bridge.East, bridge.South, bridge.West}[_index%4]})
	}
	created := decodeView(sendHTTP(0, "POST", "/v1/matches", create, 201))
	if created.Status != match.Waiting || created.ReadyCount != 0 || created.Ready {
		t.Fatal("implicit player consent")
	}
	duplicate := decodeView(sendHTTP(0, "POST", "/v1/matches", create, 201))
	if duplicate.ID != created.ID {
		t.Fatal("duplicate lobby")
	}
	conflict := create
	conflict.BoardCount = 3
	sendHTTP(0, "POST", "/v1/matches", conflict, 409)
	path := "/v1/matches/" + created.ID
	sendHTTP(8, "GET", path, nil, 404)
	sendHTTP(8, "POST", path+"/cancel", map[string]any{"expectedRevision": 0}, 404)
	sendHTTP(1, "POST", path+"/start", map[string]any{"expectedRevision": 0}, 403)
	sendHTTP(0, "POST", path+"/start", map[string]any{"expectedRevision": 0}, 409)
	views := make([]match.PublicView, 8)
	sockets := make([]*websocket.Conn, 8)
	defer func() {
		for _, socket := range sockets {
			if socket != nil {
				_ = socket.CloseNow()
			}
		}
	}()
	var matchBoards []match.Board
	wireRead := func(_index int, accept func(matchWireFrame, *table.Projection) bool) matchWireFrame {
		t.Helper()
		readCtx, readCancel := context.WithTimeout(ctx, 5*time.Second)
		defer readCancel()
		for {
			_, raw, err := sockets[_index].Read(readCtx)
			if err != nil {
				t.Fatalf("client %d: %v", _index, err)
			}
			var frame matchWireFrame
			if err := json.Unmarshal(raw, &frame); err != nil {
				t.Fatal(err)
			}
			for _, account := range accounts {
				if bytes.Contains(raw, []byte(account.SessionID)) {
					t.Fatal("wire session leak")
				}
			}
			if frame.Kind != "error" && frame.TableID != "" && frame.TableID != views[_index].TableID {
				t.Fatal("cross-room frame")
			}
			var projection *table.Projection
			if frame.Kind == "snapshot" {
				var value table.Projection
				if err := json.Unmarshal(frame.Payload, &value); err != nil {
					t.Fatal(err)
				}
				projection = &value
			}
			if frame.Kind == "event" {
				var value struct {
					Table table.Projection `json:"table"`
				}
				if err := json.Unmarshal(frame.Payload, &value); err != nil {
					t.Fatal(err)
				}
				projection = &value.Table
			}
			if projection != nil {
				if projection.ViewerSeat != views[_index].Seat {
					t.Fatal("wrong recipient seat")
				}
				if game := projection.Game; game != nil && game.Phase == bridge.PhaseAuction {
					if len(game.OwnHand) != 13 || game.FullDeal != nil || len(game.DummyHand) != 0 {
						t.Fatal("hidden-hand boundary")
					}
					if game.Board.Number < 1 || game.Board.Number > len(matchBoards) {
						t.Fatal("unexpected match board")
					}
					expected := matchBoards[game.Board.Number-1].Source.Deal.Hand(projection.ViewerSeat)
					if fmt.Sprint(game.OwnHand) != fmt.Sprint(expected) {
						t.Fatal("wrong own hand")
					}
				}
			}
			if accept(frame, projection) {
				return frame
			}
		}
	}
	wireWrite := func(_index int, message any) {
		t.Helper()
		encoded, err := json.Marshal(message)
		if err != nil {
			t.Fatal(err)
		}
		if err := sockets[_index].Write(ctx, websocket.MessageText, encoded); err != nil {
			t.Fatal(err)
		}
	}
	for _index := range sockets {
		views[_index] = decodeView(sendHTTP(_index, "GET", path, nil, 200))
		var listed []match.PublicView
		if err := json.Unmarshal(sendHTTP(_index, "GET", "/v1/matches", nil, 200), &listed); err != nil || len(listed) != 1 {
			t.Fatal("invitation discovery", err)
		}
		session, err := identities.Authenticate(ctx, accounts[_index].Token)
		if err != nil {
			t.Fatal(err)
		}
		ticket, _, err := identities.IssueTicket(ctx, session)
		if err != nil {
			t.Fatal(err)
		}
		sockets[_index], _, err = websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/v1/ws?ticket="+ticket, &websocket.DialOptions{HTTPHeader: http.Header{"Origin": []string{"http://match.test"}}})
		if err != nil {
			t.Fatal(err)
		}
		sockets[_index].SetReadLimit(1 << 20)
		requestID := uuid.NewString()
		wireWrite(_index, map[string]any{"v": 1, "kind": "command", "name": "table.subscribe", "request_id": requestID, "table_id": views[_index].TableID, "payload": map[string]any{"last_seen_seq": 0}})
		wireRead(_index, func(frame matchWireFrame, _ *table.Projection) bool {
			return frame.Kind == "ack" && frame.RequestID == requestID
		})
	}
	for _index := range sockets {
		view := decodeView(sendHTTP(_index, "GET", path, nil, 200))
		sendHTTP(_index, "POST", path+"/ready", map[string]any{"requestId": uuid.NewString(), "expectedTableRevision": view.TableRevision, "ready": true}, 200)
	}
	ready := decodeView(sendHTTP(0, "GET", path, nil, 200))
	if !ready.CanStart || ready.ReadyCount != 8 {
		t.Fatal("readiness not surfaced")
	}
	cachedOpen, err := actors.Snapshot(ctx, views[0].TableID)
	if err != nil || cachedOpen.State != table.StateWaiting {
		t.Fatal(err)
	}
	gate.fail = true
	started := decodeView(sendHTTP(0, "POST", path+"/start", map[string]any{"expectedRevision": ready.Revision}, 200))
	if !started.SyncPending || started.Status != match.Active {
		t.Fatal("commit notification failure not surfaced")
	}
	cachedOpen, err = actors.Snapshot(ctx, views[0].TableID)
	if err != nil || cachedOpen.State != table.StateWaiting {
		t.Fatal("fault did not leave stale actor")
	}
	gate.fail = false
	retried := decodeView(sendHTTP(0, "POST", path+"/start", map[string]any{"expectedRevision": ready.Revision}, 200))
	if retried.SyncPending || retried.Revision != started.Revision {
		t.Fatal("retry changed match start")
	}
	private, err := postgres.LoadMatch(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	matchBoards = private.Match.PrivateSnapshot().Boards
	for _index := range sockets {
		wireRead(_index, func(_ matchWireFrame, projection *table.Projection) bool {
			return projection != nil && projection.State == table.StateActive
		})
	}
	sendHTTP(0, "POST", path+"/cancel", map[string]any{"expectedRevision": started.Revision}, 409)
	wrongID := uuid.NewString()
	wireWrite(0, map[string]any{"v": 1, "kind": "command", "name": "table.subscribe", "request_id": wrongID, "table_id": views[4].TableID, "payload": map[string]any{"last_seen_seq": 0}})
	denied := wireRead(0, func(frame matchWireFrame, _ *table.Projection) bool { return frame.RequestID == wrongID })
	if denied.Code != "TABLE_ACCESS_DENIED" {
		t.Fatal("cross-room subscribe accepted")
	}
	wsCommand := func(_index int, name string, payload any) {
		t.Helper()
		aggregate, err := postgres.FindTable(ctx, views[_index].TableID)
		if err != nil {
			t.Fatal(err)
		}
		requestID := uuid.NewString()
		wireWrite(_index, map[string]any{"v": 1, "kind": "command", "name": name, "request_id": requestID, "table_id": aggregate.ID, "expected_revision": aggregate.Revision, "controller_epoch": aggregate.Seats[views[_index].Seat].ControllerEpoch, "payload": payload})
		frame := wireRead(_index, func(frame matchWireFrame, _ *table.Projection) bool { return frame.RequestID == requestID })
		if frame.Kind != "ack" {
			t.Fatalf("%s rejected: %s", name, frame.Code)
		}
	}
	for _index := range sockets {
		wsCommand(_index, "table.takeover", map[string]any{})
	}
	for _, _roomOffset := range []int{0, 4} {
		for _boardIndex := 0; _boardIndex < 2; _boardIndex++ {
			aggregate, err := postgres.FindTable(ctx, views[_roomOffset].TableID)
			if err != nil {
				t.Fatal(err)
			}
			if aggregate.BoardNumber != _boardIndex+1 {
				t.Fatal("wrong board progression")
			}
			for _callIndex := 0; _callIndex < 4; _callIndex++ {
				current, err := postgres.FindTable(ctx, aggregate.ID)
				if err != nil {
					t.Fatal(err)
				}
				_seatIndex := 0
				for _index, seat := range []bridge.Seat{bridge.North, bridge.East, bridge.South, bridge.West} {
					if current.Game.Turn == seat {
						_seatIndex = _index
					}
				}
				wsCommand(_roomOffset+_seatIndex, "game.make_call", map[string]any{"call": map[string]any{"kind": "PASS"}})
			}
			command := "table.next_board"
			if _boardIndex == 1 {
				command = "table.finish"
			}
			wsCommand(_roomOffset, command, map[string]any{})
		}
		if _roomOffset == 0 {
			view := decodeView(sendHTTP(4, "GET", path, nil, 200))
			if view.Status != match.Active || len(view.Comparisons) != 0 || view.OpenCompleted != 2 || view.ClosedCompleted != 0 {
				t.Fatal("slow-room isolation")
			}
		}
	}
	for _index := range sockets {
		wireRead(_index, func(_ matchWireFrame, projection *table.Projection) bool {
			return projection != nil && projection.State == table.StateFinished
		})
	}
	final := decodeView(sendHTTP(0, "GET", path, nil, 200))
	if final.Status != match.Complete || len(final.Comparisons) != 2 || final.TeamAIMP != 0 {
		t.Fatal("final comparison missing")
	}
	var group sync.WaitGroup
	failures := make(chan error, 4)
	create.RequestID = uuid.NewString()
	for _index := 0; _index < 4; _index++ {
		group.Go(func() { _, err := service.Create(ctx, accounts[0].SessionID, create); failures <- err })
	}
	group.Wait()
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	waiting := decodeView(sendHTTP(0, "POST", "/v1/matches", create, 201))
	cancelled := decodeView(sendHTTP(7, "POST", "/v1/matches/"+waiting.ID+"/cancel", map[string]any{"expectedRevision": waiting.Revision}, 200))
	if cancelled.Status != match.Cancelled || cancelled.CanCancel {
		t.Fatal("decline did not cancel")
	}
	sendHTTP(0, "POST", "/v1/matches/"+waiting.ID+"/start", map[string]any{"expectedRevision": waiting.Revision}, 409)
	again := decodeView(sendHTTP(7, "POST", "/v1/matches/"+waiting.ID+"/cancel", map[string]any{"expectedRevision": waiting.Revision}, 200))
	if again.Revision != cancelled.Revision {
		t.Fatal("duplicate cancellation")
	}
}
