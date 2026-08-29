package bridge

import "fmt"

// Suit identifies one of the four card suits.
type Suit string

const (
	Clubs    Suit = "C"
	Diamonds Suit = "D"
	Hearts   Suit = "H"
	Spades   Suit = "S"
)

// Rank identifies a card rank from two through ace.
type Rank string

const (
	Two   Rank = "2"
	Three Rank = "3"
	Four  Rank = "4"
	Five  Rank = "5"
	Six   Rank = "6"
	Seven Rank = "7"
	Eight Rank = "8"
	Nine  Rank = "9"
	Ten   Rank = "T"
	Jack  Rank = "J"
	Queen Rank = "Q"
	King  Rank = "K"
	Ace   Rank = "A"
)

var suits = [...]Suit{Clubs, Diamonds, Hearts, Spades}
var ranks = [...]Rank{Two, Three, Four, Five, Six, Seven, Eight, Nine, Ten, Jack, Queen, King, Ace}

// Card is a single immutable playing card.
type Card struct {
	Suit Suit `json:"suit"`
	Rank Rank `json:"rank"`
}

// Valid reports whether the suit is a bridge suit.
func (suit Suit) Valid() bool {
	switch suit {
	case Clubs, Diamonds, Hearts, Spades:
		return true
	default:
		return false
	}
}

// Valid reports whether the rank is between two and ace.
func (rank Rank) Valid() bool {
	return rank.value() >= 0
}

func (rank Rank) value() int {
	for _index, candidate := range ranks {
		if candidate == rank {
			return _index
		}
	}
	return -1
}

// Validate returns an error when the card is not in the standard 52-card pack.
func (card Card) Validate() error {
	if !card.Suit.Valid() {
		return fmt.Errorf("invalid card suit %q", card.Suit)
	}
	if !card.Rank.Valid() {
		return fmt.Errorf("invalid card rank %q", card.Rank)
	}
	return nil
}

// String returns the canonical two-character suit-rank representation.
func (card Card) String() string {
	return string(card.Suit) + string(card.Rank)
}

// ParseCard parses the canonical two-character suit-rank representation.
func ParseCard(value string) (Card, error) {
	if len(value) != 2 {
		return Card{}, fmt.Errorf("card must contain exactly two ASCII characters")
	}
	card := Card{Suit: Suit(value[:1]), Rank: Rank(value[1:])}
	if err := card.Validate(); err != nil {
		return Card{}, err
	}
	return card, nil
}

// FullDeck returns the standard 52-card pack in stable suit and rank order.
func FullDeck() []Card {
	deck := make([]Card, 0, len(suits)*len(ranks))
	for _, suit := range suits {
		for _, rank := range ranks {
			deck = append(deck, Card{Suit: suit, Rank: rank})
		}
	}
	return deck
}
