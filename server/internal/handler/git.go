package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/agentra-ai/agentra/server/pkg/db/generated"
	"github.com/agentra-ai/agentra/server/internal/util"
)

// Git hooks API - links commits/PRs/branches to issues

func (h *Handler) LinkCommit(w http.ResponseWriter, r *http.Request) {
	workspaceID := resolveWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}

	var req struct {
		IssueID  string `json:"issueId"`
		SHA      string `json:"sha"`
		Message  string `json:"message"`
		Branch   string `json:"branch"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}

	if req.IssueID == "" || req.SHA == "" {
		writeError(w, http.StatusBadRequest, "issueId and sha are required")
		return
	}

	issueUUID := util.ParseUUID(req.IssueID)
	if issueUUID == (pgtype.UUID{}) {
		writeError(w, http.StatusBadRequest, "invalid issueId")
		return
	}

	wsUUID := util.ParseUUID(workspaceID)

	// Verify issue belongs to workspace
	_, err := h.Queries.GetIssueInWorkspace(r.Context(), db.GetIssueInWorkspaceParams{
		ID:          issueUUID,
		WorkspaceID: wsUUID,
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "issue not found")
		return
	}

	_, err = h.Queries.LinkCommit(r.Context(), db.LinkCommitParams{
		IssueID: issueUUID,
		Sha:     pgtype.Text{String: req.SHA, Valid: req.SHA != ""},
		Message: pgtype.Text{String: req.Message, Valid: req.Message != ""},
		Branch:  pgtype.Text{String: req.Branch, Valid: req.Branch != ""},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to link commit: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "linked", "sha": req.SHA})
}

func (h *Handler) LinkPR(w http.ResponseWriter, r *http.Request) {
	workspaceID := resolveWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}

	var req struct {
		IssueID  string  `json:"issueId"`
		PRNumber int32   `json:"prNumber"`
		PRURL    string  `json:"prUrl"`
		PRState  string  `json:"prState"`
		PRTitle  string  `json:"prTitle"`
		MergedAt *string `json:"mergedAt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}

	if req.IssueID == "" || req.PRNumber == 0 {
		writeError(w, http.StatusBadRequest, "issueId and prNumber are required")
		return
	}

	issueUUID := util.ParseUUID(req.IssueID)
	if issueUUID == (pgtype.UUID{}) {
		writeError(w, http.StatusBadRequest, "invalid issueId")
		return
	}

	wsUUID := util.ParseUUID(workspaceID)

	// Verify issue belongs to workspace
	_, err := h.Queries.GetIssueInWorkspace(r.Context(), db.GetIssueInWorkspaceParams{
		ID:          issueUUID,
		WorkspaceID: wsUUID,
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "issue not found")
		return
	}

	var mergedAt pgtype.Timestamptz
	if req.MergedAt != nil && *req.MergedAt != "" {
		_ = mergedAt.Scan(*req.MergedAt)
	}

	_, err = h.Queries.LinkPR(r.Context(), db.LinkPRParams{
		IssueID:    issueUUID,
		Repository: "", // not provided in this endpoint
		PrNumber:   pgtype.Int4{Int32: req.PRNumber, Valid: req.PRNumber != 0},
		PrState:    pgtype.Text{String: req.PRState, Valid: req.PRState != ""},
		PrTitle:    pgtype.Text{String: req.PRTitle, Valid: req.PRTitle != ""},
		MergedAt:   mergedAt,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to link PR: "+err.Error())
		return
	}

	// If merged, auto-transition issue to Done
	if req.PRState == "closed" && req.MergedAt != nil {
		_, _ = h.Queries.UpdateIssue(r.Context(), db.UpdateIssueParams{
			ID:     issueUUID,
			Status: pgtype.Text{String: "done", Valid: true},
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "linked",
		"prNumber": req.PRNumber,
		"autoDone": req.PRState == "closed" && req.MergedAt != nil,
	})
}

func (h *Handler) LinkBranch(w http.ResponseWriter, r *http.Request) {
	workspaceID := resolveWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}

	var req struct {
		IssueID string `json:"issueId"`
		Branch  string `json:"branch"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}

	if req.IssueID == "" || req.Branch == "" {
		writeError(w, http.StatusBadRequest, "issueId and branch are required")
		return
	}

	issueUUID := util.ParseUUID(req.IssueID)
	if issueUUID == (pgtype.UUID{}) {
		writeError(w, http.StatusBadRequest, "invalid issueId")
		return
	}

	wsUUID := util.ParseUUID(workspaceID)

	// Verify issue belongs to workspace
	_, err := h.Queries.GetIssueInWorkspace(r.Context(), db.GetIssueInWorkspaceParams{
		ID:          issueUUID,
		WorkspaceID: wsUUID,
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "issue not found")
		return
	}

	_, err = h.Queries.LinkBranch(r.Context(), db.LinkBranchParams{
		IssueID: issueUUID,
		Branch:  pgtype.Text{String: req.Branch, Valid: req.Branch != ""},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to link branch: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "linked", "branch": req.Branch})
}

func (h *Handler) GetActiveTask(w http.ResponseWriter, r *http.Request) {
	branch := r.URL.Query().Get("branch")
	if branch == "" {
		writeError(w, http.StatusBadRequest, "branch query param is required")
		return
	}

	// Find an issue linked to this branch that has an in-progress task
	links, err := h.Queries.GetGitLinksByIssue(r.Context(), pgtype.UUID{})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query: "+err.Error())
		return
	}

	workspaceID := resolveWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}

	wsUUID := parseUUIDFromString(workspaceID)

	for _, link := range links {
		if link.Branch.String != branch {
			continue
		}
		// Get the issue
		issue, err := h.Queries.GetIssueInWorkspace(r.Context(), db.GetIssueInWorkspaceParams{
			ID:          link.IssueID,
			WorkspaceID: wsUUID,
		})
		if err != nil {
			continue
		}
		if issue.Status == "in_progress" {
			writeJSON(w, http.StatusOK, map[string]any{
				"active":  true,
				"issueId": issue.ID,
				"status":  "in_progress",
				"branch":  branch,
			})
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"active": false,
		"branch": branch,
	})
}

func (h *Handler) GetIssueLinks(w http.ResponseWriter, r *http.Request) {
	issueID := chi.URLParam(r, "issueId")
	if issueID == "" {
		writeError(w, http.StatusBadRequest, "issueId is required")
		return
	}

	issueUUID := util.ParseUUID(issueID)
	if issueUUID == (pgtype.UUID{}) {
		writeError(w, http.StatusBadRequest, "invalid issueId")
		return
	}

	links, err := h.Queries.GetGitLinksByIssue(r.Context(), issueUUID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get links: "+err.Error())
		return
	}

	// Convert to serializable format
	type LinkView struct {
		ID        string `json:"id"`
		IssueID   string `json:"issueId"`
		LinkType  string `json:"link_type"`
		SHA       string `json:"sha,omitempty"`
		Branch    string `json:"branch,omitempty"`
		PRNumber  int32  `json:"prNumber,omitempty"`
		PRState   string `json:"prState,omitempty"`
		PRTitle   string `json:"prTitle,omitempty"`
		MergedAt  string `json:"mergedAt,omitempty"`
		Message   string `json:"message,omitempty"`
	}

	result := make([]LinkView, len(links))
	for i, l := range links {
		result[i] = LinkView{
			ID:       l.ID.String(),
			IssueID:  l.IssueID.String(),
			LinkType: l.LinkType,
		}
		if l.Sha.Valid {
			result[i].SHA = l.Sha.String
		}
		if l.Branch.Valid {
			result[i].Branch = l.Branch.String
		}
		if l.PrNumber.Valid {
			result[i].PRNumber = l.PrNumber.Int32
		}
		if l.PrState.Valid {
			result[i].PRState = l.PrState.String
		}
		if l.PrTitle.Valid {
			result[i].PRTitle = l.PrTitle.String
		}
		if l.MergedAt.Valid {
			result[i].MergedAt = l.MergedAt.Time.Format("2006-01-02T15:04:05Z")
		}
		if l.Message.Valid {
			result[i].Message = l.Message.String
		}
	}

	writeJSON(w, http.StatusOK, result)
}

// parseUUIDFromString converts a string to pgtype.UUID, returning an invalid UUID on error.
func parseUUIDFromString(s string) pgtype.UUID {
	var u pgtype.UUID
	if err := u.Scan(s); err != nil {
		return pgtype.UUID{}
	}
	return u
}