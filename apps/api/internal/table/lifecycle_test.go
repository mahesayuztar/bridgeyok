package table

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"
)

type lifecycleRepository struct {
	candidates []InactiveTable
	cutoff     time.Time
}

func (repository *lifecycleRepository) ListInactiveTables(_ context.Context, cutoff time.Time, limit int) ([]InactiveTable, error) {
	repository.cutoff = cutoff
	if limit != inactiveTableBatchSize {
		return nil, nil
	}
	return append([]InactiveTable(nil), repository.candidates...), nil
}

type lifecycleRuntime struct {
	aggregate Aggregate
	requests  []CommandRequest
}

func (runtime *lifecycleRuntime) Submit(_ context.Context, request CommandRequest) (CommandResult, error) {
	runtime.requests = append(runtime.requests, request)
	request.Command.SessionID = request.SessionID
	decision, domainError := Decide(runtime.aggregate, request.Command)
	if domainError != nil {
		return CommandResult{
			Aggregate: runtime.aggregate,
			Outcome:   CommandOutcome{Status: CommandStatusRejected, ErrorCode: domainError.Code},
		}, nil
	}
	next := decision.NextState
	next.Revision = runtime.aggregate.Revision + 1
	next.LastSeq = runtime.aggregate.LastSeq + int64(len(decision.Events))
	runtime.aggregate = next
	return CommandResult{
		Aggregate: next,
		Outcome:   CommandOutcome{Status: CommandStatusAccepted, Revision: next.Revision, LastSeq: next.LastSeq},
		Events:    []PersistedEvent{{TableID: next.ID, Type: decision.Events[0].Type}},
	}, nil
}

type lifecyclePresence struct {
	offlineSince time.Time
	allOffline   bool
	expired      []CommandResult
}

func (presence *lifecyclePresence) TableOfflineSince(string) (time.Time, bool) {
	return presence.offlineSince, presence.allOffline
}

func (presence *lifecyclePresence) TableExpired(_ context.Context, result CommandResult) {
	presence.expired = append(presence.expired, result)
}

func TestLifecycleSweepRequiresFiveMinutesInactiveAndOffline(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.September, 3, 12, 0, 0, 0, time.UTC)
	aggregate, err := NewAggregate(
		"4ad4caf7-963e-4989-86bd-2a0cb276010f",
		Participant{
			ID: "c420fa81-8e92-4855-8dc0-c246f45a1f32", SessionID: "65739950-ebaa-4b23-8657-e01819359675",
			Nickname: "Owner", Role: RoleOwner, JoinedAt: now.Add(-time.Hour),
		},
	)
	if err != nil {
		t.Fatalf("NewAggregate() error = %v", err)
	}
	tests := []struct {
		name         string
		allOffline   bool
		offlineSince time.Time
		wantExpired  bool
	}{
		{name: "participant online", allOffline: false},
		{name: "offline grace still active", allOffline: true, offlineSince: now.Add(-4 * time.Minute)},
		{name: "inactive and offline for five minutes", allOffline: true, offlineSince: now.Add(-5 * time.Minute), wantExpired: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &lifecycleRepository{candidates: []InactiveTable{{
				ID: aggregate.ID, OwnerSessionID: aggregate.OwnerSessionID, Revision: aggregate.Revision,
			}}}
			runtime := &lifecycleRuntime{aggregate: aggregate}
			presence := &lifecyclePresence{offlineSince: test.offlineSince, allOffline: test.allOffline}
			lifecycle, lifecycleError := NewLifecycle(repository, runtime, presence, LifecycleOptions{
				InactivityTimeout: 5 * time.Minute, SweepInterval: 15 * time.Second,
				Logger: slog.New(slog.NewJSONHandler(io.Discard, nil)), Now: func() time.Time { return now },
			})
			if lifecycleError != nil {
				t.Fatalf("NewLifecycle() error = %v", lifecycleError)
			}
			lifecycle.sweep(t.Context())
			if repository.cutoff != now.Add(-5*time.Minute) {
				t.Fatalf("scan cutoff = %s", repository.cutoff)
			}
			if (len(presence.expired) == 1) != test.wantExpired {
				t.Fatalf("expired results = %d, want expired %v", len(presence.expired), test.wantExpired)
			}
			if test.wantExpired {
				if len(runtime.requests) != 1 || runtime.requests[0].Command.Name != CommandExpireTable || runtime.aggregate.State != StateFinished {
					t.Fatalf("expiry request = %+v, aggregate state = %s", runtime.requests, runtime.aggregate.State)
				}
			} else if len(runtime.requests) != 0 {
				t.Fatalf("unexpected expiry requests = %+v", runtime.requests)
			}
		})
	}
}
