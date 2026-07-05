package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/agentra-ai/agentra/server/internal/events"
	"github.com/agentra-ai/agentra/server/internal/logger"
	"github.com/agentra-ai/agentra/server/internal/realtime"
	db "github.com/agentra-ai/agentra/server/pkg/db/generated"
)

const (
	CommentTypeApprovalRequested = "approval_requested"
	CommentTypeApprovalGranted   = "approval_granted"
	CommentTypeApprovalRejected  = "approval_rejected"
)

// TaskApprovalHandler handles pause/approve/reject lifecycle for agent tasks.
type TaskApprovalHandler struct {
	Queries *db.Queries
	Hub     *realtime.Hub
	Bus     *events.Bus
}

func NewTaskApprovalHandler(q *db.Queries, hub *realtime.Hub, bus *events.Bus) *TaskApprovalHandler {
	return &TaskApprovalHandler{Queries: q, Hub: hub, Bus: bus}
}

func (h *TaskApprovalHandler) RegisterRoutes(r chi.Router) {
	r.Post("/{taskId}/pause", h.Pause)
	r.Post("/{taskId}/approve", h.Approve)
	r.Post("/{taskId}/reject", h.Reject)
}

func (h *TaskApprovalHandler) taskByID(w http.ResponseWriter, r *http.Request) (pgtype.UUID, bool) {
	id := chi.URLParam(r, "taskId")
	uid := parseUUID(id)
	if !uid.Valid {
		writeError(w, http.StatusBadRequest, "invalid task id")
		return pgtype.UUID{}, false
	}
	return uid, true
}

// Pause: agent requests human sign-off. Creates an "approval_requested" comment.
func (h *TaskApprovalHandler) Pause(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	taskID, ok := h.taskByID(w, r)
	if !ok {
		return
	}

	// Pull existing task for context.
	task, err := h.Queries.GetAgentTask(ctx, taskID)
	if err != nil {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}

	// Mark taskPausing via comments table (task has no approval_status — we use type	// = 'approval_requested' comment on the issue as signal).
	comment, err := h.Queries.CreateComment(ctx, db.CreateCommentParams{
		IssueID:     task.IssueID,
		AuthorType:  "agent",
		AuthorID:    task.AgentID,
		Content:     fmt.Sprintf("Approval requested — waiting for owner/admin sign-off. Task paused at %s", time.Now().Format(time.RFC3339)),
		Type:        CommentTypeApprovalRequested,
		WorkspaceID: task.WorkspaceID,
	})
	if err != nil {
		logger.Error("pause: create comment", "error", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if h.Hub != nil {
		h.Hub.Broadcast(realtime.CreateEvent("task:approval_realtime-ISSUE", task.IssueID, map[string]any{
			"task_id":     taskID.String(),
			"comment_id":  comment.ID.String(),
			"status":      "approval_requested",
		}))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":     "approval_requested",
		"comment_id": comment.ID.String(),
	})
}

// Approve: owner/admin grants sign-off → task returns to active execution.
func (h *TaskApprovalHandler) Approve(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	taskID, ok := h.taskByID(w, r)
	if !ok {
		return
	}

	task, err := h.Queries.GetAgentTask(ctx, taskID)
	if err != nil {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}

	comment, err := h.Queries.CreateComment(ctx, db.CreateCommentParams{
		IssueID:     task.IssueID,
		AuthorType:  "member",
		AuthorID:    pgtype.UUID{}, // filled by trigger
		Content:     fmt.Sprintf("Approval granted by human — task resuming at %s", time.Now().Format(time.RFC3339)),
		Type:        CommentTypeApprovalGranted,
		WorkspaceID: task.WorkspaceID,
		ParentID:    pgtype.UUID{}, // back-reference
	})
	if err != nil {
		logger.Error("approve: create comment", "error", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if h.Hub != nil {
		h.Hub.Broadcast(realtime.CreateEvent("task:approved.updated", task.IssueID, map[string]any{
			"task_id":  taskID.String(),
			"status":   "approved",
		}))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":     "approved",
		"comment_id": comment.ID.String(),
	})
}

// Reject: owner/admin rejects the task's request → change task status to failed.
func (h *TaskApprovalHandler) Reject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	taskID, ok := h.taskByID(w, r)
	if !ok {
		return
	}

	var req struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	task, err := h.Queries.GetAgentTask(ctx, taskID)
	if err != nil {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}

	// leave comment record
	_, _ = h.Queries.CreateComment(ctx, db.CreateCommentParams{
		IssueID:     task.IssueID,
		AuthorType:  "member",
		Content:     fmt.Sprintf("Approval rejected: %s", req.Reason),
		Type:        CommentTypeApprovalRejected,
		WorkspaceID: task.WorkspaceID,
	})

	// cancel the task
	if h.Bus != nil {
		h.Bus.Publish("task:reject", taskID.String())
	}

	if h.Hub != nil {
		h.Hub.Broadcast(realtime.CreateEvent("task:approval_rejected", task.IssueID, map[string]any{
			"task_id": taskID.String(),
			"reason":  req.Reason,
		}))
	}

	writeJSON(w, http.StatusOK, map[string]any{"status": "rejected"})
}
