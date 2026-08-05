package main

import (
	"context"
	"sync"
	"testing"

	"github.com/agentra-ai/agentra/server/internal/events"
	"github.com/agentra-ai/agentra/server/internal/service"
	db "github.com/agentra-ai/agentra/server/pkg/db/generated"
)

// setupSweeperTestFixture creates an issue and a task in the given status with
// timestamps old enough to trigger the sweeper. Returns (issueID, agentID, taskID).
func setupSweeperTestFixture(t *testing.T, taskStatus string) (string, string, string) {
	t.Helper()
	ctx := context.Background()

	// Find the integration test agent
	var agentID, runtimeID string
	err := testPool.QueryRow(ctx, `
		SELECT a.id, a.runtime_id FROM agent a
		JOIN member m ON m.workspace_id = a.workspace_id
		JOIN "user" u ON u.id = m.user_id
		WHERE u.email = $1
		LIMIT 1
	`, integrationTestEmail).Scan(&agentID, &runtimeID)
	if err != nil {
		t.Fatalf("failed to find test agent: %v", err)
	}

	// Create an issue assigned to the agent
	var issueID string
	err = testPool.QueryRow(ctx, `
		INSERT INTO issue (workspace_id, title, status, priority, creator_type, creator_id, assignee_type, assignee_id)
		SELECT $1, 'Sweeper test issue', 'todo', 'none', 'member', m.user_id, 'agent', $2
		FROM member m WHERE m.workspace_id = $1 LIMIT 1
		RETURNING id
	`, testWorkspaceID, agentID).Scan(&issueID)
	if err != nil {
		t.Fatalf("failed to create test issue: %v", err)
	}

	// Create a task in the desired status with old timestamps
	var taskID string
	switch taskStatus {
	case "running":
		err = testPool.QueryRow(ctx, `
			INSERT INTO agent_task_queue (agent_id, runtime_id, issue_id, status, priority, dispatched_at, started_at)
			VALUES ($1, $2, $3, 'running', 0, now() - interval '3 hours', now() - interval '3 hours')
			RETURNING id
		`, agentID, runtimeID, issueID).Scan(&taskID)
	case "dispatched":
		err = testPool.QueryRow(ctx, `
			INSERT INTO agent_task_queue (agent_id, runtime_id, issue_id, status, priority, dispatched_at)
			VALUES ($1, $2, $3, 'dispatched', 0, now() - interval '10 minutes')
			RETURNING id
		`, agentID, runtimeID, issueID).Scan(&taskID)
	}
	if err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	var runID string
	err = testPool.QueryRow(ctx, `
		INSERT INTO task_runs (task_id, agent_id, status, started_at)
		VALUES ($1, $2, $3, now() - interval '3 hours')
		RETURNING id
	`, taskID, agentID, taskStatus).Scan(&runID)
	if err != nil {
		t.Fatalf("failed to create active test Run: %v", err)
	}
	if _, err := testPool.Exec(ctx, `
		UPDATE agent_task_queue SET active_run_id = $2 WHERE id = $1
	`, taskID, runID); err != nil {
		t.Fatalf("failed to bind active test Run: %v", err)
	}

	// Set agent status to "working"
	_, err = testPool.Exec(ctx, `UPDATE agent SET status = 'working' WHERE id = $1`, agentID)
	if err != nil {
		t.Fatalf("failed to set agent status: %v", err)
	}

	return issueID, agentID, taskID
}

// projectSweeperEvent drives the exact durable event produced for taskID. The
// timestamp override keeps the assertion deterministic even while other Go
// packages share the isolated integration database.
func projectSweeperEvent(t *testing.T, queries *db.Queries, bus *events.Bus, taskID string) {
	t.Helper()
	ctx := context.Background()
	tag, err := testPool.Exec(ctx, `
		UPDATE lifecycle_outbox
		SET created_at = '1900-01-01T00:00:00Z'
		WHERE work_item_id = $1 AND event_type = 'run.failed' AND processed_at IS NULL
	`, taskID)
	if err != nil {
		t.Fatalf("prioritize sweeper lifecycle event: %v", err)
	}
	if tag.RowsAffected() != 1 {
		t.Fatalf("sweeper lifecycle events for task %s = %d, want 1", taskID, tag.RowsAffected())
	}
	worker := service.NewLifecycleOutboxWorker(queries, bus, nil)
	processed, err := worker.ProcessNext(ctx)
	if err != nil || !processed {
		t.Fatalf("project sweeper lifecycle event: processed=%v err=%v", processed, err)
	}
}

func cleanupSweeperFixture(t *testing.T, issueID, agentID string) {
	t.Helper()
	ctx := context.Background()
	testPool.Exec(ctx, `DELETE FROM agent_task_queue WHERE issue_id = $1`, issueID)
	testPool.Exec(ctx, `DELETE FROM issue WHERE id = $1`, issueID)
	testPool.Exec(ctx, `UPDATE agent SET status = 'idle' WHERE id = $1`, agentID)
}

// TestSweepStaleTasksBroadcastsWithWorkspaceID verifies that when the task sweeper
// fails a stale running task, the task:failed event is broadcast with the correct
// WorkspaceID so it reaches frontend WebSocket clients (events without WorkspaceID
// are silently dropped by the WS listener — that was the original bug).
func TestSweepStaleTasksBroadcastsWithWorkspaceID(t *testing.T) {
	if testPool == nil {
		t.Skip("no database connection")
	}

	issueID, agentID, taskID := setupSweeperTestFixture(t, "running")
	t.Cleanup(func() { cleanupSweeperFixture(t, issueID, agentID) })

	queries := db.New(testPool)
	bus := events.New()

	// Capture task:failed events to verify WorkspaceID is set
	var taskEvents []events.Event
	var mu sync.Mutex
	bus.Subscribe("task:failed", func(e events.Event) {
		mu.Lock()
		taskEvents = append(taskEvents, e)
		mu.Unlock()
	})

	// Use very short timeouts to trigger the sweep on our test task
	lifecycle := service.NewRunLifecycle(testPool, queries)
	failedTasks, err := lifecycle.FailStaleTasks(context.Background(), 300.0, 1.0)
	if err != nil {
		t.Fatalf("FailStaleTasks query failed: %v", err)
	}
	if failedTasks == 0 {
		t.Fatal("expected at least 1 stale task to be failed")
	}

	projectSweeperEvent(t, queries, bus, taskID)

	// Verify the event was published with WorkspaceID (the core of the bug fix)
	mu.Lock()
	defer mu.Unlock()
	var foundEvent bool
	for _, e := range taskEvents {
		payload, _ := e.Payload.(map[string]any)
		if payload["task_id"] == taskID {
			if e.WorkspaceID == "" {
				t.Fatal("task:failed event is missing WorkspaceID — this was the original bug")
			}
			if e.WorkspaceID != testWorkspaceID {
				t.Fatalf("expected WorkspaceID %s, got %s", testWorkspaceID, e.WorkspaceID)
			}
			foundEvent = true
			break
		}
	}
	if !foundEvent {
		t.Fatalf("expected task:failed event for task %s", taskID)
	}

	// Verify DB: task should be failed
	var status string
	err = testPool.QueryRow(context.Background(), `SELECT status FROM agent_task_queue WHERE id = $1`, taskID).Scan(&status)
	if err != nil {
		t.Fatalf("failed to query task status: %v", err)
	}
	if status != "failed" {
		t.Fatalf("expected task status 'failed', got '%s'", status)
	}
}

// TestSweepStaleTasksReconcileAgentStatus verifies that after the sweeper fails
// stale tasks, the agent status is reconciled from "working" back to "idle".
func TestSweepStaleTasksReconcileAgentStatus(t *testing.T) {
	if testPool == nil {
		t.Skip("no database connection")
	}

	issueID, agentID, taskID := setupSweeperTestFixture(t, "running")
	t.Cleanup(func() { cleanupSweeperFixture(t, issueID, agentID) })

	queries := db.New(testPool)
	bus := events.New()

	// Capture agent:status events
	var agentStatusEvents []events.Event
	var mu sync.Mutex
	bus.Subscribe("agent:status", func(e events.Event) {
		mu.Lock()
		agentStatusEvents = append(agentStatusEvents, e)
		mu.Unlock()
	})

	// Fail stale tasks with short timeout
	lifecycle := service.NewRunLifecycle(testPool, queries)
	failedTasks, err := lifecycle.FailStaleTasks(context.Background(), 300.0, 1.0)
	if err != nil {
		t.Fatalf("FailStaleTasks failed: %v", err)
	}
	if failedTasks == 0 {
		t.Fatal("expected at least 1 stale task")
	}

	projectSweeperEvent(t, queries, bus, taskID)

	// Verify agent status is now "idle" in DB
	var agentStatus string
	err = testPool.QueryRow(context.Background(), `SELECT status FROM agent WHERE id = $1`, agentID).Scan(&agentStatus)
	if err != nil {
		t.Fatalf("failed to query agent status: %v", err)
	}
	if agentStatus != "idle" {
		t.Fatalf("expected agent status 'idle', got '%s'", agentStatus)
	}

	// Verify agent:status event was published with correct WorkspaceID
	mu.Lock()
	defer mu.Unlock()
	if len(agentStatusEvents) == 0 {
		t.Fatal("expected agent:status event to be published")
	}
	lastEvent := agentStatusEvents[len(agentStatusEvents)-1]
	if lastEvent.WorkspaceID == "" {
		t.Fatal("agent:status event should have WorkspaceID set")
	}
	if lastEvent.WorkspaceID != testWorkspaceID {
		t.Fatalf("expected WorkspaceID %s, got %s", testWorkspaceID, lastEvent.WorkspaceID)
	}
}

// TestSweepDispatchedStaleTask verifies the sweeper handles dispatched tasks
// stuck beyond the dispatch timeout.
func TestSweepDispatchedStaleTask(t *testing.T) {
	if testPool == nil {
		t.Skip("no database connection")
	}

	issueID, agentID, taskID := setupSweeperTestFixture(t, "dispatched")
	t.Cleanup(func() { cleanupSweeperFixture(t, issueID, agentID) })

	queries := db.New(testPool)
	bus := events.New()

	// Capture task:failed events
	var taskEvents []events.Event
	var mu sync.Mutex
	bus.Subscribe("task:failed", func(e events.Event) {
		mu.Lock()
		taskEvents = append(taskEvents, e)
		mu.Unlock()
	})

	// Fail stale tasks — dispatch timeout of 1 second (our task is 10 minutes old)
	lifecycle := service.NewRunLifecycle(testPool, queries)
	failedTasks, err := lifecycle.FailStaleTasks(context.Background(), 1.0, 9000.0)
	if err != nil {
		t.Fatalf("FailStaleTasks failed: %v", err)
	}
	if failedTasks == 0 {
		t.Fatal("expected at least 1 stale dispatched task")
	}

	projectSweeperEvent(t, queries, bus, taskID)

	// Verify DB: task should be failed
	var status string
	err = testPool.QueryRow(context.Background(), `SELECT status FROM agent_task_queue WHERE id = $1`, taskID).Scan(&status)
	if err != nil {
		t.Fatalf("failed to query task: %v", err)
	}
	if status != "failed" {
		t.Fatalf("expected task status 'failed', got '%s'", status)
	}

	// Verify task:failed event was published WITH WorkspaceID
	mu.Lock()
	defer mu.Unlock()
	found := false
	for _, e := range taskEvents {
		payload, _ := e.Payload.(map[string]any)
		if payload["task_id"] == taskID {
			if e.WorkspaceID == "" {
				t.Fatal("task:failed event is missing WorkspaceID — this was the bug")
			}
			if e.WorkspaceID != testWorkspaceID {
				t.Fatalf("expected WorkspaceID %s, got %s", testWorkspaceID, e.WorkspaceID)
			}
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected task:failed event for task %s", taskID)
	}

	// Verify agent status reconciled to idle
	var agentStatus string
	err = testPool.QueryRow(context.Background(), `SELECT status FROM agent WHERE id = $1`, agentID).Scan(&agentStatus)
	if err != nil {
		t.Fatalf("failed to query agent: %v", err)
	}
	if agentStatus != "idle" {
		t.Fatalf("expected agent status 'idle' after sweep, got '%s'", agentStatus)
	}
}

// TestSweepOfflineRuntimeRepairsTasksWithoutNewStaleRow covers the crash
// window between marking a runtime offline and failing its active Work Items.
// A later sweep must repair from the persisted offline status even though the
// runtime is no longer returned by MarkStaleRuntimesOffline.
func TestSweepOfflineRuntimeRepairsTasksWithoutNewStaleRow(t *testing.T) {
	if testPool == nil {
		t.Skip("no database connection")
	}
	issueID, agentID, taskID := setupSweeperTestFixture(t, "running")
	t.Cleanup(func() { cleanupSweeperFixture(t, issueID, agentID) })
	ctx := context.Background()
	var runtimeID string
	if err := testPool.QueryRow(ctx, `
		SELECT runtime_id FROM agent_task_queue WHERE id = $1
	`, taskID).Scan(&runtimeID); err != nil {
		t.Fatal(err)
	}
	if _, err := testPool.Exec(ctx, `UPDATE agent_runtime SET status = 'offline' WHERE id = $1`, runtimeID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `
			UPDATE agent_runtime SET status = 'online', last_seen_at = now() WHERE id = $1
		`, runtimeID)
	})

	queries := db.New(testPool)
	lifecycle := service.NewRunLifecycle(testPool, queries)
	sweepStaleRuntimes(ctx, queries, events.New(), lifecycle)

	var status string
	var eventCount int
	if err := testPool.QueryRow(ctx, `SELECT status FROM agent_task_queue WHERE id = $1`, taskID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if err := testPool.QueryRow(ctx, `
		SELECT count(*) FROM lifecycle_outbox
		WHERE work_item_id = $1 AND event_type = 'run.failed'
	`, taskID).Scan(&eventCount); err != nil {
		t.Fatal(err)
	}
	if status != "failed" || eventCount != 1 {
		t.Fatalf("offline repair = status:%q events:%d, want failed/1", status, eventCount)
	}
}
