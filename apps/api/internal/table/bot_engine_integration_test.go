//go:build integration

package table

import (
	"context"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/mahesayuztar/bridgeyok/apps/api/internal/analysis"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/bridge"
)

func TestBotDecisionEngineUsesPinnedDDSPositionBoundary(t *testing.T) {
	aggregate := testStartedAggregate(t)
	for _, call := range []bridge.Call{bridge.Bid(1, bridge.StrainClubs), bridge.Pass(), bridge.Pass(), bridge.Pass()} {
		aggregate = acceptedDecision(t, aggregate, Command{Name: CommandMakeCall, SessionID: sessionForSeat(t, aggregate, aggregate.Game.Turn), Call: &call}).NextState
	}
	state := *aggregate.Game
	legal, domainError := state.LegalCards(state.Turn)
	if domainError != nil {
		t.Fatal(domainError)
	}
	solverPath, err := filepath.Abs("../../../../bin/bridgeyok-dds")
	if err != nil {
		t.Fatal(err)
	}
	solver, err := analysis.NewDDS(solverPath, 10*time.Second, 1)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewBotDecisionEngine(solver, BotDecisionEngineOptions{SampleCount: 1, ProgressiveSamples: []int{1}, TimeBudget: 10 * time.Second})
	decision, err := engine.decideCard(context.Background(), state, state.Turn, legal)
	if err != nil {
		t.Fatalf("decideCard() error = %v", err)
	}
	if !slices.Contains(legal, decision.card) || decision.ddsCalls != 1 || decision.samples != 1 {
		t.Fatalf("decision = %+v, legal = %v", decision, legal)
	}
}
