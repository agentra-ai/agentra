package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	looppkg "github.com/agentra-ai/agentra/server/internal/loop"
)

// CreateLoop handles POST /api/loops.
// Body: {"issue_id": "...", "max_iterations"?: int, "agent_id"?: "..."}.
func (h *Handler) CreateLoop(w http.ResponseWriter, r *http.Request) {
	workspaceID := resolveWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	if _, ok := requireUserID(w, r); !ok {
		return
	}

	var req struct {
		IssueID       string  `json:"issue_id"`
		MaxIterations *int    `json:"max_iterations"`
		AgentID       *string `json:"agent_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.IssueID == "" {
		writeError(w, http.StatusBadRequest, "issue_id is required")
		return
	}

	loop, err := h.LoopStore.CreateLoop(r.Context(), looppkg.CreateLoopInput{
		IssueID:       req.IssueID,
		WorkspaceID:   workspaceID,
		MaxIterations: req.MaxIterations,
		AgentID:       req.AgentID,
	})
	if err != nil {
		slog.Warn("create loop failed", "error", err, "issue_id", req.IssueID, "workspace_id", workspaceID)
		writeError(w, http.StatusInternalServerError, "failed to create loop")
		return
	}

	// Kick off the first stage (loop_plan). The rest of the loop is event-
	// driven via task:completed / task:failed, but the plan stage has no
	// preceding task to fire on, so the handler has to start it. If the
	// coordinator is not wired (some unit tests) or StartLoop itself fails
	// (e.g. transient DB error), we still return the created loop — the row
	// exists in 'pending' status and can be retried by an operator, and a
	// hard failure to the caller would have to undo a successful INSERT.
	//
	// On success, refetch the loop so the response reflects the post-start
	// state (status=running, current_stage=plan, started_at) — callers
	// frequently poll the just-created loop and would otherwise see stale
	// 'pending' status.
	if h.LoopCoordinator != nil {
		if err := h.LoopCoordinator.StartLoop(r.Context(), loop.ID); err != nil {
			slog.Warn("start loop failed; loop created in pending status, will not auto-resume",
				"loop_id", loop.ID, "error", err)
		} else if refreshed, gerr := h.LoopStore.GetLoop(r.Context(), loop.ID); gerr == nil {
			loop = refreshed
		}
	}

	slog.Info("loop created", "loop_id", loop.ID, "issue_id", loop.IssueID, "workspace_id", workspaceID)
	writeJSON(w, http.StatusCreated, loop)
}

// GetLoop handles GET /api/loops/{id}.
func (h *Handler) GetLoop(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	workspaceID := resolveWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	if _, ok := requireUserID(w, r); !ok {
		return
	}

	loop, err := h.LoopStore.GetLoop(r.Context(), id)
	if err != nil {
		if errors.Is(err, looppkg.ErrLoopNotFound) || errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "loop not found")
			return
		}
		slog.Warn("get loop failed", "error", err, "loop_id", id)
		writeError(w, http.StatusInternalServerError, "failed to get loop")
		return
	}
	if loop.WorkspaceID != workspaceID {
		writeError(w, http.StatusNotFound, "loop not found")
		return
	}
	writeJSON(w, http.StatusOK, loop)
}

// ListLoops handles GET /api/loops.
// Optional query params: status, issue_id, limit (default 100).
func (h *Handler) ListLoops(w http.ResponseWriter, r *http.Request) {
	workspaceID := resolveWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	if _, ok := requireUserID(w, r); !ok {
		return
	}

	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}

	var statusFilter *looppkg.Status
	if s := r.URL.Query().Get("status"); s != "" {
		st := looppkg.Status(s)
		statusFilter = &st
	}
	var issueFilter *string
	if iid := r.URL.Query().Get("issue_id"); iid != "" {
		issueFilter = &iid
	}

	loops, err := h.LoopStore.ListLoops(r.Context(), looppkg.ListLoopsInput{
		WorkspaceID: workspaceID,
		Status:      statusFilter,
		IssueID:     issueFilter,
		Limit:       limit,
	})
	if err != nil {
		slog.Warn("list loops failed", "error", err, "workspace_id", workspaceID)
		writeError(w, http.StatusInternalServerError, "failed to list loops")
		return
	}
	if loops == nil {
		loops = []*looppkg.Loop{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"loops": loops,
		"total": len(loops),
	})
}

// PauseLoop handles POST /api/loops/{id}/pause.
func (h *Handler) PauseLoop(w http.ResponseWriter, r *http.Request) {
	paused := looppkg.StatusPaused
	h.transitionLoopStatus(w, r, &paused)
}

// ResumeLoop handles POST /api/loops/{id}/resume.
func (h *Handler) ResumeLoop(w http.ResponseWriter, r *http.Request) {
	running := looppkg.StatusRunning
	h.transitionLoopStatus(w, r, &running)
}

// CancelLoop handles POST /api/loops/{id}/cancel.
func (h *Handler) CancelLoop(w http.ResponseWriter, r *http.Request) {
	cancelled := looppkg.StatusCancelled
	h.transitionLoopStatus(w, r, &cancelled)
}

// transitionLoopStatus is shared by Pause/Resume/Cancel. It enforces
// workspace ownership and returns the updated loop.
func (h *Handler) transitionLoopStatus(w http.ResponseWriter, r *http.Request, target *looppkg.Status) {
	workspaceID := resolveWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	if _, ok := requireUserID(w, r); !ok {
		return
	}

	id := chi.URLParam(r, "id")
	existing, err := h.LoopStore.GetLoop(r.Context(), id)
	if err != nil {
		if errors.Is(err, looppkg.ErrLoopNotFound) || errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "loop not found")
			return
		}
		slog.Warn("get loop failed", "error", err, "loop_id", id)
		writeError(w, http.StatusInternalServerError, "failed to update loop")
		return
	}
	if existing.WorkspaceID != workspaceID {
		writeError(w, http.StatusNotFound, "loop not found")
		return
	}

	updated, err := h.LoopStore.UpdateStatus(r.Context(), id, looppkg.UpdateStatusInput{
		Status: target,
	})
	if err != nil {
		slog.Warn("update loop status failed", "error", err, "loop_id", id, "status", *target)
		writeError(w, http.StatusInternalServerError, "failed to update loop")
		return
	}
	writeJSON(w, http.StatusOK, updated)
}
