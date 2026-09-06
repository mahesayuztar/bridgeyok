package deal

import (
	"context"
	"fmt"
	"github.com/google/uuid"

	"github.com/mahesayuztar/bridgeyok/apps/api/internal/bridge"
)

type Range struct {
	Min int
	Max int
}

type HandConstraint struct {
	HCP   *Range
	Suits map[bridge.Suit]Range
}

type Constraint struct {
	hands     map[bridge.Seat]HandConstraint
	attempts  int
	reference string
}

func NewConstraint(hands map[bridge.Seat]HandConstraint, attempts int, reference string) (*Constraint, error) {
	if attempts < 1 || attempts > 10000 || len(hands) == 0 {
		return nil, fmt.Errorf("constraints and 1–10000 attempts are required")
	}
	if reference == "" {
		return nil, fmt.Errorf("constraint reference is required")
	}
	minimumHCP, maximumHCP := 0, 0
	copied := make(map[bridge.Seat]HandConstraint, len(hands))
	for seat, hand := range hands {
		if !seat.Valid() {
			return nil, fmt.Errorf("invalid constraint seat")
		}
		clone := HandConstraint{Suits: make(map[bridge.Suit]Range, len(hand.Suits))}
		if hand.HCP != nil {
			value := *hand.HCP
			if value.Min < 0 || value.Max > 37 || value.Min > value.Max {
				return nil, fmt.Errorf("invalid HCP range")
			}
			clone.HCP = &value
		}
		minimumLength, maximumLength := 0, 0
		for _, suit := range []bridge.Suit{bridge.Clubs, bridge.Diamonds, bridge.Hearts, bridge.Spades} {
			if value, exists := hand.Suits[suit]; exists {
				if value.Min < 0 || value.Max > 13 || value.Min > value.Max {
					return nil, fmt.Errorf("invalid suit range")
				}
				minimumLength += value.Min
				maximumLength += value.Max
			} else {
				maximumLength += 13
			}
		}
		if minimumLength > 13 || maximumLength < 13 {
			return nil, fmt.Errorf("impossible hand distribution")
		}
		for suit, value := range hand.Suits {
			if !suit.Valid() {
				return nil, fmt.Errorf("invalid constraint suit")
			}
			clone.Suits[suit] = value
		}
		copied[seat] = clone
	}
	for _, seat := range []bridge.Seat{bridge.North, bridge.East, bridge.South, bridge.West} {
		if hand := copied[seat]; hand.HCP != nil {
			minimumHCP += hand.HCP.Min
			maximumHCP += hand.HCP.Max
		} else {
			maximumHCP += 37
		}
	}
	if minimumHCP > 40 || maximumHCP < 40 {
		return nil, fmt.Errorf("impossible total HCP")
	}
	for _, suit := range []bridge.Suit{bridge.Clubs, bridge.Diamonds, bridge.Hearts, bridge.Spades} {
		minimum, maximum := 0, 0
		for _, seat := range []bridge.Seat{bridge.North, bridge.East, bridge.South, bridge.West} {
			if value, exists := copied[seat].Suits[suit]; exists {
				minimum += value.Min
				maximum += value.Max
			} else {
				maximum += 13
			}
		}
		if minimum > 13 || maximum < 13 {
			return nil, fmt.Errorf("impossible suit total")
		}
	}
	source := &Constraint{hands: copied, attempts: attempts, reference: reference}
	if _, err := uuid.Parse(reference); err != nil {
		return nil, fmt.Errorf("constraint reference must be an opaque UUID")
	}
	return source, nil
}

func (source *Constraint) Generate(ctx context.Context) (Result, error) {
	for _attempt := 0; _attempt < source.attempts; _attempt++ {
		result, err := (SecureRandom{}).Generate(ctx)
		if err != nil {
			return Result{}, err
		}
		matches := true
		for seat, constraint := range source.hands {
			hcp := 0
			lengths := make(map[bridge.Suit]int, 4)
			for _, card := range result.Deal.Hand(seat) {
				lengths[card.Suit]++
				switch card.Rank {
				case bridge.Ace:
					hcp += 4
				case bridge.King:
					hcp += 3
				case bridge.Queen:
					hcp += 2
				case bridge.Jack:
					hcp++
				}
			}
			if constraint.HCP != nil && (hcp < constraint.HCP.Min || hcp > constraint.HCP.Max) {
				matches = false
			}
			for suit, length := range constraint.Suits {
				if lengths[suit] < length.Min || lengths[suit] > length.Max {
					matches = false
				}
			}
		}
		if matches {
			result.Provenance = Provenance{Type: "constraint", Version: "rejection-hcp-suits-v1", Reference: source.reference}
			if err := result.Validate(); err != nil {
				return Result{}, err
			}
			return result, nil
		}
	}
	return Result{}, ErrUnavailable
}
