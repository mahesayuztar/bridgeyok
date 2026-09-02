package realtime

import (
	"encoding/json"
	"sort"
	"time"

	"github.com/mahesayuztar/bridgeyok/apps/api/internal/table"
)

type presenceEntry struct {
	participantID string
	connections   int
	offlineSince  time.Time
}

type presenceParticipant struct {
	ParticipantID string `json:"participantId"`
	Online        bool   `json:"online"`
	OfflineSince  string `json:"offlineSince,omitempty"`
}

func (broker *broker) syncParticipantsLocked(tableID string, participants []table.Participant) {
	entries := broker.presence[tableID]
	if entries == nil {
		entries = make(map[string]*presenceEntry)
		broker.presence[tableID] = entries
	}
	active := make(map[string]struct{}, len(participants))
	for _, participant := range participants {
		if participant.LeftAt != nil {
			continue
		}
		active[participant.ID] = struct{}{}
		entry := entries[participant.ID]
		if entry == nil {
			entry = &presenceEntry{
				participantID: participant.ID,
				offlineSince:  broker.now().UTC(),
			}
			entries[participant.ID] = entry
		}
	}
	for participantID := range entries {
		if _, exists := active[participantID]; exists {
			continue
		}
		delete(entries, participantID)
	}
}

func (broker *broker) reconcileParticipants(aggregate table.Aggregate) {
	broker.mutex.Lock()
	defer broker.mutex.Unlock()
	broker.syncParticipantsLocked(aggregate.ID, aggregate.Participants)
}

func (broker *broker) markOnlineLocked(client *connection, tableID string, participantID string) {
	entry := broker.presence[tableID][participantID]
	if entry == nil {
		return
	}
	wasOnline := entry.connections > 0
	entry.connections++
	entry.offlineSince = time.Time{}
	if !wasOnline {
		broker.publishPresenceChangedLocked(tableID, entry, client)
	}
}

func (broker *broker) markOfflineLocked(client *connection, tableID string, participantID string) {
	entry := broker.presence[tableID][participantID]
	if entry == nil || entry.connections == 0 {
		return
	}
	entry.connections--
	if entry.connections > 0 {
		return
	}
	entry.offlineSince = broker.now().UTC()
	broker.publishPresenceChangedLocked(tableID, entry, client)
}

func (broker *broker) presenceSnapshotFrameLocked(tableID string) ([]byte, error) {
	participants := make([]presenceParticipant, 0, len(broker.presence[tableID]))
	for _, entry := range broker.presence[tableID] {
		participants = append(participants, presencePayload(entry))
	}
	sort.Slice(participants, func(_firstIndex int, _secondIndex int) bool {
		return participants[_firstIndex].ParticipantID < participants[_secondIndex].ParticipantID
	})
	return json.Marshal(controlEnvelope{
		Version: protocolVersion, Kind: "control", Name: "presence.snapshot", TableID: tableID,
		Payload: map[string]any{"participants": participants},
	})
}

func (broker *broker) publishPresenceSnapshot(tableID string, connections []*connection) {
	broker.mutex.Lock()
	frame, err := broker.presenceSnapshotFrameLocked(tableID)
	broker.mutex.Unlock()
	if err != nil {
		return
	}
	for _, connection := range connections {
		connection.enqueue(outboundFrame{message: frame})
	}
}

func (broker *broker) publishPresenceChangedLocked(tableID string, entry *presenceEntry, excluded *connection) {
	frame, err := json.Marshal(controlEnvelope{
		Version: protocolVersion, Kind: "control", Name: "presence.changed", TableID: tableID,
		Payload: map[string]any{"participant": presencePayload(entry)},
	})
	if err != nil {
		return
	}
	for connection := range broker.rooms[tableID] {
		if connection != excluded {
			connection.enqueue(outboundFrame{message: frame})
		}
	}
}

func presencePayload(entry *presenceEntry) presenceParticipant {
	payload := presenceParticipant{ParticipantID: entry.participantID, Online: entry.connections > 0}
	if !entry.offlineSince.IsZero() {
		payload.OfflineSince = entry.offlineSince.Format(time.RFC3339Nano)
	}
	return payload
}
