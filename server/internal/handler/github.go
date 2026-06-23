package handler

import (
	"encoding/json"
	"net/http"

	"github.com/agentra-ai/agentra/server/internal/util"
	db "github.com/agentra-ai/agentra/server/pkg/db/generated"
	"github.com/go-chi/chi"
	"github.com/jackc/pgx/v5/pgtype"
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

// requireWorkspaceUUID pulls the {id} path param and returns it as a
// pgtype.UUID. If the path param is not a valid UUID it writes a 400
// response and returns ok=false — used by every handler in this file
// so they don't have to repeat the parse + error-write pattern.
func (h *GitHubHandler) requireWorkspaceUUID(w http.ResponseWriter, r *http.Request) (pgtype.UUID, bool) {
	workspaceID := chi.URLParam(r, "id")
	u := util.ParseUUID(workspaceID)
	if !u.Valid {
		http.Error(w, "invalid workspace id", http.StatusBadRequest)
		return pgtype.UUID{}, false
	}
	return u, true
}

func (h *GitHubHandler) ListInstallations(w http.ResponseWriter, r *http.Request) {
	wsUUID, ok := h.requireWorkspaceUUID(w, r)
	if !ok {
		return
	}
	inst, err := h.queries.GetInstallation(r.Context(), wsUUID)
	if err != nil {
		http.Error(w, "not found", 404)
		return
	}
	json.NewEncoder(w).Encode(inst)
}

func (h *GitHubHandler) ConnectGitHub(w http.ResponseWriter, r *http.Request) {
	wsUUID, ok := h.requireWorkspaceUUID(w, r)
	if !ok {
		return
	}
	var req struct {
		InstallationID int64  `json:"installation_id"`
		AccountLogin   string `json:"account_login"`
		AccountType    string `json:"account_type"`
		AccessToken    string `json:"access_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	inst, err := h.queries.CreateInstallation(r.Context(), db.CreateInstallationParams{
		WorkspaceID:    wsUUID,
		InstallationID: req.InstallationID,
		AccountLogin:   req.AccountLogin,
		AccountType:    req.AccountType,
		AccessToken:    req.AccessToken,
		RefreshToken:   pgtype.Text{},
		TokenExpiresAt: pgtype.Timestamptz{},
		Repositories:   []byte("[]"),
	})
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	json.NewEncoder(w).Encode(inst)
}

func (h *GitHubHandler) DisconnectGitHub(w http.ResponseWriter, r *http.Request) {
	wsUUID, ok := h.requireWorkspaceUUID(w, r)
	if !ok {
		return
	}
	inst, err := h.queries.GetInstallation(r.Context(), wsUUID)
	if err != nil {
		http.Error(w, "not found", 404)
		return
	}
	if err := h.queries.DeleteInstallation(r.Context(), inst.ID); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.WriteHeader(204)
}
