package table

import "github.com/mahesayuztar/bridgeyok/apps/api/internal/bridge"

func nextBotCommand(aggregate Aggregate) (Command, bool) {
	if aggregate.State != StateActive || aggregate.Game == nil || aggregate.ActionRequest != nil {
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
