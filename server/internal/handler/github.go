package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/agentra-ai/agentra/pkg/db/generated"
)

type GitHubHandler struct {
	queries *db.Queries
}

func NewGitHubHandler(queries *db.Queries) *GitHubHandler {
	return &GitHubHandler{queries: queries}
}

func (h *GitHubHandler) RegisterRoutes(r chi.Router) {
	r.Get("/workspaces/{id}/github/installations", h.ListInstallations)
	r.Post("/workspaces/{id}/github/connect", h.ConnectGitHub)
	r.Delete("/workspaces/{id}/github/disconnect", h.DisconnectGitHub)
}

func (h *GitHubHandler) ListInstallations(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "id")
	inst, err := h.queries.GetInstallation(r.Context(), uuid.MustParse(workspaceID))
	if err != nil {
		http.Error(w, "not found", 404)
		return
	}
	json.NewEncoder(w).Encode(inst)
}

func (h *GitHubHandler) ConnectGitHub(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "id")
	var req struct {
		InstallationID int64  `json:"installation_id"`
		AccountLogin  string `json:"account_login"`
		AccountType   string `json:"account_type"`
		AccessToken  string `json:"access_token"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	inst, err := h.queries.CreateInstallation(r.Context(), db.CreateInstallationParams{
		WorkspaceID:    uuid.MustParse(workspaceID),
		InstallationID: req.InstallationID,
		AccountLogin:  req.AccountLogin,
		AccountType:   req.AccountType,
		AccessToken:   req.AccessToken,
		RefreshToken:  pgtype.Text{Status: pgtype.Null},
		TokenExpiresAt: pgtype.Timestamptz{Status: pgtype.Null},
		Repositories:  []byte("[]"),
	})
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	json.NewEncoder(w).Encode(inst)
}

func (h *GitHubHandler) DisconnectGitHub(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "id")
	inst, err := h.queries.GetInstallation(r.Context(), uuid.MustParse(workspaceID))
	if err != nil {
		http.Error(w, "not found", 404)
		return
	}
	h.queries.DeleteInstallation(r.Context(), inst.ID)
	w.WriteHeader(204)
}