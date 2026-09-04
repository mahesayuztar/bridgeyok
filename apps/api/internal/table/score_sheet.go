package table

import (
	"fmt"
	"sort"

	"github.com/mahesayuztar/bridgeyok/apps/api/internal/bridge"
)

// ScoreParticipant is an occupant identity captured for one board.
type ScoreParticipant struct {
	ID       string `json:"id"`
	Nickname string `json:"nickname"`
	IsBot    bool   `json:"isBot"`
}

// ScorePair identifies two board-start occupants independently of seat order.
type ScorePair struct {
	ID      string              `json:"id"`
	Members [2]ScoreParticipant `json:"members"`
}

// BoardLineup fixes score attribution when a board starts.
type BoardLineup struct {
	Seats      map[bridge.Seat]ScoreParticipant `json:"seats"`
	NorthSouth ScorePair                        `json:"northSouth"`
	EastWest   ScorePair                        `json:"eastWest"`
}

// ScoreSheetEntry records one durable duplicate result and its board-start pairs.
type ScoreSheetEntry struct {
	BoardID     string        `json:"boardId"`
	BoardNumber int           `json:"boardNumber"`
	Result      bridge.Result `json:"result"`
	Lineup      BoardLineup   `json:"lineup"`
}

// PairScoreTotal is one pair's side-oriented raw duplicate total.
type PairScoreTotal struct {
	Pair  ScorePair `json:"pair"`
	Score int       `json:"score"`
}

func (aggregate Aggregate) captureBoardLineup() (BoardLineup, error) {
	seats := make(map[bridge.Seat]ScoreParticipant, 4)
	for _, seat := range []bridge.Seat{bridge.North, bridge.East, bridge.South, bridge.West} {
		member, err := aggregate.scoreParticipantForSeat(seat)
		if err != nil {
			return BoardLineup{}, err
		}
		seats[seat] = member
	}
	northSouth, err := aggregate.captureScorePair(bridge.North, bridge.South)
	if err != nil {
		return BoardLineup{}, err
	}
	eastWest, err := aggregate.captureScorePair(bridge.East, bridge.West)
	if err != nil {
		return BoardLineup{}, err
	}
	return BoardLineup{Seats: seats, NorthSouth: northSouth, EastWest: eastWest}, nil
}

func (aggregate Aggregate) captureScorePair(firstSeat bridge.Seat, secondSeat bridge.Seat) (ScorePair, error) {
	members := [2]ScoreParticipant{}
	for _index, seat := range []bridge.Seat{firstSeat, secondSeat} {
		member, err := aggregate.scoreParticipantForSeat(seat)
		if err != nil {
			return ScorePair{}, err
		}
		members[_index] = member
	}
	sort.Slice(members[:], func(_firstIndex int, _secondIndex int) bool {
		return members[_firstIndex].ID < members[_secondIndex].ID
	})
	return ScorePair{ID: members[0].ID + ":" + members[1].ID, Members: members}, nil
}

func (aggregate Aggregate) scoreParticipantForSeat(seat bridge.Seat) (ScoreParticipant, error) {
	assignment, occupied := aggregate.Seats[seat]
	if !occupied {
		return ScoreParticipant{}, fmt.Errorf("score attribution requires occupied seat %s", seat)
	}
	member := ScoreParticipant{ID: assignment.ParticipantID, IsBot: assignment.IsBot}
	if assignment.IsBot {
		member.Nickname = "Bot"
	} else {
		for _, participant := range aggregate.Participants {
			if participant.ID == assignment.ParticipantID {
				member.Nickname = participant.Nickname
				break
			}
		}
	}
	if member.Nickname == "" {
		return ScoreParticipant{}, fmt.Errorf("score attribution participant %s is missing", assignment.ParticipantID)
	}
	return member, nil
}

func (aggregate *Aggregate) appendCurrentBoardScore() error {
	if aggregate.Game == nil || aggregate.Game.Result == nil || aggregate.BoardID == "" || aggregate.BoardNumber < 1 {
		return fmt.Errorf("completed board result is required")
	}
	if aggregate.CurrentBoardLineup == nil {
		lineup, err := aggregate.captureBoardLineup()
		if err != nil {
			return err
		}
		aggregate.CurrentBoardLineup = &lineup
	}
	for _, entry := range aggregate.ScoreSheet {
		if entry.BoardID == aggregate.BoardID {
			return nil
		}
	}
	result := *aggregate.Game.Result
	if result.Contract != nil {
		contract := *result.Contract
		result.Contract = &contract
	}
	aggregate.ScoreSheet = append(aggregate.ScoreSheet, ScoreSheetEntry{
		BoardID:     aggregate.BoardID,
		BoardNumber: aggregate.BoardNumber,
		Result:      result,
		Lineup:      *aggregate.CurrentBoardLineup,
	})
	return nil
}

func (aggregate *Aggregate) removeCurrentBoardScore() {
	for _index := len(aggregate.ScoreSheet) - 1; _index >= 0; _index-- {
		if aggregate.ScoreSheet[_index].BoardID == aggregate.BoardID {
			aggregate.ScoreSheet = append(aggregate.ScoreSheet[:_index], aggregate.ScoreSheet[_index+1:]...)
			return
		}
	}
}

func calculatePairScoreTotals(entries []ScoreSheetEntry) []PairScoreTotal {
	totals := make([]PairScoreTotal, 0)
	indexes := make(map[string]int)
	add := func(pair ScorePair, score int) {
		_index, exists := indexes[pair.ID]
		if !exists {
			indexes[pair.ID] = len(totals)
			totals = append(totals, PairScoreTotal{Pair: pair, Score: score})
			return
		}
		totals[_index].Score += score
	}
	for _, entry := range entries {
		add(entry.Lineup.NorthSouth, entry.Result.ScoreNS)
		add(entry.Lineup.EastWest, -entry.Result.ScoreNS)
	}
	return totals
}

func validateBoardLineup(lineup BoardLineup) error {
	if len(lineup.Seats) != 4 {
		return fmt.Errorf("score lineup requires four seats")
	}
	seen := make(map[string]struct{}, 4)
	for _, pair := range []ScorePair{lineup.NorthSouth, lineup.EastWest} {
		if pair.ID == "" {
			return fmt.Errorf("score pair id is required")
		}
		for _, member := range pair.Members {
			if member.ID == "" || member.Nickname == "" {
				return fmt.Errorf("score participant identity is required")
			}
			if _, duplicate := seen[member.ID]; duplicate {
				return fmt.Errorf("score participant appears in multiple seats")
			}
			seen[member.ID] = struct{}{}
		}
		if pair.Members[0].ID > pair.Members[1].ID || pair.ID != pair.Members[0].ID+":"+pair.Members[1].ID {
			return fmt.Errorf("score pair identity is not canonical")
		}
	}
	for _, seat := range []bridge.Seat{bridge.North, bridge.East, bridge.South, bridge.West} {
		member, exists := lineup.Seats[seat]
		if !exists || member.ID == "" || member.Nickname == "" {
			return fmt.Errorf("score lineup seat %s is invalid", seat)
		}
		pair := lineup.NorthSouth
		if seat.Partnership() == bridge.EastWest {
			pair = lineup.EastWest
		}
		if pair.Members[0].ID != member.ID && pair.Members[1].ID != member.ID {
			return fmt.Errorf("score lineup seat %s does not match partnership", seat)
		}
	}
	return nil
}

func validateScoreSheet(entries []ScoreSheetEntry, boardNumber int) error {
	seenBoards := make(map[string]struct{}, len(entries))
	previousBoardNumber := 0
	for _, entry := range entries {
		if entry.BoardID == "" || entry.BoardNumber <= previousBoardNumber || entry.BoardNumber > boardNumber {
			return fmt.Errorf("score sheet board ordering is invalid")
		}
		if _, duplicate := seenBoards[entry.BoardID]; duplicate {
			return fmt.Errorf("score sheet contains duplicate board")
		}
		if err := validateBoardLineup(entry.Lineup); err != nil {
			return err
		}
		if entry.Result.RulesetVersion != bridge.RulesetVersion || !entry.Result.Vulnerability.Valid() {
			return fmt.Errorf("score sheet result is invalid")
		}
		if entry.Result.PassedOut {
			if entry.Result.Contract != nil || entry.Result.ScoreNS != 0 {
				return fmt.Errorf("passed-out score sheet result is invalid")
			}
		} else if entry.Result.Contract == nil {
			return fmt.Errorf("score sheet contract is required")
		} else {
			expected, err := bridge.ScoreContract(*entry.Result.Contract, entry.Result.Vulnerability, entry.Result.TricksDeclarer)
			if err != nil || expected.ScoreNS != entry.Result.ScoreNS || expected.TricksNS != entry.Result.TricksNS || expected.TricksEW != entry.Result.TricksEW {
				return fmt.Errorf("score sheet result does not match duplicate scoring")
			}
		}
		seenBoards[entry.BoardID] = struct{}{}
		previousBoardNumber = entry.BoardNumber
	}
	return nil
}
