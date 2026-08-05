package loop_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	looppkg "github.com/agentra-ai/agentra/server/internal/loop"
	dbpkg "github.com/agentra-ai/agentra/server/pkg/db/generated"
)

// TestIntegration_RestoreOnStartup_ReenqueuesMissingTask verifies that
// RestoreOnStartup re-enqueues a stage task for a running loop whose in-flight
// task has been lost (e.g. the server died mid-loop). Without the fix, the
// loop would sit forever in 'running' status with no work to do.
func TestIntegration_RestoreOnStartup_ReenqueuesMissingTask(t *testing.T) {
	pool := testPool(t)
	q := dbpkg.New(pool)
	coord := looppkg.NewCoordinator(q, pool)
	store := looppkg.NewStore(q)

	wsID := uuid.NewString()
	issueID := uuid.NewString()
	agentID := uuid.NewString()
	seedWorkspaceAndIssue(t, pool, wsID, issueID)
	seedAgent(t, pool, wsID, agentID)
	t.Cleanup(func() {
		cleanupLoopData(t, pool, wsID, issueID, agentID)
	})

	ctx := context.Background()
	maxIters := 3
	loopRow, err := store.CreateLoop(ctx, looppkg.CreateLoopInput{
		IssueID: issueID, WorkspaceID: wsID, MaxIterations: &maxIters,
		AgentID: &agentID,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Mark the loop running, in the 'plan' stage, with a fresh started_at
	// (now) so the loop-timeout check does not fire. A real loop that's been
	// running 31 minutes would be failed by the other test case below.
	running := looppkg.StatusRunning
	plan := looppkg.StagePlan
	now := time.Now().UTC()
	if _, err := store.UpdateStatus(ctx, loopRow.ID, looppkg.UpdateStatusInput{
		Status:       &running,
		CurrentStage: &plan,
		StartedAt:    &now,
	}); err != nil {
		t.Fatal(err)
	}

	// Sanity check: confirm there is no in-flight task yet for this loop.
	if hasInFlightTask(t, pool, loopRow.ID, "loop_plan") {
		t.Fatalf("precondition violated: loop already has an in-flight loop_plan task")
	}

	// Run the restore.
	coord.RestoreOnStartup(ctx)

	// A new loop_plan task should now be queued.
	waitForTask(t, pool, agentID, loopRow.ID, "loop_plan", "queued")
}

// TestIntegration_RestoreOnStartup_TimeoutsLongRunningLoop verifies that a
// running loop whose started_at is older than 30 minutes is marked failed
// with FailureLoopTimeout. The point is to stop loops that the server
// abandoned (e.g. crashed mid-loop) from being silently picked back up
// forever.
func TestIntegration_RestoreOnStartup_TimeoutsLongRunningLoop(t *testing.T) {
	pool := testPool(t)
	q := dbpkg.New(pool)
	coord := looppkg.NewCoordinator(q, pool)
	store := looppkg.NewStore(q)

	wsID := uuid.NewString()
	issueID := uuid.NewString()
	agentID := uuid.NewString()
	seedWorkspaceAndIssue(t, pool, wsID, issueID)
	seedAgent(t, pool, wsID, agentID)
	t.Cleanup(func() {
		cleanupLoopData(t, pool, wsID, issueID, agentID)
	})

	ctx := context.Background()
	maxIters := 3
	loopRow, err := store.CreateLoop(ctx, looppkg.CreateLoopInput{
		IssueID: issueID, WorkspaceID: wsID, MaxIterations: &maxIters,
		AgentID: &agentID,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Mark the loop running with a started_at 31 minutes ago — past the
	// default 30-minute restore timeout. The test does not enqueue a task
	// for it, so the in-flight check passes and the timeout branch fires.
	running := looppkg.StatusRunning
	plan := looppkg.StagePlan
	startedAt := time.Now().UTC().Add(-31 * time.Minute)
	if _, err := store.UpdateStatus(ctx, loopRow.ID, looppkg.UpdateStatusInput{
		Status:       &running,
		CurrentStage: &plan,
		StartedAt:    &startedAt,
	}); err != nil {
		t.Fatal(err)
	}

	coord.RestoreOnStartup(ctx)

	got, err := store.GetLoop(ctx, loopRow.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != looppkg.StatusFailed {
		t.Errorf("expected status=failed, got %q", got.Status)
	}
	if got.FailureReason == nil || *got.FailureReason != string(looppkg.FailureLoopTimeout) {
		var actual string
		if got.FailureReason != nil {
			actual = *got.FailureReason
		}
		t.Errorf("expected failure_reason=%q, got %q", looppkg.FailureLoopTimeout, actual)
	}
	if got.CompletedAt == nil {
		t.Errorf("expected completed_at set on failure")
	}
}

// TestIntegration_RestoreOnStartup_LeavesPausedLoopAlone verifies that a
// loop in 'paused' status is not re-armed by RestoreOnStartup. Pausing is an
// explicit operator action; the coordinator must not silently resume a paused
// loop on server restart.
func TestIntegration_RestoreOnStartup_LeavesPausedLoopAlone(t *testing.T) {
	pool := testPool(t)
	q := dbpkg.New(pool)
	coord := looppkg.NewCoordinator(q, pool)
	store := looppkg.NewStore(q)

	wsID := uuid.NewString()
	issueID := uuid.NewString()
	agentID := uuid.NewString()
	seedWorkspaceAndIssue(t, pool, wsID, issueID)
	seedAgent(t, pool, wsID, agentID)
	t.Cleanup(func() {
		cleanupLoopData(t, pool, wsID, issueID, agentID)
	})

	ctx := context.Background()
	maxIters := 3
	loopRow, err := store.CreateLoop(ctx, looppkg.CreateLoopInput{
		IssueID: issueID, WorkspaceID: wsID, MaxIterations: &maxIters,
		AgentID: &agentID,
	})
	if err != nil {
		t.Fatal(err)
	}

	paused := looppkg.StatusPaused
	plan := looppkg.StagePlan
	if _, err := store.UpdateStatus(ctx, loopRow.ID, looppkg.UpdateStatusInput{
		Status:       &paused,
		CurrentStage: &plan,
	}); err != nil {
		t.Fatal(err)
	}

	coord.RestoreOnStartup(ctx)

	if hasInFlightTask(t, pool, loopRow.ID, "loop_plan") {
		t.Errorf("paused loop got re-enqueued; RestoreOnStartup must leave paused loops alone")
	}
	got, err := store.GetLoop(ctx, loopRow.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != looppkg.StatusPaused {
		t.Errorf("paused loop status changed to %q", got.Status)
	}
}

func TestIntegration_RestoreOnStartup_FailsRunningLoopWithoutStage(t *testing.T) {
	pool := testPool(t)
	q := dbpkg.New(pool)
	coord := looppkg.NewCoordinator(q, pool)
	store := looppkg.NewStore(q)

	wsID := uuid.NewString()
	issueID := uuid.NewString()
	agentID := uuid.NewString()
	seedWorkspaceAndIssue(t, pool, wsID, issueID)
	seedAgent(t, pool, wsID, agentID)
	t.Cleanup(func() {
		cleanupLoopData(t, pool, wsID, issueID, agentID)
	})

	loopRow, err := store.CreateLoop(context.Background(), looppkg.CreateLoopInput{
		IssueID: issueID, WorkspaceID: wsID, AgentID: &agentID,
	})
	if err != nil {
		t.Fatal(err)
	}
	running := looppkg.StatusRunning
	now := time.Now().UTC()
	if _, err := store.UpdateStatus(context.Background(), loopRow.ID, looppkg.UpdateStatusInput{
		Status:    &running,
		StartedAt: &now,
	}); err != nil {
		t.Fatal(err)
	}

	coord.RestoreOnStartup(context.Background())

	assertInvalidConfigurationFailure(t, store, loopRow.ID)
}

func TestIntegration_RestoreOnStartup_FailsRunningLoopWithoutAgent(t *testing.T) {
	pool := testPool(t)
	q := dbpkg.New(pool)
	coord := looppkg.NewCoordinator(q, pool)
	store := looppkg.NewStore(q)

	wsID := uuid.NewString()
	issueID := uuid.NewString()
	seedWorkspaceAndIssue(t, pool, wsID, issueID)
	t.Cleanup(func() {
		cleanupLoopData(t, pool, wsID, issueID, uuid.Nil.String())
	})

	loopRow, err := store.CreateLoop(context.Background(), looppkg.CreateLoopInput{
		IssueID: issueID, WorkspaceID: wsID,
	})
	if err != nil {
		t.Fatal(err)
	}
	running := looppkg.StatusRunning
	plan := looppkg.StagePlan
	now := time.Now().UTC()
	if _, err := store.UpdateStatus(context.Background(), loopRow.ID, looppkg.UpdateStatusInput{
		Status:       &running,
		CurrentStage: &plan,
		StartedAt:    &now,
	}); err != nil {
		t.Fatal(err)
	}

	coord.RestoreOnStartup(context.Background())

	assertInvalidConfigurationFailure(t, store, loopRow.ID)
	if hasInFlightTask(t, pool, loopRow.ID, "loop_plan") {
		t.Error("invalid loop configuration must not enqueue a task")
	}
}

func assertInvalidConfigurationFailure(t *testing.T, store *looppkg.Store, loopID string) {
	t.Helper()
	got, err := store.GetLoop(context.Background(), loopID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != looppkg.StatusFailed {
		t.Errorf("expected status=failed, got %q", got.Status)
	}
	if got.FailureReason == nil || *got.FailureReason != string(looppkg.FailureInvalidConfig) {
		var actual string
		if got.FailureReason != nil {
			actual = *got.FailureReason
		}
		t.Errorf("expected failure_reason=%q, got %q", looppkg.FailureInvalidConfig, actual)
	}
	if got.CompletedAt == nil {
		t.Error("expected completed_at set on invalid configuration failure")
	}
}

// hasInFlightTask returns true if there is a queued, dispatched, or running
// task of the given type for the given loop. Used by restore tests to assert
// the precondition that no work is in flight before invoking RestoreOnStartup.
//
// The status set mirrors the canonical "in-flight" list in the sqlc query
// `HasInFlightTaskForLoopStage` in pkg/db/queries/loops.sql — keep both in sync.
func hasInFlightTask(t *testing.T, pool *pgxpool.Pool, loopID, taskType string) bool {
	t.Helper()
	var n int
	err := pool.QueryRow(context.Background(), `
		SELECT count(*) FROM agent_task_queue
		WHERE loop_id = $1 AND task_type = $2
		  AND status IN ('queued', 'dispatched', 'running')`,
		loopID, taskType,
	).Scan(&n)
	if err != nil {
		t.Fatalf("count in-flight tasks: %v", err)
	}
	return n > 0
}

// waitForTask polls agent_task_queue until a task matching (agent, loop, type,
// status) appears, or the test fails after a 2-second timeout.
func waitForTask(t *testing.T, pool *pgxpool.Pool, agentID, loopID, taskType, wantStatus string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		var n int
		err := pool.QueryRow(context.Background(), `
			SELECT count(*) FROM agent_task_queue
			WHERE agent_id = $1 AND loop_id = $2 AND task_type = $3 AND status = $4`,
			agentID, loopID, taskType, wantStatus,
		).Scan(&n)
		if err == nil && n > 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("task (agent=%s loop=%s type=%s status=%s) did not appear within 2s",
		agentID, loopID, taskType, wantStatus)
}
