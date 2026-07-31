package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/agentra-ai/agentra/pkg/taskgraph"
	"github.com/agentra-ai/agentra/server/internal/service"
)

// AutoDecomposeRequest is the body for POST /api/issues/{id}/auto-decompose.
type AutoDecomposeRequest struct {
	Provider          string `json:"provider"`
	Model             string `json:"model"`
	MaxNodes          int    `json:"max_nodes"`
	AdditionalContext string `json:"additional_context"`
}

// AutoDecomposeResponse is returned after successful decomposition.
type AutoDecomposeResponse struct {
	Plan  string                `json:"plan"`
	Nodes []taskgraph.GraphNode `json:"nodes"`
	Edges []taskgraph.GraphEdge `json:"edges"`
	Usage map[string]any        `json:"usage,omitempty"`
}

// AutoDecomposeIssue handles POST /api/issues/{id}/auto-decompose.
func (h *Handler) AutoDecomposeIssue(w http.ResponseWriter, r *http.Request) {
	issueID := chi.URLParam(r, "id")
	if issueID == "" {
		writeError(w, http.StatusBadRequest, "issue ID is required")
		return
	}

	workspaceID := resolveWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}

	// Parse request body.
	var req AutoDecomposeRequest
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
	}

	force := strings.EqualFold(r.URL.Query().Get("force"), "true")

	opts := service.DecomposeOptions{
		Provider:          req.Provider,
		Model:             req.Model,
		MaxNodes:          req.MaxNodes,
		AdditionalContext: req.AdditionalContext,
		Force:             force,
	}

	if h.PlannerService == nil {
		http.Error(w, "planner service not initialized", http.StatusInternalServerError)
		return
	}
	result, err := h.PlannerService.DecomposeIssue(r.Context(), workspaceID, issueID, opts)
	if err != nil {
		h.handlePlannerError(w, err)
		return
	}

	// Map response.
	resp := AutoDecomposeResponse{
		Plan:  result.Plan,
		Nodes: result.Nodes,
		Edges: result.Edges,
		Usage: result.TokenUsage,
	}

	writeJSON(w, http.StatusOK, resp)

	slog.Info("issue auto-decomposed",
		"issue_id", issueID,
		"workspace_id", workspaceID,
		"node_count", len(result.Nodes),
		"edge_count", len(result.Edges),
		"provider", opts.Provider,
		"model", opts.Model,
	)
}

// handlePlannerError maps planner errors to HTTP status codes.
func (h *Handler) handlePlannerError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrNoDescription):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, service.ErrGraphExists):
		writeError(w, http.StatusConflict, "task graph already exists for this issue; use ?force=true to overwrite")
	case errors.Is(err, service.ErrInvalidDAG):
		writeError(w, http.StatusUnprocessableEntity, err.Error())
	default:
		slog.Error("auto-decompose failed", "error", err)
		writeError(w, http.StatusInternalServerError, "auto-decompose failed: "+err.Error())
	}
}
