package database

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"reflect"
	"testing"

	"github.com/mahesayuztar/bridgeyok/apps/api/internal/bridge"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/deal"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/table"
)

func TestBoardRecordReplay(t *testing.T) {
	cards, err := bridge.GenerateDeal(rand.NewChaCha8([32]byte{1}))
	if err != nil {
		t.Fatal(err)
	}
	initial, err := bridge.NewBoard(1, cards)
	if err != nil {
		t.Fatal(err)
	}
	record := BoardRecord{Version: 1, BoardID: "board-one", BoardNumber: 1, Source: deal.Result{Deal: cards, Provenance: deal.Provenance{Type: "prepared", Version: "test-v1"}},
		Batches: []boardRecordBatch{{Seq: 5, Revision: 4, Events: []table.Event{{Type: "BOARD_STARTED", Payload: map[string]any{"boardId": "board-one", "boardNumber": 1}}}}}}
	state := initial
	seq := int64(6)
	for _index := 0; _index < 4; _index++ {
		decision, domainError := bridge.Decide(state, bridge.MakeCallCommand(state.Turn, bridge.Pass()))
		if domainError != nil {
			t.Fatal(domainError)
		}
		batch := boardRecordBatch{Seq: seq, Revision: int64(5 + _index)}
		for _, event := range decision.Events {
			batch.Events = append(batch.Events, table.Event{Type: string(event.Type), Payload: event})
		}
		record.Batches = append(record.Batches, batch)
		state = decision.NextState
		seq += int64(len(batch.Events))
	}
	encoded, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	record.FinalHash = fmt.Sprintf("%x", sha256.Sum256(encoded))
	tests := []struct {
		name   string
		mutate func(*BoardRecord)
		valid  bool
	}{
		{name: "complete", valid: true},
		{name: "unsupported version", mutate: func(record *BoardRecord) { record.Version++ }},
		{name: "missing header", mutate: func(record *BoardRecord) { record.Batches = record.Batches[1:] }},
		{name: "missing event", mutate: func(record *BoardRecord) { record.Batches = append(record.Batches[:2], record.Batches[3:]...) }},
		{name: "duplicate revision", mutate: func(record *BoardRecord) { record.Batches[2].Revision = record.Batches[1].Revision }},
		{name: "different final state", mutate: func(record *BoardRecord) { record.FinalHash = "mismatch" }},
		{name: "wrong board identity", mutate: func(record *BoardRecord) { record.BoardID = "another-board" }},
		{name: "truncated final command", mutate: func(record *BoardRecord) {
			record.Batches[len(record.Batches)-1].Events = record.Batches[len(record.Batches)-1].Events[:1]
		}},
		{name: "mislabeled payload", mutate: func(record *BoardRecord) { record.Batches[1].Events[0].Type = "CARD_PLAYED" }},
		{name: "undo without action", mutate: func(record *BoardRecord) {
			record.Batches[0].Events[0] = table.Event{Type: "UNDO_ACCEPTED", Payload: map[string]any{}}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			encoded, err := json.Marshal(record)
			if err != nil {
				t.Fatal(err)
			}
			var candidate BoardRecord
			if err := json.Unmarshal(encoded, &candidate); err != nil {
				t.Fatal(err)
			}
			if test.mutate != nil {
				test.mutate(&candidate)
			}
			replayed, err := candidate.Replay()
			if test.valid {
				if err != nil || !reflect.DeepEqual(replayed, state) {
					t.Fatalf("replay mismatch: %v", err)
				}
			} else if err == nil {
				t.Fatal("invalid archive accepted")
			}
		})
	}
}
