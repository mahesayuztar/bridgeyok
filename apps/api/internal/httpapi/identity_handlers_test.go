package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mahesayuztar/bridgeyok/apps/api/internal/identity"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/observability"
)

const testSessionID = "0f4b9a5b-0ea8-4ad6-a866-e576ccd8be31"

type identityServiceFake struct {
	createError       error
	authenticateError error
	revokedSessionID  string
}

func (service *identityServiceFake) CreateSession(_ context.Context, nickname string) (identity.CredentialSet, error) {
	if service.createError != nil {
		return identity.CredentialSet{}, service.createError
	}
	return testCredentials(nickname), nil
}

func (service *identityServiceFake) Refresh(_ context.Context, deviceCredential string) (identity.CredentialSet, error) {
	if deviceCredential != "valid-device" {
		return identity.CredentialSet{}, identity.ErrInvalidCredential
	}
	return testCredentials("North Player"), nil
}

func (service *identityServiceFake) Authenticate(_ context.Context, accessToken string) (identity.Session, error) {
	if service.authenticateError != nil {
		return identity.Session{}, service.authenticateError
	}
	if accessToken != "valid-access" {
		return identity.Session{}, identity.ErrInvalidCredential
	}
	return testCredentials("North Player").Session, nil
}

func (service *identityServiceFake) Revoke(_ context.Context, sessionID string) error {
	service.revokedSessionID = sessionID
	return nil
}

func (service *identityServiceFake) IssueTicket(_ context.Context, session identity.Session) (string, time.Time, error) {
	if session.ID != testSessionID {
		return "", time.Time{}, errors.New("unexpected session")
	}
	return "single-use-ticket", time.Date(2026, 8, 30, 4, 0, 45, 0, time.UTC), nil
}

func TestIdentityHTTPHandlerCreateSession(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		body       string
		service    *identityServiceFake
		wantStatus int
		wantCode   string
	}{
		{name: "created", body: `{"nickname":"North Player"}`, service: &identityServiceFake{}, wantStatus: http.StatusCreated},
		{name: "invalid nickname", body: `{"nickname":"N"}`, service: &identityServiceFake{createError: identity.ErrInvalidNickname}, wantStatus: http.StatusBadRequest, wantCode: "INVALID_NICKNAME"},
		{name: "unknown field", body: `{"nickname":"North","role":"admin"}`, service: &identityServiceFake{}, wantStatus: http.StatusBadRequest, wantCode: "INVALID_REQUEST"},
		{name: "oversized", body: `{"nickname":"` + strings.Repeat("x", maxHTTPBodyBytes) + `"}`, service: &identityServiceFake{}, wantStatus: http.StatusBadRequest, wantCode: "INVALID_REQUEST"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			handler, _ := testIdentityRouter(test.service)
			request := httptest.NewRequest(http.MethodPost, "/v1/guest-sessions", strings.NewReader(test.body))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, test.wantStatus, response.Body.String())
			}
			if test.wantCode != "" && !strings.Contains(response.Body.String(), `"code":"`+test.wantCode+`"`) {
				t.Fatalf("body = %s, want code %s", response.Body.String(), test.wantCode)
			}
			if response.Header().Get("Cache-Control") != "private, no-store" {
				t.Fatalf("Cache-Control = %q", response.Header().Get("Cache-Control"))
			}
		})
	}
}

func TestIdentityHTTPHandlerRefreshSession(t *testing.T) {
	t.Parallel()

	handler, _ := testIdentityRouter(&identityServiceFake{})
	request := httptest.NewRequest(http.MethodPost, "/v1/guest-sessions/refresh", strings.NewReader(`{"deviceCredential":"valid-device"}`))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"accessToken":"access-token"`) {
		t.Fatalf("body = %s, want fresh access token", response.Body.String())
	}
}

func TestIdentityHTTPHandlerAuthenticatedEndpoints(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		method     string
		path       string
		token      string
		wantStatus int
	}{
		{name: "issue ticket", method: http.MethodPost, path: "/v1/realtime/tickets", token: "valid-access", wantStatus: http.StatusCreated},
		{name: "revoke session", method: http.MethodDelete, path: "/v1/guest-sessions/current", token: "valid-access", wantStatus: http.StatusNoContent},
		{name: "missing token", method: http.MethodPost, path: "/v1/realtime/tickets", wantStatus: http.StatusUnauthorized},
		{name: "invalid token", method: http.MethodPost, path: "/v1/realtime/tickets", token: "invalid", wantStatus: http.StatusUnauthorized},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			service := &identityServiceFake{}
			handler, _ := testIdentityRouter(service)
			request := httptest.NewRequest(test.method, test.path, nil)
			if test.token != "" {
				request.Header.Set("Authorization", "Bearer "+test.token)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, test.wantStatus, response.Body.String())
			}
			if test.path == "/v1/guest-sessions/current" && test.token == "valid-access" && service.revokedSessionID != testSessionID {
				t.Fatalf("revoked session = %q, want %q", service.revokedSessionID, testSessionID)
			}
		})
	}
}

func testIdentityRouter(service IdentityService) (http.Handler, *strings.Builder) {
	logs := &strings.Builder{}
	return NewRouter(Options{
		Logger:         observability.NewLoggerWithWriter(slog.LevelDebug, logs),
		AllowedOrigins: []string{"https://app.example"},
		Readiness:      readinessFunc(func(context.Context) error { return nil }),
		Identity:       service,
	}), logs
}

func testCredentials(nickname string) identity.CredentialSet {
	return identity.CredentialSet{
		Session: identity.Session{
			ID:        testSessionID,
			Nickname:  nickname,
			Status:    "ACTIVE",
			ExpiresAt: time.Date(2026, 9, 29, 4, 0, 0, 0, time.UTC),
		},
		AccessToken:      "access-token",
		AccessExpiresAt:  time.Date(2026, 8, 30, 4, 15, 0, 0, time.UTC),
		DeviceCredential: "device-credential",
	}
}
