package httpapi

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mahesayuztar/bridgeyok/apps/api/internal/match"
)

type matchServiceFake struct {
	calls     int
	err       error
	sessionID string
	action    string
}

func (service *matchServiceFake) Create(_ context.Context, sessionID string, _ match.CreateRequest) (match.PublicView, error) {
	service.calls++
	service.sessionID = sessionID
	return match.PublicView{ID: testTableID}, service.err
}
func (service *matchServiceFake) Get(_ context.Context, _, sessionID string) (match.PublicView, error) {
	service.calls++
	service.sessionID = sessionID
	return match.PublicView{ID: testTableID}, service.err
}
func (service *matchServiceFake) List(_ context.Context, sessionID string) ([]match.PublicView, error) {
	service.calls++
	service.sessionID = sessionID
	return []match.PublicView{}, service.err
}
func (service *matchServiceFake) Start(_ context.Context, _, sessionID string, _ int64) (match.PublicView, error) {
	service.calls++
	service.sessionID = sessionID
	service.action = "start"
	return match.PublicView{ID: testTableID}, service.err
}
func (service *matchServiceFake) Ready(_ context.Context, _, sessionID, _ string, _ int64, _ bool) (match.PublicView, error) {
	service.calls++
	service.sessionID = sessionID
	service.action = "ready"
	return match.PublicView{ID: testTableID}, service.err
}
func (service *matchServiceFake) Cancel(_ context.Context, _, sessionID string, _ int64) (match.PublicView, error) {
	service.calls++
	service.sessionID = sessionID
	service.action = "cancel"
	return match.PublicView{ID: testTableID}, service.err
}

func TestMatchHTTPHandlers(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, method, path, body string
		err                      error
		status                   int
		action                   string
		unauthorized             bool
	}{
		{name: "list", method: "GET", path: "/v1/matches", status: 200},
		{name: "get", method: "GET", path: "/v1/matches/" + testTableID, status: 200},
		{name: "create", method: "POST", path: "/v1/matches", body: `{"requestId":"create_01","boardCount":1,"assignments":[]}`, status: 201},
		{name: "start", method: "POST", path: "/v1/matches/" + testTableID + "/start", body: `{"expectedRevision":0}`, status: 200, action: "start"},
		{name: "cancel", method: "POST", path: "/v1/matches/" + testTableID + "/cancel", body: `{"expectedRevision":0}`, status: 200, action: "cancel"},
		{name: "ready", method: "POST", path: "/v1/matches/" + testTableID + "/ready", body: `{"requestId":"ready_001","expectedTableRevision":0,"ready":false}`, status: 200, action: "ready"},
		{name: "unauthenticated", method: "GET", path: "/v1/matches", status: 401, unauthorized: true},
		{name: "missing revision", method: "POST", path: "/v1/matches/" + testTableID + "/start", body: `{}`, status: 400},
		{name: "missing ready", method: "POST", path: "/v1/matches/" + testTableID + "/ready", body: `{"expectedTableRevision":0}`, status: 400},
		{name: "impersonation", method: "POST", path: "/v1/matches/" + testTableID + "/ready", body: `{"expectedTableRevision":0,"ready":true,"sessionId":"other"}`, status: 400},
		{name: "private deal injection", method: "POST", path: "/v1/matches", body: `{"deal":{}}`, status: 400},
		{name: "two bodies", method: "POST", path: "/v1/matches", body: `{} {}`, status: 400},
		{name: "oversized", method: "POST", path: "/v1/matches", body: `{"requestId":"` + strings.Repeat("x", maxHTTPBodyBytes) + `"}`, status: 400},
		{name: "not found", method: "GET", path: "/v1/matches/" + testTableID, err: match.ErrNotFound, status: 404},
		{name: "forbidden", method: "GET", path: "/v1/matches/" + testTableID, err: match.ErrForbidden, status: 403},
		{name: "invalid", method: "GET", path: "/v1/matches/" + testTableID, err: match.ErrInvalid, status: 400},
		{name: "conflict", method: "GET", path: "/v1/matches/" + testTableID, err: match.ErrState, status: 409},
		{name: "capacity", method: "GET", path: "/v1/matches/" + testTableID, err: match.ErrCapacity, status: 429},
		{name: "redacted failure", method: "GET", path: "/v1/matches/" + testTableID, err: errors.New("private-deal-database-secret"), status: 500},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			service := &matchServiceFake{err: test.err}
			handler := NewRouter(Options{Logger: slog.New(slog.NewJSONHandler(io.Discard, nil)), Identity: &identityServiceFake{}, Match: service})
			request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			if !test.unauthorized {
				request.Header.Set("Authorization", "Bearer valid-access")
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("got %d %s", response.Code, response.Body.String())
			}
			if response.Header().Get("Cache-Control") != "private, no-store" {
				t.Fatal("cacheable response")
			}
			if test.unauthorized || (test.status == 400 && test.err == nil) {
				if service.calls != 0 {
					t.Fatal("invalid request dispatched")
				}
			} else {
				if service.sessionID != testSessionID || service.action != test.action {
					t.Fatal("wrong authenticated actor/action")
				}
			}
			if strings.Contains(response.Body.String(), "private-deal-database-secret") {
				t.Fatal("private error exposed")
			}
		})
	}
}
