package match

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/mahesayuztar/bridgeyok/apps/api/internal/bridge"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/deal"
)

type Room string

type Status string

const (
	Open      Room   = "OPEN"
	Closed    Room   = "CLOSED"
	Waiting   Status = "WAITING"
	Active    Status = "ACTIVE"
	Complete  Status = "COMPLETE"
	Cancelled Status = "CANCELLED"
	MaxBoards        = 32
)

var (
	ErrInvalid   = errors.New("invalid match configuration")
	ErrForbidden = errors.New("match action forbidden")
	ErrState     = errors.New("match state conflict")
	ErrResult    = errors.New("conflicting match result")
)

type Assignment struct {
	ParticipantID string      `json:"participantId"`
	Room          Room        `json:"room"`
	Seat          bridge.Seat `json:"seat"`
}

type Board struct {
	ID       string               `json:"id"`
	Metadata bridge.BoardMetadata `json:"metadata"`
	Source   deal.Result          `json:"source"`
}

type Result struct {
	BoardID string `json:"boardId"`
	Room    Room   `json:"room"`
	ScoreNS int    `json:"scoreNS"`
}

type Snapshot struct {
	ID            string       `json:"id"`
	OwnerID       string       `json:"ownerId"`
	Status        Status       `json:"status"`
	OpenTableID   string       `json:"openTableId"`
	ClosedTableID string       `json:"closedTableId"`
	BoardIDs      []string     `json:"boardIds"`
	Assignments   []Assignment `json:"assignments"`
	Ready         []string     `json:"ready"`
	Boards        []Board      `json:"boards"`
	Results       []Result     `json:"results"`
}

type Match struct{ state Snapshot }

func New(id, ownerID, openTableID, closedTableID string, boardIDs []string) (*Match, error) {
	if id == "" || ownerID == "" || openTableID == "" || closedTableID == "" || openTableID == closedTableID || len(boardIDs) == 0 || len(boardIDs) > MaxBoards {
		return nil, ErrInvalid
	}
	seen := make(map[string]bool, len(boardIDs))
	for _, boardID := range boardIDs {
		if boardID == "" || seen[boardID] {
			return nil, ErrInvalid
		}
		seen[boardID] = true
	}
	return &Match{state: Snapshot{ID: id, OwnerID: ownerID, Status: Waiting, OpenTableID: openTableID, ClosedTableID: closedTableID, BoardIDs: slices.Clone(boardIDs)}}, nil
}

// Assign replaces a waiting lineup atomically. Team A is open NS/closed EW; Team B is its inverse.
func (match *Match) Assign(actorID string, assignments []Assignment) error {
	if actorID != match.state.OwnerID {
		return ErrForbidden
	}
	if match.state.Status != Waiting {
		return ErrState
	}
	if len(assignments) != 8 {
		return ErrInvalid
	}
	participants := make(map[string]bool, 8)
	positions := make(map[string]bool, 8)
	for _, assignment := range assignments {
		position := string(assignment.Room) + string(assignment.Seat)
		if assignment.ParticipantID == "" || participants[assignment.ParticipantID] || positions[position] || !assignment.Seat.Valid() || (assignment.Room != Open && assignment.Room != Closed) {
			return ErrInvalid
		}
		participants[assignment.ParticipantID] = true
		positions[position] = true
	}
	if !participants[match.state.OwnerID] {
		return ErrInvalid
	}
	match.state.Assignments = slices.Clone(assignments)
	match.state.Ready = nil
	return nil
}

func (match *Match) SetReady(actorID string, ready bool) error {
	if match.state.Status != Waiting {
		return ErrState
	}
	if !slices.ContainsFunc(match.state.Assignments, func(assignment Assignment) bool { return assignment.ParticipantID == actorID }) {
		return ErrForbidden
	}
	_index := slices.Index(match.state.Ready, actorID)
	if ready && _index < 0 {
		match.state.Ready = append(match.state.Ready, actorID)
	}
	if !ready && _index >= 0 {
		match.state.Ready = slices.Delete(match.state.Ready, _index, _index+1)
	}
	return nil
}

// Start generates the entire board set before changing state; callers serialize and commit the candidate before publishing.
func (match *Match) Start(ctx context.Context, actorID string, source deal.Source) error {
	if actorID != match.state.OwnerID {
		return ErrForbidden
	}
	if match.state.Status != Waiting || len(match.state.Ready) != 8 {
		return ErrState
	}
	if source == nil {
		return ErrInvalid
	}
	boards := make([]Board, 0, len(match.state.BoardIDs))
	for _index, boardID := range match.state.BoardIDs {
		if err := ctx.Err(); err != nil {
			return err
		}
		generated, err := source.Generate(ctx)
		if err != nil {
			return fmt.Errorf("generate match board: %w", err)
		}
		if err := generated.Validate(); err != nil {
			return err
		}
		metadata, err := bridge.MetadataForBoard(_index + 1)
		if err != nil {
			return err
		}
		cards := generated.Deal
		generated.Deal = bridge.Deal{North: cards.Hand(bridge.North), East: cards.Hand(bridge.East), South: cards.Hand(bridge.South), West: cards.Hand(bridge.West)}
		boards = append(boards, Board{ID: boardID, Metadata: metadata, Source: generated})
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	match.state.Boards = boards
	match.state.Status = Active
	return nil
}

// NextBoard returns a private, detached board for trusted table orchestration, never a client projection.
func (match *Match) NextBoard(room Room) (Board, bool) {
	if match.state.Status != Active || (room != Open && room != Closed) {
		return Board{}, false
	}
	for _, board := range match.state.Boards {
		if slices.ContainsFunc(match.state.Results, func(result Result) bool { return result.Room == room && result.BoardID == board.ID }) {
			continue
		}
		cards := board.Source.Deal
		board.Source.Deal = bridge.Deal{North: cards.Hand(bridge.North), East: cards.Hand(bridge.East), South: cards.Hand(bridge.South), West: cards.Hand(bridge.West)}
		return board, true
	}
	return Board{}, false
}

// RecordResult accepts only a trusted, finalized table result. Identical retries are no-ops, including after completion.
func (match *Match) RecordResult(tableID string, result Result) error {
	if (result.Room != Open && result.Room != Closed) || (result.Room == Open && tableID != match.state.OpenTableID) || (result.Room == Closed && tableID != match.state.ClosedTableID) {
		return ErrForbidden
	}
	if _, err := CompareScores(result.ScoreNS, 0); err != nil {
		return err
	}
	for _, existing := range match.state.Results {
		if existing.Room == result.Room && existing.BoardID == result.BoardID {
			if existing == result {
				return nil
			}
			return ErrResult
		}
	}
	board, available := match.NextBoard(result.Room)
	if !available || board.ID != result.BoardID {
		return ErrState
	}
	match.state.Results = append(match.state.Results, result)
	if len(match.state.Results) == 2*len(match.state.Boards) {
		match.state.Status = Complete
	}
	return nil
}

type Comparison struct {
	BoardID       string `json:"boardId"`
	OpenScoreNS   int    `json:"openScoreNS"`
	ClosedScoreNS int    `json:"closedScoreNS"`
	TeamAIMP      int    `json:"teamAIMP"`
}

// Comparisons derives totals from unique results instead of incrementing a retry-sensitive accumulator.
func (match *Match) Comparisons() ([]Comparison, int) {
	comparisons := make([]Comparison, 0, len(match.state.Boards))
	total := 0
	for _, board := range match.state.Boards {
		var openResult, closedResult *Result
		for _, result := range match.state.Results {
			if result.BoardID != board.ID {
				continue
			}
			if result.Room == Open {
				openResult = &result
			} else {
				closedResult = &result
			}
		}
		if openResult == nil || closedResult == nil {
			continue
		}
		imps, _ := CompareScores(openResult.ScoreNS, closedResult.ScoreNS)
		comparisons = append(comparisons, Comparison{BoardID: board.ID, OpenScoreNS: openResult.ScoreNS, ClosedScoreNS: closedResult.ScoreNS, TeamAIMP: imps})
		total += imps
	}
	return comparisons, total
}

// PrivateSnapshot is exclusively for persistence; it contains every hidden hand in the match.
func (match *Match) PrivateSnapshot() Snapshot {
	state := match.state
	state.BoardIDs = slices.Clone(state.BoardIDs)
	state.Assignments = slices.Clone(state.Assignments)
	state.Ready = slices.Clone(state.Ready)
	state.Results = slices.Clone(state.Results)
	state.Boards = slices.Clone(state.Boards)
	for _index := range state.Boards {
		cards := state.Boards[_index].Source.Deal
		state.Boards[_index].Source.Deal = bridge.Deal{North: cards.Hand(bridge.North), East: cards.Hand(bridge.East), South: cards.Hand(bridge.South), West: cards.Hand(bridge.West)}
	}
	return state
}

// Restore validates private persisted state by replaying its domain transitions without regenerating deals.
func Restore(state Snapshot) (*Match, error) {
	match, err := New(state.ID, state.OwnerID, state.OpenTableID, state.ClosedTableID, state.BoardIDs)
	if err != nil {
		return nil, err
	}
	if len(state.Assignments) != 0 {
		if err := match.Assign(state.OwnerID, state.Assignments); err != nil {
			return nil, err
		}
	}
	seenReady := make(map[string]bool)
	for _, participantID := range state.Ready {
		if seenReady[participantID] {
			return nil, ErrInvalid
		}
		seenReady[participantID] = true
		if err := match.SetReady(participantID, true); err != nil {
			return nil, err
		}
	}
	if state.Status == Waiting || state.Status == Cancelled {
		if len(state.Boards) != 0 || len(state.Results) != 0 {
			return nil, ErrInvalid
		}
		match.state.Status = state.Status
		return match, nil
	}
	if (state.Status != Active && state.Status != Complete) || len(state.Ready) != 8 || len(state.Boards) != len(state.BoardIDs) {
		return nil, ErrInvalid
	}
	for _index, board := range state.Boards {
		metadata, err := bridge.MetadataForBoard(_index + 1)
		if err != nil {
			return nil, err
		}
		if board.ID != state.BoardIDs[_index] || board.Metadata != metadata {
			return nil, ErrInvalid
		}
		if err := board.Source.Validate(); err != nil {
			return nil, err
		}
	}
	match.state.Boards = state.Boards
	match.state.Status = Active
	for _, result := range state.Results {
		tableID := state.OpenTableID
		if result.Room == Closed {
			tableID = state.ClosedTableID
		}
		before := len(match.state.Results)
		if err := match.RecordResult(tableID, result); err != nil {
			return nil, err
		}
		if len(match.state.Results) == before {
			return nil, ErrInvalid
		}
	}
	if match.state.Status != state.Status {
		return nil, ErrInvalid
	}
	match.state = match.PrivateSnapshot()
	return match, nil
}

type View struct {
	ID              string       `json:"id"`
	Status          Status       `json:"status"`
	OwnerID         string       `json:"ownerId"`
	TableID         string       `json:"tableId"`
	Assignment      Assignment   `json:"assignment"`
	Team            string       `json:"team"`
	BoardCount      int          `json:"boardCount"`
	ReadyCount      int          `json:"readyCount"`
	OpenCompleted   int          `json:"openCompleted"`
	ClosedCompleted int          `json:"closedCompleted"`
	Comparisons     []Comparison `json:"comparisons,omitempty"`
	TeamAIMP        int          `json:"teamAIMP"`
}

// Project exposes only the viewer's room assignment and aggregate progress until the entire match completes.
func (match *Match) Project(participantID string) (View, error) {
	_index := slices.IndexFunc(match.state.Assignments, func(assignment Assignment) bool { return assignment.ParticipantID == participantID })
	if _index < 0 {
		return View{}, ErrForbidden
	}
	assignment := match.state.Assignments[_index]
	view := View{ID: match.state.ID, Status: match.state.Status, OwnerID: match.state.OwnerID, Assignment: assignment, BoardCount: len(match.state.BoardIDs), ReadyCount: len(match.state.Ready), TableID: match.state.OpenTableID, Team: "B"}
	if assignment.Room == Closed {
		view.TableID = match.state.ClosedTableID
	}
	if (assignment.Room == Open && assignment.Seat.Partnership() == bridge.NorthSouth) || (assignment.Room == Closed && assignment.Seat.Partnership() == bridge.EastWest) {
		view.Team = "A"
	}
	for _, result := range match.state.Results {
		if result.Room == Open {
			view.OpenCompleted++
		} else {
			view.ClosedCompleted++
		}
	}
	if match.state.Status == Complete {
		view.Comparisons, view.TeamAIMP = match.Comparisons()
	}
	return view, nil
}

// Cancel lets any assigned participant decline a waiting match without changing active game results.
func (match *Match) Cancel(actorID string) error {
	if !slices.ContainsFunc(match.state.Assignments, func(assignment Assignment) bool { return assignment.ParticipantID == actorID }) {
		return ErrForbidden
	}
	if match.state.Status == Cancelled {
		return nil
	}
	if match.state.Status != Waiting {
		return ErrState
	}
	match.state.Status = Cancelled
	return nil
}
