package wsprobe

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coder/websocket"
)

const maxMessageBytes = 8 << 10

type handler struct {
	allowedOrigin string
	logger        *slog.Logger
}

// NewHandler creates the isolated WebSocket infrastructure probe handler.
func NewHandler(allowedOrigin string, logger *slog.Logger) (http.Handler, error) {
	normalizedOrigin, err := normalizeOrigin(allowedOrigin)
	if err != nil {
		return nil, err
	}
	if logger == nil {
		return nil, fmt.Errorf("logger is required")
	}

	probe := handler{allowedOrigin: normalizedOrigin, logger: logger}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", probe.serveHealth)
	mux.HandleFunc("GET /v1/ws", probe.serveWebSocket)
	return securityHeaders(mux), nil
}

func normalizeOrigin(rawOrigin string) (string, error) {
	origin := strings.TrimSpace(rawOrigin)
	parsed, err := url.Parse(origin)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.User != nil {
		return "", fmt.Errorf("WS_PROBE_ALLOWED_ORIGIN must be one exact HTTP origin")
	}
	return parsed.Scheme + "://" + parsed.Host, nil
}

func (probe handler) serveHealth(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(writer).Encode(map[string]string{"service": "bridgeyok-ws-probe", "status": "ok"}); err != nil {
		probe.logger.WarnContext(request.Context(), "websocket probe health response failed")
	}
}

func (probe handler) serveWebSocket(writer http.ResponseWriter, request *http.Request) {
	requestID := "wsp_" + rand.Text()
	origin := request.Header.Get("Origin")
	if origin != probe.allowedOrigin {
		probe.logger.WarnContext(request.Context(), "websocket probe rejected",
			"request_id", requestID,
			"reason", "origin_not_allowed",
		)
		http.Error(writer, "origin not allowed", http.StatusForbidden)
		return
	}

	connection, err := websocket.Accept(writer, request, &websocket.AcceptOptions{
		OriginPatterns: []string{probe.allowedOrigin},
	})
	if err != nil {
		probe.logger.WarnContext(request.Context(), "websocket probe handshake failed", "request_id", requestID)
		return
	}
	defer connection.CloseNow()
	connection.SetReadLimit(maxMessageBytes)
	startedAt := time.Now()
	messageCount := 0

	probe.logger.InfoContext(request.Context(), "websocket probe connected", "request_id", requestID)
	defer func() {
		probe.logger.InfoContext(request.Context(), "websocket probe disconnected",
			"request_id", requestID,
			"messages", messageCount,
			"duration_ms", time.Since(startedAt).Milliseconds(),
		)
	}()

	for {
		messageType, message, err := connection.Read(request.Context())
		if err != nil {
			status := websocket.CloseStatus(err)
			if status != websocket.StatusNormalClosure && status != websocket.StatusGoingAway && status != websocket.StatusMessageTooBig {
				probe.logger.WarnContext(request.Context(), "websocket probe read failed", "request_id", requestID, "close_status", status)
			}
			return
		}
		if messageType != websocket.MessageText {
			if err := connection.Close(websocket.StatusUnsupportedData, "text messages only"); err != nil {
				probe.logger.WarnContext(request.Context(), "websocket probe close failed", "request_id", requestID)
			}
			return
		}
		if err := connection.Write(request.Context(), websocket.MessageText, message); err != nil {
			probe.logger.WarnContext(request.Context(), "websocket probe write failed", "request_id", requestID)
			return
		}
		messageCount++
	}
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		writer.Header().Set("X-Frame-Options", "DENY")
		writer.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(writer, request)
	})
}
