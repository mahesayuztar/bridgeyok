package table

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

const inactiveTableBatchSize = 100

// InactiveTable identifies an open table at the revision observed by the lifecycle scan.
type InactiveTable struct {
	ID             string
	OwnerSessionID string
	Revision       int64
}

// InactiveTableRepository finds open tables whose durable activity is older than a cutoff.
type InactiveTableRepository interface {
	ListInactiveTables(context.Context, time.Time, int) ([]InactiveTable, error)
}

// TablePresence reports how long every participant has been offline and handles committed expiry.
type TablePresence interface {
	TableOfflineSince(string) (time.Time, bool)
	TableExpired(context.Context, CommandResult)
}

// LifecycleOptions controls abandoned table scans.
type LifecycleOptions struct {
	InactivityTimeout time.Duration
	SweepInterval     time.Duration
	Logger            *slog.Logger
	Now               func() time.Time
}

// Lifecycle closes tables only after both durable inactivity and all-offline time reach the cutoff.
type Lifecycle struct {
	repository InactiveTableRepository
	runtime    interface {
		Submit(context.Context, CommandRequest) (CommandResult, error)
	}
	presence TablePresence
	options  LifecycleOptions
}

// NewLifecycle constructs the single-instance inactive table policy runner.
func NewLifecycle(repository InactiveTableRepository, runtime interface {
	Submit(context.Context, CommandRequest) (CommandResult, error)
}, presence TablePresence, options LifecycleOptions) (*Lifecycle, error) {
	if repository == nil || runtime == nil || presence == nil {
		return nil, fmt.Errorf("table lifecycle dependencies are required")
	}
	if options.InactivityTimeout <= 0 || options.SweepInterval <= 0 {
		return nil, fmt.Errorf("table lifecycle durations must be positive")
	}
	if options.Logger == nil || options.Now == nil {
		return nil, fmt.Errorf("table lifecycle logger and clock are required")
	}
	return &Lifecycle{repository: repository, runtime: runtime, presence: presence, options: options}, nil
}

// Run scans until the application context is cancelled.
func (lifecycle *Lifecycle) Run(ctx context.Context) {
	lifecycle.sweep(ctx)
	ticker := time.NewTicker(lifecycle.options.SweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			lifecycle.sweep(ctx)
		}
	}
}

func (lifecycle *Lifecycle) sweep(ctx context.Context) {
	now := lifecycle.options.Now().UTC()
	cutoff := now.Add(-lifecycle.options.InactivityTimeout)
	candidates, err := lifecycle.repository.ListInactiveTables(ctx, cutoff, inactiveTableBatchSize)
	if err != nil {
		lifecycle.options.Logger.ErrorContext(ctx, "table_lifecycle_scan_failed", "result_code", "SCAN_ERROR")
		return
	}
	for _, candidate := range candidates {
		offlineSince, allOffline := lifecycle.presence.TableOfflineSince(candidate.ID)
		if !allOffline || offlineSince.After(cutoff) {
			continue
		}
		result, err := lifecycle.runtime.Submit(ctx, CommandRequest{
			TableID: candidate.ID, SessionID: candidate.OwnerSessionID,
			RequestID: fmt.Sprintf("table_expiry_%d", now.UnixNano()), ExpectedRevision: candidate.Revision,
			Command: Command{Name: CommandExpireTable, OccurredAt: now},
		})
		if err != nil {
			lifecycle.options.Logger.ErrorContext(ctx, "table_lifecycle_expiry_failed", "table_id", candidate.ID, "result_code", "COMMAND_ERROR")
			continue
		}
		if result.Outcome.Status != CommandStatusAccepted {
			continue
		}
		lifecycle.presence.TableExpired(ctx, result)
		lifecycle.options.Logger.InfoContext(ctx, "table_lifecycle_expired", "table_id", candidate.ID, "result_code", "EXPIRED", "revision", result.Outcome.Revision)
	}
}
