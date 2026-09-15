package match

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/bridge"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/table"
)

var ErrNotFound = errors.New("match not found")
var ErrCapacity = errors.New("match invitation capacity reached")
var requestIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{8,64}$`)

type SeatRequest struct {
	UserID string      `json:"userId"`
	Room   Room        `json:"room"`
	Seat   bridge.Seat `json:"seat"`
}

type CreateRequest struct {
	RequestID   string        `json:"requestId"`
	BoardCount  int           `json:"boardCount"`
	Assignments []SeatRequest `json:"assignments"`
}

type Stored struct {
	Match    *Match
	Revision int64
}
type Started struct {
	Stored
	Tables    []table.CommandResult
	Duplicate bool
}

type PublicView struct {
	ID              string       `json:"id"`
	Status          Status       `json:"status"`
	Revision        int64        `json:"revision"`
	IsOwner         bool         `json:"isOwner"`
	TableID         string       `json:"tableId"`
	TableRevision   int64        `json:"tableRevision"`
	Room            Room         `json:"room"`
	Seat            bridge.Seat  `json:"seat"`
	Team            string       `json:"team"`
	BoardCount      int          `json:"boardCount"`
	Ready           bool         `json:"ready"`
	ReadyCount      int          `json:"readyCount"`
	OpenCompleted   int          `json:"openCompleted"`
	ClosedCompleted int          `json:"closedCompleted"`
	CanStart        bool         `json:"canStart"`
	CanCancel       bool         `json:"canCancel"`
	SyncPending     bool         `json:"syncPending"`
	Comparisons     []Comparison `json:"comparisons,omitempty"`
	TeamAIMP        int          `json:"teamAIMP"`
}

type Repository interface {
	CreateMatchLobby(context.Context, string, CreateRequest, time.Time) (Stored, error)
	LoadMatch(context.Context, string) (Stored, error)
	ListMatchIDs(context.Context, string) ([]string, error)
	StartMatch(context.Context, string, string, int64, time.Time) (Started, error)
	CancelMatch(context.Context, string, string, int64, time.Time) (Stored, error)
	FindTable(context.Context, string) (table.Aggregate, error)
}

type Runtime interface {
	Submit(context.Context, table.CommandRequest) (table.CommandResult, error)
}
type Notifier interface {
	RefreshTables(context.Context, []string) error
}

type Service struct {
	repository Repository
	runtime    Runtime
	notifier   Notifier
	logger     *slog.Logger
	now        func() time.Time
}

func NewService(repository Repository, runtime Runtime, notifier Notifier, logger *slog.Logger, now func() time.Time) (*Service, error) {
	if repository == nil || runtime == nil || notifier == nil || logger == nil || now == nil {
		return nil, fmt.Errorf("match service dependencies required")
	}
	return &Service{repository: repository, runtime: runtime, notifier: notifier, logger: logger, now: now}, nil
}

func (request CreateRequest) Validate() error {
	if !requestIDPattern.MatchString(request.RequestID) || request.BoardCount < 1 || request.BoardCount > MaxBoards || len(request.Assignments) != 8 {
		return ErrInvalid
	}
	users := map[string]bool{}
	positions := map[string]bool{}
	for _, assignment := range request.Assignments {
		if parsed, err := uuid.Parse(assignment.UserID); err != nil || parsed.String() != assignment.UserID {
			return ErrInvalid
		}
		position := string(assignment.Room) + string(assignment.Seat)
		if users[assignment.UserID] || positions[position] || !assignment.Seat.Valid() || (assignment.Room != Open && assignment.Room != Closed) {
			return ErrInvalid
		}
		users[assignment.UserID] = true
		positions[position] = true
	}
	return nil
}

func (service *Service) Create(ctx context.Context, sessionID string, request CreateRequest) (PublicView, error) {
	if err := request.Validate(); err != nil {
		return PublicView{}, err
	}
	stored, err := service.repository.CreateMatchLobby(ctx, sessionID, request, service.now().UTC())
	if err != nil {
		return PublicView{}, err
	}
	return service.project(ctx, stored, sessionID)
}

func (service *Service) Get(ctx context.Context, matchID, sessionID string) (PublicView, error) {
	if _, err := uuid.Parse(matchID); err != nil {
		return PublicView{}, ErrNotFound
	}
	stored, err := service.repository.LoadMatch(ctx, matchID)
	if err != nil {
		return PublicView{}, err
	}
	return service.project(ctx, stored, sessionID)
}

func (service *Service) List(ctx context.Context, sessionID string) ([]PublicView, error) {
	ids, err := service.repository.ListMatchIDs(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	views := make([]PublicView, 0, len(ids))
	for _, id := range ids {
		view, err := service.Get(ctx, id, sessionID)
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	return views, nil
}

func (service *Service) Start(ctx context.Context, matchID, sessionID string, revision int64) (PublicView, error) {
	if _, err := uuid.Parse(matchID); err != nil {
		return PublicView{}, ErrNotFound
	}
	if revision < 0 {
		return PublicView{}, ErrInvalid
	}
	if _, err := service.Get(ctx, matchID, sessionID); err != nil {
		return PublicView{}, err
	}
	started, err := service.repository.StartMatch(ctx, matchID, sessionID, revision, service.now().UTC())
	if err != nil {
		return PublicView{}, err
	}
	return service.afterCommit(ctx, started.Stored, sessionID)
}

func (service *Service) Ready(ctx context.Context, matchID, sessionID, requestID string, tableRevision int64, ready bool) (PublicView, error) {
	if !requestIDPattern.MatchString(requestID) || tableRevision < 0 {
		return PublicView{}, ErrInvalid
	}
	view, err := service.Get(ctx, matchID, sessionID)
	if err != nil {
		return PublicView{}, err
	}
	result, err := service.runtime.Submit(ctx, table.CommandRequest{TableID: view.TableID, SessionID: sessionID, RequestID: requestID, ExpectedRevision: tableRevision, Command: table.Command{Name: table.CommandSetReady, Ready: ready}})
	if err != nil {
		return PublicView{}, err
	}
	if result.Outcome.Status != table.CommandStatusAccepted {
		return PublicView{}, ErrState
	}
	stored, err := service.repository.LoadMatch(ctx, matchID)
	if err != nil {
		return PublicView{}, err
	}
	return service.afterCommit(ctx, stored, sessionID)
}

func (service *Service) Cancel(ctx context.Context, matchID, sessionID string, revision int64) (PublicView, error) {
	if _, err := uuid.Parse(matchID); err != nil {
		return PublicView{}, ErrNotFound
	}
	if revision < 0 {
		return PublicView{}, ErrInvalid
	}
	if _, err := service.Get(ctx, matchID, sessionID); err != nil {
		return PublicView{}, err
	}
	stored, err := service.repository.CancelMatch(ctx, matchID, sessionID, revision, service.now().UTC())
	if err != nil {
		return PublicView{}, err
	}
	return service.afterCommit(ctx, stored, sessionID)
}

// afterCommit attempts both room refreshes even when the HTTP caller disconnected; failure leaves durable state recoverable by retry.
func (service *Service) afterCommit(ctx context.Context, stored Stored, sessionID string) (PublicView, error) {
	notifyCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	state := stored.Match.PrivateSnapshot()
	view, viewErr := service.project(notifyCtx, stored, sessionID)
	if viewErr != nil {
		return PublicView{}, viewErr
	}
	err := service.notifier.RefreshTables(notifyCtx, []string{state.OpenTableID, state.ClosedTableID})
	if err != nil {
		service.logger.WarnContext(notifyCtx, "match_refresh_pending", "match_id", state.ID, "revision", stored.Revision, "result_code", "DELIVERY_PENDING")
	}
	view.SyncPending = err != nil
	return view, nil
}

func (service *Service) project(ctx context.Context, stored Stored, sessionID string) (PublicView, error) {
	view, err := stored.Match.Project(sessionID)
	if err != nil {
		return PublicView{}, ErrNotFound
	}
	aggregate, err := service.repository.FindTable(ctx, view.TableID)
	if err != nil {
		return PublicView{}, err
	}
	ownReady := aggregate.Seats[view.Assignment.Seat].Ready
	isOwner := view.OwnerID == sessionID
	return PublicView{ID: view.ID, Status: view.Status, Revision: stored.Revision, IsOwner: isOwner, TableID: view.TableID, TableRevision: aggregate.Revision, Room: view.Assignment.Room, Seat: view.Assignment.Seat, Team: view.Team, BoardCount: view.BoardCount, Ready: ownReady, ReadyCount: view.ReadyCount, OpenCompleted: view.OpenCompleted, ClosedCompleted: view.ClosedCompleted, CanStart: view.Status == Waiting && view.ReadyCount == 8 && isOwner, CanCancel: view.Status == Waiting, Comparisons: view.Comparisons, TeamAIMP: view.TeamAIMP}, nil
}
