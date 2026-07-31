package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/agentra-ai/agentra/pkg/taskgraph"
	db "github.com/agentra-ai/agentra/server/pkg/db/generated"
)

type taskGraphResponse struct {
	Nodes []taskgraph.GraphNode `json:"nodes"`
	Edges []taskgraph.GraphEdge `json:"edges"`
}

type updateTaskGraphNodeRequest struct {
	AgentID   *string          `json:"agent_id"`
	Status    *string          `json:"status"`
	Context   *json.RawMessage `json:"context"`
	Result    *json.RawMessage `json:"result"`
	PositionX *float64         `json:"position_x"`
	PositionY *float64         `json:"position_y"`
}

// GetTaskGraph returns the persisted nodes and edges for an issue in the
// workspace authorized by RequireWorkspaceMember.
func (h *Handler) GetTaskGraph(w http.ResponseWriter, r *http.Request) {
	issueID := chi.URLParam(r, "id")
	workspaceID := ctxWorkspaceID(r.Context())
	if !h.issueBelongsToWorkspace(w, r, issueID, workspaceID) {
		return
	}

	nodes, err := h.GraphStore.ListNodesByIssue(r.Context(), issueID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list task graph nodes")
		return
	}
	edges, err := h.GraphStore.ListEdgesByIssue(r.Context(), issueID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list task graph edges")
		return
	}
	if nodes == nil {
		nodes = []taskgraph.GraphNode{}
	}
	if edges == nil {
		edges = []taskgraph.GraphEdge{}
	}

	writeJSON(w, http.StatusOK, taskGraphResponse{Nodes: nodes, Edges: edges})
}

// UpdateTaskGraphNode applies the provided mutable fields to a node after
// confirming the node belongs to the authorized workspace.
func (h *Handler) UpdateTaskGraphNode(w http.ResponseWriter, r *http.Request) {
	nodeID := chi.URLParam(r, "id")
	if !h.taskGraphNodeBelongsToWorkspace(w, r, nodeID) {
		return
	}

	var req updateTaskGraphNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Status != nil && !validTaskGraphNodeStatus(*req.Status) {
		writeError(w, http.StatusBadRequest, "invalid task graph node status")
		return
	}

	params := &taskgraph.UpdateNodeParams{
		AgentID:   req.AgentID,
		Status:    req.Status,
		PositionX: req.PositionX,
		PositionY: req.PositionY,
	}
	if req.Context != nil {
		params.Context = []byte(*req.Context)
	}
	if req.Result != nil {
		params.Result = []byte(*req.Result)
	}

	node, err := h.GraphStore.UpdateNode(r.Context(), nodeID, params)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "task graph node not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update task graph node")
		return
	}
	writeJSON(w, http.StatusOK, node)
}

// DeleteTaskGraphNode removes a node after confirming its workspace boundary.
func (h *Handler) DeleteTaskGraphNode(w http.ResponseWriter, r *http.Request) {
	nodeID := chi.URLParam(r, "id")
	if !h.taskGraphNodeBelongsToWorkspace(w, r, nodeID) {
		return
	}

	if err := h.GraphStore.DeleteNode(r.Context(), nodeID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "task graph node not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete task graph node")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) issueBelongsToWorkspace(w http.ResponseWriter, r *http.Request, issueID, workspaceID string) bool {
	issueUUID := parseUUID(issueID)
	workspaceUUID := parseUUID(workspaceID)
	if !issueUUID.Valid || !workspaceUUID.Valid {
		writeError(w, http.StatusBadRequest, "invalid issue or workspace id")
		return false
	}

	_, err := h.Queries.GetIssueInWorkspace(r.Context(), db.GetIssueInWorkspaceParams{
		ID:          issueUUID,
		WorkspaceID: workspaceUUID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "issue not found")
			return false
		}
		writeError(w, http.StatusInternalServerError, "failed to load issue")
		return false
	}
	return true
}

func (h *Handler) taskGraphNodeBelongsToWorkspace(w http.ResponseWriter, r *http.Request, nodeID string) bool {
	if !parseUUID(nodeID).Valid {
		writeError(w, http.StatusBadRequest, "invalid task graph node id")
		return false
	}

	node, err := h.GraphStore.GetNode(r.Context(), nodeID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "task graph node not found")
			return false
		}
		writeError(w, http.StatusInternalServerError, "failed to load task graph node")
		return false
	}
	if node.WorkspaceID != ctxWorkspaceID(r.Context()) {
		writeError(w, http.StatusNotFound, "task graph node not found")
		return false
	}
	return true
}

func validTaskGraphNodeStatus(status string) bool {
	switch taskgraph.NodeStatus(status) {
	case taskgraph.StatusPending,
		taskgraph.StatusRunning,
		taskgraph.StatusCompleted,
		taskgraph.StatusFailed,
		taskgraph.StatusBlocked:
		return true
	default:
		return false
	}
}
