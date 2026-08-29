package bridge

import "fmt"

// PlayedCard associates a card with the hand that contributed it.
type PlayedCard struct {
	Seat Seat `json:"seat"`
	Card Card `json:"card"`
}

// Trick contains cards in play order and the winner after completion.
type Trick struct {
	Leader Seat         `json:"leader"`
	Plays  []PlayedCard `json:"plays"`
	Winner Seat         `json:"winner,omitempty"`
}

func (trick Trick) clone() Trick {
	clone := trick
	clone.Plays = append([]PlayedCard{}, trick.Plays...)
	return clone
}

func (trick Trick) validateOrder() error {
	if err := trick.Leader.Validate(); err != nil {
		return fmt.Errorf("leader: %w", err)
	}
	if len(trick.Plays) > 4 {
		return fmt.Errorf("trick has %d cards, maximum is 4", len(trick.Plays))
	}
	expectedSeat := trick.Leader
	for _index, play := range trick.Plays {
		if play.Seat != expectedSeat {
			return fmt.Errorf("play %d belongs to %s, want %s", _index, play.Seat, expectedSeat)
		}
		if err := play.Card.Validate(); err != nil {
			return fmt.Errorf("play %d: %w", _index, err)
		}
		expectedSeat = expectedSeat.Next()
	}
	return nil
}

func trickWinner(trick Trick, strain Strain) (Seat, error) {
	if !strain.Valid() {
		return "", fmt.Errorf("invalid strain %q", strain)
	}
	if err := trick.validateOrder(); err != nil {
		return "", err
	}
	if len(trick.Plays) != 4 {
		return "", fmt.Errorf("trick has %d cards, want 4", len(trick.Plays))
	}

	winningPlay := trick.Plays[0]
	trumpSuit, hasTrump := strainSuit(strain)
	for _, candidate := range trick.Plays[1:] {
		candidateTrump := hasTrump && candidate.Card.Suit == trumpSuit
		winningTrump := hasTrump && winningPlay.Card.Suit == trumpSuit
		switch {
		case candidateTrump && !winningTrump:
			winningPlay = candidate
		case candidateTrump == winningTrump && candidate.Card.Suit == winningPlay.Card.Suit && candidate.Card.Rank.value() > winningPlay.Card.Rank.value():
			winningPlay = candidate
		}
	}
	return winningPlay.Seat, nil
}

func strainSuit(strain Strain) (Suit, bool) {
	switch strain {
	case StrainClubs:
		return Clubs, true
	case StrainDiamonds:
		return Diamonds, true
	case StrainHearts:
		return Hearts, true
	case StrainSpades:
		return Spades, true
	default:
		return "", false
	}
}
