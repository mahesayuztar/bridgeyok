package bridge

import "testing"

func TestSeatRotationAndPartnership(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		seat        Seat
		next        Seat
		partner     Seat
		partnership Partnership
	}{
		{name: "north", seat: North, next: East, partner: South, partnership: NorthSouth},
		{name: "east", seat: East, next: South, partner: West, partnership: EastWest},
		{name: "south", seat: South, next: West, partner: North, partnership: NorthSouth},
		{name: "west", seat: West, next: North, partner: East, partnership: EastWest},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if err := test.seat.Validate(); err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
			if got := test.seat.Next(); got != test.next {
				t.Errorf("Next() = %s, want %s", got, test.next)
			}
			if got := test.seat.Partner(); got != test.partner {
				t.Errorf("Partner() = %s, want %s", got, test.partner)
			}
			if got := test.seat.Partnership(); got != test.partnership {
				t.Errorf("Partnership() = %s, want %s", got, test.partnership)
			}
		})
	}
}

func TestSeatValidateRejectsUnknownSeat(t *testing.T) {
	t.Parallel()

	if err := Seat("X").Validate(); err == nil {
		t.Fatal("Validate() error = nil, want invalid seat error")
	}
}

func TestPartnershipOpponent(t *testing.T) {
	t.Parallel()

	if !NorthSouth.Opponent(EastWest) || !EastWest.Opponent(NorthSouth) {
		t.Fatal("opposing partnerships were not recognized")
	}
	if NorthSouth.Opponent(NorthSouth) {
		t.Fatal("same partnership reported as opponent")
	}
}
