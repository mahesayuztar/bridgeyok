package analysis

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/bridge"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/deal"
)

type boardRepositoryStub struct {
	board Board
	err   error
	calls int
}

func (repository *boardRepositoryStub) CompletedAnalysisBoard(_ context.Context, _ string, _ string) (Board, error) {
	repository.calls++
	return repository.board, repository.err
}

type solverStub struct {
	err   error
	calls int
}

func (solver *solverStub) Solve(_ context.Context, _ bridge.Deal, _ bridge.BoardMetadata) (Result, error) {
	solver.calls++
	return Result{SolverVersion: SolverVersion}, solver.err
}

func TestAnalysisUsesOnlyAuthorizedImmutableBoard(t *testing.T) {
	t.Parallel()
	boardID := uuid.NewString()
	for _, test := range []struct {
		name            string
		requestedID     string
		returnedID      string
		repositoryError error
		solverError     error
		wantError       error
		repositoryCalls int
		solverCalls     int
	}{
		{"completed", boardID, boardID, nil, nil, nil, 1, 1},
		{"invalid id", "invalid", boardID, nil, nil, ErrNotFound, 0, 0},
		{"unauthorized", boardID, boardID, ErrNotFound, nil, ErrNotFound, 1, 0},
		{"active", boardID, boardID, ErrNotCompleted, nil, ErrNotCompleted, 1, 0},
		{"wrong board", boardID, uuid.NewString(), nil, nil, ErrUnavailable, 1, 0},
		{"solver failure", boardID, boardID, nil, ErrUnavailable, ErrUnavailable, 1, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			repository := &boardRepositoryStub{board: Board{ID: test.returnedID, Source: deal.Result{Provenance: deal.Provenance{Type: "prepared", Version: "v1"}}}, err: test.repositoryError}
			solver := &solverStub{err: test.solverError}
			service, err := NewService(repository, solver)
			if err != nil {
				t.Fatal(err)
			}
			response, err := service.Analyze(t.Context(), test.requestedID, uuid.NewString())
			if !errors.Is(err, test.wantError) || repository.calls != test.repositoryCalls || solver.calls != test.solverCalls {
				t.Fatalf("Analyze error=%v repository calls=%d solver calls=%d", err, repository.calls, solver.calls)
			}
			if err == nil && (response.BoardID != boardID || response.Provenance != repository.board.Source.Provenance) {
				t.Fatal("analysis lost board provenance")
			}
		})
	}
	if _, err := NewService(nil, &solverStub{}); err == nil {
		t.Fatal("missing repository accepted")
	}
	if _, err := NewService(&boardRepositoryStub{}, nil); err == nil {
		t.Fatal("missing solver accepted")
	}
}
