package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/agentra-ai/agentra/server/internal/middleware"
	db "github.com/agentra-ai/agentra/server/pkg/db/generated"
)

type ProjectHandler struct {
	Queries *db.Queries
}

func NewProjectHandler(q *db.Queries) *ProjectHandler {
	return &ProjectHandler{Queries: q}
}

func (h *ProjectHandler) RegisterRoutes(r chi.Router) {
	r.With(middleware.RequireWorkspaceMember()).Get("/", h.List)
	r.With(middleware.RequireWorkspaceRole("owner", "admin")).Post("/", h.Create)
	r.Get("/{projectId}", h.Get)
	r.With(middleware.RequireWorkspaceRole("owner", "admin")).Put("/{projectId}", h.Update)
	r.With(middleware.RequireWorkspaceRole("owner", "admin")).Delete("/{projectId}", h.Delete)
	r.With(middleware.RequireWorkspaceRole("owner", "admin")).Post("/{projectId}/issues/{issueId}/{action}", h.AssignIssue)
	r.Get("/{projectId}/issues", h.ListIssues)
	r.Get("/{projectId}/milestones", h.ListMilestones)
	r.With(middleware.RequireWorkspaceRole("owner", "admin")).Post("/{projectId}/milestones", h.CreateMilestone)
	r.With(middleware.RequireWorkspaceRole("owner", "admin")).Patch("/{projectId}/milestones/{milestoneId}", h.UpdateMilestone)
}

func (h *ProjectHandler) Create(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "TODO")
}

func (h *ProjectHandler) List(w http.ResponseWriter, r *http.Request)         { writeError(w, http.StatusNotImplemented, "TODO") }
func (h *ProjectHandler) Get(w http.ResponseWriter, r *http.Request)          { writeError(w, http.StatusNotImplemented, "TODO") }
func (h *ProjectHandler) Update(w http.ResponseWriter, r *http.Request)       { writeError(w, http.StatusNotImplemented, "TODO") }
func (h *ProjectHandler) Delete(w http.ResponseWriter, r *http.Request)       { writeError(w, http.StatusNotImplemented, "TODO") }
func (h *ProjectHandler) AssignIssue(w http.ResponseWriter, r *http.Request)  { writeError(w, http.StatusNotImplemented, "TODO") }
func (h *ProjectHandler) ListIssues(w http.ResponseWriter, r *http.Request)   { writeError(w, http.StatusNotImplemented, "TODO") }
func (h *ProjectHandler) ListMilestones(w http.ResponseWriter, r *http.Request) { writeError(w, http.StatusNotImplemented, "TODO") }
func (h *ProjectHandler) CreateMilestone(w http.ResponseWriter, r *http.Request) { writeError(w, http.StatusNotImplemented, "TODO") }
func (h *ProjectHandler) UpdateMilestone(w http.ResponseWriter, r *http.Request) { writeError(w, http.StatusNotImplemented, "TODO") }
