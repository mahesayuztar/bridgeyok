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
	midAuctionDecision, domainError := Decide(auctionState, MakeCallCommand(North, Bid(1, StrainNoTrump)))
	if domainError != nil {
		f.Fatalf("Decide(mid-auction setup) error = %v", domainError)
	}
	midAuctionState := midAuctionDecision.NextState
	playState := midAuctionState
	for _, call := range []Call{Pass(), Pass(), Pass()} {
		decision, domainError := Decide(playState, MakeCallCommand(playState.Turn, call))
		if domainError != nil {
			f.Fatalf("Decide(setup call) error = %v", domainError)
		}
		playState = decision.NextState
	}
	states := []State{auctionState, midAuctionState, playState}
	evolvedState := playState
	for _playIndex := 1; _playIndex <= 52; _playIndex++ {
		evolvedState = advanceFuzzPlay(f, evolvedState)
		switch _playIndex {
		case 1, 2, 3, 48, 51, 52:
			states = append(states, evolvedState)
		}
	}
	passedOutState := auctionState
	for passedOutState.Phase == PhaseAuction {
		decision, domainError := Decide(passedOutState, MakeCallCommand(passedOutState.Turn, Pass()))
		if domainError != nil {
			f.Fatalf("Decide(passed-out setup) error = %v", domainError)
		}
		passedOutState = decision.NextState
	}
	states = append(states, passedOutState)

	for _, seed := range [][]byte{{}, {0}, {2, 1, 3}, {1, 2, 3, 4, 5, 6}, {255, 255, 255, 255, 255, 255}} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		state := states[int(byteAt(data, 0))%len(states)]
		seats := [...]Seat{North, East, South, West}
		actor := seats[int(byteAt(data, 1))%len(seats)]
		var command Command
		switch byteAt(data, 2) % 6 {
		case 0:
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
		case 1:
			suitValues := [...]Suit{Clubs, Diamonds, Hearts, Spades, "INVALID"}
			rankValues := [...]Rank{Two, Three, Four, Five, Six, Seven, Eight, Nine, Ten, Jack, Queen, King, Ace, "INVALID"}
			command = PlayCardCommand(actor, Card{
				Suit: suitValues[int(byteAt(data, 3))%len(suitValues)],
				Rank: rankValues[int(byteAt(data, 4))%len(rankValues)],
			})
		case 2:
			legalCalls := state.Auction.LegalCalls()
			if len(legalCalls) == 0 {
				command = MakeCallCommand(actor, Pass())
			} else {
				command = MakeCallCommand(state.Turn, legalCalls[int(byteAt(data, 3))%len(legalCalls)])
			}
		case 3:
			playActor := state.Turn
			if state.Auction.Contract != nil && state.Turn == state.Auction.Contract.Dummy() {
				playActor = state.Auction.Contract.Declarer
			}
			legalCards, domainError := state.LegalCards(playActor)
			if domainError != nil || len(legalCards) == 0 {
				command = PlayCardCommand(actor, Card{Suit: "INVALID", Rank: "INVALID"})
			} else {
				command = PlayCardCommand(playActor, legalCards[int(byteAt(data, 3))%len(legalCards)])
			}
		case 4:
			command = Command{ActorSeat: actor, Name: "UNKNOWN"}
		case 5:
			call := Pass()
			card := Card{Suit: Spades, Rank: Ace}
			command = Command{ActorSeat: actor, Name: CommandPlayCard, Call: &call, Card: &card}
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
			replayed, err := Reduce(state, first.Events)
			if err != nil {
				t.Fatalf("accepted events failed replay: %v", err)
			}
			if !reflect.DeepEqual(replayed, first.NextState) {
				t.Fatal("accepted event replay differs from decision state")
			}
		} else if !reflect.DeepEqual(first, Decision{}) {
			t.Fatal("rejected command returned a partial decision")
		}
	})
}

func advanceFuzzPlay(f *testing.F, state State) State {
	f.Helper()
	actor := state.Turn
	if state.Auction.Contract != nil && state.Turn == state.Auction.Contract.Dummy() {
		actor = state.Auction.Contract.Declarer
	}
	legalCards, domainError := state.LegalCards(actor)
	if domainError != nil || len(legalCards) == 0 {
		f.Fatalf("LegalCards(%s) cards = %d, error = %v", actor, len(legalCards), domainError)
	}
	decision, domainError := Decide(state, PlayCardCommand(actor, legalCards[0]))
	if domainError != nil {
		f.Fatalf("Decide(fuzz setup play) error = %v", domainError)
	}
	return decision.NextState
}

func byteAt(data []byte, _index int) byte {
	if _index >= len(data) {
		return 0
	}
	return data[_index]
}
