package service

import (
	"context"
	"fmt"
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

	agent, cleanup := createMetricTestFixture(t, ctx, pool, queries)
	defer cleanup()

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
		TaskType:    "standard",
		RuntimeType: "local",
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	// Fire the hook the same way CompleteTask does.
	ts := &TaskService{Queries: queries}
	ts.recordTaskMetric(ctx, db.AgentTaskQueue{
		ID:          task.ID,
		IssueID:     issue.ID,
		TaskType:    task.TaskType,
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
	if rows[0].TaskType != "other" {
		t.Errorf("metric task_type=%q, want other", rows[0].TaskType)
	}
}

func TestNormalizeMetricTaskType(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		input string
		want  string
	}{
		{input: "feature", want: "feature"},
		{input: "docs", want: "docs"},
		{input: "other", want: "other"},
		{input: "standard", want: "other"},
		{input: "loop_review", want: "other"},
		{input: "", want: "other"},
	} {
		t.Run(test.input, func(t *testing.T) {
			t.Parallel()
			if got := normalizeMetricTaskType(test.input); got != test.want {
				t.Fatalf("normalizeMetricTaskType(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}

func createMetricTestFixture(t *testing.T, ctx context.Context, pool *pgxpool.Pool, queries *db.Queries) (db.Agent, func()) {
	t.Helper()
	suffix := time.Now().UnixNano()
	email := fmt.Sprintf("metrics-test-%d@agentra.ai", suffix)
	slug := fmt.Sprintf("metrics-test-%d", suffix)

	var userID pgtype.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO "user" (name, email)
		VALUES ('Metrics Test User', $1)
		RETURNING id
	`, email).Scan(&userID); err != nil {
		t.Fatalf("create user fixture: %v", err)
	}

	var workspaceID pgtype.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO workspace (name, slug, description)
		VALUES ('Metrics Test Workspace', $1, 'Temporary metrics test workspace')
		RETURNING id
	`, slug).Scan(&workspaceID); err != nil {
		t.Fatalf("create workspace fixture: %v", err)
	}

	cleanup := func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM workspace WHERE id = $1`, workspaceID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM "user" WHERE id = $1`, userID)
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO member (workspace_id, user_id, role)
		VALUES ($1, $2, 'owner')
	`, workspaceID, userID); err != nil {
		cleanup()
		t.Fatalf("create member fixture: %v", err)
	}

	var runtimeID pgtype.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO agent_runtime (
			workspace_id, daemon_id, name, runtime_mode, provider, status,
			device_info, metadata, last_seen_at
		)
		VALUES ($1, NULL, 'Metrics Test Runtime', 'cloud', 'metrics_test',
			'online', 'Metrics test runtime', '{}'::jsonb, now())
		RETURNING id
	`, workspaceID).Scan(&runtimeID); err != nil {
		cleanup()
		t.Fatalf("create runtime fixture: %v", err)
	}

	var agentID pgtype.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO agent (
			workspace_id, name, description, runtime_mode, runtime_config,
			runtime_id, visibility, max_concurrent_tasks, owner_id, tools, triggers
		)
		VALUES ($1, 'Metrics Test Agent', '', 'cloud', '{}'::jsonb, $2,
			'workspace', 1, $3, '[]'::jsonb, '[]'::jsonb)
		RETURNING id
	`, workspaceID, runtimeID, userID).Scan(&agentID); err != nil {
		cleanup()
		t.Fatalf("create agent fixture: %v", err)
	}

	agent, err := queries.GetAgent(ctx, agentID)
	if err != nil {
		cleanup()
		t.Fatalf("load agent fixture: %v", err)
	}
	return agent, cleanup
}
