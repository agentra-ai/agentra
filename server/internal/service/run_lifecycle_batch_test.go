package service

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/agentra-ai/agentra/server/pkg/db/generated"
)

func TestRunLifecycleCancelForIssueEmitsOneFactPerWorkItem(t *testing.T) {
	ctx, pool, q := lifecycleBatchPool(t)
	agent, cleanup := createMetricTestFixture(t, ctx, pool, q)
	defer cleanup()
	issue := createLifecycleBatchIssue(t, ctx, q, agent)

	running, err := q.CreateAgentTask(ctx, db.CreateAgentTaskParams{
		AgentID: agent.ID, RuntimeID: agent.RuntimeID, IssueID: issue.ID,
		Priority: 1, RuntimeType: "local", TaskType: "standard",
	})
	if err != nil {
		t.Fatal(err)
	}
	var runID pgtype.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO task_runs (task_id, agent_id, status, started_at)
		VALUES ($1, $2, 'running', now()) RETURNING id
	`, running.ID, agent.ID).Scan(&runID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		UPDATE agent_task_queue
		SET status = 'running', started_at = now(), active_run_id = $2
		WHERE id = $1
	`, running.ID, runID); err != nil {
		t.Fatal(err)
	}
	queued, err := q.CreateAgentTask(ctx, db.CreateAgentTaskParams{
		AgentID: agent.ID, RuntimeID: agent.RuntimeID, IssueID: issue.ID,
		Priority: 1, RuntimeType: "local", TaskType: "standard",
	})
	if err != nil {
		t.Fatal(err)
	}

	lifecycle := NewRunLifecycle(pool, q)
	cancelled, err := lifecycle.CancelForIssue(ctx, issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	if cancelled != 2 {
		t.Fatalf("cancelled Work Items = %d, want 2", cancelled)
	}
	for _, taskID := range []pgtype.UUID{queued.ID, running.ID} {
		task, err := q.GetAgentTask(ctx, taskID)
		if err != nil {
			t.Fatal(err)
		}
		if task.Status != "cancelled" || task.ActiveRunID.Valid {
			t.Fatalf("cancelled Work Item = status:%q active_run:%v", task.Status, task.ActiveRunID)
		}
	}
	run, err := q.GetTaskRun(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != "cancelled" {
		t.Fatalf("cancelled Run status = %q", run.Status)
	}
	var eventCount, runBound int
	if err := pool.QueryRow(ctx, `
		SELECT count(*), count(run_id)
		FROM lifecycle_outbox
		WHERE work_item_id IN ($1, $2) AND event_type = 'run.cancelled'
	`, queued.ID, running.ID).Scan(&eventCount, &runBound); err != nil {
		t.Fatal(err)
	}
	if eventCount != 2 || runBound != 1 {
		t.Fatalf("cancel events = total:%d run_bound:%d, want 2/1", eventCount, runBound)
	}
	again, err := lifecycle.CancelForIssue(ctx, issue.ID)
	if err != nil || again != 0 {
		t.Fatalf("repeat cancel = %d, err:%v", again, err)
	}
}

func TestRunLifecycleRecoverRuntimeBindsRetryToInterruptedRun(t *testing.T) {
	ctx, pool, q := lifecycleBatchPool(t)
	agent, cleanup := createMetricTestFixture(t, ctx, pool, q)
	defer cleanup()
	issue := createLifecycleBatchIssue(t, ctx, q, agent)
	task, err := q.CreateAgentTask(ctx, db.CreateAgentTaskParams{
		AgentID: agent.ID, RuntimeID: agent.RuntimeID, IssueID: issue.ID,
		Priority: 1, RuntimeType: "local", TaskType: "standard",
	})
	if err != nil {
		t.Fatal(err)
	}
	var runID pgtype.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO task_runs (task_id, agent_id, status, started_at)
		VALUES ($1, $2, 'running', now()) RETURNING id
	`, task.ID, agent.ID).Scan(&runID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		UPDATE agent_task_queue
		SET status = 'running', started_at = now(), active_run_id = $2
		WHERE id = $1
	`, task.ID, runID); err != nil {
		t.Fatal(err)
	}

	lifecycle := NewRunLifecycle(pool, q)
	requeued, failed, err := lifecycle.RecoverTasksForRuntime(ctx, agent.RuntimeID)
	if err != nil {
		t.Fatal(err)
	}
	if requeued != 1 || failed != 0 {
		t.Fatalf("recovery = requeued:%d failed:%d", requeued, failed)
	}
	recovered, err := q.GetAgentTask(ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.Status != "queued" || recovered.RetryCount != 1 || recovered.ActiveRunID.Valid {
		t.Fatalf("recovered Work Item = status:%q retries:%d active:%v", recovered.Status, recovered.RetryCount, recovered.ActiveRunID)
	}
	run, err := q.GetTaskRun(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != "failed" {
		t.Fatalf("interrupted Run status = %q", run.Status)
	}
	var eventRunID pgtype.UUID
	if err := pool.QueryRow(ctx, `
		SELECT run_id FROM lifecycle_outbox
		WHERE work_item_id = $1 AND event_type = 'run.retry_scheduled'
	`, task.ID).Scan(&eventRunID); err != nil {
		t.Fatal(err)
	}
	if eventRunID != runID {
		t.Fatalf("retry event Run = %v, want %v", eventRunID, runID)
	}
}

func lifecycleBatchPool(t *testing.T) (context.Context, *pgxpool.Pool, *db.Queries) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping lifecycle batch integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return ctx, pool, db.New(pool)
}

func createLifecycleBatchIssue(t *testing.T, ctx context.Context, q *db.Queries, agent db.Agent) db.Issue {
	t.Helper()
	issue, err := q.CreateIssue(ctx, db.CreateIssueParams{
		WorkspaceID: agent.WorkspaceID, Title: "lifecycle batch transition",
		Status: "todo", Priority: "high", CreatorType: "member", CreatorID: agent.OwnerID,
	})
	if err != nil {
		t.Fatal(err)
	}
	return issue
}
