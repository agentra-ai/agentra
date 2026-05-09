package taskgraph

import (
	"context"
	"fmt"
)

// HandoffContext is the full context bundle passed to an agent when it claims
// a graph node. It contains everything the agent needs to pick up where prior
// siblings left off.
type HandoffContext struct {
	ParentIssue       map[string]any    `json:"parent_issue"`
	CompletedSiblings []HandoffSibling  `json:"completed_siblings"`
	RelevantMemories  []any             `json:"relevant_memories,omitempty"`
	Artifacts         []HandoffArtifact `json:"artifacts"`
	Instructions      string            `json:"instructions"`
}

// HandoffSibling describes a completed sibling node whose results may be
// relevant for the next agent.
type HandoffSibling struct {
	NodeID    string         `json:"node_id"`
	AgentName string         `json:"agent_name"`
	NodeType  string         `json:"node_type"`
	Result    map[string]any `json:"result"`
}

// HandoffArtifact describes a file or resource artifact produced by a sibling.
type HandoffArtifact struct {
	Type string `json:"type"`
	Path string `json:"path,omitempty"`
	ID   string `json:"id,omitempty"`
}

// HandoffProtocol builds handoff context bundles so agents can receive
// structured information about what came before them in a task graph.
type HandoffProtocol struct {
	store *GraphStore
}

// NewHandoffProtocol creates a HandoffProtocol backed by the given store.
func NewHandoffProtocol(store *GraphStore) *HandoffProtocol {
	return &HandoffProtocol{store: store}
}

// BuildHandoffContext constructs the full context bundle for a node.
// It gathers parent issue info, completed sibling results, and artifacts.
func (h *HandoffProtocol) BuildHandoffContext(ctx context.Context, nodeID string) (*HandoffContext, error) {
	node, err := h.store.GetNode(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("get node: %w", err)
	}

	// Get all siblings in the same issue
	siblings, err := h.store.ListNodesByIssue(ctx, node.IssueID)
	if err != nil {
		return nil, fmt.Errorf("list siblings: %w", err)
	}

	hc := &HandoffContext{
		ParentIssue: map[string]any{
			"issue_id": node.IssueID,
		},
		CompletedSiblings: []HandoffSibling{},
		Artifacts:         []HandoffArtifact{},
	}

	for _, sib := range siblings {
		if sib.ID != nodeID && sib.Status == StatusCompleted {
			hc.CompletedSiblings = append(hc.CompletedSiblings, HandoffSibling{
				NodeID:   sib.ID,
				NodeType: string(sib.NodeType),
				Result:   sib.Result,
			})
		}
	}

	return hc, nil
}
