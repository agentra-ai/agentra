package loop_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	looppkg "github.com/agentra-ai/agentra/server/internal/loop"
	dbpkg "github.com/agentra-ai/agentra/server/pkg/db/generated"
)

// TestIntegration_PlanDevelopReviewApprovedDone walks a loop through the full
// happy path: Plan completes -> Develop completes -> Review approves -> Done.
// Uses the durable event projector so the production transaction path runs end-to-end.
func TestIntegration_PlanDevelopReviewApprovedDone(t *testing.T) {
	pool := testPool(t)
	q := dbpkg.New(pool)
	coord := looppkg.NewCoordinator(q, pool)
	projector := looppkg.NewLifecycleProjector(pool, q, coord)
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

	running := looppkg.StatusRunning
	plan := looppkg.StagePlan
	if _, err := store.UpdateStatus(ctx, loopRow.ID, looppkg.UpdateStatusInput{
		Status: &running, CurrentStage: &plan,
	}); err != nil {
		t.Fatal(err)
	}

	// Plan completes -> next stage should be develop.
	seedAndProjectTaskCompleted(t, pool, agentID, projector, loopRow.ID, "loop_plan", nil)
	waitForStage(t, store, loopRow.ID, looppkg.StageDevelop)

	// Develop completes -> next stage should be review.
	seedAndProjectTaskCompleted(t, pool, agentID, projector, loopRow.ID, "loop_develop", nil)
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
	seedAndProjectTaskCompleted(t, pool, agentID, projector, loopRow.ID, "loop_review", result)
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

// seedAndProjectTaskCompleted inserts an exact terminal Run + durable event,
// then drives the production Engineering Loop projector synchronously.
func seedAndProjectTaskCompleted(
	t *testing.T,
	pool *pgxpool.Pool,
	agentID string,
	projector *looppkg.LifecycleProjector,
	loopID, taskType string,
	result *looppkg.TaskResult,
) {
	t.Helper()
	var output string
	if result != nil {
		b, _ := json.Marshal(result)
		output = string(b)
	}
	seedTerminalLifecycleEvent(t, pool, loopID, agentID, taskType, "run.completed", output, "")
	processed, err := projector.ProcessNext(context.Background())
	if err != nil || !processed {
		t.Fatalf("project completed task = processed:%v err:%v", processed, err)
	}
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

// TestIntegration_StartLoop_RollsBackOnInvalidStageAgent proves that the first
// Work Item and the pending->running transition share one transaction. A bad
// stage override must leave neither half committed.
func TestIntegration_StartLoop_RollsBackOnInvalidStageAgent(t *testing.T) {
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
	loopRow, err := store.CreateLoop(ctx, looppkg.CreateLoopInput{
		IssueID: issueID, WorkspaceID: wsID, AgentID: &agentID,
		Config: []byte(`{"stage_agents":{"plan":"not-a-uuid"}}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := coord.StartLoop(ctx, loopRow.ID); err == nil {
		t.Fatal("expected StartLoop to reject invalid plan agent")
	}

	got, err := store.GetLoop(ctx, loopRow.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != looppkg.StatusPending || got.CurrentStage != nil || got.StartedAt != nil {
		t.Fatalf("failed StartLoop changed loop: status=%q stage=%v started_at=%v", got.Status, got.CurrentStage, got.StartedAt)
	}
	var tasks int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM agent_task_queue WHERE loop_id = $1`, loopRow.ID).Scan(&tasks); err != nil {
		t.Fatal(err)
	}
	if tasks != 0 {
		t.Fatalf("failed StartLoop committed %d Work Items", tasks)
	}
}
