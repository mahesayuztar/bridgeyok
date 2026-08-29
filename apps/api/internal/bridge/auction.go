package bridge

import (
	"fmt"
	"reflect"
)

// Strain identifies a bid denomination in ascending auction order.
type Strain string

const (
	StrainClubs    Strain = "C"
	StrainDiamonds Strain = "D"
	StrainHearts   Strain = "H"
	StrainSpades   Strain = "S"
	StrainNoTrump  Strain = "NT"
)

var strains = [...]Strain{StrainClubs, StrainDiamonds, StrainHearts, StrainSpades, StrainNoTrump}

// CallKind identifies pass, bid, double, or redouble.
type CallKind string

const (
	CallPass     CallKind = "PASS"
	CallBid      CallKind = "BID"
	CallDouble   CallKind = "DOUBLE"
	CallRedouble CallKind = "REDOUBLE"
)

// Doubling identifies the multiplier applied to a final contract.
type Doubling string

const (
	Undoubled Doubling = "UNDOUBLED"
	Doubled   Doubling = "DOUBLED"
	Redoubled Doubling = "REDOUBLED"
)

// Call is one auction action.
type Call struct {
	Kind   CallKind `json:"kind"`
	Level  int      `json:"level,omitempty"`
	Strain Strain   `json:"strain,omitempty"`
}

// CallRecord associates a legal call with its caller.
type CallRecord struct {
	Seat Seat `json:"seat"`
	Call Call `json:"call"`
}

// Contract is the final bid, multiplier, and declarer.
type Contract struct {
	Level    int      `json:"level"`
	Strain   Strain   `json:"strain"`
	Doubling Doubling `json:"doubling"`
	Declarer Seat     `json:"declarer"`
}

// Auction contains the immutable result of calls made so far.
type Auction struct {
	Dealer    Seat         `json:"dealer"`
	Turn      Seat         `json:"turn,omitempty"`
	Calls     []CallRecord `json:"calls"`
	Complete  bool         `json:"complete"`
	PassedOut bool         `json:"passedOut"`
	Contract  *Contract    `json:"contract,omitempty"`
}

// Pass returns the canonical pass call.
func Pass() Call {
	return Call{Kind: CallPass}
}

// Bid returns a bid call. Legality is evaluated when it is submitted.
func Bid(level int, strain Strain) Call {
	return Call{Kind: CallBid, Level: level, Strain: strain}
}

// Double returns the canonical double call.
func Double() Call {
	return Call{Kind: CallDouble}
}

// Redouble returns the canonical redouble call.
func Redouble() Call {
	return Call{Kind: CallRedouble}
}

// Valid reports whether the strain is one of C, D, H, S, or NT.
func (strain Strain) Valid() bool {
	return strain.value() >= 0
}

func (strain Strain) value() int {
	for _index, candidate := range strains {
		if candidate == strain {
			return _index
		}
	}
	return -1
}

// Validate enforces the structural shape of a call.
func (call Call) Validate() error {
	switch call.Kind {
	case CallPass, CallDouble, CallRedouble:
		if call.Level != 0 || call.Strain != "" {
			return fmt.Errorf("%s call must not include level or strain", call.Kind)
		}
		return nil
	case CallBid:
		if call.Level < 1 || call.Level > 7 {
			return fmt.Errorf("bid level %d is outside 1 through 7", call.Level)
		}
		if !call.Strain.Valid() {
			return fmt.Errorf("invalid bid strain %q", call.Strain)
		}
		return nil
	default:
		return fmt.Errorf("invalid call kind %q", call.Kind)
	}
}

func (call Call) higherThan(other Call) bool {
	return call.Kind == CallBid && other.Kind == CallBid &&
		(call.Level > other.Level || call.Level == other.Level && call.Strain.value() > other.Strain.value())
}

// TargetTricks returns the tricks required to fulfill the contract.
func (contract Contract) TargetTricks() int {
	return 6 + contract.Level
}

// Dummy returns declarer's partner.
func (contract Contract) Dummy() Seat {
	return contract.Declarer.Partner()
}

// OpeningLeader returns the defender seated to declarer's left.
func (contract Contract) OpeningLeader() Seat {
	return contract.Declarer.Next()
}

// Validate enforces a complete and scoreable contract.
func (contract Contract) Validate() error {
	if err := Bid(contract.Level, contract.Strain).Validate(); err != nil {
		return err
	}
	if err := contract.Declarer.Validate(); err != nil {
		return err
	}
	switch contract.Doubling {
	case Undoubled, Doubled, Redoubled:
		return nil
	default:
		return fmt.Errorf("invalid doubling state %q", contract.Doubling)
	}
}

// NewAuction starts an empty auction with the dealer on turn.
func NewAuction(dealer Seat) (Auction, error) {
	if err := dealer.Validate(); err != nil {
		return Auction{}, err
	}
	return Auction{Dealer: dealer, Turn: dealer, Calls: []CallRecord{}}, nil
}

// MakeCall returns a new auction after accepting one legal in-turn call.
func (auction Auction) MakeCall(actor Seat, call Call) (Auction, *DomainError) {
	if err := auction.Validate(); err != nil {
		return auction, reject(ErrorInvalidState, err.Error())
	}
	if auction.Complete {
		return auction, reject(ErrorAuctionComplete, "auction is already complete")
	}
	if actor != auction.Turn {
		return auction, reject(ErrorNotYourTurn, "call actor is not on turn")
	}
	if err := call.Validate(); err != nil {
		return auction, reject(ErrorIllegalCall, err.Error())
	}
	if !auction.isLegal(call) {
		return auction, reject(ErrorIllegalCall, "call is not legal in the current auction")
	}

	next := auction.clone()
	next.Calls = append(next.Calls, CallRecord{Seat: actor, Call: call})
	next.Turn = actor.Next()
	next.resolveCompletion()
	return next, nil
}

// LegalCalls returns every call the current player can legally make in stable order.
func (auction Auction) LegalCalls() []Call {
	if auction.Complete || auction.Validate() != nil {
		return nil
	}

	legal := []Call{Pass()}
	lastBid, _, _, hasBid := auction.currentContract()
	for level := 1; level <= 7; level++ {
		for _, strain := range strains {
			candidate := Bid(level, strain)
			if !hasBid || candidate.higherThan(lastBid) {
				legal = append(legal, candidate)
			}
		}
	}
	for _, candidate := range []Call{Double(), Redouble()} {
		if auction.isLegal(candidate) {
			legal = append(legal, candidate)
		}
	}
	return legal
}

func (auction Auction) isLegal(call Call) bool {
	if call.Kind == CallPass {
		return true
	}
	lastBid, bidder, doubling, hasBid := auction.currentContract()
	switch call.Kind {
	case CallBid:
		return !hasBid || call.higherThan(lastBid)
	case CallDouble:
		return hasBid && doubling == Undoubled && auction.Turn.Partnership().Opponent(bidder.Partnership())
	case CallRedouble:
		return hasBid && doubling == Doubled && auction.Turn.Partnership() == bidder.Partnership()
	default:
		return false
	}
}

func (auction Auction) currentContract() (Call, Seat, Doubling, bool) {
	lastBid := Call{}
	bidder := Seat("")
	doubling := Undoubled
	found := false
	for _, record := range auction.Calls {
		switch record.Call.Kind {
		case CallBid:
			lastBid = record.Call
			bidder = record.Seat
			doubling = Undoubled
			found = true
		case CallDouble:
			doubling = Doubled
		case CallRedouble:
			doubling = Redoubled
		case CallPass:
		}
	}
	return lastBid, bidder, doubling, found
}

func (auction *Auction) resolveCompletion() {
	consecutivePasses := 0
	for _index := len(auction.Calls) - 1; _index >= 0; _index-- {
		if auction.Calls[_index].Call.Kind != CallPass {
			break
		}
		consecutivePasses++
	}

	lastBid, bidder, doubling, hasBid := auction.currentContract()
	if !hasBid && len(auction.Calls) == 4 && consecutivePasses == 4 {
		auction.Complete = true
		auction.PassedOut = true
		auction.Turn = ""
		return
	}
	if !hasBid || consecutivePasses != 3 {
		return
	}

	declarer := bidder
	for _, record := range auction.Calls {
		if record.Call.Kind == CallBid && record.Call.Strain == lastBid.Strain && record.Seat.Partnership() == bidder.Partnership() {
			declarer = record.Seat
			break
		}
	}
	auction.Complete = true
	auction.Turn = ""
	auction.Contract = &Contract{
		Level:    lastBid.Level,
		Strain:   lastBid.Strain,
		Doubling: doubling,
		Declarer: declarer,
	}
}

func (auction Auction) clone() Auction {
	clone := auction
	clone.Calls = append([]CallRecord{}, auction.Calls...)
	if auction.Contract != nil {
		contract := *auction.Contract
		clone.Contract = &contract
	}
	return clone
}

// Validate replays the call history and rejects tampered auction state.
func (auction Auction) Validate() error {
	if err := auction.Dealer.Validate(); err != nil {
		return fmt.Errorf("dealer: %w", err)
	}
	if auction.Calls == nil {
		return fmt.Errorf("calls must be initialized")
	}

	replayed := Auction{Dealer: auction.Dealer, Turn: auction.Dealer, Calls: []CallRecord{}}
	for _index, record := range auction.Calls {
		if record.Seat != replayed.Turn {
			return fmt.Errorf("call %d belongs to %s while %s is on turn", _index, record.Seat, replayed.Turn)
		}
		if err := record.Call.Validate(); err != nil {
			return fmt.Errorf("call %d: %w", _index, err)
		}
		if !replayed.isLegal(record.Call) {
			return fmt.Errorf("call %d is illegal", _index)
		}
		replayed.Calls = append(replayed.Calls, record)
		replayed.Turn = record.Seat.Next()
		replayed.resolveCompletion()
		if replayed.Complete && _index != len(auction.Calls)-1 {
			return fmt.Errorf("call history continues after completion")
		}
	}

	if auction.Turn != replayed.Turn || auction.Complete != replayed.Complete || auction.PassedOut != replayed.PassedOut || !reflect.DeepEqual(auction.Contract, replayed.Contract) {
		return fmt.Errorf("auction summary does not match call history")
	}
	return nil
}
