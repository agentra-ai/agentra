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

// TestRecordTaskMetric_FireAndForget verifies the hook at the heart of Issue #11:
// when TaskService.CompleteTask runs, a metric row lands in agent_task_metrics.
// Skips cleanly if DATABASE_URL is unreachable (no Docker, no CI).
func TestRecordTaskMetric_FireAndForget(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		c := os.Getenv("POSTGRES_HOST")
		if c == "" {
			c = "localhost"
		}
		dbURL = "postgres://agentra:agentra@" + c + ":5432/agentra?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Skipf("skipping: cannot connect: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("skipping: db not reachable: %v", err)
	}

	queries := db.New(pool)

	// Fixture: pick any agent from the workspace.
	agents, err := queries.ListAgentsByWorkspace(ctx, mustFirstWorkspaceID(t, queries))
	if err != nil || len(agents) == 0 {
		t.Skipf("no agent fixture: %v", err)
	}
	agent := agents[0]

	// Fixture: one issue to attach metrics to.
	issue, err := queries.CreateIssue(ctx, db.CreateIssueParams{
		WorkspaceID: agent.WorkspaceID,
		Title:       "[smoke] metrics hook " + time.Now().Format("20060102150405"),
		Status:      "todo",
		Priority:    "low",
		CreatorType: "member",
		CreatorID:   agent.OwnerID,
	})
	if err != nil {
		t.Fatalf("create issue: %v", err)
	}

	// Fixture: a completed task is required for the FK in agent_task_metrics.
	task, err := queries.CreateAgentTask(ctx, db.CreateAgentTaskParams{
		AgentID:     agent.ID,
		RuntimeID:   agent.RuntimeID,
		IssueID:     issue.ID,
		Priority:    1,
		TaskType:    "docs",
		RuntimeType: "local",
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	// Fire the hook the same way CompleteTask does.
	ts := &TaskService{Queries: queries}
	ts.recordTaskMetric(ctx, db.AgentTaskQueue{
		ID:          task.ID,
		WorkspaceID: agent.WorkspaceID,
		IssueID:     issue.ID,
		TaskType:    "docs",
		RuntimeType: "local",
	}, "completed", 1234, 100, 200, 0.001, "")

	// Verify: exactly one metric row exists for this issue, status=completed.
	rows, err := queries.GetMetricsByIssue(ctx, issue.ID)
	if err != nil {
		t.Fatalf("GetMetricsByIssue: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 metric row, got %d", len(rows))
	}
	if rows[0].Status != "completed" {
		t.Errorf("metric status=%q, want completed", rows[0].Status)
	}
	if rows[0].TaskType != "docs" {
		t.Errorf("metric task_type=%q, want docs", rows[0].TaskType)
	}
}


func mustFirstWorkspaceID(t *testing.T, queries *db.Queries) pgtype.UUID {
	t.Helper()
	ws, err := queries.ListWorkspaces(context.Background())
	if err != nil || len(ws) == 0 {
		t.Skipf("no workspace fixture: %v", err)
	}
	return ws[0].ID
}
