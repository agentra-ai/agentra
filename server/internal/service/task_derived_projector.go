package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/agentra-ai/agentra/server/internal/events"
	"github.com/agentra-ai/agentra/server/internal/util"
	db "github.com/agentra-ai/agentra/server/pkg/db/generated"
	"github.com/agentra-ai/agentra/server/pkg/protocol"
)

const taskDerivedPollInterval = 250 * time.Millisecond

// TaskDerivedLifecycleProjector owns correctness-critical activity and inbox
// projections for terminal lifecycle facts. Its receipt commits in the same
// transaction as every database projection, so a listener error can no longer
// be hidden by the core outbox acknowledgement.
type TaskDerivedLifecycleProjector struct {
	starter runLifecycleTxStarter
	queries *db.Queries
	bus     *events.Bus
}

func NewTaskDerivedLifecycleProjector(starter runLifecycleTxStarter, queries *db.Queries, bus *events.Bus) *TaskDerivedLifecycleProjector {
	return &TaskDerivedLifecycleProjector{starter: starter, queries: queries, bus: bus}
}

func (p *TaskDerivedLifecycleProjector) Run(ctx context.Context) {
	if p == nil || p.starter == nil || p.queries == nil {
		return
	}
	p.Drain(ctx)
	ticker := time.NewTicker(taskDerivedPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.Drain(ctx)
		}
	}
}

func (p *TaskDerivedLifecycleProjector) Drain(ctx context.Context) {
	for {
		processed, err := p.ProcessNext(ctx)
		if err != nil {
			slog.Warn("task-derived lifecycle projection failed", "error", err)
			return
		}
		if !processed {
			return
		}
	}
}

// ProcessNext commits activity, subscriptions, all subscriber inbox items, and
// the consumer receipt atomically. Realtime notifications are emitted only
// after commit and remain a non-authoritative hint; clients can refetch the
// durable rows.
func (p *TaskDerivedLifecycleProjector) ProcessNext(ctx context.Context) (bool, error) {
	if p == nil || p.starter == nil || p.queries == nil {
		return false, ErrLifecycleUnavailable
	}
	tx, err := p.starter.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin task-derived projection: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	q := p.queries.WithTx(tx)
	event, err := q.ClaimTaskDerivedLifecycleEvent(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("claim task-derived event: %w", err)
	}
	realtime, err := p.apply(ctx, q, event)
	if err != nil {
		_ = tx.Rollback(ctx)
		failureErr := p.queries.RecordTaskDerivedLifecycleFailure(ctx, db.RecordTaskDerivedLifecycleFailureParams{
			EventID: event.ID, LastError: pgtype.Text{String: err.Error(), Valid: true},
		})
		if failureErr != nil {
			return true, fmt.Errorf("apply task-derived event: %v; record failure: %w", err, failureErr)
		}
		return true, err
	}
	if err := q.RecordTaskDerivedLifecycleReceipt(ctx, event.ID); err != nil {
		return true, fmt.Errorf("record task-derived receipt: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return true, fmt.Errorf("commit task-derived projection: %w", err)
	}
	if p.bus != nil {
		for _, realtimeEvent := range realtime {
			p.bus.Publish(realtimeEvent)
		}
	}
	return true, nil
}

func (p *TaskDerivedLifecycleProjector) apply(ctx context.Context, q *db.Queries, event db.LifecycleOutbox) ([]events.Event, error) {
	if event.EventVersion != lifecycleEventVersion {
		return nil, fmt.Errorf("unsupported lifecycle event version %d", event.EventVersion)
	}
	task, err := q.GetAgentTask(ctx, event.WorkItemID)
	if err != nil {
		return nil, fmt.Errorf("load task-derived Work Item: %w", err)
	}
	issue, err := q.GetIssue(ctx, task.IssueID)
	if err != nil {
		return nil, fmt.Errorf("load task-derived Issue: %w", err)
	}

	action := "task_completed"
	failed := false
	switch event.EventType {
	case LifecycleEventRunCompleted:
	case LifecycleEventRunFailed, LifecycleEventWorkRejected:
		action, failed = "task_failed", true
	default:
		return nil, fmt.Errorf("unsupported task-derived event %q", event.EventType)
	}
	activity, err := q.CreateActivityForLifecycleEvent(ctx, db.CreateActivityForLifecycleEventParams{
		WorkspaceID: issue.WorkspaceID, IssueID: issue.ID,
		ActorType: pgtype.Text{String: "agent", Valid: true}, ActorID: task.AgentID,
		Action: action, Details: []byte("{}"), LifecycleEventID: event.ID,
	})
	if err != nil {
		return nil, fmt.Errorf("project task activity: %w", err)
	}
	realtime := []events.Event{taskActivityRealtimeEvent(issue.WorkspaceID, activity)}
	if failed {
		return p.projectFailureInbox(ctx, q, event, task, issue, realtime)
	}
	return p.projectCompletionCommentInbox(ctx, q, event, task, issue, realtime)
}

func (p *TaskDerivedLifecycleProjector) projectFailureInbox(
	ctx context.Context,
	q *db.Queries,
	event db.LifecycleOutbox,
	task db.AgentTaskQueue,
	issue db.Issue,
	realtime []events.Event,
) ([]events.Event, error) {
	subscribers, err := q.ListIssueSubscribers(ctx, issue.ID)
	if err != nil {
		return nil, fmt.Errorf("list task failure subscribers: %w", err)
	}
	for _, subscriber := range subscribers {
		if subscriber.UserType != "member" {
			continue
		}
		item, err := projectTaskInboxItem(ctx, q, event.ID, task.AgentID, issue, subscriber.UserID,
			"task_failed", "action_required", pgtype.Text{}, []byte("{}"))
		if err != nil {
			return nil, fmt.Errorf("project task failure inbox item: %w", err)
		}
		realtime = append(realtime, taskInboxRealtimeEvent(issue.Status, item))
	}
	return realtime, nil
}

func (p *TaskDerivedLifecycleProjector) projectCompletionCommentInbox(
	ctx context.Context,
	q *db.Queries,
	event db.LifecycleOutbox,
	task db.AgentTaskQueue,
	issue db.Issue,
	realtime []events.Event,
) ([]events.Event, error) {
	comment, err := q.GetCommentForLifecycleEvent(ctx, event.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return realtime, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load lifecycle completion comment: %w", err)
	}
	if err := q.AddIssueSubscriber(ctx, db.AddIssueSubscriberParams{
		IssueID: issue.ID, UserType: comment.AuthorType, UserID: comment.AuthorID, Reason: "commenter",
	}); err != nil {
		return nil, fmt.Errorf("subscribe lifecycle comment author: %w", err)
	}
	realtime = append(realtime, taskSubscriberRealtimeEvent(issue.WorkspaceID, issue.ID, comment.AuthorType, comment.AuthorID))
	details, err := json.Marshal(map[string]string{"comment_id": util.UUIDToString(comment.ID)})
	if err != nil {
		return nil, fmt.Errorf("marshal lifecycle comment details: %w", err)
	}
	notified := map[string]bool{}
	subscribers, err := q.ListIssueSubscribers(ctx, issue.ID)
	if err != nil {
		return nil, fmt.Errorf("list lifecycle comment subscribers: %w", err)
	}
	for _, subscriber := range subscribers {
		if subscriber.UserType != "member" {
			continue
		}
		recipientID := util.UUIDToString(subscriber.UserID)
		item, err := projectTaskInboxItem(ctx, q, event.ID, task.AgentID, issue, subscriber.UserID,
			"new_comment", "info", pgtype.Text{String: comment.Content, Valid: true}, details)
		if err != nil {
			return nil, fmt.Errorf("project lifecycle comment subscriber inbox item: %w", err)
		}
		notified[recipientID] = true
		realtime = append(realtime, taskInboxRealtimeEvent(issue.Status, item))
	}

	recipients := map[string]pgtype.UUID{}
	mentions := util.ParseMentions(comment.Content)
	for _, mention := range mentions {
		if mention.Type == "member" {
			id := util.ParseUUID(mention.ID)
			if id.Valid {
				recipients[mention.ID] = id
			}
		}
		if mention.Type == "all" {
			members, err := q.ListMembers(ctx, issue.WorkspaceID)
			if err != nil {
				return nil, fmt.Errorf("expand lifecycle comment @all: %w", err)
			}
			for _, member := range members {
				recipients[util.UUIDToString(member.UserID)] = member.UserID
			}
		}
	}
	for recipientID, recipientUUID := range recipients {
		if notified[recipientID] {
			continue
		}
		item, err := projectTaskInboxItem(ctx, q, event.ID, task.AgentID, issue, recipientUUID,
			"mentioned", "info", pgtype.Text{}, details)
		if err != nil {
			return nil, fmt.Errorf("project lifecycle comment mention inbox item: %w", err)
		}
		realtime = append(realtime, taskInboxRealtimeEvent(issue.Status, item))
	}
	return realtime, nil
}

func projectTaskInboxItem(
	ctx context.Context,
	q *db.Queries,
	eventID pgtype.UUID,
	agentID pgtype.UUID,
	issue db.Issue,
	recipientID pgtype.UUID,
	notificationType string,
	severity string,
	body pgtype.Text,
	details []byte,
) (db.InboxItem, error) {
	return q.CreateInboxItemForLifecycleEvent(ctx, db.CreateInboxItemForLifecycleEventParams{
		WorkspaceID: issue.WorkspaceID, RecipientType: "member", RecipientID: recipientID,
		Type: notificationType, Severity: severity, IssueID: issue.ID, Title: issue.Title,
		Body: body, ActorType: pgtype.Text{String: "agent", Valid: true}, ActorID: agentID,
		Details: details, LifecycleEventID: eventID,
	})
}

func taskActivityRealtimeEvent(workspaceID pgtype.UUID, activity db.ActivityLog) events.Event {
	actorType := ""
	if activity.ActorType.Valid {
		actorType = activity.ActorType.String
	}
	return events.Event{
		Type: protocol.EventActivityCreated, WorkspaceID: util.UUIDToString(workspaceID), ActorType: "system",
		Payload: map[string]any{
			"issue_id": util.UUIDToString(activity.IssueID),
			"entry": map[string]any{
				"type": "activity", "id": util.UUIDToString(activity.ID),
				"actor_type": actorType, "actor_id": util.UUIDToString(activity.ActorID),
				"action": activity.Action, "details": json.RawMessage(activity.Details),
				"created_at": util.TimestampToString(activity.CreatedAt),
			},
		},
	}
}

func taskInboxRealtimeEvent(issueStatus string, item db.InboxItem) events.Event {
	response := map[string]any{
		"id": util.UUIDToString(item.ID), "workspace_id": util.UUIDToString(item.WorkspaceID),
		"recipient_type": item.RecipientType, "recipient_id": util.UUIDToString(item.RecipientID),
		"type": item.Type, "severity": item.Severity, "issue_id": util.UUIDToPtr(item.IssueID),
		"title": item.Title, "body": util.TextToPtr(item.Body), "read": item.Read,
		"archived": item.Archived, "created_at": util.TimestampToString(item.CreatedAt),
		"actor_type": util.TextToPtr(item.ActorType), "actor_id": util.UUIDToPtr(item.ActorID),
		"details": json.RawMessage(item.Details), "issue_status": issueStatus,
	}
	return events.Event{
		Type: protocol.EventInboxNew, WorkspaceID: util.UUIDToString(item.WorkspaceID),
		ActorType: "agent", ActorID: util.UUIDToString(item.ActorID),
		Payload: map[string]any{"item": response},
	}
}

func taskSubscriberRealtimeEvent(workspaceID, issueID pgtype.UUID, userType string, userID pgtype.UUID) events.Event {
	return events.Event{
		Type: protocol.EventSubscriberAdded, WorkspaceID: util.UUIDToString(workspaceID), ActorType: "system",
		Payload: map[string]any{
			"issue_id": util.UUIDToString(issueID), "user_type": userType,
			"user_id": util.UUIDToString(userID), "reason": "commenter",
		},
	}
}
