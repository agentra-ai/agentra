package taskgraph

import (
	"context"
	"fmt"
)

// GraphScheduler orchestrates node lifecycle transitions and dependency
// resolution on top of a GraphStore.
type GraphScheduler struct {
	store *GraphStore
}

// NewGraphScheduler creates a new scheduler backed by the given store.
func NewGraphScheduler(store *GraphStore) *GraphScheduler {
	return &GraphScheduler{store: store}
}

// GetReadyNodes returns all pending nodes whose dependencies are satisfied.
func (s *GraphScheduler) GetReadyNodes(ctx context.Context, issueID string) ([]GraphNode, error) {
	return s.store.GetReadyNodes(ctx, issueID)
}

// TransitionNode updates a node's status. It validates the transition is
// a known status before persisting.
func (s *GraphScheduler) TransitionNode(ctx context.Context, nodeID string, toStatus NodeStatus) error {
	statusStr := string(toStatus)
	_, err := s.store.UpdateNode(ctx, nodeID, &UpdateNodeParams{Status: &statusStr})
	if err != nil {
		return fmt.Errorf("transition node %s to %s: %w", nodeID, toStatus, err)
	}
	return nil
}

// IsGraphComplete returns true when every executor and synthesis node in the
// issue's graph has reached status=completed.
func (s *GraphScheduler) IsGraphComplete(ctx context.Context, issueID string) (bool, error) {
	nodes, err := s.store.ListNodesByIssue(ctx, issueID)
	if err != nil {
		return false, fmt.Errorf("list nodes: %w", err)
	}
	for _, n := range nodes {
		if n.NodeType == NodeTypeExecutor || n.NodeType == NodeTypeSynthesis {
			if n.Status != StatusCompleted {
				return false, nil
			}
		}
	}
	return true, nil
}
