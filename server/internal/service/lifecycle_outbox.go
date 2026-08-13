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
	"github.com/agentra-ai/agentra/server/internal/mention"
	"github.com/agentra-ai/agentra/server/internal/util"
	db "github.com/agentra-ai/agentra/server/pkg/db/generated"
	"github.com/agentra-ai/agentra/server/pkg/protocol"
	"github.com/agentra-ai/agentra/server/pkg/redact"
)

const lifecycleOutboxPollInterval = 250 * time.Millisecond

// LifecycleOutboxWorker is the recovery Adapter for durable Lifecycle facts.
// Each database projection is idempotent by event_id or run_id; realtime is
// intentionally at-least-once and frontend Run gating absorbs duplicates.
type LifecycleOutboxWorker struct {
	queries *db.Queries
	bus     *events.Bus
	trace   *TraceService
}

func NewLifecycleOutboxWorker(queries *db.Queries, bus *events.Bus, trace *TraceService) *LifecycleOutboxWorker {
	return &LifecycleOutboxWorker{queries: queries, bus: bus, trace: trace}
}

func (w *LifecycleOutboxWorker) Run(ctx context.Context) {
	if w == nil || w.queries == nil {
		return
	}
	w.drain(ctx)
	ticker := time.NewTicker(lifecycleOutboxPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.drain(ctx)
		}
	}
}

func (w *LifecycleOutboxWorker) drain(ctx context.Context) {
	for {
		processed, err := w.ProcessNext(ctx)
		if err != nil {
			slog.Warn("lifecycle outbox projection failed", "error", err)
			return
		}
		if !processed {
			return
		}
	}
}

// ProcessNext claims and projects one event. It is exported so crash/replay
// integration tests can drive the worker deterministically without sleeping.
func (w *LifecycleOutboxWorker) ProcessNext(ctx context.Context) (bool, error) {
	if w == nil || w.queries == nil {
		return false, ErrLifecycleUnavailable
	}
	event, err := w.queries.ClaimLifecycleOutboxEvent(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("claim lifecycle event: %w", err)
	}
	return true, w.processClaimed(ctx, event)
}

// processClaimed projects and acknowledges an event whose fencing lease is
// already held by this worker. Keeping claim separate makes the crash window
// directly testable without assuming a shared integration database has no
// other pending events.
func (w *LifecycleOutboxWorker) processClaimed(ctx context.Context, event db.LifecycleOutbox) error {
	if err := w.project(ctx, event); err != nil {
		_, releaseErr := w.queries.ReleaseLifecycleOutboxEvent(ctx, db.ReleaseLifecycleOutboxEventParams{
			ID: event.ID, LockToken: event.LockToken,
			LastError: pgtype.Text{String: err.Error(), Valid: true},
		})
		if releaseErr != nil {
			return fmt.Errorf("project lifecycle event: %v; release: %w", err, releaseErr)
		}
		return err
	}
	updated, err := w.queries.MarkLifecycleOutboxEventProcessed(ctx, db.MarkLifecycleOutboxEventProcessedParams{
		ID: event.ID, LockToken: event.LockToken,
	})
	if err != nil {
		return fmt.Errorf("mark lifecycle event processed: %w", err)
	}
	if updated != 1 {
		return fmt.Errorf("lifecycle event lease lost before acknowledgement")
	}
	return nil
}

func (w *LifecycleOutboxWorker) project(ctx context.Context, event db.LifecycleOutbox) error {
	if event.EventVersion != lifecycleEventVersion {
		return fmt.Errorf("unsupported lifecycle event version %d", event.EventVersion)
	}
	task, err := w.queries.GetAgentTask(ctx, event.WorkItemID)
	if err != nil {
		return fmt.Errorf("load work item: %w", err)
	}
	issue, err := w.queries.GetIssue(ctx, task.IssueID)
	if err != nil {
		return fmt.Errorf("load issue: %w", err)
	}
	if event.EventType != LifecycleEventRunCancelled && event.EventType != LifecycleEventWorkRejected && !event.RunID.Valid {
		return fmt.Errorf("lifecycle event %q requires a run id", event.EventType)
	}

	var run db.TaskRun
	if event.RunID.Valid {
		run, err = w.queries.GetTaskRun(ctx, event.RunID)
		if err != nil {
			return fmt.Errorf("load run: %w", err)
		}
		if err := w.projectTrace(ctx, event.EventType, task, run); err != nil {
			return err
		}
	}

	switch event.EventType {
	case LifecycleEventRunStarted, LifecycleEventRunCheckpoint:
		return nil
	case LifecycleEventRunCompleted:
		w.reconcileAgentStatus(ctx, task.AgentID)
		if err := w.projectCompletionComment(ctx, event, issue, task, run); err != nil {
			return err
		}
		if err := w.projectMetric(ctx, event, issue, task, run, "completed"); err != nil {
			return err
		}
		return w.publishTaskEvent(event, issue.WorkspaceID, task, protocol.EventTaskCompleted, "completed")
	case LifecycleEventRunFailed:
		w.reconcileAgentStatus(ctx, task.AgentID)
		if err := w.projectFailureComment(ctx, event, issue, task, run); err != nil {
			return err
		}
		if err := w.projectMetric(ctx, event, issue, task, run, "failed"); err != nil {
			return err
		}
		return w.publishTaskEvent(event, issue.WorkspaceID, task, protocol.EventTaskFailed, "failed")
	case LifecycleEventRunRetry:
		w.reconcileAgentStatus(ctx, task.AgentID)
		if err := w.projectMetric(ctx, event, issue, task, run, "failed"); err != nil {
			return err
		}
		return w.publishTaskEvent(event, issue.WorkspaceID, task, protocol.EventTaskRetry, "queued")
	case LifecycleEventRunCancelled:
		w.reconcileAgentStatus(ctx, task.AgentID)
		if event.RunID.Valid {
			if err := w.projectMetric(ctx, event, issue, task, run, "cancelled"); err != nil {
				return err
			}
		}
		return w.publishTaskEvent(event, issue.WorkspaceID, task, protocol.EventTaskCancelled, "cancelled")
	case LifecycleEventWorkRejected:
		w.reconcileAgentStatus(ctx, task.AgentID)
		if task.Error.Valid && task.Error.String != "" {
			if err := w.createLifecycleComment(ctx, event.ID, issue, task, redact.Text(task.Error.String), "system"); err != nil {
				return err
			}
		}
		return w.publishTaskEvent(event, issue.WorkspaceID, task, protocol.EventTaskFailed, "failed")
	default:
		return fmt.Errorf("unsupported lifecycle event type %q", event.EventType)
	}
}

func (w *LifecycleOutboxWorker) projectTrace(ctx context.Context, eventType string, task db.AgentTaskQueue, run db.TaskRun) error {
	if w.trace == nil || w.trace.TraceService == nil {
		return nil
	}
	provider, model := w.resolveAgentProvider(ctx, task.AgentID)
	trace, err := w.trace.StartTrace(ctx,
		util.UUIDToString(run.ID), util.UUIDToString(task.ID),
		util.UUIDToString(task.AgentID), util.UUIDToString(task.IssueID),
		provider, model,
	)
	if err != nil {
		return fmt.Errorf("ensure execution trace: %w", err)
	}
	status := ""
	switch eventType {
	case LifecycleEventRunCompleted:
		status = "completed"
	case LifecycleEventRunFailed, LifecycleEventRunRetry:
		status = "failed"
	case LifecycleEventRunCancelled:
		status = "aborted"
	}
	if status == "" || trace.Status == status {
		return nil
	}
	if err := w.trace.EndTrace(ctx, trace.ID, status); err != nil {
		return fmt.Errorf("end execution trace: %w", err)
	}
	return nil
}

func (w *LifecycleOutboxWorker) projectCompletionComment(ctx context.Context, event db.LifecycleOutbox, issue db.Issue, task db.AgentTaskQueue, run db.TaskRun) error {
	if task.TriggerCommentID.Valid || !run.Output.Valid {
		return nil
	}
	var payload protocol.TaskCompletedPayload
	if err := json.Unmarshal([]byte(run.Output.String), &payload); err != nil || payload.Output == "" {
		return nil
	}
	return w.createLifecycleComment(ctx, event.ID, issue, task, redact.Text(payload.Output), "comment")
}

func (w *LifecycleOutboxWorker) projectFailureComment(ctx context.Context, event db.LifecycleOutbox, issue db.Issue, task db.AgentTaskQueue, run db.TaskRun) error {
	if !run.Error.Valid || run.Error.String == "" {
		return nil
	}
	return w.createLifecycleComment(ctx, event.ID, issue, task, redact.Text(run.Error.String), "system")
}

func (w *LifecycleOutboxWorker) createLifecycleComment(ctx context.Context, eventID pgtype.UUID, issue db.Issue, task db.AgentTaskQueue, content, commentType string) error {
	content = mention.ExpandIssueIdentifiers(ctx, w.queries, issue.WorkspaceID, content)
	comment, err := w.queries.CreateCommentForLifecycleEvent(ctx, db.CreateCommentForLifecycleEventParams{
		IssueID: issue.ID, WorkspaceID: issue.WorkspaceID,
		AuthorType: "agent", AuthorID: task.AgentID,
		Content: content, Type: commentType,
		LifecycleEventID: eventID, ParentID: task.TriggerCommentID,
	})
	if err != nil {
		return fmt.Errorf("project lifecycle comment: %w", err)
	}
	if w.bus != nil && commentType == "comment" {
		w.bus.Publish(events.Event{
			ID: util.UUIDToString(eventID), Type: protocol.EventCommentCreated,
			WorkspaceID: util.UUIDToString(issue.WorkspaceID), ActorType: "agent",
			ActorID: util.UUIDToString(task.AgentID),
			Payload: map[string]any{"comment": map[string]any{
				"id": util.UUIDToString(comment.ID), "issue_id": util.UUIDToString(comment.IssueID),
				"author_type": comment.AuthorType, "author_id": util.UUIDToString(comment.AuthorID),
				"content": comment.Content, "type": comment.Type,
				"parent_id":  util.UUIDToPtr(comment.ParentID),
				"created_at": comment.CreatedAt.Time.Format(time.RFC3339),
			}, "issue_title": issue.Title, "issue_status": issue.Status},
		})
	}
	return nil
}

func (w *LifecycleOutboxWorker) projectMetric(ctx context.Context, event db.LifecycleOutbox, issue db.Issue, task db.AgentTaskQueue, run db.TaskRun, status string) error {
	var payload protocol.TaskCompletedPayload
	if run.Output.Valid {
		_ = json.Unmarshal([]byte(run.Output.String), &payload)
	}
	tokenInput, tokenOutput := int64(0), int64(0)
	if payload.TokenUsage != nil {
		tokenInput = payload.TokenUsage.InputTokens
		tokenOutput = payload.TokenUsage.OutputTokens + payload.TokenUsage.ReasoningOutputTokens
	}
	provider, model := w.resolveAgentProvider(ctx, task.AgentID)
	_, err := w.queries.InsertAgentTaskMetric(ctx, db.InsertAgentTaskMetricParams{
		WorkspaceID: issue.WorkspaceID, TaskID: task.ID, IssueID: task.IssueID, RunID: event.RunID,
		Provider: provider, Model: model, RuntimeMode: "local",
		TaskType: normalizeMetricTaskType(task.TaskType), IssuePriority: issue.Priority,
		Status: status, ErrorCategory: run.Error, DurationMs: int64(run.DurationMs.Int32),
		TokenInput: clampInt32(tokenInput), TokenOutput: clampInt32(tokenOutput),
	})
	if err != nil {
		return fmt.Errorf("project lifecycle metric: %w", err)
	}
	return nil
}

func (w *LifecycleOutboxWorker) publishTaskEvent(event db.LifecycleOutbox, workspaceID pgtype.UUID, task db.AgentTaskQueue, eventType, status string) error {
	if w.bus == nil {
		return nil
	}
	payload := map[string]any{
		"task_id": util.UUIDToString(task.ID), "agent_id": util.UUIDToString(task.AgentID),
		"issue_id": util.UUIDToString(task.IssueID), "status": status,
	}
	if event.RunID.Valid {
		payload["run_id"] = util.UUIDToString(event.RunID)
	}
	w.bus.Publish(events.Event{
		ID: util.UUIDToString(event.ID), Type: eventType,
		WorkspaceID: util.UUIDToString(workspaceID), ActorType: "system", Payload: payload,
	})
	return nil
}

func (w *LifecycleOutboxWorker) resolveAgentProvider(ctx context.Context, agentID pgtype.UUID) (string, string) {
	agent, err := w.queries.GetAgent(ctx, agentID)
	if err != nil {
		return "", ""
	}
	model := ""
	if agent.ModelOverride.Valid {
		model = agent.ModelOverride.String
	}
	return agent.Provider, model
}

func (w *LifecycleOutboxWorker) reconcileAgentStatus(ctx context.Context, agentID pgtype.UUID) {
	running, err := w.queries.CountRunningTasks(ctx, agentID)
	if err != nil {
		return
	}
	status := "idle"
	if running > 0 {
		status = "working"
	}
	agent, err := w.queries.UpdateAgentStatus(ctx, db.UpdateAgentStatusParams{ID: agentID, Status: status})
	if err != nil || w.bus == nil {
		return
	}
	w.bus.Publish(events.Event{
		Type: protocol.EventAgentStatus, WorkspaceID: util.UUIDToString(agent.WorkspaceID),
		ActorType: "system", Payload: map[string]any{"agent": agentToMap(agent)},
	})
}
