package realtime

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/mahesayuztar/bridgeyok/apps/api/internal/table"
)

const protocolTableID = "0f4b9a5b-0ea8-4ad6-a866-e576ccd8be31"

func TestDecodeClientEnvelope(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		message string
		wantErr bool
	}{
		{name: "heartbeat", message: `{"v":1,"kind":"control","name":"client.heartbeat","payload":{}}`},
		{name: "subscribe", message: commandMessage("table.subscribe", `{"last_seen_seq":0}`, "")},
		{name: "ready mutation", message: commandMessage("table.set_ready", `{"ready":true}`, `,"expected_revision":2,"controller_epoch":1`)},
		{name: "pass call", message: commandMessage("game.make_call", `{"call":{"kind":"PASS"}}`, `,"expected_revision":2,"controller_epoch":1`)},
		{name: "unknown envelope field", message: `{"v":1,"kind":"control","name":"client.heartbeat","payload":{},"extra":true}`, wantErr: true},
		{name: "unknown command", message: commandMessage("table.unknown", `{}`, `,"expected_revision":0`), wantErr: true},
		{name: "mutation missing revision", message: commandMessage("table.lock", `{"locked":true}`, ""), wantErr: true},
		{name: "invalid controller epoch", message: commandMessage("table.lock", `{"locked":true}`, `,"expected_revision":0,"controller_epoch":0`), wantErr: true},
		{name: "extra payload field", message: commandMessage("table.take_seat", `{"seat":"N","hidden":true}`, `,"expected_revision":0`), wantErr: true},
		{name: "trailing value", message: `{"v":1,"kind":"control","name":"client.heartbeat","payload":{}} {}`, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := decodeClientEnvelope([]byte(test.message))
			if (err != nil) != test.wantErr {
				t.Fatalf("decodeClientEnvelope() error = %v, wantErr %t", err, test.wantErr)
			}
		})
	}
}

func TestTableCommandMapsValidatedMutation(t *testing.T) {
	t.Parallel()

	envelope, err := decodeClientEnvelope([]byte(commandMessage("table.takeover", `{}`, `,"expected_revision":3,"controller_epoch":2`)))
	if err != nil {
		t.Fatalf("decodeClientEnvelope() error = %v", err)
	}
	now := time.Date(2026, time.August, 30, 12, 0, 0, 0, time.UTC)
	command, err := tableCommand(envelope, bytes.NewReader(make([]byte, 128)), now)
	if err != nil {
		t.Fatalf("tableCommand() error = %v", err)
	}
	if command.Name != table.CommandTakeoverControl || command.ControllerEpoch != 2 || !command.OccurredAt.Equal(now) {
		t.Fatalf("tableCommand() = %+v", command)
	}
}

func commandMessage(name string, payload string, fields string) string {
	return fmt.Sprintf(`{"v":1,"kind":"command","name":%q,"request_id":"request_01","table_id":%q%s,"payload":%s}`, name, protocolTableID, fields, payload)
}
