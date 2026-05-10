package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

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
	Nodes []autoDecomposeNode   `json:"nodes"`
	Edges []autoDecomposeEdge   `json:"edges"`
	Usage map[string]any        `json:"usage,omitempty"`
}

type autoDecomposeNode struct {
	ID          string         `json:"id"`
	NodeType    string         `json:"node_type"`
	AgentID     string         `json:"agent_id,omitempty"`
	Status      string         `json:"status"`
	Context     map[string]any `json:"context"`
	Depth       int            `json:"depth"`
	PositionX   float64        `json:"position_x"`
	PositionY   float64        `json:"position_y"`
}

type autoDecomposeEdge struct {
	ID         string `json:"id"`
	FromNodeID string `json:"from_node_id"`
	ToNodeID   string `json:"to_node_id"`
	EdgeType   string `json:"edge_type"`
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
		Nodes: make([]autoDecomposeNode, len(result.Nodes)),
		Edges: make([]autoDecomposeEdge, len(result.Edges)),
		Usage: result.TokenUsage,
	}

	for i, n := range result.Nodes {
		resp.Nodes[i] = autoDecomposeNode{
			ID:        n.ID,
			NodeType:  string(n.NodeType),
			AgentID:   n.AgentID,
			Status:    string(n.Status),
			Context:   n.Context,
			Depth:     n.Depth,
			PositionX: n.PositionX,
			PositionY: n.PositionY,
		}
	}

	for i, e := range result.Edges {
		resp.Edges[i] = autoDecomposeEdge{
			ID:         e.ID,
			FromNodeID: e.FromNodeID,
			ToNodeID:   e.ToNodeID,
			EdgeType:   string(e.EdgeType),
		}
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

