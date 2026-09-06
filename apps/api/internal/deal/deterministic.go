//go:build testfixture || integration

package deal

import (
	"context"
	"crypto/sha256"
	"math/rand/v2"

	"github.com/mahesayuztar/bridgeyok/apps/api/internal/bridge"
)

type Deterministic struct{ Seed [32]byte }

func (source Deterministic) Generate(ctx context.Context) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	seed := sha256.Sum256(source.Seed[:])
	cards, err := bridge.GenerateDeal(rand.NewChaCha8(seed))
	if err != nil {
		return Result{}, ErrUnavailable
	}
	return Result{Deal: cards, Provenance: Provenance{Type: "deterministic", Version: "chacha8-sha256-v1"}}, nil
}
