package table

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/mahesayuztar/bridgeyok/apps/api/internal/observability"
)

const (
	testCommandTableID   = "77bfad45-a1d8-4117-9cf2-e61663f81e70"
	testCommandSessionID = "0f4b9a5b-0ea8-4ad6-a866-e576ccd8be31"
	testCommandRequestID = "request_01"
)

type commandRepositoryFake struct {
	result CommandResult
	err    error
	order  *[]string
	calls  int
}

func (repository *commandRepositoryFake) ProcessCommand(_ context.Context, _ CommandRequest, _, _ time.Time) (CommandResult, error) {
	repository.calls++
	if repository.order != nil {
		*repository.order = append(*repository.order, "commit")
	}
	return repository.result, repository.err
}

type committedPublisherFake struct {
	err   error
	order *[]string
	calls int
	batch CommittedBatch
}

func (publisher *committedPublisherFake) PublishCommitted(_ context.Context, batch CommittedBatch) error {
	publisher.calls++
	publisher.batch = batch
	if publisher.order != nil {
		*publisher.order = append(*publisher.order, "publish")
	}
	return publisher.err
}

func TestNewCommandProcessor(t *testing.T) {
	t.Parallel()

	logger := observability.NewLoggerWithWriter(slog.LevelDebug, &strings.Builder{})
	now := func() time.Time { return testJoinedAt }
	tests := []struct {
		name       string
		repository CommandRepository
		logger     *slog.Logger
		now        func() time.Time
	}{
		{name: "repository", logger: logger, now: now},
		{name: "logger", repository: &commandRepositoryFake{}, now: now},
		{name: "clock", repository: &commandRepositoryFake{}, logger: logger},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, err := NewCommandProcessor(test.repository, nil, test.logger, test.now); err == nil {
				t.Fatal("NewCommandProcessor() error = nil")
			}
		})
	}
}

func TestCommandProcessorCommitBeforePublish(t *testing.T) {
	t.Parallel()

	order := []string{}
	result := acceptedCommandResult(t)
	repository := &commandRepositoryFake{result: result, order: &order}
	publisher := &committedPublisherFake{order: &order}
	logs := &strings.Builder{}
	processor := testCommandProcessor(t, repository, publisher, logs)

	got, err := processor.Process(context.Background(), testCommandRequest())
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if len(order) != 2 || order[0] != "commit" || order[1] != "publish" {
		t.Fatalf("operation order = %v, want [commit publish]", order)
	}
	if publisher.calls != 1 || publisher.batch.Revision != result.Outcome.Revision || len(publisher.batch.Events) != 1 {
		t.Fatalf("published batch = %+v", publisher.batch)
	}
	if got.Outcome != result.Outcome {
		t.Fatalf("Process() outcome = %+v, want %+v", got.Outcome, result.Outcome)
	}
	if !strings.Contains(logs.String(), `"msg":"table_command_processed"`) || !strings.Contains(logs.String(), `"result_code":"ACCEPTED"`) {
		t.Fatalf("logs = %s", logs.String())
	}
	if strings.Contains(logs.String(), testCommandSessionID) || strings.Contains(logs.String(), "participant-owner") {
		t.Fatalf("logs contain private identity: %s", logs.String())
	}
}

func TestCommandProcessorDoesNotPublishRejectedOrDuplicate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		result     CommandResult
		resultCode string
	}{
		{name: "rejected", result: rejectedCommandResult(t, ErrorSeatTaken), resultCode: string(ErrorSeatTaken)},
		{name: "duplicate", result: duplicateCommandResult(t), resultCode: "DUPLICATE"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			publisher := &committedPublisherFake{}
			logs := &strings.Builder{}
			processor := testCommandProcessor(t, &commandRepositoryFake{result: test.result}, publisher, logs)
			if _, err := processor.Process(context.Background(), testCommandRequest()); err != nil {
				t.Fatalf("Process() error = %v", err)
			}
			if publisher.calls != 0 {
				t.Fatalf("publisher calls = %d, want 0", publisher.calls)
			}
			if !strings.Contains(logs.String(), `"result_code":"`+test.resultCode+`"`) {
				t.Fatalf("logs = %s", logs.String())
			}
		})
	}
}

func TestCommandProcessorFailureBehavior(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		repository    *commandRepositoryFake
		publisher     *committedPublisherFake
		wantCommitted bool
		wantLog       string
	}{
		{name: "persistence", repository: &commandRepositoryFake{err: errors.New("database unavailable")}, publisher: &committedPublisherFake{}, wantLog: "table_command_persistence_failed"},
		{name: "publisher", repository: &commandRepositoryFake{result: acceptedCommandResult(t)}, publisher: &committedPublisherFake{err: errors.New("socket unavailable")}, wantCommitted: true, wantLog: "table_command_publish_failed"},
		{name: "invalid repository result", repository: &commandRepositoryFake{result: CommandResult{}}, publisher: &committedPublisherFake{}, wantLog: "table_command_result_invalid"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			logs := &strings.Builder{}
			processor := testCommandProcessor(t, test.repository, test.publisher, logs)
			result, err := processor.Process(context.Background(), testCommandRequest())
			if err == nil {
				t.Fatal("Process() error = nil")
			}
			if (result.Outcome.Status == CommandStatusAccepted) != test.wantCommitted {
				t.Fatalf("committed result = %+v, want committed %v", result, test.wantCommitted)
			}
			if !strings.Contains(logs.String(), test.wantLog) {
				t.Fatalf("logs = %s, want %s", logs.String(), test.wantLog)
			}
		})
	}
}

func TestCommandProcessorRejectsInvalidEnvelopeBeforePersistence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(CommandRequest) CommandRequest
	}{
		{name: "table id", mutate: func(request CommandRequest) CommandRequest { request.TableID = "invalid"; return request }},
		{name: "session id", mutate: func(request CommandRequest) CommandRequest { request.SessionID = "invalid"; return request }},
		{name: "revision", mutate: func(request CommandRequest) CommandRequest { request.ExpectedRevision = -1; return request }},
		{name: "command", mutate: func(request CommandRequest) CommandRequest { request.Command.Name = ""; return request }},
		{name: "request length", mutate: func(request CommandRequest) CommandRequest { request.RequestID = "short"; return request }},
		{name: "request character", mutate: func(request CommandRequest) CommandRequest { request.RequestID = "request.01"; return request }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			repository := &commandRepositoryFake{}
			processor := testCommandProcessor(t, repository, nil, &strings.Builder{})
			if _, err := processor.Process(context.Background(), test.mutate(testCommandRequest())); err == nil {
				t.Fatal("Process() error = nil")
			}
			if repository.calls != 0 {
				t.Fatalf("repository calls = %d, want 0", repository.calls)
			}
		})
	}
}

func testCommandProcessor(t *testing.T, repository CommandRepository, publisher CommittedPublisher, logs *strings.Builder) *CommandProcessor {
	t.Helper()
	processor, err := NewCommandProcessor(repository, publisher, observability.NewLoggerWithWriter(slog.LevelDebug, logs), func() time.Time {
		return testJoinedAt
	})
	if err != nil {
		t.Fatalf("NewCommandProcessor() error = %v", err)
	}
	return processor
}

func testCommandRequest() CommandRequest {
	return CommandRequest{
		TableID:          testCommandTableID,
		SessionID:        testCommandSessionID,
		RequestID:        testCommandRequestID,
		ExpectedRevision: 0,
		Command:          Command{Name: CommandLockTable, Locked: true},
	}
}

func acceptedCommandResult(t *testing.T) CommandResult {
	t.Helper()
	aggregate := testAggregate(t)
	aggregate.ID = testCommandTableID
	aggregate.Revision = 1
	aggregate.LastSeq = 1
	event := PersistedEvent{TableID: testCommandTableID, Seq: 1, Revision: 1, Type: "TABLE_LOCKED", Payload: map[string]any{"locked": true}, OccurredAt: testJoinedAt}
	return CommandResult{
		Outcome:   CommandOutcome{RequestID: testCommandRequestID, CommandName: CommandLockTable, Status: CommandStatusAccepted, Revision: 1, FirstSeq: 1, LastSeq: 1},
		Aggregate: aggregate,
		Events:    []PersistedEvent{event},
	}
}

func rejectedCommandResult(t *testing.T, code ErrorCode) CommandResult {
	t.Helper()
	aggregate := testAggregate(t)
	aggregate.ID = testCommandTableID
	return CommandResult{
		Outcome:   CommandOutcome{RequestID: testCommandRequestID, CommandName: CommandLockTable, Status: CommandStatusRejected, ErrorCode: code},
		Aggregate: aggregate,
	}
}

func duplicateCommandResult(t *testing.T) CommandResult {
	t.Helper()
	result := acceptedCommandResult(t)
	result.Duplicate = true
	result.Events = nil
	return result
}
