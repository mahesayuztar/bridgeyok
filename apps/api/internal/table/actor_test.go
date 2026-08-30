package table

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/mahesayuztar/bridgeyok/apps/api/internal/observability"
)

const (
	actorTableID   = "4ad4caf7-963e-4989-86bd-2a0cb276010f"
	actorSessionID = "65739950-ebaa-4b23-8657-e01819359675"
)

type actorHydrator struct {
	mutex     sync.Mutex
	aggregate Aggregate
	err       error
	calls     int
}

func (hydrator *actorHydrator) FindTable(context.Context, string) (Aggregate, error) {
	hydrator.mutex.Lock()
	defer hydrator.mutex.Unlock()
	hydrator.calls++
	return hydrator.aggregate.clone(), hydrator.err
}

func (hydrator *actorHydrator) callCount() int {
	hydrator.mutex.Lock()
	defer hydrator.mutex.Unlock()
	return hydrator.calls
}

type actorCommandHandler struct {
	mutex         sync.Mutex
	aggregate     Aggregate
	wait          <-chan struct{}
	started       chan CommandRequest
	requestIDs    []string
	active        int
	maximumActive int
	err           error
}

func (handler *actorCommandHandler) Process(_ context.Context, request CommandRequest) (CommandResult, error) {
	handler.mutex.Lock()
	handler.active++
	if handler.active > handler.maximumActive {
		handler.maximumActive = handler.active
	}
	handler.requestIDs = append(handler.requestIDs, request.RequestID)
	handler.mutex.Unlock()
	if handler.started != nil {
		handler.started <- request
	}
	if handler.wait != nil {
		<-handler.wait
	}
	handler.mutex.Lock()
	handler.active--
	aggregate := handler.aggregate.clone()
	handler.mutex.Unlock()
	aggregate.Revision = request.ExpectedRevision + 1
	aggregate.LastSeq = request.ExpectedRevision + 1
	return CommandResult{
		Aggregate: aggregate,
		Outcome: CommandOutcome{
			RequestID:   request.RequestID,
			CommandName: request.Command.Name,
			Status:      CommandStatusAccepted,
			Revision:    aggregate.Revision,
			FirstSeq:    aggregate.LastSeq,
			LastSeq:     aggregate.LastSeq,
		},
	}, handler.err
}

func (handler *actorCommandHandler) execution() ([]string, int) {
	handler.mutex.Lock()
	defer handler.mutex.Unlock()
	return append([]string(nil), handler.requestIDs...), handler.maximumActive
}

func TestNewActorRegistry(t *testing.T) {
	t.Parallel()
	aggregate := actorAggregate(t)
	hydrator := &actorHydrator{aggregate: aggregate}
	handler := &actorCommandHandler{aggregate: aggregate}
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	valid := ActorRegistryOptions{QueueCapacity: 1, IdleTimeout: time.Minute, Logger: logger, Now: time.Now}

	tests := []struct {
		name     string
		hydrator AggregateHydrator
		handler  CommandHandler
		options  ActorRegistryOptions
	}{
		{name: "missing hydrator", handler: handler, options: valid},
		{name: "missing handler", hydrator: hydrator, options: valid},
		{name: "invalid queue capacity", hydrator: hydrator, handler: handler, options: ActorRegistryOptions{IdleTimeout: time.Minute, Logger: logger, Now: time.Now}},
		{name: "invalid idle timeout", hydrator: hydrator, handler: handler, options: ActorRegistryOptions{QueueCapacity: 1, Logger: logger, Now: time.Now}},
		{name: "missing logger", hydrator: hydrator, handler: handler, options: ActorRegistryOptions{QueueCapacity: 1, IdleTimeout: time.Minute, Now: time.Now}},
		{name: "missing clock", hydrator: hydrator, handler: handler, options: ActorRegistryOptions{QueueCapacity: 1, IdleTimeout: time.Minute, Logger: logger}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, err := NewActorRegistry(test.hydrator, test.handler, test.options); err == nil {
				t.Fatal("NewActorRegistry() error = nil")
			}
		})
	}
}

func TestActorRegistrySerializesCommandsAndBoundsQueue(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		aggregate := actorAggregate(t)
		release := make(chan struct{})
		hydrator := &actorHydrator{aggregate: aggregate}
		handler := &actorCommandHandler{aggregate: aggregate, wait: release, started: make(chan CommandRequest, 2)}
		logs := &bytes.Buffer{}
		registry := actorRegistryForTest(t, hydrator, handler, 1, time.Hour, observability.NewLoggerWithWriter(slog.LevelDebug, logs))

		firstResult := make(chan error, 1)
		go func() {
			_, err := registry.Submit(t.Context(), actorCommand("request-1", 0))
			firstResult <- err
		}()
		<-handler.started

		secondResult := make(chan error, 1)
		go func() {
			_, err := registry.Submit(t.Context(), actorCommand("request-2", 1))
			secondResult <- err
		}()
		synctest.Wait()

		if _, err := registry.Submit(t.Context(), actorCommand("request-3", 2)); !errors.Is(err, ErrActorQueueFull) {
			t.Fatalf("third Submit() error = %v, want ErrActorQueueFull", err)
		}
		close(release)
		if err := <-firstResult; err != nil {
			t.Fatalf("first Submit() error = %v", err)
		}
		if err := <-secondResult; err != nil {
			t.Fatalf("second Submit() error = %v", err)
		}
		requestIDs, maximumActive := handler.execution()
		if strings.Join(requestIDs, ",") != "request-1,request-2" || maximumActive != 1 {
			t.Fatalf("execution = %v with maximum active %d, want serial first and second commands", requestIDs, maximumActive)
		}
		if hydrator.callCount() != 1 {
			t.Fatalf("FindTable() calls = %d, want 1", hydrator.callCount())
		}
		if !strings.Contains(logs.String(), `"msg":"table_actor_queue_full"`) {
			t.Fatalf("logs = %q, want queue saturation event", logs.String())
		}
		if strings.Contains(logs.String(), actorSessionID) || strings.Contains(logs.String(), "Owner") {
			t.Fatalf("actor lifecycle logs exposed session or aggregate data: %q", logs.String())
		}
		if err := registry.Drain(t.Context()); err != nil {
			t.Fatalf("Drain() error = %v", err)
		}
	})
}

func TestActorRegistryKeepsCommittedStateWhenHandlerReturnsPublishError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		aggregate := actorAggregate(t)
		publishError := errors.New("publisher unavailable")
		hydrator := &actorHydrator{aggregate: aggregate}
		handler := &actorCommandHandler{aggregate: aggregate, err: publishError}
		registry := actorRegistryForTest(t, hydrator, handler, 2, time.Hour, nil)

		result, err := registry.Submit(t.Context(), actorCommand("request-1", 0))
		if !errors.Is(err, publishError) {
			t.Fatalf("Submit() error = %v, want publish error", err)
		}
		current, snapshotErr := registry.Snapshot(t.Context(), actorTableID)
		if snapshotErr != nil {
			t.Fatalf("Snapshot() error = %v", snapshotErr)
		}
		if current.Revision != result.Aggregate.Revision {
			t.Fatalf("Snapshot() revision = %d, want committed revision %d", current.Revision, result.Aggregate.Revision)
		}
		if drainErr := registry.Drain(t.Context()); drainErr != nil {
			t.Fatalf("Drain() error = %v", drainErr)
		}
	})
}

func TestActorRegistryHydratesLazilyAndOwnsSnapshot(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		aggregate := actorAggregate(t)
		hydrator := &actorHydrator{aggregate: aggregate}
		handler := &actorCommandHandler{aggregate: aggregate}
		registry := actorRegistryForTest(t, hydrator, handler, 4, time.Hour, nil)

		if stats := registry.Stats(); stats.ActiveActors != 0 || hydrator.callCount() != 0 {
			t.Fatalf("initial state = %+v with %d hydrations, want empty and lazy", stats, hydrator.callCount())
		}
		first, err := registry.Snapshot(t.Context(), actorTableID)
		if err != nil {
			t.Fatalf("Snapshot() error = %v", err)
		}
		first.Participants[0].Nickname = "mutated outside actor"
		second, err := registry.Snapshot(t.Context(), actorTableID)
		if err != nil {
			t.Fatalf("second Snapshot() error = %v", err)
		}
		if second.Participants[0].Nickname != "Owner" {
			t.Fatalf("second Snapshot() nickname = %q, want defensive actor state", second.Participants[0].Nickname)
		}
		result, err := registry.Submit(t.Context(), actorCommand("request-1", 0))
		if err != nil {
			t.Fatalf("Submit() error = %v", err)
		}
		current, err := registry.Snapshot(t.Context(), actorTableID)
		if err != nil {
			t.Fatalf("current Snapshot() error = %v", err)
		}
		if current.Revision != result.Aggregate.Revision || hydrator.callCount() != 1 {
			t.Fatalf("current revision = %d with %d hydrations, want revision %d and one hydration", current.Revision, hydrator.callCount(), result.Aggregate.Revision)
		}
		if err := registry.Drain(t.Context()); err != nil {
			t.Fatalf("Drain() error = %v", err)
		}
	})
}

func TestActorRegistryRetriesFailedHydrate(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		aggregate := actorAggregate(t)
		hydrateError := errors.New("database unavailable")
		hydrator := &actorHydrator{aggregate: aggregate, err: hydrateError}
		handler := &actorCommandHandler{aggregate: aggregate}
		registry := actorRegistryForTest(t, hydrator, handler, 2, time.Hour, nil)

		if _, err := registry.Snapshot(t.Context(), actorTableID); !errors.Is(err, hydrateError) {
			t.Fatalf("Snapshot() error = %v, want hydrate error", err)
		}
		hydrator.mutex.Lock()
		hydrator.err = nil
		hydrator.mutex.Unlock()
		if _, err := registry.Snapshot(t.Context(), actorTableID); err != nil {
			t.Fatalf("Snapshot() retry error = %v", err)
		}
		if hydrator.callCount() != 2 {
			t.Fatalf("FindTable() calls = %d, want failed hydrate retried", hydrator.callCount())
		}
		if err := registry.Drain(t.Context()); err != nil {
			t.Fatalf("Drain() error = %v", err)
		}
	})
}

func TestActorRegistryEvictsIdleActorAndRehydrates(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		aggregate := actorAggregate(t)
		hydrator := &actorHydrator{aggregate: aggregate}
		handler := &actorCommandHandler{aggregate: aggregate}
		registry := actorRegistryForTest(t, hydrator, handler, 2, time.Minute, nil)

		if _, err := registry.Snapshot(t.Context(), actorTableID); err != nil {
			t.Fatalf("Snapshot() error = %v", err)
		}
		time.Sleep(time.Minute)
		synctest.Wait()
		if stats := registry.Stats(); stats.ActiveActors != 0 {
			t.Fatalf("Stats() = %+v, want idle actor evicted", stats)
		}
		if _, err := registry.Snapshot(t.Context(), actorTableID); err != nil {
			t.Fatalf("Snapshot() after eviction error = %v", err)
		}
		if hydrator.callCount() != 2 {
			t.Fatalf("FindTable() calls = %d, want rehydrate after eviction", hydrator.callCount())
		}
		if err := registry.Drain(t.Context()); err != nil {
			t.Fatalf("Drain() error = %v", err)
		}
	})
}

func TestActorRegistryDrainRejectsNewWorkAndFinishesQueue(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		aggregate := actorAggregate(t)
		release := make(chan struct{})
		hydrator := &actorHydrator{aggregate: aggregate}
		handler := &actorCommandHandler{aggregate: aggregate, wait: release, started: make(chan CommandRequest, 2)}
		registry := actorRegistryForTest(t, hydrator, handler, 2, time.Hour, nil)

		firstResult := make(chan error, 1)
		go func() {
			_, err := registry.Submit(t.Context(), actorCommand("request-1", 0))
			firstResult <- err
		}()
		<-handler.started
		secondResult := make(chan error, 1)
		go func() {
			_, err := registry.Submit(t.Context(), actorCommand("request-2", 1))
			secondResult <- err
		}()
		synctest.Wait()

		drainResult := make(chan error, 1)
		go func() {
			drainResult <- registry.Drain(t.Context())
		}()
		synctest.Wait()
		if _, err := registry.Submit(t.Context(), actorCommand("request-3", 2)); !errors.Is(err, ErrActorRegistryDraining) {
			t.Fatalf("Submit() during drain error = %v, want ErrActorRegistryDraining", err)
		}
		close(release)
		if err := <-firstResult; err != nil {
			t.Fatalf("first Submit() error = %v", err)
		}
		if err := <-secondResult; err != nil {
			t.Fatalf("second Submit() error = %v", err)
		}
		if err := <-drainResult; err != nil {
			t.Fatalf("Drain() error = %v", err)
		}
		if stats := registry.Stats(); stats.ActiveActors != 0 || !stats.Draining {
			t.Fatalf("Stats() = %+v, want empty draining registry", stats)
		}
	})
}

func actorRegistryForTest(t *testing.T, hydrator AggregateHydrator, handler CommandHandler, queueCapacity int, idleTimeout time.Duration, logger *slog.Logger) *ActorRegistry {
	t.Helper()
	if logger == nil {
		logger = slog.New(slog.NewJSONHandler(io.Discard, nil))
	}
	registry, err := NewActorRegistry(hydrator, handler, ActorRegistryOptions{
		QueueCapacity: queueCapacity,
		IdleTimeout:   idleTimeout,
		Logger:        logger,
		Now:           time.Now,
	})
	if err != nil {
		t.Fatalf("NewActorRegistry() error = %v", err)
	}
	return registry
}

func actorAggregate(t *testing.T) Aggregate {
	t.Helper()
	aggregate, err := NewAggregate(actorTableID, Participant{
		ID:        "fc723b96-bbf6-4de9-9cef-6c821e0f5b3f",
		SessionID: actorSessionID,
		Nickname:  "Owner",
		Role:      RoleOwner,
		JoinedAt:  time.Date(2026, time.August, 30, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("NewAggregate() error = %v", err)
	}
	return aggregate
}

func actorCommand(requestID string, expectedRevision int64) CommandRequest {
	return CommandRequest{
		TableID:          actorTableID,
		SessionID:        actorSessionID,
		RequestID:        requestID,
		ExpectedRevision: expectedRevision,
		Command:          Command{Name: CommandLockTable, Locked: true},
	}
}
