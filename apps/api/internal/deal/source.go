package deal

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/bridge"
)

var ErrUnavailable = errors.New("deal source unavailable")

type Provenance struct {
	Type      string `json:"type"`
	Version   string `json:"version"`
	Reference string `json:"reference,omitempty"`
}

type Result struct {
	Deal       bridge.Deal `json:"deal"`
	Provenance Provenance  `json:"provenance"`
}

type Source interface {
	Generate(context.Context) (Result, error)
}

type SecureRandom struct{}

func (SecureRandom) Generate(ctx context.Context) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	cards, err := bridge.GenerateDeal(rand.Reader)
	if err != nil {
		return Result{}, ErrUnavailable
	}
	return Result{Deal: cards, Provenance: Provenance{Type: "secure_random", Version: "fisher-yates-v1"}}, nil
}

func (result Result) Validate() error {
	if err := result.Deal.Validate(); err != nil {
		return fmt.Errorf("invalid source deal")
	}
	switch result.Provenance.Type {
	case "secure_random", "deterministic", "prepared", "constraint":
	default:
		return fmt.Errorf("invalid source type")
	}
	if result.Provenance.Version == "" || len(result.Provenance.Version) > 64 {
		return fmt.Errorf("invalid source version")
	}
	if result.Provenance.Reference != "" {
		if _, err := uuid.Parse(result.Provenance.Reference); err != nil {
			return fmt.Errorf("source reference must be an opaque UUID")
		}
	}
	return nil
}

type Prepared struct{ result Result }

func NewPrepared(cards bridge.Deal, reference string) (*Prepared, error) {
	result := Result{Deal: cards, Provenance: Provenance{Type: "prepared", Version: "v1", Reference: reference}}
	if reference == "" {
		return nil, fmt.Errorf("prepared source reference is required")
	}
	if err := result.Validate(); err != nil {
		return nil, err
	}
	result.Deal = bridge.Deal{North: cards.Hand(bridge.North), East: cards.Hand(bridge.East), South: cards.Hand(bridge.South), West: cards.Hand(bridge.West)}
	return &Prepared{result: result}, nil
}

func (source *Prepared) Generate(ctx context.Context) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	cards := source.result.Deal
	return Result{Deal: bridge.Deal{North: cards.Hand(bridge.North), East: cards.Hand(bridge.East), South: cards.Hand(bridge.South), West: cards.Hand(bridge.West)}, Provenance: source.result.Provenance}, nil
}
