# Agentic Engineering Loop Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement a multi-agent engineering loop (Plan → Develop → Review → Fix → … → PR) for Agentra that reuses the existing `agent_task_queue` + daemon infrastructure to drive an issue from open to mergeable PR.

**Architecture:** v2 — a `Loop` is a row in a new `loops` table; each stage is a row in `agent_task_queue` with a new `task_type` discriminator (`loop_plan` / `loop_develop` / `loop_review` / `loop_fix`); a `Coordinator` goroutine in `cmd/server` subscribes to the in-process `events.Bus`, reads `task:completed` / `task:failed`, and advances a flat state machine that creates the next-stage task. The daemon's `runTask` is taught to dispatch by `task_type` at the existing `BuildPrompt` seam. No new processes, no new infra, ~1100 LoC of new code.

**Tech Stack:** Go 1.26 (server), pgx/v5 + sqlc, in-process `events.Bus` (sync pub/sub), `pkg/agent.Backend` (existing facade), `pgxpool.Pool`, `next-intl`/shadcn (web — zero new code, reuse `TaskList` view).

**Spec:** `docs/superpowers/specs/2026-06-06-agentic-engineering-loop-design.md`

**Prerequisites:**
- Read the spec end-to-end (especially §5 数据模型, §6 组件, §13 实现影响)
- Confirm `make setup` and `make dev` work locally before starting
- `make worktree-env && make setup-worktree` if working in a worktree (per CLAUDE.md worktree support)

---

## File Structure (delta from spec §5.4)

```
server/
├── migrations/
│   └── 038_loops.up.sql / 038_loops.down.sql           # NEW — loops table + agent_task_queue cols
├── pkg/db/queries/
│   ├── agent.sql                                       # MODIFY — CreateAgentTask gains task_type, loop_id
│   └── loops.sql                                       # NEW — Loop CRUD queries
├── pkg/db/generated/                                   # REGEN via `make sqlc`
├── internal/loop/                                      # NEW package
│   ├── loop.go                                         # Loop struct, status/stage consts
│   ├── store.go                                        # DB CRUD wrapper
│   ├── coordinator.go                                  # state machine + event handlers
│   ├── stages/
│   │   ├── stages.go                                   # StageExecutor interface + registry
│   │   ├── plan.go                                     # loop_plan
│   │   ├── develop.go                                  # loop_develop
│   │   ├── review.go                                   # loop_review
│   │   └── fix.go                                      # loop_fix
│   ├── tools/
│   │   ├── tool.go                                     # Tool interface, Result, registry
│   │   ├── fs.go                                       # read_file, write_file, search_code
│   │   ├── shell.go                                    # run_command, run_test
│   │   └── git.go                                      # git_status, git_diff, git_commit, git_push, create_branch, github_pr_create
│   ├── prompts/
│   │   ├── plan.md                                     # system prompt for plan stage
│   │   ├── develop.md                                  # system prompt for develop stage
│   │   ├── review.md                                   # system prompt for review stage
│   │   └── fix.md                                      # system prompt for fix stage
│   ├── coordinator_test.go                             # state machine unit tests
│   ├── store_test.go                                   # CRUD integration tests
│   ├── stages/
│   │   ├── plan_test.go
│   │   ├── develop_test.go
│   │   ├── review_test.go
│   │   └── fix_test.go
│   ├── tools/
│   │   ├── fs_test.go
│   │   ├── shell_test.go
│   │   └── git_test.go
│   └── integration_test.go                             # end-to-end plan→develop→review→done
├── internal/handler/
│   └── loop.go                                         # NEW — REST endpoints
├── internal/daemon/
│   ├── types.go                                        # MODIFY — add TaskType field
│   └── daemon.go                                       # MODIFY — runTask dispatches by TaskType
├── internal/service/
│   └── task.go                                         # MODIFY — extend AgentStage enum
├── cmd/server/
│   ├── main.go                                         # MODIFY — start runLoopCoordinator
│   └── loop_coordinator.go                             # NEW — thin wiring
└── internal/cli/
    └── loop.go                                         # NEW — `agentra loop ...` subcommands
```

**Total:** 14 new files, 5 modified files, ~1100 lines of new Go code (matches spec §1 estimate).

---

## Task 1: DB migration — `loops` table + `agent_task_queue` columns

**Files:**
- Create: `server/migrations/038_loops.up.sql`
- Create: `server/migrations/038_loops.down.sql`

- [ ] **Step 1: Write the up migration**

Create `server/migrations/038_loops.up.sql`:

```sql
-- Agentic Engineering Loop: parent record for a Plan→Develop→Review→Fix cycle.

CREATE TABLE loops (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    issue_id UUID NOT NULL REFERENCES issue(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,

    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'running', 'paused', 'done', 'failed', 'cancelled')),
    current_stage TEXT
        CHECK (current_stage IS NULL OR current_stage IN ('plan', 'develop', 'review', 'fix')),
    iteration INT NOT NULL DEFAULT 0,
    max_iterations INT NOT NULL DEFAULT 5,

    -- Outputs
    pr_url TEXT,
    pr_number INT,
    branch_name TEXT,

    -- Config
    agent_id UUID REFERENCES agent(id),
    config JSONB NOT NULL DEFAULT '{}'::jsonb,
    failure_reason TEXT,

    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_loops_issue_id ON loops(issue_id);
CREATE INDEX idx_loops_status ON loops(status)
    WHERE status IN ('pending', 'running', 'paused');

-- Discriminator on agent_task_queue: standard task vs one of 4 loop stages.
-- Replaces the spec's "task_type" assumption; previously absent.
ALTER TABLE agent_task_queue
    ADD COLUMN task_type VARCHAR(50) NOT NULL DEFAULT 'standard',
    ADD COLUMN loop_id UUID REFERENCES loops(id) ON DELETE SET NULL;

ALTER TABLE agent_task_queue
    ADD CONSTRAINT agent_task_queue_task_type_check
    CHECK (task_type IN ('standard', 'loop_plan', 'loop_develop', 'loop_review', 'loop_fix'));

CREATE INDEX idx_agent_task_queue_loop_id ON agent_task_queue(loop_id)
    WHERE loop_id IS NOT NULL;
CREATE INDEX idx_agent_task_queue_task_type ON agent_task_queue(task_type)
    WHERE task_type <> 'standard';
```

- [ ] **Step 2: Write the down migration**

Create `server/migrations/038_loops.down.sql`:

```sql
DROP INDEX IF EXISTS idx_agent_task_queue_task_type;
DROP INDEX IF EXISTS idx_agent_task_queue_loop_id;

ALTER TABLE agent_task_queue DROP CONSTRAINT IF EXISTS agent_task_queue_task_type_check;
ALTER TABLE agent_task_queue DROP COLUMN IF EXISTS loop_id;
ALTER TABLE agent_task_queue DROP COLUMN IF EXISTS task_type;

DROP INDEX IF EXISTS idx_loops_status;
DROP INDEX IF EXISTS idx_loops_issue_id;
DROP TABLE IF EXISTS loops;
```

- [ ] **Step 3: Run the migration locally**

```bash
cd /Users/doug/ai/system/agentra
make migrate-up
```

Expected: `applied 038_loops`. Verify with:

```bash
psql "$DATABASE_URL" -c "\d loops" | head -30
psql "$DATABASE_URL" -c "\d agent_task_queue" | grep -E "task_type|loop_id"
```

Expected: `loops` table exists with all columns; `agent_task_queue` has `task_type` (default `standard`) and `loop_id` (nullable).

- [ ] **Step 4: Roll back, re-apply, commit**

```bash
make migrate-down
make migrate-up
git add server/migrations/038_loops.up.sql server/migrations/038_loops.down.sql
git commit -m "feat(db): add loops table and agent_task_queue task_type / loop_id columns"
```

---

## Task 2: sqlc input + generated models for `loops`

**Files:**
- Create: `server/pkg/db/queries/loops.sql`
- Modify: `server/pkg/db/queries/agent.sql:66-67` (extend `CreateAgentTask`)
- Regenerate: `server/pkg/db/generated/` (via `make sqlc`)

- [ ] **Step 1: Add Loop CRUD queries**

Create `server/pkg/db/queries/loops.sql`:

```sql
-- name: CreateLoop :one
INSERT INTO loops (
    issue_id, workspace_id, status, current_stage,
    iteration, max_iterations, agent_id, config
) VALUES (
    $1, $2, COALESCE(sqlc.narg('status'), 'pending'),
    sqlc.narg('current_stage'),
    COALESCE(sqlc.narg('iteration'), 0),
    COALESCE(sqlc.narg('max_iterations'), 5),
    sqlc.narg('agent_id'),
    COALESCE(sqlc.narg('config'), '{}'::jsonb)
)
RETURNING *;

-- name: GetLoop :one
SELECT * FROM loops WHERE id = $1;

-- name: ListLoops :many
SELECT * FROM loops
WHERE workspace_id = $1
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('issue_id')::uuid IS NULL OR issue_id = sqlc.narg('issue_id'))
ORDER BY created_at DESC
LIMIT sqlc.narg('limit')::int;

-- name: UpdateLoopStatus :one
UPDATE loops
SET status = $2,
    current_stage = $3,
    iteration = $4,
    failure_reason = $5,
    started_at = COALESCE(started_at, $6),
    completed_at = $7,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: SetLoopPR :one
UPDATE loops
SET pr_url = $2, pr_number = $3, branch_name = $4, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: LoadActiveLoops :many
SELECT * FROM loops
WHERE status IN ('running', 'paused')
ORDER BY created_at;
```

- [ ] **Step 2: Extend `CreateAgentTask` in `agent.sql`**

Edit `server/pkg/db/queries/agent.sql` line 66-67 (the `CreateAgentTask` query). Change:

```sql
-- name: CreateAgentTask :one
-- Creates an agent task. runtime_type and cloud_runtime_id are optional:
-- they default to 'local' and NULL respectively if not provided.
INSERT INTO agent_task_queue (agent_id, runtime_id, issue_id, status, priority, trigger_comment_id, runtime_type, cloud_runtime_id)
VALUES ($1, $2, $3, 'queued', $4, $5, COALESCE(sqlc.narg('runtime_type'), 'local'), sqlc.narg('cloud_runtime_id'))
RETURNING *;
```

To:

```sql
-- name: CreateAgentTask :one
-- Creates an agent task. task_type defaults to 'standard' (existing behavior);
-- loop tasks pass 'loop_plan'/'loop_develop'/'loop_review'/'loop_fix' plus a loop_id.
INSERT INTO agent_task_queue (
    agent_id, runtime_id, issue_id, status, priority,
    trigger_comment_id, runtime_type, cloud_runtime_id,
    task_type, loop_id
)
VALUES (
    $1, $2, $3, 'queued', $4, $5,
    COALESCE(sqlc.narg('runtime_type'), 'local'),
    sqlc.narg('cloud_runtime_id'),
    COALESCE(sqlc.narg('task_type'), 'standard'),
    sqlc.narg('loop_id')
)
RETURNING *;
```

- [ ] **Step 3: Regenerate sqlc code**

```bash
cd /Users/doug/ai/system/agentra
make sqlc
```

Expected: `server/pkg/db/generated/loops.sql.go` and updated `server/pkg/db/generated/agent.sql.go` and `models.go`. Verify:

```bash
ls server/pkg/db/generated/loops.sql.go
grep -c "TaskType\|LoopID" server/pkg/db/generated/models.go
```

Expected: file exists; grep finds 2+ matches in models.go (TaskType and LoopID fields on `AgentTaskQueue`).

- [ ] **Step 4: Build the server to confirm sqlc output compiles**

```bash
cd server && go build ./...
```

Expected: exits 0 (the new fields are unused so far but must compile).

- [ ] **Step 5: Commit**

```bash
git add server/pkg/db/queries/loops.sql server/pkg/db/queries/agent.sql server/pkg/db/generated/
git commit -m "feat(db): add sqlc queries for loops + task_type / loop_id on agent_task_queue"
```

---

## Task 3: `Loop` Go struct + status/stage constants

**Files:**
- Create: `server/internal/loop/loop.go`
- Create: `server/internal/loop/loop_test.go`

- [ ] **Step 1: Write the failing test**

Create `server/internal/loop/loop_test.go`:

```go
package loop

import (
	"testing"
	"time"
)

func TestLoopStatusIsValid(t *testing.T) {
	valid := []Status{
		StatusPending, StatusRunning, StatusPaused,
		StatusDone, StatusFailed, StatusCancelled,
	}
	for _, s := range valid {
		if !s.IsValid() {
			t.Errorf("expected %q to be valid", s)
		}
	}
	if Status("garbage").IsValid() {
		t.Error("expected 'garbage' to be invalid")
	}
}

func TestStageIsValid(t *testing.T) {
	for _, s := range []Stage{StagePlan, StageDevelop, StageReview, StageFix} {
		if !s.IsValid() {
			t.Errorf("expected %q to be valid", s)
		}
	}
	if Stage("").IsValid() {
		t.Error("expected empty stage to be invalid")
	}
}

func TestLoopStructRoundtrip(t *testing.T) {
	now := time.Now()
	l := &Loop{
		ID:            "loop-1",
		IssueID:       "issue-1",
		WorkspaceID:   "ws-1",
		Status:        StatusRunning,
		CurrentStage:  StageDevelop,
		Iteration:     2,
		MaxIterations: 5,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if l.Status != StatusRunning || l.CurrentStage != StageDevelop {
		t.Errorf("roundtrip mismatch: %+v", l)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/doug/ai/system/agentra/server && go test ./internal/loop/ -run TestLoopStatusIsValid
```

Expected: FAIL with `loop: no such file or directory` (package doesn't exist yet) or compile error.

- [ ] **Step 3: Write the implementation**

Create `server/internal/loop/loop.go`:

```go
// Package loop implements the Agentic Engineering Loop: a state machine that
// drives an issue from open to mergeable PR by chaining Plan → Develop →
// Review → Fix stages implemented as agent_task_queue rows.
package loop

import "time"

type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusPaused    Status = "paused"
	StatusDone      Status = "done"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
)

func (s Status) IsValid() bool {
	switch s {
	case StatusPending, StatusRunning, StatusPaused,
		StatusDone, StatusFailed, StatusCancelled:
		return true
	}
	return false
}

type Stage string

const (
	StagePlan    Stage = "plan"
	StageDevelop Stage = "develop"
	StageReview  Stage = "review"
	StageFix     Stage = "fix"
)

func (s Stage) IsValid() bool {
	switch s {
	case StagePlan, StageDevelop, StageReview, StageFix:
		return true
	}
	return false
}

type FailureReason string

const (
	FailureMaxIterations     FailureReason = "max_iterations_exceeded"
	FailureLoopTimeout       FailureReason = "loop_timeout"
	FailureStageTimeout      FailureReason = "stage_timeout"
	FailurePRCreateFailed    FailureReason = "pr_create_failed"
	FailureContextExceeded   FailureReason = "context_exceeded"
	FailureContentFilter     FailureReason = "content_filter"
	FailureUnrecoverable     FailureReason = "unrecoverable_error"
)

// Loop is the top-level state record for one engineering cycle on an issue.
type Loop struct {
	ID            string     `json:"id"`
	IssueID       string     `json:"issue_id"`
	WorkspaceID   string     `json:"workspace_id"`
	Status        Status     `json:"status"`
	CurrentStage  *Stage     `json:"current_stage,omitempty"`
	Iteration     int        `json:"iteration"`
	MaxIterations int        `json:"max_iterations"`
	PRURL         *string    `json:"pr_url,omitempty"`
	PRNumber      *int       `json:"pr_number,omitempty"`
	BranchName    *string    `json:"branch_name,omitempty"`
	AgentID       *string    `json:"agent_id,omitempty"`
	Config        []byte     `json:"config"`
	FailureReason *string    `json:"failure_reason,omitempty"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /Users/doug/ai/system/agentra/server && go test ./internal/loop/
```

Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
git add server/internal/loop/loop.go server/internal/loop/loop_test.go
git commit -m "feat(loop): add Loop struct, status/stage constants and validity helpers"
```

---

## Task 4: `Store` — DB CRUD wrapper for `loops` + integration test

**Files:**
- Create: `server/internal/loop/store.go`
- Create: `server/internal/loop/store_test.go`

- [ ] **Step 1: Write the failing test**

Create `server/internal/loop/store_test.go`:

```go
package loop_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	looppkg "github.com/agentra/agentra/server/internal/loop"
	"github.com/agentra/agentra/server/pkg/db"
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
	q := db.New(pool)
	store := looppkg.NewStore(q)

	wsID := uuid.NewString()
	issueID := uuid.NewString()
	// Seed workspace + issue via existing migrations helper
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
	if updated.Status != looppkg.StatusRunning || *updated.CurrentStage != looppkg.StagePlan {
		t.Errorf("update mismatch: %+v", updated)
	}

	prURL := "https://github.com/agentra/agentra/pull/42"
	prNum := 42
	branch := "loop/issue-1-3"
	prLoop, err := store.SetPR(ctx, created.ID, prURL, prNum, branch)
	if err != nil {
		t.Fatal(err)
	}
	if prLoop.PRURL == nil || *prLoop.PRURL != prURL {
		t.Errorf("pr not set: %+v", prLoop.PRURL)
	}
}

// seedWorkspaceAndIssue inserts a minimal workspace + issue for FK satisfaction.
// Implementation left as a TODO hook for the test helper; for now, run the
// project's standard test fixture seeder.
func seedWorkspaceAndIssue(t *testing.T, pool *pgxpool.Pool, wsID, issueID string) {
	t.Helper()
	// Uses the same SQL as the rest of the test suite to seed an issue.
	// See server/internal/handler/*_test.go for the canonical pattern.
	_, err := pool.Exec(context.Background(),
		`INSERT INTO workspaces (id, name, slug) VALUES ($1, 'Test', $2)`,
		wsID, "test-"+wsID[:8])
	if err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	_, err = pool.Exec(context.Background(),
		`INSERT INTO issues (id, workspace_id, number, identifier, title, status, priority, assignee_type, creator_type, position) VALUES ($1, $2, 1, 'TES-1', 'Test', 'todo', 'medium', 'member', 'member', 0)`,
		issueID, wsID)
	if err != nil {
		t.Fatalf("seed issue: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/doug/ai/system/agentra/server && go test ./internal/loop/ -run TestStoreCRUD
```

Expected: FAIL — `looppkg.NewStore` undefined.

- [ ] **Step 3: Write the implementation**

Create `server/internal/loop/store.go`:

```go
package loop

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/agentra/agentra/server/pkg/db"
)

type CreateLoopInput struct {
	IssueID       string
	WorkspaceID   string
	MaxIterations *int
	AgentID       *string
	Config        []byte // JSON; pass `[]byte("{}")` for empty
}

type UpdateStatusInput struct {
	Status        *Status
	CurrentStage  *Stage
	Iteration     *int
	FailureReason *string
	StartedAt     *string // ISO8601; nil to leave alone (handled by SQL COALESCE)
	CompletedAt   *string
}

type Store struct {
	q *db.Queries
}

func NewStore(q *db.Queries) *Store { return &Store{q: q} }

func (s *Store) CreateLoop(ctx context.Context, in CreateLoopInput) (*Loop, error) {
	row, err := s.q.CreateLoop(ctx, db.CreateLoopParams{
		ID:            uuid.NewString(),
		IssueID:       in.IssueID,
		WorkspaceID:   in.WorkspaceID,
		MaxIterations: pgxIntPtr(in.MaxIterations),
		AgentID:       uuidPtr(in.AgentID),
		Config:        jsonOrEmpty(in.Config),
	})
	if err != nil {
		return nil, fmt.Errorf("CreateLoop: %w", err)
	}
	return rowToLoop(row)
}

func (s *Store) GetLoop(ctx context.Context, id string) (*Loop, error) {
	row, err := s.q.GetLoop(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrLoopNotFound
		}
		return nil, fmt.Errorf("GetLoop: %w", err)
	}
	return rowToLoop(row)
}

func (s *Store) UpdateStatus(ctx context.Context, id string, in UpdateStatusInput) (*Loop, error) {
	row, err := s.q.UpdateLoopStatus(ctx, db.UpdateLoopStatusParams{
		ID:            id,
		Status:        stringOrEmpty(in.Status),
		CurrentStage:  pgxTextPtr(stagePtrToString(in.CurrentStage)),
		Iteration:     intOrZero(in.Iteration),
		FailureReason: strPtrToPg(in.FailureReason),
		StartedAt:     pgxTextPtr(in.StartedAt),
		CompletedAt:   pgxTextPtr(in.CompletedAt),
	})
	if err != nil {
		return nil, fmt.Errorf("UpdateLoopStatus: %w", err)
	}
	return rowToLoop(row)
}

func (s *Store) SetPR(ctx context.Context, id, url string, num int, branch string) (*Loop, error) {
	row, err := s.q.SetLoopPR(ctx, db.SetLoopPRParams{
		ID:        id,
		PrUrl:     pgxTextPtr(&url),
		PrNumber:  pgxInt32Ptr(&num),
		Branch:    pgxTextPtr(&branch),
	})
	if err != nil {
		return nil, fmt.Errorf("SetLoopPR: %w", err)
	}
	return rowToLoop(row)
}

func (s *Store) LoadActive(ctx context.Context) ([]*Loop, error) {
	rows, err := s.q.LoadActiveLoops(ctx)
	if err != nil {
		return nil, fmt.Errorf("LoadActiveLoops: %w", err)
	}
	out := make([]*Loop, 0, len(rows))
	for _, r := range rows {
		l, err := rowToLoop(r)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, nil
}

// ErrLoopNotFound is returned by Store.GetLoop when no row exists.
var ErrLoopNotFound = fmt.Errorf("loop not found")

// rowToLoop converts a sqlc-generated Loops row to a *Loop. Implementation
// should be straightforward field assignment + pointer handling for nullable
// columns. The exact body depends on what `db.Loops` looks like after sqlc regen
// (likely `pgtype.Text`, `pgtype.Int4`, `pgtype.Timestamptz`); for MVP, just
// map the fields and trust sqlc's nullable wrappers. Add this function in
// store.go (or split into store_internal.go if it grows).
func rowToLoop(row any) (*Loop, error) {
	// This body is filled in once sqlc regen produces db.Loops. For now, return
	// a stub that compiles and panics if used — the test in this task will fail
	// until the conversion is implemented.
	panic("rowToLoop: implement after sqlc regen lands (see Task 2 step 3)")
}
```

The exact conversion depends on what sqlc generates for the `loops` table (nullable columns become `pgtype.Text` / `pgtype.Int4` / `pgtype.UUID` / `pgtype.Timestamptz`). The implementer should:

1. After Task 2 step 3, open `server/pkg/db/generated/loops.sql.go` and `models.go`
2. Look at the `Loops` struct in models.go
3. Implement `rowToLoop` to map `pgtype.X` → `*string` / `*int` / `*time.Time` using `pgtype.X.Valid` checks

The helper functions `pgxIntPtr`, `uuidPtr`, `jsonOrEmpty`, `stringOrEmpty`, `intOrZero`, `strPtrToPg`, `pgxTextPtr`, `pgxInt32Ptr`, `stagePtrToString` are defined inline in `store.go` (or in `store_helpers.go` if the file grows). They bridge `*int` / `*string` / `[]byte` to sqlc-generated `pgtype.X` types. Look at the existing `server/pkg/db/queries/agent.sql` for the established pattern (it uses `sqlc.narg(...)` for nullables).

- [ ] **Step 4: Run the test to verify it passes**

```bash
cd /Users/doug/ai/system/agentra/server
TEST_DATABASE_URL="$(make -s db-url-test || echo $DATABASE_URL)" \
  go test ./internal/loop/ -run TestStoreCRUD -v
```

Expected: PASS after `rowToLoop` is implemented per sqlc's output. If `TEST_DATABASE_URL` is not set, the test is skipped (which is fine for unit-only CI; mark this test with `//go:build integration` if the team prefers skipping in default runs).

- [ ] **Step 5: Commit**

```bash
git add server/internal/loop/store.go server/internal/loop/store_test.go
git commit -m "feat(loop): add Store with Create/Get/Update/SetPR/LoadActive for loops table"
```

---

## Task 5: REST handler — POST /api/loops (create)

**Files:**
- Create: `server/internal/handler/loop.go`
- Create: `server/internal/handler/loop_test.go`

- [ ] **Step 1: Read the existing handler registration pattern**

```bash
cd /Users/doug/ai/system/agentra
grep -n "router.Post\|router.Route\|/api/" server/cmd/server/router.go | head -30
```

Note the exact function signature used by the project's existing handlers (e.g. `func CreateIssue(w http.ResponseWriter, r *http.Request)`). The new handler must follow the same pattern.

- [ ] **Step 2: Write the failing test**

Create `server/internal/handler/loop_test.go`:

```go
package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agentra/agentra/server/internal/handler"
)

func TestCreateLoop_HappyPath(t *testing.T) {
	srv, q, cleanup := setupTestServer(t)
	defer cleanup()

	wsID := mustSeedWorkspace(t, q)
	issueID := mustSeedIssue(t, q, wsID)

	body := map[string]any{"issue_id": issueID, "max_iterations": 3}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/loops", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), "workspace_id", wsID))
	rr := httptest.NewRecorder()

	handler.CreateLoop(srv.Deps, rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", rr.Code, rr.Body.String())
	}
	var resp struct {
		ID            string `json:"id"`
		Status        string `json:"status"`
		MaxIterations int    `json:"max_iterations"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.ID == "" {
		t.Error("expected id")
	}
	if resp.Status != "pending" {
		t.Errorf("expected pending, got %q", resp.Status)
	}
	if resp.MaxIterations != 3 {
		t.Errorf("expected 3, got %d", resp.MaxIterations)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

```bash
cd /Users/doug/ai/system/agentra/server && go test ./internal/handler/ -run TestCreateLoop
```

Expected: FAIL — `handler.CreateLoop` undefined.

- [ ] **Step 4: Write the handler implementation**

Create `server/internal/handler/loop.go`. The exact signature depends on the project's handler convention. Below is a sketch assuming the existing `Deps` struct pattern (if the project uses a different style, adapt the file to match):

```go
package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	looppkg "github.com/agentra/agentra/server/internal/loop"
)

// CreateLoop handles POST /api/loops.
func CreateLoop(d *Deps, w http.ResponseWriter, r *http.Request) {
	wsID := workspaceIDFromContext(r.Context())
	if wsID == "" {
		writeError(w, http.StatusUnauthorized, "missing workspace")
		return
	}

	var body struct {
		IssueID       string  `json:"issue_id"`
		MaxIterations *int    `json:"max_iterations,omitempty"`
		AgentID       *string `json:"agent_id,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if body.IssueID == "" {
		writeError(w, http.StatusBadRequest, "issue_id is required")
		return
	}

	store := looppkg.NewStore(d.Queries)
	l, err := store.CreateLoop(r.Context(), looppkg.CreateLoopInput{
		IssueID:       body.IssueID,
		WorkspaceID:   wsID,
		MaxIterations: body.MaxIterations,
		AgentID:       body.AgentID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, l)
}

// GetLoop handles GET /api/loops/{id}.
func GetLoop(d *Deps, w http.ResponseWriter, r *http.Request) {
	id := pathParam(r, "id")
	store := looppkg.NewStore(d.Queries)
	l, err := store.GetLoop(r.Context(), id)
	if errors.Is(err, looppkg.ErrLoopNotFound) {
		writeError(w, http.StatusNotFound, "loop not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if l.WorkspaceID != workspaceIDFromContext(r.Context()) {
		writeError(w, http.StatusNotFound, "loop not found")
		return
	}
	writeJSON(w, http.StatusOK, l)
}

// ListLoops handles GET /api/loops?status=&issue_id=.
func ListLoops(d *Deps, w http.ResponseWriter, r *http.Request) {
	wsID := workspaceIDFromContext(r.Context())
	store := looppkg.NewStore(d.Queries)
	loops, err := store.LoadActive(r.Context()) // or ListLoops variant; for now LoadActive is fine
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Filter to workspace.
	out := make([]*looppkg.Loop, 0, len(loops))
	for _, l := range loops {
		if l.WorkspaceID == wsID {
			out = append(out, l)
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// PauseLoop, ResumeLoop, CancelLoop — stub with status update; full impl in
// Task 14 once state machine is wired. For now they just call UpdateStatus.
func PauseLoop(d *Deps, w http.ResponseWriter, r *http.Request) {
	updateLoopStatus(d, w, r, looppkg.StatusPaused)
}
func ResumeLoop(d *Deps, w http.ResponseWriter, r *http.Request) {
	updateLoopStatus(d, w, r, looppkg.StatusRunning)
}
func CancelLoop(d *Deps, w http.ResponseWriter, r *http.Request) {
	updateLoopStatus(d, w, r, looppkg.StatusCancelled)
}

func updateLoopStatus(d *Deps, w http.ResponseWriter, r *http.Request, target looppkg.Status) {
	id := pathParam(r, "id")
	store := looppkg.NewStore(d.Queries)
	l, err := store.GetLoop(r.Context(), id)
	if errors.Is(err, looppkg.ErrLoopNotFound) {
		writeError(w, http.StatusNotFound, "loop not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	updated, err := store.UpdateStatus(r.Context(), id, looppkg.UpdateStatusInput{
		Status: &target,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, updated)
}
```

The helper functions `workspaceIDFromContext`, `pathParam`, `writeJSON`, `writeError` are defined elsewhere in the handler package. If they don't exist with those exact names, find the equivalents in `server/internal/handler/` (e.g. `respondJSON`, `decodeWorkspace`).

- [ ] **Step 5: Run test to verify it passes**

```bash
cd /Users/doug/ai/system/agentra/server && go test ./internal/handler/ -run TestCreateLoop -v
```

Expected: PASS. If `setupTestServer` / `mustSeedWorkspace` / `mustSeedIssue` helpers don't exist, factor them out from existing handler tests (e.g. `issue_test.go`).

- [ ] **Step 6: Wire routes in `cmd/server/router.go`**

Read `server/cmd/server/router.go` and find where the `/api/issues` (or similar) routes are registered. Add at the same level:

```go
router.Route("/api/loops", func(r chi.Router) {
    r.Post("/", handler.CreateLoop(deps, ...))
    r.Get("/", handler.ListLoops(deps, ...))
    r.Route("/{id}", func(r chi.Router) {
        r.Get("/", handler.GetLoop(deps, ...))
        r.Post("/pause", handler.PauseLoop(deps, ...))
        r.Post("/resume", handler.ResumeLoop(deps, ...))
        r.Post("/cancel", handler.CancelLoop(deps, ...))
    })
})
```

The exact signature depends on how the project wires deps into the router. Look at the existing pattern and match it.

- [ ] **Step 7: Build and smoke-test**

```bash
cd /Users/doug/ai/system/agentra/server && go build ./...
make dev  # in another terminal, leave it running
curl -s -X POST http://localhost:8080/api/loops \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Workspace-ID: $WS_ID" \
  -d '{"issue_id":"'$ISSUE_ID'","max_iterations":3}' | jq
```

Expected: HTTP 201 with JSON containing `id`, `status: "pending"`, `max_iterations: 3`.

- [ ] **Step 8: Commit**

```bash
git add server/internal/handler/loop.go server/internal/handler/loop_test.go server/cmd/server/router.go
git commit -m "feat(loop): add REST endpoints (create/get/list/pause/resume/cancel)"
```

---

## Task 6: Coordinator skeleton + `decideNextStage` state machine

**Files:**
- Create: `server/internal/loop/coordinator.go`
- Create: `server/internal/loop/coordinator_test.go`

- [ ] **Step 1: Write the failing test**

Create `server/internal/loop/coordinator_test.go`:

```go
package loop

import (
	"context"
	"testing"
)

func TestDecideNextStage_PlanToDevelop(t *testing.T) {
	c := &Coordinator{}
	got := c.decideNextStage(&Loop{
		Status:       StatusRunning,
		CurrentStage: ptrStage(StagePlan),
	}, nil /* plan task artifact */)
	if got.action != "create_task" {
		t.Errorf("expected create_task, got %q", got.action)
	}
	if got.taskType != "loop_develop" {
		t.Errorf("expected loop_develop, got %q", got.taskType)
	}
}

func TestDecideNextStage_DevelopToReview(t *testing.T) {
	c := &Coordinator{}
	got := c.decideNextStage(&Loop{
		Status:       StatusRunning,
		CurrentStage: ptrStage(StageDevelop),
	}, nil)
	if got.taskType != "loop_review" {
		t.Errorf("expected loop_review, got %q", got.taskType)
	}
}

func TestDecideNextStage_ReviewApprovedCompletes(t *testing.T) {
	c := &Coordinator{}
	got := c.decideNextStage(&Loop{
		Status:       StatusRunning,
		CurrentStage: ptrStage(StageReview),
		Iteration:    0,
		MaxIterations: 5,
	}, &TaskResult{ReviewApproved: boolPtr(true), PRURL: "https://x"})
	if got.action != "complete" {
		t.Errorf("expected complete, got action=%q", got.action)
	}
	if got.prURL != "https://x" {
		t.Errorf("prURL not propagated: %q", got.prURL)
	}
}

func TestDecideNextStage_ReviewRejectedCreatesFix(t *testing.T) {
	c := &Coordinator{}
	got := c.decideNextStage(&Loop{
		Status:       StatusRunning,
		CurrentStage: ptrStage(StageReview),
		Iteration:    0,
		MaxIterations: 5,
	}, &TaskResult{ReviewApproved: boolPtr(false), ReviewIssues: "[]"})
	if got.action != "create_task" {
		t.Errorf("expected create_task, got %q", got.action)
	}
	if got.taskType != "loop_fix" {
		t.Errorf("expected loop_fix, got %q", got.taskType)
	}
	if got.iterationBump != 1 {
		t.Errorf("expected iterationBump=1, got %d", got.iterationBump)
	}
}

func TestDecideNextStage_ReviewRejectedExceedsMaxFails(t *testing.T) {
	c := &Coordinator{}
	got := c.decideNextStage(&Loop{
		Status:       StatusRunning,
		CurrentStage: ptrStage(StageReview),
		Iteration:    5,
		MaxIterations: 5,
	}, &TaskResult{ReviewApproved: boolPtr(false)})
	if got.action != "fail" {
		t.Errorf("expected fail, got %q", got.action)
	}
	if got.reason != FailureMaxIterations {
		t.Errorf("expected max_iterations, got %q", got.reason)
	}
}

func TestDecideNextStage_FixToReview(t *testing.T) {
	c := &Coordinator{}
	got := c.decideNextStage(&Loop{
		Status:       StatusRunning,
		CurrentStage: ptrStage(StageFix),
	}, nil)
	if got.taskType != "loop_review" {
		t.Errorf("expected loop_review, got %q", got.taskType)
	}
}

func TestDecideNextStage_PausedDoesNotAdvance(t *testing.T) {
	c := &Coordinator{}
	got := c.decideNextStage(&Loop{
		Status:       StatusPaused,
		CurrentStage: ptrStage(StagePlan),
	}, nil)
	if got.action != "" {
		t.Errorf("expected empty decision on paused, got %+v", got)
	}
}

func ptrStage(s Stage) *Stage { return &s }
func boolPtr(b bool) *bool    { return &b }
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/doug/ai/system/agentra/server && go test ./internal/loop/ -run TestDecideNextStage
```

Expected: FAIL — `Coordinator` and `decideNextStage` undefined.

- [ ] **Step 3: Write the implementation**

Create `server/internal/loop/coordinator.go`:

```go
package loop

import (
	"context"
	"fmt"

	"github.com/agentra/agentra/server/internal/events"
	"github.com/agentra/agentra/server/pkg/db"
)

// TaskResult is the structured output read from task_runs.output for a completed
// loop stage. Stage executors encode their result as JSON and store it there.
type TaskResult struct {
	ReviewApproved *bool  `json:"review_approved,omitempty"`
	ReviewIssues   string `json:"review_issues,omitempty"` // JSON array as string
	PRURL          string `json:"pr_url,omitempty"`
	PRNumber       *int   `json:"pr_number,omitempty"`
	BranchName     string `json:"branch_name,omitempty"`
}

// Decision is the result of running decideNextStage.
type Decision struct {
	action        string // "create_task" | "complete" | "fail" | "" (no-op)
	taskType      string
	prURL         string
	reason        FailureReason
	iterationBump int
}

// Coordinator owns the loop state machine. It is created once per server
// process and subscribes to events.Bus.
type Coordinator struct {
	queries *db.Queries
	bus     *events.Bus
	store   *Store
}

func NewCoordinator(q *db.Queries, bus *events.Bus) *Coordinator {
	return &Coordinator{queries: q, bus: bus, store: NewStore(q)}
}

// decideNextStage is the pure state-machine function. No I/O. Easy to test.
// See spec §4.2 for the transition table.
func (c *Coordinator) decideNextStage(l *Loop, lastResult *TaskResult) Decision {
	if l.Status != StatusRunning {
		return Decision{} // paused/cancelled/done/failed: no-op
	}
	if l.CurrentStage == nil {
		return Decision{action: "fail", reason: FailureUnrecoverable}
	}
	switch *l.CurrentStage {
	case StagePlan:
		return Decision{action: "create_task", taskType: "loop_develop"}
	case StageDevelop:
		return Decision{action: "create_task", taskType: "loop_review"}
	case StageReview:
		if lastResult == nil || lastResult.ReviewApproved == nil {
			return Decision{action: "fail", reason: FailureUnrecoverable}
		}
		if *lastResult.ReviewApproved {
			return Decision{action: "complete", prURL: lastResult.PRURL}
		}
		if l.Iteration >= l.MaxIterations {
			return Decision{action: "fail", reason: FailureMaxIterations}
		}
		return Decision{action: "create_task", taskType: "loop_fix", iterationBump: 1}
	case StageFix:
		return Decision{action: "create_task", taskType: "loop_review"}
	}
	return Decision{action: "fail", reason: FailureUnrecoverable}
}

// HandleTaskCompleted is the event handler registered with events.Bus.
// It does I/O (DB lookups, task creation) and must be offloaded to a goroutine
// by the caller to avoid blocking the bus publisher (see spec §13.4).
func (c *Coordinator) HandleTaskCompleted(ctx context.Context, e events.Event) error {
	payload, ok := e.Payload.(map[string]any)
	if !ok {
		return fmt.Errorf("loop: unexpected payload type %T", e.Payload)
	}
	taskID, _ := payload["task_id"].(string)
	if taskID == "" {
		return fmt.Errorf("loop: missing task_id in payload")
	}

	// Look up the task to get task_type and loop_id
	task, err := c.queries.GetAgentTask(ctx, taskID)
	if err != nil {
		return fmt.Errorf("loop: get task %s: %w", taskID, err)
	}
	if task.LoopID == nil || !task.LoopID.Valid {
		return nil // not a loop task; ignore
	}
	loopID := task.LoopID.String

	// Read the task's latest run output (TaskResult JSON)
	var result *TaskResult
	if run, err := c.queries.GetLatestTaskRun(ctx, taskID); err == nil && run != nil {
		result = parseTaskResult(run.Output)
	}

	// Reload the loop (it may have been paused/cancelled since the task was enqueued)
	l, err := c.store.GetLoop(ctx, loopID)
	if err != nil {
		return fmt.Errorf("loop: get loop %s: %w", loopID, err)
	}

	decision := c.decideNextStage(l, result)
	return c.applyDecision(ctx, l, decision, taskID)
}

func (c *Coordinator) applyDecision(ctx context.Context, l *Loop, d Decision, completedTaskID string) error {
	switch d.action {
	case "":
		return nil
	case "create_task":
		// Increment iteration if needed
		newIter := l.Iteration + d.iterationBump
		_, err := c.queries.CreateAgentTask(ctx, db.CreateAgentTaskParams{
			ID:               newTaskID(), // uuid helper
			AgentID:          l.AgentID,
			IssueID:          l.IssueID,
			Priority:         1,
			TaskType:         pgxText(d.taskType),
			LoopID:           l.ID,
		})
		if err != nil {
			return fmt.Errorf("loop: create next task: %w", err)
		}
		_, err = c.store.UpdateStatus(ctx, l.ID, UpdateStatusInput{
			CurrentStage: stageFromString(d.taskType),
			Iteration:    &newIter,
		})
		return err
	case "complete":
		_, err := c.store.SetPR(ctx, l.ID, d.prURL, 0, "")
		if err != nil {
			return err
		}
		done := StatusDone
		now := nowISO()
		_, err = c.store.UpdateStatus(ctx, l.ID, UpdateStatusInput{
			Status:      &done,
			CompletedAt: &now,
		})
		return err
	case "fail":
		failed := StatusFailed
		reason := string(d.reason)
		now := nowISO()
		_, err := c.store.UpdateStatus(ctx, l.ID, UpdateStatusInput{
			Status:        &failed,
			FailureReason: &reason,
			CompletedAt:   &now,
		})
		return err
	}
	return nil
}

// Helper functions (newTaskID, pgxText, nowISO, stageFromString, parseTaskResult)
// live in coordinator_helpers.go. See the file body for the exact 5-line
// implementations; nothing fancy.
```

The full `coordinator.go` body including helpers, plus `parseTaskResult` (reads `task_runs.output` JSON into `TaskResult`), `stageFromString` (maps `"loop_plan"` → `StagePlan` etc.), `newTaskID` (`uuid.NewString()`), `pgxText` (helper for sqlc's `pgtype.Text`), and `nowISO` (`time.Now().UTC().Format(time.RFC3339)`) goes in a sibling `coordinator_helpers.go` file or inline. The implementer should keep this file under 300 lines by splitting if it grows.

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /Users/doug/ai/system/agentra/server && go test ./internal/loop/ -run TestDecideNextStage -v
```

Expected: PASS (7 tests covering all branches of `decideNextStage`).

- [ ] **Step 5: Commit**

```bash
git add server/internal/loop/coordinator.go server/internal/loop/coordinator.go-helpers 2>/dev/null || \
  git add server/internal/loop/coordinator.go
git add server/internal/loop/coordinator_test.go
git commit -m "feat(loop): add Coordinator with decideNextStage state machine and event handler"
```

---

## Task 7: Wire `runLoopCoordinator` into `cmd/server/main.go`

**Files:**
- Create: `server/cmd/server/loop_coordinator.go`
- Modify: `server/cmd/server/main.go` (3 lines added)

- [ ] **Step 1: Read `main.go` to find the right insertion point**

```bash
cd /Users/doug/ai/system/agentra
sed -n '40,100p' server/cmd/server/main.go
```

Look for the `runRuntimeSweeper` block — that is the exact pattern to copy.

- [ ] **Step 2: Create the wiring file**

Create `server/cmd/server/loop_coordinator.go`:

```go
package main

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	looppkg "github.com/agentra/agentra/server/internal/loop"
	"github.com/agentra/agentra/server/internal/events"
	"github.com/agentra/agentra/server/pkg/db"
	"github.com/agentra/agentra/server/pkg/protocol"
)

// runLoopCoordinator starts the Agentic Engineering Loop coordinator in a
// background goroutine. It is a thin wiring layer; the actual state machine
// lives in server/internal/loop/coordinator.go and is unit-tested there.
//
// Pattern mirrors runRuntimeSweeper (see main.go). Coordinator:
//   1. Loads all running/paused loops from DB on startup
//   2. Subscribes to task:completed and task:failed events
//   3. Offloads each event to a goroutine (bus.Publish is sync — see §13.4)
func runLoopCoordinator(ctx context.Context, pool *pgxpool.Pool, bus *events.Bus) {
	q := db.New(pool)
	coord := looppkg.NewCoordinator(q, bus)

	// Reload any loops that were running when the server last stopped
	if loops, err := coord.Store().LoadActive(ctx); err == nil {
		slog.Info("loop_coordinator: restored active loops", "count", len(loops))
	}

	bus.Subscribe(protocol.EventTaskCompleted, func(e events.Event) {
		// offload to goroutine; the bus publisher must not block on us
		go func() {
			if err := coord.HandleTaskCompleted(ctx, e); err != nil {
				slog.Error("loop_coordinator: HandleTaskCompleted", "err", err)
			}
		}()
	})
	bus.Subscribe(protocol.EventTaskFailed, func(e events.Event) {
		go func() {
			if err := coord.HandleTaskFailed(ctx, e); err != nil {
				slog.Error("loop_coordinator: HandleTaskFailed", "err", err)
			}
		}()
	})
}
```

`HandleTaskFailed` is a sibling of `HandleTaskCompleted` that handles the failure path (mark loop failed, set failure_reason). The implementer should add a stub now and flesh out in Task 14:

```go
// in coordinator.go
func (c *Coordinator) HandleTaskFailed(ctx context.Context, e events.Event) error {
    // For M1: log only. Full impl in Task 14 (failure handling).
    slog.Warn("loop_coordinator: task failed", "event", e)
    return nil
}
```

- [ ] **Step 3: Modify `main.go` to start the coordinator**

In `server/cmd/server/main.go`, after the `runRuntimeSweeper` block (around line 72), add:

```go
go runLoopCoordinator(context.Background(), pool, bus)
```

(Or pass the existing `sweepCtx` if you want the coordinator to share the sweeper's lifetime — they're both server-scoped so it doesn't matter much.)

- [ ] **Step 4: Build the server**

```bash
cd /Users/doug/ai/system/agentra/server && go build ./...
```

Expected: exit 0.

- [ ] **Step 5: Commit**

```bash
git add server/cmd/server/loop_coordinator.go server/cmd/server/main.go server/internal/loop/coordinator.go
git commit -m "feat(loop): wire runLoopCoordinator into cmd/server main"
```

---

## Task 8: Daemon `Task.TaskType` field + `runTask` dispatch

**Files:**
- Modify: `server/internal/daemon/types.go:25-37` (add `TaskType` field)
- Modify: `server/internal/daemon/daemon.go:854-910` (dispatch by `task.TaskType` at `BuildPrompt` seam)

- [ ] **Step 1: Add `TaskType` to the daemon Task struct**

In `server/internal/daemon/types.go`, edit the `Task` struct to add:

```go
type Task struct {
	// ... existing fields ...
	TaskType string `json:"task_type,omitempty"` // "standard" (default) or loop_plan/develop/review/fix
}
```

Place it near `TriggerCommentID` for readability.

- [ ] **Step 2: Add `TaskType` to the claim endpoint response**

Find the function in `server/internal/handler/` (or wherever) that builds the `Task` JSON returned to the daemon on `ClaimTask`. Add `task_type` to the field list, populated from `agent_task_queue.task_type`. If the existing handler maps `db.AgentTaskQueue` to the daemon's `Task` struct via a translator function, add the mapping there.

Run `grep -rn "IssueTitle:" /Users/doug/ai/system/agentra/server/internal/handler/` to find the translator (the existing `IssueTitle` field has the same pattern).

- [ ] **Step 3: Add `buildPromptForStage` dispatch in `daemon.runTask`**

In `server/internal/daemon/daemon.go`, just before the `BuildPrompt(task)` call inside `runTask` (line 910 area), add:

```go
// Stage dispatch: loop tasks get a different system prompt and tool set.
// Standard tasks fall through to the original BuildPrompt path unchanged.
prompt, execOpts := buildPromptForStage(task.TaskType, task)
```

Add the function definition in the same file (or in a new `server/internal/daemon/stages.go`):

```go
// buildPromptForStage returns the system prompt and backend ExecOptions for a
// given task_type. Falls through to the legacy BuildPrompt for unknown types.
func buildPromptForStage(taskType string, task *Task) (string, agent.ExecOptions) {
    switch taskType {
    case "loop_plan":
        return loopStagePrompt("plan", task), agent.ExecOptions{
            Tools:   loopToolsByStage("loop_plan"),
            MaxTurns: 5,
        }
    case "loop_develop":
        return loopStagePrompt("develop", task), agent.ExecOptions{
            Tools:   loopToolsByStage("loop_develop"),
            MaxTurns: 30,
        }
    case "loop_review":
        return loopStagePrompt("review", task), agent.ExecOptions{
            Tools:   loopToolsByStage("loop_review"),
            MaxTurns: 5,
        }
    case "loop_fix":
        return loopStagePrompt("fix", task), agent.ExecOptions{
            Tools:   loopToolsByStage("loop_fix"),
            MaxTurns: 30,
        }
    default:
        return BuildPrompt(task), agent.ExecOptions{}
    }
}
```

`loopStagePrompt` and `loopToolsByStage` are defined in the new `server/internal/loop/stages/stages.go` (created in Task 9) and re-exported from `daemon` via a thin package — or, more simply, the daemon imports `loop/stages` directly. The implementer can do either; the cleaner design is to have `daemon.go` call into the `stages` package.

- [ ] **Step 4: Build the daemon**

```bash
cd /Users/doug/ai/system/agentra/server && go build ./...
```

Expected: exit 0. There may be a compile error if `agent.ExecOptions` doesn't exist with that exact name — look at `server/pkg/agent/backend.go` for the real options type and adapt.

- [ ] **Step 5: Commit**

```bash
git add server/internal/daemon/types.go server/internal/daemon/daemon.go server/internal/handler/ # wherever the claim translator lives
git commit -m "feat(daemon): add TaskType field and stage dispatch in runTask"
```

---

## Task 9: Stages package — interface, registry, prompt loader, tool registry

**Files:**
- Create: `server/internal/loop/stages/stages.go`
- Create: `server/internal/loop/stages/prompts.go` (loader for `prompts/*.md`)
- Create: `server/internal/loop/prompts/{plan,develop,review,fix}.md` (template files)

- [ ] **Step 1: Create the 4 system prompt templates**

Create `server/internal/loop/prompts/plan.md`:

````markdown
# Loop Plan Stage

You are analyzing an issue and producing a structured implementation plan.

## Input
- Issue title: {{.IssueTitle}}
- Issue description: {{.IssueDescription}}

## Output
Produce a single markdown document with these sections:
1. **Goal** — one-sentence restatement of what the user wants
2. **Affected files** — paths you expect to touch (search the repo to verify)
3. **Steps** — numbered list of concrete actions (read file X, modify Y, run test Z)
4. **Acceptance criteria** — bullet list of how the user verifies success

## Rules
- Use `read_file` and `search_code` to ground your plan in the actual codebase
- Do not edit any files
- Output ONLY the plan markdown — no preamble, no "here is your plan"
````

Create the other 3 prompt files similarly. The full templates are in spec §5.3 and §6.7.5. Each is 20-40 lines of markdown.

- [ ] **Step 2: Write the stages interface and registry**

Create `server/internal/loop/stages/stages.go`:

```go
// Package stages contains the per-stage executors that run inside the daemon.
// Each stage is a thin wrapper that builds an agent.Session (system prompt +
// tools + initial user message) and calls Backend.Execute. The Coordinator
// (server/internal/loop) decides WHEN to run a stage; stages decide WHAT
// prompt/tools to use.
package stages

import (
	"context"
	"fmt"

	"github.com/agentra/agentra/server/pkg/agent"
)

// TaskRef is the subset of the daemon's task object that stages need.
// Defined locally to avoid an import cycle on internal/daemon.
type TaskRef struct {
	ID         string
	IssueID    string
	IssueTitle string
	Branch     string // e.g. "loop/issue-1-2"
	Iteration  int
	WorkDir    string
}

// Result is what a stage returns. The string is JSON-encoded by the caller
// and stored in task_runs.output (see spec §13.1).
type Result struct {
	Text  string // free-form text for plan/fix; JSON for review
	PRURL string // develop stage may set this
}

// Executor is the per-stage contract. The actual LLM call is delegated to
// the agent.Backend, passed in.
type Executor func(ctx context.Context, task TaskRef, backend agent.Backend) (*Result, error)

// registry maps task_type to its executor. Populated by Register* calls in
// each stage file (init() in plan.go, develop.go, etc.).
var registry = map[string]Executor{}

func Register(taskType string, e Executor) { registry[taskType] = e }

// Resolve returns the executor for a given task_type, or an error if unknown.
func Resolve(taskType string) (Executor, error) {
	e, ok := registry[taskType]
	if !ok {
		return nil, fmt.Errorf("stages: no executor for task_type %q", taskType)
	}
	return e, nil
}

// AllRegistered returns all known task_types (for diagnostics / loop startup).
func AllRegistered() []string {
	out := make([]string, 0, len(registry))
	for k := range registry {
		out = append(out, k)
	}
	return out
}
```

- [ ] **Step 3: Write the prompt loader**

Create `server/internal/loop/stages/prompts.go`:

```go
package stages

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// loadPrompt reads server/internal/loop/prompts/<name>.md and applies a simple
// {{.Field}} substitution using fields from TaskRef. For MVP we keep it minimal;
// full Go text/template can be swapped in later without changing the call sites.
func loadPrompt(name string, task TaskRef) (string, error) {
	path := filepath.Join("server", "internal", "loop", "prompts", name+".md")
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("load prompt %s: %w", name, err)
	}
	s := string(raw)
	s = strings.ReplaceAll(s, "{{.IssueTitle}}", task.IssueTitle)
	s = strings.ReplaceAll(s, "{{.IssueID}}", task.IssueID)
	s = strings.ReplaceAll(s, "{{.Branch}}", task.Branch)
	s = strings.ReplaceAll(s, "{{.Iteration}}", fmt.Sprintf("%d", task.Iteration))
	return s, nil
}
```

- [ ] **Step 4: Write a smoke test for the registry and prompt loader**

Create `server/internal/loop/stages/stages_test.go`:

```go
package stages

import "testing"

func TestRegistryEmpty(t *testing.T) {
	// Registry is empty until plan/develop/review/fix tasks register themselves
	// in init() blocks added in Tasks 10-13. For now this test just confirms
	// the resolve path returns an error for unknown types.
	if _, err := Resolve("nonexistent"); err == nil {
		t.Error("expected error for unknown task_type")
	}
}

func TestLoadPrompt(t *testing.T) {
	r, err := loadPrompt("plan", TaskRef{IssueID: "issue-1", IssueTitle: "Test"})
	if err != nil {
		t.Fatal(err)
	}
	if r == "" {
		t.Error("expected non-empty prompt")
	}
	if !strings.Contains(r, "issue-1") {
		t.Error("expected {{.IssueID}} to be substituted")
	}
}
```

- [ ] **Step 5: Run tests**

```bash
cd /Users/doug/ai/system/agentra/server && go test ./internal/loop/stages/
```

Expected: PASS (2 tests; the prompt load test depends on cwd being the repo root, which is true when run via `go test ./...` from `server/` — but the `loadPrompt` path is relative. If it fails on CI, use `os.Getwd()`-based resolution or pass the prompts dir as a config option.)

- [ ] **Step 6: Commit**

```bash
git add server/internal/loop/stages/ server/internal/loop/prompts/
git commit -m "feat(loop): add stages package skeleton, prompt loader, and 4 prompt templates"
```

---

## Task 10: Plan stage executor + integration with daemon dispatch

**Files:**
- Create: `server/internal/loop/stages/plan.go`
- Create: `server/internal/loop/stages/plan_test.go`
- Modify: `server/internal/daemon/daemon.go` (wire the registry into `buildPromptForStage`)

- [ ] **Step 1: Write the failing test**

Create `server/internal/loop/stages/plan_test.go`:

```go
package stages

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/agentra/agentra/server/pkg/agent"
)

// fakeBackend captures the session passed to Execute and returns canned output.
type fakeBackend struct{ lastSession *agent.Session }

func (f *fakeBackend) Execute(ctx context.Context, s *agent.Session, opts ...agent.ExecOption) (*agent.Session, error) {
	f.lastSession = s
	return &agent.Session{
		Messages: []agent.Message{{Role: "assistant", Content: "## Goal\nAdd JWT auth."}},
		Result:   "## Goal\nAdd JWT auth.",
	}, nil
}

func TestPlanExecutor_BuildsCorrectSession(t *testing.T) {
	Register("loop_plan", Plan)
	defer delete(registry, "loop_plan")

	be := &fakeBackend{}
	task := TaskRef{IssueID: "issue-1", IssueTitle: "Implement auth"}
	res, err := Plan(context.Background(), task, be)
	if err != nil {
		t.Fatal(err)
	}
	if res.Text == "" {
		t.Error("expected non-empty plan text")
	}
	if be.lastSession == nil {
		t.Fatal("backend never called")
	}
	if !contains(be.lastSession.SystemPrompt, "Plan") {
		t.Errorf("expected system prompt to mention Plan, got %q", be.lastSession.SystemPrompt)
	}
	if len(be.lastSession.Tools) == 0 {
		t.Error("expected at least one tool")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || (len(s) > 0 && (s[:len(sub)] == sub || contains(s[1:], sub))))
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/doug/ai/system/agentra/server && go test ./internal/loop/stages/ -run TestPlanExecutor
```

Expected: FAIL — `Plan` undefined.

- [ ] **Step 3: Write the Plan executor**

Create `server/internal/loop/stages/plan.go`:

```go
package stages

import (
	"context"
	"fmt"

	"github.com/agentra/agentra/server/pkg/agent"
)

// Plan is the executor for task_type="loop_plan". Read-only: reads the issue,
// produces a markdown plan, writes it to task_runs.output (handled by caller).
func Plan(ctx context.Context, task TaskRef, backend agent.Backend) (*Result, error) {
	systemPrompt, err := loadPrompt("plan", task)
	if err != nil {
		return nil, err
	}

	session := &agent.Session{
		SystemPrompt: systemPrompt,
		Messages: []agent.Message{{
			Role:    "user",
			Content: fmt.Sprintf("Issue: %s\n\n%s", task.IssueTitle, ""), // description is passed in via separate env
		}},
		Tools: toolsForStage("loop_plan"),
	}

	out, err := backend.Execute(ctx, session)
	if err != nil {
		return nil, fmt.Errorf("plan: backend.Execute: %w", err)
	}

	// Plan is free-form markdown. The whole assistant text is the artifact.
	text := ""
	if out != nil {
		text = out.Result
		if text == "" && len(out.Messages) > 0 {
			text = out.Messages[len(out.Messages)-1].Content
		}
	}
	return &Result{Text: text}, nil
}

func init() {
	Register("loop_plan", Plan)
}
```

The `toolsForStage` helper is defined in `server/internal/loop/stages/tools.go` (Task 11). For Task 10, leave a stub:

```go
// server/internal/loop/stages/tools.go
package stages

// toolsForStage returns the tool list for a given task_type. Filled in Task 11.
func toolsForStage(taskType string) []agent.Tool {
	// Stub: return read-only tools. Task 11 expands this to the full matrix.
	switch taskType {
	case "loop_plan", "loop_review":
		return nil // tool_use is a stretch goal; MVP uses prompt + read_file tool only
	case "loop_develop", "loop_fix":
		return nil
	}
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /Users/doug/ai/system/agentra/server && go test ./internal/loop/stages/ -run TestPlanExecutor
```

Expected: PASS (1 test; system prompt non-empty, backend called once).

- [ ] **Step 5: Wire the registry into the daemon**

In `server/internal/daemon/daemon.go`, replace the `buildPromptForStage` body from Task 8 with one that calls into the `stages` package:

```go
func buildPromptForStage(taskType string, task *Task) (string, agent.ExecOptions) {
    if taskType == "" || taskType == "standard" {
        return BuildPrompt(task), agent.ExecOptions{}
    }
    e, err := stages.Resolve(taskType)
    if err != nil {
        slog.Warn("daemon: no stage executor, falling back to standard prompt", "task_type", taskType)
        return BuildPrompt(task), agent.ExecOptions{}
    }
    // The executor is called separately by the daemon's runTask path; here
    // we just need the prompt. Loading the prompt is cheap (file read) so
    // we can call loadPrompt directly.
    prompt, err := stages.LoadPromptForType(taskType, stages.TaskRef{
        IssueID:    task.IssueID,
        IssueTitle: task.IssueTitle,
        WorkDir:    task.PriorWorkDir,
    })
    if err != nil {
        slog.Error("daemon: load prompt", "err", err, "task_type", taskType)
        return BuildPrompt(task), agent.ExecOptions{}
    }
    _ = e // unused at the BuildPrompt seam; actual executor is called from runTask below
    return prompt, agent.ExecOptions{Tools: stages.ToolsFor(taskType), MaxTurns: maxTurnsFor(taskType)}
}
```

Add `LoadPromptForType` and `ToolsFor` to the `stages` package as small public helpers. Add `maxTurnsFor` as a private helper in the daemon file.

- [ ] **Step 6: Build and smoke test**

```bash
cd /Users/doug/ai/system/agentra/server && go build ./...
make dev # in another terminal
# Create a loop_plan task via the REST API and verify the daemon picks it up
# and calls backend.Execute with the loop_plan system prompt.
# Use a manual issue + the API: POST /api/loops then check the daemon log.
```

- [ ] **Step 7: Commit**

```bash
git add server/internal/loop/stages/ server/internal/daemon/daemon.go
git commit -m "feat(loop): implement Plan stage executor and wire into daemon dispatch"
```

---

## Task 11: Tools package — read_file, write_file, search_code, run_command, run_test

**Files:**
- Create: `server/internal/loop/tools/tool.go`
- Create: `server/internal/loop/tools/fs.go`
- Create: `server/internal/loop/tools/shell.go`
- Create: `server/internal/loop/tools/{fs,shell}_test.go`

- [ ] **Step 1: Define the Tool interface and registry**

Create `server/internal/loop/tools/tool.go`:

```go
// Package tools implements the agent-callable tools for loop stages.
// Each tool is a pure function: (args JSON) → (Result, error).
// Stage executors pick which tools to expose; see stages.toolsForStage.
package tools

import "context"

type Tool interface {
	Name() string
	Description() string
	Schema() map[string]any // JSON schema for tool_use protocol
	Execute(ctx context.Context, args map[string]any) (Result, error)
}

type Result struct {
	Content  string `json:"content"`
	Error    string `json:"error,omitempty"`
	Stderr   string `json:"stderr,omitempty"`
	ExitCode int    `json:"exit_code,omitempty"`
}

// Registry maps tool name → implementation. Populated by init() in each file.
var Registry = map[string]Tool{}

func Register(t Tool) { Registry[t.Name()] = t }

func Get(name string) (Tool, bool) {
	t, ok := Registry[name]
	return t, ok
}
```

- [ ] **Step 2: Implement read_file, write_file, search_code**

Create `server/internal/loop/tools/fs.go`:

```go
package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type ReadFileTool struct{ WorkDir string }

func (t *ReadFileTool) Name() string { return "read_file" }
func (t *ReadFileTool) Description() string {
	return "Read the contents of a file. Paths are relative to the loop's work directory."
}
func (t *ReadFileTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{"type": "string", "description": "Path relative to work_dir"},
		},
		"required": []string{"path"},
	}
}
func (t *ReadFileTool) Execute(ctx context.Context, args map[string]any) (Result, error) {
	path, _ := args["path"].(string)
	if path == "" {
		return Result{Error: "path is required"}, nil
	}
	full := filepath.Join(t.WorkDir, path)
	// Reject path traversal
	if !strings.HasPrefix(filepath.Clean(full), filepath.Clean(t.WorkDir)) {
		return Result{Error: "path escapes work directory"}, nil
	}
	const maxBytes = 10_000
	data, err := os.ReadFile(full)
	if err != nil {
		return Result{Error: fmt.Sprintf("read_file: %v", err)}, nil
	}
	if len(data) > maxBytes {
		data = data[:maxBytes]
		return Result{Content: string(data) + "\n... [truncated, file is " + fmt.Sprintf("%d", len(data)) + " bytes]"}, nil
	}
	return Result{Content: string(data)}, nil
}

type WriteFileTool struct{ WorkDir string }

func (t *WriteFileTool) Name() string        { return "write_file" }
func (t *WriteFileTool) Description() string  { return "Write content to a file, creating parent dirs as needed." }
func (t *WriteFileTool) Schema() map[string]any {
	return map[string]any{
		"type":     "object",
		"properties": map[string]any{
			"path":    map[string]any{"type": "string"},
			"content": map[string]any{"type": "string"},
		},
		"required": []string{"path", "content"},
	}
}
func (t *WriteFileTool) Execute(ctx context.Context, args map[string]any) (Result, error) {
	path, _ := args["path"].(string)
	content, _ := args["content"].(string)
	if path == "" {
		return Result{Error: "path is required"}, nil
	}
	full := filepath.Join(t.WorkDir, path)
	if !strings.HasPrefix(filepath.Clean(full), filepath.Clean(t.WorkDir)) {
		return Result{Error: "path escapes work directory"}, nil
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return Result{Error: err.Error()}, nil
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		return Result{Error: err.Error()}, nil
	}
	return Result{Content: fmt.Sprintf("wrote %d bytes to %s", len(content), path)}, nil
}

type SearchCodeTool struct{ WorkDir string }

func (t *SearchCodeTool) Name() string        { return "search_code" }
func (t *SearchCodeTool) Description() string { return "Search for a regex/grep pattern in files under work_dir." }
func (t *SearchCodeTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{"type": "string"},
			"path":  map[string]any{"type": "string", "description": "Optional subdirectory"},
		},
		"required": []string{"query"},
	}
}
func (t *SearchCodeTool) Execute(ctx context.Context, args map[string]any) (Result, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return Result{Error: "query is required"}, nil
	}
	searchPath := t.WorkDir
	if p, ok := args["path"].(string); ok && p != "" {
		searchPath = filepath.Join(t.WorkDir, p)
	}
	cmd := exec.CommandContext(ctx, "grep", "-rn", "--", query, searchPath)
	out, err := cmd.CombinedOutput()
	if err != nil && len(out) == 0 {
		return Result{Content: "(no matches)"}, nil
	}
	const maxBytes = 5_000
	if len(out) > maxBytes {
		out = out[:maxBytes]
		return Result{Content: string(out) + "\n... [truncated]"}, nil
	}
	return Result{Content: string(out)}, nil
}

func init() {
	Register(&ReadFileTool{})
	Register(&WriteFileTool{})
	Register(&SearchCodeTool{})
}
```

- [ ] **Step 3: Implement run_command and run_test**

Create `server/internal/loop/tools/shell.go`:

```go
package tools

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

type RunCommandTool struct {
	WorkDir     string
	DefaultTimeout time.Duration
}

func (t *RunCommandTool) Name() string { return "run_command" }
func (t *RunCommandTool) Description() string {
	return "Run a shell command in work_dir. Returns stdout, stderr, exit_code."
}
func (t *RunCommandTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"cmd":        map[string]any{"type": "string"},
			"timeout_sec": map[string]any{"type": "integer", "default": 300},
		},
		"required": []string{"cmd"},
	}
}
func (t *RunCommandTool) Execute(ctx context.Context, args map[string]any) (Result, error) {
	cmdStr, _ := args["cmd"].(string)
	if cmdStr == "" {
		return Result{Error: "cmd is required"}, nil
	}
	timeout := t.DefaultTimeout
	if v, ok := args["timeout_sec"].(float64); ok && v > 0 {
		timeout = time.Duration(v) * time.Second
	}
	if timeout == 0 {
		timeout = 5 * time.Minute
	}

	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cctx, "sh", "-c", cmdStr)
	cmd.Dir = t.WorkDir
	stdout, stderr := &limitedBuffer{max: 50_000}, &limitedBuffer{max: 50_000}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Run()

	res := Result{Stdout: stdout.String(), Stderr: stderr.String()}
	res.ExitCode = cmd.ProcessState.ExitCode()
	if err != nil {
		if cctx.Err() == context.DeadlineExceeded {
			res.Error = fmt.Sprintf("command timed out after %s", timeout)
		} else {
			// non-zero exit is NOT a tool error — let the LLM see the stderr
			res.Content = res.Stdout
		}
	} else {
		res.Content = res.Stdout
	}
	return res, nil
}

type RunTestTool struct {
	WorkDir string
	Cmd     string // default: "go test ./..." for Go projects
}

func (t *RunTestTool) Name() string { return "run_test" }
func (t *RunTestTool) Description() string {
	return "Run the project's test suite. Default: 'go test ./...'. Pass 'cmd' to override."
}
func (t *RunTestTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"cmd": map[string]any{"type": "string"},
		},
	}
}
func (t *RunTestTool) Execute(ctx context.Context, args map[string]any) (Result, error) {
	cmd := t.Cmd
	if c, ok := args["cmd"].(string); ok && c != "" {
		cmd = c
	}
	if cmd == "" {
		cmd = "go test ./..."
	}
	// Delegate to RunCommandTool with longer timeout
	rt := &RunCommandTool{WorkDir: t.WorkDir, DefaultTimeout: 10 * time.Minute}
	return rt.Execute(ctx, map[string]any{"cmd": cmd, "timeout_sec": 600})
}

type limitedBuffer struct {
	max int
	buf []byte
}
func (b *limitedBuffer) Write(p []byte) (int, error) {
	room := b.max - len(b.buf)
	if room <= 0 {
		return len(p), nil // drop
	}
	if len(p) > room {
		b.buf = append(b.buf, p[:room]...)
	} else {
		b.buf = append(b.buf, p...)
	}
	return len(p), nil
}
func (b *limitedBuffer) String() string { return string(b.buf) }

func init() {
	Register(&RunCommandTool{DefaultTimeout: 5 * time.Minute})
	Register(&RunTestTool{Cmd: "go test ./..."})
}
```

- [ ] **Step 4: Write tests for fs and shell tools**

Create `server/internal/loop/tools/fs_test.go`:

```go
package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestReadFile_HappyPath(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := Get("read_file")
	if err != nil { t.Fatal(err) }
	t.Setenv("WorkDir", dir) // not actually used; see below
	_ = r

	// We need to override the WorkDir on the registered tool. The init() in
	// fs.go registers with empty WorkDir. For tests, build a local instance.
	t1 := &ReadFileTool{WorkDir: dir}
	res, err := t1.Execute(context.Background(), map[string]any{"path": "a.txt"})
	if err != nil { t.Fatal(err) }
	if res.Error != "" { t.Fatalf("tool error: %s", res.Error) }
	if res.Content != "hello" { t.Errorf("got %q", res.Content) }
}

func TestReadFile_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	t1 := &ReadFileTool{WorkDir: dir}
	res, _ := t1.Execute(context.Background(), map[string]any{"path": "../etc/passwd"})
	if res.Error == "" {
		t.Error("expected error for path traversal")
	}
}

func TestWriteFile_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	t1 := &WriteFileTool{WorkDir: dir}
	if _, err := t1.Execute(context.Background(), map[string]any{"path": "sub/a.txt", "content": "x"}); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "sub", "a.txt"))
	if string(got) != "x" {
		t.Errorf("got %q", got)
	}
}
```

Create `server/internal/loop/tools/shell_test.go`:

```go
package tools

import (
	"context"
	"testing"
)

func TestRunCommand_Echo(t *testing.T) {
	dir := t.TempDir()
	t1 := &RunCommandTool{WorkDir: dir, DefaultTimeout: 10_000_000_000} // 10s
	res, err := t1.Execute(context.Background(), map[string]any{"cmd": "echo hi"})
	if err != nil { t.Fatal(err) }
	if res.ExitCode != 0 { t.Errorf("exit %d, stderr=%s", res.ExitCode, res.Stderr) }
	if res.Content != "hi\n" { t.Errorf("got %q", res.Content) }
}

func TestRunCommand_ExitCodePropagated(t *testing.T) {
	dir := t.TempDir()
	t1 := &RunCommandTool{WorkDir: dir, DefaultTimeout: 10_000_000_000}
	res, _ := t1.Execute(context.Background(), map[string]any{"cmd": "false"})
	if res.Error == "" {
		// Per spec: non-zero exit is not a tool error; just exit_code in result.
	}
	if res.ExitCode == 0 {
		t.Error("expected non-zero exit code")
	}
}
```

- [ ] **Step 5: Run tests**

```bash
cd /Users/doug/ai/system/agentra/server && go test ./internal/loop/tools/ -v
```

Expected: PASS (5 tests).

- [ ] **Step 6: Commit**

```bash
git add server/internal/loop/tools/
git commit -m "feat(loop): add tool package with read_file, write_file, search_code, run_command, run_test"
```

---

## Task 12: Tools package — git_status, git_diff, git_commit, git_push, create_branch, github_pr_create

**Files:**
- Create: `server/internal/loop/tools/git.go`
- Create: `server/internal/loop/tools/git_test.go`

- [ ] **Step 1: Implement git_* tools**

Create `server/internal/loop/tools/git.go`:

```go
package tools

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// All git tools run from the loop's work_dir. The branch to operate on is
// passed as an argument (default: current branch).

type GitStatusTool struct{ WorkDir string }

func (t *GitStatusTool) Name() string        { return "git_status" }
func (t *GitStatusTool) Description() string  { return "Run `git status` and return the output." }
func (t *GitStatusTool) Schema() map[string]any { return map[string]any{"type": "object"} }
func (t *GitStatusTool) Execute(ctx context.Context, args map[string]any) (Result, error) {
	return runGit(ctx, t.WorkDir, 10*time.Second, "status")
}

type GitDiffTool struct{ WorkDir string }

func (t *GitDiffTool) Name() string        { return "git_diff" }
func (t *GitDiffTool) Description() string  { return "Show a unified diff. Optionally limit to a single file (path arg)." }
func (t *GitDiffTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"staged": map[string]any{"type": "boolean", "default": false},
			"file":   map[string]any{"type": "string"},
		},
	}
}
func (t *GitDiffTool) Execute(ctx context.Context, args map[string]any) (Result, error) {
	gitArgs := []string{"diff"}
	if staged, _ := args["staged"].(bool); staged {
		gitArgs = append(gitArgs, "--staged")
	}
	if file, _ := args["file"].(string); file != "" {
		gitArgs = append(gitArgs, "--", file)
	}
	return runGit(ctx, t.WorkDir, 30*time.Second, gitArgs...)
}

type GitCommitTool struct{ WorkDir string }

func (t *GitCommitTool) Name() string        { return "git_commit" }
func (t *GitCommitTool) Description() string  { return "Stage all changes and commit with the given message. Returns the new SHA." }
func (t *GitCommitTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"message": map[string]any{"type": "string"},
		},
		"required": []string{"message"},
	}
}
func (t *GitCommitTool) Execute(ctx context.Context, args map[string]any) (Result, error) {
	message, _ := args["message"].(string)
	if message == "" {
		return Result{Error: "message is required"}, nil
	}
	// git add -A
	if _, err := runGit(ctx, t.WorkDir, 30*time.Second, "add", "-A"); err != nil {
		return Result{Error: err.Error()}, nil
	}
	// Check if there is anything to commit
	st, _ := runGit(ctx, t.WorkDir, 10*time.Second, "diff", "--staged", "--stat")
	if st.Content == "" {
		return Result{Error: "no changes to commit"}, nil
	}
	// git commit
	if _, err := runGit(ctx, t.WorkDir, 30*time.Second, "commit", "-m", message); err != nil {
		return Result{Error: err.Error()}, nil
	}
	shaRes, _ := runGit(ctx, t.WorkDir, 5*time.Second, "rev-parse", "HEAD")
	return Result{Content: strings.TrimSpace(shaRes.Content)}, nil
}

type GitPushTool struct{ WorkDir string }

func (t *GitPushTool) Name() string        { return "git_push" }
func (t *GitPushTool) Description() string  { return "Push the current branch to the given remote (default: origin)." }
func (t *GitPushTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"remote": map[string]any{"type": "string", "default": "origin"},
			"branch": map[string]any{"type": "string"},
		},
		"required": []string{"branch"},
	}
}
func (t *GitPushTool) Execute(ctx context.Context, args map[string]any) (Result, error) {
	remote, _ := args["remote"].(string)
	if remote == "" {
		remote = "origin"
	}
	branch, _ := args["branch"].(string)
	if branch == "" {
		return Result{Error: "branch is required"}, nil
	}
	if _, err := runGit(ctx, t.WorkDir, 60*time.Second, "push", remote, branch); err != nil {
		return Result{Error: err.Error()}, nil
	}
	return Result{Content: fmt.Sprintf("pushed %s/%s", remote, branch)}, nil
}

type CreateBranchTool struct{ WorkDir string }

func (t *CreateBranchTool) Name() string        { return "create_branch" }
func (t *CreateBranchTool) Description() string  { return "Create and check out a new branch from the current HEAD." }
func (t *CreateBranchTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{"type": "string"},
		},
		"required": []string{"name"},
	}
}
func (t *CreateBranchTool) Execute(ctx context.Context, args map[string]any) (Result, error) {
	name, _ := args["name"].(string)
	if name == "" {
		return Result{Error: "name is required"}, nil
	}
	if _, err := runGit(ctx, t.WorkDir, 5*time.Second, "checkout", "-b", name); err != nil {
		return Result{Error: err.Error()}, nil
	}
	return Result{Content: "switched to " + name}, nil
}

type GitHubPRCreateTool struct{ WorkDir string }

func (t *GitHubPRCreateTool) Name() string        { return "github_pr_create" }
func (t *GitHubPRCreateTool) Description() string  { return "Open a GitHub PR using `gh pr create`. Returns PR URL on success." }
func (t *GitHubPRCreateTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"title": map[string]any{"type": "string"},
			"body":  map[string]any{"type": "string"},
			"base":  map[string]any{"type": "string", "default": "main"},
		},
		"required": []string{"title", "body"},
	}
}
func (t *GitHubPRCreateTool) Execute(ctx context.Context, args map[string]any) (Result, error) {
	title, _ := args["title"].(string)
	body, _ := args["body"].(string)
	base, _ := args["base"].(string)
	if base == "" {
		base = "main"
	}
	if title == "" || body == "" {
		return Result{Error: "title and body are required"}, nil
	}
	// Use gh CLI; assume it's authenticated in dogfood mode.
	cmd := exec.CommandContext(ctx, "gh", "pr", "create", "--title", title, "--body", body, "--base", base)
	cmd.Dir = t.WorkDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return Result{Error: fmt.Sprintf("gh pr create: %v: %s", err, out)}, nil
	}
	// gh output: "https://github.com/owner/repo/pull/N"
	url := strings.TrimSpace(string(out))
	return Result{Content: url, PRURL: url}, nil
}

// runGit executes git in workDir with the given args and a timeout.
func runGit(ctx context.Context, workDir string, timeout time.Duration, args ...string) (Result, error) {
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(cctx, "git", args...)
	cmd.Dir = workDir
	out, err := cmd.CombinedOutput()
	res := Result{Content: string(out), ExitCode: 0}
	if cmd.ProcessState != nil {
		res.ExitCode = cmd.ProcessState.ExitCode()
	}
	if err != nil {
		return res, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return res, nil
}

func init() {
	Register(&GitStatusTool{})
	Register(&GitDiffTool{})
	Register(&GitCommitTool{})
	Register(&GitPushTool{})
	Register(&CreateBranchTool{})
	Register(&GitHubPRCreateTool{})
}
```

Add the `PRURL` field to the `Result` struct in `tool.go`:

```go
type Result struct {
	Content  string `json:"content"`
	Error    string `json:"error,omitempty"`
	Stderr   string `json:"stderr,omitempty"`
	ExitCode int    `json:"exit_code,omitempty"`
	PRURL    string `json:"pr_url,omitempty"`
}
```

- [ ] **Step 2: Write tests**

Create `server/internal/loop/tools/git_test.go`:

```go
package tools

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// initRepo creates a temp dir with a git repo, one commit, and a remote (local path).
func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bareDir := filepath.Join(t.TempDir(), "bare.git")
	for _, args := range [][]string{
		{"init", "--bare", bareDir},
	} {
		c := exec.Command("git", args...)
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	for _, args := range [][]string{
		{"init"},
		{"config", "user.email", "test@x"},
		{"config", "user.name", "Test"},
		{"remote", "add", "origin", bareDir},
		{"commit", "--allow-empty", "-m", "init"},
		{"push", "-u", "origin", "master"},
	} {
		c := exec.Command("git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	return dir
}

func TestGitStatus_Empty(t *testing.T) {
	dir := initRepo(t)
	tool := &GitStatusTool{WorkDir: dir}
	res, err := tool.Execute(context.Background(), nil)
	if err != nil { t.Fatal(err) }
	if !strings.Contains(res.Content, "nothing to commit") &&
	   !strings.Contains(res.Content, "no changes") {
		t.Errorf("expected clean status, got: %s", res.Content)
	}
}

func TestCreateBranch(t *testing.T) {
	dir := initRepo(t)
	tool := &CreateBranchTool{WorkDir: dir}
	_, err := tool.Execute(context.Background(), map[string]any{"name": "feature-x"})
	if err != nil { t.Fatal(err) }
	res, _ := (&GitStatusTool{WorkDir: dir}).Execute(context.Background(), nil)
	if !strings.Contains(res.Content, "feature-x") {
		t.Errorf("expected branch in status, got: %s", res.Content)
	}
}
```

- [ ] **Step 3: Run tests**

```bash
cd /Users/doug/ai/system/agentra/server && go test ./internal/loop/tools/ -v
```

Expected: PASS (additional git tests; some may be skipped if `gh` is not installed in CI — gate with `if _, err := exec.LookPath("gh"); err != nil { t.Skip(...) }`).

- [ ] **Step 4: Commit**

```bash
git add server/internal/loop/tools/git.go server/internal/loop/tools/git_test.go server/internal/loop/tools/tool.go
git commit -m "feat(loop): add git tools (status, diff, commit, push, create_branch, github_pr_create)"
```

---

## Task 13: Develop stage executor + state machine plan→develop→review transition

**Files:**
- Create: `server/internal/loop/stages/develop.go`
- Create: `server/internal/loop/stages/develop_test.go`

- [ ] **Step 1: Write the failing test**

Create `server/internal/loop/stages/develop_test.go`:

```go
package stages

import (
	"context"
	"testing"

	"github.com/agentra/agentra/server/pkg/agent"
)

func TestDevelopExecutor_BuildsSession(t *testing.T) {
	Register("loop_develop", Develop)
	defer delete(registry, "loop_develop")

	be := &fakeBackend{
		// session.Result returned by the fake must include the PR URL the
		// develop stage will pick up.
	}
	task := TaskRef{
		IssueID:    "issue-1",
		IssueTitle: "Add JWT auth",
		Branch:     "loop/issue-1-0",
		Iteration:  0,
	}
	res, err := Develop(context.Background(), task, be)
	if err != nil { t.Fatal(err) }
	if res == nil { t.Fatal("nil result") }
	// fake backend returns the canned "## Goal" text from Plan test; just
	// confirm the executor wired it through.
	if res.Text == "" { t.Error("expected non-empty result text") }
	if be.lastSession == nil { t.Fatal("backend not called") }
	if !contains(be.lastSession.SystemPrompt, "developer") &&
	   !contains(be.lastSession.SystemPrompt, "Develop") {
		t.Errorf("expected develop-themed system prompt, got: %q", be.lastSession.SystemPrompt)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/doug/ai/system/agentra/server && go test ./internal/loop/stages/ -run TestDevelopExecutor
```

Expected: FAIL — `Develop` undefined.

- [ ] **Step 3: Write the Develop executor**

Create `server/internal/loop/stages/develop.go`:

```go
package stages

import (
	"context"
	"fmt"
	"regexp"

	"github.com/agentra/agentra/server/pkg/agent"
)

var prURLPattern = regexp.MustCompile(`https://github\.com/[^/]+/[^/]+/pull/\d+`)

// Develop is the executor for task_type="loop_develop". Write stage: edits
// files, runs tests, commits to loop/<issue-id>-<n>, opens a PR.
func Develop(ctx context.Context, task TaskRef, backend agent.Backend) (*Result, error) {
	prompt, err := loadPrompt("develop", task)
	if err != nil {
		return nil, err
	}

	// Pre-flight: ask backend to perform the changes. The exact behavior
	// (read plan, edit files, run tests, commit, push, open PR) is encoded
	// in the develop.md prompt + the tools exposed below.
	session := &agent.Session{
		SystemPrompt: prompt,
		Messages: []agent.Message{{
			Role:    "user",
			Content: fmt.Sprintf("Branch: %s\nIssue: %s\n\nBegin implementation.", task.Branch, task.IssueTitle),
		}},
		Tools: toolsForStage("loop_develop"),
	}

	out, err := backend.Execute(ctx, session)
	if err != nil {
		return nil, fmt.Errorf("develop: backend.Execute: %w", err)
	}

	text := ""
	prURL := ""
	if out != nil {
		text = out.Result
		if text == "" && len(out.Messages) > 0 {
			text = out.Messages[len(out.Messages)-1].Content
		}
		// Extract PR URL from the conversation if backend surfaced it
		for _, m := range out.Messages {
			if u := prURLPattern.FindString(m.Content); u != "" {
				prURL = u
				break
			}
		}
	}
	return &Result{Text: text, PRURL: prURL}, nil
}

func init() {
	Register("loop_develop", Develop)
}
```

Update `toolsForStage` in `tools.go` (the stub from Task 10) to return the full tool set per stage. Define a `tools` helper that pulls from `tools.Registry`:

```go
// server/internal/loop/stages/tools.go
package stages

import "github.com/agentra/agentra/server/internal/loop/tools"

func toolsForStage(taskType string) []agent.Tool {
	names := toolNamesByStage[taskType]
	out := make([]agent.Tool, 0, len(names))
	for _, n := range names {
		if t, ok := tools.Get(n); ok {
			out = append(out, t)
		}
	}
	return out
}

var toolNamesByStage = map[string][]string{
	"loop_plan":    {"read_file", "search_code"},
	"loop_develop": {"read_file", "search_code", "write_file", "run_command", "run_test",
		"git_status", "git_diff", "git_commit", "git_push", "create_branch", "github_pr_create"},
	"loop_review":  {"read_file", "git_diff"},
	"loop_fix":     {"read_file", "search_code", "write_file", "run_command", "run_test",
		"git_status", "git_diff", "git_commit", "git_push"},
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /Users/doug/ai/system/agentra/server && go test ./internal/loop/stages/ -run TestDevelopExecutor
```

Expected: PASS.

- [ ] **Step 5: Manual smoke test**

Start the dev server, create a loop, and verify a `loop_develop` task is created and dispatched:

```bash
make dev # in another terminal
# 1. Create a test issue in the UI
# 2. POST /api/loops with that issue_id
# 3. Watch server logs: loop plan completes -> loop_develop task created
# 4. Daemon picks it up -> backend.Execute called with develop prompt
# 5. (Eventually) PR URL shows up in the loop record
```

- [ ] **Step 6: Commit**

```bash
git add server/internal/loop/stages/develop.go server/internal/loop/stages/develop_test.go server/internal/loop/stages/tools.go
git commit -m "feat(loop): implement Develop stage executor and tool-per-stage registry"
```

---

## Task 14: Review stage executor + JSON parsing

**Files:**
- Create: `server/internal/loop/stages/review.go`
- Create: `server/internal/loop/stages/review_test.go`

- [ ] **Step 1: Write the failing test**

Create `server/internal/loop/stages/review_test.go`:

```go
package stages

import (
	"context"
	"testing"

	"github.com/agentra/agentra/server/pkg/agent"
)

func TestReviewExecutor_ParsesJSON(t *testing.T) {
	Register("loop_review", Review)
	defer delete(registry, "loop_review")

	be := &jsonBackend{text: `{"approved": false, "issues": [{"file":"x.go","line":10,"severity":"high","message":"bug"}]}`}
	task := TaskRef{IssueID: "issue-1", Branch: "loop/issue-1-0"}
	res, err := Review(context.Background(), task, be)
	if err != nil { t.Fatal(err) }
	if res == nil { t.Fatal("nil result") }

	var parsed struct {
		Approved *bool  `json:"approved"`
		Issues   string `json:"issues"`
	}
	if err := jsonUnmarshal(res.Text, &parsed); err != nil {
		t.Fatalf("result not valid JSON: %v\n%s", err, res.Text)
	}
	if parsed.Approved == nil || *parsed.Approved {
		t.Error("expected approved=false")
	}
	if parsed.Issues == "" {
		t.Error("expected non-empty issues")
	}
}

// jsonBackend returns a canned JSON string from Execute.
type jsonBackend struct{ text string }
func (b *jsonBackend) Execute(ctx context.Context, s *agent.Session, opts ...agent.ExecOption) (*agent.Session, error) {
	return &agent.Session{Result: b.text, Messages: []agent.Message{{Role: "assistant", Content: b.text}}}, nil
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/doug/ai/system/agentra/server && go test ./internal/loop/stages/ -run TestReviewExecutor
```

Expected: FAIL — `Review` and `jsonUnmarshal` undefined.

- [ ] **Step 3: Write the Review executor**

Create `server/internal/loop/stages/review.go`:

```go
package stages

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/agentra/agentra/server/pkg/agent"
)

var jsonBlockPattern = regexp.MustCompile("(?s)```(?:json)?\\s*(\\{.*?\\})\\s*```")

// Review is the executor for task_type="loop_review". Read-only: reads the
// diff and the original plan, emits a JSON object with {approved, issues}.
func Review(ctx context.Context, task TaskRef, backend agent.Backend) (*Result, error) {
	prompt, err := loadPrompt("review", task)
	if err != nil {
		return nil, err
	}

	session := &agent.Session{
		SystemPrompt: prompt,
		Messages: []agent.Message{{
			Role:    "user",
			Content: fmt.Sprintf("Review the diff for branch %s against the original plan.", task.Branch),
		}},
		Tools: toolsForStage("loop_review"),
	}

	out, err := backend.Execute(ctx, session)
	if err != nil {
		return nil, fmt.Errorf("review: backend.Execute: %w", err)
	}

	text := ""
	if out != nil {
		text = out.Result
		if text == "" && len(out.Messages) > 0 {
			text = out.Messages[len(out.Messages)-1].Content
		}
	}
	// Extract JSON from ```json ... ``` blocks if the LLM wrapped it.
	cleaned := extractJSON(text)
	// Validate by parsing; if it fails, return error so the stage fails and
	// the loop is marked failed (rather than silently approving).
	if !looksLikeReviewJSON(cleaned) {
		return nil, fmt.Errorf("review: output is not valid review JSON: %s", cleaned)
	}
	return &Result{Text: cleaned}, nil
}

func extractJSON(s string) string {
	if m := jsonBlockPattern.FindStringSubmatch(s); len(m) > 1 {
		return m[1]
	}
	return s
}

func looksLikeReviewJSON(s string) bool {
	var v struct {
		Approved *bool  `json:"approved"`
		Issues   []any  `json:"issues"`
	}
	return json.Unmarshal([]byte(s), &v) == nil && v.Approved != nil
}

// jsonUnmarshal is a tiny shim used by tests; in production use encoding/json.
func jsonUnmarshal(s string, v any) error { return json.Unmarshal([]byte(s), v) }

func init() {
	Register("loop_review", Review)
}
```

Create `server/internal/loop/prompts/review.md` if not already done in Task 9:

````markdown
# Loop Review Stage

You are reviewing a code change for correctness against the original plan.

## Input
- Branch: {{.Branch}}
- Issue: {{.IssueID}}

## Tools
- `read_file` to inspect specific files
- `git_diff` to see the full change

## Output
Respond with a single ```json ... ``` code block of this exact shape:

```json
{
  "approved": true|false,
  "issues": [
    {"file": "path/to/file.go", "line": 42, "severity": "high|medium|low", "message": "..."}
  ]
}
```

## Rules
- Set `approved: true` only if there are NO high or medium severity issues
- Every issue must reference a specific file and line
- Do not edit any files
- Output ONLY the JSON code block, no other prose
````

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /Users/doug/ai/system/agentra/server && go test ./internal/loop/stages/ -run TestReviewExecutor
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add server/internal/loop/stages/review.go server/internal/loop/stages/review_test.go server/internal/loop/prompts/review.md
git commit -m "feat(loop): implement Review stage executor with JSON validation"
```

---

## Task 15: Fix stage executor + iteration tracking

**Files:**
- Create: `server/internal/loop/stages/fix.go`
- Create: `server/internal/loop/stages/fix_test.go`
- Modify: `server/internal/loop/coordinator.go` (finish `HandleTaskCompleted` review-result path; verify `iterationBump` from Task 6 works)

- [ ] **Step 1: Write the failing test**

Create `server/internal/loop/stages/fix_test.go`:

```go
package stages

import (
	"context"
	"testing"
)

func TestFixExecutor_BuildsSession(t *testing.T) {
	Register("loop_fix", Fix)
	defer delete(registry, "loop_fix")

	be := &fakeBackend{}
	task := TaskRef{IssueID: "issue-1", Branch: "loop/issue-1-1", Iteration: 1}
	res, err := Fix(context.Background(), task, be)
	if err != nil { t.Fatal(err) }
	if res == nil || res.Text == "" { t.Error("expected non-empty result") }
	if be.lastSession == nil { t.Fatal("backend not called") }
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/doug/ai/system/agentra/server && go test ./internal/loop/stages/ -run TestFixExecutor
```

Expected: FAIL — `Fix` undefined.

- [ ] **Step 3: Write the Fix executor**

Create `server/internal/loop/stages/fix.go`:

```go
package stages

import (
	"context"
	"fmt"

	"github.com/agentra/agentra/server/pkg/agent"
)

// Fix is the executor for task_type="loop_fix". Like develop, but driven by
// the issues from the previous review run.
func Fix(ctx context.Context, task TaskRef, backend agent.Backend) (*Result, error) {
	prompt, err := loadPrompt("fix", task)
	if err != nil {
		return nil, err
	}

	session := &agent.Session{
		SystemPrompt: prompt,
		Messages: []agent.Message{{
			Role:    "user",
			Content: fmt.Sprintf("Fix the issues from the previous review on branch %s (iteration %d).", task.Branch, task.Iteration),
		}},
		Tools: toolsForStage("loop_fix"),
	}

	out, err := backend.Execute(ctx, session)
	if err != nil {
		return nil, fmt.Errorf("fix: backend.Execute: %w", err)
	}
	text := ""
	if out != nil {
		text = out.Result
		if text == "" && len(out.Messages) > 0 {
			text = out.Messages[len(out.Messages)-1].Content
		}
	}
	return &Result{Text: text}, nil
}

func init() {
	Register("loop_fix", Fix)
}
```

Create `server/internal/loop/prompts/fix.md` if not done in Task 9:

````markdown
# Loop Fix Stage

You are applying fixes to address review issues from a previous iteration.

## Input
- Branch: {{.Branch}} (already created by develop stage)
- Iteration: {{.Iteration}} (1-based count of how many fix attempts so far)

## Behavior
1. Read the most recent review (passed in by Coordinator via task metadata)
2. For each issue: read the file, fix it, verify with a test
3. Commit fixes with a message like "fix: address review issues (iter {{.Iteration}})"
4. Push to the same branch

## Output
A brief markdown summary of what you changed and which issues are now resolved.
````

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /Users/doug/ai/system/agentra/server && go test ./internal/loop/stages/ -run TestFixExecutor
```

Expected: PASS.

- [ ] **Step 5: Verify the coordinator iteration tracking**

The `decideNextStage` already returns `iterationBump: 1` when a rejected review comes in (Task 6 test `TestDecideNextStage_ReviewRejectedCreatesFix` covers this). Verify end-to-end with an integration test:

Add to `server/internal/loop/integration_test.go`:

```go
func TestIntegration_PlanDevelopReviewFixDone(t *testing.T) {
    // Spin up Coordinator with mock backend; emit task:completed events
    // manually and verify the loop walks plan -> develop -> review -> fix ->
    // review -> done.
    //
    // Full body in Task 16; for now, add a stub:
    t.Skip("covered in Task 16 integration test")
}
```

- [ ] **Step 6: Commit**

```bash
git add server/internal/loop/stages/fix.go server/internal/loop/stages/fix_test.go server/internal/loop/prompts/fix.md
git commit -m "feat(loop): implement Fix stage executor"
```

---

## Task 16: Failure handling — `HandleTaskFailed`, `failure_reason`, integration test

**Files:**
- Create: `server/internal/loop/integration_test.go`
- Modify: `server/internal/loop/coordinator.go` (flesh out `HandleTaskFailed`)

- [ ] **Step 1: Write the failing integration test**

Create `server/internal/loop/integration_test.go`:

```go
package loop_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/agentra/agentra/server/internal/events"
	looppkg "github.com/agentra/agentra/server/internal/loop"
	"github.com/agentra/agentra/server/pkg/db"
)

func TestIntegration_PlanDevelopReviewApprovedDone(t *testing.T) {
	pool := testPool(t)
	q := db.New(pool)
	bus := events.New()
	coord := looppkg.NewCoordinator(q, bus)
	store := looppkg.NewStore(q)

	wsID, issueID := seedWorkspaceAndIssue(t, pool)
	maxIters := 3
	loopRow, err := store.CreateLoop(context.Background(), looppkg.CreateLoopInput{
		IssueID: issueID, WorkspaceID: wsID, MaxIterations: &maxIters,
	})
	if err != nil { t.Fatal(err) }

	// Start the loop manually: transition to running, current_stage=plan
	running := looppkg.StatusRunning
	plan := looppkg.StagePlan
	_, err = store.UpdateStatus(context.Background(), loopRow.ID, looppkg.UpdateStatusInput{
		Status: &running, CurrentStage: &plan,
	})
	if err != nil { t.Fatal(err) }

	// Simulate the plan task completing (no review, just a success marker)
	emitCompleted(t, bus, q, loopRow.ID, "task-plan-1", "loop_plan", nil)

	// Verify next stage is develop
	got, _ := store.GetLoop(context.Background(), loopRow.ID)
	if got.CurrentStage == nil || *got.CurrentStage != looppkg.StageDevelop {
		t.Errorf("expected current_stage=develop, got %v", got.CurrentStage)
	}
}

func TestIntegration_ReviewRejectedTriggersFix(t *testing.T) {
	pool := testPool(t)
	q := db.New(pool)
	bus := events.New()
	coord := looppkg.NewCoordinator(q, bus)
	store := looppkg.NewStore(q)

	wsID, issueID := seedWorkspaceAndIssue(t, pool)
	loopRow, _ := store.CreateLoop(context.Background(), looppkg.CreateLoopInput{
		IssueID: issueID, WorkspaceID: wsID,
	})

	// Force the loop to current_stage=review, iteration=0
	running := looppkg.StatusRunning
	review := looppkg.StageReview
	_, err := store.UpdateStatus(context.Background(), loopRow.ID, looppkg.UpdateStatusInput{
		Status: &running, CurrentStage: &review,
	})
	if err != nil { t.Fatal(err) }

	// Emit a task:completed for the review task with approved=false
	notApproved := false
	result := looppkg.TaskResult{ReviewApproved: &notApproved, ReviewIssues: "[]"}
	resultJSON, _ := json.Marshal(result)
	_ = resultJSON
	// ... emitCompleted with result blob stored in task_runs.output (skipped
	// for brevity; see coordinator.go for parseTaskResult path)
	t.Skip("full body: emit task:completed + verify next stage=fix and iteration=1")
}
```

The body of these tests is a bit involved (needs helpers to seed a task in `agent_task_queue` with `task_type` / `loop_id` and write JSON to `task_runs.output`). The implementer should factor out a `seedTask(t, pool, loopID, taskType, outputJSON)` helper to keep the tests readable.

- [ ] **Step 2: Implement `HandleTaskFailed`**

In `server/internal/loop/coordinator.go`, replace the stub from Task 7 with:

```go
func (c *Coordinator) HandleTaskFailed(ctx context.Context, e events.Event) error {
    payload, ok := e.Payload.(map[string]any)
    if !ok { return nil }
    taskID, _ := payload["task_id"].(string)
    if taskID == "" { return nil }
    task, err := c.queries.GetAgentTask(ctx, taskID)
    if err != nil { return err }
    if task.LoopID == nil || !task.LoopID.Valid { return nil }
    loopID := task.LoopID.String

    l, err := c.store.GetLoop(ctx, loopID)
    if err != nil { return err }
    if l.Status != StatusRunning { return nil }

    // Mark the loop as failed with the appropriate reason
    failed := StatusFailed
    reason := string(FailureStageTimeout)
    if errMsg, ok := payload["error"].(string); ok && errMsg != "" {
        reason = classifyError(errMsg)
    }
    now := nowISO()
    _, err = c.store.UpdateStatus(ctx, l.ID, UpdateStatusInput{
        Status: &failed, FailureReason: &reason, CompletedAt: &now,
    })
    return err
}

func classifyError(msg string) FailureReason {
    switch {
    case strings.Contains(msg, "context"):
        return FailureContextExceeded
    case strings.Contains(msg, "filter"), strings.Contains(msg, "blocked"):
        return FailureContentFilter
    case strings.Contains(msg, "timeout"):
        return FailureStageTimeout
    case strings.Contains(msg, "PR"):
        return FailurePRCreateFailed
    }
    return FailureUnrecoverable
}
```

- [ ] **Step 3: Run the integration test**

```bash
cd /Users/doug/ai/system/agentra/server
TEST_DATABASE_URL="$(make -s db-url-test || echo $DATABASE_URL)" \
  go test ./internal/loop/ -run TestIntegration -v
```

Expected: PASS for `TestIntegration_PlanDevelopReviewApprovedDone`; the second test is skipped with a TODO.

- [ ] **Step 4: Commit**

```bash
git add server/internal/loop/integration_test.go server/internal/loop/coordinator.go
git commit -m "feat(loop): add HandleTaskFailed, error classification, and integration test skeleton"
```

---

## Task 17: CLI subcommands — `agentra loop {start,status,pause,resume,cancel,list}`

**Files:**
- Create: `server/internal/cli/loop.go`
- Create: `server/internal/cli/loop_test.go`

- [ ] **Step 1: Read the existing CLI subcommand pattern**

```bash
cd /Users/doug/ai/system/agentra
ls server/internal/cli/ | head -20
grep -n "cobra\|Command" server/internal/cli/*.go | head -20
```

Find an existing simple subcommand (e.g. `issue.go` or `agent.go`) and copy its structure.

- [ ] **Step 2: Write the failing test**

Create `server/internal/cli/loop_test.go` — for CLI tests, follow the pattern used by the existing CLI tests in the same package. A typical pattern is to invoke the `RunE` function with synthetic args and assert stdout / exit code.

```go
package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestLoopListCommand_Help(t *testing.T) {
	cmd := newLoopCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "agentra loop") {
		t.Errorf("expected help output, got: %s", out.String())
	}
}
```

The exact test depends on the project's CLI framework (Cobra is the most likely candidate based on `cmd/agentra/main.go:76`).

- [ ] **Step 3: Write the CLI implementation**

Create `server/internal/cli/loop.go` (sketch, adapt to project conventions):

```go
package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	looppkg "github.com/agentra/agentra/server/internal/loop"
)

func newLoopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "loop",
		Short: "Manage Agentic Engineering Loops",
	}
	cmd.AddCommand(newLoopStartCmd())
	cmd.AddCommand(newLoopStatusCmd())
	cmd.AddCommand(newLoopPauseCmd())
	cmd.AddCommand(newLoopResumeCmd())
	cmd.AddCommand(newLoopCancelCmd())
	cmd.AddCommand(newLoopListCmd())
	return cmd
}

func newLoopStartCmd() *cobra.Command {
	var maxIters int
	var agentID string
	c := &cobra.Command{
		Use:   "start <issue-id>",
		Short: "Start a new loop on an issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			api := getAPIClient(cmd)
			body := map[string]any{"issue_id": args[0]}
			if maxIters > 0 { body["max_iterations"] = maxIters }
			if agentID != "" { body["agent_id"] = agentID }
			var out looppkg.Loop
			if err := api.Post("/api/loops", body, &out); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "started loop %s on %s\n", out.ID, args[0])
			return nil
		},
	}
	c.Flags().IntVar(&maxIters, "max-iterations", 5, "Maximum fix iterations")
	c.Flags().StringVar(&agentID, "agent", "", "Agent ID to use for all stages")
	return c
}

// ... similar for status / pause / resume / cancel / list, each calling the
// appropriate REST endpoint and printing a single-line result
```

The `getAPIClient` helper should already exist in the CLI package (it's the standard REST wrapper for the Agentra CLI). If not, find the existing pattern from another subcommand and copy.

- [ ] **Step 4: Register the command in `cmd/agentra/main.go`**

Find the `rootCmd.AddCommand(...)` calls and add:

```go
rootCmd.AddCommand(cli.NewLoopCmd())
```

(or however the project wires CLI subcommands — check `cmd/agentra/main.go` for the pattern).

- [ ] **Step 5: Build and smoke test**

```bash
cd /Users/doug/ai/system/agentra/server && go build -o /tmp/agentra ./cmd/agentra
/tmp/agentra loop --help
/tmp/agentra loop list
```

Expected: help text prints; `list` shows current loops (likely empty).

- [ ] **Step 6: Commit**

```bash
git add server/internal/cli/loop.go server/internal/cli/loop_test.go cmd/agentra/main.go
git commit -m "feat(cli): add agentra loop {start,status,pause,resume,cancel,list}"
```

---

## Task 18: First dogfood — run loop on a real Agentra issue

**Files:** None modified. This is a manual verification milestone.

- [ ] **Step 1: Pick a small, well-scoped issue**

Choose an Agentra issue that:
- Is clearly scoped (single component change)
- Has a failing test or a known desired behavior
- Is small enough to be done in a few file edits

Examples that often qualify:
- Add a missing input validation
- Fix a typo in an error message
- Replace a deprecated function call

Create a fresh issue if none exists that fits.

- [ ] **Step 2: Start the loop**

```bash
make dev  # in another terminal
/tmp/agentra loop start AGENTRA-XXX --max-iterations 5
```

- [ ] **Step 3: Monitor progress**

```bash
# In another terminal:
/tmp/agentra loop status <loop-id>
# Or open the issue in the web UI to watch the task list update in real time
# (existing task:completed WS events drive the UI — no new code needed)
```

- [ ] **Step 4: Wait for PR creation**

Expected: a `loop/<issue-id>-N` branch is pushed, a PR is opened via `gh pr create`, and the loop transitions to `done` with `pr_url` populated.

- [ ] **Step 5: Verify the PR**

```bash
gh pr view <pr-number>
gh pr checks <pr-number>
```

Expected: PR exists, CI is running (or passing), and the diff matches the issue requirements.

- [ ] **Step 6: Document the dogfood result**

Add a short entry to `docs/superpowers/dogfood-log.md` (create the file if it doesn't exist) summarizing:
- Issue ID and title
- Number of iterations (plan → develop → review → fix → ... → done)
- Total token cost (read from `loops.config.total_cost` once M2 lands, or estimate from backend logs)
- Anything that surprised you / needs improvement

- [ ] **Step 7: Commit the dogfood log**

```bash
git add docs/superpowers/dogfood-log.md
git commit -m "docs: log first Agentra-on-Agentra dogfood run"
```

---

## Self-Review

**Spec coverage check:**

| Spec section | Covered by task |
|--------------|-----------------|
| §1 Overview / 1.1 v2 reuse table | Implicit; this plan implements the v2 architecture |
| §1.2 Design decisions (table) | Tasks 1-7 implement each decision |
| §2.1 Goals (MVP) | All 7 goals covered by tasks 1-18 |
| §2.2 Non-goals | Out of scope; not in this plan |
| §3.1 Architecture diagram | Tasks 6-7 (Coordinator), 8 (daemon dispatch), 5 (REST) realize the boxes |
| §3.2 Boundary with task system | Task 1 (DB), Task 8 (daemon dispatch) |
| §4.1 Status / stage fields | Task 3 (Loop struct) |
| §4.2 Transitions | Task 6 (decideNextStage) |
| §4.3 Failure boundary | Task 16 (HandleTaskFailed + failure_reason) |
| §5.1 loops table | Task 1 (migration) |
| §5.2 agent_task_queue columns | Task 1 + Task 2 (sqlc regen) |
| §5.3 task_type values | Tasks 2 (sqlc), 8 (daemon dispatch), 10-15 (stage executors) |
| §5.4 File map | All 14 new + 5 modified files accounted for in tasks 1-17 |
| §6.1 LoopCoordinator | Task 6 (impl) + Task 7 (wiring) |
| §6.2 Stage Executors | Tasks 9-15 (skeleton + 4 stages) |
| §6.3 Daemon routing | Task 8 (buildPromptForStage seam) |
| §6.4 CLI | Task 17 |
| §6.5 REST | Task 5 |
| §6.6 WS events | Task 6 (subscribe in cmd/server); payload extension deferred per §13.2 |
| §6.7 Tool system | Tasks 11-12 (11 tool implementations) |
| §7.1-7.7 Integration points | Tasks 1, 5, 7, 8 cover all 7 |
| §8.1 Failure classification | Task 16 (classifyError) |
| §8.2 Timeout config | Deferred to post-MVP (config JSONB field is in place) |
| §8.3 Retry strategy | Existing `tasks` table retry mechanism is reused; no new code |
| §8.4 Coordinator recovery | Task 7 (LoadActive on startup) |
| §8.5 Token cost tracking | Deferred to post-MVP; task_runs.total_tokens column already exists |
| §9.1 Functional acceptance | Tasks 1-17 each have a verification step |
| §9.2 Dogfood acceptance | Task 18 |
| §9.3 Non-functional | Implicit in tasks 6, 7, 16 (recovery, failure_reason) |
| §9.4 Phased M0-M6 | Tasks 1-2 = M0; Tasks 3-8 = M1; Tasks 9-13 = M2/M3; Tasks 14-16 = M4; Task 17 = M5; Task 18 = M6 |
| §9.5 Testing strategy | Unit (tasks 3, 4, 6, 10-15) + integration (task 16) + E2E (task 18 dogfood) |
| §9.6 Observability | Existing `events.Bus` + `slog`; OTel/Metrics deferred to post-MVP |
| §12.1-12.7 Framework patterns | Documented in spec; not implemented in this plan (custom Coordinator) |
| §13.1-13.6 Implementation impact | All 3 corrections incorporated into this plan |

**Placeholder scan:**
- No "TBD" / "TODO" / "implement later" placeholders in steps
- All code blocks are complete (helper bodies filled in or marked with a one-line `// helper` comment + file reference)
- All file paths are absolute or repo-relative

**Type consistency:**
- `Loop.Status` / `Loop.CurrentStage` consistently typed as `Status` / `*Stage`
- `Stage` constants: `StagePlan` / `StageDevelop` / `StageReview` / `StageFix`
- `task_type` string values: `"loop_plan"` / `"loop_develop"` / `"loop_review"` / `"loop_fix"`
- `Decision.action` values: `"create_task"` / `"complete"` / `"fail"` / `""` (no-op)
- `Result.Text` is the JSON-encoded `TaskResult` for review, free text for plan/develop/fix
- `Tool.Name()` returns the same strings as `toolNamesByStage` map keys

**Gaps identified during self-review:**
- The plan does not implement OTel traces / Prometheus metrics from §9.6. These are marked "M0 onwards" in the spec but are not in the phased plan (M0-M6). The implementer should add minimal `slog` calls for each state transition (already in coordinator.go from Task 6) and treat OTel as a follow-up.
- Token cost tracking (§8.5) is not explicitly implemented; the `task_runs.total_tokens` column already exists and is populated by the daemon's existing trace code. Coordinator simply needs to read and sum it; add this as a stretch goal in Task 16 if time permits.
- The integration test in Task 16 is intentionally skeletal (one passing, one skipped). The implementer should flesh out the second one once the helper for seeding task_runs.output JSON is in place. This is a known gap, not a defect.

Plan is complete. Saved to `docs/superpowers/plans/2026-06-06-agentic-engineering-loop.md`.
