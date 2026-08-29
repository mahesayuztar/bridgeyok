package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/coder/websocket"
)

type checkConfig struct {
	url    string
	origin string
	mode   string
	http   *http.Client
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	appConfig, err := loadConfig()
	if err != nil {
		logger.Error("invalid websocket check configuration", "error", err)
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	switch appConfig.mode {
	case "echo":
		err = checkEcho(ctx, appConfig)
	case "rejected-origin":
		err = checkRejectedOrigin(ctx, appConfig)
	case "oversized":
		err = checkOversized(ctx, appConfig)
	default:
		err = fmt.Errorf("WS_CHECK_MODE must be echo, rejected-origin, or oversized")
	}
	if err != nil {
		logger.Error("websocket check failed", "mode", appConfig.mode, "error", err)
		os.Exit(1)
	}
	logger.Info("websocket check passed", "mode", appConfig.mode)
}

func loadConfig() (checkConfig, error) {
	websocketURL := strings.TrimSpace(os.Getenv("WS_CHECK_URL"))
	parsedURL, err := url.Parse(websocketURL)
	if err != nil || parsedURL.Scheme != "wss" || parsedURL.Host == "" {
		return checkConfig{}, fmt.Errorf("WS_CHECK_URL must be a valid wss URL")
	}
	origin := strings.TrimSpace(os.Getenv("WS_CHECK_ORIGIN"))
	if origin == "" {
		return checkConfig{}, fmt.Errorf("WS_CHECK_ORIGIN is required")
	}
	caFile := strings.TrimSpace(os.Getenv("WS_CHECK_CA_FILE"))
	certificate, err := os.ReadFile(caFile)
	if err != nil {
		return checkConfig{}, fmt.Errorf("read WS_CHECK_CA_FILE: %w", err)
	}
	rootCAs := x509.NewCertPool()
	if !rootCAs.AppendCertsFromPEM(certificate) {
		return checkConfig{}, fmt.Errorf("WS_CHECK_CA_FILE does not contain a valid certificate")
	}

	return checkConfig{
		url:    websocketURL,
		origin: origin,
		mode:   strings.TrimSpace(os.Getenv("WS_CHECK_MODE")),
		http: &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			RootCAs:    rootCAs,
		}}},
	}, nil
}

func dial(ctx context.Context, appConfig checkConfig) (*websocket.Conn, *http.Response, error) {
	return websocket.Dial(ctx, appConfig.url, &websocket.DialOptions{
		HTTPClient: appConfig.http,
		HTTPHeader: http.Header{"Origin": []string{appConfig.origin}},
	})
}

func checkEcho(ctx context.Context, appConfig checkConfig) error {
	connection, response, err := dial(ctx, appConfig)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	if response != nil && response.Body != nil {
		defer response.Body.Close()
	}
	defer connection.CloseNow()
	message := []byte(`{"kind":"control","name":"local-gate"}`)
	if err := connection.Write(ctx, websocket.MessageText, message); err != nil {
		return fmt.Errorf("write echo message: %w", err)
	}
	messageType, echoed, err := connection.Read(ctx)
	if err != nil {
		return fmt.Errorf("read echo message: %w", err)
	}
	if messageType != websocket.MessageText || string(echoed) != string(message) {
		return fmt.Errorf("unexpected echo response")
	}

	readerError := make(chan error, 1)
	go func() {
		_, _, readErr := connection.Read(ctx)
		readerError <- readErr
	}()
	if err := connection.Ping(ctx); err != nil {
		return fmt.Errorf("ping: %w", err)
	}
	if err := connection.Close(websocket.StatusNormalClosure, "local gate complete"); err != nil {
		return fmt.Errorf("close: %w", err)
	}
	select {
	case readErr := <-readerError:
		status := websocket.CloseStatus(readErr)
		if readErr != nil && status != websocket.StatusNormalClosure && status != websocket.StatusGoingAway {
			return fmt.Errorf("reader shutdown: %w", readErr)
		}
		return nil
	case <-ctx.Done():
		return fmt.Errorf("wait for reader shutdown: %w", ctx.Err())
	}
}

func checkRejectedOrigin(ctx context.Context, appConfig checkConfig) error {
	connection, response, err := dial(ctx, appConfig)
	if connection != nil {
		connection.CloseNow()
	}
	if response != nil && response.Body != nil {
		defer response.Body.Close()
	}
	if err == nil || response == nil || response.StatusCode != http.StatusForbidden {
		return fmt.Errorf("origin rejection returned response %v and error %v", response, err)
	}
	return nil
}

func checkOversized(ctx context.Context, appConfig checkConfig) error {
	connection, response, err := dial(ctx, appConfig)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	if response != nil && response.Body != nil {
		defer response.Body.Close()
	}
	defer connection.CloseNow()
	message := []byte(strings.Repeat("x", (8<<10)+1))
	if err := connection.Write(ctx, websocket.MessageText, message); err != nil {
		return fmt.Errorf("write oversized message: %w", err)
	}
	_, _, err = connection.Read(ctx)
	if websocket.CloseStatus(err) != websocket.StatusMessageTooBig {
		return fmt.Errorf("oversized message close status = %d, want %d", websocket.CloseStatus(err), websocket.StatusMessageTooBig)
	}
	return nil
}
