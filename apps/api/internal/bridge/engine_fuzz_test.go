package bridge

import (
	"reflect"
	"testing"
)

func FuzzDecide(f *testing.F) {
	deal, err := GenerateDeal(constantReader(0x80))
	if err != nil {
		f.Fatalf("GenerateDeal() error = %v", err)
	}
	auctionState, err := NewBoard(1, deal)
	if err != nil {
		f.Fatalf("NewBoard() error = %v", err)
	}
	playState := auctionState
	for _, call := range []Call{Bid(1, StrainNoTrump), Pass(), Pass(), Pass()} {
		decision, domainError := Decide(playState, MakeCallCommand(playState.Turn, call))
		if domainError != nil {
			f.Fatalf("Decide(setup call) error = %v", domainError)
		}
		playState = decision.NextState
	}

	for _, seed := range [][]byte{{}, {0}, {1, 2, 3, 4, 5, 6}, {255, 255, 255, 255, 255, 255}} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		state := auctionState
		if byteAt(data, 0)%2 == 1 {
			state = playState
		}
		seats := [...]Seat{North, East, South, West}
		actor := seats[int(byteAt(data, 1))%len(seats)]
		var command Command
		if byteAt(data, 2)%2 == 0 {
			kinds := [...]CallKind{CallPass, CallBid, CallDouble, CallRedouble, "INVALID"}
			strainValues := [...]Strain{StrainClubs, StrainDiamonds, StrainHearts, StrainSpades, StrainNoTrump, "INVALID"}
			call := Call{
				Kind:   kinds[int(byteAt(data, 3))%len(kinds)],
				Level:  int(byteAt(data, 4)) - 2,
				Strain: strainValues[int(byteAt(data, 5))%len(strainValues)],
			}
			if call.Kind != CallBid && byteAt(data, 6)%2 == 0 {
				call.Level = 0
				call.Strain = ""
			}
			command = MakeCallCommand(actor, call)
		} else {
			suitValues := [...]Suit{Clubs, Diamonds, Hearts, Spades, "INVALID"}
			rankValues := [...]Rank{Two, Three, Four, Five, Six, Seven, Eight, Nine, Ten, Jack, Queen, King, Ace, "INVALID"}
			command = PlayCardCommand(actor, Card{
				Suit: suitValues[int(byteAt(data, 3))%len(suitValues)],
				Rank: rankValues[int(byteAt(data, 4))%len(rankValues)],
			})
		}

		before := state.clone()
		first, firstError := Decide(state, command)
		second, secondError := Decide(state, command)
		if !reflect.DeepEqual(first, second) || !reflect.DeepEqual(firstError, secondError) {
			t.Fatal("Decide() is not deterministic")
		}
		if !reflect.DeepEqual(state, before) {
			t.Fatal("Decide() mutated input state")
		}
		if firstError == nil {
			if err := first.NextState.ValidateInvariants(); err != nil {
				t.Fatalf("accepted decision violates invariants: %v", err)
			}
		}
	})
}

func byteAt(data []byte, _index int) byte {
	if _index >= len(data) {
		return 0
	}
	return data[_index]
}
