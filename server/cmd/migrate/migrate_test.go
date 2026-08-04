package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestMigrationsApplyCleanlyAgainstFreshSchema is a regression test for the
// migration-drift bug. Migrations 032/033/034/036/037 originally referenced
// plural table names (agents, workspaces, issues) in their FOREIGN KEY clauses
// even though the schema uses singular names (agent, workspace, issue). That
// made `migrate up` fail on a fresh database. The test applies every up-file
// in order inside an isolated schema and asserts that representative tables
// from each of the previously-broken migrations exist at the end.
//
// It uses TEST_DATABASE_URL (the same env var as the loop tests). It is safe
// to run against a live database because it creates and tears down its own
// schema; nothing is created in the public schema.
func TestMigrationsApplyCleanlyAgainstFreshSchema(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping migration smoke test")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	// Unique schema name per run, so concurrent test runs don't collide and
	// leftover state from a previous failure doesn't poison this run.
	schema := fmt.Sprintf("migrate_smoke_%d", os.Getpid())
	t.Cleanup(func() {
		// Best-effort cleanup. If the test failed mid-way the schema may
		// still exist; drop it so reruns start clean.
		_, _ = pool.Exec(context.Background(), fmt.Sprintf(`DROP SCHEMA IF EXISTS %q CASCADE`, schema))
	})

	// Create the empty schema and put it first in the search_path so all
	// application DDL lands in it. Keep public as a fallback because shared
	// extensions such as pgvector may already be installed there; in that
	// case CREATE EXTENSION IF NOT EXISTS is a no-op and unqualified types
	// such as vector must still resolve.
	if _, err := pool.Exec(ctx, fmt.Sprintf(`DROP SCHEMA IF EXISTS %q CASCADE; CREATE SCHEMA %q`, schema, schema)); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`SET search_path TO %q, public`, schema)); err != nil {
		t.Fatalf("set search_path: %v", err)
	}

	// Re-run the migrator's bootstrap so subsequent code can rely on the
	// schema_migrations table being present.
	if _, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`); err != nil {
		t.Fatalf("create schema_migrations: %v", err)
	}

	// Collect every *.up.sql in lexicographic order — same ordering the
	// migrator uses.
	migrationsDir := findMigrationsDir(t)
	pattern := filepath.Join(migrationsDir, "*.up.sql")
	files, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(files) == 0 {
		t.Fatalf("no up migrations found under %q", migrationsDir)
	}
	sort.Strings(files)

	var historicalTaskID string
	for _, file := range files {
		version := extractVersion(file)
		if version == "049_run_identity" {
			historicalTaskID = seedRunIdentityHistory(t, ctx, pool)
		}
		sqlBytes, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}

		if _, err := pool.Exec(ctx, string(sqlBytes)); err != nil {
			t.Fatalf("migration %s failed: %v\n--- file: %s", version, err, file)
		}

		if _, err := pool.Exec(ctx,
			"INSERT INTO schema_migrations (version) VALUES ($1) ON CONFLICT DO NOTHING",
			version,
		); err != nil {
			t.Fatalf("record %s: %v", version, err)
		}
	}

	assertRunIdentityHistory(t, ctx, pool, historicalTaskID)

	// Representative tables from each previously-broken migration plus the
	// core tables the schema is built around. If any of these are missing,
	// a REFERENCES clause somewhere still points at a non-existent table.
	expected := []string{
		// Core tables (singular, used by the new FK clauses).
		"agent", "workspace", "issue", "agent_task_queue",
		// 032_agent_memory
		"agent_memories",
		// 033_team_memory
		"team_memory",
		// 034_task_graph
		"task_graph_nodes", "task_graph_edges",
		// 035_trace_tables (already correct, included for coverage)
		"task_runs", "trace_steps",
		// 036_github_tables (renamed to issue_git_links in 037_git_hooks)
		"github_installations", "issue_git_links",
		// 037_agent_delegation
		"agent_delegation_policies",
		// 038_loops
		"loops",
	}

	for _, table := range expected {
		var exists bool
		err := pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM information_schema.tables
				WHERE table_schema = $1 AND table_name = $2
			)
		`, schema, table).Scan(&exists)
		if err != nil {
			t.Fatalf("check table %s: %v", table, err)
		}
		if !exists {
			t.Errorf("expected table %q to exist after migrations, but it does not", table)
		}
	}
}

// seedRunIdentityHistory creates the failure-prone legacy shape immediately
// before migration 049: two execution traces but only one task_run. The old
// lifecycle wrote those ledgers independently, so this is a valid production
// state rather than synthetic corruption.
func seedRunIdentityHistory(t *testing.T, ctx context.Context, pool *pgxpool.Pool) string {
	t.Helper()
	var userID, workspaceID, runtimeID, agentID, issueID, taskID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO "user" (name, email)
		VALUES ('Run migration user', 'run-migration@example.test')
		RETURNING id
	`).Scan(&userID); err != nil {
		t.Fatalf("seed migration user: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO workspace (name, slug)
		VALUES ('Run migration workspace', 'run-migration-workspace')
		RETURNING id
	`).Scan(&workspaceID); err != nil {
		t.Fatalf("seed migration workspace: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO agent_runtime (workspace_id, name, runtime_mode, provider)
		VALUES ($1, 'Run migration runtime', 'local', 'codex')
		RETURNING id
	`, workspaceID).Scan(&runtimeID); err != nil {
		t.Fatalf("seed migration runtime: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO agent (workspace_id, name, runtime_mode, runtime_id, owner_id)
		VALUES ($1, 'Run migration agent', 'local', $2, $3)
		RETURNING id
	`, workspaceID, runtimeID, userID).Scan(&agentID); err != nil {
		t.Fatalf("seed migration agent: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO issue (workspace_id, title, creator_type, creator_id)
		VALUES ($1, 'Run migration issue', 'member', $2)
		RETURNING id
	`, workspaceID, userID).Scan(&issueID); err != nil {
		t.Fatalf("seed migration issue: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO agent_task_queue (
			agent_id, issue_id, status, runtime_id, started_at, completed_at
		)
		VALUES ($1, $2, 'completed', $3, now() - interval '2 minutes', now())
		RETURNING id
	`, agentID, issueID, runtimeID).Scan(&taskID); err != nil {
		t.Fatalf("seed migration task: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO task_runs (task_id, agent_id, status, started_at, completed_at)
		VALUES ($1, $2, 'completed', now() - interval '2 minutes', now() - interval '90 seconds')
	`, taskID, agentID); err != nil {
		t.Fatalf("seed legacy task run: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO execution_traces (
			task_id, agent_id, issue_id, provider, model, status, start_time, end_time
		) VALUES
			($1, $2, $3, 'codex', 'legacy-1', 'completed', now() - interval '2 minutes', now() - interval '90 seconds'),
			($1, $2, $3, 'codex', 'legacy-2', 'failed', now() - interval '1 minute', now())
	`, taskID, agentID, issueID); err != nil {
		t.Fatalf("seed legacy execution traces: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO task_message (task_id, seq, type, content)
		VALUES ($1, 1, 'text', 'legacy attempt output')
	`, taskID); err != nil {
		t.Fatalf("seed legacy task message: %v", err)
	}
	return taskID
}

func assertRunIdentityHistory(t *testing.T, ctx context.Context, pool *pgxpool.Pool, taskID string) {
	t.Helper()
	if taskID == "" {
		t.Fatal("run identity migration fixture was not created")
	}
	var runCount, traceCount, traceRunCount int
	if err := pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM task_runs WHERE task_id = $1),
			count(*),
			count(DISTINCT run_id)
		FROM execution_traces
		WHERE task_id = $1
	`, taskID).Scan(&runCount, &traceCount, &traceRunCount); err != nil {
		t.Fatalf("read migrated run history: %v", err)
	}
	if runCount != 2 || traceCount != 2 || traceRunCount != 2 {
		t.Fatalf("migrated history = runs:%d traces:%d trace runs:%d, want 2/2/2", runCount, traceCount, traceRunCount)
	}

	var messageRunID, otherRunID string
	if err := pool.QueryRow(ctx, `
		SELECT run_id FROM task_message WHERE task_id = $1 AND seq = 1
	`, taskID).Scan(&messageRunID); err != nil {
		t.Fatalf("read migrated message run: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT id FROM task_runs WHERE task_id = $1 AND id <> $2 ORDER BY created_at LIMIT 1
	`, taskID, messageRunID).Scan(&otherRunID); err != nil {
		t.Fatalf("read other migrated run: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO task_message (task_id, run_id, seq, type, content)
		VALUES ($1, $2, 1, 'text', 'retry output')
	`, taskID, otherRunID); err != nil {
		t.Fatalf("insert duplicate cursor in another run: %v", err)
	}
	var messageCount int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM task_message WHERE task_id = $1 AND seq = 1
	`, taskID).Scan(&messageCount); err != nil {
		t.Fatalf("count migrated messages: %v", err)
	}
	if messageCount != 2 {
		t.Fatalf("messages with reused cursor = %d, want 2", messageCount)
	}
}

// findMigrationsDir locates the migrations directory. It anchors on the
// location of this test file (cmd/migrate/migrate_test.go) so the test works
// regardless of the directory `go test` was invoked from.
func findMigrationsDir(t *testing.T) string {
	t.Helper()
	// this file lives at server/cmd/migrate/migrate_test.go, so the
	// migrations directory is two levels up.
	candidates := []string{
		filepath.Join("..", "..", "migrations"),
		filepath.Join("migrations"),
		filepath.Join("server", "migrations"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			abs, err := filepath.Abs(c)
			if err != nil {
				t.Fatalf("abs %s: %v", c, err)
			}
			return abs
		}
	}
	t.Fatalf("migrations directory not found; tried %s", strings.Join(candidates, ", "))
	return ""
}
