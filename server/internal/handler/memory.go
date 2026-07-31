// Issue #32 — workspace & agent memory CRUD.
//
// Routes (all authenticated; workspace membership enforced):
//   GET    /api/workspaces/{id}/memories            — list team memories
//   POST   /api/workspaces/{id}/memories            — create team memory
//   DELETE /api/workspaces/{id}/memories/{memoryId} — delete team memory
//   GET    /api/agents/{agentId}/memories            — list agent memories
//   POST   /api/agents/{agentId}/memories            — create agent memory
//   DELETE /api/agents/{agentId}/memories/{memoryId} — delete agent memory
//
// Agent-memory routes resolve the agent's workspace in the handler and
// require its own workspace membership check inline (via the same
// GetMemberByUserAndWorkspace query the URL-based middleware uses).

package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pgvector/pgvector-go"

	"github.com/agentra-ai/agentra/server/internal/handlerutil"
	"github.com/agentra-ai/agentra/server/internal/util"
	db "github.com/agentra-ai/agentra/server/pkg/db/generated"
)

type MemoryHandler struct {
	Queries *db.Queries
}

func NewMemoryHandler(q *db.Queries) *MemoryHandler {
	return &MemoryHandler{Queries: q}
}

// RegisterRoutes is kept for test/composition callers. Router wiring in
// cmd/server/router.go registers team + agent routes separately so each
// can live at the right nesting level (team under /{id}, agent at root).
func (h *MemoryHandler) RegisterRoutes(r chi.Router) {
	h.RegisterTeamRoutes(r)
	h.RegisterAgentRoutes(r)
}

// RegisterTeamRoutes wires workspace-scoped memory endpoints onto the
// caller's router (typically already nested under /api/workspaces/{id}).
func (h *MemoryHandler) RegisterTeamRoutes(r chi.Router) {
	r.Get("/", h.ListTeamMemories)
	r.Get("/search", h.SearchMemories)
	r.Post("/", h.CreateTeamMemory)
	r.Delete("/{memoryId}", h.DeleteTeamMemory)
}

type memorySearchItem struct {
	ID          string  `json:"id"`
	WorkspaceID string  `json:"workspace_id"`
	AgentID     string  `json:"agent_id,omitempty"`
	MemoryType  string  `json:"memory_type"`
	Content     string  `json:"content"`
	CreatedAt   string  `json:"created_at"`
	Score       float32 `json:"score"`
}

// SearchMemories performs workspace-scoped BM25 search across team memories
// and non-private agent memories. The workspace route middleware establishes
// the membership boundary before this handler runs.
func (h *MemoryHandler) SearchMemories(w http.ResponseWriter, r *http.Request) {
	wsID, ok := h.workspaceID(w, r)
	if !ok {
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		writeError(w, http.StatusBadRequest, "q query parameter is required")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	rows, err := h.Queries.SearchAllMemoriesBM25(r.Context(), db.SearchAllMemoriesBM25Params{
		WorkspaceID:    wsID,
		PlaintoTsquery: query,
		Limit:          int32(limit),
	})
	if err != nil {
		slog.Error("SearchMemories query failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to search memories")
		return
	}

	items := make([]memorySearchItem, len(rows))
	for i, row := range rows {
		items[i] = memorySearchItem{
			ID:          uuidToString(row.ID),
			WorkspaceID: uuidToString(wsID),
			AgentID:     uuidToString(row.AgentID),
			MemoryType:  row.MemoryType,
			Content:     row.Content,
			CreatedAt:   timestampToString(row.CreatedAt),
			Score:       row.Score,
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"memories": items})
}

// RegisterAgentRoutes wires agent-scoped memory endpoints onto the
// caller's router. Each handler resolves the agent's workspace inline and
// enforces membership there.
func (h *MemoryHandler) RegisterAgentRoutes(r chi.Router) {
	r.Get("/api/agents/{agentId}/memories", h.ListAgentMemories)
	r.Post("/api/agents/{agentId}/memories", h.CreateAgentMemory)
	r.Delete("/api/agents/{agentId}/memories/{memoryId}", h.DeleteAgentMemory)
}

// workspaceID extracts + validates the workspace id from the URL param.
func (h *MemoryHandler) workspaceID(w http.ResponseWriter, r *http.Request) (pgtype.UUID, bool) {
	id := chi.URLParam(r, "id")
	uid := util.ParseUUID(id)
	if !uid.Valid {
		writeError(w, http.StatusBadRequest, "invalid workspace id")
		return pgtype.UUID{}, false
	}
	return uid, true
}

// requireAgentWorkspaceMember resolves the agent from the URL param and
// confirms the caller is a member of that agent's workspace. On success it
// returns the agent ID, the workspace ID, and true.
func (h *MemoryHandler) requireAgentWorkspaceMember(w http.ResponseWriter, r *http.Request) (pgtype.UUID, pgtype.UUID, bool) {
	agentID := chi.URLParam(r, "agentId")
	aid := util.ParseUUID(agentID)
	if !aid.Valid {
		writeError(w, http.StatusBadRequest, "invalid agent id")
		return pgtype.UUID{}, pgtype.UUID{}, false
	}

	agent, err := h.Queries.GetAgent(r.Context(), aid)
	if err != nil {
		writeError(w, http.StatusNotFound, "agent not found")
		return pgtype.UUID{}, pgtype.UUID{}, false
	}

	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "user not authenticated")
		return pgtype.UUID{}, pgtype.UUID{}, false
	}

	if _, err := h.Queries.GetMemberByUserAndWorkspace(r.Context(), db.GetMemberByUserAndWorkspaceParams{
		UserID:      util.ParseUUID(userID),
		WorkspaceID: agent.WorkspaceID,
	}); err != nil {
		writeError(w, http.StatusForbidden, "not a member of this workspace")
		return pgtype.UUID{}, pgtype.UUID{}, false
	}

	return aid, agent.WorkspaceID, true
}

// zeroVector is a 1536-dimensional zero vector. Embeddings are a
// migration-level NOT NULL column (the ivfflat index needs a fixed
// dimension); the frontend doesn't send them on create, so we seed with
// zeros as a placeholder until an embedding backfill job runs.
var zeroVector = func() pgvector.Vector {
	v := make([]float32, 1536)
	return pgvector.NewVector(v)
}()

// --- Team memories -----------------------------------------------------------

func (h *MemoryHandler) ListTeamMemories(w http.ResponseWriter, r *http.Request) {
	wsID, ok := h.workspaceID(w, r)
	if !ok {
		return
	}

	items, err := h.Queries.ListTeamMemories(r.Context(), wsID)
	if err != nil {
		slog.Error("ListTeamMemories query failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list memories")
		return
	}
	if items == nil {
		items = []db.TeamMemory{}
	}
	writeJSON(w, http.StatusOK, items)
}

type teamMemoryBody struct {
	MemoryType string `json:"memory_type"`
	Content    string `json:"content"`
}

func (h *MemoryHandler) CreateTeamMemory(w http.ResponseWriter, r *http.Request) {
	wsID, ok := h.workspaceID(w, r)
	if !ok {
		return
	}

	var body teamMemoryBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Content) == "" {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.MemoryType == "" {
		body.MemoryType = "context"
	}
	if !validMemoryType(body.MemoryType) {
		writeError(w, http.StatusBadRequest, "invalid memory type")
		return
	}
	body.Content = strings.TrimSpace(body.Content)

	row, err := h.Queries.CreateTeamMemory(r.Context(), db.CreateTeamMemoryParams{
		WorkspaceID: wsID,
		MemoryType:  body.MemoryType,
		Content:     body.Content,
		CreatedBy:   util.ParseUUID(handlerutil.RequestUserID(r)),
		Embedding:   zeroVector,
	})
	if err != nil {
		slog.Error("CreateTeamMemory query failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create memory")
		return
	}
	writeJSON(w, http.StatusCreated, row)
}

func (h *MemoryHandler) DeleteTeamMemory(w http.ResponseWriter, r *http.Request) {
	wsID, ok := h.workspaceID(w, r)
	if !ok {
		return
	}

	uid := util.ParseUUID(chi.URLParam(r, "memoryId"))
	if !uid.Valid {
		writeError(w, http.StatusBadRequest, "invalid memory id")
		return
	}

	if _, err := h.Queries.DeleteTeamMemory(r.Context(), db.DeleteTeamMemoryParams{
		ID:          uid,
		WorkspaceID: wsID,
	}); err != nil {
		writeError(w, http.StatusNotFound, "memory not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Agent memories ----------------------------------------------------------

func (h *MemoryHandler) ListAgentMemories(w http.ResponseWriter, r *http.Request) {
	aid, _, ok := h.requireAgentWorkspaceMember(w, r)
	if !ok {
		return
	}

	items, err := h.Queries.ListAgentMemories(r.Context(), aid)
	if err != nil {
		slog.Error("ListAgentMemories query failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list memories")
		return
	}
	if items == nil {
		items = []db.AgentMemory{}
	}
	writeJSON(w, http.StatusOK, items)
}

type agentMemoryBody struct {
	MemoryType string `json:"memory_type"`
	Content    string `json:"content"`
	IsPrivate  *bool  `json:"is_private,omitempty"`
}

func (h *MemoryHandler) CreateAgentMemory(w http.ResponseWriter, r *http.Request) {
	aid, wsID, ok := h.requireAgentWorkspaceMember(w, r)
	if !ok {
		return
	}

	var body agentMemoryBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Content) == "" {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.MemoryType == "" {
		body.MemoryType = "context"
	}
	if !validMemoryType(body.MemoryType) {
		writeError(w, http.StatusBadRequest, "invalid memory type")
		return
	}
	body.Content = strings.TrimSpace(body.Content)

	isPrivate := pgtype.Bool{Valid: true, Bool: false}
	if body.IsPrivate != nil {
		isPrivate.Bool = *body.IsPrivate
	}

	row, err := h.Queries.CreateAgentMemory(r.Context(), db.CreateAgentMemoryParams{
		AgentID:     aid,
		WorkspaceID: wsID,
		MemoryType:  body.MemoryType,
		Content:     body.Content,
		IsPrivate:   isPrivate,
		Embedding:   zeroVector,
	})
	if err != nil {
		slog.Error("CreateAgentMemory query failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create memory")
		return
	}
	writeJSON(w, http.StatusCreated, row)
}

func (h *MemoryHandler) DeleteAgentMemory(w http.ResponseWriter, r *http.Request) {
	aid, _, ok := h.requireAgentWorkspaceMember(w, r)
	if !ok {
		return
	}

	uid := util.ParseUUID(chi.URLParam(r, "memoryId"))
	if !uid.Valid {
		writeError(w, http.StatusBadRequest, "invalid memory id")
		return
	}

	if _, err := h.Queries.DeleteAgentMemory(r.Context(), db.DeleteAgentMemoryParams{
		ID:      uid,
		AgentID: aid,
	}); err != nil {
		writeError(w, http.StatusNotFound, "memory not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func validMemoryType(memoryType string) bool {
	switch memoryType {
	case "learning", "task_result", "context", "pattern":
		return true
	default:
		return false
	}
}
