package loop_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/agentra-ai/agentra/server/internal/events"
	looppkg "github.com/agentra-ai/agentra/server/internal/loop"
	dbpkg "github.com/agentra-ai/agentra/server/pkg/db/generated"
	"github.com/agentra-ai/agentra/server/pkg/protocol"
)

// TestIntegration_PlanDevelopReviewApprovedDone walks a loop through the full
// happy path: Plan completes -> Develop completes -> Review approves -> Done.
// Uses the real events.Bus so the Coordinator handlers run end-to-end.
func TestIntegration_PlanDevelopReviewApprovedDone(t *testing.T) {
	pool := testPool(t)
	q := dbpkg.New(pool)
	bus := events.New()
	coord := looppkg.NewCoordinator(q, bus)
	store := looppkg.NewStore(q)

	// Mirror the wiring in cmd/server/loop_coordinator.go: subscribe the
	// Coordinator's task lifecycle handlers to the bus.
	bus.Subscribe(protocol.EventTaskCompleted, coord.HandleTaskCompleted)
	bus.Subscribe(protocol.EventTaskFailed, coord.HandleTaskFailed)

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

	running := looppkg.StatusRunning
	plan := looppkg.StagePlan
	if _, err := store.UpdateStatus(ctx, loopRow.ID, looppkg.UpdateStatusInput{
		Status: &running, CurrentStage: &plan,
	}); err != nil {
		t.Fatal(err)
	}

	// Plan completes -> next stage should be develop.
	seedAndEmitTaskCompleted(t, pool, agentID, bus, loopRow.ID, "loop_plan", nil)
	waitForStage(t, store, loopRow.ID, looppkg.StageDevelop)

	// Develop completes -> next stage should be review.
	seedAndEmitTaskCompleted(t, pool, agentID, bus, loopRow.ID, "loop_develop", nil)
	waitForStage(t, store, loopRow.ID, looppkg.StageReview)

	// Review approves -> loop should be done.
	approved := true
	result := &looppkg.TaskResult{
		ReviewApproved: &approved,
		ReviewIssues:   "",
		PRURL:          "https://github.com/agentra-ai/agentra/pull/42",
		PRNumber:       intPtr(42),
		BranchName:     "loop/issue-1-0",
	}
	seedAndEmitTaskCompleted(t, pool, agentID, bus, loopRow.ID, "loop_review", result)
	waitForStatus(t, store, loopRow.ID, looppkg.StatusDone)

	// Final state assertions.
	final, err := store.GetLoop(ctx, loopRow.ID)
	if err != nil {
		t.Fatal(err)
	}
	if final.PRURL == nil || *final.PRURL != "https://github.com/agentra-ai/agentra/pull/42" {
		t.Errorf("expected PRURL set, got %v", final.PRURL)
	}
}

// seedAndEmitTaskCompleted inserts a task in agent_task_queue and (optionally)
// a task_run with the given output JSON, then publishes a task:completed event.
// The Coordinator's handler will see the event and run the state machine.
func seedAndEmitTaskCompleted(
	t *testing.T,
	pool *pgxpool.Pool,
	agentID string,
	bus *events.Bus,
	loopID, taskType string,
	result *looppkg.TaskResult,
) {
	t.Helper()
	ctx := context.Background()
	taskID := uuid.NewString()

	// Look up the agent's runtime_id (agent_task_queue.runtime_id is NOT NULL,
	// added in migration 004).
	var runtimeID string
	if err := pool.QueryRow(ctx, `SELECT runtime_id FROM agent WHERE id = $1`, agentID).Scan(&runtimeID); err != nil {
		t.Fatalf("lookup agent.runtime_id: %v", err)
	}

	// Mark any prior in-flight task of this type for the same loop as
	// 'completed'. The Coordinator's previous stage created a 'queued' task
	// (default status) when advancing the state machine; without this update
	// the unique index `idx_one_pending_task_per_issue` would block the
	// Coordinator's next stage from inserting its own queued task.
	if _, err := pool.Exec(ctx, `
		UPDATE agent_task_queue
		SET status = 'completed', completed_at = NOW()
		WHERE loop_id = $1 AND task_type = $2 AND status IN ('queued', 'dispatched', 'running')`,
		loopID, taskType); err != nil {
		t.Fatalf("mark prior task complete: %v", err)
	}

	// Insert the agent_task_queue row. The Coordinator only reads id, agent_id,
	// task_type, and loop_id; we use defaults for the other columns.
	_, err := pool.Exec(ctx, `
		INSERT INTO agent_task_queue (id, agent_id, runtime_id, issue_id, status, priority, task_type, loop_id, created_at)
		VALUES ($1, $2, $3, (SELECT issue_id FROM loops WHERE id = $4), 'completed', 1, $5, $4, NOW())`,
		taskID, agentID, runtimeID, loopID, taskType)
	if err != nil {
		t.Fatalf("seed task: %v", err)
	}

	// Insert the task_run row with the result JSON (or empty for plan/develop).
	var output string
	if result != nil {
		b, _ := json.Marshal(result)
		output = string(b)
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO task_runs (id, task_id, agent_id, status, output, started_at, created_at)
		VALUES ($1, $2, $3, 'completed', $4, NOW(), NOW())`,
		uuid.NewString(), taskID, agentID, output)
	if err != nil {
		t.Fatalf("seed task_run: %v", err)
	}

	// Publish the event. The Coordinator handler runs in a goroutine.
	bus.Publish(events.Event{
		Type:    protocol.EventTaskCompleted,
		Payload: map[string]any{"task_id": taskID},
	})
}

// waitForStage polls the store until the loop's current_stage matches,
// or the test fails after a 2-second timeout.
func waitForStage(t *testing.T, store *looppkg.Store, loopID string, want looppkg.Stage) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		l, err := store.GetLoop(context.Background(), loopID)
		if err == nil && l.CurrentStage != nil && *l.CurrentStage == want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	l, _ := store.GetLoop(context.Background(), loopID)
	t.Fatalf("loop %s: current_stage did not reach %q within 2s; got %v", loopID, want, l.CurrentStage)
}

// waitForStatus polls the store until the loop's status matches, or times out.
func waitForStatus(t *testing.T, store *looppkg.Store, loopID string, want looppkg.Status) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		l, err := store.GetLoop(context.Background(), loopID)
		if err == nil && l.Status == want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	l, _ := store.GetLoop(context.Background(), loopID)
	t.Fatalf("loop %s: status did not reach %q within 2s; got %q", loopID, want, l.Status)
}

// seedAgent inserts a minimal agent_runtime + agent row for FK satisfaction.
// The agent must belong to a workspace and have a runtime_id (NOT NULL FK
// to agent_runtime, added in migration 004).
func seedAgent(t *testing.T, pool *pgxpool.Pool, wsID, agentID string) {
	t.Helper()
	ctx := context.Background()
	runtimeID := uuid.NewString()
	_, err := pool.Exec(ctx,
		`INSERT INTO agent_runtime (id, workspace_id, name, runtime_mode, provider) VALUES ($1, $2, 'Test Runtime', 'local', 'legacy_local')`,
		runtimeID, wsID)
	if err != nil {
		t.Fatalf("seed agent_runtime: %v", err)
	}
	_, err = pool.Exec(ctx,
		`INSERT INTO agent (id, workspace_id, name, runtime_mode, runtime_id) VALUES ($1, $2, 'Test Agent', 'local', $3)`,
		agentID, wsID, runtimeID)
	if err != nil {
		t.Fatalf("seed agent: %v", err)
	}
}

// cleanupLoopData removes the seeded rows after a test finishes. Order matters
// because of FK constraints: child rows (task_runs, agent_task_queue, loops)
// before the agent, agent before agent_runtime, agent_runtime before
// workspace. CASCADE on the workspace would handle everything, but explicit
// deletes document the dependency order.
func cleanupLoopData(t *testing.T, pool *pgxpool.Pool, wsID, issueID, agentID string) {
	t.Helper()
	ctx := context.Background()
	_, _ = pool.Exec(ctx, `DELETE FROM task_runs WHERE agent_id = $1`, agentID)
	_, _ = pool.Exec(ctx, `DELETE FROM agent_task_queue WHERE agent_id = $1`, agentID)
	_, _ = pool.Exec(ctx, `DELETE FROM loops WHERE workspace_id = $1`, wsID)
	_, _ = pool.Exec(ctx, `DELETE FROM issue WHERE id = $1`, issueID)
	_, _ = pool.Exec(ctx, `DELETE FROM agent WHERE id = $1`, agentID)
	_, _ = pool.Exec(ctx, `DELETE FROM agent_runtime WHERE workspace_id = $1`, wsID)
	_, _ = pool.Exec(ctx, `DELETE FROM workspace WHERE id = $1`, wsID)
}

func intPtr(i int) *int { return &i }

// TestIntegration_StartLoop_TransitionsPendingToRunning verifies that
// Coordinator.StartLoop on a freshly created (status=pending, no stage) loop
// transitions it to status=running, current_stage=plan, enqueues a queued
// loop_plan task, and stamps started_at. This is the production path that
// CreateLoop in the handler now drives — previously the integration test
// set this state by hand, which masked the missing production wiring.
func TestIntegration_StartLoop_TransitionsPendingToRunning(t *testing.T) {
	pool := testPool(t)
	q := dbpkg.New(pool)
	bus := events.New()
	coord := looppkg.NewCoordinator(q, bus)
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
	// Sanity: fresh loop is pending with no stage and no started_at.
	if loopRow.Status != looppkg.StatusPending {
		t.Fatalf("precondition: expected status=pending, got %q", loopRow.Status)
	}
	if loopRow.CurrentStage != nil {
		t.Fatalf("precondition: expected current_stage=nil, got %v", *loopRow.CurrentStage)
	}
	if loopRow.StartedAt != nil {
		t.Fatalf("precondition: expected started_at=nil, got %v", *loopRow.StartedAt)
	}

	// Drive the production entry point: the handler calls StartLoop after
	// the loop is created.
	if err := coord.StartLoop(ctx, loopRow.ID); err != nil {
		t.Fatalf("StartLoop: %v", err)
	}

	// Loop should now be running in the plan stage with started_at set.
	got, err := store.GetLoop(ctx, loopRow.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != looppkg.StatusRunning {
		t.Errorf("expected status=running, got %q", got.Status)
	}
	if got.CurrentStage == nil || *got.CurrentStage != looppkg.StagePlan {
		t.Errorf("expected current_stage=plan, got %v", got.CurrentStage)
	}
	if got.StartedAt == nil {
		t.Error("expected started_at set after StartLoop")
	}

	// A loop_plan task should now be queued.
	waitForTask(t, pool, agentID, loopRow.ID, "loop_plan", "queued")
}

// TestIntegration_StartLoop_RefusesNonPending verifies that StartLoop fails
// loudly when called on a loop that is not in 'pending' status. This prevents
// accidental double-starts: re-calling on a running loop would create a second
// loop_plan task, and there is no policy that wants that.
func TestIntegration_StartLoop_RefusesNonPending(t *testing.T) {
	pool := testPool(t)
	q := dbpkg.New(pool)
	bus := events.New()
	coord := looppkg.NewCoordinator(q, bus)
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
	loopRow, err := store.CreateLoop(ctx, looppkg.CreateLoopInput{
		IssueID: issueID, WorkspaceID: wsID, AgentID: &agentID,
	})
	if err != nil {
		t.Fatal(err)
	}
	// Move it out of pending first.
	running := looppkg.StatusRunning
	if _, err := store.UpdateStatus(ctx, loopRow.ID, looppkg.UpdateStatusInput{Status: &running}); err != nil {
		t.Fatal(err)
	}

	if err := coord.StartLoop(ctx, loopRow.ID); err == nil {
		t.Error("expected StartLoop to refuse non-pending loop, got nil error")
	}
}
