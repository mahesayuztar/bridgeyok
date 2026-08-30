package bridge

import (
	"reflect"
	"testing"
)

func TestCallValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		call    Call
		wantErr bool
	}{
		{name: "pass", call: Pass()},
		{name: "one club", call: Bid(1, StrainClubs)},
		{name: "seven no trump", call: Bid(7, StrainNoTrump)},
		{name: "double", call: Double()},
		{name: "redouble", call: Redouble()},
		{name: "level zero", call: Bid(0, StrainClubs), wantErr: true},
		{name: "level eight", call: Bid(8, StrainClubs), wantErr: true},
		{name: "invalid strain", call: Bid(1, "X"), wantErr: true},
		{name: "pass with bid fields", call: Call{Kind: CallPass, Level: 1, Strain: StrainClubs}, wantErr: true},
		{name: "unknown kind", call: Call{Kind: "CLAIM"}, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := test.call.Validate()
			if (err != nil) != test.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func TestAuctionPassedOut(t *testing.T) {
	t.Parallel()

	auction := mustAuction(t, North)
	for _, actor := range []Seat{North, East, South, West} {
		var domainError *DomainError
		auction, domainError = auction.MakeCall(actor, Pass())
		if domainError != nil {
			t.Fatalf("MakeCall(%s, Pass) error = %v", actor, domainError)
		}
	}
	if !auction.Complete || !auction.PassedOut || auction.Contract != nil || auction.Turn != "" {
		t.Fatalf("passed-out auction = %+v", auction)
	}
	if err := auction.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestAuctionDeterminesFinalContractAndFirstDeclarer(t *testing.T) {
	t.Parallel()

	auction := runAuction(t, North, []Call{
		Bid(1, StrainHearts),
		Pass(),
		Bid(2, StrainHearts),
		Pass(),
		Pass(),
		Pass(),
	})

	want := &Contract{Level: 2, Strain: StrainHearts, Doubling: Undoubled, Declarer: North}
	if !reflect.DeepEqual(auction.Contract, want) {
		t.Fatalf("Contract = %+v, want %+v", auction.Contract, want)
	}
	if auction.Contract.Dummy() != South || auction.Contract.OpeningLeader() != East || auction.Contract.TargetTricks() != 8 {
		t.Fatalf("derived contract positions are incorrect: %+v", auction.Contract)
	}
}

func TestAuctionDoubleAndRedouble(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		calls    []Call
		doubling Doubling
	}{
		{
			name: "double after two passes",
			calls: []Call{
				Bid(1, StrainClubs), Pass(), Pass(), Double(), Pass(), Pass(), Pass(),
			},
			doubling: Doubled,
		},
		{
			name: "redouble",
			calls: []Call{
				Bid(1, StrainClubs), Double(), Redouble(), Pass(), Pass(), Pass(),
			},
			doubling: Redoubled,
		},
		{
			name: "new bid clears double",
			calls: []Call{
				Bid(1, StrainClubs), Double(), Bid(1, StrainDiamonds), Pass(), Pass(), Pass(),
			},
			doubling: Undoubled,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			auction := runAuction(t, North, test.calls)
			if auction.Contract == nil || auction.Contract.Doubling != test.doubling {
				t.Fatalf("Contract = %+v, want doubling %s", auction.Contract, test.doubling)
			}
		})
	}
}

func TestAuctionRejectsIllegalCallsWithoutMutation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		setup []Call
		actor Seat
		call  Call
		code  ErrorCode
	}{
		{name: "wrong turn", actor: East, call: Pass(), code: ErrorNotYourTurn},
		{name: "double without bid", actor: North, call: Double(), code: ErrorIllegalCall},
		{name: "invalid bid", actor: North, call: Bid(8, StrainClubs), code: ErrorIllegalCall},
		{name: "insufficient bid", setup: []Call{Bid(1, StrainSpades)}, actor: East, call: Bid(1, StrainHearts), code: ErrorIllegalCall},
		{name: "same side double", setup: []Call{Bid(1, StrainClubs), Pass()}, actor: South, call: Double(), code: ErrorIllegalCall},
		{name: "opponents cannot redouble", setup: []Call{Bid(1, StrainClubs), Double(), Pass()}, actor: West, call: Redouble(), code: ErrorIllegalCall},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			auction := runCalls(t, mustAuction(t, North), test.setup)
			before := auction.clone()
			after, domainError := auction.MakeCall(test.actor, test.call)
			if domainError == nil || domainError.Code != test.code {
				t.Fatalf("MakeCall() error = %+v, want code %s", domainError, test.code)
			}
			if !reflect.DeepEqual(after, before) || !reflect.DeepEqual(auction, before) {
				t.Fatal("rejected call mutated auction state")
			}
		})
	}
}

func TestAuctionRejectsCallAfterCompletion(t *testing.T) {
	t.Parallel()

	auction := runAuction(t, North, []Call{Pass(), Pass(), Pass(), Pass()})
	after, domainError := auction.MakeCall(North, Bid(1, StrainClubs))
	if domainError == nil || domainError.Code != ErrorAuctionComplete {
		t.Fatalf("MakeCall() error = %+v, want %s", domainError, ErrorAuctionComplete)
	}
	if !reflect.DeepEqual(after, auction) {
		t.Fatal("terminal call mutated auction")
	}
}

func TestAuctionAcceptedResultDoesNotAliasInput(t *testing.T) {
	t.Parallel()

	auction := runCalls(t, mustAuction(t, North), []Call{Bid(1, StrainClubs)})
	before := auction.clone()
	after, domainError := auction.MakeCall(East, Pass())
	if domainError != nil {
		t.Fatalf("MakeCall() error = %v", domainError)
	}
	after.Calls[0].Call = Bid(7, StrainNoTrump)
	if !reflect.DeepEqual(auction, before) {
		t.Fatal("mutating accepted auction changed input auction")
	}
}

func TestAuctionLegalCalls(t *testing.T) {
	t.Parallel()

	auction := runCalls(t, mustAuction(t, North), []Call{Bid(1, StrainSpades)})
	legal := auction.LegalCalls()
	if !containsCall(legal, Pass()) || !containsCall(legal, Bid(1, StrainNoTrump)) || !containsCall(legal, Double()) {
		t.Fatalf("LegalCalls() = %+v", legal)
	}
	if containsCall(legal, Bid(1, StrainHearts)) || containsCall(legal, Redouble()) {
		t.Fatalf("LegalCalls() contains illegal action: %+v", legal)
	}
}

func TestAuctionValidateRejectsTamperedState(t *testing.T) {
	t.Parallel()

	valid := runCalls(t, mustAuction(t, North), []Call{Bid(1, StrainClubs), Pass()})
	tests := []struct {
		name   string
		mutate func(Auction) Auction
	}{
		{
			name: "wrong turn summary",
			mutate: func(auction Auction) Auction {
				auction.Turn = North
				return auction
			},
		},
		{
			name: "wrong caller",
			mutate: func(auction Auction) Auction {
				auction.Calls[0].Seat = East
				return auction
			},
		},
		{
			name: "illegal history",
			mutate: func(auction Auction) Auction {
				auction.Calls[1].Call = Double()
				auction.Calls = append(auction.Calls, CallRecord{Seat: South, Call: Double()})
				auction.Turn = West
				return auction
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if err := test.mutate(valid.clone()).Validate(); err == nil {
				t.Fatal("Validate() error = nil")
			}
		})
	}
}

func mustAuction(t *testing.T, dealer Seat) Auction {
	t.Helper()
	auction, err := NewAuction(dealer)
	if err != nil {
		t.Fatalf("NewAuction(%s) error = %v", dealer, err)
	}
	return auction
}

func runAuction(t *testing.T, dealer Seat, calls []Call) Auction {
	t.Helper()
	return runCalls(t, mustAuction(t, dealer), calls)
}

func runCalls(t *testing.T, auction Auction, calls []Call) Auction {
	t.Helper()
	for _, call := range calls {
		var domainError *DomainError
		auction, domainError = auction.MakeCall(auction.Turn, call)
		if domainError != nil {
			t.Fatalf("MakeCall(%s, %+v) error = %v", auction.Turn, call, domainError)
		}
	}
	return auction
}

func containsCall(calls []Call, want Call) bool {
	for _, call := range calls {
		if call == want {
			return true
		}
	}
	return false
}
