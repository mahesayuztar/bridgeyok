//go:build testfixture

// Package bridgefixture serializes deterministic engine histories for tests only.
package bridgefixture

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"

	"github.com/mahesayuztar/bridgeyok/apps/api/internal/bridge"
)

// SchemaVersion identifies the current deterministic fixture envelope.
const SchemaVersion = 1

// Step preserves one actor command and the exact events it must produce.
type Step struct {
	Command bridge.Command `json:"command"`
	Events  []bridge.Event `json:"events"`
}

// Fixture contains a complete deterministic board scenario for replay tests.
type Fixture struct {
	SchemaVersion  int            `json:"schemaVersion"`
	RulesetVersion string         `json:"rulesetVersion"`
	InitialState   bridge.State   `json:"initialState"`
	Steps          []Step         `json:"steps"`
	ExpectedResult *bridge.Result `json:"expectedResult"`
}

// Replay validates command decisions and durable event replay for the complete scenario.
func (fixture Fixture) Replay() (bridge.State, error) {
	if fixture.SchemaVersion != SchemaVersion {
		return bridge.State{}, fmt.Errorf("unsupported fixture schema version %d", fixture.SchemaVersion)
	}
	if fixture.RulesetVersion != bridge.RulesetVersion {
		return bridge.State{}, fmt.Errorf("unsupported fixture ruleset version %q", fixture.RulesetVersion)
	}
	if fixture.Steps == nil {
		return bridge.State{}, fmt.Errorf("steps must be initialized")
	}
	if fixture.ExpectedResult == nil {
		return bridge.State{}, fmt.Errorf("expected result is required")
	}

	state := fixture.InitialState
	for _stepIndex, step := range fixture.Steps {
		if step.Events == nil {
			return bridge.State{}, fmt.Errorf("step %d events must be initialized", _stepIndex)
		}
		decision, domainError := bridge.Decide(state, step.Command)
		if domainError != nil {
			return bridge.State{}, fmt.Errorf("step %d decide: %w", _stepIndex, domainError)
		}
		if !reflect.DeepEqual(decision.Events, step.Events) {
			return bridge.State{}, fmt.Errorf("step %d events do not match decision", _stepIndex)
		}
		replayed, err := bridge.Reduce(state, step.Events)
		if err != nil {
			return bridge.State{}, fmt.Errorf("step %d replay: %w", _stepIndex, err)
		}
		if !reflect.DeepEqual(replayed, decision.NextState) {
			return bridge.State{}, fmt.Errorf("step %d replay state does not match decision", _stepIndex)
		}
		state = decision.NextState
	}
	if state.Phase != bridge.PhaseBoardScored || state.Result == nil {
		return bridge.State{}, fmt.Errorf("fixture does not end with a scored board")
	}
	if !reflect.DeepEqual(state.Result, fixture.ExpectedResult) {
		return bridge.State{}, fmt.Errorf("fixture result does not match expected result")
	}
	return state, nil
}

// Marshal validates a fixture and returns stable indented JSON.
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
