package handler

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
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
	InstanceID  string `json:"instance_id"`
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
	req.InstanceID = strings.TrimSpace(req.InstanceID)
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
	for i := range req.Runtimes {
		provider, err := validateLocalRuntimeProvider(req.Runtimes[i].Type)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("runtimes[%d].type: %s", i, err))
			return
		}
		req.Runtimes[i].Type = provider
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
		provider := runtime.Type
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
		// Registration is retry-safe: only a new process instance may recover
		// orphaned tasks. Older daemons without instance_id remain compatible,
		// but do not activate crash recovery until upgraded.
		shouldRecover := false
		if req.InstanceID != "" {
			if previous, lookupErr := h.Queries.GetAgentRuntimeByIdentity(r.Context(), db.GetAgentRuntimeByIdentityParams{
				WorkspaceID: parseUUID(req.WorkspaceID),
				DaemonID:    strToText(req.DaemonID),
				Provider:    provider,
			}); lookupErr == nil {
				var previousMetadata struct {
					InstanceID string `json:"instance_id"`
				}
				_ = json.Unmarshal(previous.Metadata, &previousMetadata)
				shouldRecover = previousMetadata.InstanceID != req.InstanceID
			}
		}
		metadata, _ := json.Marshal(map[string]any{
			"version":     runtime.Version,
			"cli_version": req.CLIVersion,
			"instance_id": req.InstanceID,
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
		if status == "online" && shouldRecover {
			requeued, failed, err := h.TaskService.RecoverTasksForRuntime(r.Context(), registered.ID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to recover runtime tasks: "+err.Error())
				return
			}
			if requeued > 0 || failed > 0 {
				slog.Info("recovered tasks after daemon registration",
					"runtime_id", uuidToString(registered.ID), "requeued", requeued, "failed", failed)
			}
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

	claimed, err := h.TaskService.ClaimTaskForRuntime(r.Context(), parseUUID(runtimeID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to claim task: "+err.Error())
		return
	}

	if claimed == nil {
		slog.Debug("no task to claim", "runtime_id", runtimeID)
		writeJSON(w, http.StatusOK, map[string]any{"task": nil})
		return
	}
	task := claimed.Task

	// Build response with fresh agent data (name + skills).
	resp := taskToResponse(task)
	resp.RunID = uuidToString(claimed.RunID)
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

	// A checkpoint on this task takes precedence after a daemon crash/retry.
	// Otherwise, continue the most recent completed conversation for the same
	// (agent, issue) pair.
	if task.SessionID.Valid {
		resp.PriorSessionID = task.SessionID.String
		if task.WorkDir.Valid {
			resp.PriorWorkDir = task.WorkDir.String
		}
	} else if prior, err := h.Queries.GetLastTaskSession(r.Context(), db.GetLastTaskSessionParams{
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

func daemonRunRef(taskID, runID string) (service.RunRef, error) {
	ref := service.RunRef{WorkItemID: parseUUID(taskID), RunID: parseUUID(strings.TrimSpace(runID))}
	if !ref.WorkItemID.Valid || !ref.RunID.Valid {
		return service.RunRef{}, fmt.Errorf("run_id is required")
	}
	return ref, nil
}

func writeRunTransitionError(w http.ResponseWriter, err error) {
	if errors.Is(err, service.ErrStaleRun) {
		writeError(w, http.StatusConflict, service.ErrStaleRun.Error())
		return
	}
	writeError(w, http.StatusBadRequest, err.Error())
}

// StartTask marks a dispatched task as running.
type TaskStartRequest struct {
	RunID string `json:"run_id"`
}

func (h *Handler) StartTask(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")
	var req TaskStartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	ref, err := daemonRunRef(taskID, req.RunID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	started, err := h.TaskService.StartTask(r.Context(), ref)
	if err != nil {
		slog.Warn("start task failed", "task_id", taskID, "error", err)
		writeRunTransitionError(w, err)
		return
	}

	resp := taskToResponse(*started)
	resp.RunID = req.RunID
	slog.Info("task started", "task_id", taskID, "run_id", resp.RunID, "agent_id", uuidToString(started.AgentID))
	writeJSON(w, http.StatusOK, resp)
}

// ReportTaskProgress broadcasts a progress update.
type TaskProgressRequest struct {
	RunID   string `json:"run_id"`
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
	ref, err := daemonRunRef(taskID, req.RunID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Look up task to get workspace ID via the associated issue.
	workspaceID := ""
	task, err := h.Queries.GetAgentTask(r.Context(), ref.WorkItemID)
	if err == nil {
		if issue, err := h.Queries.GetIssue(r.Context(), task.IssueID); err == nil {
			workspaceID = uuidToString(issue.WorkspaceID)
		}
	}

	if err := h.TaskService.ReportProgress(r.Context(), ref, workspaceID, req.Summary, req.Step, req.Total); err != nil {
		writeRunTransitionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ReportAgentStage receives an agent stage change from the daemon.
type TaskStageRequest struct {
	RunID string `json:"run_id"`
	Stage string `json:"stage"` // "reading", "implementing", "testing", "committing", "done"
}

func (h *Handler) ReportAgentStage(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")

	var req TaskStageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	ref, err := daemonRunRef(taskID, req.RunID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	task, err := h.Queries.GetAgentTask(r.Context(), ref.WorkItemID)
	if err != nil {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}

	workspaceID := ""
	if issue, err := h.Queries.GetIssue(r.Context(), task.IssueID); err == nil {
		workspaceID = uuidToString(issue.WorkspaceID)
	}

	if err := h.TaskService.ReportAgentStage(r.Context(), ref, uuidToString(task.AgentID), workspaceID, service.AgentStage(req.Stage)); err != nil {
		writeRunTransitionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// CompleteTask marks a running task as completed.
type TaskCompleteRequest struct {
	RunID      string                   `json:"run_id"`
	PRURL      string                   `json:"pr_url"`
	Output     string                   `json:"output"`
	SessionID  string                   `json:"session_id"` // provider session ID for future resumption
	WorkDir    string                   `json:"work_dir"`   // working directory used during execution
	DurationMs int64                    `json:"duration_ms,omitempty"`
	TokenUsage *protocol.TaskTokenUsage `json:"token_usage,omitempty"`
	Artifacts  []protocol.TaskArtifact  `json:"artifacts,omitempty"`
}

type TaskSessionCheckpointRequest struct {
	RunID     string `json:"run_id"`
	SessionID string `json:"session_id"`
	WorkDir   string `json:"work_dir"`
}

// CheckpointTaskSession records resumable state before provider execution
// completes, so a fresh daemon process can continue an interrupted task.
func (h *Handler) CheckpointTaskSession(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")
	var req TaskSessionCheckpointRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.SessionID = strings.TrimSpace(req.SessionID)
	req.WorkDir = strings.TrimSpace(req.WorkDir)
	if req.SessionID == "" || req.WorkDir == "" {
		writeError(w, http.StatusBadRequest, "session_id and work_dir are required")
		return
	}
	ref, err := daemonRunRef(taskID, req.RunID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	taskRecord, err := h.Queries.GetAgentTask(r.Context(), ref.WorkItemID)
	if err != nil {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	issue, err := h.Queries.GetIssue(r.Context(), taskRecord.IssueID)
	if err != nil {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	if _, ok := h.requireWorkspaceMember(w, r, uuidToString(issue.WorkspaceID), "task not found"); !ok {
		return
	}

	task, err := h.TaskService.CheckpointTaskSession(r.Context(), ref, req.SessionID, req.WorkDir)
	if err != nil {
		slog.Warn("checkpoint task session failed", "task_id", taskID, "error", err)
		writeRunTransitionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, taskToResponse(*task))
}

func (h *Handler) CompleteTask(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")

	var req TaskCompleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := validateTaskCompletion(req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	ref, err := daemonRunRef(taskID, req.RunID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	result, _ := json.Marshal(req)
	task, err := h.TaskService.CompleteTask(r.Context(), ref, result, req.SessionID, req.WorkDir)
	if err != nil {
		slog.Warn("complete task failed", "task_id", taskID, "error", err)
		writeRunTransitionError(w, err)
		return
	}

	slog.Info("task completed", "task_id", taskID, "agent_id", uuidToString(task.AgentID))
	writeJSON(w, http.StatusOK, taskToResponse(*task))
}

func validateTaskCompletion(req TaskCompleteRequest) error {
	if req.DurationMs < 0 {
		return fmt.Errorf("duration_ms must not be negative")
	}
	if usage := req.TokenUsage; usage != nil {
		if usage.InputTokens < 0 || usage.OutputTokens < 0 || usage.ReasoningOutputTokens < 0 ||
			usage.CacheReadTokens < 0 || usage.CacheWriteTokens < 0 {
			return fmt.Errorf("token_usage values must not be negative")
		}
	}
	if len(req.Artifacts) > 100 {
		return fmt.Errorf("artifacts must contain at most 100 entries")
	}
	for i, artifact := range req.Artifacts {
		if strings.TrimSpace(artifact.Kind) == "" {
			return fmt.Errorf("artifacts[%d].kind is required", i)
		}
		if strings.TrimSpace(artifact.Path) == "" && strings.TrimSpace(artifact.URI) == "" {
			return fmt.Errorf("artifacts[%d] requires path or uri", i)
		}
		if artifact.SHA256 != "" {
			digest, err := hex.DecodeString(artifact.SHA256)
			if err != nil || len(digest) != 32 {
				return fmt.Errorf("artifacts[%d].sha256 must be a 64-character hex digest", i)
			}
		}
	}
	return nil
}

// GetTaskStatus returns the current status of a task.
// Used by the daemon to check whether a task was cancelled mid-execution.
func (h *Handler) GetTaskStatus(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")
	ref, err := daemonRunRef(taskID, r.URL.Query().Get("run_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	task, err := h.TaskService.Lifecycle.Status(r.Context(), ref)
	if err != nil {
		writeRunTransitionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": task.Status})
}

// FailTask marks a running task as failed.
type TaskFailRequest struct {
	RunID string `json:"run_id"`
	Error string `json:"error"`
}

func (h *Handler) FailTask(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")

	var req TaskFailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	ref, err := daemonRunRef(taskID, req.RunID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	task, err := h.TaskService.FailTask(r.Context(), ref, req.Error)
	if err != nil {
		slog.Warn("fail task failed", "task_id", taskID, "error", err)
		writeRunTransitionError(w, err)
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
	RunID    string               `json:"run_id"`
	Messages []TaskMessageRequest `json:"messages"`
}

const (
	maxTaskMessageBodyBytes = 8 * 1024 * 1024
	defaultTaskMessageLimit = 500
	maxTaskMessageLimit     = 5000
)

var taskMessageTypes = map[string]bool{
	"text":        true,
	"thinking":    true,
	"tool_use":    true,
	"tool_result": true,
	"error":       true,
}

func validateTaskMessage(msg TaskMessageRequest) error {
	if msg.Seq <= 0 || int64(msg.Seq) > 2147483647 {
		return fmt.Errorf("seq must be between 1 and 2147483647")
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
func (h *Handler) persistTaskMessage(ctx context.Context, task db.AgentTaskQueue, runID pgtype.UUID, workspaceID, traceID string, msg TaskMessageRequest) (bool, error) {
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

	outcome, err := h.Queries.CreateTaskMessage(ctx, db.CreateTaskMessageParams{
		TaskID:  task.ID,
		RunID:   runID,
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
	if !outcome.Active {
		return false, service.ErrStaleRun
	}
	if !outcome.Inserted {
		return false, nil
	}

	if traceID != "" {
		h.recordTraceMessage(ctx, traceID, msg)
	}
	h.publish(protocol.EventTaskMessage, workspaceID, "system", "", protocol.TaskMessagePayload{
		TaskID:  uuidToString(task.ID),
		RunID:   uuidToString(runID),
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
func (h *Handler) RecordGatewayTaskLog(ctx context.Context, workspaceID, taskID, runID string, seq int, stream, content string) error {
	task, err := h.TaskService.ValidateCloudGatewayTask(ctx, workspaceID, taskID)
	if err != nil {
		return err
	}
	ref, err := daemonRunRef(taskID, runID)
	if err != nil {
		return err
	}
	if _, err := h.TaskService.Lifecycle.AssertRunning(ctx, ref); err != nil {
		return err
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
	traceID := ""
	if h.TraceService != nil && h.TraceService.TraceService != nil {
		if trace, lookupErr := h.TraceService.GetTraceByRun(ctx, runID); lookupErr == nil {
			traceID = trace.ID
		}
	}
	_, err = h.persistTaskMessage(ctx, task, ref.RunID, workspaceID, traceID, msg)
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
	req.RunID = strings.TrimSpace(req.RunID)
	runID := parseUUID(req.RunID)
	if !runID.Valid {
		writeError(w, http.StatusBadRequest, "run_id is required")
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
	if _, err := h.Queries.GetActiveTaskRun(r.Context(), db.GetActiveTaskRunParams{
		TaskID: task.ID,
		ID:     runID,
	}); err != nil {
		writeError(w, http.StatusConflict, "run_id is not the active run for this task")
		return
	}

	// Look up execution trace for recording steps (best-effort).
	traceID := ""
	if h.TraceService != nil && h.TraceService.TraceService != nil {
		if trace, lookupErr := h.TraceService.GetTraceByRun(r.Context(), req.RunID); lookupErr == nil {
			traceID = trace.ID
		}
	}

	for _, msg := range req.Messages {
		if _, err := h.persistTaskMessage(r.Context(), task, runID, workspaceID, traceID, msg); err != nil {
			if errors.Is(err, service.ErrStaleRun) {
				writeRunTransitionError(w, err)
				return
			}
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
		if parseErr != nil || sinceSeq < 0 || int64(sinceSeq) > 2147483647 {
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
			RunID:   uuidToString(m.RunID),
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
