package match

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/bridge"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/table"
)

type serviceRepository struct {
	stored  Stored
	failure error
}

func (repository *serviceRepository) CreateMatchLobby(context.Context, string, CreateRequest, time.Time) (Stored, error) {
	return repository.stored, repository.failure
}
func (repository *serviceRepository) LoadMatch(context.Context, string) (Stored, error) {
	return repository.stored, repository.failure
}
func (repository *serviceRepository) ListMatchIDs(context.Context, string) ([]string, error) {
	return []string{repository.stored.Match.PrivateSnapshot().ID}, repository.failure
}
func (repository *serviceRepository) StartMatch(ctx context.Context, _, actor string, revision int64, _ time.Time) (Started, error) {
	if repository.failure != nil {
		return Started{}, repository.failure
	}
	if actor != repository.stored.Match.PrivateSnapshot().OwnerID {
		return Started{}, ErrForbidden
	}
	if repository.stored.Match.PrivateSnapshot().Status == Active {
		return Started{Stored: repository.stored, Duplicate: true}, nil
	}
	if repository.stored.Revision != revision {
		return Started{}, ErrState
	}
	if err := repository.stored.Match.Start(ctx, actor, &testSource{}); err != nil {
		return Started{}, err
	}
	repository.stored.Revision++
	return Started{Stored: repository.stored}, nil
}
func (repository *serviceRepository) CancelMatch(_ context.Context, _, actor string, _ int64, _ time.Time) (Stored, error) {
	if repository.failure != nil {
		return Stored{}, repository.failure
	}
	err := repository.stored.Match.Cancel(actor)
	return repository.stored, err
}
func (repository *serviceRepository) FindTable(context.Context, string) (table.Aggregate, error) {
	return table.Aggregate{Revision: 2, Seats: map[bridge.Seat]table.SeatAssignment{bridge.North: {Ready: true}}}, repository.failure
}

type serviceRuntime struct {
	request table.CommandRequest
	err     error
	reject  bool
}

func (runtime *serviceRuntime) Submit(_ context.Context, request table.CommandRequest) (table.CommandResult, error) {
	runtime.request = request
	status := table.CommandStatusAccepted
	if runtime.reject {
		status = table.CommandStatusRejected
	}
	return table.CommandResult{Outcome: table.CommandOutcome{Status: status}}, runtime.err
}

type serviceNotifier struct {
	err      error
	ctxError error
	tables   []string
	calls    int
}

func (notifier *serviceNotifier) RefreshTables(ctx context.Context, tables []string) error {
	notifier.ctxError = ctx.Err()
	notifier.tables = append(notifier.tables, tables...)
	notifier.calls++
	return notifier.err
}

func newServiceTest(t *testing.T) (*Service, *serviceRepository, *serviceRuntime, *serviceNotifier) {
	t.Helper()
	state := waitingMatch(t).PrivateSnapshot()
	state.ID = uuid.NewString()
	candidate, err := Restore(state)
	if err != nil {
		t.Fatal(err)
	}
	repository := &serviceRepository{stored: Stored{Match: candidate}}
	runtime := &serviceRuntime{}
	notifier := &serviceNotifier{}
	service, err := NewService(repository, runtime, notifier, slog.New(slog.NewJSONHandler(io.Discard, nil)), time.Now)
	if err != nil {
		t.Fatal(err)
	}
	return service, repository, runtime, notifier
}

func TestCreateRequestValidate(t *testing.T) {
	t.Parallel()
	request := CreateRequest{RequestID: "create_01", BoardCount: 2}
	for _, room := range []Room{Open, Closed} {
		for _, seat := range []bridge.Seat{bridge.North, bridge.East, bridge.South, bridge.West} {
			request.Assignments = append(request.Assignments, SeatRequest{UserID: uuid.NewString(), Room: room, Seat: seat})
		}
	}
	if err := request.Validate(); err != nil {
		t.Fatal(err)
	}
	changes := []func(*CreateRequest){func(r *CreateRequest) { r.RequestID = "short" }, func(r *CreateRequest) { r.BoardCount = 0 }, func(r *CreateRequest) { r.BoardCount = 33 }, func(r *CreateRequest) { r.Assignments = r.Assignments[:7] }, func(r *CreateRequest) { r.Assignments[1].UserID = r.Assignments[0].UserID }, func(r *CreateRequest) { r.Assignments[0].Seat = "bad" }, func(r *CreateRequest) { r.Assignments[0].Room = "bad" }, func(r *CreateRequest) { r.Assignments[0].UserID = "bad" }, func(r *CreateRequest) { r.Assignments[1].Seat = r.Assignments[0].Seat }}
	for _, change := range changes {
		copy := request
		copy.Assignments = append([]SeatRequest(nil), request.Assignments...)
		change(&copy)
		if err := copy.Validate(); !errors.Is(err, ErrInvalid) {
			t.Fatal("accepted invalid input", err)
		}
	}
}

func TestServiceAuthorizationAndRecovery(t *testing.T) {
	t.Parallel()
	service, repository, runtime, notifier := newServiceTest(t)
	id := repository.stored.Match.PrivateSnapshot().ID
	if _, err := service.Get(t.Context(), "bad", "p0"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := service.Get(t.Context(), id, "outsider"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := service.Start(t.Context(), id, "p1", 0); !errors.Is(err, ErrForbidden) {
		t.Fatal(err)
	}
	if _, err := service.Start(t.Context(), id, "p0", 3); !errors.Is(err, ErrState) {
		t.Fatal(err)
	}
	if _, err := service.Cancel(t.Context(), id, "outsider", 0); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	runtime.reject = true
	if _, err := service.Ready(t.Context(), id, "p0", "ready_01", 2, true); !errors.Is(err, ErrState) {
		t.Fatal(err)
	}
	runtime.reject = false
	if _, err := service.Ready(t.Context(), id, "p0", "ready_01", 2, true); err != nil {
		t.Fatal(err)
	}
	if runtime.request.SessionID != "p0" || runtime.request.TableID != "open" || runtime.request.ExpectedRevision != 2 || !runtime.request.Command.Ready {
		t.Fatal("wrong actor dispatch")
	}
	notifier.err = errors.New("refresh failed")
	started, err := service.Start(t.Context(), id, "p0", 0)
	if err != nil || started.Status != Active || !started.SyncPending {
		t.Fatal("committed start lost on notification failure", err)
	}
	notifier.err = nil
	retried, err := service.Start(t.Context(), id, "p0", 0)
	if err != nil || retried.SyncPending || retried.Revision != started.Revision {
		t.Fatal("retry failed to reconcile", err)
	}
	if len(notifier.tables) != 6 {
		t.Fatal("did not refresh both rooms")
	}
	encoded, err := json.Marshal(retried)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"ownerId", "participantId", "sessionId", "deal", "source", "p0", "p1"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("private field %s exposed", forbidden)
		}
	}
	cancelledCtx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := service.afterCommit(cancelledCtx, repository.stored, "p0"); err != nil || notifier.ctxError != nil {
		t.Fatal("request cancellation prevented recovery", err)
	}
	views, err := service.List(t.Context(), "p0")
	if err != nil || len(views) != 1 {
		t.Fatal(err)
	}
}
