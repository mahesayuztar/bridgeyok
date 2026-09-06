package deal

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/bridge"
)

func TestSourcesProduceValidPrivateRecords(t *testing.T) {
	t.Parallel()
	initial, err := (SecureRandom{}).Generate(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := NewPrepared(initial.Deal, uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	constraint, err := NewConstraint(map[bridge.Seat]HandConstraint{bridge.North: {HCP: &Range{Min: 0, Max: 37}, Suits: map[bridge.Suit]Range{bridge.Spades: {Min: 0, Max: 13}}}}, 1, uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name   string
		source Source
	}{{"secure_random", SecureRandom{}}, {"prepared", prepared}, {"constraint", constraint}} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			result, err := test.source.Generate(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if err := result.Validate(); err != nil {
				t.Fatal(err)
			}
			if result.Provenance.Type != test.name {
				t.Fatal("wrong provenance")
			}
			encoded, err := json.Marshal(result)
			if err != nil {
				t.Fatal(err)
			}
			var restored Result
			if err := json.Unmarshal(encoded, &restored); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(restored, result) {
				t.Fatal("source record changed after hydration")
			}
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			if _, err := test.source.Generate(ctx); !errors.Is(err, context.Canceled) {
				t.Fatalf("canceled generation: %v", err)
			}
		})
	}
	initial.Deal.North[0] = bridge.Card{}
	first, err := prepared.Generate(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	first.Deal.North[0] = bridge.Card{}
	second, err := prepared.Generate(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if err := second.Validate(); err != nil {
		t.Fatal("prepared record shares mutable hands")
	}
}

func TestSourceFailuresReturnNoPartialDeal(t *testing.T) {
	t.Parallel()
	if _, err := NewPrepared(bridge.Deal{}, uuid.NewString()); err == nil {
		t.Fatal("invalid prepared deal accepted")
	}
	generated, err := (SecureRandom{}).Generate(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewPrepared(generated.Deal, "secret-seed"); err == nil {
		t.Fatal("unsafe reference accepted")
	}
	tests := []struct {
		name      string
		hands     map[bridge.Seat]HandConstraint
		attempts  int
		reference string
	}{
		{"empty", nil, 1, uuid.NewString()},
		{"invalid seat", map[bridge.Seat]HandConstraint{"X": {}}, 1, uuid.NewString()},
		{"invalid HCP", map[bridge.Seat]HandConstraint{bridge.North: {HCP: &Range{Min: 20, Max: 10}}}, 1, uuid.NewString()},
		{"invalid suit", map[bridge.Seat]HandConstraint{bridge.North: {Suits: map[bridge.Suit]Range{"X": {Min: 0, Max: 13}}}}, 1, uuid.NewString()},
		{"impossible hand", map[bridge.Seat]HandConstraint{bridge.North: {Suits: map[bridge.Suit]Range{bridge.Spades: {Min: 7, Max: 13}, bridge.Hearts: {Min: 7, Max: 13}}}}, 1, uuid.NewString()},
		{"impossible suit", map[bridge.Seat]HandConstraint{bridge.North: {Suits: map[bridge.Suit]Range{bridge.Spades: {Min: 7, Max: 13}}}, bridge.East: {Suits: map[bridge.Suit]Range{bridge.Spades: {Min: 7, Max: 13}}}}, 1, uuid.NewString()},
		{"impossible points", map[bridge.Seat]HandConstraint{bridge.North: {HCP: &Range{Min: 21, Max: 37}}, bridge.East: {HCP: &Range{Min: 21, Max: 37}}}, 1, uuid.NewString()},
		{"unbounded", map[bridge.Seat]HandConstraint{bridge.North: {}}, 10001, uuid.NewString()},
		{"unsafe reference", map[bridge.Seat]HandConstraint{bridge.North: {}}, 1, "seed=secret"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewConstraint(test.hands, test.attempts, test.reference); err == nil {
				t.Fatal("invalid constraints accepted")
			}
		})
	}
	source, err := NewConstraint(map[bridge.Seat]HandConstraint{bridge.North: {HCP: &Range{Min: 37, Max: 37}, Suits: map[bridge.Suit]Range{bridge.Spades: {Min: 13, Max: 13}}}}, 2, uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	result, err := source.Generate(t.Context())
	if !errors.Is(err, ErrUnavailable) || !reflect.DeepEqual(result, Result{}) {
		t.Fatal("exhaustion must return no partial deal")
	}
}

func TestConstraintMatchesAndCopiesConfiguration(t *testing.T) {
	t.Parallel()
	hcp := Range{Min: 8, Max: 16}
	suits := map[bridge.Suit]Range{bridge.Spades: {Min: 4, Max: 6}}
	source, err := NewConstraint(map[bridge.Seat]HandConstraint{bridge.North: {HCP: &hcp, Suits: suits}}, 10000, uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	hcp.Min = 37
	suits[bridge.Spades] = Range{Min: 13, Max: 13}
	result, err := source.Generate(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	points, spades := 0, 0
	for _, card := range result.Deal.North {
		if card.Suit == bridge.Spades {
			spades++
		}
		points += map[bridge.Rank]int{bridge.Ace: 4, bridge.King: 3, bridge.Queen: 2, bridge.Jack: 1}[card.Rank]
	}
	if points < 8 || points > 16 || spades < 4 || spades > 6 {
		t.Fatal("generated hand violates configured constraints")
	}
}
