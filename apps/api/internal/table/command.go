package table

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

const processedCommandLifetime = 24 * time.Hour

// CommandStatus identifies whether a durable command was accepted or rejected.
type CommandStatus string

const (
	CommandStatusAccepted CommandStatus = "ACCEPTED"
	CommandStatusRejected CommandStatus = "REJECTED"
)

// CommandRequest is one authenticated, revision-fenced mutation request.
type CommandRequest struct {
	TableID          string
	SessionID        string
	RequestID        string
	ExpectedRevision int64
	Command          Command
}

// CommandOutcome is the idempotent response stored for one request ID.
type CommandOutcome struct {
	RequestID   string        `json:"requestId"`
	CommandName CommandName   `json:"commandName"`
	Status      CommandStatus `json:"status"`
	ErrorCode   ErrorCode     `json:"errorCode,omitempty"`
	Revision    int64         `json:"revision"`
	FirstSeq    int64         `json:"firstSeq,omitempty"`
	LastSeq     int64         `json:"lastSeq"`
}

// PersistedEvent is one ordered event committed with its private snapshot.
type PersistedEvent struct {
	TableID    string
	Seq        int64
	Revision   int64
	Type       string
	Payload    any
	OccurredAt time.Time
}

// CommandResult is returned only after its transaction has committed.
type CommandResult struct {
	Outcome   CommandOutcome
	Aggregate Aggregate
	Events    []PersistedEvent
	Duplicate bool
}

// CommittedBatch contains only durable event facts safe for a downstream publisher.
type CommittedBatch struct {
	TableID  string
	Revision int64
	Events   []PersistedEvent
}

// CommandRepository atomically decides, persists, and returns one command outcome.
type CommandRepository interface {
	ProcessCommand(context.Context, CommandRequest, time.Time, time.Time) (CommandResult, error)
}

// CommittedPublisher publishes events only after CommandRepository returns a committed result.
type CommittedPublisher interface {
	PublishCommitted(context.Context, CommittedBatch) error
}

// CommandProcessor validates command envelopes and enforces commit-before-publish ordering.
type CommandProcessor struct {
	repository CommandRepository
	publisher  CommittedPublisher
	logger     *slog.Logger
	now        func() time.Time
}

// NewCommandProcessor constructs the durable command application boundary.
func NewCommandProcessor(repository CommandRepository, publisher CommittedPublisher, logger *slog.Logger, now func() time.Time) (*CommandProcessor, error) {
	if repository == nil {
		return nil, fmt.Errorf("command repository is required")
	}
	if logger == nil {
		return nil, fmt.Errorf("command logger is required")
	}
	if now == nil {
		return nil, fmt.Errorf("command clock is required")
	}
	return &CommandProcessor{repository: repository, publisher: publisher, logger: logger, now: now}, nil
}

// Process commits one command before making its events visible to a publisher.
func (processor *CommandProcessor) Process(ctx context.Context, request CommandRequest) (CommandResult, error) {
	if err := validateCommandRequest(request); err != nil {
		return CommandResult{}, err
	}
	request.Command.SessionID = request.SessionID
	startedAt := processor.now().UTC()
	result, err := processor.repository.ProcessCommand(ctx, request, startedAt, startedAt.Add(processedCommandLifetime))
	latency := processor.now().UTC().Sub(startedAt)
	if err != nil {
		processor.logger.ErrorContext(ctx, "table_command_persistence_failed",
			"request_id", request.RequestID,
			"table_id", request.TableID,
			"command_name", request.Command.Name,
			"result_code", "DB_ERROR",
			"latency_ms", latency.Milliseconds(),
		)
		return CommandResult{}, fmt.Errorf("persist table command: %w", err)
	}
	if err := validateCommandResult(request, result); err != nil {
		processor.logger.ErrorContext(ctx, "table_command_result_invalid",
			"request_id", request.RequestID,
			"table_id", request.TableID,
			"command_name", request.Command.Name,
			"result_code", "INVALID_RESULT",
		)
		return CommandResult{}, err
	}

	resultCode := string(result.Outcome.Status)
	if result.Duplicate {
		resultCode = "DUPLICATE"
	} else if result.Outcome.Status == CommandStatusRejected {
		resultCode = string(result.Outcome.ErrorCode)
	}
	processor.logger.InfoContext(ctx, "table_command_processed",
		"request_id", request.RequestID,
		"table_id", request.TableID,
		"command_name", request.Command.Name,
		"result_code", resultCode,
		"revision", result.Outcome.Revision,
		"seq", result.Outcome.LastSeq,
		"event_count", len(result.Events),
		"duplicate", result.Duplicate,
		"latency_ms", latency.Milliseconds(),
	)

	if processor.publisher == nil || result.Duplicate || result.Outcome.Status != CommandStatusAccepted {
		return result, nil
	}
	batch := CommittedBatch{TableID: request.TableID, Revision: result.Outcome.Revision, Events: append([]PersistedEvent(nil), result.Events...)}
	if err := processor.publisher.PublishCommitted(ctx, batch); err != nil {
		processor.logger.ErrorContext(ctx, "table_command_publish_failed",
			"request_id", request.RequestID,
			"table_id", request.TableID,
			"command_name", request.Command.Name,
			"result_code", "PUBLISH_ERROR",
			"revision", result.Outcome.Revision,
			"seq", result.Outcome.LastSeq,
			"committed", true,
		)
		return result, fmt.Errorf("publish committed table events: %w", err)
	}
	return result, nil
}

func validateCommandRequest(request CommandRequest) error {
	if _, err := uuid.Parse(request.TableID); err != nil {
		return fmt.Errorf("table id is invalid")
	}
	if _, err := uuid.Parse(request.SessionID); err != nil {
		return fmt.Errorf("session id is invalid")
	}
	if request.ExpectedRevision < 0 || request.Command.Name == "" {
		return fmt.Errorf("command envelope is invalid")
	}
	if len(request.RequestID) < 8 || len(request.RequestID) > 64 {
		return fmt.Errorf("request id is invalid")
	}
	for _, character := range request.RequestID {
		if character < 'a' || character > 'z' {
			if character < 'A' || character > 'Z' {
				if character < '0' || character > '9' {
					if character != '-' && character != '_' {
						return fmt.Errorf("request id is invalid")
					}
				}
			}
		}
	}
	return nil
}

func validateCommandResult(request CommandRequest, result CommandResult) error {
	if err := result.Aggregate.Validate(); err != nil {
		return fmt.Errorf("validate command aggregate: %w", err)
	}
	if result.Aggregate.ID != request.TableID || result.Outcome.RequestID != request.RequestID || result.Outcome.CommandName != request.Command.Name || result.Outcome.Revision < 0 || result.Outcome.LastSeq < 0 {
		return fmt.Errorf("command outcome does not match request")
	}
	if result.Duplicate {
		if len(result.Events) != 0 || result.Aggregate.Revision < result.Outcome.Revision || result.Aggregate.LastSeq < result.Outcome.LastSeq {
			return fmt.Errorf("duplicate command result is inconsistent")
		}
		return nil
	}
	if result.Aggregate.Revision != result.Outcome.Revision || result.Aggregate.LastSeq != result.Outcome.LastSeq {
		return fmt.Errorf("command outcome does not match aggregate")
	}
	switch result.Outcome.Status {
	case CommandStatusAccepted:
		if result.Outcome.ErrorCode != "" || len(result.Events) == 0 || result.Outcome.FirstSeq != result.Events[0].Seq || result.Outcome.LastSeq != result.Events[len(result.Events)-1].Seq {
			return fmt.Errorf("accepted command result is inconsistent")
		}
		for _index, event := range result.Events {
			if event.TableID != request.TableID || event.Revision != result.Outcome.Revision || event.Seq != result.Outcome.FirstSeq+int64(_index) {
				return fmt.Errorf("accepted command events are not contiguous")
			}
		}
	case CommandStatusRejected:
		if result.Outcome.ErrorCode == "" || len(result.Events) != 0 || result.Outcome.FirstSeq != 0 {
			return fmt.Errorf("rejected command result is inconsistent")
		}
	default:
		return fmt.Errorf("unknown command status %q", result.Outcome.Status)
	}
	return nil
}
