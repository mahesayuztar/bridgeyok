package bridge

import (
	"encoding/binary"
	"reflect"
	"testing"
)

type deterministicSource struct {
	state uint64
}

func (source *deterministicSource) next() uint64 {
	value := source.state
	value ^= value << 13
	value ^= value >> 7
	value ^= value << 17
	source.state = value
	return value
}

func (source *deterministicSource) Read(target []byte) (int, error) {
	for _offset := 0; _offset < len(target); {
		var buffer [8]byte
		binary.LittleEndian.PutUint64(buffer[:], source.next())
		_offset += copy(target[_offset:], buffer[:])
	}
	return len(target), nil
}

func (source *deterministicSource) intn(upperBound int) int {
	return int(source.next() % uint64(upperBound))
}

func TestRandomizedLegalGames(t *testing.T) {
	const gameCount = 10_000
	source := &deterministicSource{state: 0x627269646765796f}
	passedOutCount := 0

	for _gameIndex := 0; _gameIndex < gameCount; _gameIndex++ {
		deal, err := GenerateDeal(source)
		if err != nil {
			t.Fatalf("game %d GenerateDeal() error = %v", _gameIndex, err)
		}
		state, err := NewBoard(_gameIndex+1, deal)
		if err != nil {
			t.Fatalf("game %d NewBoard() error = %v", _gameIndex, err)
		}
		initial := state.clone()
		captureReplay := _gameIndex%1000 <= 1
		events := []Event{}
		forcePassedOut := _gameIndex%100 == 0

		for _callIndex := 0; state.Phase == PhaseAuction; _callIndex++ {
			if _callIndex >= 64 {
				t.Fatalf("game %d auction did not terminate", _gameIndex)
			}
			legalCalls := state.Auction.LegalCalls()
			selected := Pass()
			if _callIndex == 0 && !forcePassedOut {
				selected = legalCalls[1+source.intn(len(legalCalls)-1)]
			} else if !forcePassedOut && _callIndex < 16 && len(legalCalls) > 1 && source.intn(3) != 0 {
				selected = legalCalls[source.intn(len(legalCalls))]
			}
			decision, domainError := Decide(state, MakeCallCommand(state.Turn, selected))
			if domainError != nil {
				t.Fatalf("game %d call %d error = %v", _gameIndex, _callIndex, domainError)
			}
			if captureReplay {
				events = append(events, decision.Events...)
			}
			state = decision.NextState
		}
		if state.Auction.PassedOut {
			passedOutCount++
			if state.Result == nil || !state.Result.PassedOut || state.Result.ScoreNS != 0 {
				t.Fatalf("game %d passed-out state is inconsistent", _gameIndex)
			}
			if captureReplay {
				assertRandomizedReplay(t, _gameIndex, initial, events, state)
			}
			continue
		}

		for _playIndex := 0; state.Phase != PhaseBoardScored; _playIndex++ {
			if _playIndex >= 52 {
				t.Fatalf("game %d play did not terminate", _gameIndex)
			}
			actor := state.Turn
			if state.Turn == state.Auction.Contract.Dummy() {
				actor = state.Auction.Contract.Declarer
			}
			legalCards, domainError := state.LegalCards(actor)
			if domainError != nil || len(legalCards) == 0 {
				t.Fatalf("game %d play %d legal cards = %d, error = %v", _gameIndex, _playIndex, len(legalCards), domainError)
			}
			card := legalCards[source.intn(len(legalCards))]
			decision, domainError := Decide(state, PlayCardCommand(actor, card))
			if domainError != nil {
				t.Fatalf("game %d play %d error = %v", _gameIndex, _playIndex, domainError)
			}
			if captureReplay {
				events = append(events, decision.Events...)
			}
			state = decision.NextState
		}

		if state.Result == nil || state.Result.PassedOut || len(state.CompletedTricks) != 13 || state.TricksNS+state.TricksEW != 13 {
			t.Fatalf("game %d terminal state is inconsistent", _gameIndex)
		}
		if captureReplay {
			assertRandomizedReplay(t, _gameIndex, initial, events, state)
		}
	}
	if passedOutCount != gameCount/100 {
		t.Fatalf("passed-out games = %d, want %d", passedOutCount, gameCount/100)
	}
}

func assertRandomizedReplay(t *testing.T, gameIndex int, initial State, events []Event, want State) {
	t.Helper()
	replayed, err := Reduce(initial, events)
	if err != nil {
		t.Fatalf("game %d Reduce() error = %v", gameIndex, err)
	}
	if !reflect.DeepEqual(replayed, want) {
		t.Fatalf("game %d replay state differs from decided state", gameIndex)
	}
}
