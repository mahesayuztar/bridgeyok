package analysis

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/mahesayuztar/bridgeyok/apps/api/internal/bridge"
)

type DDS struct {
	executable string
	timeout    time.Duration
	slots      chan struct{}
}

type solverOutput struct {
	SolverVersion string           `json:"solverVersion"`
	Table         [][]int          `json:"table"`
	ScoreNS       int              `json:"scoreNS"`
	Contracts     []solverContract `json:"contracts"`
}

type solverContract struct {
	Level        int `json:"level"`
	Denomination int `json:"denomination"`
	Seats        int `json:"seats"`
	OverTricks   int `json:"overTricks"`
	UnderTricks  int `json:"underTricks"`
}

type boundedOutput struct{ bytes.Buffer }

func (output *boundedOutput) Write(value []byte) (int, error) {
	if output.Len()+len(value) > 16384 {
		return 0, ErrUnavailable
	}
	return output.Buffer.Write(value)
}

func NewDDS(executable string, timeout time.Duration, concurrency int) (*DDS, error) {
	if executable == "" || timeout <= 0 || timeout > time.Minute || concurrency < 1 || concurrency > 4 {
		return nil, fmt.Errorf("invalid DDS configuration")
	}
	return &DDS{executable: executable, timeout: timeout, slots: make(chan struct{}, concurrency)}, nil
}

func (solver *DDS) Solve(ctx context.Context, cards bridge.Deal, metadata bridge.BoardMetadata) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if err := cards.Validate(); err != nil {
		return Result{}, ErrUnavailable
	}
	expected, err := bridge.MetadataForBoard(metadata.Number)
	if err != nil || expected != metadata {
		return Result{}, ErrUnavailable
	}
	select {
	case solver.slots <- struct{}{}:
	default:
		return Result{}, ErrBusy
	}
	defer func() { <-solver.slots }()
	ctx, cancel := context.WithTimeout(ctx, solver.timeout)
	defer cancel()
	seats := []bridge.Seat{bridge.North, bridge.East, bridge.South, bridge.West}
	suits := []bridge.Suit{bridge.Spades, bridge.Hearts, bridge.Diamonds, bridge.Clubs}
	dealer := 0
	for _seatIndex, seat := range seats {
		if seat == metadata.Dealer {
			dealer = _seatIndex
		}
	}
	vulnerability := map[bridge.Vulnerability]int{bridge.VulnerabilityNone: 0, bridge.VulnerabilityBoth: 1, bridge.VulnerabilityNS: 2, bridge.VulnerabilityEW: 3}[metadata.Vulnerability]
	var input strings.Builder
	fmt.Fprintf(&input, "%d %d", dealer, vulnerability)
	for _, seat := range seats {
		for _, suit := range suits {
			holding := 0
			for _, card := range cards.Hand(seat) {
				if card.Suit == suit {
					holding |= 1 << (strings.Index("23456789TJQKA", string(card.Rank)) + 2)
				}
			}
			fmt.Fprintf(&input, " %d", holding)
		}
	}
	command := exec.CommandContext(ctx, solver.executable)
	command.Stdin = strings.NewReader(input.String())
	var output boundedOutput
	command.Stdout = &output
	command.Stderr = io.Discard
	command.WaitDelay = time.Second
	if err := command.Run(); err != nil {
		if ctx.Err() != nil {
			return Result{}, ctx.Err()
		}
		return Result{}, ErrUnavailable
	}
	decoder := json.NewDecoder(&output)
	decoder.DisallowUnknownFields()
	var raw solverOutput
	if err := decoder.Decode(&raw); err != nil {
		return Result{}, ErrUnavailable
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return Result{}, ErrUnavailable
	}
	return mapSolverOutput(raw)
}

func mapSolverOutput(raw solverOutput) (Result, error) {
	if raw.SolverVersion != SolverVersion || len(raw.Table) != 5 || raw.Contracts == nil || len(raw.Contracts) > 10 || raw.ScoreNS < -7600 || raw.ScoreNS > 7600 {
		return Result{}, ErrUnavailable
	}
	seats := []bridge.Seat{bridge.North, bridge.East, bridge.South, bridge.West}
	strains := []bridge.Strain{bridge.StrainSpades, bridge.StrainHearts, bridge.StrainDiamonds, bridge.StrainClubs, bridge.StrainNoTrump}
	result := Result{SolverVersion: raw.SolverVersion, DoubleDummyTable: make(map[bridge.Seat]map[bridge.Strain]int, 4), MakeableContracts: []MakeableContract{}, Par: Par{ScoreNS: raw.ScoreNS, Contracts: []ParContract{}}}
	for _, seat := range seats {
		result.DoubleDummyTable[seat] = make(map[bridge.Strain]int, 5)
	}
	for _strainIndex, row := range raw.Table {
		if len(row) != 4 {
			return Result{}, ErrUnavailable
		}
		for _seatIndex, tricks := range row {
			if tricks < 0 || tricks > 13 {
				return Result{}, ErrUnavailable
			}
			seat, strain := seats[_seatIndex], strains[_strainIndex]
			result.DoubleDummyTable[seat][strain] = tricks
			if tricks > 6 {
				result.MakeableContracts = append(result.MakeableContracts, MakeableContract{Declarer: seat, Strain: strain, Level: tricks - 6})
			}
		}
	}
	if (raw.ScoreNS == 0) != (len(raw.Contracts) == 0) {
		return Result{}, ErrUnavailable
	}
	parStrains := []bridge.Strain{bridge.StrainNoTrump, bridge.StrainSpades, bridge.StrainHearts, bridge.StrainDiamonds, bridge.StrainClubs}
	for _, contract := range raw.Contracts {
		if contract.Level < 1 || contract.Level > 7 || contract.Denomination < 0 || contract.Denomination > 4 || contract.Seats < 0 || contract.Seats > 5 || contract.OverTricks < 0 || contract.UnderTricks < 0 || contract.Level+contract.OverTricks > 7 || contract.UnderTricks > contract.Level+6 || (contract.OverTricks > 0 && contract.UnderTricks > 0) {
			return Result{}, ErrUnavailable
		}
		declarers := []bridge.Seat{}
		if contract.Seats < 4 {
			declarers = append(declarers, seats[contract.Seats])
		} else if contract.Seats == 4 {
			declarers = append(declarers, bridge.North, bridge.South)
		} else {
			declarers = append(declarers, bridge.East, bridge.West)
		}
		result.Par.Contracts = append(result.Par.Contracts, ParContract{Declarers: declarers, Strain: parStrains[contract.Denomination], Level: contract.Level, OverTricks: contract.OverTricks, UnderTricks: contract.UnderTricks, Doubled: contract.UnderTricks > 0})
	}
	return result, nil
}
