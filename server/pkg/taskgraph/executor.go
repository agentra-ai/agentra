package taskgraph

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
)

func (s *DelegationScheduler) executeParallel(ctx context.Context, nodes []GraphNode) error {
	var wg sync.WaitGroup
	for _, node := range nodes {
		wg.Add(1)
		go func(n GraphNode) {
			defer wg.Done()
			err := s.executeNode(ctx, &n)
			if err != nil {
				slog.Error("parallel execution failed", "node_id", n.ID, "error", err)
			}
		}(node)
	}
	wg.Wait()
	return nil
}

func (s *DelegationScheduler) executeSequential(ctx context.Context, chain []GraphNode) error {
	for _, node := range chain {
		err := s.executeNode(ctx, &node)
		if err != nil {
			// Stop chain on failure
			s.transitionNode(ctx, node.ID, StatusFailed)
			return err
		}
	}
	return nil
}

func (s *DelegationScheduler) executeNode(ctx context.Context, node *GraphNode) error {
	// Build handoff context
	handoffCtx, err := s.buildHandoffContext(ctx, node.ID)
	if err != nil {
		return err
	}

	// Transition to running
	if err := s.transitionNode(ctx, node.ID, StatusRunning); err != nil {
		return err
	}

	// Execute based on node type
	result, err := s.executor.Execute(ctx, node, handoffCtx)
	if err != nil {
		s.transitionNode(ctx, node.ID, StatusFailed)
		return err
	}

	// Store result and transition to completed
	node.Result = result
	resultJSON, _ := json.Marshal(result)
	s.store.UpdateNode(ctx, node.ID, &UpdateNodeParams{Result: resultJSON})
	s.transitionNode(ctx, node.ID, StatusCompleted)

	return nil
}

func (s *DelegationScheduler) buildHandoffContext(ctx context.Context, nodeID string) (*HandoffContext, error) {
	// Use the existing HandoffProtocol
	protocol := NewHandoffProtocol(s.store)
	return protocol.BuildHandoffContext(ctx, nodeID)
}

// transitionNode updates a node's status.
func (s *DelegationScheduler) transitionNode(ctx context.Context, nodeID string, toStatus NodeStatus) error {
	statusStr := string(toStatus)
	_, err := s.store.UpdateNode(ctx, nodeID, &UpdateNodeParams{Status: &statusStr})
	return err
}

// transitionNodeStore is an alias for TransitionNode on GraphStore.
type transitionNodeStore interface {
	UpdateNode(ctx context.Context, id string, params *UpdateNodeParams) (*GraphNode, error)
}
