package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/agentra-ai/agentra/server/pkg/db/generated"
	"github.com/agentra-ai/agentra/server/internal/service"
)

// ExecuteRequest is the body for POST /api/workspaces/:id/execute.
type ExecuteRequest struct {
	Goal        string            `json:"goal"`
	Team        []string          `json:"team"`
	Constraints map[string]string `json:"constraints"`
}

// ExecuteResponse is returned after goal execution is initiated.
type ExecuteResponse struct {
	TaskGraphID string `json:"task_graph_id"`
	IssueID     string `json:"issue_id"`
	Message     string `json:"message"`
}

// ExecuteGoal handles POST /api/workspaces/:id/execute.
// Creates an issue from a goal string and auto-decomposes it into a task graph.
func (h *Handler) ExecuteGoal(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "id")
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}

	var req ExecuteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if req.Goal == "" {
		writeError(w, http.StatusBadRequest, "goal is required")
		return
	}

	title := req.Goal
	if len(title) > 200 {
		title = title[:200] + "..."
	}

	wsUUID := parseUUID(workspaceID)
	issue, err := h.Queries.CreateIssue(r.Context(), db.CreateIssueParams{
		WorkspaceID: wsUUID,
		Title:       title,
		Description: pgtype.Text{String: req.Goal, Valid: true},
		Status:      "open",
		Priority:    "medium",
		CreatorType: "member",
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create issue: "+err.Error())
		return
	}

	force := true
	decomposeResult, err := h.PlannerService.DecomposeIssue(r.Context(), workspaceID, issue.ID.String(), service.DecomposeOptions{
		Provider:          "anthropic",
		MaxNodes:          10,
		AdditionalContext: formatConstraints(req.Constraints),
		Force:             force,
	})
	if err != nil {
		writeJSON(w, http.StatusOK, ExecuteResponse{
			IssueID: issue.ID.String(),
			Message: "Issue created. Auto-decomposition failed: " + err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, ExecuteResponse{
		TaskGraphID: issue.ID.String(),
		IssueID:     issue.ID.String(),
		Message:     fmt.Sprintf("Goal executed. Task graph created with %d nodes.", len(decomposeResult.Nodes)),
	})
}

func formatConstraints(constraints map[string]string) string {
	if len(constraints) == 0 {
		return ""
	}
	var parts []string
	for k, v := range constraints {
		parts = append(parts, k+": "+v)
	}
	return "Constraints: " + joinStrings(parts, ", ")
}

func joinStrings(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += sep + parts[i]
	}
	return result
}