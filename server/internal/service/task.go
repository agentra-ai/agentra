package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/agentra-ai/agentra/server/internal/auth"
	"github.com/agentra-ai/agentra/server/internal/events"
	"github.com/agentra-ai/agentra/server/internal/loop/stages"
	"github.com/agentra-ai/agentra/server/internal/mention"
	"github.com/agentra-ai/agentra/server/internal/realtime"
	"github.com/agentra-ai/agentra/server/internal/util"
	runtimeagent "github.com/agentra-ai/agentra/server/pkg/agent"
	"github.com/agentra-ai/agentra/server/pkg/crypto"
	db "github.com/agentra-ai/agentra/server/pkg/db/generated"
	"github.com/agentra-ai/agentra/server/pkg/protocol"
	"github.com/agentra-ai/agentra/server/pkg/redact"
)

type TaskService struct {
	Queries      *db.Queries
	Hub          *realtime.Hub
	Bus          *events.Bus
	TraceService *TraceService
	Lifecycle    *RunLifecycle
}

// ClaimedTask is a dispatched Work Item plus the Run identity allocated for
// the attempt. Every Runtime Adapter must carry RunID through all callbacks.
type ClaimedTask struct {
	Task  db.AgentTaskQueue
	RunID pgtype.UUID
}

// ErrGatewayTaskNotAuthorized intentionally hides whether a task exists in a
// different workspace. Gateway callers must never be able to use task IDs as a
// cross-tenant oracle.
var ErrGatewayTaskNotAuthorized = errors.New("cloud gateway task not authorized")

func NewTaskService(q *db.Queries, txStarter runLifecycleTxStarter, hub *realtime.Hub, bus *events.Bus, traceSvc *TraceService) *TaskService {
	return &TaskService{
		Queries:      q,
		Hub:          hub,
		Bus:          bus,
		TraceService: traceSvc,
		Lifecycle:    NewRunLifecycle(txStarter, q),
	}
}

// ValidateCloudGatewayTask binds a gateway event to the active cloud runtime
// configured for its authenticated workspace.
func (s *TaskService) ValidateCloudGatewayTask(ctx context.Context, workspaceID, taskID string) (db.AgentTaskQueue, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	taskUUID := util.ParseUUID(taskID)
	if workspaceID == "" || !taskUUID.Valid {
		return db.AgentTaskQueue{}, ErrGatewayTaskNotAuthorized
	}

	task, err := s.Queries.GetAgentTask(ctx, taskUUID)
	if err != nil || task.RuntimeType != "cloud" || !task.CloudRuntimeID.Valid {
		return db.AgentTaskQueue{}, ErrGatewayTaskNotAuthorized
	}

	issue, err := s.Queries.GetIssue(ctx, task.IssueID)
	if err != nil || util.UUIDToString(issue.WorkspaceID) != workspaceID {
		return db.AgentTaskQueue{}, ErrGatewayTaskNotAuthorized
	}

	runtime, err := s.Queries.GetCloudRuntimeByID(ctx, task.CloudRuntimeID)
	if err != nil || runtime.WorkspaceID != issue.WorkspaceID {
		return db.AgentTaskQueue{}, ErrGatewayTaskNotAuthorized
	}

	return task, nil
}

// EnqueueTaskForIssue creates a queued task for an agent-assigned issue.
// No context snapshot is stored — the agent fetches all data it needs at
// runtime via the agentra CLI.
func (s *TaskService) EnqueueTaskForIssue(ctx context.Context, issue db.Issue, triggerCommentID ...pgtype.UUID) (db.AgentTaskQueue, error) {
	if !issue.AssigneeID.Valid {
		slog.Error("task enqueue failed", "issue_id", util.UUIDToString(issue.ID), "error", "issue has no assignee")
		return db.AgentTaskQueue{}, fmt.Errorf("issue has no assignee")
	}

	agent, err := s.Queries.GetAgent(ctx, issue.AssigneeID)
	if err != nil {
		slog.Error("task enqueue failed", "issue_id", util.UUIDToString(issue.ID), "error", err)
		return db.AgentTaskQueue{}, fmt.Errorf("load agent: %w", err)
	}
	if agent.ArchivedAt.Valid {
		slog.Debug("task enqueue skipped: agent is archived", "issue_id", util.UUIDToString(issue.ID), "agent_id", util.UUIDToString(agent.ID))
		return db.AgentTaskQueue{}, fmt.Errorf("agent is archived")
	}
	if !agent.RuntimeID.Valid {
		slog.Error("task enqueue failed", "issue_id", util.UUIDToString(issue.ID), "error", "agent has no runtime")
		return db.AgentTaskQueue{}, fmt.Errorf("agent has no runtime")
	}

	// Determine runtime type and cloud runtime ID
	var runtimeType interface{}
	var cloudRuntimeID pgtype.UUID
	taskWorkspaceID := issue.WorkspaceID
	if s.ShouldUseCloudRuntime(ctx, taskWorkspaceID, &agent) {
		runtimeType = "cloud"
		cr, err := s.Queries.GetCloudRuntimeByWorkspace(ctx, agent.WorkspaceID)
		if err == nil && cr.IsActive {
			cloudRuntimeID = cr.ID
		}
	} else {
		runtimeType = "local"
	}

	var commentID pgtype.UUID
	if len(triggerCommentID) > 0 {
		commentID = triggerCommentID[0]
	}

	task, err := s.Queries.CreateAgentTask(ctx, db.CreateAgentTaskParams{
		AgentID:          issue.AssigneeID,
		RuntimeID:        agent.RuntimeID,
		IssueID:          issue.ID,
		Priority:         priorityToInt(issue.Priority),
		TriggerCommentID: commentID,
		RuntimeType:      runtimeType,
		CloudRuntimeID:   cloudRuntimeID,
	})
	if err != nil {
		slog.Error("task enqueue failed", "issue_id", util.UUIDToString(issue.ID), "error", err)
		return db.AgentTaskQueue{}, fmt.Errorf("create task: %w", err)
	}

	slog.Info("task enqueued", "task_id", util.UUIDToString(task.ID), "issue_id", util.UUIDToString(issue.ID), "agent_id", util.UUIDToString(issue.AssigneeID))
	return task, nil
}

// EnqueueTaskForMention creates a queued task for a mentioned agent on an issue.
// Unlike EnqueueTaskForIssue, this takes an explicit agent ID rather than
// deriving it from the issue assignee.
func (s *TaskService) EnqueueTaskForMention(ctx context.Context, issue db.Issue, agentID pgtype.UUID, triggerCommentID pgtype.UUID) (db.AgentTaskQueue, error) {
	agent, err := s.Queries.GetAgent(ctx, agentID)
	if err != nil {
		slog.Error("mention task enqueue failed: agent not found", "issue_id", util.UUIDToString(issue.ID), "agent_id", util.UUIDToString(agentID), "error", err)
		return db.AgentTaskQueue{}, fmt.Errorf("load agent: %w", err)
	}
	if agent.ArchivedAt.Valid {
		slog.Debug("mention task enqueue skipped: agent is archived", "issue_id", util.UUIDToString(issue.ID), "agent_id", util.UUIDToString(agentID))
		return db.AgentTaskQueue{}, fmt.Errorf("agent is archived")
	}
	if !agent.RuntimeID.Valid {
		slog.Error("mention task enqueue failed: agent has no runtime", "issue_id", util.UUIDToString(issue.ID), "agent_id", util.UUIDToString(agentID))
		return db.AgentTaskQueue{}, fmt.Errorf("agent has no runtime")
	}

	// Determine runtime type and cloud runtime ID
	var runtimeType interface{}
	var cloudRuntimeID pgtype.UUID
	taskWorkspaceID := issue.WorkspaceID
	if s.ShouldUseCloudRuntime(ctx, taskWorkspaceID, &agent) {
		runtimeType = "cloud"
		cr, err := s.Queries.GetCloudRuntimeByWorkspace(ctx, agent.WorkspaceID)
		if err == nil && cr.IsActive {
			cloudRuntimeID = cr.ID
		}
	} else {
		runtimeType = "local"
	}

	task, err := s.Queries.CreateAgentTask(ctx, db.CreateAgentTaskParams{
		AgentID:          agentID,
		RuntimeID:        agent.RuntimeID,
		IssueID:          issue.ID,
		Priority:         priorityToInt(issue.Priority),
		TriggerCommentID: triggerCommentID,
		RuntimeType:      runtimeType,
		CloudRuntimeID:   cloudRuntimeID,
	})
	if err != nil {
		slog.Error("mention task enqueue failed", "issue_id", util.UUIDToString(issue.ID), "agent_id", util.UUIDToString(agentID), "error", err)
		return db.AgentTaskQueue{}, fmt.Errorf("create task: %w", err)
	}

	slog.Info("mention task enqueued", "task_id", util.UUIDToString(task.ID), "issue_id", util.UUIDToString(issue.ID), "agent_id", util.UUIDToString(agentID))
	return task, nil
}

// CancelTasksForIssue cancels all active tasks for an issue.
func (s *TaskService) CancelTasksForIssue(ctx context.Context, issueID pgtype.UUID) error {
	return s.Queries.CancelAgentTasksByIssue(ctx, issueID)
}

// CancelTask cancels a single task by ID. It broadcasts a task:cancelled event
// so frontends can update immediately.
func (s *TaskService) CancelTask(ctx context.Context, taskID pgtype.UUID) (*db.AgentTaskQueue, error) {
	task, runID, err := s.Lifecycle.Cancel(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("cancel task: %w", err)
	}

	slog.Info("task cancelled", "task_id", util.UUIDToString(task.ID), "issue_id", util.UUIDToString(task.IssueID))

	// Reconcile agent status
	s.ReconcileAgentStatus(ctx, task.AgentID)
	if runID.Valid {
		s.endExecutionTrace(ctx, runID, "aborted")
	}

	// Broadcast cancellation as a task:failed event so frontends clear the live card
	s.broadcastTaskEvent(ctx, protocol.EventTaskCancelled, task, runID)

	return &task, nil
}

// ClaimTask atomically claims the next queued task for an agent,
// respecting max_concurrent_tasks.
func (s *TaskService) ClaimTask(ctx context.Context, agentID pgtype.UUID) (*ClaimedTask, error) {
	agent, err := s.Queries.GetAgent(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("agent not found: %w", err)
	}

	running, err := s.Queries.CountRunningTasks(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("count running tasks: %w", err)
	}
	if running >= int64(agent.MaxConcurrentTasks) {
		slog.Debug("task claim: no capacity", "agent_id", util.UUIDToString(agentID), "running", running, "max", agent.MaxConcurrentTasks)
		return nil, nil // No capacity
	}

	claimed, err := s.Queries.ClaimAgentTaskRun(ctx, agentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			slog.Debug("task claim: no tasks available", "agent_id", util.UUIDToString(agentID))
			return nil, nil // No tasks available
		}
		return nil, fmt.Errorf("claim task: %w", err)
	}

	task, err := s.Queries.GetAgentTask(ctx, claimed.TaskID)
	if err != nil {
		return nil, fmt.Errorf("load claimed task: %w", err)
	}
	slog.Info("task claimed", "task_id", util.UUIDToString(task.ID), "run_id", util.UUIDToString(claimed.RunID), "agent_id", util.UUIDToString(agentID))

	// Update agent status to working
	s.updateAgentStatus(ctx, agentID, "working")

	// Broadcast task:dispatch
	s.broadcastTaskDispatch(ctx, task, claimed.RunID)

	return &ClaimedTask{Task: task, RunID: claimed.RunID}, nil
}

// ClaimTaskForRuntime claims the next runnable task for a runtime while
// still respecting each agent's max_concurrent_tasks limit.
func (s *TaskService) ClaimTaskForRuntime(ctx context.Context, runtimeID pgtype.UUID) (*ClaimedTask, error) {
	runtime, err := s.Queries.GetAgentRuntime(ctx, runtimeID)
	if err != nil {
		return nil, fmt.Errorf("load runtime: %w", err)
	}
	provider := runtimeagent.ProviderType(strings.ToLower(strings.TrimSpace(runtime.Provider)))
	descriptor, ok := runtimeagent.DescriptorFor(provider)
	if !ok {
		return nil, fmt.Errorf("runtime provider %q has no adapter contract", runtime.Provider)
	}

	tasks, err := s.Queries.ListPendingTasksByRuntime(ctx, runtimeID)
	if err != nil {
		return nil, fmt.Errorf("list pending tasks: %w", err)
	}

	capacityBlockedAgents := map[string]struct{}{}
	for _, candidate := range tasks {
		if candidate.Status != "queued" {
			continue
		}
		if err := stages.ValidateAdapterForTaskType(descriptor, candidate.TaskType); err != nil {
			errMsg := fmt.Sprintf("task rejected before dispatch: %v", err)
			if _, rejectErr := s.rejectQueuedTask(ctx, candidate.ID, errMsg); rejectErr != nil {
				if errors.Is(rejectErr, pgx.ErrNoRows) {
					continue
				}
				return nil, rejectErr
			}
			continue
		}

		agentKey := util.UUIDToString(candidate.AgentID)
		if _, blocked := capacityBlockedAgents[agentKey]; blocked {
			continue
		}

		task, hasCapacity, err := s.claimTaskCandidate(ctx, candidate)
		if err != nil {
			return nil, err
		}
		if !hasCapacity {
			capacityBlockedAgents[agentKey] = struct{}{}
			continue
		}
		if task != nil {
			return task, nil
		}
	}

	return nil, nil
}

func (s *TaskService) claimTaskCandidate(ctx context.Context, candidate db.AgentTaskQueue) (*ClaimedTask, bool, error) {
	agent, err := s.Queries.GetAgent(ctx, candidate.AgentID)
	if err != nil {
		return nil, false, fmt.Errorf("agent not found: %w", err)
	}
	running, err := s.Queries.CountRunningTasks(ctx, candidate.AgentID)
	if err != nil {
		return nil, false, fmt.Errorf("count running tasks: %w", err)
	}
	if running >= int64(agent.MaxConcurrentTasks) {
		return nil, false, nil
	}

	claimed, err := s.Queries.ClaimAgentTaskRunByID(ctx, candidate.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, true, nil
		}
		return nil, true, fmt.Errorf("claim task candidate: %w", err)
	}
	task, err := s.Queries.GetAgentTask(ctx, claimed.TaskID)
	if err != nil {
		return nil, true, fmt.Errorf("load claimed task: %w", err)
	}
	slog.Info("task claimed", "task_id", util.UUIDToString(task.ID), "run_id", util.UUIDToString(claimed.RunID), "agent_id", util.UUIDToString(task.AgentID))
	s.updateAgentStatus(ctx, task.AgentID, "working")
	s.broadcastTaskDispatch(ctx, task, claimed.RunID)
	return &ClaimedTask{Task: task, RunID: claimed.RunID}, true, nil
}

func (s *TaskService) rejectQueuedTask(ctx context.Context, taskID pgtype.UUID, errMsg string) (*db.AgentTaskQueue, error) {
	task, err := s.Queries.RejectQueuedAgentTask(ctx, db.RejectQueuedAgentTaskParams{
		ID:    taskID,
		Error: pgtype.Text{String: errMsg, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("reject queued task: %w", err)
	}
	slog.Warn("task rejected before dispatch",
		"task_id", util.UUIDToString(task.ID),
		"agent_id", util.UUIDToString(task.AgentID),
		"task_type", task.TaskType,
		"error", errMsg,
	)
	s.createAgentComment(ctx, task.IssueID, task.AgentID, redact.Text(errMsg), "system", task.TriggerCommentID)
	s.ReconcileAgentStatus(ctx, task.AgentID)
	s.broadcastTaskEvent(ctx, protocol.EventTaskFailed, task)
	return &task, nil
}

// StartTask transitions the dispatch-allocated Run and its Work Item to
// running in one authoritative Lifecycle transaction.
func (s *TaskService) StartTask(ctx context.Context, ref RunRef) (*db.AgentTaskQueue, error) {
	task, err := s.Lifecycle.Start(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("start task: %w", err)
	}

	slog.Info("task started", "task_id", util.UUIDToString(task.ID), "run_id", util.UUIDToString(ref.RunID), "issue_id", util.UUIDToString(task.IssueID))

	// Create execution trace (new execution_traces table).
	if s.TraceService != nil && s.TraceService.TraceService != nil {
		provider, model := s.resolveAgentProvider(ctx, task.AgentID)
		_, err := s.TraceService.StartTrace(
			ctx,
			util.UUIDToString(ref.RunID),
			util.UUIDToString(task.ID),
			util.UUIDToString(task.AgentID),
			util.UUIDToString(task.IssueID),
			provider,
			model,
		)
		if err != nil {
			slog.Warn("start task: failed to create execution trace", "task_id", util.UUIDToString(task.ID), "error", err)
		}
	}

	return &task, nil
}

// CheckpointTaskSession persists the provider session and execution directory
// while a task is running. A daemon restart can then resume the same task
// instead of falling back to the last completed task on the issue.
func (s *TaskService) CheckpointTaskSession(ctx context.Context, ref RunRef, sessionID, workDir string) (*db.AgentTaskQueue, error) {
	task, err := s.Lifecycle.Checkpoint(ctx, ref, sessionID, workDir)
	if err != nil {
		return nil, fmt.Errorf("checkpoint task session: %w", err)
	}
	return &task, nil
}

// RecoverTasksForRuntime handles orphaned dispatched/running tasks when a
// stable runtime identity registers from a fresh daemon process. Retryable
// tasks return to the queue with their session checkpoint intact; exhausted
// tasks fail explicitly instead of remaining stuck forever.
func (s *TaskService) RecoverTasksForRuntime(ctx context.Context, runtimeID pgtype.UUID) (requeued, failed int, err error) {
	recovered, err := s.Queries.RecoverTasksForRuntime(ctx, runtimeID)
	if err != nil {
		return 0, 0, fmt.Errorf("recover tasks for runtime: %w", err)
	}
	for _, recovery := range recovered {
		task, lookupErr := s.Queries.GetAgentTask(ctx, recovery.TaskID)
		if lookupErr != nil {
			return requeued, failed, fmt.Errorf("load recovered task: %w", lookupErr)
		}
		if recovery.RunID.Valid {
			s.endExecutionTrace(ctx, recovery.RunID, "failed")
		}
		switch task.Status {
		case "queued":
			requeued++
			s.broadcastTaskEvent(ctx, protocol.EventTaskRetry, task)
			// Requeued work is claimed normally; the claim allocates its next
			// Run before either Runtime Adapter sees a dispatch.
		case "failed":
			failed++
			s.broadcastTaskEvent(ctx, protocol.EventTaskFailed, task)
		}
	}
	return requeued, failed, nil
}

// resolveAgentProvider looks up the provider and model for an agent.
func (s *TaskService) resolveAgentProvider(ctx context.Context, agentID pgtype.UUID) (provider, model string) {
	agent, err := s.Queries.GetAgent(ctx, agentID)
	if err != nil {
		return "", ""
	}
	provider = agent.Provider
	if agent.ModelOverride.Valid {
		model = agent.ModelOverride.String
	}
	return
}

// CompleteTask marks a task as completed.
// Issue status is NOT changed here — the agent manages it via the CLI.
func (s *TaskService) CompleteTask(ctx context.Context, ref RunRef, result []byte, sessionID, workDir string) (*db.AgentTaskQueue, error) {
	var payload protocol.TaskCompletedPayload
	_ = json.Unmarshal(result, &payload)
	tokenInput := int64(0)
	tokenOutput := int64(0)
	totalTokens := int64(0)
	if payload.TokenUsage != nil {
		tokenInput = payload.TokenUsage.InputTokens
		tokenOutput = payload.TokenUsage.OutputTokens + payload.TokenUsage.ReasoningOutputTokens
		totalTokens = tokenInput + tokenOutput
	}
	task, err := s.Lifecycle.Complete(ctx, ref, RunCompletion{
		Result:      result,
		SessionID:   sessionID,
		WorkDir:     workDir,
		DurationMs:  payload.DurationMs,
		TotalTokens: totalTokens,
	})
	if err != nil {
		// Log the current task state to help debug why the update matched no rows.
		if existing, lookupErr := s.Queries.GetAgentTask(ctx, ref.WorkItemID); lookupErr == nil {
			slog.Warn("complete task failed: task not in running state",
				"task_id", util.UUIDToString(ref.WorkItemID),
				"run_id", util.UUIDToString(ref.RunID),
				"current_status", existing.Status,
				"issue_id", util.UUIDToString(existing.IssueID),
				"agent_id", util.UUIDToString(existing.AgentID),
			)
		} else {
			slog.Warn("complete task failed: task not found",
				"task_id", util.UUIDToString(ref.WorkItemID),
				"lookup_error", lookupErr,
			)
		}
		return nil, fmt.Errorf("complete task: %w", err)
	}

	slog.Info("task completed", "task_id", util.UUIDToString(task.ID), "issue_id", util.UUIDToString(task.IssueID))
	// End execution trace (new execution_traces table).
	s.endExecutionTrace(ctx, ref.RunID, "completed")

	// Post agent output as a comment, but only for assignment-triggered tasks.
	// Comment-triggered tasks: the agent replies via CLI with --parent, so
	// posting here would create a duplicate.
	if !task.TriggerCommentID.Valid {
		if payload.Output != "" {
			s.createAgentComment(ctx, task.IssueID, task.AgentID, redact.Text(payload.Output), "comment", task.TriggerCommentID)
		}
	}

	// Reconcile agent status
	s.ReconcileAgentStatus(ctx, task.AgentID)

	// Append-only telemetry record (Issue #11). Fire-and-forget: never block
	// the completion path on a metrics write failure.
	s.recordTaskMetric(ctx, task, "completed", payload.DurationMs, tokenInput, tokenOutput, 0, "")

	// Broadcast
	s.broadcastTaskEvent(ctx, protocol.EventTaskCompleted, task, ref.RunID)

	return &task, nil
}

// recordTaskMetric writes one row to agent_task_metrics in a detached
// 2s-timeout context. Best-effort: log a warning on failure.
func (s *TaskService) recordTaskMetric(ctx context.Context, task db.AgentTaskQueue, status string, durationMs, tokenIn, tokenOut int64, costUsd float64, errCategory string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	issuePriority := "medium"
	if issue, err := s.Queries.GetIssue(ctx, task.IssueID); err == nil && issue.Priority != "" {
		issuePriority = issue.Priority
	}

	var pgCost pgtype.Numeric
	if costUsd != 0 {
		_ = pgCost.Scan(fmt.Sprintf("%.6f", costUsd))
		pgCost.Valid = true
	}

	errMsg := pgtype.Text{String: errCategory, Valid: errCategory != ""}

	// WorkspaceID is not on AgentTaskQueue; look up via issue.
	var workspaceID pgtype.UUID
	if issue, err := s.Queries.GetIssue(ctx, task.IssueID); err == nil {
		workspaceID = issue.WorkspaceID
	} else {
		slog.Warn("record metric: lookup workspace failed", "task_id", util.UUIDToString(task.ID), "error", err)
		return
	}

	if _, err := s.Queries.InsertAgentTaskMetric(ctx, db.InsertAgentTaskMetricParams{
		WorkspaceID:   workspaceID,
		TaskID:        task.ID,
		IssueID:       task.IssueID,
		Provider:      "", // resolved by join with runtime at query time
		Model:         "",
		RuntimeMode:   task.RuntimeType,
		TaskType:      normalizeMetricTaskType(task.TaskType),
		IssuePriority: issuePriority,
		Status:        status,
		ErrorCategory: errMsg,
		DurationMs:    durationMs,
		TokenInput:    int32(tokenIn),
		TokenOutput:   int32(tokenOut),
		CostUsd:       pgCost,
	}); err != nil {
		slog.Warn("record metric failed", "task_id", util.UUIDToString(task.ID), "error", err)
	}
}

// normalizeMetricTaskType separates queue execution stages (standard,
// loop_plan, and so on) from the analytics work taxonomy. Until issues carry
// an explicit work classification, execution-stage values must be recorded as
// "other" instead of violating the metrics table constraint or inventing data.
func normalizeMetricTaskType(taskType string) string {
	switch taskType {
	case "feature", "bug", "refactor", "test", "docs", "other":
		return taskType
	default:
		return "other"
	}
}

// RetryTask resets a failed or active task back to queued for automatic retry.
// Returns the updated task if retry was successful, or nil if max retries exceeded.
func (s *TaskService) RetryTask(ctx context.Context, ref RunRef, errMsg string) (*db.AgentTaskQueue, bool, error) {
	task, retried, err := s.Lifecycle.RetryActive(ctx, ref, errMsg)
	if err != nil {
		return nil, false, fmt.Errorf("retry task: %w", err)
	}
	if !retried {
		return nil, false, nil
	}

	slog.Info("task retry scheduled", "task_id", util.UUIDToString(task.ID),
		"retry_count", task.RetryCount, "max_retries", task.MaxRetries,
		"issue_id", util.UUIDToString(task.IssueID))
	s.endExecutionTrace(ctx, ref.RunID, "failed")

	// Broadcast retry event
	s.broadcastTaskEvent(ctx, protocol.EventTaskRetry, task, ref.RunID)

	return &task, true, nil
}

// FailTask marks a task as failed.
// Issue status is NOT changed here — the agent manages it via the CLI.
func (s *TaskService) FailTask(ctx context.Context, ref RunRef, errMsg string) (*db.AgentTaskQueue, error) {
	task, err := s.Lifecycle.Fail(ctx, ref, errMsg)
	if err != nil {
		if existing, lookupErr := s.Queries.GetAgentTask(ctx, ref.WorkItemID); lookupErr == nil {
			slog.Warn("fail task failed: task not in dispatched/running state",
				"task_id", util.UUIDToString(ref.WorkItemID),
				"run_id", util.UUIDToString(ref.RunID),
				"current_status", existing.Status,
				"issue_id", util.UUIDToString(existing.IssueID),
				"agent_id", util.UUIDToString(existing.AgentID),
			)
		} else {
			slog.Warn("fail task failed: task not found",
				"task_id", util.UUIDToString(ref.WorkItemID),
				"lookup_error", lookupErr,
			)
		}
		return nil, fmt.Errorf("fail task: %w", err)
	}

	slog.Warn("task failed", "task_id", util.UUIDToString(task.ID), "issue_id", util.UUIDToString(task.IssueID), "error", errMsg)

	// End execution trace (new execution_traces table).
	s.endExecutionTrace(ctx, ref.RunID, "failed")
	if errMsg != "" {
		s.createAgentComment(ctx, task.IssueID, task.AgentID, redact.Text(errMsg), "system", task.TriggerCommentID)
	}
	// Reconcile agent status
	s.ReconcileAgentStatus(ctx, task.AgentID)

	// Broadcast
	s.broadcastTaskEvent(ctx, protocol.EventTaskFailed, task, ref.RunID)

	return &task, nil
}

// ReportProgress broadcasts a progress update via the event bus.
func (s *TaskService) ReportProgress(ctx context.Context, ref RunRef, workspaceID string, summary string, step, total int) error {
	if _, err := s.Lifecycle.AssertRunning(ctx, ref); err != nil {
		return err
	}
	s.Bus.Publish(events.Event{
		Type:        protocol.EventTaskProgress,
		WorkspaceID: workspaceID,
		ActorType:   "system",
		ActorID:     "",
		Payload: protocol.TaskProgressPayload{
			TaskID:  util.UUIDToString(ref.WorkItemID),
			RunID:   util.UUIDToString(ref.RunID),
			Summary: summary,
			Step:    step,
			Total:   total,
		},
	})
	return nil
}

// AgentStage represents the current stage of agent execution.
type AgentStage string

const (
	AgentStageReading      AgentStage = "reading"
	AgentStageImplementing AgentStage = "implementing"
	AgentStageTesting      AgentStage = "testing"
	AgentStageCommitting   AgentStage = "committing"
	AgentStageDone         AgentStage = "done"
)

// ReportAgentStage broadcasts an agent stage change via the event bus.
func (s *TaskService) ReportAgentStage(ctx context.Context, ref RunRef, agentID string, workspaceID string, stage AgentStage) error {
	if _, err := s.Lifecycle.AssertRunning(ctx, ref); err != nil {
		return err
	}
	s.Bus.Publish(events.Event{
		Type:        protocol.EventAgentStage,
		WorkspaceID: workspaceID,
		ActorType:   "system",
		ActorID:     agentID,
		Payload: map[string]any{
			"task_id":  util.UUIDToString(ref.WorkItemID),
			"run_id":   util.UUIDToString(ref.RunID),
			"agent_id": agentID,
			"stage":    stage,
		},
	})
	return nil
}

// ReconcileAgentStatus checks running task count and sets agent status accordingly.
func (s *TaskService) ReconcileAgentStatus(ctx context.Context, agentID pgtype.UUID) {
	running, err := s.Queries.CountRunningTasks(ctx, agentID)
	if err != nil {
		return
	}
	newStatus := "idle"
	if running > 0 {
		newStatus = "working"
	}
	slog.Debug("agent status reconciled", "agent_id", util.UUIDToString(agentID), "status", newStatus, "running_tasks", running)
	s.updateAgentStatus(ctx, agentID, newStatus)
}

func (s *TaskService) updateAgentStatus(ctx context.Context, agentID pgtype.UUID, status string) {
	agent, err := s.Queries.UpdateAgentStatus(ctx, db.UpdateAgentStatusParams{
		ID:     agentID,
		Status: status,
	})
	if err != nil {
		return
	}
	s.Bus.Publish(events.Event{
		Type:        protocol.EventAgentStatus,
		WorkspaceID: util.UUIDToString(agent.WorkspaceID),
		ActorType:   "system",
		ActorID:     "",
		Payload:     map[string]any{"agent": agentToMap(agent)},
	})
}

// LoadAgentSkills loads an agent's skills with their files for task execution.
func (s *TaskService) LoadAgentSkills(ctx context.Context, agentID pgtype.UUID) []AgentSkillData {
	skills, err := s.Queries.ListAgentSkills(ctx, agentID)
	if err != nil || len(skills) == 0 {
		return nil
	}

	result := make([]AgentSkillData, 0, len(skills))
	for _, sk := range skills {
		data := AgentSkillData{Name: sk.Name, Content: sk.Content}
		files, _ := s.Queries.ListSkillFiles(ctx, sk.ID)
		for _, f := range files {
			data.Files = append(data.Files, AgentSkillFileData{Path: f.Path, Content: f.Content})
		}
		result = append(result, data)
	}
	return result
}

// AgentSkillData represents a skill for task execution responses.
type AgentSkillData struct {
	Name    string               `json:"name"`
	Content string               `json:"content"`
	Files   []AgentSkillFileData `json:"files,omitempty"`
}

// AgentSkillFileData represents a supporting file within a skill.
type AgentSkillFileData struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// ShouldUseCloudRuntime determines if a task should use cloud runtime
func (s *TaskService) ShouldUseCloudRuntime(ctx context.Context, workspaceID pgtype.UUID, agent *db.Agent) bool {
	// If agent prefers local, don't use cloud
	if agent != nil && agent.PreferredRuntime == "local" {
		return false
	}

	// Check workspace has active cloud runtime
	runtime, err := s.Queries.GetCloudRuntimeByWorkspace(ctx, workspaceID)
	if err != nil || !runtime.IsActive {
		return false
	}

	// Agent prefers 'cloud' or 'any' (or nil/empty) and workspace has cloud runtime
	return agent == nil || agent.PreferredRuntime == "any" || agent.PreferredRuntime == "cloud"
}

func priorityToInt(p string) int32 {
	switch p {
	case "urgent":
		return 4
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

func (s *TaskService) broadcastTaskDispatch(ctx context.Context, task db.AgentTaskQueue, runID pgtype.UUID) {
	var payload map[string]any
	if task.Context != nil {
		json.Unmarshal(task.Context, &payload)
	}
	if payload == nil {
		payload = map[string]any{}
	}
	payload["task_id"] = util.UUIDToString(task.ID)
	payload["run_id"] = util.UUIDToString(runID)
	payload["runtime_id"] = util.UUIDToString(task.RuntimeID)

	workspaceID := ""
	if issue, err := s.Queries.GetIssue(ctx, task.IssueID); err == nil {
		workspaceID = util.UUIDToString(issue.WorkspaceID)
	}
	if workspaceID == "" {
		return // Issue deleted; skip broadcast to avoid global leak
	}
	s.Bus.Publish(events.Event{
		Type:        protocol.EventTaskDispatch,
		WorkspaceID: workspaceID,
		ActorType:   "system",
		ActorID:     "",
		Payload:     payload,
	})

	// Cloud runtime dispatch: send task to cloud gateway if configured
	if task.RuntimeType == "cloud" && task.CloudRuntimeID.Valid {
		taskIDStr := util.UUIDToString(task.ID)

		// Retrieve cloud runtime configuration for this workspace
		cr, err := s.Queries.GetCloudRuntimeByWorkspace(ctx, util.ParseUUID(workspaceID))
		if err != nil || !cr.IsActive {
			slog.Warn("cloud dispatch: no active runtime", "workspace_id", workspaceID, "task_id", taskIDStr)
			return
		}

		gatewayID := s.Hub.GatewayHub.GetGatewayForWorkspace(workspaceID)
		if gatewayID == "" {
			slog.Warn("cloud dispatch: no gateway connected", "workspace_id", workspaceID, "task_id", taskIDStr)
			return
		}

		// Decrypt API key using workspace-specific passphrase
		passphrase := workspaceID + string(auth.JWTSecret())
		apiKey, err := crypto.DecryptAPIKey(string(cr.EncryptedApiKey), passphrase)
		if err != nil {
			slog.Error("cloud dispatch: failed to decrypt API key", "error", err, "workspace_id", workspaceID, "task_id", taskIDStr)
			return
		}

		// Collect issue, agent, and skill details for dispatch
		var issueTitle, agentName, agentInstructions string
		var skills []AgentSkillData

		if issue, err := s.Queries.GetIssue(ctx, task.IssueID); err == nil {
			issueTitle = issue.Title
		}

		if agent, err := s.Queries.GetAgent(ctx, task.AgentID); err == nil {
			agentName = agent.Name
			agentInstructions = agent.Instructions
			skills = s.LoadAgentSkills(ctx, task.AgentID)
		}

		// Build dispatch configuration for gateway
		config := map[string]any{
			"task_id":      taskIDStr,
			"agent_id":     util.UUIDToString(task.AgentID),
			"issue_id":     util.UUIDToString(task.IssueID),
			"issue_title":  issueTitle,
			"agent_name":   agentName,
			"instructions": agentInstructions,
			"skills":       skills,
			"api_key":      apiKey,
			"gateway_url":  cr.GatewayUrl.String,
			"provider":     cr.Provider,
		}

		// Send dispatch message to gateway
		msg, err := json.Marshal(protocol.GatewayTaskDispatchMessage{
			Type:   protocol.EventTaskDispatch,
			TaskID: taskIDStr,
			RunID:  util.UUIDToString(runID),
			Config: config,
		})
		if err != nil {
			slog.Error("cloud dispatch: failed to marshal message", "error", err, "task_id", taskIDStr)
			return
		}

		if err := s.Hub.GatewayHub.SendToGateway(gatewayID, msg); err != nil {
			slog.Error("cloud dispatch: failed to send to gateway", "error", err, "gateway_id", gatewayID, "task_id", taskIDStr)
			return
		}

		slog.Info("cloud task dispatched", "task_id", taskIDStr, "gateway_id", gatewayID, "workspace_id", workspaceID)
	}
}

func (s *TaskService) broadcastTaskEvent(ctx context.Context, eventType string, task db.AgentTaskQueue, runIDs ...pgtype.UUID) {
	workspaceID := ""
	if issue, err := s.Queries.GetIssue(ctx, task.IssueID); err == nil {
		workspaceID = util.UUIDToString(issue.WorkspaceID)
	}
	if workspaceID == "" {
		return // Issue deleted; skip broadcast to avoid global leak
	}
	payload := map[string]any{
		"task_id":  util.UUIDToString(task.ID),
		"agent_id": util.UUIDToString(task.AgentID),
		"issue_id": util.UUIDToString(task.IssueID),
		"status":   task.Status,
	}
	if len(runIDs) > 0 && runIDs[0].Valid {
		payload["run_id"] = util.UUIDToString(runIDs[0])
	}
	s.Bus.Publish(events.Event{
		Type:        eventType,
		WorkspaceID: workspaceID,
		ActorType:   "system",
		ActorID:     "",
		Payload:     payload,
	})
}

func (s *TaskService) broadcastIssueUpdated(issue db.Issue) {
	prefix := s.getIssuePrefix(issue.WorkspaceID)
	s.Bus.Publish(events.Event{
		Type:        protocol.EventIssueUpdated,
		WorkspaceID: util.UUIDToString(issue.WorkspaceID),
		ActorType:   "system",
		ActorID:     "",
		Payload:     map[string]any{"issue": issueToMap(issue, prefix)},
	})
}

func (s *TaskService) getIssuePrefix(workspaceID pgtype.UUID) string {
	ws, err := s.Queries.GetWorkspace(context.Background(), workspaceID)
	if err != nil {
		return ""
	}
	return ws.IssuePrefix
}

func (s *TaskService) createAgentComment(ctx context.Context, issueID, agentID pgtype.UUID, content, commentType string, parentID pgtype.UUID) {
	if content == "" {
		return
	}
	// Look up issue to get workspace ID for mention expansion and broadcasting.
	issue, err := s.Queries.GetIssue(ctx, issueID)
	if err != nil {
		return
	}
	// Expand bare issue identifiers (e.g. MUL-117) into mention links.
	content = mention.ExpandIssueIdentifiers(ctx, s.Queries, issue.WorkspaceID, content)
	comment, err := s.Queries.CreateComment(ctx, db.CreateCommentParams{
		IssueID:    issueID,
		AuthorType: "agent",
		AuthorID:   agentID,
		Content:    content,
		Type:       commentType,
		ParentID:   parentID,
	})
	if err != nil {
		return
	}
	s.Bus.Publish(events.Event{
		Type:        protocol.EventCommentCreated,
		WorkspaceID: util.UUIDToString(issue.WorkspaceID),
		ActorType:   "agent",
		ActorID:     util.UUIDToString(agentID),
		Payload: map[string]any{
			"comment": map[string]any{
				"id":          util.UUIDToString(comment.ID),
				"issue_id":    util.UUIDToString(comment.IssueID),
				"author_type": comment.AuthorType,
				"author_id":   util.UUIDToString(comment.AuthorID),
				"content":     comment.Content,
				"type":        comment.Type,
				"parent_id":   util.UUIDToPtr(comment.ParentID),
				"created_at":  comment.CreatedAt.Time.Format("2006-01-02T15:04:05Z"),
			},
			"issue_title":  issue.Title,
			"issue_status": issue.Status,
		},
	})
}

func issueToMap(issue db.Issue, issuePrefix string) map[string]any {
	return map[string]any{
		"id":              util.UUIDToString(issue.ID),
		"workspace_id":    util.UUIDToString(issue.WorkspaceID),
		"number":          issue.Number,
		"identifier":      issuePrefix + "-" + strconv.Itoa(int(issue.Number)),
		"title":           issue.Title,
		"description":     util.TextToPtr(issue.Description),
		"status":          issue.Status,
		"priority":        issue.Priority,
		"assignee_type":   util.TextToPtr(issue.AssigneeType),
		"assignee_id":     util.UUIDToPtr(issue.AssigneeID),
		"creator_type":    issue.CreatorType,
		"creator_id":      util.UUIDToString(issue.CreatorID),
		"parent_issue_id": util.UUIDToPtr(issue.ParentIssueID),
		"position":        issue.Position,
		"due_date":        util.TimestampToPtr(issue.DueDate),
		"created_at":      util.TimestampToString(issue.CreatedAt),
		"updated_at":      util.TimestampToString(issue.UpdatedAt),
	}
}

// agentToMap builds a simple map for broadcasting agent status updates.
func agentToMap(a db.Agent) map[string]any {
	var rc any
	if a.RuntimeConfig != nil {
		json.Unmarshal(a.RuntimeConfig, &rc)
	}
	var tools any
	if a.Tools != nil {
		json.Unmarshal(a.Tools, &tools)
	}
	var triggers any
	if a.Triggers != nil {
		json.Unmarshal(a.Triggers, &triggers)
	}
	return map[string]any{
		"id":                   util.UUIDToString(a.ID),
		"workspace_id":         util.UUIDToString(a.WorkspaceID),
		"runtime_id":           util.UUIDToString(a.RuntimeID),
		"name":                 a.Name,
		"description":          a.Description,
		"avatar_url":           util.TextToPtr(a.AvatarUrl),
		"runtime_mode":         a.RuntimeMode,
		"runtime_config":       rc,
		"visibility":           a.Visibility,
		"status":               a.Status,
		"max_concurrent_tasks": a.MaxConcurrentTasks,
		"owner_id":             util.UUIDToPtr(a.OwnerID),
		"skills":               []any{},
		"tools":                tools,
		"triggers":             triggers,
		"created_at":           util.TimestampToString(a.CreatedAt),
		"updated_at":           util.TimestampToString(a.UpdatedAt),
		"archived_at":          util.TimestampToPtr(a.ArchivedAt),
		"archived_by":          util.UUIDToPtr(a.ArchivedBy),
	}
}

// endExecutionTrace ends the projection for one exact Run. It never guesses
// an attempt by selecting the most recent Trace for a Work Item.
func (s *TaskService) endExecutionTrace(ctx context.Context, runID pgtype.UUID, status string) {
	if s.TraceService == nil || s.TraceService.TraceService == nil {
		return
	}
	trace, err := s.TraceService.GetTraceByRun(ctx, util.UUIDToString(runID))
	if err != nil {
		slog.Warn("end execution trace: failed to find trace", "run_id", util.UUIDToString(runID), "error", err)
		return
	}
	if err := s.TraceService.EndTrace(ctx, trace.ID, status); err != nil {
		slog.Warn("end execution trace: failed", "trace_id", trace.ID, "error", err)
	}
}
