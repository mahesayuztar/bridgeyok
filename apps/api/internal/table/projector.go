package table

import "github.com/mahesayuztar/bridgeyok/apps/api/internal/bridge"

// Projection is the only client-visible representation of private table state.
type Projection struct {
	TableID             string                         `json:"tableId"`
	State               State                          `json:"state"`
	Locked              bool                           `json:"locked"`
	Revision            int64                          `json:"revision"`
	LastSeq             int64                          `json:"lastSeq"`
	BoardID             string                         `json:"boardId,omitempty"`
	BoardNumber         int                            `json:"boardNumber"`
	ViewerParticipantID string                         `json:"viewerParticipantId"`
	ViewerRole          Role                           `json:"viewerRole"`
	ViewerSeat          bridge.Seat                    `json:"viewerSeat,omitempty"`
	Participants        []ProjectedParticipant         `json:"participants"`
	Seats               map[bridge.Seat]SeatAssignment `json:"seats"`
	Game                *ProjectedGame                 `json:"game,omitempty"`
}

// ProjectedParticipant omits session identifiers from client state.
type ProjectedParticipant struct {
	ID       string `json:"id"`
	Nickname string `json:"nickname"`
	Role     Role   `json:"role"`
}

// ProjectedGame contains public board state plus viewer-authorized cards.
type ProjectedGame struct {
	RulesetVersion  string               `json:"rulesetVersion"`
	Board           bridge.BoardMetadata `json:"board"`
	Phase           bridge.Phase         `json:"phase"`
	Auction         bridge.Auction       `json:"auction"`
	LegalCalls      []bridge.Call        `json:"legalCalls,omitempty"`
	Turn            bridge.Seat          `json:"turn,omitempty"`
	DummyRevealed   bool                 `json:"dummyRevealed"`
	CurrentTrick    bridge.Trick         `json:"currentTrick"`
	CompletedTricks []bridge.Trick       `json:"completedTricks"`
	TricksNS        int                  `json:"tricksNS"`
	TricksEW        int                  `json:"tricksEW"`
	Result          *bridge.Result       `json:"result,omitempty"`
	OwnHand         bridge.Hand          `json:"ownHand,omitempty"`
	DummyHand       bridge.Hand          `json:"dummyHand,omitempty"`
	FullDeal        *bridge.Deal         `json:"fullDeal,omitempty"`
}

// Project builds recipient-specific state without exposing credentials or hidden hands.
func Project(aggregate Aggregate, viewerSessionID string) (Projection, *DomainError) {
	viewer, exists := aggregate.activeParticipant(viewerSessionID)
	if !exists {
		return Projection{}, reject(ErrorNotParticipant, "viewer is not an active participant")
	}
	projection := Projection{
		TableID:             aggregate.ID,
		State:               aggregate.State,
		Locked:              aggregate.Locked,
		Revision:            aggregate.Revision,
		LastSeq:             aggregate.LastSeq,
		BoardID:             aggregate.BoardID,
		BoardNumber:         aggregate.BoardNumber,
		ViewerParticipantID: viewer.ID,
		ViewerRole:          viewer.Role,
		Participants:        make([]ProjectedParticipant, 0, len(aggregate.Participants)),
		Seats:               make(map[bridge.Seat]SeatAssignment, len(aggregate.Seats)),
	}
	for _, participant := range aggregate.Participants {
		if participant.LeftAt == nil {
			projection.Participants = append(projection.Participants, ProjectedParticipant{ID: participant.ID, Nickname: participant.Nickname, Role: participant.Role})
		}
	}
	for seat, assignment := range aggregate.Seats {
		projection.Seats[seat] = assignment
		if assignment.ParticipantID == viewer.ID {
			projection.ViewerSeat = seat
		}
	}
	if aggregate.Game == nil {
		return projection, nil
	}
	game := aggregate.Game
	projectedGame := &ProjectedGame{
		RulesetVersion:  game.RulesetVersion,
		Board:           game.Board,
		Phase:           game.Phase,
		Auction:         projectAuction(game.Auction),
		Turn:            game.Turn,
		DummyRevealed:   game.DummyRevealed,
		CurrentTrick:    projectTrick(game.CurrentTrick),
		CompletedTricks: make([]bridge.Trick, len(game.CompletedTricks)),
		TricksNS:        game.TricksNS,
		TricksEW:        game.TricksEW,
		Result:          projectResult(game.Result),
		OwnHand:         bridge.Hand{},
	}
	if game.Phase == bridge.PhaseAuction {
		projectedGame.LegalCalls = append([]bridge.Call(nil), game.Auction.LegalCalls()...)
	}
	for _index, trick := range game.CompletedTricks {
		projectedGame.CompletedTricks[_index] = projectTrick(trick)
	}
	if projection.ViewerSeat.Valid() {
		projectedGame.OwnHand = game.Deal.Hand(projection.ViewerSeat)
	}
	if game.DummyRevealed && game.Auction.Contract != nil {
		projectedGame.DummyHand = game.Deal.Hand(game.Auction.Contract.Dummy())
	}
	if game.Phase == bridge.PhaseBoardScored {
		deal := projectDeal(game.Deal)
		projectedGame.FullDeal = &deal
	}
	projection.Game = projectedGame
	return projection, nil
}

func projectAuction(auction bridge.Auction) bridge.Auction {
	projected := auction
	projected.Calls = append([]bridge.CallRecord(nil), auction.Calls...)
	if auction.Contract != nil {
		contract := *auction.Contract
		projected.Contract = &contract
	}
	return projected
}

func projectTrick(trick bridge.Trick) bridge.Trick {
	projected := trick
	projected.Plays = append([]bridge.PlayedCard(nil), trick.Plays...)
	return projected
}

func projectResult(result *bridge.Result) *bridge.Result {
	if result == nil {
		return nil
	}
	projected := *result
	if result.Contract != nil {
		contract := *result.Contract
		projected.Contract = &contract
	}
	return &projected
}

func projectDeal(deal bridge.Deal) bridge.Deal {
	return bridge.Deal{
		North: deal.Hand(bridge.North),
		East:  deal.Hand(bridge.East),
		South: deal.Hand(bridge.South),
		West:  deal.Hand(bridge.West),
	}
}
