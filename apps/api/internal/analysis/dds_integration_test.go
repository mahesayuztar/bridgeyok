//go:build integration

package analysis

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/mahesayuztar/bridgeyok/apps/api/internal/bridge"
)

func TestPinnedDDSGoldenFixtures(t *testing.T) {
	path, err := filepath.Abs("../../../../bin/bridgeyok-dds")
	if err != nil {
		t.Fatal(err)
	}
	solver, err := NewDDS(path, 10*time.Second, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range goldenFixtures(t) {
		t.Run(fixture.Name, func(t *testing.T) {
			metadata, err := bridge.MetadataForBoard(fixture.BoardNumber)
			if err != nil {
				t.Fatal(err)
			}
			result, err := solver.Solve(t.Context(), fixture.Deal, metadata)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(result, fixture.Expected) {
				t.Fatalf("DDS golden mismatch\ngot %+v\nwant %+v", result, fixture.Expected)
			}
		})
	}
}

func TestPinnedDDSCurrentPositions(t *testing.T) {
	path, err := filepath.Abs("../../../../bin/bridgeyok-dds")
	if err != nil {
		t.Fatal(err)
	}
	solver, err := NewDDS(path, 10*time.Second, 1)
	if err != nil {
		t.Fatal(err)
	}
	states, _ := positionStates(t)
	for _, step := range []int{0, 1, 2, 3, 4, 45, 46, 47, 48, 49, 50, 51} {
		state := states[step]
		predictions, err := solver.SolvePosition(t.Context(), state)
		if err != nil {
			t.Fatalf("position %d: %v", step, err)
		}
		actor := state.Turn
		if actor == state.Auction.Contract.Dummy() {
			actor = state.Auction.Contract.Declarer
		}
		legal, _ := state.LegalCards(actor)
		if len(predictions) != len(legal) {
			t.Fatalf("position %d omitted legal cards", step)
		}
		for _, prediction := range predictions {
			decision, err := bridge.Decide(state, bridge.PlayCardCommand(actor, prediction.Card))
			if err != nil {
				t.Fatal("solver returned illegal card")
			}
			next := decision.NextState
			if next.Phase == bridge.PhaseBoardScored {
				expected := next.TricksNS
				if state.Turn.Partnership() == bridge.EastWest {
					expected = next.TricksEW
				}
				if prediction.Tricks != expected {
					t.Fatalf("final-card score got %d want %d", prediction.Tricks, expected)
				}
			} else if step >= 45 {
				following, err := solver.SolvePosition(t.Context(), next)
				if err != nil {
					t.Fatal(err)
				}
				best := -1
				for _, candidate := range following {
					if candidate.Tricks > best {
						best = candidate.Tricks
					}
				}
				if next.Turn.Partnership() != state.Turn.Partnership() {
					best = 13 - best
				}
				if prediction.Tricks != best {
					t.Fatalf("position %d card %+v: score %d != continuation %d", step, prediction.Card, prediction.Tricks, best)
				}
			}
		}
	}
}
