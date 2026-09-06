package analysis

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/mahesayuztar/bridgeyok/apps/api/internal/bridge"
)

type goldenFixture struct {
	Name        string      `json:"name"`
	BoardNumber int         `json:"boardNumber"`
	Deal        bridge.Deal `json:"deal"`
	Expected    Result      `json:"expected"`
}

func goldenFixtures(t *testing.T) []goldenFixture {
	t.Helper()
	data, err := os.ReadFile("testdata/dds-golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Fixtures []goldenFixture `json:"fixtures"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	if len(document.Fixtures) != 3 {
		t.Fatal("missing DDS fixtures")
	}
	return document.Fixtures
}

func TestDDSRejectsUnavailableAndMalformedSolver(t *testing.T) {
	t.Parallel()
	fixture := goldenFixtures(t)[0]
	metadata, err := bridge.MetadataForBoard(fixture.BoardNumber)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name   string
		script string
		want   error
	}{
		{"failure", "exit 3", ErrUnavailable},
		{"malformed", "printf secret", ErrUnavailable},
		{"oversized", "head -c 20000 /dev/zero", ErrUnavailable},
		{"timeout", "exec sleep 10", context.DeadlineExceeded},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "solver")
			if err := os.WriteFile(path, []byte("#!/bin/sh\n"+test.script+"\n"), 0700); err != nil {
				t.Fatal(err)
			}
			solver, err := NewDDS(path, 100*time.Millisecond, 1)
			if err != nil {
				t.Fatal(err)
			}
			before, err := json.Marshal(fixture.Deal)
			if err != nil {
				t.Fatal(err)
			}
			result, err := solver.Solve(t.Context(), fixture.Deal, metadata)
			if !errors.Is(err, test.want) || !reflect.DeepEqual(result, Result{}) {
				t.Fatalf("Solve error=%v result=%+v", err, result)
			}
			after, err := json.Marshal(fixture.Deal)
			if err != nil {
				t.Fatal(err)
			}
			if string(before) != string(after) {
				t.Fatal("solver mutated immutable input")
			}
			if len(solver.slots) != 0 {
				t.Fatal("failed solver retained capacity")
			}
		})
	}
	solver, err := NewDDS("/missing/dds", time.Second, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := solver.Solve(t.Context(), fixture.Deal, metadata); !errors.Is(err, ErrUnavailable) {
		t.Fatal("missing solver accepted")
	}
	solver.slots <- struct{}{}
	if _, err := solver.Solve(t.Context(), fixture.Deal, metadata); !errors.Is(err, ErrBusy) {
		t.Fatal("unbounded solver concurrency")
	}
	<-solver.slots
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := solver.Solve(ctx, fixture.Deal, metadata); !errors.Is(err, context.Canceled) {
		t.Fatal("canceled solve started")
	}
	if _, err := solver.Solve(t.Context(), bridge.Deal{}, metadata); !errors.Is(err, ErrUnavailable) {
		t.Fatal("invalid deal accepted")
	}
	invalidMetadata := metadata
	invalidMetadata.Dealer = bridge.East
	if _, err := solver.Solve(t.Context(), fixture.Deal, invalidMetadata); !errors.Is(err, ErrUnavailable) {
		t.Fatal("changed board metadata accepted")
	}
}

func TestDDSOutputMappingRejectsInvalidResults(t *testing.T) {
	t.Parallel()
	valid := solverOutput{SolverVersion: SolverVersion, Table: [][]int{{6, 6, 6, 6}, {6, 6, 6, 6}, {6, 6, 6, 6}, {6, 6, 6, 6}, {6, 6, 6, 6}}, Contracts: nil}
	data, err := json.Marshal(valid)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name   string
		mutate func(*solverOutput)
	}{
		{"wrong version", func(raw *solverOutput) { raw.SolverVersion = "unknown" }},
		{"missing strain", func(raw *solverOutput) { raw.Table = raw.Table[:4] }},
		{"missing seat", func(raw *solverOutput) { raw.Table[0] = []int{6} }},
		{"invalid tricks", func(raw *solverOutput) { raw.Table[0][0] = 14 }},
		{"missing contracts", func(raw *solverOutput) { raw.Contracts = nil }},
	} {
		t.Run(test.name, func(t *testing.T) {
			var raw solverOutput
			if err := json.Unmarshal(data, &raw); err != nil {
				t.Fatal(err)
			}
			raw.Contracts = []solverContract{}
			test.mutate(&raw)
			if _, err := mapSolverOutput(raw); !errors.Is(err, ErrUnavailable) {
				t.Fatal("malformed output accepted")
			}
		})
	}
	if err := json.Unmarshal([]byte(`{"solverVersion":"dds-2.9.0-8d75755","table":[[6,6,6,6],[6,6,6,6],[6,6,6,6],[6,6,6,6],[6,6,6,6]],"scoreNS":0,"contracts":[]}`), &valid); err != nil {
		t.Fatal(err)
	}
	result, err := mapSolverOutput(valid)
	if err != nil {
		t.Fatal(err)
	}
	if result.Par.ScoreNS != 0 || len(result.Par.Contracts) != 0 || len(result.MakeableContracts) != 0 {
		t.Fatal("passed out mapping incorrect")
	}
}

func TestDDSParSeatStrainAndMalformedContracts(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name         string
		seats        int
		denomination int
		wantSeats    []bridge.Seat
		wantStrain   bridge.Strain
	}{
		{"north notrump", 0, 0, []bridge.Seat{bridge.North}, bridge.StrainNoTrump},
		{"east spades", 1, 1, []bridge.Seat{bridge.East}, bridge.StrainSpades},
		{"south hearts", 2, 2, []bridge.Seat{bridge.South}, bridge.StrainHearts},
		{"west diamonds", 3, 3, []bridge.Seat{bridge.West}, bridge.StrainDiamonds},
		{"ns clubs", 4, 4, []bridge.Seat{bridge.North, bridge.South}, bridge.StrainClubs},
		{"ew notrump", 5, 0, []bridge.Seat{bridge.East, bridge.West}, bridge.StrainNoTrump},
	} {
		t.Run(test.name, func(t *testing.T) {
			raw := solverOutput{SolverVersion: SolverVersion, Table: [][]int{{13, 0, 7, 6}, {6, 6, 6, 6}, {6, 6, 6, 6}, {6, 6, 6, 6}, {6, 6, 6, 6}}, ScoreNS: 100, Contracts: []solverContract{{Level: 4, Denomination: test.denomination, Seats: test.seats, UnderTricks: 1}}}
			result, err := mapSolverOutput(raw)
			if err != nil {
				t.Fatal(err)
			}
			contract := result.Par.Contracts[0]
			if !reflect.DeepEqual(contract.Declarers, test.wantSeats) || contract.Strain != test.wantStrain || !contract.Doubled || contract.UnderTricks != 1 {
				t.Fatal("DDS par encodings mapped incorrectly")
			}
			if len(result.MakeableContracts) != 2 || result.MakeableContracts[0].Level != 7 || result.MakeableContracts[1].Level != 1 {
				t.Fatal("makeable boundary levels incorrect")
			}
		})
	}
	for _, test := range []struct {
		name     string
		contract solverContract
	}{
		{"zero level", solverContract{}}, {"unknown seats", solverContract{Level: 1, Seats: 6}}, {"unknown denomination", solverContract{Level: 1, Denomination: 5}}, {"too many tricks", solverContract{Level: 7, OverTricks: 1}}, {"negative tricks", solverContract{Level: 1, UnderTricks: -1}}, {"contradictory tricks", solverContract{Level: 4, UnderTricks: 1, OverTricks: 1}},
	} {
		t.Run(test.name, func(t *testing.T) {
			raw := solverOutput{SolverVersion: SolverVersion, Table: [][]int{{6, 6, 6, 6}, {6, 6, 6, 6}, {6, 6, 6, 6}, {6, 6, 6, 6}, {6, 6, 6, 6}}, ScoreNS: 100, Contracts: []solverContract{test.contract}}
			if _, err := mapSolverOutput(raw); !errors.Is(err, ErrUnavailable) {
				t.Fatal("malformed par accepted")
			}
		})
	}
}
