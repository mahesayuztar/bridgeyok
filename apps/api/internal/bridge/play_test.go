package bridge

import "testing"

func TestTrickWinner(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		strain Strain
		plays  []PlayedCard
		want   Seat
	}{
		{
			name:   "highest led suit wins no trump",
			strain: StrainNoTrump,
			plays: []PlayedCard{
				{Seat: North, Card: Card{Suit: Hearts, Rank: Ten}},
				{Seat: East, Card: Card{Suit: Hearts, Rank: Ace}},
				{Seat: South, Card: Card{Suit: Spades, Rank: Ace}},
				{Seat: West, Card: Card{Suit: Hearts, Rank: King}},
			},
			want: East,
		},
		{
			name:   "low trump beats led ace",
			strain: StrainSpades,
			plays: []PlayedCard{
				{Seat: East, Card: Card{Suit: Hearts, Rank: Ace}},
				{Seat: South, Card: Card{Suit: Hearts, Rank: King}},
				{Seat: West, Card: Card{Suit: Spades, Rank: Two}},
				{Seat: North, Card: Card{Suit: Clubs, Rank: Ace}},
			},
			want: West,
		},
		{
			name:   "highest trump wins",
			strain: StrainDiamonds,
			plays: []PlayedCard{
				{Seat: South, Card: Card{Suit: Clubs, Rank: Ace}},
				{Seat: West, Card: Card{Suit: Diamonds, Rank: Two}},
				{Seat: North, Card: Card{Suit: Diamonds, Rank: Queen}},
				{Seat: East, Card: Card{Suit: Clubs, Rank: King}},
			},
			want: North,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			winner, err := trickWinner(Trick{Leader: test.plays[0].Seat, Plays: test.plays}, test.strain)
			if err != nil {
				t.Fatalf("trickWinner() error = %v", err)
			}
			if winner != test.want {
				t.Errorf("trickWinner() = %s, want %s", winner, test.want)
			}
		})
	}
}

func TestTrickWinnerRejectsInvalidTrick(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		trick Trick
	}{
		{name: "too short", trick: Trick{Leader: North, Plays: []PlayedCard{{Seat: North, Card: Card{Suit: Clubs, Rank: Ace}}}}},
		{name: "wrong order", trick: Trick{Leader: North, Plays: []PlayedCard{{Seat: East, Card: Card{Suit: Clubs, Rank: Ace}}}}},
		{name: "invalid leader", trick: Trick{Leader: "X", Plays: []PlayedCard{}}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, err := trickWinner(test.trick, StrainNoTrump); err == nil {
				t.Fatal("trickWinner() error = nil")
			}
		})
	}
	valid := Trick{Leader: North, Plays: []PlayedCard{
		{Seat: North, Card: Card{Suit: Clubs, Rank: Ace}},
		{Seat: East, Card: Card{Suit: Clubs, Rank: King}},
		{Seat: South, Card: Card{Suit: Clubs, Rank: Queen}},
		{Seat: West, Card: Card{Suit: Clubs, Rank: Jack}},
	}}
	if _, err := trickWinner(valid, "INVALID"); err == nil {
		t.Fatal("trickWinner(invalid strain) error = nil")
	}
}
