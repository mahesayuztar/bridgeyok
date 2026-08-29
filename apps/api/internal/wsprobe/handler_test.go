package wsprobe

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestNewHandler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		allowedOrigin string
		logger        *slog.Logger
		wantError     bool
	}{
		{name: "valid exact origin", allowedOrigin: "https://bridgeyok.localhost:8443", logger: testLogger(), wantError: false},
		{name: "origin with path", allowedOrigin: "https://bridgeyok.localhost/path", logger: testLogger(), wantError: true},
		{name: "wildcard origin", allowedOrigin: "*", logger: testLogger(), wantError: true},
		{name: "missing logger", allowedOrigin: "https://bridgeyok.localhost:8443", logger: nil, wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewHandler(test.allowedOrigin, test.logger)
			if (err != nil) != test.wantError {
				t.Fatalf("NewHandler() error = %v, wantError %t", err, test.wantError)
			}
		})
	}
}

func TestHandlerHealth(t *testing.T) {
	t.Parallel()

	handler, err := NewHandler("https://bridgeyok.localhost:8443", testLogger())
	if err != nil {
		t.Fatalf("NewHandler() unexpected error: %v", err)
	}
	request := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if response.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("missing security headers")
	}
}

func TestHandlerWebSocket(t *testing.T) {
	t.Parallel()

	const allowedOrigin = "https://bridgeyok.localhost:8443"
	handler, err := NewHandler(allowedOrigin, testLogger())
	if err != nil {
		t.Fatalf("NewHandler() unexpected error: %v", err)
	}
	server := httptest.NewTLSServer(handler)
	defer server.Close()
	websocketURL := "wss" + strings.TrimPrefix(server.URL, "https") + "/v1/ws"

	tests := []struct {
		name       string
		origin     string
		message    []byte
		wantStatus int
	}{
		{name: "echoes allowed text", origin: allowedOrigin, message: []byte(`{"kind":"control","name":"local-gate"}`), wantStatus: 0},
		{name: "rejects another origin", origin: "https://attacker.example", message: nil, wantStatus: http.StatusForbidden},
		{name: "rejects oversized message", origin: allowedOrigin, message: []byte(strings.Repeat("x", maxMessageBytes+1)), wantStatus: int(websocket.StatusMessageTooBig)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			connection, response, err := websocket.Dial(ctx, websocketURL, &websocket.DialOptions{
				HTTPClient: server.Client(),
				HTTPHeader: http.Header{"Origin": []string{test.origin}},
			})
			if test.wantStatus == http.StatusForbidden {
				if connection != nil {
					connection.CloseNow()
				}
				if response != nil && response.Body != nil {
					defer response.Body.Close()
				}
				if err == nil || response == nil || response.StatusCode != http.StatusForbidden {
					t.Fatalf("Dial() error = %v, response = %v, want status %d", err, response, http.StatusForbidden)
				}
				return
			}
			if err != nil {
				t.Fatalf("Dial() unexpected error: %v", err)
			}
			defer connection.CloseNow()

			if err := connection.Write(ctx, websocket.MessageText, test.message); err != nil {
				t.Fatalf("Write() unexpected error: %v", err)
			}
			messageType, echoed, err := connection.Read(ctx)
			if test.wantStatus == int(websocket.StatusMessageTooBig) {
				if websocket.CloseStatus(err) != websocket.StatusMessageTooBig {
					t.Fatalf("Read() error = %v, want close status %d", err, websocket.StatusMessageTooBig)
				}
				return
			}
			if err != nil {
				t.Fatalf("Read() unexpected error: %v", err)
			}
			if messageType != websocket.MessageText || string(echoed) != string(test.message) {
				t.Fatalf("echo = (%d, %q), want (%d, %q)", messageType, echoed, websocket.MessageText, test.message)
			}
		})
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}
