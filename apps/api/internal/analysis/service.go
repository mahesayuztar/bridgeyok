package analysis

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/bridge"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/deal"
)

var (
	ErrNotFound     = errors.New("analysis board not found")
	ErrNotCompleted = errors.New("analysis requires a completed board")
	ErrUnavailable  = errors.New("analysis unavailable")
	ErrBusy         = errors.New("analysis capacity exhausted")
)

const SolverVersion = "dds-2.9.0-8d75755"

type Board struct {
	ID       string               `json:"boardId"`
	Metadata bridge.BoardMetadata `json:"metadata"`
	Source   deal.Result          `json:"-"`
}

type Repository interface {
	CompletedAnalysisBoard(context.Context, string, string) (Board, error)
}

type Solver interface {
	Solve(context.Context, bridge.Deal, bridge.BoardMetadata) (Result, error)
}

type MakeableContract struct {
	Declarer bridge.Seat   `json:"declarer"`
	Strain   bridge.Strain `json:"strain"`
	Level    int           `json:"level"`
}

type ParContract struct {
	Declarers   []bridge.Seat `json:"declarers"`
	Strain      bridge.Strain `json:"strain"`
	Level       int           `json:"level"`
	OverTricks  int           `json:"overTricks"`
	UnderTricks int           `json:"underTricks"`
	Doubled     bool          `json:"doubled"`
}

type Par struct {
	ScoreNS   int           `json:"scoreNS"`
	Contracts []ParContract `json:"contracts"`
}

type Result struct {
	SolverVersion     string                                `json:"solverVersion"`
	DoubleDummyTable  map[bridge.Seat]map[bridge.Strain]int `json:"doubleDummyTable"`
	MakeableContracts []MakeableContract                    `json:"makeableContracts"`
	Par               Par                                   `json:"par"`
}

type Response struct {
	BoardID    string               `json:"boardId"`
	Metadata   bridge.BoardMetadata `json:"metadata"`
	Provenance deal.Provenance      `json:"provenance"`
	Result     Result               `json:"analysis"`
}

type Service struct {
	repository Repository
	solver     Solver
}

func NewService(repository Repository, solver Solver) (*Service, error) {
	if repository == nil || solver == nil {
		return nil, fmt.Errorf("analysis repository and solver are required")
	}
	return &Service{repository: repository, solver: solver}, nil
}

func (service *Service) Analyze(ctx context.Context, boardID string, sessionID string) (Response, error) {
	if _, err := uuid.Parse(boardID); err != nil {
		return Response{}, ErrNotFound
	}
	board, err := service.repository.CompletedAnalysisBoard(ctx, boardID, sessionID)
	if err != nil {
		return Response{}, err
	}
	if board.ID != boardID {
		return Response{}, ErrUnavailable
	}
	result, err := service.solver.Solve(ctx, board.Source.Deal, board.Metadata)
	if err != nil {
		return Response{}, err
	}
	return Response{BoardID: board.ID, Metadata: board.Metadata, Provenance: board.Source.Provenance, Result: result}, nil
}
