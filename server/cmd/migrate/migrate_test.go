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

	// Create the empty schema and switch the search_path so all DDL lands
	// in it. schema_migrations lives there too.
	if _, err := pool.Exec(ctx, fmt.Sprintf(`DROP SCHEMA IF EXISTS %q CASCADE; CREATE SCHEMA %q`, schema, schema)); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`SET search_path TO %q`, schema)); err != nil {
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

	for _, file := range files {
		version := extractVersion(file)
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
