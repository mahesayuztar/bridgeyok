//go:build testfixture || integration

package deal

import (
	"reflect"
	"testing"
)

func TestDeterministicSourceReplaysWithoutPersistingSeed(t *testing.T) {
	t.Parallel()
	source := Deterministic{Seed: [32]byte{42}}
	first, err := source.Generate(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	second, err := source.Generate(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("deterministic replay changed")
	}
	if err := first.Validate(); err != nil {
		t.Fatal(err)
	}
	if first.Provenance.Type != "deterministic" || first.Provenance.Reference != "" {
		t.Fatal("unexpected deterministic provenance")
	}
	other, err := (Deterministic{Seed: [32]byte{43}}).Generate(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if reflect.DeepEqual(first.Deal, other.Deal) {
		t.Fatal("distinct seeds produced identical deals")
	}
}
