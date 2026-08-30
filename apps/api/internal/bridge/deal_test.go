package bridge

import (
	"bytes"
	"errors"
	"io"
	"reflect"
	"testing"
)

type constantReader byte

func (reader constantReader) Read(target []byte) (int, error) {
	for _index := range target {
		target[_index] = byte(reader)
	}
	return len(target), nil
}

func TestGenerateDeal(t *testing.T) {
	t.Parallel()

	first, err := GenerateDeal(constantReader(0x80))
	if err != nil {
		t.Fatalf("GenerateDeal() error = %v", err)
	}
	second, err := GenerateDeal(constantReader(0x80))
	if err != nil {
		t.Fatalf("GenerateDeal() second error = %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("same random byte stream produced different deals")
	}
	if err := first.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestGenerateDealRejectsInvalidSource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		source io.Reader
	}{
		{name: "nil", source: nil},
		{name: "short", source: &failingReader{}},
		{name: "permanently rejected", source: constantReader(0)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, err := GenerateDeal(test.source); err == nil {
				t.Fatal("GenerateDeal() error = nil")
			}
		})
	}
}

func TestReadRandomIndexRetriesRejectedSample(t *testing.T) {
	t.Parallel()

	source := bytes.NewReader(append(make([]byte, 8), bytes.Repeat([]byte{0xff}, 8)...))
	value, err := readRandomIndex(source, 3)
	if err != nil {
		t.Fatalf("readRandomIndex() error = %v", err)
	}
	if value != 0 {
		t.Fatalf("readRandomIndex() = %d, want 0", value)
	}
}

type failingReader struct{}

func (*failingReader) Read([]byte) (int, error) {
	return 0, errors.New("random source failed")
}

func TestDealHandReturnsDefensiveCopy(t *testing.T) {
	t.Parallel()

	deal, err := GenerateDeal(constantReader(0x80))
	if err != nil {
		t.Fatalf("GenerateDeal() error = %v", err)
	}
	hand := deal.Hand(North)
	original := deal.North[0]
	hand[0] = deal.East[0]
	if deal.North[0] != original {
		t.Fatal("Hand() allowed caller to mutate the deal")
	}
}

func TestDealValidateRejectsBrokenDeals(t *testing.T) {
	t.Parallel()

	validDeal, err := GenerateDeal(constantReader(0x80))
	if err != nil {
		t.Fatalf("GenerateDeal() error = %v", err)
	}

	tests := []struct {
		name   string
		mutate func(Deal) Deal
	}{
		{
			name: "wrong hand size",
			mutate: func(deal Deal) Deal {
				deal.North = deal.North[:12]
				return deal
			},
		},
		{
			name: "duplicate card",
			mutate: func(deal Deal) Deal {
				deal.East[0] = deal.North[0]
				return deal
			},
		},
		{
			name: "invalid card",
			mutate: func(deal Deal) Deal {
				deal.West[0] = Card{Suit: "X", Rank: Ace}
				return deal
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if err := test.mutate(validDeal.clone()).Validate(); err == nil {
				t.Fatal("Validate() error = nil")
			}
		})
	}
}
