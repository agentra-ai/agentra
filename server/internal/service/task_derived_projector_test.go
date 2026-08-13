package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/agentra-ai/agentra/server/internal/events"
	"github.com/agentra-ai/agentra/server/internal/util"
	db "github.com/agentra-ai/agentra/server/pkg/db/generated"
	"github.com/agentra-ai/agentra/server/pkg/protocol"
)

func TestTaskDerivedProjectorPersistsFailureActivityAndInbox(t *testing.T) {
	ctx, pool, q := lifecycleBatchPool(t)
	agent, cleanup := createMetricTestFixture(t, ctx, pool, q)
	defer cleanup()
	issue := createLifecycleBatchIssue(t, ctx, q, agent)
	if err := q.AddIssueSubscriber(ctx, db.AddIssueSubscriberParams{
		IssueID: issue.ID, UserType: "member", UserID: agent.OwnerID, Reason: "manual",
	}); err != nil {
		t.Fatal(err)
	}
	task, runID := seedTaskDerivedRunning(t, ctx, pool, q, agent, issue)
	lifecycle := NewRunLifecycle(pool, q)
	if _, err := lifecycle.Fail(ctx, RunRef{WorkItemID: task.ID, RunID: runID}, "provider failed"); err != nil {
		t.Fatal(err)
	}
	eventID := prepareTaskDerivedEvent(t, ctx, pool, task.ID, LifecycleEventRunFailed, "1800-01-01T00:00:00Z")

	bus := events.New()
	var mu sync.Mutex
	activityEvents, inboxEvents := 0, 0
	bus.Subscribe(protocol.EventActivityCreated, func(events.Event) { mu.Lock(); activityEvents++; mu.Unlock() })
	bus.Subscribe(protocol.EventInboxNew, func(events.Event) { mu.Lock(); inboxEvents++; mu.Unlock() })
	projector := NewTaskDerivedLifecycleProjector(pool, q, bus)
	processed, err := projector.ProcessNext(ctx)
	if err != nil || !processed {
		t.Fatalf("task-derived failure projection = processed:%v err:%v", processed, err)
	}

	var activities, inbox, receipts int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM activity_log WHERE lifecycle_event_id = $1`, eventID).Scan(&activities); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM inbox_item WHERE lifecycle_event_id = $1`, eventID).Scan(&inbox); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM lifecycle_event_receipt
		WHERE event_id = $1 AND consumer = 'task-derived'
	`, eventID).Scan(&receipts); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if activities != 1 || inbox != 1 || receipts != 1 || activityEvents != 1 || inboxEvents != 1 {
		t.Fatalf("failure projection = activity:%d inbox:%d receipt:%d realtime:%d/%d", activities, inbox, receipts, activityEvents, inboxEvents)
	}
}

func TestTaskDerivedProjectorOwnsCompletionCommentSubscribersAndMentions(t *testing.T) {
	ctx, pool, q := lifecycleBatchPool(t)
	agent, cleanup := createMetricTestFixture(t, ctx, pool, q)
	defer cleanup()
	issue := createLifecycleBatchIssue(t, ctx, q, agent)
	if err := q.AddIssueSubscriber(ctx, db.AddIssueSubscriberParams{
		IssueID: issue.ID, UserType: "member", UserID: agent.OwnerID, Reason: "manual",
	}); err != nil {
		t.Fatal(err)
	}
	mentionedUser := createTaskDerivedMember(t, ctx, pool, agent.WorkspaceID)
	task, runID := seedTaskDerivedRunning(t, ctx, pool, q, agent, issue)
	content := fmt.Sprintf("done [@Reviewer](mention://member/%s)", util.UUIDToString(mentionedUser))
	result, err := json.Marshal(protocol.TaskCompletedPayload{Output: content})
	if err != nil {
		t.Fatal(err)
	}
	lifecycle := NewRunLifecycle(pool, q)
	if _, err := lifecycle.Complete(ctx, RunRef{WorkItemID: task.ID, RunID: runID}, RunCompletion{Result: result}); err != nil {
		t.Fatal(err)
	}
	eventID := prepareTaskDerivedEvent(t, ctx, pool, task.ID, LifecycleEventRunCompleted, "1700-01-01T00:00:00Z")
	if _, err := q.CreateCommentForLifecycleEvent(ctx, db.CreateCommentForLifecycleEventParams{
		IssueID: issue.ID, WorkspaceID: issue.WorkspaceID,
		AuthorType: "agent", AuthorID: agent.ID, Content: content, Type: "comment",
		LifecycleEventID: eventID,
	}); err != nil {
		t.Fatal(err)
	}

	projector := NewTaskDerivedLifecycleProjector(pool, q, events.New())
	processed, err := projector.ProcessNext(ctx)
	if err != nil || !processed {
		t.Fatalf("task-derived completion projection = processed:%v err:%v", processed, err)
	}
	rows, err := pool.Query(ctx, `
		SELECT recipient_id, type FROM inbox_item
		WHERE lifecycle_event_id = $1 ORDER BY type
	`, eventID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got := map[string]string{}
	for rows.Next() {
		var recipient, notificationType string
		if err := rows.Scan(&recipient, &notificationType); err != nil {
			t.Fatal(err)
		}
		got[recipient] = notificationType
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if got[util.UUIDToString(agent.OwnerID)] != "new_comment" || got[util.UUIDToString(mentionedUser)] != "mentioned" || len(got) != 2 {
		t.Fatalf("completion comment inbox projections = %#v", got)
	}
	var subscribed bool
	if err := pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM issue_subscriber
			WHERE issue_id = $1 AND user_type = 'agent' AND user_id = $2 AND reason = 'commenter'
		)
	`, issue.ID, agent.ID).Scan(&subscribed); err != nil {
		t.Fatal(err)
	}
	if !subscribed {
		t.Fatal("completion comment author was not subscribed by durable projection")
	}
}

func TestTaskDerivedProjectorRecordsFailureWithoutPartialProjection(t *testing.T) {
	ctx, pool, q := lifecycleBatchPool(t)
	agent, cleanup := createMetricTestFixture(t, ctx, pool, q)
	defer cleanup()
	issue := createLifecycleBatchIssue(t, ctx, q, agent)
	task, runID := seedTaskDerivedRunning(t, ctx, pool, q, agent, issue)
	lifecycle := NewRunLifecycle(pool, q)
	if _, err := lifecycle.Fail(ctx, RunRef{WorkItemID: task.ID, RunID: runID}, "bad version"); err != nil {
		t.Fatal(err)
	}
	eventID := prepareTaskDerivedEvent(t, ctx, pool, task.ID, LifecycleEventRunFailed, "1600-01-01T00:00:00Z")
	if _, err := pool.Exec(ctx, `UPDATE lifecycle_outbox SET event_version = 99 WHERE id = $1`, eventID); err != nil {
		t.Fatal(err)
	}

	projector := NewTaskDerivedLifecycleProjector(pool, q, nil)
	processed, err := projector.ProcessNext(ctx)
	if err == nil || !processed {
		t.Fatalf("invalid version projection = processed:%v err:%v", processed, err)
	}
	var activities, receipts, attempts int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM activity_log WHERE lifecycle_event_id = $1`, eventID).Scan(&activities); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM lifecycle_event_receipt WHERE event_id = $1 AND consumer = 'task-derived'`, eventID).Scan(&receipts); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT attempts FROM lifecycle_event_delivery WHERE event_id = $1 AND consumer = 'task-derived'`, eventID).Scan(&attempts); err != nil {
		t.Fatal(err)
	}
	if activities != 0 || receipts != 0 || attempts != 1 {
		t.Fatalf("failed projection = activity:%d receipt:%d attempts:%d", activities, receipts, attempts)
	}
}

func seedTaskDerivedRunning(t *testing.T, ctx context.Context, pool *pgxpool.Pool, q *db.Queries, agent db.Agent, issue db.Issue) (db.AgentTaskQueue, pgtype.UUID) {
	t.Helper()
	task, err := q.CreateAgentTask(ctx, db.CreateAgentTaskParams{
		AgentID: agent.ID, RuntimeID: agent.RuntimeID, IssueID: issue.ID,
		Priority: 1, TaskType: "standard",
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
		UPDATE agent_task_queue SET status = 'running', started_at = now(), active_run_id = $2
		WHERE id = $1
	`, task.ID, runID); err != nil {
		t.Fatal(err)
	}
	return task, runID
}

func prepareTaskDerivedEvent(t *testing.T, ctx context.Context, pool *pgxpool.Pool, taskID pgtype.UUID, eventType, createdAt string) pgtype.UUID {
	t.Helper()
	var eventID pgtype.UUID
	if err := pool.QueryRow(ctx, `
		UPDATE lifecycle_outbox
		SET processed_at = now(), created_at = $3
		WHERE work_item_id = $1 AND event_type = $2
		RETURNING id
	`, taskID, eventType, createdAt).Scan(&eventID); err != nil {
		t.Fatal(err)
	}
	return eventID
}

func createTaskDerivedMember(t *testing.T, ctx context.Context, pool *pgxpool.Pool, workspaceID pgtype.UUID) pgtype.UUID {
	t.Helper()
	var userID pgtype.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO "user" (name, email) VALUES ('Mentioned User', $1) RETURNING id
	`, fmt.Sprintf("mentioned-%d@agentra.ai", time.Now().UnixNano())).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO member (workspace_id, user_id, role) VALUES ($1, $2, 'member')
	`, workspaceID, userID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM "user" WHERE id = $1`, userID) })
	return userID
}
