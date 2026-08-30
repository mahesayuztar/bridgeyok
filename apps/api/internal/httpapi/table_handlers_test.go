package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mahesayuztar/bridgeyok/apps/api/internal/bridge"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/identity"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/observability"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/table"
)

const (
	testTableID       = "77bfad45-a1d8-4117-9cf2-e61663f81e70"
	testParticipantID = "4280536e-75e4-4869-b91a-b0d3fc4e1aa8"
	testInviteCode    = "AEBAGBAFAYDQQCIKBMGA2DQPCA"
)

type tableServiceFake struct {
	previewError error
	joinError    error
	getError     error
	leaveError   error
	leftTableID  string
	leftSession  string
}

type realtimeServiceFake struct {
	changedTableIDs []string
}

func (service *realtimeServiceFake) ServeHTTP(writer http.ResponseWriter, _ *http.Request) {
	writer.WriteHeader(http.StatusNoContent)
}

func (service *realtimeServiceFake) TableChanged(_ context.Context, tableID string) {
	service.changedTableIDs = append(service.changedTableIDs, tableID)
}

func (service *tableServiceFake) Create(_ context.Context, session identity.Session) (table.CreatedTable, error) {
	return table.CreatedTable{InviteCode: testInviteCode, Projection: testTableProjection(session)}, nil
}

func (service *tableServiceFake) Preview(_ context.Context, _ string) (table.Preview, error) {
	if service.previewError != nil {
		return table.Preview{}, service.previewError
	}
	return table.Preview{State: table.StateWaiting, ParticipantCount: 1, Capacity: 4}, nil
}

func (service *tableServiceFake) Join(_ context.Context, _ string, session identity.Session) (table.Projection, error) {
	if service.joinError != nil {
		return table.Projection{}, service.joinError
	}
	return testTableProjection(session), nil
}

func (service *tableServiceFake) Get(_ context.Context, _ string, session identity.Session) (table.Projection, error) {
	if service.getError != nil {
		return table.Projection{}, service.getError
	}
	return testTableProjection(session), nil
}

func (service *tableServiceFake) Leave(_ context.Context, tableID string, session identity.Session) error {
	service.leftTableID = tableID
	service.leftSession = session.ID
	return service.leaveError
}

func TestTableHTTPHandlerCreateAndPreview(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		method     string
		path       string
		token      string
		service    *tableServiceFake
		wantStatus int
		wantBody   string
	}{
		{name: "create", method: http.MethodPost, path: "/v1/tables", token: "valid-access", service: &tableServiceFake{}, wantStatus: http.StatusCreated, wantBody: `"inviteCode":"` + testInviteCode + `"`},
		{name: "create unauthenticated", method: http.MethodPost, path: "/v1/tables", service: &tableServiceFake{}, wantStatus: http.StatusUnauthorized, wantBody: `"code":"AUTHENTICATION_REQUIRED"`},
		{name: "preview", method: http.MethodGet, path: "/v1/tables/" + testInviteCode + "/preview", service: &tableServiceFake{}, wantStatus: http.StatusOK, wantBody: `"participantCount":1`},
		{name: "preview missing", method: http.MethodGet, path: "/v1/tables/" + testInviteCode + "/preview", service: &tableServiceFake{previewError: table.ErrTableNotFound}, wantStatus: http.StatusNotFound, wantBody: `"code":"TABLE_NOT_FOUND"`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			handler := testTableRouter(test.service)
			request := httptest.NewRequest(test.method, test.path, nil)
			if test.token != "" {
				request.Header.Set("Authorization", "Bearer "+test.token)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, test.wantStatus, response.Body.String())
			}
			if !strings.Contains(response.Body.String(), test.wantBody) {
				t.Fatalf("body = %s, want %s", response.Body.String(), test.wantBody)
			}
			if response.Header().Get("Cache-Control") != "private, no-store" {
				t.Fatalf("Cache-Control = %q", response.Header().Get("Cache-Control"))
			}
		})
	}
}

func TestTableHTTPHandlerJoinAndGet(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		method     string
		path       string
		service    *tableServiceFake
		wantStatus int
		wantCode   string
	}{
		{name: "join", method: http.MethodPost, path: "/v1/tables/" + testInviteCode + "/join", service: &tableServiceFake{}, wantStatus: http.StatusOK},
		{name: "join unavailable", method: http.MethodPost, path: "/v1/tables/" + testInviteCode + "/join", service: &tableServiceFake{joinError: table.ErrTableUnavailable}, wantStatus: http.StatusNotFound, wantCode: "TABLE_UNAVAILABLE"},
		{name: "get", method: http.MethodGet, path: "/v1/tables/" + testTableID, service: &tableServiceFake{}, wantStatus: http.StatusOK},
		{name: "get missing", method: http.MethodGet, path: "/v1/tables/" + testTableID, service: &tableServiceFake{getError: table.ErrTableNotFound}, wantStatus: http.StatusNotFound, wantCode: "TABLE_NOT_FOUND"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			handler := testTableRouter(test.service)
			request := httptest.NewRequest(test.method, test.path, nil)
			request.Header.Set("Authorization", "Bearer valid-access")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, test.wantStatus, response.Body.String())
			}
			if test.wantCode != "" && !strings.Contains(response.Body.String(), `"code":"`+test.wantCode+`"`) {
				t.Fatalf("body = %s, want code %s", response.Body.String(), test.wantCode)
			}
			if strings.Contains(response.Body.String(), "session-owner") {
				t.Fatalf("response contains private session identifier: %s", response.Body.String())
			}
		})
	}
}

func TestTableHTTPHandlerLeave(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		service    *tableServiceFake
		wantStatus int
		wantCode   string
	}{
		{name: "left", service: &tableServiceFake{}, wantStatus: http.StatusNoContent},
		{name: "owner conflict", service: &tableServiceFake{leaveError: &table.DomainError{Code: table.ErrorOwnerCannotLeave, Message: "owner"}}, wantStatus: http.StatusConflict, wantCode: string(table.ErrorOwnerCannotLeave)},
		{name: "missing", service: &tableServiceFake{leaveError: table.ErrTableNotFound}, wantStatus: http.StatusNotFound, wantCode: "TABLE_NOT_FOUND"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			handler := testTableRouter(test.service)
			request := httptest.NewRequest(http.MethodPost, "/v1/tables/"+testTableID+"/leave", nil)
			request.Header.Set("Authorization", "Bearer valid-access")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, test.wantStatus, response.Body.String())
			}
			if test.wantCode != "" && !strings.Contains(response.Body.String(), `"code":"`+test.wantCode+`"`) {
				t.Fatalf("body = %s, want code %s", response.Body.String(), test.wantCode)
			}
			if test.wantStatus == http.StatusNoContent && (test.service.leftTableID != testTableID || test.service.leftSession != testSessionID) {
				t.Fatalf("Leave() received table=%q session=%q", test.service.leftTableID, test.service.leftSession)
			}
		})
	}
}

func TestTableHTTPHandlerNotifiesRealtimeOnlyAfterLifecycleMutation(t *testing.T) {
	t.Parallel()

	service := &tableServiceFake{}
	realtimeService := &realtimeServiceFake{}
	handler := NewRouter(Options{
		Logger:         observability.NewLoggerWithWriter(slog.LevelDebug, &strings.Builder{}),
		AllowedOrigins: []string{"https://app.example"},
		Readiness:      readinessFunc(func(context.Context) error { return nil }),
		Identity:       &identityServiceFake{},
		Table:          service,
		Realtime:       realtimeService,
	})
	requests := []struct {
		method string
		path   string
	}{
		{method: http.MethodPost, path: "/v1/tables/" + testInviteCode + "/join"},
		{method: http.MethodGet, path: "/v1/tables/" + testTableID},
		{method: http.MethodPost, path: "/v1/tables/" + testTableID + "/leave"},
	}
	for _, scriptedRequest := range requests {
		request := httptest.NewRequest(scriptedRequest.method, scriptedRequest.path, nil)
		request.Header.Set("Authorization", "Bearer valid-access")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code < 200 || response.Code >= 300 {
			t.Fatalf("%s %s status = %d", scriptedRequest.method, scriptedRequest.path, response.Code)
		}
	}
	if len(realtimeService.changedTableIDs) != 2 || realtimeService.changedTableIDs[0] != testTableID || realtimeService.changedTableIDs[1] != testTableID {
		t.Fatalf("realtime notifications = %v, want join and leave only", realtimeService.changedTableIDs)
	}
}

func TestTableHTTPHandlerUnavailableService(t *testing.T) {
	t.Parallel()

	handler := testTableRouter(nil)
	request := httptest.NewRequest(http.MethodPost, "/v1/tables", nil)
	request.Header.Set("Authorization", "Bearer valid-access")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), `"code":"SERVICE_UNAVAILABLE"`) {
		t.Fatalf("response = %d %s", response.Code, response.Body.String())
	}
}

func testTableRouter(service TableService) http.Handler {
	return NewRouter(Options{
		Logger:         observability.NewLoggerWithWriter(slog.LevelDebug, &strings.Builder{}),
		AllowedOrigins: []string{"https://app.example"},
		Readiness:      readinessFunc(func(context.Context) error { return nil }),
		Identity:       &identityServiceFake{},
		Table:          service,
	})
}

func testTableProjection(session identity.Session) table.Projection {
	return table.Projection{
		TableID:             testTableID,
		State:               table.StateWaiting,
		ViewerParticipantID: testParticipantID,
		ViewerRole:          table.RoleParticipant,
		ViewerSeat:          bridge.North,
		Participants: []table.ProjectedParticipant{{
			ID: testParticipantID, Nickname: session.Nickname, Role: table.RoleParticipant,
		}},
		Seats: map[bridge.Seat]table.SeatAssignment{
			bridge.North: {ParticipantID: testParticipantID, ControllerEpoch: 1},
		},
	}
}
