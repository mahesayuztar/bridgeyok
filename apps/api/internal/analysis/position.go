package analysis

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/bridge"
)

type PositionRepository interface {
	AnalysisPosition(context.Context, string, string, string, *int) (bridge.State, error)
}

type CardPrediction struct {
	Card   bridge.Card `json:"card"`
	Tricks int         `json:"tricks"`
}

type PositionResponse struct {
	BoardID     string           `json:"boardId"`
	PositionKey string           `json:"positionKey"`
	Turn        bridge.Seat      `json:"turn"`
	Cards       []CardPrediction `json:"cards"`
}

func (service *Service) AnalyzePosition(ctx context.Context, boardID, sessionID, positionKey string, step *int) (PositionResponse, error) {
	if _, err := uuid.Parse(boardID); err != nil {
		return PositionResponse{}, ErrNotFound
	}
	repository, ok := service.repository.(PositionRepository)
	solver, solverOK := service.solver.(interface {
		SolvePosition(context.Context, bridge.State) ([]CardPrediction, error)
	})
	if !ok || !solverOK {
		return PositionResponse{}, ErrUnavailable
	}
	state, err := repository.AnalysisPosition(ctx, boardID, sessionID, positionKey, step)
	if err != nil {
		return PositionResponse{}, err
	}
	cards, err := solver.SolvePosition(ctx, state)
	if err != nil {
		return PositionResponse{}, err
	}
	return PositionResponse{BoardID: boardID, PositionKey: positionKey, Turn: state.Turn, Cards: cards}, nil
}

var ErrPositionChanged = fmt.Errorf("analysis position changed")

func ReplayPosition(final bridge.State, cards bridge.Deal, step int) (bridge.State, error) {
	plays := []bridge.PlayedCard{}
	for _, trick := range final.CompletedTricks {
		plays = append(plays, trick.Plays...)
	}
	plays = append(plays, final.CurrentTrick.Plays...)
	if step < 0 || step > len(plays) {
		return bridge.State{}, ErrPositionChanged
	}
	state, err := bridge.NewBoard(final.Board.Number, cards)
	if err != nil {
		return bridge.State{}, err
	}
	for _, call := range final.Auction.Calls {
		decision, err := bridge.Decide(state, bridge.MakeCallCommand(call.Seat, call.Call))
		if err != nil {
			return bridge.State{}, err
		}
		state = decision.NextState
	}
	for _, play := range plays[:step] {
		actor := state.Turn
		if state.Auction.Contract != nil && actor == state.Auction.Contract.Dummy() {
			actor = state.Auction.Contract.Declarer
		}
		decision, err := bridge.Decide(state, bridge.PlayCardCommand(actor, play.Card))
		if err != nil || play.Seat != state.Turn {
			return bridge.State{}, ErrUnavailable
		}
		state = decision.NextState
	}
	return state, nil
}
