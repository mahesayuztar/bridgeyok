package bridge

import "fmt"

// Vulnerability identifies which partnerships receive vulnerable scoring.
type Vulnerability string

const (
	VulnerabilityNone Vulnerability = "NONE"
	VulnerabilityNS   Vulnerability = "NS"
	VulnerabilityEW   Vulnerability = "EW"
	VulnerabilityBoth Vulnerability = "BOTH"
)

// BoardMetadata is the dealer and vulnerability derived from a board number.
type BoardMetadata struct {
	Number        int           `json:"number"`
	Dealer        Seat          `json:"dealer"`
	Vulnerability Vulnerability `json:"vulnerability"`
}

var vulnerabilityCycle = [...]Vulnerability{
	VulnerabilityNone,
	VulnerabilityNS,
	VulnerabilityEW,
	VulnerabilityBoth,
	VulnerabilityNS,
	VulnerabilityEW,
	VulnerabilityBoth,
	VulnerabilityNone,
	VulnerabilityEW,
	VulnerabilityBoth,
	VulnerabilityNone,
	VulnerabilityNS,
	VulnerabilityBoth,
	VulnerabilityNone,
	VulnerabilityNS,
	VulnerabilityEW,
}

// MetadataForBoard returns the standard repeating 16-board dealer and vulnerability cycle.
func MetadataForBoard(boardNumber int) (BoardMetadata, error) {
	if boardNumber < 1 {
		return BoardMetadata{}, fmt.Errorf("board number must be positive")
	}
	cycleIndex := (boardNumber - 1) % len(vulnerabilityCycle)
	dealer := [...]Seat{North, East, South, West}[cycleIndex%4]
	return BoardMetadata{
		Number:        boardNumber,
		Dealer:        dealer,
		Vulnerability: vulnerabilityCycle[cycleIndex],
	}, nil
}

// Valid reports whether the vulnerability value is part of the standard cycle.
func (vulnerability Vulnerability) Valid() bool {
	switch vulnerability {
	case VulnerabilityNone, VulnerabilityNS, VulnerabilityEW, VulnerabilityBoth:
		return true
	default:
		return false
	}
}

// IsVulnerable reports whether the partnership is vulnerable on the board.
func (vulnerability Vulnerability) IsVulnerable(partnership Partnership) bool {
	switch vulnerability {
	case VulnerabilityNS:
		return partnership == NorthSouth
	case VulnerabilityEW:
		return partnership == EastWest
	case VulnerabilityBoth:
		return partnership.Valid()
	default:
		return false
	}
}
