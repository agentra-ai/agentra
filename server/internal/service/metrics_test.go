package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/agentra-ai/agentra/server/pkg/db/generated"
)

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
