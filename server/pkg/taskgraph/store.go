package taskgraph

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/google/uuid"

	db "github.com/agentra-ai/agentra/server/pkg/db/generated"
)

// GraphStore wraps the generated sqlc Queries for task graph persistence.
type GraphStore struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

// NewGraphStore creates a new GraphStore backed by the given connection pool.
func NewGraphStore(pool *pgxpool.Pool) *GraphStore {
	return &GraphStore{pool: pool, queries: db.New(pool)}
}

// CreateNode inserts a new task graph node. The contextJSON is stored as-is.
func (s *GraphStore) CreateNode(ctx context.Context, workspaceID, issueID string, nodeType NodeType, depth int, contextJSON []byte) (*GraphNode, error) {
	wsUUID, err := uuid.Parse(workspaceID)
	if err != nil {
		return nil, fmt.Errorf("invalid workspace_id: %w", err)
	}
	issUUID, err := uuid.Parse(issueID)
	if err != nil {
		return nil, fmt.Errorf("invalid issue_id: %w", err)
	}

	row, err := s.queries.CreateTaskNode(ctx, db.CreateTaskNodeParams{
		WorkspaceID: uuidToPg(wsUUID),
		IssueID:     uuidToPg(issUUID),
		AgentID:     pgtype.UUID{}, // unassigned on creation
		NodeType:    string(nodeType),
		Context:     safeBytes(contextJSON),
		Depth:       pgtype.Int4{Int32: int32(depth), Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("create node: %w", err)
	}
	return nodeFromDB(&row), nil
}

// GetNode retrieves a single node by ID.
func (s *GraphStore) GetNode(ctx context.Context, id string) (*GraphNode, error) {
	nodeID, err := parseUUID(id)
	if err != nil {
		return nil, err
	}
	row, err := s.queries.GetTaskNode(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("get node: %w", err)
	}
	return nodeFromDB(&row), nil
}

// ListNodesByIssue returns all nodes for an issue, ordered by depth then creation time.
func (s *GraphStore) ListNodesByIssue(ctx context.Context, issueID string) ([]GraphNode, error) {
	issUUID, err := parseUUID(issueID)
	if err != nil {
		return nil, err
	}
	rows, err := s.queries.ListNodesByIssue(ctx, issUUID)
	if err != nil {
		return nil, fmt.Errorf("list nodes by issue: %w", err)
	}
	nodes := make([]GraphNode, len(rows))
	for i, r := range rows {
		nodes[i] = *nodeFromDB(&r)
	}
	return nodes, nil
}

// UpdateNode updates fields on an existing node. Only non-nil fields in params
// are applied; nil fields leave the current value unchanged.
func (s *GraphStore) UpdateNode(ctx context.Context, id string, params *UpdateNodeParams) (*GraphNode, error) {
	nodeID, err := parseUUID(id)
	if err != nil {
		return nil, err
	}

	updateParams := db.UpdateTaskNodeParams{ID: nodeID}

	if params.AgentID != nil {
		agentUUID, err := uuid.Parse(*params.AgentID)
		if err != nil {
			return nil, fmt.Errorf("invalid agent_id: %w", err)
		}
		updateParams.AgentID = uuidToPg(agentUUID)
	}
	if params.Status != nil {
		updateParams.Status = pgtype.Text{String: *params.Status, Valid: true}
	}
	if params.Context != nil {
		updateParams.Context = params.Context
	}
	if params.Result != nil {
		updateParams.Result = params.Result
	}
	if params.PositionX != nil {
		updateParams.PositionX = pgtype.Float8{Float64: *params.PositionX, Valid: true}
	}
	if params.PositionY != nil {
		updateParams.PositionY = pgtype.Float8{Float64: *params.PositionY, Valid: true}
	}

	row, err := s.queries.UpdateTaskNode(ctx, updateParams)
	if err != nil {
		return nil, fmt.Errorf("update node: %w", err)
	}
	return nodeFromDB(&row), nil
}

// GetReadyNodes returns pending nodes whose dependencies are all satisfied.
func (s *GraphStore) GetReadyNodes(ctx context.Context, issueID string) ([]GraphNode, error) {
	issUUID, err := parseUUID(issueID)
	if err != nil {
		return nil, err
	}
	rows, err := s.queries.GetReadyNodes(ctx, issUUID)
	if err != nil {
		return nil, fmt.Errorf("get ready nodes: %w", err)
	}
	nodes := make([]GraphNode, len(rows))
	for i, r := range rows {
		nodes[i] = *nodeFromDB(&r)
	}
	return nodes, nil
}

// CreateEdge inserts a new edge between two nodes.
func (s *GraphStore) CreateEdge(ctx context.Context, from, to string, edgeType EdgeType, metadata []byte) (*GraphEdge, error) {
	fromUUID, err := parseUUID(from)
	if err != nil {
		return nil, fmt.Errorf("invalid from_node_id: %w", err)
	}
	toUUID, err := parseUUID(to)
	if err != nil {
		return nil, fmt.Errorf("invalid to_node_id: %w", err)
	}

	row, err := s.queries.CreateTaskEdge(ctx, db.CreateTaskEdgeParams{
		FromNodeID: fromUUID,
		ToNodeID:   toUUID,
		EdgeType:   string(edgeType),
		Metadata:   safeBytes(metadata),
	})
	if err != nil {
		return nil, fmt.Errorf("create edge: %w", err)
	}
	return edgeFromDB(&row), nil
}

// ListEdgesByIssue returns all edges that involve any node of the given issue.
func (s *GraphStore) ListEdgesByIssue(ctx context.Context, issueID string) ([]GraphEdge, error) {
	issUUID, err := parseUUID(issueID)
	if err != nil {
		return nil, err
	}
	rows, err := s.queries.ListEdgesByIssue(ctx, issUUID)
	if err != nil {
		return nil, fmt.Errorf("list edges by issue: %w", err)
	}
	edges := make([]GraphEdge, len(rows))
	for i, r := range rows {
		edges[i] = *edgeFromDB(&r)
	}
	return edges, nil
}

// DeleteNode removes a node and returns an error if the operation fails.
func (s *GraphStore) DeleteNode(ctx context.Context, id string) error {
	nodeID, err := parseUUID(id)
	if err != nil {
		return err
	}
	_, err = s.queries.DeleteTaskNode(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("delete node: %w", err)
	}
	return nil
}

// DeleteEdge removes an edge and returns an error if the operation fails.
func (s *GraphStore) DeleteEdge(ctx context.Context, id string) error {
	edgeID, err := parseUUID(id)
	if err != nil {
		return err
	}
	_, err = s.queries.DeleteTaskEdge(ctx, edgeID)
	if err != nil {
		return fmt.Errorf("delete edge: %w", err)
	}
	return nil
}

// --- helpers ---

// parseUUID parses a string UUID and returns a pgtype.UUID suitable for sqlc calls.
func parseUUID(s string) (pgtype.UUID, error) {
	u, err := uuid.Parse(s)
	if err != nil {
		return pgtype.UUID{}, fmt.Errorf("invalid uuid %q: %w", s, err)
	}
	return uuidToPg(u), nil
}

// uuidToPg converts a uuid.UUID to pgtype.UUID.
func uuidToPg(u uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: [16]byte(u), Valid: true}
}

// pgToUUIDString converts a pgtype.UUID to its string representation.
// Returns empty string when the UUID is not valid (NULL in DB).
func pgToUUIDString(pg pgtype.UUID) string {
	if !pg.Valid {
		return ""
	}
	return uuid.UUID(pg.Bytes).String()
}

// safeBytes returns an empty byte slice when b is nil so the DB receives
// a non-null value (avoids NULL-handling edge cases for JSON/bytea columns).
func safeBytes(b []byte) []byte {
	if b == nil {
		return []byte("{}")
	}
	return b
}

// unmarshalJSONBytes unmarshals JSON bytes into map[string]any.
// Returns nil when the bytes are empty or invalid.
func unmarshalJSONBytes(b []byte) map[string]any {
	if len(b) == 0 {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil
	}
	return m
}

// nodeFromDB converts a database TaskGraphNode to a domain GraphNode.
func nodeFromDB(n *db.TaskGraphNode) *GraphNode {
	return &GraphNode{
		ID:          pgToUUIDString(n.ID),
		WorkspaceID: pgToUUIDString(n.WorkspaceID),
		IssueID:     pgToUUIDString(n.IssueID),
		AgentID:     pgToUUIDString(n.AgentID),
		NodeType:    NodeType(n.NodeType),
		Status:      NodeStatus(n.Status),
		Context:     unmarshalJSONBytes(n.Context),
		Result:      unmarshalJSONBytes(n.Result),
		PositionX:   n.PositionX.Float64,
		PositionY:   n.PositionY.Float64,
		Depth:       int(n.Depth.Int32),
		CreatedAt:   n.CreatedAt.Time.Format(time.RFC3339),
	}
}

// edgeFromDB converts a database TaskGraphEdge to a domain GraphEdge.
func edgeFromDB(e *db.TaskGraphEdge) *GraphEdge {
	return &GraphEdge{
		ID:         pgToUUIDString(e.ID),
		FromNodeID: pgToUUIDString(e.FromNodeID),
		ToNodeID:   pgToUUIDString(e.ToNodeID),
		EdgeType:   EdgeType(e.EdgeType),
		Metadata:   unmarshalJSONBytes(e.Metadata),
	}
}
