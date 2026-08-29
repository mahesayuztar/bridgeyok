package httpapi

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mahesayuztar/bridgeyok/apps/api/internal/observability"
)

type readinessFunc func(context.Context) error

func (readiness readinessFunc) Ping(ctx context.Context) error {
	return readiness(ctx)
}

func TestHealthEndpoints(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		path       string
		readiness  readinessFunc
		wantStatus int
		wantBody   string
	}{
		{
			name:       "liveness",
			path:       "/health/live",
			readiness:  func(context.Context) error { return nil },
			wantStatus: http.StatusOK,
			wantBody:   `"status":"ok"`,
		},
		{
			name:       "ready",
			path:       "/health/ready",
			readiness:  func(context.Context) error { return nil },
			wantStatus: http.StatusOK,
			wantBody:   `"status":"ready"`,
		},
		{
			name:       "dependency unavailable",
			path:       "/health/ready",
			readiness:  func(context.Context) error { return errors.New("database unavailable") },
			wantStatus: http.StatusServiceUnavailable,
			wantBody:   `"status":"unavailable"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			handler, _ := testRouter(test.readiness)
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
			if !strings.Contains(response.Body.String(), test.wantBody) {
				t.Fatalf("body = %q, want containing %q", response.Body.String(), test.wantBody)
			}
			if response.Header().Get("Content-Type") != "application/json; charset=utf-8" {
				t.Fatalf("Content-Type = %q", response.Header().Get("Content-Type"))
			}
		})
	}
}

func TestCORS(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		method     string
		origin     string
		wantStatus int
		wantOrigin string
	}{
		{name: "allowed origin", method: http.MethodGet, origin: "https://app.example", wantStatus: http.StatusOK, wantOrigin: "https://app.example"},
		{name: "disallowed origin", method: http.MethodGet, origin: "https://attacker.example", wantStatus: http.StatusForbidden},
		{name: "allowed preflight", method: http.MethodOptions, origin: "https://app.example", wantStatus: http.StatusNoContent, wantOrigin: "https://app.example"},
		{name: "server request without origin", method: http.MethodGet, wantStatus: http.StatusOK},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			handler, _ := testRouter(readinessFunc(func(context.Context) error { return nil }))
			request := httptest.NewRequest(test.method, "/health/live", nil)
			if test.origin != "" {
				request.Header.Set("Origin", test.origin)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
			if response.Header().Get("Access-Control-Allow-Origin") != test.wantOrigin {
				t.Fatalf("Access-Control-Allow-Origin = %q, want %q", response.Header().Get("Access-Control-Allow-Origin"), test.wantOrigin)
			}
		})
	}
}

func TestRequestID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		requestID string
		wantSame  bool
	}{
		{name: "trusted valid identifier", requestID: "req_client-123", wantSame: true},
		{name: "replace invalid identifier", requestID: "contains spaces and /", wantSame: false},
		{name: "generate missing identifier", wantSame: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			handler, logs := testRouter(readinessFunc(func(context.Context) error { return nil }))
			request := httptest.NewRequest(http.MethodGet, "/health/live", nil)
			if test.requestID != "" {
				request.Header.Set(requestIDHeader, test.requestID)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			responseID := response.Header().Get(requestIDHeader)
			if responseID == "" {
				t.Fatal("response request ID is empty")
			}
			if (responseID == test.requestID) != test.wantSame {
				t.Fatalf("response request ID = %q, input = %q", responseID, test.requestID)
			}
			if !strings.Contains(logs.String(), responseID) {
				t.Fatalf("structured log does not contain request ID %q: %s", responseID, logs.String())
			}
		})
	}
}

func TestSecurityHeadersAndNotFound(t *testing.T) {
	t.Parallel()

	handler, _ := testRouter(readinessFunc(func(context.Context) error { return nil }))
	request := httptest.NewRequest(http.MethodGet, "/missing", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
	if response.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q", response.Header().Get("X-Content-Type-Options"))
	}
	if !strings.Contains(response.Body.String(), `"status":404`) {
		t.Fatalf("body = %q, want problem details", response.Body.String())
	}
}

func TestRecoveryHidesPanicValue(t *testing.T) {
	t.Parallel()

	logs := &bytes.Buffer{}
	logger := observability.NewLoggerWithWriter(slog.LevelDebug, logs)
	secret := "sensitive-runtime-value"
	handler := requestIDMiddleware(requestLoggingMiddleware(logger)(recoveryMiddleware(logger)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic(secret)
	}))))
	request := httptest.NewRequest(http.MethodGet, "/panic", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
	if strings.Contains(response.Body.String(), secret) || strings.Contains(logs.String(), secret) {
		t.Fatalf("panic value leaked in response or logs: body=%q logs=%q", response.Body.String(), logs.String())
	}
}

func testRouter(readiness ReadinessChecker) (http.Handler, *bytes.Buffer) {
	logs := &bytes.Buffer{}
	logger := observability.NewLoggerWithWriter(slog.LevelDebug, logs)
	return NewRouter(Options{
		Logger:         logger,
		AllowedOrigins: []string{"https://app.example"},
		Readiness:      readiness,
	}), logs
}
