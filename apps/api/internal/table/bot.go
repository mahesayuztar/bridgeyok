package table

import (
	"slices"

	"github.com/mahesayuztar/bridgeyok/apps/api/internal/bridge"
)

func nextBotCommand(aggregate Aggregate) (Command, bool) {
	if aggregate.ActionRequest != nil {
		for _, seat := range []bridge.Seat{bridge.North, bridge.East, bridge.South, bridge.West} {
			accepted, ready := botConsensusResponse(aggregate, seat)
			if !ready {
				continue
			}
			name := CommandRespondClaim
			if aggregate.ActionRequest.Kind == ActionRequestUndo {
				name = CommandRespondUndo
			}
			return Command{Name: name, BotSeat: seat, Accepted: accepted}, true
		}
		return Command{}, false
	}
	if aggregate.State != StateActive || aggregate.Game == nil {
		return Command{}, false
	}

	game := aggregate.Game
	switch game.Phase {
	case bridge.PhaseAuction:
		assignment, seated := aggregate.Seats[game.Turn]
		legalCalls := game.Auction.LegalCalls()
		if !seated || !assignment.IsBot || len(legalCalls) == 0 {
			return Command{}, false
		}
		return Command{Name: CommandMakeCall, BotSeat: game.Turn, Call: &legalCalls[0]}, true
	case bridge.PhaseOpeningLead, bridge.PhasePlay:
		actorSeat := game.Turn
		if game.Auction.Contract != nil && game.Turn == game.Auction.Contract.Dummy() {
			actorSeat = game.Auction.Contract.Declarer
		}
		assignment, seated := aggregate.Seats[actorSeat]
		if !seated || !assignment.IsBot {
			return Command{}, false
		}
		legalCards, domainError := game.LegalCards(actorSeat)
		if domainError != nil || len(legalCards) == 0 {
			return Command{}, false
		}
		return Command{Name: CommandPlayCard, BotSeat: actorSeat, Card: &legalCards[0]}, true
	default:
		return Command{}, false
	}
}

func botConsensusResponse(aggregate Aggregate, seat bridge.Seat) (bool, bool) {
	request := aggregate.ActionRequest
	if request == nil || !aggregate.Seats[seat].IsBot || seat == request.RequesterSeat || slices.Contains(request.ApprovedBy, seat) {
		return false, false
	}
	if request.Kind == ActionRequestClaim && seat.Partnership() == request.RequesterSeat.Partnership() {
		return false, false
	}
	partner := seat.Partner()
	assignment, occupied := aggregate.Seats[partner]
	if !occupied {
		return false, false
	}
	if assignment.IsBot {
		return false, true
	}
	if partner == request.RequesterSeat || slices.Contains(request.ApprovedBy, partner) {
		return true, true
	}
	return false, false
}
