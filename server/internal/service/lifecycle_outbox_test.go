package service

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/agentra-ai/agentra/server/internal/events"
	"github.com/agentra-ai/agentra/server/internal/util"
	db "github.com/agentra-ai/agentra/server/pkg/db/generated"
	"github.com/agentra-ai/agentra/server/pkg/protocol"
)

func TestLifecycleOutboxCoreProjectionIsIdempotent(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping lifecycle outbox integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	q := db.New(pool)
	agent, cleanup := createMetricTestFixture(t, ctx, pool, q)
	defer cleanup()
	issue, err := q.CreateIssue(ctx, db.CreateIssueParams{
		WorkspaceID: agent.WorkspaceID, Title: "lifecycle outbox projection",
		Status: "todo", Priority: "high", CreatorType: "member", CreatorID: agent.OwnerID,
	})
	if err != nil {
		t.Fatal(err)
	}
	task, err := q.CreateAgentTask(ctx, db.CreateAgentTaskParams{
		AgentID: agent.ID, RuntimeID: agent.RuntimeID, IssueID: issue.ID,
		Priority: 1, RuntimeType: "local", TaskType: "standard",
	})
	if err != nil {
		t.Fatal(err)
	}
	runID := uuid.NewString()
	result, _ := json.Marshal(protocol.TaskCompletedPayload{
		Output: "durable output", DurationMs: 321,
		TokenUsage: &protocol.TaskTokenUsage{InputTokens: 12, OutputTokens: 34},
	})
	if _, err := pool.Exec(ctx, `
		UPDATE agent_task_queue SET status = 'completed', completed_at = now(), result = $2 WHERE id = $1
	`, task.ID, result); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO task_runs (
			id, task_id, agent_id, status, started_at, completed_at, duration_ms,
			total_tokens, output, created_at
		) VALUES ($3, $1, $4, 'completed', now(), now(), 321, 46, $2, now())
	`, task.ID, result, runID, agent.ID); err != nil {
		t.Fatal(err)
	}
	event, err := q.AppendLifecycleOutboxEvent(ctx, db.AppendLifecycleOutboxEventParams{
		WorkItemID: task.ID, RunID: util.ParseUUID(runID),
		EventType: LifecycleEventRunCompleted, EventVersion: 1, Payload: []byte(`{"status":"completed"}`),
	})
	if err != nil {
		t.Fatal(err)
	}

	bus := events.New()
	broadcasts := 0
	bus.Subscribe(protocol.EventTaskCompleted, func(e events.Event) {
		if e.ID == util.UUIDToString(event.ID) {
			broadcasts++
		}
	})
	worker := NewLifecycleOutboxWorker(q, bus, nil)

	// Simulate a crash after projection side effects but before acknowledgement:
	// the same event is projected again on restart.
	if err := worker.project(ctx, event); err != nil {
		t.Fatal(err)
	}
	if err := worker.project(ctx, event); err != nil {
		t.Fatal(err)
	}
	var comments, metrics int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM comment WHERE lifecycle_event_id = $1`, event.ID).Scan(&comments); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM agent_task_metrics WHERE run_id = $1`, runID).Scan(&metrics); err != nil {
		t.Fatal(err)
	}
	if comments != 1 || metrics != 1 {
		t.Fatalf("replayed projections = comments:%d metrics:%d, want 1/1", comments, metrics)
	}
	if broadcasts != 2 {
		t.Fatalf("at-least-once realtime broadcasts = %d, want 2", broadcasts)
	}

	lockToken := uuid.NewString()
	if _, err := pool.Exec(ctx, `
		UPDATE lifecycle_outbox SET locked_at = now(), lock_token = $2 WHERE id = $1
	`, event.ID, lockToken); err != nil {
		t.Fatal(err)
	}
	event.LockToken = util.ParseUUID(lockToken)
	if err := worker.processClaimed(ctx, event); err != nil {
		t.Fatalf("process claimed event: %v", err)
	}
	stored, err := q.GetLifecycleOutboxEvent(ctx, event.ID)
	if err != nil || !stored.ProcessedAt.Valid {
		t.Fatalf("processed_at = %v err:%v", stored.ProcessedAt, err)
	}
}
