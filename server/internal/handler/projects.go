package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/agentra-ai/agentra/server/internal/logger"
	db "github.com/agentra-ai/agentra/server/pkg/db/generated"
)

// ProjectResponse is the JSON response for a project.
type ProjectResponse struct {
	ID          string  `json:"id"`
	WorkspaceID string  `json:"workspace_id"`
	Title       string  `json:"title"`
	Slug        string  `json:"slug"`
	OwnerID     string  `json:"owner_id"`
	Deadline    *string `json:"deadline"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

func projectToResponse(p db.Project) ProjectResponse {
	return ProjectResponse{
		ID:          uuidToString(p.ID),
		WorkspaceID: uuidToString(p.WorkspaceID),
		Title:       p.Title,
		Slug:        p.Slug,
		OwnerID:     uuidToString(p.OwnerID),
		Deadline:    timestampToPtr(p.Deadline),
		CreatedAt:   timestampToString(p.CreatedAt),
		UpdatedAt:   timestampToString(p.UpdatedAt),
	}
}

// MilestoneResponse is the JSON response for a milestone.
type MilestoneResponse struct {
	ID        string  `json:"id"`
	ProjectID string  `json:"project_id"`
	Title     string  `json:"title"`
	Deadline  *string `json:"deadline"`
	Status    string  `json:"status"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

func milestoneToResponse(m db.Milestone) MilestoneResponse {
	return MilestoneResponse{
		ID:        uuidToString(m.ID),
		ProjectID: uuidToString(m.ProjectID),
		Title:     m.Title,
		Deadline:  timestampToPtr(m.Deadline),
		Status:    m.Status,
		CreatedAt: timestampToString(m.CreatedAt),
		UpdatedAt: timestampToString(m.UpdatedAt),
	}
}

// ProjectHandler handles project-related HTTP requests.
type ProjectHandler struct {
	Queries *db.Queries
	Hub     interface{} // reserved for future broadcast
}

// NewProjectHandler creates a new ProjectHandler.
func NewProjectHandler(queries *db.Queries) *ProjectHandler {
	return &ProjectHandler{Queries: queries}
}

func (h *ProjectHandler) projectInWorkspace(ctx context.Context, workspaceID, projectID string) (db.Project, error) {
	project, err := h.Queries.GetProject(ctx, parseUUID(projectID))
	if err != nil {
		return db.Project{}, err
	}
	if project.WorkspaceID != parseUUID(workspaceID) {
		return db.Project{}, pgx.ErrNoRows
	}
	return project, nil
}

func (h *ProjectHandler) requireProject(w http.ResponseWriter, r *http.Request) (db.Project, bool) {
	project, err := h.projectInWorkspace(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "projectId"))
	if err == nil {
		return project, true
	}
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "project not found")
	} else {
		writeError(w, http.StatusInternalServerError, "failed to load project")
	}
	return db.Project{}, false
}

// RegisterRoutes registers project routes. Caller is responsible for mounting
// the router under a workspace-scoped path with appropriate middleware.
func (h *ProjectHandler) RegisterRoutes(r chi.Router) {
	r.Get("/", h.ListProjects)
	r.Post("/", h.CreateProject)
	r.Get("/unassigned", h.ListUnassignedIssues)
	r.Route("/{projectId}", func(r chi.Router) {
		r.Get("/", h.GetProject)
		r.Put("/", h.UpdateProject)
		r.Delete("/", h.DeleteProject)
		r.Get("/issues", h.ListProjectIssues)
		r.Post("/issues/{issueId}", h.AssignOrRemoveIssue)
		r.Get("/milestones", h.ListMilestones)
		r.Post("/milestones", h.CreateMilestone)
		r.Patch("/milestones/{milestoneId}", h.UpdateMilestone)
	})
}

type CreateProjectRequest struct {
	Title    string  `json:"title"`
	Slug     string  `json:"slug"`
	Deadline *string `json:"deadline"`
}

func (h *ProjectHandler) CreateProject(w http.ResponseWriter, r *http.Request) {
	var req CreateProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Title == "" || req.Slug == "" {
		writeError(w, http.StatusBadRequest, "title and slug are required")
		return
	}

	workspaceID := chi.URLParam(r, "id")
	ownerID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	var deadline pgtype.Timestamptz
	if req.Deadline != nil && *req.Deadline != "" {
		t, err := time.Parse(time.RFC3339, *req.Deadline)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid deadline format, expected RFC3339")
			return
		}
		deadline = pgtype.Timestamptz{Time: t, Valid: true}
	}

	project, err := h.Queries.CreateProject(r.Context(), db.CreateProjectParams{
		WorkspaceID: parseUUID(workspaceID),
		Title:       req.Title,
		Slug:        req.Slug,
		OwnerID:     parseUUID(ownerID),
		Deadline:    deadline,
	})
	if err != nil {
		slog.Warn("create project failed", append(logger.RequestAttrs(r), "error", err, "workspace_id", workspaceID)...)
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "project slug already exists in this workspace")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create project")
		return
	}

	resp := projectToResponse(project)
	slog.Info("project created", append(logger.RequestAttrs(r), "project_id", resp.ID, "title", project.Title)...)
	writeJSON(w, http.StatusCreated, resp)
}

func (h *ProjectHandler) ListProjects(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "id")
	projects, err := h.Queries.ListProjectsByWorkspace(r.Context(), parseUUID(workspaceID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list projects")
		return
	}

	resp := make([]ProjectResponse, len(projects))
	for i, p := range projects {
		resp[i] = projectToResponse(p)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *ProjectHandler) GetProject(w http.ResponseWriter, r *http.Request) {
	project, ok := h.requireProject(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, projectToResponse(project))
}

type UpdateProjectRequest struct {
	Title    *string `json:"title"`
	Slug     *string `json:"slug"`
	Deadline *string `json:"deadline"`
}

func (h *ProjectHandler) UpdateProject(w http.ResponseWriter, r *http.Request) {
	projectId := chi.URLParam(r, "projectId")
	if _, ok := h.requireProject(w, r); !ok {
		return
	}

	var req UpdateProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	params := db.UpdateProjectParams{ID: parseUUID(projectId)}
	if req.Title != nil {
		params.Title = pgtype.Text{String: *req.Title, Valid: true}
	}
	if req.Slug != nil {
		params.Slug = pgtype.Text{String: *req.Slug, Valid: true}
	}
	if req.Deadline != nil {
		if *req.Deadline == "" {
			params.Deadline = pgtype.Timestamptz{Valid: false}
		} else {
			t, err := time.Parse(time.RFC3339, *req.Deadline)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid deadline format, expected RFC3339")
				return
			}
			params.Deadline = pgtype.Timestamptz{Time: t, Valid: true}
		}
	}

	project, err := h.Queries.UpdateProject(r.Context(), params)
	if err != nil {
		slog.Warn("update project failed", append(logger.RequestAttrs(r), "error", err, "project_id", projectId)...)
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "project slug already exists in this workspace")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update project")
		return
	}

	writeJSON(w, http.StatusOK, projectToResponse(project))
}

func (h *ProjectHandler) DeleteProject(w http.ResponseWriter, r *http.Request) {
	projectId := chi.URLParam(r, "projectId")
	if _, ok := h.requireProject(w, r); !ok {
		return
	}
	err := h.Queries.DeleteProject(r.Context(), parseUUID(projectId))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete project")
		return
	}
	slog.Info("project deleted", append(logger.RequestAttrs(r), "project_id", projectId)...)
	w.WriteHeader(http.StatusNoContent)
}

func (h *ProjectHandler) ListUnassignedIssues(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "id")
	issues, err := h.Queries.ListIssuesWithoutProject(r.Context(), parseUUID(workspaceID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list unassigned issues")
		return
	}
	resp := make([]IssueResponse, len(issues))
	prefix := "" // no issue prefix lookup needed for picker context
	for i, issue := range issues {
		resp[i] = issueToResponse(issue, prefix)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *ProjectHandler) ListProjectIssues(w http.ResponseWriter, r *http.Request) {
	projectId := chi.URLParam(r, "projectId")
	if _, ok := h.requireProject(w, r); !ok {
		return
	}
	issues, err := h.Queries.ListIssuesByProject(r.Context(), parseUUID(projectId))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list project issues")
		return
	}
	resp := make([]IssueResponse, len(issues))
	prefix := ""
	for i, issue := range issues {
		resp[i] = issueToResponse(issue, prefix)
	}
	writeJSON(w, http.StatusOK, resp)
}

type AssignIssueRequest struct {
	Action string `json:"action"` // "assign" | "remove"
}

func (h *ProjectHandler) AssignOrRemoveIssue(w http.ResponseWriter, r *http.Request) {
	projectId := chi.URLParam(r, "projectId")
	issueId := chi.URLParam(r, "issueId")
	workspaceID := chi.URLParam(r, "id")
	if _, ok := h.requireProject(w, r); !ok {
		return
	}

	var req AssignIssueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	switch req.Action {
	case "assign":
		issue, err := h.Queries.AssignIssueToProject(r.Context(), db.AssignIssueToProjectParams{
			ID:          parseUUID(issueId),
			ProjectID:   parseUUID(projectId),
			WorkspaceID: parseUUID(workspaceID),
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to assign issue to project")
			return
		}
		writeJSON(w, http.StatusOK, issueToResponse(issue, ""))
	case "remove":
		issue, err := h.Queries.GetIssueInWorkspace(r.Context(), db.GetIssueInWorkspaceParams{
			ID:          parseUUID(issueId),
			WorkspaceID: parseUUID(workspaceID),
		})
		if err != nil || !issue.ProjectID.Valid || issue.ProjectID != parseUUID(projectId) {
			writeError(w, http.StatusNotFound, "issue not found in project")
			return
		}
		issue, err = h.Queries.RemoveIssueFromProject(r.Context(), db.RemoveIssueFromProjectParams{
			ID:          parseUUID(issueId),
			WorkspaceID: parseUUID(workspaceID),
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to remove issue from project")
			return
		}
		writeJSON(w, http.StatusOK, issueToResponse(issue, ""))
	default:
		writeError(w, http.StatusBadRequest, "invalid action: must be 'assign' or 'remove'")
	}
}

type CreateMilestoneRequest struct {
	Title    string  `json:"title"`
	Deadline *string `json:"deadline"`
}

func (h *ProjectHandler) CreateMilestone(w http.ResponseWriter, r *http.Request) {
	projectId := chi.URLParam(r, "projectId")
	if _, ok := h.requireProject(w, r); !ok {
		return
	}

	var req CreateMilestoneRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Title == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}

	var deadline pgtype.Timestamptz
	if req.Deadline != nil && *req.Deadline != "" {
		t, err := time.Parse(time.RFC3339, *req.Deadline)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid deadline format")
			return
		}
		deadline = pgtype.Timestamptz{Time: t, Valid: true}
	}

	milestone, err := h.Queries.CreateMilestone(r.Context(), db.CreateMilestoneParams{
		ProjectID: parseUUID(projectId),
		Title:     req.Title,
		Deadline:  deadline,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create milestone")
		return
	}

	writeJSON(w, http.StatusCreated, milestoneToResponse(milestone))
}

func (h *ProjectHandler) ListMilestones(w http.ResponseWriter, r *http.Request) {
	projectId := chi.URLParam(r, "projectId")
	if _, ok := h.requireProject(w, r); !ok {
		return
	}
	milestones, err := h.Queries.ListMilestonesByProject(r.Context(), parseUUID(projectId))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list milestones")
		return
	}
	resp := make([]MilestoneResponse, len(milestones))
	for i, m := range milestones {
		resp[i] = milestoneToResponse(m)
	}
	writeJSON(w, http.StatusOK, resp)
}

type UpdateMilestoneRequest struct {
	Status   *string `json:"status"`
	Title    *string `json:"title"`
	Deadline *string `json:"deadline"`
}

func (h *ProjectHandler) UpdateMilestone(w http.ResponseWriter, r *http.Request) {
	projectId := chi.URLParam(r, "projectId")
	milestoneId := chi.URLParam(r, "milestoneId")
	if _, ok := h.requireProject(w, r); !ok {
		return
	}
	milestones, err := h.Queries.ListMilestonesByProject(r.Context(), parseUUID(projectId))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load milestones")
		return
	}
	found := false
	for _, milestone := range milestones {
		if milestone.ID == parseUUID(milestoneId) {
			found = true
			break
		}
	}
	if !found {
		writeError(w, http.StatusNotFound, "milestone not found")
		return
	}

	var req UpdateMilestoneRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	params := db.UpdateMilestoneStatusParams{ID: parseUUID(milestoneId)}
	if req.Status != nil {
		params.Status = pgtype.Text{String: *req.Status, Valid: true}
	}
	if req.Title != nil {
		params.Title = pgtype.Text{String: *req.Title, Valid: true}
	}
	if req.Deadline != nil {
		if *req.Deadline == "" {
			params.Deadline = pgtype.Timestamptz{Valid: false}
		} else {
			t, err := time.Parse(time.RFC3339, *req.Deadline)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid deadline format")
				return
			}
			params.Deadline = pgtype.Timestamptz{Time: t, Valid: true}
		}
	}

	milestone, err := h.Queries.UpdateMilestoneStatus(r.Context(), params)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update milestone")
		return
	}

	writeJSON(w, http.StatusOK, milestoneToResponse(milestone))
}
