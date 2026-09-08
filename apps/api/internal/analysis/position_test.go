package analysis

import (
	"reflect"
	"testing"

	"github.com/mahesayuztar/bridgeyok/apps/api/internal/bridge"
)

func positionStates(t *testing.T) ([]bridge.State, bridge.Deal) {
	t.Helper()
	cards := goldenFixtures(t)[0].Deal
	state, err := bridge.NewBoard(1, cards)
	if err != nil {
		t.Fatal(err)
	}
	for _, call := range []bridge.Call{bridge.Bid(1, bridge.StrainNoTrump), bridge.Pass(), bridge.Pass(), bridge.Pass()} {
		decision, err := bridge.Decide(state, bridge.MakeCallCommand(state.Turn, call))
		if err != nil {
			t.Fatal(err)
		}
		state = decision.NextState
	}
	states := []bridge.State{state}
	for state.Phase != bridge.PhaseBoardScored {
		actor := state.Turn
		if actor == state.Auction.Contract.Dummy() {
			actor = state.Auction.Contract.Declarer
		}
		cards, err := state.LegalCards(actor)
		if err != nil {
			t.Fatal(err)
		}
		decision, err := bridge.Decide(state, bridge.PlayCardCommand(actor, cards[0]))
		if err != nil {
			t.Fatal(err)
		}
		state = decision.NextState
		states = append(states, state)
	}
	return states, cards
}

func TestReplayPosition(t *testing.T) {
	states, cards := positionStates(t)
	for _step, expected := range states {
		actual, err := ReplayPosition(states[52], cards, _step)
		if err != nil || !reflect.DeepEqual(actual, expected) {
			t.Fatalf("card %d: reconstructed state differs: %v", _step, err)
		}
	}
	for _, step := range []int{-1, 53} {
		if _, err := ReplayPosition(states[52], cards, step); err == nil {
			t.Fatal("accepted invalid cursor")
		}
	}
}
