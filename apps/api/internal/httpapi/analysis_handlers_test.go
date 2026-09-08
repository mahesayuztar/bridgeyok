package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/analysis"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/bridge"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/deal"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/httpapi/apigen"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/observability"
)

type analysisRepositoryFake struct {
	board     analysis.Board
	err       error
	sessionID string
}

func (repository *analysisRepositoryFake) CompletedAnalysisBoard(_ context.Context, _ string, sessionID string) (analysis.Board, error) {
	repository.sessionID = sessionID
	return repository.board, repository.err
}

type analysisSolverFake struct {
	calls int
	err   error
}

func (solver *analysisSolverFake) Solve(_ context.Context, _ bridge.Deal, _ bridge.BoardMetadata) (analysis.Result, error) {
	solver.calls++
	return analysis.Result{SolverVersion: analysis.SolverVersion, DoubleDummyTable: map[bridge.Seat]map[bridge.Strain]int{bridge.North: {bridge.StrainClubs: 6, bridge.StrainDiamonds: 6, bridge.StrainHearts: 6, bridge.StrainSpades: 6, bridge.StrainNoTrump: 6}, bridge.East: {bridge.StrainClubs: 6, bridge.StrainDiamonds: 6, bridge.StrainHearts: 6, bridge.StrainSpades: 6, bridge.StrainNoTrump: 6}, bridge.South: {bridge.StrainClubs: 6, bridge.StrainDiamonds: 6, bridge.StrainHearts: 6, bridge.StrainSpades: 6, bridge.StrainNoTrump: 6}, bridge.West: {bridge.StrainClubs: 6, bridge.StrainDiamonds: 6, bridge.StrainHearts: 6, bridge.StrainSpades: 6, bridge.StrainNoTrump: 6}}, MakeableContracts: []analysis.MakeableContract{}, Par: analysis.Par{Contracts: []analysis.ParContract{}}}, solver.err
}

func TestAnalysisHTTPContractAuthorizationAndFailures(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name            string
		repositoryError error
		solverError     error
		token           string
		status          int
		calls           int
		code            string
	}{
		{name: "completed", token: "valid-access", status: 200, calls: 1, code: "OK"},
		{name: "unauthenticated", token: "invalid", status: 401},
		{name: "outsider", token: "valid-access", repositoryError: analysis.ErrNotFound, status: 404, code: "BOARD_NOT_FOUND"},
		{name: "active board", token: "valid-access", repositoryError: analysis.ErrNotCompleted, status: 409, code: "BOARD_NOT_COMPLETED"},
		{name: "busy", token: "valid-access", solverError: analysis.ErrBusy, status: 503, calls: 1, code: "ANALYSIS_BUSY"},
		{name: "timeout", token: "valid-access", solverError: context.DeadlineExceeded, status: 504, calls: 1, code: "ANALYSIS_TIMEOUT"},
		{name: "solver error", token: "valid-access", solverError: errors.New("secret deal SA seed 42"), status: 503, calls: 1, code: "ANALYSIS_UNAVAILABLE"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			boardID := uuid.NewString()
			repository := &analysisRepositoryFake{board: analysis.Board{ID: boardID, Metadata: bridge.BoardMetadata{Number: 1, Dealer: bridge.North, Vulnerability: bridge.VulnerabilityNone}, Source: deal.Result{Provenance: deal.Provenance{Type: "secure_random", Version: "fisher-yates-v1"}}}, err: test.repositoryError}
			solver := &analysisSolverFake{err: test.solverError}
			service, err := analysis.NewService(repository, solver)
			if err != nil {
				t.Fatal(err)
			}
			var logs bytes.Buffer
			router := NewRouter(Options{Logger: observability.NewLoggerWithWriter(slog.LevelDebug, &logs), Identity: &identityServiceFake{}, Analysis: service})
			request := httptest.NewRequest(http.MethodGet, "/v1/boards/"+boardID+"/analysis", nil)
			request.Header.Set("Authorization", "Bearer "+test.token)
			request.Header.Set("X-Request-ID", "analysis_test_01")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != test.status || solver.calls != test.calls {
				t.Fatalf("status=%d calls=%d body=%s", response.Code, solver.calls, response.Body.String())
			}
			if response.Header().Get("Cache-Control") != "private, no-store" {
				t.Fatal("analysis response can be cached")
			}
			if test.status == 200 {
				var generated apigen.BoardAnalysis
				decoder := json.NewDecoder(response.Body)
				decoder.DisallowUnknownFields()
				if err := decoder.Decode(&generated); err != nil {
					t.Fatal(err)
				}
				if generated.BoardId.String() != boardID || generated.Analysis.SolverVersion != analysis.SolverVersion || generated.Analysis.Par.ScoreNS != 0 {
					t.Fatal("analysis response violates generated contract")
				}
				if repository.sessionID != testSessionID {
					t.Fatal("repository authorization used wrong session")
				}
			} else if test.code != "" && !strings.Contains(response.Body.String(), test.code) {
				t.Fatal("missing actionable analysis error")
			}
			if strings.Contains(logs.String()+response.Body.String(), "secret deal") || strings.Contains(logs.String(), "valid-access") {
				t.Fatal("solver or credential secret leaked")
			}
			if test.code != "" && (!strings.Contains(logs.String(), "board_analysis_processed") || !strings.Contains(logs.String(), "analysis_test_01") || !strings.Contains(logs.String(), test.code)) {
				t.Fatal("missing correlated analysis telemetry")
			}
		})
	}
}

type positionServiceFake struct {
	key       string
	step      *int
	sessionID string
	err       error
}

func (service *positionServiceFake) Analyze(context.Context, string, string) (analysis.Response, error) {
	return analysis.Response{}, errors.New("unexpected deal analysis")
}

func (service *positionServiceFake) AnalyzePosition(_ context.Context, boardID, sessionID, key string, step *int) (analysis.PositionResponse, error) {
	service.key, service.step, service.sessionID = key, step, sessionID
	return analysis.PositionResponse{BoardID: boardID, PositionKey: key, Turn: bridge.East, Cards: []analysis.CardPrediction{{Card: bridge.Card{Suit: bridge.Spades, Rank: bridge.Rank("A")}, Tricks: 9}}}, service.err
}

func TestPositionAnalysisHTTP(t *testing.T) {
	for _, test := range []struct {
		name, query, token string
		err                error
		status             int
	}{
		{name: "live revision", query: "?positionKey=12", token: "valid-access", status: 200},
		{name: "opening lead", query: "?positionKey=0&step=0", token: "valid-access", status: 200},
		{name: "partial trick", query: "?positionKey=3&step=3", token: "valid-access", status: 200},
		{name: "invalid cursor", query: "?positionKey=53&step=53", token: "valid-access", status: 400},
		{name: "unauthenticated", query: "?positionKey=12", token: "invalid", status: 401},
		{name: "outsider", query: "?positionKey=12", token: "valid-access", err: analysis.ErrNotFound, status: 404},
		{name: "revision changed", query: "?positionKey=12", token: "valid-access", err: analysis.ErrPositionChanged, status: 409},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := &positionServiceFake{err: test.err}
			router := NewRouter(Options{Logger: observability.NewLoggerWithWriter(slog.LevelDebug, &bytes.Buffer{}), Identity: &identityServiceFake{}, Analysis: service})
			request := httptest.NewRequest(http.MethodGet, "/v1/boards/"+uuid.NewString()+"/analysis"+test.query, nil)
			request.Header.Set("Authorization", "Bearer "+test.token)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("status %d want %d: %s", response.Code, test.status, response.Body)
			}
			if response.Header().Get("Cache-Control") != "private, no-store" {
				t.Fatal("position can be cached")
			}
			if test.status == 200 {
				var result apigen.PositionAnalysis
				if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
					t.Fatal(err)
				}
				if result.PositionKey != service.key || service.sessionID != testSessionID || len(result.Cards) != 1 || result.Cards[0].Tricks != 9 {
					t.Fatal("position identity or predictions lost")
				}
				if strings.Contains(test.query, "step=") != (service.step != nil) {
					t.Fatal("history cursor lost")
				}
			}
		})
	}
}
