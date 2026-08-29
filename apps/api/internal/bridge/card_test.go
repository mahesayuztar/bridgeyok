package bridge

import "testing"

func TestParseCard(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   string
		want    Card
		wantErr bool
	}{
		{name: "ace of spades", value: "SA", want: Card{Suit: Spades, Rank: Ace}},
		{name: "ten of clubs", value: "CT", want: Card{Suit: Clubs, Rank: Ten}},
		{name: "lowercase rejected", value: "sa", wantErr: true},
		{name: "ten spelled with two characters rejected", value: "S10", wantErr: true},
		{name: "unknown suit rejected", value: "XA", wantErr: true},
		{name: "unknown rank rejected", value: "S1", wantErr: true},
		{name: "empty rejected", value: "", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseCard(test.value)
			if test.wantErr {
				if err == nil {
					t.Fatalf("ParseCard(%q) error = nil", test.value)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseCard(%q) error = %v", test.value, err)
			}
			if got != test.want {
				t.Errorf("ParseCard(%q) = %+v, want %+v", test.value, got, test.want)
			}
			if got.String() != test.value {
				t.Errorf("String() = %q, want %q", got.String(), test.value)
			}
		})
	}
}

func TestFullDeck(t *testing.T) {
	t.Parallel()

	deck := FullDeck()
	if len(deck) != 52 {
		t.Fatalf("len(FullDeck()) = %d, want 52", len(deck))
	}

	seen := make(map[Card]struct{}, len(deck))
	for _, card := range deck {
		if err := card.Validate(); err != nil {
			t.Fatalf("invalid card %v: %v", card, err)
		}
		if _, exists := seen[card]; exists {
			t.Fatalf("duplicate card %s", card)
		}
		seen[card] = struct{}{}
	}
}

func FuzzParseCard(f *testing.F) {
	for _, seed := range []string{"SA", "C2", "DT", "", "S10", "♥A"} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, value string) {
		card, err := ParseCard(value)
		if err != nil {
			return
		}
		if card.String() != value {
			t.Fatalf("successful parse did not round trip: got %q, want %q", card.String(), value)
		}
		if err := card.Validate(); err != nil {
			t.Fatalf("successful parse produced invalid card: %v", err)
		}
	})
}
