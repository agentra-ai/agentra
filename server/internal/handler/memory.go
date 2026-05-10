package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/agentra-ai/agentra/server/pkg/memory"
)

type MemoryHandler struct {
	svc *memory.MemoryService
}

func NewMemoryHandler(svc *memory.MemoryService) *MemoryHandler {
	return &MemoryHandler{svc: svc}
}

func (h *MemoryHandler) RegisterRoutes(r chi.Router) {
	r.Get("/workspaces/{id}/memories", h.ListMemories)
	r.Post("/workspaces/{id}/memories", h.CreateMemory)
	r.Get("/agents/{id}/memories", h.ListAgentMemories)
	r.Patch("/memories/{id}", h.UpdateMemory)
	r.Delete("/memories/{id}", h.DeleteMemory)
	r.Get("/memories/search", h.SearchMemories)
}

func (h *MemoryHandler) ListMemories(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "id")
	memories, err := h.svc.ListTeamMemories(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, memories)
}

func (h *MemoryHandler) CreateMemory(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MemoryType string `json:"memory_type"`
		Content    string `json:"content"`
		IsPrivate  bool   `json:"is_private"`
		AgentID    string `json:"agent_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	workspaceID := chi.URLParam(r, "id")
	var result *memory.StoreResult
	var err error
	if req.AgentID != "" {
		result, err = h.svc.StoreAgentMemory(r.Context(), req.AgentID, workspaceID, memory.MemoryType(req.MemoryType), req.Content, req.IsPrivate)
	} else {
		result, err = h.svc.StoreTeamMemory(r.Context(), workspaceID, memory.MemoryType(req.MemoryType), req.Content, "")
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *MemoryHandler) ListAgentMemories(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "id")
	// For now, just use SearchAll to get agent memories
	memories, err := h.svc.SearchAll(r.Context(), agentID, "", true, 50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, memories)
}

func (h *MemoryHandler) UpdateMemory(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not implemented")
}

func (h *MemoryHandler) DeleteMemory(w http.ResponseWriter, r *http.Request) {
	memoryID := chi.URLParam(r, "id")
	agentID := r.URL.Query().Get("agent_id")
	if agentID == "" {
		writeError(w, http.StatusBadRequest, "agent_id required")
		return
	}
	if err := h.svc.DeleteAgentMemory(r.Context(), memoryID, agentID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (h *MemoryHandler) SearchMemories(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	workspaceID := r.URL.Query().Get("workspace_id")
	results, err := h.svc.SearchAll(r.Context(), workspaceID, query, true, 20)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"memories": results})
}
