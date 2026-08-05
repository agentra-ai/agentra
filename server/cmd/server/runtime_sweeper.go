package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/agentra-ai/agentra/server/internal/events"
	"github.com/agentra-ai/agentra/server/internal/service"
	"github.com/agentra-ai/agentra/server/internal/util"
	db "github.com/agentra-ai/agentra/server/pkg/db/generated"
	"github.com/agentra-ai/agentra/server/pkg/protocol"
)

const (
	// sweepInterval is how often we check for stale runtimes and tasks.
	sweepInterval = 30 * time.Second
	// staleThresholdSeconds marks runtimes offline if no heartbeat for this long.
	// The daemon heartbeat interval is 15s, so 45s = 3 missed heartbeats.
	staleThresholdSeconds = 45.0
	// dispatchTimeoutSeconds fails tasks stuck in 'dispatched' beyond this.
	// The dispatched→running transition should be near-instant, so 5 minutes
	// means something went wrong (e.g. StartTask API call failed silently).
	dispatchTimeoutSeconds = 300.0
	// runningTimeoutSeconds fails tasks stuck in 'running' beyond this.
	// The default agent timeout is 2h, so 2.5h gives a generous buffer.
	runningTimeoutSeconds = 9000.0
)

// runRuntimeSweeper periodically marks runtimes as offline if their
// last_seen_at exceeds the stale threshold, and fails orphaned tasks.
// This handles cases where the daemon crashes, is killed without calling
// the deregister endpoint, or leaves tasks in a non-terminal state.
func runRuntimeSweeper(ctx context.Context, queries *db.Queries, bus *events.Bus, lifecycle *service.RunLifecycle) {
	ticker := time.NewTicker(sweepInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sweepStaleRuntimes(ctx, queries, bus, lifecycle)
			sweepStaleTasks(ctx, lifecycle)
		}
	}
}

// sweepStaleRuntimes marks runtimes offline if they haven't heartbeated,
// then fails any tasks belonging to those offline runtimes.
func sweepStaleRuntimes(ctx context.Context, queries *db.Queries, bus *events.Bus, lifecycle *service.RunLifecycle) {
	staleRows, err := queries.MarkStaleRuntimesOffline(ctx, staleThresholdSeconds)
	if err != nil {
		slog.Warn("runtime sweeper: failed to mark stale runtimes offline", "error", err)
	}

	// Always repair tasks for every offline runtime. If the server crashed after
	// marking a runtime offline but before this call, the next sweep has no new
	// staleRows but still closes the orphaned Run and emits its durable fact.
	failedTasks, err := lifecycle.FailTasksForOfflineRuntimes(ctx)
	if err != nil {
		slog.Warn("runtime sweeper: failed to clean up stale tasks", "error", err)
	} else if failedTasks > 0 {
		slog.Info("runtime sweeper: failed orphaned tasks", "count", failedTasks)
	}
	if len(staleRows) == 0 {
		return
	}

	// Collect unique workspace IDs to notify.
	workspaces := make(map[string]bool)
	for _, row := range staleRows {
		wsID := util.UUIDToString(row.WorkspaceID)
		workspaces[wsID] = true
	}

	slog.Info("runtime sweeper: marked stale runtimes offline", "count", len(staleRows), "workspaces", len(workspaces))

	// Notify frontend clients so they re-fetch runtime list.
	for wsID := range workspaces {
		bus.Publish(events.Event{
			Type:        protocol.EventDaemonRegister,
			WorkspaceID: wsID,
			ActorType:   "system",
			Payload: map[string]any{
				"action": "stale_sweep",
			},
		})
	}
}

// sweepStaleTasks fails tasks stuck in dispatched/running for too long,
// even when the runtime is still online. This handles cases where:
// - The agent process hangs and the daemon is still heartbeating
// - The daemon failed to report task completion/failure
// - A server restart left tasks in a non-terminal state
func sweepStaleTasks(ctx context.Context, lifecycle *service.RunLifecycle) {
	failedTasks, err := lifecycle.FailStaleTasks(ctx, dispatchTimeoutSeconds, runningTimeoutSeconds)
	if err != nil {
		slog.Warn("task sweeper: failed to clean up stale tasks", "error", err)
		return
	}
	if failedTasks == 0 {
		return
	}

	slog.Info("task sweeper: failed stale tasks", "count", failedTasks)
}
