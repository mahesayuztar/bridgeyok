//go:build integration

package httpapi

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"reflect"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/bridge"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/database"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/identity"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/match"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/table"
)

func TestMatchProcessHardening(t *testing.T) {
	binary, container := os.Getenv("PHASE5_TEST_API_BINARY"), os.Getenv("PHASE5_DATABASE_CONTAINER")
	if binary == "" || !strings.HasPrefix(container, "bridgeyok-hardening-") {
		t.Skip("run scripts/harden-phase5.sh with its disposable database")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
	defer cancel()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	secret := strings.Repeat("phase5-process-test-", 3)
	logFile, err := os.CreateTemp(t.TempDir(), "api-log")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = logFile.Close() }()
	var process *exec.Cmd
	stop := func(signal os.Signal) {
		t.Helper()
		if process == nil {
			return
		}
		if err := process.Process.Signal(signal); err != nil {
			t.Fatal(err)
		}
		err := process.Wait()
		if signal == syscall.SIGTERM && err != nil {
			t.Fatal("graceful shutdown", err)
		}
		process = nil
	}
	defer func() {
		if process != nil {
			_ = process.Process.Kill()
			_ = process.Wait()
		}
	}()
	waitReady := func() {
		t.Helper()
		deadline := time.Now().Add(20 * time.Second)
		for time.Now().Before(deadline) {
			requestCtx, done := context.WithTimeout(ctx, time.Second)
			request, err := http.NewRequestWithContext(requestCtx, "GET", baseURL+"/health/ready", nil)
			if err != nil {
				done()
				t.Fatal(err)
			}
			response, err := http.DefaultClient.Do(request)
			ready := false
			if err == nil {
				ready = response.StatusCode == 200
				_ = response.Body.Close()
			}
			done()
			if ready {
				return
			}
			select {
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			case <-time.After(100 * time.Millisecond):
			}
		}
		t.Fatal("API did not become ready")
	}
	start := func(executable string) {
		t.Helper()
		process = exec.CommandContext(ctx, executable)
		process.Env = append(os.Environ(), "DATABASE_URL="+os.Getenv("TEST_DATABASE_URL"), "AUTH_SECRET="+secret, "APP_ENV=test", "API_HOST=127.0.0.1", fmt.Sprintf("PORT=%d", port), "ALLOWED_ORIGINS=http://hardening.test", "DATABASE_MAX_CONNS=12", "REALTIME_MAX_CONNECTIONS=64", "REALTIME_MESSAGE_RATE=200", "REALTIME_MESSAGE_BURST=200")
		process.Stdout, process.Stderr = logFile, logFile
		if err := process.Start(); err != nil {
			t.Fatal(err)
		}
		waitReady()
	}
	start(binary)
	postgres, err := database.Open(ctx, os.Getenv("TEST_DATABASE_URL"), 12)
	if err != nil {
		t.Fatal(err)
	}
	defer postgres.Close()
	identities, err := identity.NewService(postgres, []byte(secret), rand.Reader, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	accounts := make([]identity.AccountLogin, 32)
	views := make([]match.PublicView, 32)
	sockets := make([]*websocket.Conn, 32)
	closeSockets := func() {
		for _index, socket := range sockets {
			if socket != nil {
				_ = socket.CloseNow()
				sockets[_index] = nil
			}
		}
	}
	defer closeSockets()
	suffix := strings.ToLower(rand.Text()[:8])
	for _index := range accounts {
		accounts[_index], err = identities.Register(ctx, fmt.Sprintf("proc%d_%s", _index, suffix), "hardening-process-password", fmt.Sprintf("Process %d", _index), "spade")
		if err != nil {
			t.Fatal(err)
		}
	}
	sendHTTP := func(_index int, method, path string, input any, want int) []byte {
		t.Helper()
		body, err := json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
		request, err := http.NewRequestWithContext(ctx, method, baseURL+path, bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Authorization", "Bearer "+accounts[_index].Token)
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = response.Body.Close() }()
		raw, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != want {
			t.Fatalf("%s %s status=%d want=%d", method, path, response.StatusCode, want)
		}
		return raw
	}
	loadView := func(_index int, path string) match.PublicView {
		t.Helper()
		var view match.PublicView
		if err := json.Unmarshal(sendHTTP(_index, "GET", path, nil, 200), &view); err != nil {
			t.Fatal(err)
		}
		return view
	}
	for _matchIndex := 0; _matchIndex < 4; _matchIndex++ {
		_matchOffset := _matchIndex * 8
		request := match.CreateRequest{RequestID: uuid.NewString(), BoardCount: 2}
		for _seatIndex := 0; _seatIndex < 8; _seatIndex++ {
			room := match.Open
			if _seatIndex >= 4 {
				room = match.Closed
			}
			request.Assignments = append(request.Assignments, match.SeatRequest{UserID: accounts[_matchOffset+_seatIndex].Profile.ID, Room: room, Seat: []bridge.Seat{bridge.North, bridge.East, bridge.South, bridge.West}[_seatIndex%4]})
		}
		var created match.PublicView
		if err := json.Unmarshal(sendHTTP(_matchOffset, "POST", "/v1/matches", request, 201), &created); err != nil {
			t.Fatal(err)
		}
		path := "/v1/matches/" + created.ID
		for _seatIndex := 0; _seatIndex < 8; _seatIndex++ {
			view := loadView(_matchOffset+_seatIndex, path)
			sendHTTP(_matchOffset+_seatIndex, "POST", path+"/ready", map[string]any{"requestId": uuid.NewString(), "expectedTableRevision": view.TableRevision, "ready": true}, 200)
		}
		ready := loadView(_matchOffset, path)
		sendHTTP(_matchOffset, "POST", path+"/start", map[string]any{"expectedRevision": ready.Revision}, 200)
		for _seatIndex := 0; _seatIndex < 8; _seatIndex++ {
			views[_matchOffset+_seatIndex] = loadView(_matchOffset+_seatIndex, path)
		}
	}
	t.Log("32 accounts and four active two-board matches prepared")
	sources := make(map[string][]match.Board, 4)
	for _index := 0; _index < len(views); _index += 8 {
		stored, err := postgres.LoadMatch(ctx, views[_index].ID)
		if err != nil {
			t.Fatal(err)
		}
		sources[views[_index].ID] = stored.Match.PrivateSnapshot().Boards
	}
	var latencies []time.Duration
	var timingMutex sync.Mutex
	wireCommand := func(_index int, name string, payload any) {
		t.Helper()
		aggregate, err := postgres.FindTable(ctx, views[_index].TableID)
		if err != nil {
			t.Fatal(err)
		}
		requestID := uuid.NewString()
		envelope := map[string]any{"v": 1, "kind": "command", "name": name, "request_id": requestID, "table_id": aggregate.ID, "payload": payload}
		if name != "table.subscribe" {
			envelope["expected_revision"] = aggregate.Revision
			if epoch := aggregate.Seats[views[_index].Seat].ControllerEpoch; epoch > 0 {
				envelope["controller_epoch"] = epoch
			}
		}
		encoded, err := json.Marshal(envelope)
		if err != nil {
			t.Fatal(err)
		}
		commandCtx, done := context.WithTimeout(ctx, 5*time.Second)
		defer done()
		started := time.Now()
		if err := sockets[_index].Write(commandCtx, websocket.MessageText, encoded); err != nil {
			t.Fatal(err)
		}
		for {
			_, raw, err := sockets[_index].Read(commandCtx)
			if err != nil {
				t.Fatal(err)
			}
			var frame matchWireFrame
			if err := json.Unmarshal(raw, &frame); err != nil {
				t.Fatal(err)
			}
			if frame.Kind == "error" {
				t.Fatalf("client %d %s rejected: %s", _index, name, frame.Code)
			}
			if frame.Kind != "error" && frame.TableID != "" && frame.TableID != aggregate.ID {
				t.Fatal("cross-room frame")
			}
			for _, account := range accounts {
				if bytes.Contains(raw, []byte(account.Token)) || bytes.Contains(raw, []byte(account.SessionID)) {
					t.Fatal("private identity in frame")
				}
			}
			var projection *table.Projection
			switch frame.Kind {
			case "snapshot":
				if err := json.Unmarshal(frame.Payload, &projection); err != nil {
					t.Fatal(err)
				}
			case "event":
				var event struct {
					Table *table.Projection `json:"table"`
				}
				if err := json.Unmarshal(frame.Payload, &event); err != nil {
					t.Fatal(err)
				}
				projection = event.Table
			}
			if projection != nil {
				if projection.ViewerSeat != views[_index].Seat || projection.MatchID != views[_index].ID {
					t.Fatal("wrong recovered recipient")
				}
				if game := projection.Game; game != nil && game.Phase == bridge.PhaseAuction {
					boards := sources[views[_index].ID]
					if game.Board.Number < 1 || game.Board.Number > len(boards) || game.FullDeal != nil || len(game.DummyHand) != 0 {
						t.Fatal("recovered hidden-hand boundary")
					}
					expected := boards[game.Board.Number-1].Source.Deal.Hand(projection.ViewerSeat)
					if !reflect.DeepEqual(game.OwnHand, expected) {
						t.Fatal("recovered hand differs from immutable source")
					}
				}
			}
			if frame.RequestID == requestID {
				if frame.Kind != "ack" {
					t.Fatalf("%s: %s", name, frame.Code)
				}
				timingMutex.Lock()
				latencies = append(latencies, time.Since(started))
				timingMutex.Unlock()
				return
			}
		}
	}
	connect := func() {
		t.Helper()
		for _index := range accounts {
			var ticket struct {
				Ticket string `json:"ticket"`
			}
			if err := json.Unmarshal(sendHTTP(_index, "POST", "/v1/realtime/tickets", map[string]any{}, 201), &ticket); err != nil {
				t.Fatal(err)
			}
			socket, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(baseURL, "http")+"/v1/ws?ticket="+ticket.Ticket, &websocket.DialOptions{HTTPHeader: http.Header{"Origin": []string{"http://hardening.test"}}})
			if err != nil {
				t.Fatal(err)
			}
			sockets[_index] = socket
			socket.SetReadLimit(1 << 20)
			wireCommand(_index, "table.subscribe", map[string]any{"last_seen_seq": 0})
			wireCommand(_index, "table.takeover", map[string]any{})
		}
	}
	connect()
	for _roomIndex := 0; _roomIndex < 8; _roomIndex++ {
		wireCommand(_roomIndex*4, "game.make_call", map[string]any{"call": map[string]any{"kind": "PASS"}})
	}
	before := make([]table.Aggregate, 8)
	for _roomIndex := range before {
		before[_roomIndex], err = postgres.FindTable(ctx, views[_roomIndex*4].TableID)
		if err != nil {
			t.Fatal(err)
		}
	}
	t.Log("32 sockets connected; crashing API after one call in every room")
	stop(syscall.SIGKILL)
	closeSockets()
	start(binary)
	for _roomIndex, previous := range before {
		recovered, err := postgres.FindTable(ctx, previous.ID)
		if err != nil || !reflect.DeepEqual(previous, recovered) {
			t.Fatal("restart changed durable table", _roomIndex, err)
		}
	}
	connect()
	if output, err := exec.CommandContext(ctx, "docker", "stop", "-t", "1", container).CombinedOutput(); err != nil {
		t.Fatalf("stop isolated database: %v %s", err, output)
	}
	request, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/health/ready", nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != 503 {
		t.Fatal("database outage readiness", response.StatusCode)
	}
	request, err = http.NewRequestWithContext(ctx, "POST", baseURL+"/v1/matches/"+views[0].ID+"/start", strings.NewReader(`{"expectedRevision":0}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+accounts[0].Token)
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusServiceUnavailable {
		t.Fatal("outage mutation did not fail closed", response.StatusCode)
	}
	if output, err := exec.CommandContext(ctx, "docker", "start", container).CombinedOutput(); err != nil {
		t.Fatalf("start isolated database: %v %s", err, output)
	}
	waitReady()
	postgres.Pool().Reset()
	t.Log("database recovered; reconnecting clients with a fresh inspection pool")
	closeSockets()
	connect()
	var group sync.WaitGroup
	for _roomIndex := 0; _roomIndex < 8; _roomIndex++ {
		_roomOffset := _roomIndex * 4
		group.Go(func() {
			for {
				aggregate, err := postgres.FindTable(ctx, views[_roomOffset].TableID)
				if err != nil {
					t.Error(err)
					return
				}
				if aggregate.State == table.StateFinished {
					return
				}
				if aggregate.Game.Phase == bridge.PhaseAuction {
					_seatIndex := slices.Index([]bridge.Seat{bridge.North, bridge.East, bridge.South, bridge.West}, aggregate.Game.Turn)
					wireCommand(_roomOffset+_seatIndex, "game.make_call", map[string]any{"call": map[string]any{"kind": "PASS"}})
				} else {
					name := "table.next_board"
					if aggregate.BoardNumber == 2 {
						name = "table.finish"
					}
					wireCommand(_roomOffset, name, map[string]any{})
				}
			}
		})
	}
	group.Wait()
	finals := make([]match.PublicView, 4)
	for _index := range finals {
		finals[_index] = loadView(_index*8, "/v1/matches/"+views[_index*8].ID)
		if finals[_index].Status != match.Complete || len(finals[_index].Comparisons) != 2 || finals[_index].TeamAIMP != 0 {
			t.Fatal("final result missing")
		}
	}
	stop(syscall.SIGTERM)
	closeSockets()
	rollback := os.Getenv("PHASE5_ROLLBACK_BINARY")
	if rollback == "" {
		t.Fatal("rollback binary required by hardening runner")
	}
	start(rollback)
	for _index, final := range finals {
		recovered := loadView(_index*8, "/v1/matches/"+final.ID)
		if !reflect.DeepEqual(final, recovered) {
			t.Fatal("restart changed final match")
		}
	}
	stop(syscall.SIGTERM)
	logs, err := os.ReadFile(logFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	for _, account := range accounts {
		if bytes.Contains(logs, []byte(account.Token)) || bytes.Contains(logs, []byte(account.SessionID)) {
			t.Fatal("private identity in process logs")
		}
	}
	if bytes.Contains(logs, []byte(secret)) || bytes.Contains(logs, []byte("DATA RACE")) {
		t.Fatal("secret or race in process logs")
	}
	slices.Sort(latencies)
	t.Logf("4 matches / 8 rooms / 32 sockets; %d ACK samples p50=%s p95=%s max=%s; SIGKILL recovery, database outage and compatible application rollback PASS", len(latencies), latencies[len(latencies)/2], latencies[len(latencies)*95/100], latencies[len(latencies)-1])
}
