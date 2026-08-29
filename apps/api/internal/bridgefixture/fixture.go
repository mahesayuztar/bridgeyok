//go:build testfixture

// Package bridgefixture serializes deterministic engine histories for tests only.
package bridgefixture

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/mahesayuztar/bridgeyok/apps/api/internal/bridge"
)

// Fixture contains a valid initial board and the events replayed from it.
type Fixture struct {
	InitialState bridge.State   `json:"initialState"`
	Events       []bridge.Event `json:"events"`
}

// Replay validates and reduces the complete fixture history.
func (fixture Fixture) Replay() (bridge.State, error) {
	if fixture.Events == nil {
		return bridge.State{}, fmt.Errorf("events must be initialized")
	}
	state, err := bridge.Reduce(fixture.InitialState, fixture.Events)
	if err != nil {
		return bridge.State{}, fmt.Errorf("replay fixture: %w", err)
	}
	return state, nil
}

// Marshal validates a fixture and returns canonical indented JSON.
func Marshal(fixture Fixture) ([]byte, error) {
	if _, err := fixture.Replay(); err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(fixture, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal fixture: %w", err)
	}
	return append(data, '\n'), nil
}

// Unmarshal strictly decodes and validates one fixture document.
func Unmarshal(data []byte) (Fixture, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	var fixture Fixture
	if err := decoder.Decode(&fixture); err != nil {
		return Fixture{}, fmt.Errorf("decode fixture: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return Fixture{}, fmt.Errorf("decode fixture: multiple JSON values")
		}
		return Fixture{}, fmt.Errorf("decode fixture trailing data: %w", err)
	}
	if _, err := fixture.Replay(); err != nil {
		return Fixture{}, err
	}
	return fixture, nil
}
