package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/mahesayuztar/bridgeyok/apps/api/internal/table"
)

type broker struct {
	logger *slog.Logger
	mutex  sync.Mutex
	rooms  map[string]map[*connection]struct{}
}

func newBroker(logger *slog.Logger) *broker {
	return &broker{logger: logger, rooms: make(map[string]map[*connection]struct{})}
}

func (broker *broker) subscribe(client *connection, tableID string, initialFrames [][]byte) bool {
	broker.mutex.Lock()
	defer broker.mutex.Unlock()
	if currentTableID := client.subscription(); currentTableID != "" {
		broker.removeLocked(client, currentTableID)
	}
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
	client.setSubscription(tableID)
	return true
}

func (broker *broker) unsubscribe(client *connection) {
	broker.mutex.Lock()
	defer broker.mutex.Unlock()
	tableID := client.subscription()
	if tableID == "" {
		return
	}
	broker.removeLocked(client, tableID)
	client.setSubscription("")
}

func (broker *broker) publishResult(ctx context.Context, result table.CommandResult) error {
	if result.Duplicate || result.Outcome.Status != table.CommandStatusAccepted || len(result.Events) == 0 {
		return nil
	}
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
	return nil
}

func (broker *broker) publishSnapshot(ctx context.Context, aggregate table.Aggregate) error {
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
