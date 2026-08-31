package bridge

import (
	"fmt"
	"reflect"
)

// ValidateInvariants proves card conservation, turn order, auction, trick, and score consistency.
func (state State) ValidateInvariants() error {
	if state.RulesetVersion != RulesetVersion {
		return fmt.Errorf("unsupported ruleset version %q", state.RulesetVersion)
	}
	expectedMetadata, err := MetadataForBoard(state.Board.Number)
	if err != nil || state.Board != expectedMetadata {
		return fmt.Errorf("board metadata does not match standard cycle")
	}
	if err := state.Auction.Validate(); err != nil {
		return fmt.Errorf("auction: %w", err)
	}
	if state.Auction.Dealer != state.Board.Dealer {
		return fmt.Errorf("auction dealer does not match board")
	}
	if state.CompletedTricks == nil {
		return fmt.Errorf("completed tricks must be initialized")
	}
	if err := state.validateCardsAndTricks(); err != nil {
		return err
	}

	switch state.Phase {
	case PhaseAuction:
		if state.Auction.Complete || state.Turn != state.Auction.Turn || state.DummyRevealed || state.CurrentTrick.Leader != "" || len(state.CurrentTrick.Plays) != 0 || len(state.CompletedTricks) != 0 || state.Result != nil || state.Claimed {
			return fmt.Errorf("auction phase contains play or terminal state")
		}
		if err := state.Deal.Validate(); err != nil {
			return fmt.Errorf("auction deal: %w", err)
		}
	case PhaseOpeningLead:
		if !state.hasActiveContract() || state.DummyRevealed || len(state.CurrentTrick.Plays) > 1 || len(state.CompletedTricks) != 0 || state.Result != nil || state.Claimed {
			return fmt.Errorf("opening-lead phase is inconsistent")
		}
	case PhasePlay:
		if !state.hasActiveContract() || !state.DummyRevealed || state.Result != nil || state.Claimed || len(state.CompletedTricks) > 13 {
			return fmt.Errorf("play phase is inconsistent")
		}
	case PhaseBoardScored:
		if state.Turn != "" || state.Result == nil {
			return fmt.Errorf("scored phase is missing terminal result")
		}
		if err := state.Result.Validate(); err != nil {
			return fmt.Errorf("result: %w", err)
		}
		if state.Result.Vulnerability != state.Board.Vulnerability {
			return fmt.Errorf("result vulnerability does not match board")
		}
		if state.Auction.PassedOut {
			if !state.Result.PassedOut || len(state.CompletedTricks) != 0 || state.DummyRevealed || state.Claimed {
				return fmt.Errorf("passed-out board contains play state")
			}
			if err := state.Deal.Validate(); err != nil {
				return fmt.Errorf("passed-out deal: %w", err)
			}
		} else if state.Claimed {
			if !state.DummyRevealed || len(state.CurrentTrick.Plays) != 0 || len(state.CompletedTricks) >= 13 || state.Result.TricksNS+state.Result.TricksEW != 13 || state.Result.TricksNS < state.TricksNS || state.Result.TricksEW < state.TricksEW || !reflect.DeepEqual(state.Result.Contract, state.Auction.Contract) {
				return fmt.Errorf("claimed result does not match board state")
			}
		} else if !state.DummyRevealed || len(state.CompletedTricks) != 13 || state.Result.TricksNS != state.TricksNS || state.Result.TricksEW != state.TricksEW || !reflect.DeepEqual(state.Result.Contract, state.Auction.Contract) {
			return fmt.Errorf("played result does not match board state")
		}
	default:
		return fmt.Errorf("invalid phase %q", state.Phase)
	}
	return nil
}

func (state State) hasActiveContract() bool {
	if !state.Auction.Complete || state.Auction.PassedOut || state.Auction.Contract == nil {
		return false
	}
	if state.CurrentTrick.Leader == "" {
		return false
	}
	if len(state.CompletedTricks) == 0 && state.CurrentTrick.Leader != state.Auction.Contract.OpeningLeader() {
		return false
	}
	expectedTurn := state.CurrentTrick.Leader
	for range state.CurrentTrick.Plays {
		expectedTurn = expectedTurn.Next()
	}
	return state.Turn == expectedTurn
}

func (state State) validateCardsAndTricks() error {
	if state.CurrentTrick.Winner != "" {
		return fmt.Errorf("current trick cannot have a winner")
	}
	seen := make(map[Card]struct{}, 52)
	cardCount := 0
	for _, seat := range [...]Seat{North, East, South, West} {
		for _, card := range state.Deal.hand(seat) {
			if err := addCardLocation(seen, card); err != nil {
				return err
			}
			cardCount++
		}
	}

	tricksNS := 0
	tricksEW := 0
	for _index, trick := range state.CompletedTricks {
		if len(trick.Plays) != 4 {
			return fmt.Errorf("completed trick %d has %d cards", _index, len(trick.Plays))
		}
		winner, err := trickWinner(trick, state.contractStrain())
		if err != nil || trick.Winner != winner {
			return fmt.Errorf("completed trick %d has invalid winner", _index)
		}
		if _index > 0 && trick.Leader != state.CompletedTricks[_index-1].Winner {
			return fmt.Errorf("completed trick %d leader does not match prior winner", _index)
		}
		for _, play := range trick.Plays {
			if err := addCardLocation(seen, play.Card); err != nil {
				return err
			}
			cardCount++
		}
		if winner.Partnership() == NorthSouth {
			tricksNS++
		} else {
			tricksEW++
		}
	}
	if tricksNS != state.TricksNS || tricksEW != state.TricksEW || tricksNS+tricksEW != len(state.CompletedTricks) {
		return fmt.Errorf("trick totals do not match completed tricks")
	}

	if err := state.CurrentTrick.validateOrder(); err != nil && state.Phase != PhaseAuction && !state.Auction.PassedOut {
		return fmt.Errorf("current trick: %w", err)
	}
	if len(state.CompletedTricks) > 0 && state.CurrentTrick.Leader != state.CompletedTricks[len(state.CompletedTricks)-1].Winner {
		return fmt.Errorf("current leader does not match prior trick winner")
	}
	for _, play := range state.CurrentTrick.Plays {
		if err := addCardLocation(seen, play.Card); err != nil {
			return err
		}
		cardCount++
	}
	if cardCount != 52 || len(seen) != 52 {
		return fmt.Errorf("card locations contain %d cards and %d unique cards, want 52", cardCount, len(seen))
	}
	if state.Auction.Contract != nil && !state.Auction.PassedOut {
		firstLeader := state.CurrentTrick.Leader
		if len(state.CompletedTricks) > 0 {
			firstLeader = state.CompletedTricks[0].Leader
		}
		if firstLeader != state.Auction.Contract.OpeningLeader() {
			return fmt.Errorf("first trick leader %s does not match opening leader %s", firstLeader, state.Auction.Contract.OpeningLeader())
		}
	}
	if err := state.validatePlayHistory(); err != nil {
		return err
	}
	return nil
}

func (state State) validatePlayHistory() error {
	reconstructed := state.Deal.clone()
	for _, trick := range state.CompletedTricks {
		for _, play := range trick.Plays {
			reconstructed.setHand(play.Seat, append(reconstructed.hand(play.Seat), play.Card))
		}
	}
	for _, play := range state.CurrentTrick.Plays {
		reconstructed.setHand(play.Seat, append(reconstructed.hand(play.Seat), play.Card))
	}
	if err := reconstructed.Validate(); err != nil {
		return fmt.Errorf("reconstructed deal: %w", err)
	}

	for _trickIndex, trick := range state.CompletedTricks {
		if err := consumeLegalPlays(&reconstructed, trick); err != nil {
			return fmt.Errorf("completed trick %d: %w", _trickIndex, err)
		}
	}
	if err := consumeLegalPlays(&reconstructed, state.CurrentTrick); err != nil {
		return fmt.Errorf("current trick: %w", err)
	}
	return nil
}

func consumeLegalPlays(hands *Deal, trick Trick) error {
	if len(trick.Plays) == 0 {
		return nil
	}
	ledSuit := trick.Plays[0].Card.Suit
	for _playIndex, play := range trick.Plays {
		hand := hands.hand(play.Seat)
		if _playIndex > 0 && play.Card.Suit != ledSuit && hand.hasSuit(ledSuit) {
			return fmt.Errorf("seat %s did not follow suit %s", play.Seat, ledSuit)
		}
		if !hands.removeCard(play.Seat, play.Card) {
			return fmt.Errorf("seat %s did not hold card %s", play.Seat, play.Card)
		}
	}
	return nil
}

func (state State) contractStrain() Strain {
	if state.Auction.Contract == nil {
		return ""
	}
	return state.Auction.Contract.Strain
}

func addCardLocation(seen map[Card]struct{}, card Card) error {
	if err := card.Validate(); err != nil {
		return err
	}
	if _, exists := seen[card]; exists {
		return fmt.Errorf("card %s appears in multiple locations", card)
	}
	seen[card] = struct{}{}
	return nil
}
