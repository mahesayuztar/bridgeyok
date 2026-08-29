// Package bridge implements the deterministic Contract Bridge rules engine.
package bridge

import "fmt"

// Seat identifies one of the four clockwise positions at a bridge table.
type Seat string

const (
	North Seat = "N"
	East  Seat = "E"
	South Seat = "S"
	West  Seat = "W"
)

// Partnership identifies a pair of opposite seats.
type Partnership string

const (
	NorthSouth Partnership = "NS"
	EastWest   Partnership = "EW"
)

// Valid reports whether the seat is one of N, E, S, or W.
func (seat Seat) Valid() bool {
	switch seat {
	case North, East, South, West:
		return true
	default:
		return false
	}
}

// Next returns the next seat clockwise.
func (seat Seat) Next() Seat {
	switch seat {
	case North:
		return East
	case East:
		return South
	case South:
		return West
	case West:
		return North
	default:
		return ""
	}
}

// Partner returns the seat opposite this seat.
func (seat Seat) Partner() Seat {
	switch seat {
	case North:
		return South
	case East:
		return West
	case South:
		return North
	case West:
		return East
	default:
		return ""
	}
}

// Partnership returns the partnership containing the seat.
func (seat Seat) Partnership() Partnership {
	switch seat {
	case North, South:
		return NorthSouth
	case East, West:
		return EastWest
	default:
		return ""
	}
}

// Validate returns an error when the seat is outside the fixed table positions.
func (seat Seat) Validate() error {
	if !seat.Valid() {
		return fmt.Errorf("invalid seat %q", seat)
	}
	return nil
}

// Opponent reports whether two seats belong to opposing partnerships.
func (partnership Partnership) Opponent(other Partnership) bool {
	return (partnership == NorthSouth && other == EastWest) ||
		(partnership == EastWest && other == NorthSouth)
}

// Valid reports whether the partnership is NS or EW.
func (partnership Partnership) Valid() bool {
	return partnership == NorthSouth || partnership == EastWest
}
