package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestPublicRouterDoesNotExposeEcho(t *testing.T) {
	t.Parallel()

	handler, _ := testRouter(readinessFunc(func(context.Context) error { return nil }))
	request := httptest.NewRequest(http.MethodGet, "/v1/ws", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestWebSocketInfrastructureSpike(t *testing.T) {
	t.Parallel()

	const allowedOrigin = "https://app.example"
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Origin") != allowedOrigin {
			http.Error(writer, "origin not allowed", http.StatusForbidden)
			return
		}
		connection, err := websocket.Accept(writer, request, &websocket.AcceptOptions{
			OriginPatterns: []string{"app.example"},
		})
		if err != nil {
			return
		}
		defer connection.CloseNow()
		connection.SetReadLimit(8 << 10)
		messageType, message, err := connection.Read(request.Context())
		if err != nil {
			return
		}
		if err := connection.Write(request.Context(), messageType, message); err != nil {
			return
		}
		_ = connection.Close(websocket.StatusNormalClosure, "spike complete")
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	websocketURL := "wss" + strings.TrimPrefix(server.URL, "https")
	connection, response, err := websocket.Dial(ctx, websocketURL, &websocket.DialOptions{
		HTTPClient: server.Client(),
		HTTPHeader: http.Header{"Origin": []string{allowedOrigin}},
	})
	if err != nil {
		t.Fatalf("Dial() unexpected error: %v, response: %v", err, response)
	}
	defer connection.CloseNow()

	message := []byte(`{"kind":"control","name":"spike"}`)
	if err := connection.Write(ctx, websocket.MessageText, message); err != nil {
		t.Fatalf("Write() unexpected error: %v", err)
	}
	messageType, echoed, err := connection.Read(ctx)
	if err != nil {
		t.Fatalf("Read() unexpected error: %v", err)
	}
	if messageType != websocket.MessageText || string(echoed) != string(message) {
		t.Fatalf("echo = (%d, %q), want (%d, %q)", messageType, echoed, websocket.MessageText, message)
	}
}

func TestWebSocketInfrastructureSpikeRejectsOtherOrigins(t *testing.T) {
	t.Parallel()

	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Origin") != "https://app.example" {
			http.Error(writer, "origin not allowed", http.StatusForbidden)
			return
		}
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	websocketURL := "wss" + strings.TrimPrefix(server.URL, "https")
	connection, response, err := websocket.Dial(ctx, websocketURL, &websocket.DialOptions{
		HTTPClient: server.Client(),
		HTTPHeader: http.Header{"Origin": []string{"https://attacker.example"}},
	})
	if connection != nil {
		connection.CloseNow()
	}
	if err == nil || errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("Dial() expected origin rejection")
	}
	if response == nil || response.StatusCode != http.StatusForbidden {
		t.Fatalf("response = %v, want status %d", response, http.StatusForbidden)
	}
}
