package realtime

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/coder/websocket"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/identity"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/table"
)

// IdentityService authenticates a one-time handshake and every live message.
type IdentityService interface {
	ConsumeTicket(context.Context, string) (identity.Session, error)
	ValidateSession(context.Context, string) (identity.Session, error)
}

// TableRuntime serializes mutations and owns the current process-local table state.
type TableRuntime interface {
	Submit(context.Context, table.CommandRequest) (table.CommandResult, error)
	Snapshot(context.Context, string) (table.Aggregate, error)
	Refresh(context.Context, string) (table.Aggregate, error)
}

// EventStore loads a bounded durable event gap for reconnect recovery.
type EventStore interface {
	ListEventsAfter(context.Context, string, int64, int) ([]table.PersistedEvent, error)
}

// Options controls realtime resource bounds and connection liveness.
type Options struct {
	Logger                   *slog.Logger
	AllowedOrigins           []string
	Identity                 IdentityService
	Tables                   TableRuntime
	Events                   EventStore
	Random                   io.Reader
	Now                      func() time.Time
	ReadLimitBytes           int64
	OutboundQueueCapacity    int
	OutboundQueueBytes       int
	WriteTimeout             time.Duration
	PingInterval             time.Duration
	PongTimeout              time.Duration
	MaxConnections           int
	MaxConnectionsPerSession int
	MessageRate              int
	MessageBurst             int
	RecoveryLimit            int
}

// Server is the single-instance authenticated WebSocket endpoint and local room registry.
type Server struct {
	options Options
	broker  *broker

	mutex                sync.Mutex
	connections          map[*connection]struct{}
	connectionsBySession map[string]int
	activeReservations   int
	draining             bool
	wait                 sync.WaitGroup
	randomMutex          sync.Mutex
}

type outboundFrame struct {
	message     []byte
	closeStatus websocket.StatusCode
	closeReason string
	delivered   chan struct{}
}

type connection struct {
	id       string
	session  identity.Session
	socket   *websocket.Conn
	server   *Server
	ctx      context.Context
	cancel   context.CancelFunc
	done     chan struct{}
	outbound chan outboundFrame
	limiter  tokenBucket

	queueMutex      sync.Mutex
	queuedBytes     int
	stateMutex      sync.Mutex
	tableID         string
	closeStatus     websocket.StatusCode
	closeReason     string
	closeRequested  bool
	draining        bool
	invalidMessages int
}

type tokenBucket struct {
	tokens float64
	last   time.Time
	rate   float64
	burst  float64
}

// NewServer constructs the product WebSocket endpoint without starting background goroutines.
func NewServer(options Options) (*Server, error) {
	if options.Logger == nil || options.Identity == nil || options.Tables == nil || options.Events == nil || options.Random == nil || options.Now == nil {
		return nil, fmt.Errorf("realtime dependencies are required")
	}
	if len(options.AllowedOrigins) == 0 {
		return nil, fmt.Errorf("realtime allowed origins are required")
	}
	for _, origin := range options.AllowedOrigins {
		parsed, err := url.Parse(origin)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" || strings.ContainsAny(origin, "*?[") {
			return nil, fmt.Errorf("realtime origins must be exact HTTP origins")
		}
	}
	if options.ReadLimitBytes < 1 || options.OutboundQueueCapacity < 1 || options.OutboundQueueBytes < 1 || options.MaxConnections < 1 || options.MaxConnectionsPerSession < 1 || options.MessageRate < 1 || options.MessageBurst < 1 || options.RecoveryLimit < 1 || options.RecoveryLimit > 255 {
		return nil, fmt.Errorf("realtime resource limits must be positive and bounded")
	}
	if options.WriteTimeout <= 0 || options.PingInterval <= 0 || options.PongTimeout <= 0 {
		return nil, fmt.Errorf("realtime timeouts must be positive")
	}
	server := &Server{
		options:              options,
		connections:          make(map[*connection]struct{}),
		connectionsBySession: make(map[string]int),
	}
	server.broker = newBroker(options.Logger)
	return server, nil
}

// ServeHTTP consumes a single-use ticket and upgrades an exact-origin request.
func (server *Server) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		server.writeHandshakeError(writer, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED")
		return
	}
	origin := request.Header.Get("Origin")
	if origin == "" || !slices.Contains(server.options.AllowedOrigins, origin) {
		server.options.Logger.WarnContext(request.Context(), "realtime_handshake_rejected", "result_code", "ORIGIN_NOT_ALLOWED")
		server.writeHandshakeError(writer, http.StatusForbidden, "ORIGIN_NOT_ALLOWED")
		return
	}
	ticket := request.URL.Query().Get("ticket")
	if ticket == "" {
		server.options.Logger.WarnContext(request.Context(), "realtime_handshake_rejected", "result_code", "TICKET_REQUIRED")
		server.writeHandshakeError(writer, http.StatusUnauthorized, "TICKET_REQUIRED")
		return
	}
	if server.isDraining() {
		server.options.Logger.WarnContext(request.Context(), "realtime_handshake_rejected", "result_code", "SERVER_DRAINING")
		server.writeHandshakeError(writer, http.StatusServiceUnavailable, "SERVER_DRAINING")
		return
	}
	session, err := server.options.Identity.ConsumeTicket(request.Context(), ticket)
	if err != nil {
		server.options.Logger.WarnContext(request.Context(), "realtime_handshake_rejected", "result_code", "INVALID_TICKET")
		server.writeHandshakeError(writer, http.StatusUnauthorized, "INVALID_TICKET")
		return
	}
	reserved, rejectionCode := server.reserve(session.ID)
	if !reserved {
		status := http.StatusTooManyRequests
		if rejectionCode == "SERVER_DRAINING" {
			status = http.StatusServiceUnavailable
		}
		server.options.Logger.WarnContext(request.Context(), "realtime_handshake_rejected", "result_code", rejectionCode)
		server.writeHandshakeError(writer, status, rejectionCode)
		return
	}
	released := false
	defer func() {
		if !released {
			server.release(nil, session.ID)
		}
	}()

	socket, err := websocket.Accept(writer, request, &websocket.AcceptOptions{
		OriginPatterns:  server.options.AllowedOrigins,
		CompressionMode: websocket.CompressionDisabled,
	})
	if err != nil {
		server.options.Logger.WarnContext(request.Context(), "realtime_handshake_rejected", "result_code", "UPGRADE_FAILED")
		return
	}
	socket.SetReadLimit(server.options.ReadLimitBytes)
	ctx, cancel := context.WithCancel(request.Context())
	client := &connection{
		id:       "conn_" + rand.Text(),
		session:  session,
		socket:   socket,
		server:   server,
		ctx:      ctx,
		cancel:   cancel,
		done:     make(chan struct{}),
		outbound: make(chan outboundFrame, server.options.OutboundQueueCapacity),
		limiter: tokenBucket{
			tokens: float64(server.options.MessageBurst),
			last:   server.options.Now().UTC(),
			rate:   float64(server.options.MessageRate),
			burst:  float64(server.options.MessageBurst),
		},
	}
	shouldDrain := server.attach(client)
	released = true
	startedAt := server.options.Now().UTC()
	server.options.Logger.InfoContext(ctx, "realtime_connection_opened", "connection_id", client.id, "result_code", "CONNECTED")
	defer func() {
		client.cancel()
		<-client.done
		server.broker.unsubscribe(client)
		server.release(client, session.ID)
		if err := socket.CloseNow(); err != nil && !errors.Is(err, net.ErrClosed) {
			server.options.Logger.Warn("realtime_connection_cleanup_failed", "connection_id", client.id, "result_code", "CLOSE_ERROR")
		}
		server.options.Logger.Info("realtime_connection_closed",
			"connection_id", client.id,
			"result_code", "DISCONNECTED",
			"latency_ms", server.options.Now().UTC().Sub(startedAt).Milliseconds(),
		)
	}()

	go client.writeLoop()
	if shouldDrain {
		client.shutdown()
	}
	client.readLoop()
}

// TableChanged refreshes actor state and publishes a recipient snapshot after REST lifecycle commits.
func (server *Server) TableChanged(ctx context.Context, tableID string) {
	aggregate, err := server.options.Tables.Refresh(ctx, tableID)
	if err != nil {
		server.options.Logger.WarnContext(ctx, "realtime_lifecycle_refresh_failed", "table_id", tableID, "result_code", "REFRESH_ERROR")
		return
	}
	if err := server.broker.publishSnapshot(ctx, aggregate); err != nil {
		server.options.Logger.ErrorContext(ctx, "realtime_lifecycle_publish_failed", "table_id", tableID, "result_code", "PROJECTION_ERROR")
	}
}

// Drain rejects new handshakes, notifies live clients, and waits for close handshakes.
func (server *Server) Drain(ctx context.Context) error {
	server.mutex.Lock()
	if !server.draining {
		server.draining = true
	}
	connections := make([]*connection, 0, len(server.connections))
	for connection := range server.connections {
		connections = append(connections, connection)
	}
	server.mutex.Unlock()
	server.options.Logger.InfoContext(ctx, "realtime_server_draining", "active_connections", len(connections))
	for _, connection := range connections {
		connection.shutdown()
	}
	done := make(chan struct{})
	go func() {
		server.wait.Wait()
		close(done)
	}()
	select {
	case <-done:
		server.options.Logger.InfoContext(ctx, "realtime_server_drained")
		return nil
	case <-ctx.Done():
		return fmt.Errorf("drain realtime connections: %w", ctx.Err())
	}
}

func (server *Server) reserve(sessionID string) (bool, string) {
	server.mutex.Lock()
	defer server.mutex.Unlock()
	if server.draining {
		return false, "SERVER_DRAINING"
	}
	if server.activeReservations >= server.options.MaxConnections || server.connectionsBySession[sessionID] >= server.options.MaxConnectionsPerSession {
		return false, "CONNECTION_LIMIT"
	}
	server.activeReservations++
	server.connectionsBySession[sessionID]++
	server.wait.Add(1)
	return true, ""
}

func (server *Server) attach(connection *connection) bool {
	server.mutex.Lock()
	defer server.mutex.Unlock()
	server.connections[connection] = struct{}{}
	return server.draining
}

func (server *Server) isDraining() bool {
	server.mutex.Lock()
	defer server.mutex.Unlock()
	return server.draining
}

func (server *Server) release(connection *connection, sessionID string) {
	server.mutex.Lock()
	if connection != nil {
		delete(server.connections, connection)
	}
	server.activeReservations--
	server.connectionsBySession[sessionID]--
	if server.connectionsBySession[sessionID] == 0 {
		delete(server.connectionsBySession, sessionID)
	}
	server.mutex.Unlock()
	server.wait.Done()
}

func (connection *connection) readLoop() {
	for {
		messageType, message, err := connection.socket.Read(connection.ctx)
		if err != nil {
			status := websocket.CloseStatus(err)
			if status != websocket.StatusNormalClosure && status != websocket.StatusGoingAway && status != websocket.StatusServiceRestart && status != websocket.StatusMessageTooBig {
				connection.server.options.Logger.WarnContext(connection.ctx, "realtime_read_failed", "connection_id", connection.id, "result_code", "READ_ERROR", "close_status", status)
			}
			connection.closeWith(websocket.StatusNormalClosure, "connection closed")
			return
		}
		if messageType != websocket.MessageText {
			<-connection.sendErrorAndClose(ClientEnvelope{}, "UNSUPPORTED_DATA", false, websocket.StatusUnsupportedData, "text messages only")
			return
		}
		if !utf8.Valid(message) {
			<-connection.sendErrorAndClose(ClientEnvelope{}, "INVALID_TEXT", false, websocket.StatusInvalidFramePayloadData, "message is not valid UTF-8")
			return
		}
		if !connection.limiter.allow(connection.server.options.Now().UTC()) {
			<-connection.sendErrorAndClose(ClientEnvelope{}, "RATE_LIMITED", true, websocket.StatusPolicyViolation, "message rate exceeded")
			return
		}
		if connection.isDraining() {
			connection.closeWith(websocket.StatusServiceRestart, "server draining")
			return
		}
		if _, err := connection.server.options.Identity.ValidateSession(connection.ctx, connection.session.ID); err != nil {
			connection.server.options.Logger.WarnContext(connection.ctx, "realtime_message_rejected", "connection_id", connection.id, "result_code", "SESSION_INACTIVE")
			<-connection.sendErrorAndClose(ClientEnvelope{}, "SESSION_INACTIVE", false, websocket.StatusPolicyViolation, "session inactive")
			return
		}
		envelope, err := decodeClientEnvelope(message)
		if err != nil {
			connection.invalidMessages++
			connection.server.options.Logger.WarnContext(connection.ctx, "realtime_message_rejected", "connection_id", connection.id, "result_code", "INVALID_MESSAGE")
			if connection.invalidMessages >= 3 {
				<-connection.sendErrorAndClose(ClientEnvelope{}, "INVALID_MESSAGE", false, websocket.StatusProtocolError, "repeated invalid messages")
				return
			}
			connection.sendError(ClientEnvelope{}, "INVALID_MESSAGE", false, nil, nil)
			continue
		}
		connection.invalidMessages = 0
		if envelope.Kind == kindControl {
			connection.sendControl("heartbeat.ack", "", "", map[string]any{})
			continue
		}
		if envelope.Name == nameSubscribe || envelope.Name == nameResume {
			connection.handleSubscription(envelope)
			continue
		}
		connection.handleMutation(envelope)
	}
}

func (connection *connection) handleSubscription(envelope ClientEnvelope) {
	payload, _ := decodeSubscriptionPayload(envelope.Payload)
	aggregate, err := connection.server.options.Tables.Refresh(connection.ctx, envelope.TableID)
	if err != nil {
		connection.sendError(envelope, "TABLE_NOT_FOUND", false, nil, nil)
		return
	}
	projection, domainError := table.Project(aggregate, connection.session.ID)
	if domainError != nil {
		connection.sendError(envelope, "TABLE_ACCESS_DENIED", false, nil, nil)
		return
	}
	frames, syncMode := connection.recoveryFrames(envelope, payload.LastSeenSeq, projection)
	ack, err := json.Marshal(ackEnvelope{
		Version: protocolVersion, Kind: "ack", Name: "table.subscribed", RequestID: envelope.RequestID,
		TableID: envelope.TableID, Revision: projection.Revision, Seq: projection.LastSeq,
		Payload: ackPayload{Status: "accepted", SyncMode: syncMode},
	})
	if err != nil {
		connection.closeWith(websocket.StatusInternalError, "encode failure")
		return
	}
	frames = append([][]byte{ack}, frames...)
	if !connection.server.broker.subscribe(connection, envelope.TableID, frames) {
		return
	}
	connection.server.options.Logger.InfoContext(connection.ctx, "realtime_reconnect_completed",
		"connection_id", connection.id,
		"table_id", envelope.TableID,
		"result_code", syncMode,
		"revision", projection.Revision,
		"seq", projection.LastSeq,
	)
	latest, err := connection.server.options.Tables.Refresh(connection.ctx, envelope.TableID)
	if err == nil && latest.LastSeq > aggregate.LastSeq {
		latestProjection, projectionError := table.Project(latest, connection.session.ID)
		if projectionError == nil {
			if frame, frameError := snapshotFrame(latestProjection); frameError == nil && !connection.enqueue(outboundFrame{message: frame}) {
				connection.closeSlowConsumer()
			}
		}
	}
}

func (connection *connection) recoveryFrames(envelope ClientEnvelope, lastSeenSeq int64, projection table.Projection) ([][]byte, string) {
	if lastSeenSeq >= projection.LastSeq || projection.LastSeq-lastSeenSeq > int64(connection.server.options.RecoveryLimit) {
		frame, err := snapshotFrame(projection)
		if err != nil {
			return nil, "snapshot"
		}
		return [][]byte{frame}, "snapshot"
	}
	events, err := connection.server.options.Events.ListEventsAfter(connection.ctx, envelope.TableID, lastSeenSeq, connection.server.options.RecoveryLimit+1)
	if err == nil && contiguousEvents(events, lastSeenSeq, projection.LastSeq) {
		frames, frameError := eventFrames(events, projection)
		if frameError == nil {
			return frames, "events"
		}
	}
	connection.server.options.Logger.WarnContext(connection.ctx, "realtime_recovery_snapshot_fallback",
		"connection_id", connection.id,
		"table_id", envelope.TableID,
		"result_code", "SNAPSHOT_FALLBACK",
	)
	frame, frameError := snapshotFrame(projection)
	if frameError != nil {
		return nil, "snapshot"
	}
	return [][]byte{frame}, "snapshot"
}

func (connection *connection) handleMutation(envelope ClientEnvelope) {
	if connection.subscription() != envelope.TableID {
		connection.sendError(envelope, "TABLE_NOT_SUBSCRIBED", false, nil, nil)
		return
	}
	aggregate, err := connection.server.options.Tables.Snapshot(connection.ctx, envelope.TableID)
	if err != nil {
		connection.sendError(envelope, "TABLE_NOT_FOUND", false, nil, nil)
		return
	}
	projection, domainError := table.Project(aggregate, connection.session.ID)
	if domainError != nil {
		connection.sendError(envelope, "TABLE_ACCESS_DENIED", false, nil, nil)
		return
	}
	controllerEpoch := int64(0)
	if projection.ViewerSeat.Valid() {
		if envelope.ControllerEpoch == nil {
			revision, seq := aggregate.Revision, aggregate.LastSeq
			connection.sendError(envelope, string(table.ErrorStaleController), false, &revision, &seq)
			return
		}
		controllerEpoch = *envelope.ControllerEpoch
	} else if envelope.ControllerEpoch != nil || envelope.Name == nameTakeover {
		revision, seq := aggregate.Revision, aggregate.LastSeq
		connection.sendError(envelope, string(table.ErrorSeatRequired), false, &revision, &seq)
		return
	}
	connection.server.randomMutex.Lock()
	command, err := tableCommand(envelope, connection.server.options.Random, connection.server.options.Now().UTC())
	connection.server.randomMutex.Unlock()
	if err != nil {
		connection.sendError(envelope, "INVALID_COMMAND", false, nil, nil)
		return
	}
	command.SessionID = connection.session.ID
	command.ControllerEpoch = controllerEpoch
	result, err := connection.server.options.Tables.Submit(connection.ctx, table.CommandRequest{
		TableID: envelope.TableID, SessionID: connection.session.ID, RequestID: envelope.RequestID,
		ExpectedRevision: *envelope.ExpectedRevision, Command: command,
	})
	if err != nil {
		code := "INTERNAL_ERROR"
		retryable := true
		if errors.Is(err, table.ErrActorQueueFull) || errors.Is(err, table.ErrActorRegistryDraining) {
			code = "SERVER_BUSY"
		}
		connection.sendError(envelope, code, retryable, nil, nil)
		return
	}
	if result.Outcome.Status == table.CommandStatusRejected {
		revision, seq := result.Outcome.Revision, result.Outcome.LastSeq
		connection.sendError(envelope, string(result.Outcome.ErrorCode), result.Outcome.ErrorCode == table.ErrorStateChanged, &revision, &seq)
		return
	}
	ack, err := json.Marshal(ackEnvelope{
		Version: protocolVersion, Kind: "ack", Name: "command.accepted", RequestID: envelope.RequestID,
		TableID: envelope.TableID, Revision: result.Outcome.Revision, Seq: result.Outcome.LastSeq,
		Payload: ackPayload{Status: "accepted", Duplicate: result.Duplicate},
	})
	if err != nil || !connection.enqueue(outboundFrame{message: ack}) {
		connection.closeSlowConsumer()
		return
	}
	if err := connection.server.broker.publishResult(connection.ctx, result); err != nil {
		connection.server.options.Logger.ErrorContext(connection.ctx, "realtime_broadcast_failed",
			"connection_id", connection.id,
			"table_id", envelope.TableID,
			"request_id", envelope.RequestID,
			"result_code", "PROJECTION_ERROR",
		)
	}
}

func (connection *connection) writeLoop() {
	defer close(connection.done)
	pingTicker := time.NewTicker(connection.server.options.PingInterval)
	defer pingTicker.Stop()
	expiresIn := connection.session.ExpiresAt.Sub(connection.server.options.Now().UTC())
	if expiresIn < 0 {
		expiresIn = 0
	}
	expiryTimer := time.NewTimer(expiresIn)
	defer expiryTimer.Stop()
	for {
		select {
		case frame := <-connection.outbound:
			connection.releaseQueuedBytes(len(frame.message))
			writeCtx, cancel := context.WithTimeout(context.Background(), connection.server.options.WriteTimeout)
			err := connection.socket.Write(writeCtx, websocket.MessageText, frame.message)
			cancel()
			if frame.delivered != nil {
				close(frame.delivered)
			}
			if err != nil {
				connection.closeWith(websocket.StatusGoingAway, "write failed")
				connection.performClose()
				return
			}
			if frame.closeStatus != 0 {
				connection.closeWith(frame.closeStatus, frame.closeReason)
				connection.performClose()
				return
			}
		case <-pingTicker.C:
			pingCtx, cancel := context.WithTimeout(context.Background(), connection.server.options.PongTimeout)
			err := connection.socket.Ping(pingCtx)
			cancel()
			if err != nil {
				connection.closeWith(websocket.StatusGoingAway, "heartbeat timeout")
				connection.performClose()
				return
			}
		case <-expiryTimer.C:
			connection.closeWith(websocket.StatusPolicyViolation, "session expired")
			connection.performClose()
			return
		case <-connection.ctx.Done():
			connection.performClose()
			return
		}
	}
}

func (connection *connection) enqueue(frame outboundFrame) bool {
	if connection.ctx.Err() != nil {
		return false
	}
	connection.queueMutex.Lock()
	defer connection.queueMutex.Unlock()
	if connection.queuedBytes+len(frame.message) > connection.server.options.OutboundQueueBytes {
		return false
	}
	select {
	case connection.outbound <- frame:
		connection.queuedBytes += len(frame.message)
		return true
	default:
		return false
	}
}

func (connection *connection) releaseQueuedBytes(bytes int) {
	connection.queueMutex.Lock()
	connection.queuedBytes -= bytes
	connection.queueMutex.Unlock()
}

func (connection *connection) sendError(envelope ClientEnvelope, code string, retryable bool, revision *int64, seq *int64) {
	frame, err := json.Marshal(errorEnvelope{
		Version: protocolVersion, Kind: "error", Name: "command.rejected", RequestID: envelope.RequestID,
		TableID: envelope.TableID, Code: code, Retryable: retryable, Revision: revision, Seq: seq, Payload: map[string]any{},
	})
	if err != nil || !connection.enqueue(outboundFrame{message: frame}) {
		connection.closeSlowConsumer()
	}
}

func (connection *connection) sendErrorAndClose(envelope ClientEnvelope, code string, retryable bool, status websocket.StatusCode, reason string) <-chan struct{} {
	delivered := make(chan struct{})
	frame, err := json.Marshal(errorEnvelope{
		Version: protocolVersion, Kind: "error", Name: "command.rejected", RequestID: envelope.RequestID,
		TableID: envelope.TableID, Code: code, Retryable: retryable, Payload: map[string]any{},
	})
	if err != nil || !connection.enqueue(outboundFrame{message: frame, closeStatus: status, closeReason: reason, delivered: delivered}) {
		close(delivered)
		connection.closeWith(status, reason)
	}
	return delivered
}

func (connection *connection) sendControl(name string, requestID string, tableID string, payload map[string]any) {
	frame, err := json.Marshal(controlEnvelope{Version: protocolVersion, Kind: "control", Name: name, RequestID: requestID, TableID: tableID, Payload: payload})
	if err != nil || !connection.enqueue(outboundFrame{message: frame}) {
		connection.closeSlowConsumer()
	}
}

func (connection *connection) shutdown() {
	connection.stateMutex.Lock()
	if connection.draining || connection.closeRequested {
		connection.stateMutex.Unlock()
		return
	}
	connection.draining = true
	connection.stateMutex.Unlock()
	frame, err := json.Marshal(controlEnvelope{Version: protocolVersion, Kind: "control", Name: "server.draining", Payload: map[string]any{}})
	if err != nil || !connection.enqueue(outboundFrame{message: frame, closeStatus: websocket.StatusServiceRestart, closeReason: "server draining"}) {
		connection.closeWith(websocket.StatusServiceRestart, "server draining")
	}
}

func (connection *connection) closeSlowConsumer() {
	connection.closeWith(websocket.StatusPolicyViolation, "slow consumer")
}

func (connection *connection) closePolicyAfterQueued(reason string) {
	frame, err := json.Marshal(controlEnvelope{Version: protocolVersion, Kind: "control", Name: "table.access_revoked", Payload: map[string]any{}})
	if err != nil || !connection.enqueue(outboundFrame{message: frame, closeStatus: websocket.StatusPolicyViolation, closeReason: reason}) {
		connection.closeWith(websocket.StatusPolicyViolation, reason)
	}
}

func (connection *connection) closeWith(status websocket.StatusCode, reason string) {
	connection.stateMutex.Lock()
	if !connection.closeRequested {
		connection.closeRequested = true
		connection.closeStatus = status
		connection.closeReason = reason
	}
	connection.stateMutex.Unlock()
	connection.cancel()
}

func (connection *connection) performClose() {
	connection.stateMutex.Lock()
	status, reason := connection.closeStatus, connection.closeReason
	connection.stateMutex.Unlock()
	if status == 0 {
		status, reason = websocket.StatusNormalClosure, "connection closed"
	}
	if err := connection.socket.Close(status, reason); err != nil && !errors.Is(err, net.ErrClosed) {
		connection.server.options.Logger.Warn("realtime_close_failed", "connection_id", connection.id, "result_code", "CLOSE_ERROR")
	}
}

func (connection *connection) subscription() string {
	connection.stateMutex.Lock()
	defer connection.stateMutex.Unlock()
	return connection.tableID
}

func (connection *connection) setSubscription(tableID string) {
	connection.stateMutex.Lock()
	connection.tableID = tableID
	connection.stateMutex.Unlock()
}

func (connection *connection) isDraining() bool {
	connection.stateMutex.Lock()
	defer connection.stateMutex.Unlock()
	return connection.draining
}

func (bucket *tokenBucket) allow(now time.Time) bool {
	elapsed := now.Sub(bucket.last).Seconds()
	if elapsed > 0 {
		bucket.tokens = min(bucket.burst, bucket.tokens+elapsed*bucket.rate)
		bucket.last = now
	}
	if bucket.tokens < 1 {
		return false
	}
	bucket.tokens--
	return true
}

func contiguousEvents(events []table.PersistedEvent, afterSeq int64, lastSeq int64) bool {
	if len(events) == 0 || events[0].Seq != afterSeq+1 || events[len(events)-1].Seq != lastSeq {
		return false
	}
	for _index := 1; _index < len(events); _index++ {
		if events[_index].Seq != events[_index-1].Seq+1 {
			return false
		}
	}
	return true
}

func (server *Server) writeHandshakeError(writer http.ResponseWriter, status int, code string) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.Header().Set("Cache-Control", "private, no-store")
	writer.WriteHeader(status)
	if err := json.NewEncoder(writer).Encode(map[string]any{"code": code, "status": status}); err != nil {
		server.options.Logger.Warn("realtime_handshake_response_failed", "result_code", "WRITE_ERROR")
	}
}
