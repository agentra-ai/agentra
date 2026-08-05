package loop_test

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	looppkg "github.com/agentra-ai/agentra/server/internal/loop"
	db "github.com/agentra-ai/agentra/server/pkg/db/generated"
)

func TestLifecycleProjectorAdvancesOnceUnderConcurrentReplay(t *testing.T) {
	pool := testPool(t)
	q := db.New(pool)
	coord := looppkg.NewCoordinator(q, pool)
	projectorA := looppkg.NewLifecycleProjector(pool, q, coord)
	projectorB := looppkg.NewLifecycleProjector(pool, q, coord)

	wsID, issueID, agentID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	seedWorkspaceAndIssue(t, pool, wsID, issueID)
	seedAgent(t, pool, wsID, agentID)
	t.Cleanup(func() { cleanupLoopData(t, pool, wsID, issueID, agentID) })

	store := looppkg.NewStore(q)
	maxIterations := 3
	loopRow, err := store.CreateLoop(context.Background(), looppkg.CreateLoopInput{
		IssueID: issueID, WorkspaceID: wsID, MaxIterations: &maxIterations, AgentID: &agentID,
	})
	if err != nil {
		t.Fatal(err)
	}
	running, plan := looppkg.StatusRunning, looppkg.StagePlan
	if _, err := store.UpdateStatus(context.Background(), loopRow.ID, looppkg.UpdateStatusInput{
		Status: &running, CurrentStage: &plan,
	}); err != nil {
		t.Fatal(err)
	}

	eventID := seedTerminalLifecycleEvent(t, pool, loopRow.ID, agentID, "loop_plan", "run.completed", "{}", "")

	var wg sync.WaitGroup
	results := make(chan bool, 2)
	errs := make(chan error, 2)
	for _, projector := range []*looppkg.LifecycleProjector{projectorA, projectorB} {
		wg.Add(1)
		go func(p *looppkg.LifecycleProjector) {
			defer wg.Done()
			processed, processErr := p.ProcessNext(context.Background())
			results <- processed
			errs <- processErr
		}(projector)
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent projection: %v", err)
		}
	}
	processedCount := 0
	for processed := range results {
		if processed {
			processedCount++
		}
	}
	if processedCount != 1 {
		t.Fatalf("workers that processed event = %d, want 1", processedCount)
	}

	updated, err := store.GetLoop(context.Background(), loopRow.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.CurrentStage == nil || *updated.CurrentStage != looppkg.StageDevelop {
		t.Fatalf("loop stage = %v, want develop", updated.CurrentStage)
	}
	var tasks, receipts int
	if err := pool.QueryRow(context.Background(), `
		SELECT count(*) FROM agent_task_queue
		WHERE loop_id = $1 AND task_type = 'loop_develop' AND status = 'queued'
	`, loopRow.ID).Scan(&tasks); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(context.Background(), `
		SELECT count(*) FROM lifecycle_event_receipt
		WHERE event_id = $1 AND consumer = 'engineering-loop'
	`, eventID).Scan(&receipts); err != nil {
		t.Fatal(err)
	}
	if tasks != 1 || receipts != 1 {
		t.Fatalf("projection = tasks:%d receipts:%d, want 1/1", tasks, receipts)
	}

	// A separately inserted duplicate fact is acknowledged as a no-op because
	// the Loop has already advanced beyond the source Work Item's stage.
	duplicateEventID := duplicateLifecycleEvent(t, pool, eventID)
	processed, err := projectorA.ProcessNext(context.Background())
	if err != nil || !processed {
		t.Fatalf("duplicate projection = processed:%v err:%v", processed, err)
	}
	if err := pool.QueryRow(context.Background(), `
		SELECT count(*) FROM agent_task_queue
		WHERE loop_id = $1 AND task_type = 'loop_develop' AND status = 'queued'
	`, loopRow.ID).Scan(&tasks); err != nil {
		t.Fatal(err)
	}
	if tasks != 1 {
		t.Fatalf("duplicate event created %d develop tasks, want 1", tasks)
	}
	if err := pool.QueryRow(context.Background(), `
		SELECT count(*) FROM lifecycle_event_receipt WHERE event_id = $1
	`, duplicateEventID).Scan(&receipts); err != nil || receipts != 1 {
		t.Fatalf("duplicate receipt = %d err:%v", receipts, err)
	}
}

func TestRestoreSkipsStageWithPendingTerminalLifecycleEvent(t *testing.T) {
	pool := testPool(t)
	q := db.New(pool)
	coord := looppkg.NewCoordinator(q, pool)
	store := looppkg.NewStore(q)

	wsID, issueID, agentID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	seedWorkspaceAndIssue(t, pool, wsID, issueID)
	seedAgent(t, pool, wsID, agentID)
	t.Cleanup(func() { cleanupLoopData(t, pool, wsID, issueID, agentID) })
	maxIterations := 3
	loopRow, err := store.CreateLoop(context.Background(), looppkg.CreateLoopInput{
		IssueID: issueID, WorkspaceID: wsID, MaxIterations: &maxIterations, AgentID: &agentID,
	})
	if err != nil {
		t.Fatal(err)
	}
	running, plan := looppkg.StatusRunning, looppkg.StagePlan
	if _, err := store.UpdateStatus(context.Background(), loopRow.ID, looppkg.UpdateStatusInput{
		Status: &running, CurrentStage: &plan,
	}); err != nil {
		t.Fatal(err)
	}
	seedTerminalLifecycleEvent(t, pool, loopRow.ID, agentID, "loop_plan", "run.completed", "{}", "")

	coord.RestoreOnStartup(context.Background())
	var queued int
	if err := pool.QueryRow(context.Background(), `
		SELECT count(*) FROM agent_task_queue
		WHERE loop_id = $1 AND task_type = 'loop_plan' AND status = 'queued'
	`, loopRow.ID).Scan(&queued); err != nil {
		t.Fatal(err)
	}
	if queued != 0 {
		t.Fatalf("restore re-enqueued %d plan tasks while terminal event was pending", queued)
	}
}

func seedTerminalLifecycleEvent(t *testing.T, pool *pgxpool.Pool, loopID, agentID, taskType, eventType, output, runError string) string {
	t.Helper()
	ctx := context.Background()
	taskID, runID, eventID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	var runtimeID string
	if err := pool.QueryRow(ctx, `SELECT runtime_id FROM agent WHERE id = $1`, agentID).Scan(&runtimeID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		UPDATE agent_task_queue
		SET status = 'completed', completed_at = now(), active_run_id = NULL
		WHERE loop_id = $1 AND task_type = $2
		  AND status IN ('queued', 'dispatched', 'running')
	`, loopID, taskType); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO agent_task_queue (
			id, agent_id, runtime_id, issue_id, status, priority, task_type, loop_id, created_at
		) VALUES (
			$1, $2, $3, (SELECT issue_id FROM loops WHERE id = $4),
			CASE WHEN $6 = 'run.completed' THEN 'completed' ELSE 'failed' END,
			1, $5, $4, now()
		)
	`, taskID, agentID, runtimeID, loopID, taskType, eventType); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO task_runs (id, task_id, agent_id, status, output, error, started_at, completed_at, created_at)
		VALUES ($1, $2, $3,
			CASE WHEN $6 = 'run.completed' THEN 'completed' ELSE 'failed' END,
			$4, NULLIF($5, ''), now(), now(), now())
	`, runID, taskID, agentID, output, runError, eventType); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO lifecycle_outbox (id, work_item_id, run_id, event_type, event_version, payload)
		VALUES ($1, $2, $3, $4, 1, '{}'::jsonb)
	`, eventID, taskID, runID, eventType); err != nil {
		t.Fatal(err)
	}
	return eventID
}

func duplicateLifecycleEvent(t *testing.T, pool *pgxpool.Pool, sourceEventID string) string {
	t.Helper()
	eventID := uuid.NewString()
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO lifecycle_outbox (id, work_item_id, run_id, event_type, event_version, payload)
		SELECT $1, work_item_id, run_id, event_type, event_version, payload
		FROM lifecycle_outbox WHERE id = $2
	`, eventID, sourceEventID); err != nil {
		t.Fatal(err)
	}
	return eventID
}
