package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/mahesayuztar/bridgeyok/apps/api/internal/table"
)

type broker struct {
	logger            *slog.Logger
	gracePeriod       time.Duration
	now               func() time.Time
	expireParticipant func(context.Context, string, string, uint64)
	mutex             sync.Mutex
	rooms             map[string]map[*connection]struct{}
	presence          map[string]map[string]*presenceEntry
}

func newBroker(logger *slog.Logger, gracePeriod time.Duration, now func() time.Time, expireParticipant func(context.Context, string, string, uint64)) *broker {
	return &broker{
		logger: logger, gracePeriod: gracePeriod, now: now, expireParticipant: expireParticipant,
		rooms: make(map[string]map[*connection]struct{}), presence: make(map[string]map[string]*presenceEntry),
	}
}

func (broker *broker) subscribe(client *connection, tableID string, initialFrames [][]byte, participants []table.Participant, participantID string) bool {
	broker.mutex.Lock()
	defer broker.mutex.Unlock()
	if currentTableID, _ := client.subscriptionInfo(); currentTableID != "" {
		broker.removeLocked(client, currentTableID)
	}
	broker.syncParticipantsLocked(tableID, participants)
	for _, frame := range initialFrames {
		if !client.enqueue(outboundFrame{message: frame}) {
			client.closeSlowConsumer()
			return false
		}
	}
	room := broker.rooms[tableID]
	if room == nil {
		room = make(map[*connection]struct{})
		broker.rooms[tableID] = room
	}
	room[client] = struct{}{}
	client.setSubscription(tableID, participantID)
	broker.markOnlineLocked(client, tableID, participantID)
	if frame, err := broker.presenceSnapshotFrameLocked(tableID); err == nil {
		client.enqueue(outboundFrame{message: frame})
	}
	return true
}

func (broker *broker) unsubscribe(client *connection) {
	broker.mutex.Lock()
	defer broker.mutex.Unlock()
	tableID, _ := client.subscriptionInfo()
	if tableID == "" {
		return
	}
	broker.removeLocked(client, tableID)
	client.setSubscription("", "")
}

func (broker *broker) publishResult(ctx context.Context, result table.CommandResult) error {
	if result.Duplicate || result.Outcome.Status != table.CommandStatusAccepted || len(result.Events) == 0 {
		return nil
	}
	broker.reconcileParticipants(result.Aggregate)
	connections := broker.connections(result.Aggregate.ID)
	projectionFrames := make(map[string][][]byte)
	for _, connection := range connections {
		frames, exists := projectionFrames[connection.session.ID]
		if !exists {
			projection, domainError := table.Project(result.Aggregate, connection.session.ID)
			if domainError != nil {
				broker.logger.WarnContext(ctx, "realtime_projection_rejected",
					"connection_id", connection.id,
					"table_id", result.Aggregate.ID,
					"result_code", domainError.Code,
				)
				broker.unsubscribe(connection)
				connection.closePolicyAfterQueued("table access revoked")
				continue
			}
			var err error
			frames, err = eventFrames(result.Events, projection)
			if err != nil {
				return err
			}
			projectionFrames[connection.session.ID] = frames
		}
		for _, frame := range frames {
			if !connection.enqueue(outboundFrame{message: frame}) {
				broker.logger.WarnContext(ctx, "realtime_slow_consumer",
					"connection_id", connection.id,
					"table_id", result.Aggregate.ID,
					"result_code", "OUTBOUND_QUEUE_FULL",
				)
				broker.unsubscribe(connection)
				connection.closeSlowConsumer()
				break
			}
		}
	}
	for _, automatedResult := range result.AutomatedResults {
		if err := broker.publishResult(ctx, automatedResult); err != nil {
			return err
		}
	}
	return nil
}

func (broker *broker) publishSnapshot(ctx context.Context, aggregate table.Aggregate) error {
	broker.reconcileParticipants(aggregate)
	connections := broker.connections(aggregate.ID)
	projectionFrames := make(map[string][]byte)
	for _, connection := range connections {
		frame, exists := projectionFrames[connection.session.ID]
		if !exists {
			projection, domainError := table.Project(aggregate, connection.session.ID)
			if domainError != nil {
				broker.unsubscribe(connection)
				connection.closePolicyAfterQueued("table access revoked")
				continue
			}
			var err error
			frame, err = snapshotFrame(projection)
			if err != nil {
				return err
			}
			projectionFrames[connection.session.ID] = frame
		}
		if !connection.enqueue(outboundFrame{message: frame}) {
			broker.logger.WarnContext(ctx, "realtime_slow_consumer",
				"connection_id", connection.id,
				"table_id", aggregate.ID,
				"result_code", "OUTBOUND_QUEUE_FULL",
			)
			broker.unsubscribe(connection)
			connection.closeSlowConsumer()
		}
	}
	broker.publishPresenceSnapshot(aggregate.ID, connections)
	return nil
}

func (broker *broker) connections(tableID string) []*connection {
	broker.mutex.Lock()
	defer broker.mutex.Unlock()
	room := broker.rooms[tableID]
	connections := make([]*connection, 0, len(room))
	for connection := range room {
		connections = append(connections, connection)
	}
	return connections
}

func (broker *broker) removeLocked(client *connection, tableID string) {
	room := broker.rooms[tableID]
	delete(room, client)
	if len(room) == 0 {
		delete(broker.rooms, tableID)
	}
	_, participantID := client.subscriptionInfo()
	broker.markOfflineLocked(client, tableID, participantID)
}

func (broker *broker) drain() {
	broker.mutex.Lock()
	defer broker.mutex.Unlock()
	for _, entries := range broker.presence {
		for _, entry := range entries {
			broker.cancelExpiryLocked(entry)
		}
	}
	broker.rooms = make(map[string]map[*connection]struct{})
	broker.presence = make(map[string]map[string]*presenceEntry)
}

func eventFrames(events []table.PersistedEvent, projection table.Projection) ([][]byte, error) {
	frames := make([][]byte, 0, len(events))
	for _, event := range events {
		frame, err := json.Marshal(eventEnvelope{
			Version:  protocolVersion,
			Kind:     "event",
			Name:     strings.ToLower(strings.ReplaceAll(event.Type, "_", ".")),
			TableID:  event.TableID,
			Revision: event.Revision,
			Seq:      event.Seq,
			Payload:  projectedEventPayload{EventType: event.Type, Table: projection},
		})
		if err != nil {
			return nil, fmt.Errorf("encode projected event: %w", err)
		}
		frames = append(frames, frame)
	}
	return frames, nil
}

func snapshotFrame(projection table.Projection) ([]byte, error) {
	frame, err := json.Marshal(snapshotEnvelope{
		Version:  protocolVersion,
		Kind:     "snapshot",
		Name:     "table.snapshot",
		TableID:  projection.TableID,
		Revision: projection.Revision,
		Seq:      projection.LastSeq,
		Payload:  projection,
	})
	if err != nil {
		return nil, fmt.Errorf("encode table snapshot: %w", err)
	}
	return frame, nil
}
