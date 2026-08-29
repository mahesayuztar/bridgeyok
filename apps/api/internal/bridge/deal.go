package bridge

import (
	"encoding/binary"
	"fmt"
	"io"
	"sort"
)

// Hand is the remaining collection of cards held by one seat.
type Hand []Card

// Deal contains one hand for each fixed table seat.
type Deal struct {
	North Hand `json:"north"`
	East  Hand `json:"east"`
	South Hand `json:"south"`
	West  Hand `json:"west"`
}

// GenerateDeal shuffles and deals a pack using the injected random source.
// Production callers must provide a cryptographically secure source.
func GenerateDeal(randomSource io.Reader) (Deal, error) {
	if randomSource == nil {
		return Deal{}, fmt.Errorf("random source is required")
	}

	deck := FullDeck()
	for _index := len(deck) - 1; _index > 0; _index-- {
		randomIndex, err := readRandomIndex(randomSource, uint64(_index+1))
		if err != nil {
			return Deal{}, fmt.Errorf("shuffle card %d: %w", _index, err)
		}
		deck[_index], deck[randomIndex] = deck[randomIndex], deck[_index]
	}

	deal := Deal{}
	for _index, card := range deck {
		seat := [...]Seat{North, East, South, West}[_index%4]
		hand := append(deal.hand(seat), card)
		deal.setHand(seat, hand)
	}
	deal.sortHands()

	if err := deal.Validate(); err != nil {
		return Deal{}, fmt.Errorf("validate generated deal: %w", err)
	}
	return deal, nil
}

func readRandomIndex(randomSource io.Reader, upperBound uint64) (int, error) {
	if upperBound == 0 {
		return 0, fmt.Errorf("upper bound must be positive")
	}

	threshold := -upperBound % upperBound
	var buffer [8]byte
	for {
		if _, err := io.ReadFull(randomSource, buffer[:]); err != nil {
			return 0, fmt.Errorf("read random bytes: %w", err)
		}
		value := binary.LittleEndian.Uint64(buffer[:])
		if value >= threshold {
			return int(value % upperBound), nil
		}
	}
}

// Hand returns a defensive copy of the hand for a seat.
func (deal Deal) Hand(seat Seat) Hand {
	hand := deal.hand(seat)
	if len(hand) == 0 {
		return Hand{}
	}
	return append(Hand(nil), hand...)
}

func (deal Deal) hand(seat Seat) Hand {
	switch seat {
	case North:
		return deal.North
	case East:
		return deal.East
	case South:
		return deal.South
	case West:
		return deal.West
	default:
		return nil
	}
}

func (deal *Deal) setHand(seat Seat, hand Hand) {
	switch seat {
	case North:
		deal.North = hand
	case East:
		deal.East = hand
	case South:
		deal.South = hand
	case West:
		deal.West = hand
	}
}

func (deal *Deal) removeCard(seat Seat, card Card) bool {
	hand := deal.hand(seat)
	for _index, candidate := range hand {
		if candidate == card {
			hand = append(hand[:_index], hand[_index+1:]...)
			if len(hand) == 0 {
				hand = Hand{}
			}
			deal.setHand(seat, hand)
			return true
		}
	}
	return false
}

func (hand Hand) contains(card Card) bool {
	for _, candidate := range hand {
		if candidate == card {
			return true
		}
	}
	return false
}

func (hand Hand) hasSuit(suit Suit) bool {
	for _, card := range hand {
		if card.Suit == suit {
			return true
		}
	}
	return false
}

func (deal Deal) clone() Deal {
	return Deal{
		North: deal.Hand(North),
		East:  deal.Hand(East),
		South: deal.Hand(South),
		West:  deal.Hand(West),
	}
}

func (deal *Deal) sortHands() {
	for _, seat := range [...]Seat{North, East, South, West} {
		hand := deal.hand(seat)
		sort.Slice(hand, func(_leftIndex, _rightIndex int) bool {
			left := hand[_leftIndex]
			right := hand[_rightIndex]
			if left.Suit != right.Suit {
				return suitValue(left.Suit) < suitValue(right.Suit)
			}
			return left.Rank.value() < right.Rank.value()
		})
		deal.setHand(seat, hand)
	}
}

func suitValue(suit Suit) int {
	for _index, candidate := range suits {
		if candidate == suit {
			return _index
		}
	}
	return -1
}

// Validate enforces a standard pack with exactly thirteen unique cards per seat.
func (deal Deal) Validate() error {
	seen := make(map[Card]Seat, 52)
	for _, seat := range [...]Seat{North, East, South, West} {
		hand := deal.hand(seat)
		if len(hand) != 13 {
			return fmt.Errorf("seat %s has %d cards, want 13", seat, len(hand))
		}
		for _, card := range hand {
			if err := card.Validate(); err != nil {
				return fmt.Errorf("seat %s: %w", seat, err)
			}
			if previousSeat, exists := seen[card]; exists {
				return fmt.Errorf("card %s appears in seats %s and %s", card, previousSeat, seat)
			}
			seen[card] = seat
		}
	}
	if len(seen) != 52 {
		return fmt.Errorf("deal has %d unique cards, want 52", len(seen))
	}
	return nil
}
