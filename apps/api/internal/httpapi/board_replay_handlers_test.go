package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mahesayuztar/bridgeyok/apps/api/internal/analysis"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/bridge"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/httpapi/apigen"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/table"
)

type boardReplayFake struct {
	replay    table.BoardReplay
	err       error
	sessionID string
}

func (service *boardReplayFake) CompletedBoardReplay(_ context.Context, boardID, sessionID string) (table.BoardReplay, error) {
	service.sessionID = sessionID
	replay := service.replay
	replay.BoardID = boardID
	return replay, service.err
}

func TestBoardReplayAuthorizationAndFailures(t *testing.T) {
	fullDeal, err := bridge.GenerateDeal(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	state, err := bridge.NewBoard(1, fullDeal)
	if err != nil {
		t.Fatal(err)
	}
	for _callIndex := 0; _callIndex < 4; _callIndex++ {
		decision, domainError := bridge.Decide(state, bridge.MakeCallCommand(state.Turn, bridge.Pass()))
		if domainError != nil {
			t.Fatal(domainError)
		}
		state = decision.NextState
	}

	for _, test := range []struct {
		name, token, code string
		err               error
		status            int
	}{
		{name: "completed", token: "valid-access", status: 200},
		{name: "unauthenticated", token: "invalid", status: 401},
		{name: "outsider", token: "valid-access", err: analysis.ErrNotFound, status: 404, code: "BOARD_NOT_FOUND"},
		{name: "unfinished", token: "valid-access", err: analysis.ErrNotCompleted, status: 409, code: "BOARD_NOT_COMPLETED"},
		{name: "corrupt archive", token: "valid-access", err: errors.New("private deal"), status: 503, code: "REPLAY_UNAVAILABLE"},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := &boardReplayFake{err: test.err, replay: table.BoardReplay{FullDeal: fullDeal, Game: state}}
			router := NewRouter(Options{Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Identity: &identityServiceFake{}, Replay: service})
			request := httptest.NewRequest(http.MethodGet, "/v1/boards/"+testTableID+"/replay", nil)
			request.Header.Set("Authorization", "Bearer "+test.token)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != test.status || !strings.Contains(response.Body.String(), test.code) {
				t.Fatalf("response: %d %s", response.Code, response.Body.String())
			}
			if test.status == http.StatusOK {
				var generated apigen.BoardReplay
				decoder := json.NewDecoder(strings.NewReader(response.Body.String()))
				decoder.DisallowUnknownFields()
				if err := decoder.Decode(&generated); err != nil {
					t.Fatal(err)
				}
				if generated.BoardId.String() != testTableID || !generated.Game.Result.PassedOut || !generated.Game.Phase.Valid() {
					t.Fatal("response violates replay contract")
				}
			}
			if response.Header().Get("Cache-Control") != "private, no-store" {
				t.Fatal("replay must not be cached")
			}
			if test.status == 401 && service.sessionID != "" {
				t.Fatal("unauthenticated repository access")
			}
			if test.status != 401 && service.sessionID != testSessionID {
				t.Fatal("wrong participant authorization")
			}
			if strings.Contains(response.Body.String(), "private deal") {
				t.Fatal("private failure details leaked")
			}
		})
	}
}
