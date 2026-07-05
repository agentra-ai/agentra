package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/agentra-ai/agentra/server/internal/middleware"
	"github.com/agentra-ai/agentra/server/internal/util"
	db "github.com/agentra-ai/agentra/server/pkg/db/generated"
)

type MemberHandler struct {
	Queries *db.Queries
}

func NewMemberHandler(q *db.Queries) *MemberHandler {
	return &MemberHandler{Queries: q}
}

func (h *MemberHandler) RegisterRoutes(r chi.Router) {
	r.Use(middleware.RequireWorkspaceMemberFromURL(h.Queries, "id"))
	r.Get("/seats", h.GetSeats)
	r.Get("/members", h.ListMembers)
	r.With(middleware.RequireWorkspaceRoleFromURL(h.Queries, "id", "owner", "admin")).Post("/members/invite", h.InviteMember)
	r.With(middleware.RequireWorkspaceRoleFromURL(h.Queries, "id", "owner", "admin")).Delete("/members/{memberId}", h.RemoveMember)
	r.With(middleware.RequireWorkspaceRoleFromURL(h.Queries, "id", "owner", "admin")).Patch("/members/{memberId}", h.UpdateMemberRole)
}

type InviteRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

type UpdateRoleRequest struct {
	Role string `json:"role"`
}

func seatMemberToResponse(m db.ListMembersWithUserRow) map[string]any {
	resp := map[string]any{
		"id":                util.UUIDToString(m.ID),
		"user_id":           util.UUIDToString(m.UserID),
		"role":              m.Role,
		"invitation_status": m.InvitationStatus,
		"email":             m.UserEmail,
		"name":              m.UserName,
	}
	if m.UserAvatarUrl.Valid {
		resp["avatar_url"] = m.UserAvatarUrl.String
	}
	if m.CreatedAt.Valid {
		resp["joined_at"] = m.CreatedAt.Time.Format(time.RFC3339)
	}
	return resp
}

func (h *MemberHandler) GetSeats(w http.ResponseWriter, r *http.Request) {
	wsID := util.ParseUUID(chi.URLParam(r, "id"))
	if !wsID.Valid {
		writeError(w, http.StatusBadRequest, "invalid workspace id")
		return
	}
	plan, err := h.Queries.GetWorkspacePlan(r.Context(), wsID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	used, err := h.Queries.CountActiveMembers(r.Context(), wsID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	members, err := h.Queries.ListMembersWithUser(r.Context(), wsID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp := map[string]any{
		"used":    used,
		"max":     plan.MaxSeats,
		"plan":    plan.Plan,
		"members": make([]map[string]any, 0, len(members)),
	}
	for _, m := range members {
		resp["members"] = append(resp["members"].([]map[string]any), seatMemberToResponse(m))
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *MemberHandler) ListMembers(w http.ResponseWriter, r *http.Request) {
	wsID := util.ParseUUID(chi.URLParam(r, "id"))
	if !wsID.Valid {
		writeError(w, http.StatusBadRequest, "invalid workspace id")
		return
	}
	members, err := h.Queries.ListMembersWithUser(r.Context(), wsID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp := make([]map[string]any, 0, len(members))
	for _, m := range members {
		resp = append(resp, seatMemberToResponse(m))
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *MemberHandler) InviteMember(w http.ResponseWriter, r *http.Request) {
	wsID := util.ParseUUID(chi.URLParam(r, "id"))
	if !wsID.Valid {
		writeError(w, http.StatusBadRequest, "invalid workspace id")
		return
	}
	var req InviteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Email == "" {
		writeError(w, http.StatusBadRequest, "email required")
		return
	}
	if req.Role == "" {
		req.Role = "member"
	}

	plan, err := h.Queries.GetWorkspacePlan(r.Context(), wsID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	used, err := h.Queries.CountActiveMembers(r.Context(), wsID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if int32(used) >= plan.MaxSeats {
		writeError(w, http.StatusConflict,
			fmt.Sprintf("seat limit reached (%d/%d); upgrade plan to add more seats", used, plan.MaxSeats))
		return
	}

	user, err := h.Queries.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found; they must login first")
		return
	}
	if _, err := h.Queries.GetMemberByUserAndWorkspace(r.Context(), db.GetMemberByUserAndWorkspaceParams{
		UserID:      user.ID,
		WorkspaceID: wsID,
	}); err == nil {
		writeError(w, http.StatusConflict, "user already a member of this workspace")
		return
	}

	member, err := h.Queries.CreateMember(r.Context(), db.CreateMemberParams{
		WorkspaceID: wsID,
		UserID:      user.ID,
		Role:        req.Role,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := h.Queries.UpdateMemberInvitationStatus(r.Context(), db.UpdateMemberInvitationStatusParams{
		ID:               member.ID,
		InvitationStatus: "invited",
	}); err != nil {
		// non-fatal
	}

	writeJSON(w, http.StatusCreated, seatMemberToResponse(db.ListMembersWithUserRow{
		ID:               member.ID,
		WorkspaceID:      member.WorkspaceID,
		UserID:           member.UserID,
		Role:             member.Role,
		InvitationStatus: "invited",
		CreatedAt:        member.CreatedAt,
		UserName:         user.Name,
		UserEmail:        user.Email,
	}))
}

func (h *MemberHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	wsID := util.ParseUUID(chi.URLParam(r, "id"))
	if !wsID.Valid {
		writeError(w, http.StatusBadRequest, "invalid workspace id")
		return
	}
	memberID := util.ParseUUID(chi.URLParam(r, "memberId"))
	if !memberID.Valid {
		writeError(w, http.StatusBadRequest, "invalid member id")
		return
	}
	callerID := requestUserID(r)
	member, err := h.Queries.GetMember(r.Context(), memberID)
	if err != nil {
		writeError(w, http.StatusNotFound, "member not found")
		return
	}
	if util.UUIDToString(member.UserID) == callerID {
		writeError(w, http.StatusForbidden, "cannot remove yourself")
		return
	}
	if err := h.Queries.DeleteMember(r.Context(), memberID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *MemberHandler) UpdateMemberRole(w http.ResponseWriter, r *http.Request) {
	wsID := util.ParseUUID(chi.URLParam(r, "id"))
	if !wsID.Valid {
		writeError(w, http.StatusBadRequest, "invalid workspace id")
		return
	}
	memberID := util.ParseUUID(chi.URLParam(r, "memberId"))
	if !memberID.Valid {
		writeError(w, http.StatusBadRequest, "invalid member id")
		return
	}
	var req UpdateRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Role == "" {
		writeError(w, http.StatusBadRequest, "role required")
		return
	}
	updated, err := h.Queries.UpdateMemberRole(r.Context(), db.UpdateMemberRoleParams{
		ID:   memberID,
		Role: req.Role,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":   util.UUIDToString(updated.ID),
		"role": updated.Role,
	})
}

var _ = pgtype.UUID{}
