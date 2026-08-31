package table

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

var (
	ErrActorQueueFull        = errors.New("table actor queue is full")
	ErrActorStopped          = errors.New("table actor has stopped")
	ErrActorRegistryDraining = errors.New("table actor registry is draining")
)

const maxConsecutiveBotActions = 64

// AggregateHydrator loads the durable private state owned by a table actor.
type AggregateHydrator interface {
	FindTable(context.Context, string) (Aggregate, error)
}

// CommandHandler processes one command through the durable application boundary.
type CommandHandler interface {
	Process(context.Context, CommandRequest) (CommandResult, error)
}

// ActorRegistryOptions controls local queue capacity and idle eviction.
type ActorRegistryOptions struct {
	QueueCapacity int
	IdleTimeout   time.Duration
	Logger        *slog.Logger
	Now           func() time.Time
}

// ActorRegistryStats describes the process-local actor lifecycle.
type ActorRegistryStats struct {
	ActiveActors int
	Draining     bool
}

// ActorRegistry owns at most one local actor for each table ID.
type ActorRegistry struct {
	hydrator AggregateHydrator
	handler  CommandHandler
	options  ActorRegistryOptions

	mutex    sync.Mutex
	actors   map[string]*tableActor
	draining bool
	wait     sync.WaitGroup
}

type actorRequest struct {
	ctx      context.Context
	command  *CommandRequest
	refresh  bool
	response chan actorResponse
}

type actorResponse struct {
	commandResult CommandResult
	aggregate     Aggregate
	err           error
}

type tableActor struct {
	tableID        string
	hydrator       AggregateHydrator
	handler        CommandHandler
	queue          chan actorRequest
	idle           time.Duration
	logger         *slog.Logger
	now            func() time.Time
	onStop         func(*tableActor)
	startedAt      time.Time
	lifecycleMutex sync.Mutex
	accepting      bool
}

// NewActorRegistry creates a process-local table actor registry.
func NewActorRegistry(hydrator AggregateHydrator, handler CommandHandler, options ActorRegistryOptions) (*ActorRegistry, error) {
	if hydrator == nil {
		return nil, fmt.Errorf("actor hydrator is required")
	}
	if handler == nil {
		return nil, fmt.Errorf("actor command handler is required")
	}
	if options.QueueCapacity < 1 {
		return nil, fmt.Errorf("actor queue capacity must be positive")
	}
	if options.IdleTimeout <= 0 {
		return nil, fmt.Errorf("actor idle timeout must be positive")
	}
	if options.Logger == nil {
		return nil, fmt.Errorf("actor logger is required")
	}
	if options.Now == nil {
		return nil, fmt.Errorf("actor clock is required")
	}
	return &ActorRegistry{
		hydrator: hydrator,
		handler:  handler,
		options:  options,
		actors:   make(map[string]*tableActor),
	}, nil
}

// Submit serializes a durable command through the table's local actor.
func (registry *ActorRegistry) Submit(ctx context.Context, request CommandRequest) (CommandResult, error) {
	if err := validateCommandRequest(request); err != nil {
		return CommandResult{}, err
	}
	for _attempt := 0; _attempt < 2; _attempt++ {
		actor, err := registry.actor(request.TableID)
		if err != nil {
			return CommandResult{}, err
		}
		result, err := actor.submit(ctx, request)
		if !errors.Is(err, ErrActorStopped) {
			return result, err
		}
		registry.retire(actor)
	}
	return CommandResult{}, ErrActorStopped
}

// Snapshot returns a defensive copy of the table state owned by its local actor.
func (registry *ActorRegistry) Snapshot(ctx context.Context, tableID string) (Aggregate, error) {
	if _, err := uuid.Parse(tableID); err != nil {
		return Aggregate{}, fmt.Errorf("table id is invalid")
	}
	for _attempt := 0; _attempt < 2; _attempt++ {
		actor, err := registry.actor(tableID)
		if err != nil {
			return Aggregate{}, err
		}
		aggregate, err := actor.snapshot(ctx)
		if !errors.Is(err, ErrActorStopped) {
			return aggregate, err
		}
		registry.retire(actor)
	}
	return Aggregate{}, ErrActorStopped
}

// Refresh reloads durable table state through the table's serialized actor queue.
func (registry *ActorRegistry) Refresh(ctx context.Context, tableID string) (Aggregate, error) {
	if _, err := uuid.Parse(tableID); err != nil {
		return Aggregate{}, fmt.Errorf("table id is invalid")
	}
	for _attempt := 0; _attempt < 2; _attempt++ {
		actor, err := registry.actor(tableID)
		if err != nil {
			return Aggregate{}, err
		}
		aggregate, err := actor.refresh(ctx)
		if !errors.Is(err, ErrActorStopped) {
			return aggregate, err
		}
		registry.retire(actor)
	}
	return Aggregate{}, ErrActorStopped
}

// Stats returns a synchronized process-local lifecycle snapshot.
func (registry *ActorRegistry) Stats() ActorRegistryStats {
	registry.mutex.Lock()
	defer registry.mutex.Unlock()
	return ActorRegistryStats{ActiveActors: len(registry.actors), Draining: registry.draining}
}

// Drain stops admission and waits for every already queued actor request.
func (registry *ActorRegistry) Drain(ctx context.Context) error {
	registry.mutex.Lock()
	if !registry.draining {
		registry.draining = true
		actors := make([]*tableActor, 0, len(registry.actors))
		for _, actor := range registry.actors {
			actors = append(actors, actor)
		}
		registry.options.Logger.InfoContext(ctx, "table_actor_registry_draining", "active_actors", len(actors))
		for _, actor := range actors {
			actor.beginDrain()
		}
	}
	registry.mutex.Unlock()

	done := make(chan struct{})
	go func() {
		registry.wait.Wait()
		close(done)
	}()
	select {
	case <-done:
		registry.options.Logger.InfoContext(ctx, "table_actor_registry_drained")
		return nil
	case <-ctx.Done():
		return fmt.Errorf("drain table actors: %w", ctx.Err())
	}
}

func (registry *ActorRegistry) actor(tableID string) (*tableActor, error) {
	registry.mutex.Lock()
	defer registry.mutex.Unlock()
	if registry.draining {
		return nil, ErrActorRegistryDraining
	}
	if actor := registry.actors[tableID]; actor != nil {
		return actor, nil
	}
	actor := &tableActor{
		tableID:   tableID,
		hydrator:  registry.hydrator,
		handler:   registry.handler,
		queue:     make(chan actorRequest, registry.options.QueueCapacity),
		idle:      registry.options.IdleTimeout,
		logger:    registry.options.Logger,
		now:       registry.options.Now,
		onStop:    registry.actorStopped,
		startedAt: registry.options.Now().UTC(),
		accepting: true,
	}
	registry.actors[tableID] = actor
	registry.wait.Add(1)
	go actor.run()
	return actor, nil
}

func (registry *ActorRegistry) retire(actor *tableActor) {
	registry.mutex.Lock()
	defer registry.mutex.Unlock()
	if registry.actors[actor.tableID] == actor {
		delete(registry.actors, actor.tableID)
	}
}

func (registry *ActorRegistry) actorStopped(actor *tableActor) {
	registry.retire(actor)
	registry.wait.Done()
}

func (actor *tableActor) submit(ctx context.Context, command CommandRequest) (CommandResult, error) {
	response := make(chan actorResponse, 1)
	if err := actor.enqueue(ctx, actorRequest{ctx: ctx, command: &command, response: response}); err != nil {
		return CommandResult{}, err
	}
	select {
	case result := <-response:
		return result.commandResult, result.err
	case <-ctx.Done():
		return CommandResult{}, ctx.Err()
	}
}

func (actor *tableActor) snapshot(ctx context.Context) (Aggregate, error) {
	response := make(chan actorResponse, 1)
	if err := actor.enqueue(ctx, actorRequest{ctx: ctx, response: response}); err != nil {
		return Aggregate{}, err
	}
	select {
	case result := <-response:
		return result.aggregate, result.err
	case <-ctx.Done():
		return Aggregate{}, ctx.Err()
	}
}

func (actor *tableActor) refresh(ctx context.Context) (Aggregate, error) {
	response := make(chan actorResponse, 1)
	if err := actor.enqueue(ctx, actorRequest{ctx: ctx, refresh: true, response: response}); err != nil {
		return Aggregate{}, err
	}
	select {
	case result := <-response:
		return result.aggregate, result.err
	case <-ctx.Done():
		return Aggregate{}, ctx.Err()
	}
}

func (actor *tableActor) enqueue(ctx context.Context, request actorRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	actor.lifecycleMutex.Lock()
	defer actor.lifecycleMutex.Unlock()
	if !actor.accepting {
		return ErrActorStopped
	}
	select {
	case actor.queue <- request:
		return nil
	default:
		attributes := []any{
			"table_id", actor.tableID,
			"queue_depth", len(actor.queue),
			"queue_capacity", cap(actor.queue),
		}
		if request.command != nil {
			attributes = append(attributes, "request_id", request.command.RequestID, "command_name", request.command.Command.Name)
		}
		actor.logger.WarnContext(ctx, "table_actor_queue_full", attributes...)
		return ErrActorQueueFull
	}
}

func (actor *tableActor) beginDrain() {
	actor.lifecycleMutex.Lock()
	defer actor.lifecycleMutex.Unlock()
	if !actor.accepting {
		return
	}
	actor.accepting = false
	close(actor.queue)
}

func (actor *tableActor) run() {
	actor.logger.Info("table_actor_started", "table_id", actor.tableID, "queue_capacity", cap(actor.queue))
	idleTimer := time.NewTimer(actor.idle)
	defer idleTimer.Stop()
	var aggregate *Aggregate
	stopReason := "drained"
	defer func() {
		actor.logger.Info("table_actor_stopped",
			"table_id", actor.tableID,
			"reason", stopReason,
			"lifetime_ms", actor.now().UTC().Sub(actor.startedAt).Milliseconds(),
		)
		actor.onStop(actor)
	}()

	for {
		select {
		case request, open := <-actor.queue:
			if !open {
				return
			}
			actor.handle(request, &aggregate)
			if !idleTimer.Stop() {
				select {
				case <-idleTimer.C:
				default:
				}
			}
			idleTimer.Reset(actor.idle)
		case <-idleTimer.C:
			actor.lifecycleMutex.Lock()
			if actor.accepting && len(actor.queue) == 0 {
				actor.accepting = false
				actor.lifecycleMutex.Unlock()
				stopReason = "idle"
				return
			}
			actor.lifecycleMutex.Unlock()
			idleTimer.Reset(actor.idle)
		}
	}
}

func (actor *tableActor) handle(request actorRequest, aggregate **Aggregate) {
	hydratedState := false
	if *aggregate == nil || request.refresh {
		startedAt := actor.now().UTC()
		hydrated, err := actor.hydrator.FindTable(request.ctx, actor.tableID)
		if err == nil && hydrated.ID != actor.tableID {
			err = fmt.Errorf("hydrated table id does not match actor")
		}
		if err == nil {
			err = hydrated.Validate()
		}
		latency := actor.now().UTC().Sub(startedAt)
		if err != nil {
			actor.logger.ErrorContext(request.ctx, "table_actor_hydrate_failed",
				"table_id", actor.tableID,
				"result_code", "HYDRATE_ERROR",
				"latency_ms", latency.Milliseconds(),
			)
			request.response <- actorResponse{err: fmt.Errorf("hydrate table actor: %w", err)}
			return
		}
		cached := hydrated.clone()
		*aggregate = &cached
		hydratedState = true
		actor.logger.InfoContext(request.ctx, "table_actor_hydrated",
			"table_id", actor.tableID,
			"revision", hydrated.Revision,
			"seq", hydrated.LastSeq,
			"latency_ms", latency.Milliseconds(),
		)
	}

	if request.command == nil {
		if hydratedState {
			actor.driveBots(request.ctx, aggregate)
		}
		request.response <- actorResponse{aggregate: (*aggregate).clone()}
		return
	}
	result, err := actor.handler.Process(request.ctx, *request.command)
	if result.Aggregate.ID == actor.tableID {
		cached := result.Aggregate.clone()
		*aggregate = &cached
	}
	if err == nil && !result.Duplicate && result.Outcome.Status == CommandStatusAccepted {
		result.AutomatedResults = actor.driveBots(request.ctx, aggregate)
	}
	request.response <- actorResponse{commandResult: result, err: err}
}

func (actor *tableActor) driveBots(ctx context.Context, aggregate **Aggregate) []CommandResult {
	results := make([]CommandResult, 0)
	for _actionIndex := 0; _actionIndex < maxConsecutiveBotActions; _actionIndex++ {
		command, ready := nextBotCommand(**aggregate)
		if !ready {
			return results
		}
		request := CommandRequest{
			TableID:          actor.tableID,
			SessionID:        (*aggregate).OwnerSessionID,
			RequestID:        fmt.Sprintf("bot_action_%d", (*aggregate).Revision+1),
			ExpectedRevision: (*aggregate).Revision,
			Command:          command,
		}
		result, err := actor.handler.Process(ctx, request)
		if err != nil {
			actor.logger.ErrorContext(ctx, "table_bot_action_failed", "table_id", actor.tableID, "command_name", command.Name, "result_code", "PROCESS_ERROR")
			return results
		}
		if result.Outcome.Status != CommandStatusAccepted || result.Aggregate.ID != actor.tableID {
			actor.logger.ErrorContext(ctx, "table_bot_action_rejected", "table_id", actor.tableID, "command_name", command.Name, "result_code", result.Outcome.ErrorCode)
			return results
		}
		cached := result.Aggregate.clone()
		*aggregate = &cached
		results = append(results, result)
	}
	actor.logger.ErrorContext(ctx, "table_bot_action_limit_reached", "table_id", actor.tableID, "result_code", "ACTION_LIMIT")
	return results
}
