package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/agentra-ai/agentra/server/internal/agent/seed"
	"github.com/agentra-ai/agentra/server/internal/service"
	db "github.com/agentra-ai/agentra/server/pkg/db/generated"
	"github.com/agentra-ai/agentra/server/pkg/protocol"
	"github.com/agentra-ai/agentra/server/pkg/redact"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// ---------------------------------------------------------------------------
// Daemon Registration & Heartbeat
// ---------------------------------------------------------------------------

// reconcileAgentsForWorkspace reconciles the status of all agents in a workspace
// after daemon registration. This ensures agents aren't left in stale "working"
// status when the daemon restarts without any task state changes.
func (h *Handler) reconcileAgentsForWorkspace(workspaceID string) {
	ctx := context.Background()
	agents, err := h.Queries.ListAgents(ctx, parseUUID(workspaceID))
	if err != nil {
		return
	}
	for _, agent := range agents {
		running, err := h.Queries.CountRunningTasks(ctx, agent.ID)
		if err != nil {
			continue
		}
		newStatus := "idle"
		if running > 0 {
			newStatus = "working"
		}
		if string(agent.Status) == newStatus {
			continue
		}
		updated, err := h.Queries.UpdateAgentStatus(ctx, db.UpdateAgentStatusParams{
			ID:     agent.ID,
			Status: newStatus,
		})
		if err != nil {
			continue
		}
		h.publish(protocol.EventAgentStatus, workspaceID, "system", "", map[string]any{
			"agent_id": uuidToString(updated.ID),
			"status":   updated.Status,
		})
	}
}

type DaemonRegisterRequest struct {
	WorkspaceID string `json:"workspace_id"`
	DaemonID    string `json:"daemon_id"`
	DeviceName  string `json:"device_name"`
	CLIVersion  string `json:"cli_version"` // agentra CLI version
	Runtimes    []struct {
		Name    string `json:"name"`
		Type    string `json:"type"`
		Version string `json:"version"` // agent CLI version (claude/codex)
		Status  string `json:"status"`
	} `json:"runtimes"`
}

func (h *Handler) DaemonRegister(w http.ResponseWriter, r *http.Request) {
	var req DaemonRegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.WorkspaceID = strings.TrimSpace(req.WorkspaceID)
	req.DaemonID = strings.TrimSpace(req.DaemonID)
	req.DeviceName = strings.TrimSpace(req.DeviceName)

	if req.DaemonID == "" {
		writeError(w, http.StatusBadRequest, "daemon_id is required")
		return
	}
	if req.WorkspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	if len(req.Runtimes) == 0 {
		writeError(w, http.StatusBadRequest, "at least one runtime is required")
		return
	}

	// Verify the caller is a member of the target workspace.
	if _, ok := h.requireWorkspaceMember(w, r, req.WorkspaceID, "workspace not found"); !ok {
		return
	}

	ws, err := h.Queries.GetWorkspace(r.Context(), parseUUID(req.WorkspaceID))
	if err != nil {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}

	resp := make([]AgentRuntimeResponse, 0, len(req.Runtimes))
	for _, runtime := range req.Runtimes {
		provider := strings.TrimSpace(runtime.Type)
		if provider == "" {
			provider = "unknown"
		}
		name := strings.TrimSpace(runtime.Name)
		if name == "" {
			name = provider
			if req.DeviceName != "" {
				name = fmt.Sprintf("%s (%s)", provider, req.DeviceName)
			}
		}
		deviceInfo := strings.TrimSpace(req.DeviceName)
		if runtime.Version != "" && deviceInfo != "" {
			deviceInfo = fmt.Sprintf("%s · %s", deviceInfo, runtime.Version)
		} else if runtime.Version != "" {
			deviceInfo = runtime.Version
		}
		status := "online"
		if runtime.Status == "offline" {
			status = "offline"
		}
		metadata, _ := json.Marshal(map[string]any{
			"version":     runtime.Version,
			"cli_version": req.CLIVersion,
		})

		registered, err := h.Queries.UpsertAgentRuntime(r.Context(), db.UpsertAgentRuntimeParams{
			WorkspaceID: parseUUID(req.WorkspaceID),
			DaemonID:    strToText(req.DaemonID),
			Name:        name,
			RuntimeMode: "local",
			Provider:    provider,
			Status:      status,
			DeviceInfo:  deviceInfo,
			Metadata:    metadata,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to register runtime: "+err.Error())
			return
		}
		resp = append(resp, runtimeToResponse(registered))
	}

	slog.Info("daemon registered", "workspace_id", req.WorkspaceID, "daemon_id", req.DaemonID, "runtimes_count", len(resp))

	h.publish(protocol.EventDaemonRegister, req.WorkspaceID, "system", "", map[string]any{
		"runtimes": resp,
	})

	// Reconcile agent statuses after daemon registration to handle the case
	// where the daemon restarted and left agents in stale "working" status.
	h.reconcileAgentsForWorkspace(req.WorkspaceID)

	// Seed default specialist agents on first daemon registration. We use
	// the first registered runtime as the runtime for the seeded agents —
	// specialists don't pin a specific runtime, so any online one works.
	// The first workspace member is used as the agent owner; if the
	// workspace somehow has no members yet (shouldn't happen — workspace
	// creation always creates an owner), the agents are inserted with a
	// null owner, which the column allows.
	if len(resp) > 0 {
		owner := firstWorkspaceMember(r.Context(), h.Queries, parseUUID(req.WorkspaceID))
		if _, err := seed.SeedForWorkspace(r.Context(), h.Queries, parseUUID(req.WorkspaceID), owner, parseUUID(resp[0].ID)); err != nil {
			slog.Warn("seed default agents after daemon register failed",
				"workspace_id", req.WorkspaceID, "error", err)
		}
	}

	// Include workspace repos so the daemon can cache them locally.
	var repos []RepoData
	if ws.Repos != nil {
		json.Unmarshal(ws.Repos, &repos)
	}
	if repos == nil {
		repos = []RepoData{}
	}

	writeJSON(w, http.StatusOK, map[string]any{"runtimes": resp, "repos": repos})
}

// DaemonDeregister marks runtimes as offline when the daemon shuts down.
func (h *Handler) DaemonDeregister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RuntimeIDs []string `json:"runtime_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.RuntimeIDs) == 0 {
		writeError(w, http.StatusBadRequest, "runtime_ids is required")
		return
	}

	// Track affected workspaces for WS notifications.
	affectedWorkspaces := make(map[string]bool)

	for _, rid := range req.RuntimeIDs {
		// Look up the runtime to find its workspace.
		rt, err := h.Queries.GetAgentRuntime(r.Context(), parseUUID(rid))
		if err != nil {
			slog.Warn("deregister: runtime not found", "runtime_id", rid, "error", err)
			continue
		}

		if err := h.Queries.SetAgentRuntimeOffline(r.Context(), parseUUID(rid)); err != nil {
			slog.Warn("deregister: failed to set offline", "runtime_id", rid, "error", err)
			continue
		}

		affectedWorkspaces[uuidToString(rt.WorkspaceID)] = true
	}

	// Notify frontend clients so they re-fetch runtime list.
	for wsID := range affectedWorkspaces {
		h.publish(protocol.EventDaemonRegister, wsID, "system", "", map[string]any{
			"action": "deregister",
		})
	}

	slog.Info("daemon deregistered", "runtime_ids", req.RuntimeIDs)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type DaemonHeartbeatRequest struct {
	RuntimeID string `json:"runtime_id"`
}

func (h *Handler) DaemonHeartbeat(w http.ResponseWriter, r *http.Request) {
	var req DaemonHeartbeatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.RuntimeID == "" {
		writeError(w, http.StatusBadRequest, "runtime_id is required")
		return
	}

	_, err := h.Queries.UpdateAgentRuntimeHeartbeat(r.Context(), parseUUID(req.RuntimeID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "heartbeat failed")
		return
	}

	slog.Debug("daemon heartbeat", "runtime_id", req.RuntimeID)

	resp := map[string]any{"status": "ok"}

	// Check for pending ping requests for this runtime.
	if pending := h.PingStore.PopPending(req.RuntimeID); pending != nil {
		resp["pending_ping"] = map[string]string{"id": pending.ID}
	}

	// Check for pending update requests for this runtime.
	if pending := h.UpdateStore.PopPending(req.RuntimeID); pending != nil {
		resp["pending_update"] = map[string]string{
			"id":             pending.ID,
			"target_version": pending.TargetVersion,
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// ClaimTaskByRuntime atomically claims the next queued task for a runtime.
// The response includes the agent's name and skills, fetched fresh from the DB.
func (h *Handler) ClaimTaskByRuntime(w http.ResponseWriter, r *http.Request) {
	runtimeID := chi.URLParam(r, "runtimeId")

	task, err := h.TaskService.ClaimTaskForRuntime(r.Context(), parseUUID(runtimeID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to claim task: "+err.Error())
		return
	}

	if task == nil {
		slog.Debug("no task to claim", "runtime_id", runtimeID)
		writeJSON(w, http.StatusOK, map[string]any{"task": nil})
		return
	}

	// Build response with fresh agent data (name + skills).
	resp := taskToResponse(*task)
	if agent, err := h.Queries.GetAgent(r.Context(), task.AgentID); err == nil {
		skills := h.TaskService.LoadAgentSkills(r.Context(), task.AgentID)
		resp.Agent = &TaskAgentData{
			ID:           uuidToString(agent.ID),
			Name:         agent.Name,
			Instructions: agent.Instructions,
			Skills:       skills,
		}
	}

	// Include workspace ID, repos, and issue title so the daemon can set up worktrees and branch naming.
	if issue, err := h.Queries.GetIssue(r.Context(), task.IssueID); err == nil {
		resp.WorkspaceID = uuidToString(issue.WorkspaceID)
		resp.IssueTitle = issue.Title
		if ws, err := h.Queries.GetWorkspace(r.Context(), issue.WorkspaceID); err == nil && ws.Repos != nil {
			var repos []RepoData
			if json.Unmarshal(ws.Repos, &repos) == nil && len(repos) > 0 {
				resp.Repos = repos
			}
		}
	}

	// Look up the prior session for this (agent, issue) pair so the daemon
	// can resume the Claude Code conversation context.
	if prior, err := h.Queries.GetLastTaskSession(r.Context(), db.GetLastTaskSessionParams{
		AgentID: task.AgentID,
		IssueID: task.IssueID,
	}); err == nil && prior.SessionID.Valid {
		resp.PriorSessionID = prior.SessionID.String
		if prior.WorkDir.Valid {
			resp.PriorWorkDir = prior.WorkDir.String
		}
	}

	// For loop tasks, look up the loop's branch_name and iteration so the
	// daemon can build per-stage prompts that reference the real branch
	// (review/fix) and the current fix iteration (fix). The loop row is
	// the source of truth — `agent_task_queue` does not carry these
	// values directly. If the loop is missing (e.g. deleted between
	// enqueue and claim), the daemon's buildPromptForStage falls back to
	// a placeholder and logs a warning.
	if task.LoopID.Valid {
		loopRef, err := h.Queries.GetLoopBranchAndIteration(r.Context(), task.LoopID)
		if err != nil {
			slog.Warn("claim: failed to load loop branch/iteration",
				"task_id", uuidToString(task.ID), "loop_id", uuidToString(task.LoopID), "error", err)
		} else {
			if loopRef.BranchName.Valid {
				resp.Branch = loopRef.BranchName.String
			}
			resp.Iteration = int(loopRef.Iteration)
		}
	}

	slog.Info("task claimed by runtime", "task_id", uuidToString(task.ID), "runtime_id", runtimeID, "agent_id", uuidToString(task.AgentID), "prior_session", resp.PriorSessionID, "task_type", resp.TaskType, "branch", resp.Branch, "iteration", resp.Iteration)
	writeJSON(w, http.StatusOK, map[string]any{"task": resp})
}

// ListPendingTasksByRuntime returns queued/dispatched tasks for a runtime.
func (h *Handler) ListPendingTasksByRuntime(w http.ResponseWriter, r *http.Request) {
	runtimeID := chi.URLParam(r, "runtimeId")

	tasks, err := h.Queries.ListPendingTasksByRuntime(r.Context(), parseUUID(runtimeID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list pending tasks")
		return
	}

	resp := make([]AgentTaskResponse, len(tasks))
	for i, t := range tasks {
		resp[i] = taskToResponse(t)
	}

	writeJSON(w, http.StatusOK, resp)
}

// ---------------------------------------------------------------------------
// Task Lifecycle (called by daemon)
// ---------------------------------------------------------------------------

// StartTask marks a dispatched task as running.
func (h *Handler) StartTask(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")

	task, err := h.TaskService.StartTask(r.Context(), parseUUID(taskID))
	if err != nil {
		slog.Warn("start task failed", "task_id", taskID, "error", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	slog.Info("task started", "task_id", taskID, "agent_id", uuidToString(task.AgentID))
	writeJSON(w, http.StatusOK, taskToResponse(*task))
}

// ReportTaskProgress broadcasts a progress update.
type TaskProgressRequest struct {
	Summary string `json:"summary"`
	Step    int    `json:"step"`
	Total   int    `json:"total"`
}

func (h *Handler) ReportTaskProgress(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")

	var req TaskProgressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Look up task to get workspace ID via the associated issue.
	workspaceID := ""
	task, err := h.Queries.GetAgentTask(r.Context(), parseUUID(taskID))
	if err == nil {
		if issue, err := h.Queries.GetIssue(r.Context(), task.IssueID); err == nil {
			workspaceID = uuidToString(issue.WorkspaceID)
		}
	}

	h.TaskService.ReportProgress(r.Context(), taskID, workspaceID, req.Summary, req.Step, req.Total)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ReportAgentStage receives an agent stage change from the daemon.
type TaskStageRequest struct {
	Stage string `json:"stage"` // "reading", "implementing", "testing", "committing", "done"
}

func (h *Handler) ReportAgentStage(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")

	var req TaskStageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	task, err := h.Queries.GetAgentTask(r.Context(), parseUUID(taskID))
	if err != nil {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}

	workspaceID := ""
	if issue, err := h.Queries.GetIssue(r.Context(), task.IssueID); err == nil {
		workspaceID = uuidToString(issue.WorkspaceID)
	}

	h.TaskService.ReportAgentStage(r.Context(), taskID, uuidToString(task.AgentID), workspaceID, service.AgentStage(req.Stage))
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// CompleteTask marks a running task as completed.
type TaskCompleteRequest struct {
	PRURL     string `json:"pr_url"`
	Output    string `json:"output"`
	SessionID string `json:"session_id"` // Claude session ID for future resumption
	WorkDir   string `json:"work_dir"`   // working directory used during execution
}

func (h *Handler) CompleteTask(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")

	var req TaskCompleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, _ := json.Marshal(req)
	task, err := h.TaskService.CompleteTask(r.Context(), parseUUID(taskID), result, req.SessionID, req.WorkDir)
	if err != nil {
		slog.Warn("complete task failed", "task_id", taskID, "error", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	slog.Info("task completed", "task_id", taskID, "agent_id", uuidToString(task.AgentID))
	writeJSON(w, http.StatusOK, taskToResponse(*task))
}

// GetTaskStatus returns the current status of a task.
// Used by the daemon to check whether a task was cancelled mid-execution.
func (h *Handler) GetTaskStatus(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")
	task, err := h.Queries.GetAgentTask(r.Context(), parseUUID(taskID))
	if err != nil {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": task.Status})
}

// FailTask marks a running task as failed.
type TaskFailRequest struct {
	Error string `json:"error"`
}

func (h *Handler) FailTask(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")

	var req TaskFailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	task, err := h.TaskService.FailTask(r.Context(), parseUUID(taskID), req.Error)
	if err != nil {
		slog.Warn("fail task failed", "task_id", taskID, "error", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	slog.Info("task failed", "task_id", taskID, "agent_id", uuidToString(task.AgentID), "task_error", req.Error)
	writeJSON(w, http.StatusOK, taskToResponse(*task))
}

// ---------------------------------------------------------------------------
// Task Messages (live agent output)
// ---------------------------------------------------------------------------

type TaskMessageRequest struct {
	Seq     int            `json:"seq"`
	Type    string         `json:"type"`
	Tool    string         `json:"tool,omitempty"`
	Content string         `json:"content,omitempty"`
	Input   map[string]any `json:"input,omitempty"`
	Output  string         `json:"output,omitempty"`
}

type TaskMessageBatchRequest struct {
	Messages []TaskMessageRequest `json:"messages"`
}

const (
	maxTaskMessageBodyBytes  = 8 * 1024 * 1024
	defaultTaskMessageLimit  = 500
	maxTaskMessageLimit      = 5000
)

var taskMessageTypes = map[string]bool{
	"text":        true,
	"thinking":    true,
	"tool_use":    true,
	"tool_result": true,
	"error":       true,
}

func validateTaskMessage(msg TaskMessageRequest) error {
	if msg.Seq <= 0 {
		return fmt.Errorf("seq must be positive")
	}
	if !taskMessageTypes[msg.Type] {
		return fmt.Errorf("unsupported message type %q", msg.Type)
	}
	if len(msg.Tool) > protocol.TaskMessageFieldBytes || len(msg.Content) > protocol.TaskMessageFieldBytes || len(msg.Output) > protocol.TaskMessageFieldBytes {
		return fmt.Errorf("message field exceeds %d bytes", protocol.TaskMessageFieldBytes)
	}
	if msg.Input != nil {
		inputJSON, err := json.Marshal(msg.Input)
		if err != nil {
			return fmt.Errorf("invalid input: %w", err)
		}
		if len(inputJSON) > protocol.TaskMessageFieldBytes {
			return fmt.Errorf("message input exceeds %d bytes", protocol.TaskMessageFieldBytes)
		}
	}
	return nil
}

// persistTaskMessage is the single task-output ledger path for both local
// daemons and cloud gateways. It publishes only after a successful insert;
// duplicate cursors are accepted but never broadcast twice.
func (h *Handler) persistTaskMessage(ctx context.Context, task db.AgentTaskQueue, workspaceID, traceID string, msg TaskMessageRequest) (bool, error) {
	msg.Content = redact.Text(msg.Content)
	msg.Output = redact.Text(msg.Output)
	msg.Input = redact.InputMap(msg.Input)

	var inputJSON []byte
	if msg.Input != nil {
		var err error
		inputJSON, err = json.Marshal(msg.Input)
		if err != nil {
			return false, fmt.Errorf("marshal task message input: %w", err)
		}
	}

	inserted, err := h.Queries.CreateTaskMessage(ctx, db.CreateTaskMessageParams{
		TaskID:  task.ID,
		Seq:     int32(msg.Seq),
		Type:    msg.Type,
		Tool:    pgtype.Text{String: msg.Tool, Valid: msg.Tool != ""},
		Content: pgtype.Text{String: msg.Content, Valid: msg.Content != ""},
		Input:   inputJSON,
		Output:  pgtype.Text{String: msg.Output, Valid: msg.Output != ""},
	})
	if err != nil {
		return false, fmt.Errorf("persist task message: %w", err)
	}
	if inserted == 0 {
		return false, nil
	}

	if traceID != "" {
		h.recordTraceMessage(ctx, traceID, msg)
	}
	h.publish(protocol.EventTaskMessage, workspaceID, "system", "", protocol.TaskMessagePayload{
		TaskID:  uuidToString(task.ID),
		IssueID: uuidToString(task.IssueID),
		Seq:     msg.Seq,
		Type:    msg.Type,
		Tool:    msg.Tool,
		Content: msg.Content,
		Input:   msg.Input,
		Output:  msg.Output,
	})
	return true, nil
}

// RecordGatewayTaskLog validates tenant ownership and normalizes a cloud
// gateway stdout/stderr frame into the same persisted task_message stream used
// by local daemons.
func (h *Handler) RecordGatewayTaskLog(ctx context.Context, workspaceID, taskID string, seq int, stream, content string) error {
	task, err := h.TaskService.ValidateCloudGatewayTask(ctx, workspaceID, taskID)
	if err != nil {
		return err
	}
	if task.Status != "dispatched" && task.Status != "running" {
		return fmt.Errorf("cloud gateway task is not active")
	}

	messageType := "text"
	if stream == protocol.GatewayStreamStderr {
		messageType = "error"
	} else if stream != protocol.GatewayStreamStdout {
		return fmt.Errorf("unsupported gateway log stream %q", stream)
	}
	msg := TaskMessageRequest{Seq: seq, Type: messageType, Content: content}
	if err := validateTaskMessage(msg); err != nil {
		return err
	}
	_, err = h.persistTaskMessage(ctx, task, workspaceID, "", msg)
	return err
}

// ReportTaskMessages receives a batch of agent execution messages from the daemon.
func (h *Handler) ReportTaskMessages(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")

	var req TaskMessageBatchRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxTaskMessageBodyBytes)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.Messages) == 0 {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}
	if len(req.Messages) > protocol.TaskMessageBatchSize {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("messages must contain at most %d entries", protocol.TaskMessageBatchSize))
		return
	}
	for _, msg := range req.Messages {
		if err := validateTaskMessage(msg); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	task, err := h.Queries.GetAgentTask(r.Context(), parseUUID(taskID))
	if err != nil {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}

	issue, err := h.Queries.GetIssue(r.Context(), task.IssueID)
	if err != nil {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	workspaceID := uuidToString(issue.WorkspaceID)
	if _, ok := h.requireWorkspaceMember(w, r, workspaceID, "task not found"); !ok {
		return
	}

	// Look up execution trace for recording steps (best-effort).
	traceID := ""
	if h.TraceService != nil && h.TraceService.TraceService != nil {
		if trace, lookupErr := h.TraceService.GetTraceByTask(r.Context(), taskID); lookupErr == nil {
			traceID = trace.ID
		}
	}

	for _, msg := range req.Messages {
		if _, err := h.persistTaskMessage(r.Context(), task, workspaceID, traceID, msg); err != nil {
			slog.Error("report task messages: persist failed", "task_id", taskID, "error", err)
			writeError(w, http.StatusInternalServerError, "failed to persist task messages")
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ListTaskMessages returns the persisted messages for a task (for catch-up after reconnect).
func (h *Handler) ListTaskMessages(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")

	task, err := h.Queries.GetAgentTask(r.Context(), parseUUID(taskID))
	if err != nil {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	issue, err := h.Queries.GetIssue(r.Context(), task.IssueID)
	if err != nil {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	if _, ok := h.requireWorkspaceMember(w, r, uuidToString(issue.WorkspaceID), "task not found"); !ok {
		return
	}

	limit := defaultTaskMessageLimit
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		limit, err = strconv.Atoi(limitStr)
		if err != nil || limit < 1 || limit > maxTaskMessageLimit {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("limit must be between 1 and %d", maxTaskMessageLimit))
			return
		}
	}

	var messages []db.TaskMessage
	if sinceStr := r.URL.Query().Get("since"); sinceStr != "" {
		sinceSeq, parseErr := strconv.Atoi(sinceStr)
		if parseErr != nil || sinceSeq < 0 {
			writeError(w, http.StatusBadRequest, "invalid since parameter")
			return
		}
		messages, err = h.Queries.ListTaskMessagesSince(r.Context(), db.ListTaskMessagesSinceParams{
			TaskID: parseUUID(taskID),
			Seq:    int32(sinceSeq),
			Limit:  int32(limit),
		})
	} else {
		messages, err = h.Queries.ListTaskMessages(r.Context(), db.ListTaskMessagesParams{
			TaskID: parseUUID(taskID),
			Limit:  int32(limit),
		})
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list task messages")
		return
	}

	issueID := uuidToString(task.IssueID)

	resp := make([]protocol.TaskMessagePayload, len(messages))
	for i, m := range messages {
		var input map[string]any
		if m.Input != nil {
			json.Unmarshal(m.Input, &input)
		}
		resp[i] = protocol.TaskMessagePayload{
			TaskID:  taskID,
			IssueID: issueID,
			Seq:     int(m.Seq),
			Type:    m.Type,
			Tool:    m.Tool.String,
			Content: m.Content.String,
			Input:   input,
			Output:  m.Output.String,
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// GetActiveTaskForIssue returns the currently running task for an issue, if any.
func (h *Handler) GetActiveTaskForIssue(w http.ResponseWriter, r *http.Request) {
	issueID := chi.URLParam(r, "id")

	tasks, err := h.Queries.ListActiveTasksByIssue(r.Context(), parseUUID(issueID))
	if err != nil || len(tasks) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"task": nil})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"task": taskToResponse(tasks[0])})
}

// CancelTask cancels a running or queued task by ID.
func (h *Handler) CancelTask(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")

	task, err := h.TaskService.CancelTask(r.Context(), parseUUID(taskID))
	if err != nil {
		slog.Warn("cancel task failed", "task_id", taskID, "error", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	slog.Info("task cancelled by user", "task_id", taskID, "issue_id", uuidToString(task.IssueID))
	writeJSON(w, http.StatusOK, taskToResponse(*task))
}

// ListTasksByIssue returns all tasks (any status) for an issue — used for execution history.
func (h *Handler) ListTasksByIssue(w http.ResponseWriter, r *http.Request) {
	issueID := chi.URLParam(r, "id")

	tasks, err := h.Queries.ListTasksByIssue(r.Context(), parseUUID(issueID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list tasks")
		return
	}

	resp := make([]AgentTaskResponse, len(tasks))
	for i, t := range tasks {
		resp[i] = taskToResponse(t)
	}

	writeJSON(w, http.StatusOK, resp)
}
