package loop_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	looppkg "github.com/agentra-ai/agentra/server/internal/loop"
	dbpkg "github.com/agentra-ai/agentra/server/pkg/db/generated"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestStoreCRUD(t *testing.T) {
	pool := testPool(t)
	q := dbpkg.New(pool)
	store := looppkg.NewStore(q)

	wsID := uuid.NewString()
	issueID := uuid.NewString()
	seedWorkspaceAndIssue(t, pool, wsID, issueID)

	ctx := context.Background()
	maxIters := 7
	created, err := store.CreateLoop(ctx, looppkg.CreateLoopInput{
		IssueID:       issueID,
		WorkspaceID:   wsID,
		MaxIterations: &maxIters,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Status != looppkg.StatusPending {
		t.Errorf("expected pending, got %q", created.Status)
	}
	if created.MaxIterations != 7 {
		t.Errorf("expected max_iterations=7, got %d", created.MaxIterations)
	}
	if created.IssueID != issueID || created.WorkspaceID != wsID {
		t.Errorf("ids not echoed: issue=%q ws=%q", created.IssueID, created.WorkspaceID)
	}

	got, err := store.GetLoop(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != created.ID {
		t.Errorf("GetLoop returned %q", got.ID)
	}

	running := looppkg.StatusRunning
	plan := looppkg.StagePlan
	updated, err := store.UpdateStatus(ctx, created.ID, looppkg.UpdateStatusInput{
		Status:       &running,
		CurrentStage: &plan,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != looppkg.StatusRunning || updated.CurrentStage == nil || *updated.CurrentStage != looppkg.StagePlan {
		t.Errorf("update mismatch: %+v", updated)
	}
	if updated.Iteration != 0 {
		t.Errorf("expected iteration=0 by default, got %d", updated.Iteration)
	}

	prURL := "https://github.com/agentra-ai/agentra/pull/42"
	prNum := 42
	branch := "loop/issue-1-3"
	prLoop, err := store.SetPR(ctx, created.ID, prURL, prNum, branch)
	if err != nil {
		t.Fatal(err)
	}
	if prLoop.PRURL == nil || *prLoop.PRURL != prURL {
		t.Errorf("pr_url not set: %+v", prLoop.PRURL)
	}
	if prLoop.PRNumber == nil || *prLoop.PRNumber != prNum {
		t.Errorf("pr_number not set: %+v", prLoop.PRNumber)
	}
	if prLoop.BranchName == nil || *prLoop.BranchName != branch {
		t.Errorf("branch_name not set: %+v", prLoop.BranchName)
	}
}

func TestStoreGetNotFound(t *testing.T) {
	pool := testPool(t)
	q := dbpkg.New(pool)
	store := looppkg.NewStore(q)

	_, err := store.GetLoop(context.Background(), uuid.NewString())
	if err != looppkg.ErrLoopNotFound {
		t.Errorf("expected ErrLoopNotFound, got %v", err)
	}
}

func TestStoreLoadActive(t *testing.T) {
	pool := testPool(t)
	q := dbpkg.New(pool)
	store := looppkg.NewStore(q)

	wsID := uuid.NewString()
	issueID := uuid.NewString()
	seedWorkspaceAndIssue(t, pool, wsID, issueID)

	ctx := context.Background()
	created, err := store.CreateLoop(ctx, looppkg.CreateLoopInput{
		IssueID:     issueID,
		WorkspaceID: wsID,
	})
	if err != nil {
		t.Fatal(err)
	}

	running := looppkg.StatusRunning
	if _, err := store.UpdateStatus(ctx, created.ID, looppkg.UpdateStatusInput{Status: &running}); err != nil {
		t.Fatal(err)
	}

	loops, err := store.LoadActive(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, l := range loops {
		if l.ID == created.ID {
			found = true
			if l.Status != looppkg.StatusRunning {
				t.Errorf("active loop not running: %q", l.Status)
			}
		}
	}
	if !found {
		t.Errorf("created loop not returned by LoadActive")
	}
}

// seedWorkspaceAndIssue inserts a minimal workspace + issue for FK satisfaction.
// Required to satisfy loops.issue_id and loops.workspace_id FKs.
func seedWorkspaceAndIssue(t *testing.T, pool *pgxpool.Pool, wsID, issueID string) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO workspace (id, name, slug) VALUES ($1, 'Test', $2)`,
		wsID, "test-"+wsID[:8])
	if err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	_, err = pool.Exec(context.Background(),
		`INSERT INTO issue (id, workspace_id, title, status, priority, creator_type, creator_id, number) VALUES ($1, $2, 'Test', 'todo', 'medium', 'member', $3, 1)`,
		issueID, wsID, uuid.NewString())
	if err != nil {
		t.Fatalf("seed issue: %v", err)
	}
}
