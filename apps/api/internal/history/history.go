package history

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/mahesayuztar/bridgeyok/apps/api/internal/bridge"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/table"
)

const (
	DefaultPageSize = 16
	MaxPageSize     = 40
)

var ErrInvalidCursor = errors.New("invalid history cursor")
var ErrInvalidPageSize = errors.New("invalid history page size")

type Cursor struct {
	CompletedAt time.Time
	BoardID     string
}

type Participant struct {
	ID       string `json:"id"`
	Nickname string `json:"nickname"`
	IsBot    bool   `json:"isBot"`
}

type BoardLineup struct {
	Seats      map[bridge.Seat]Participant `json:"seats"`
	NorthSouth table.ScorePair             `json:"northSouth"`
	EastWest   table.ScorePair             `json:"eastWest"`
}

type Board struct {
	BoardID       string        `json:"boardId"`
	TableID       string        `json:"tableId"`
	BoardNumber   int           `json:"boardNumber"`
	Result        bridge.Result `json:"result"`
	Lineup        BoardLineup   `json:"lineup"`
	ViewerSeat    bridge.Seat   `json:"viewerSeat"`
	CompletedAt   time.Time     `json:"completedAt"`
	MatchID       string        `json:"matchId,omitempty"`
	MatchStatus   string        `json:"matchStatus,omitempty"`
	MatchRoom     string        `json:"matchRoom,omitempty"`
	Team          string        `json:"team,omitempty"`
	TeamAIMP      *int          `json:"teamAIMP,omitempty"`
	ReplayAllowed bool          `json:"replayAllowed"`
}

type Page struct {
	Items      []Board `json:"items"`
	NextCursor string  `json:"nextCursor,omitempty"`
}

func EncodeCursor(cursor Cursor) string {
	payload, _ := json.Marshal(cursor)
	return base64.RawURLEncoding.EncodeToString(payload)
}

func DecodeCursor(value string) (Cursor, error) {
	if value == "" {
		return Cursor{}, nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return Cursor{}, ErrInvalidCursor
	}
	var cursor Cursor
	if err := json.Unmarshal(decoded, &cursor); err != nil || cursor.BoardID == "" || cursor.CompletedAt.IsZero() {
		return Cursor{}, ErrInvalidCursor
	}
	return cursor, nil
}

func ValidatePageSize(value int) error {
	if value < 1 || value > MaxPageSize {
		return fmt.Errorf("%w: must be between 1 and %d", ErrInvalidPageSize, MaxPageSize)
	}
	return nil
}
