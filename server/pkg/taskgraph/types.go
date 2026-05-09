package taskgraph

type NodeType string

const (
	NodeTypeRoot      NodeType = "root"
	NodeTypePlanner   NodeType = "planner"
	NodeTypeExecutor  NodeType = "executor"
	NodeTypeSynthesis NodeType = "synthesis"
)

type NodeStatus string

const (
	StatusPending   NodeStatus = "pending"
	StatusRunning   NodeStatus = "running"
	StatusCompleted NodeStatus = "completed"
	StatusFailed    NodeStatus = "failed"
	StatusBlocked   NodeStatus = "blocked"
)

type EdgeType string

const (
	EdgeDependsOn EdgeType = "depends_on"
	EdgeHandoff   EdgeType = "handoff"
	EdgeTriggers  EdgeType = "triggers"
)

type GraphNode struct {
	ID          string         `json:"id"`
	WorkspaceID string         `json:"workspace_id"`
	IssueID     string         `json:"issue_id"`
	AgentID     string         `json:"agent_id,omitempty"`
	NodeType    NodeType       `json:"node_type"`
	Status      NodeStatus     `json:"status"`
	Context     map[string]any `json:"context"`
	Result      map[string]any `json:"result,omitempty"`
	PositionX   float64        `json:"position_x"`
	PositionY   float64        `json:"position_y"`
	Depth       int            `json:"depth"`
	CreatedAt   string         `json:"created_at"`
}

type GraphEdge struct {
	ID         string         `json:"id"`
	FromNodeID string         `json:"from_node_id"`
	ToNodeID   string         `json:"to_node_id"`
	EdgeType   EdgeType       `json:"edge_type"`
	Metadata   map[string]any `json:"metadata"`
}

// UpdateNodeParams holds the fields that can be updated on a node.
// Nil fields are left unchanged.
type UpdateNodeParams struct {
	AgentID   *string
	Status    *string
	Context   []byte
	Result    []byte
	PositionX *float64
	PositionY *float64
}
