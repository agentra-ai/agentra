package taskgraph

import (
	"context"
	"sync"
)

// Executor executes a graph node and returns its result.
type Executor struct {
	store *GraphStore
}

// NewExecutor creates a new Executor.
func NewExecutor(store *GraphStore) *Executor {
	return &Executor{store: store}
}

// Execute runs a node and returns its result map.
func (e *Executor) Execute(ctx context.Context, node *GraphNode, handoff *HandoffContext) (map[string]any, error) {
	// TODO: implement actual execution (call agent backend, etc.)
	result := map[string]any{
		"node_id": node.ID,
		"status":  "executed",
	}
	return result, nil
}

// DelegationScheduler determines execution strategy for task graph nodes.
type DelegationScheduler struct {
	store     *GraphStore
	executor  *Executor
	container *ContainerManager
}

// NewDelegationScheduler creates a new DelegationScheduler.
func NewDelegationScheduler(store *GraphStore, executor *Executor, container *ContainerManager) *DelegationScheduler {
	return &DelegationScheduler{
		store:     store,
		executor:  executor,
		container: container,
	}
}

// Schedule determines execution strategy for all ready nodes in an issue.
func (s *DelegationScheduler) Schedule(ctx context.Context, issueID string) error {
	readyNodes, err := s.store.GetReadyNodes(ctx, issueID)
	if err != nil {
		return err
	}

	// Classify nodes by dependency
	parallelNodes, sequentialChains := classifyByDependency(readyNodes)

	var wg sync.WaitGroup

	// Execute parallel nodes concurrently
	if len(parallelNodes) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.executeParallel(ctx, parallelNodes)
		}()
	}

	// Execute sequential chains
	for _, chain := range sequentialChains {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.executeSequential(ctx, chain)
		}()
	}

	wg.Wait()
	return nil
}

func classifyByDependency(nodes []GraphNode) ([]GraphNode, [][]GraphNode) {
	parallel := []GraphNode{}
	sequential := [][]GraphNode{}

	for _, node := range nodes {
		if hasBlockingDependencies(node) {
			// Add to sequential chain
			found := false
			for _, chain := range sequential {
				if canJoinChain(node, chain) {
					sequential = append(sequential, append(chain, node))
					found = true
					break
				}
			}
			if !found {
				sequential = append(sequential, []GraphNode{node})
			}
		} else {
			parallel = append(parallel, node)
		}
	}

	return parallel, sequential
}

func hasBlockingDependencies(node GraphNode) bool {
	// Check if any parent nodes are not completed
	// This would use store.ListEdgesByIssue to find incoming edges
	return false
}

func canJoinChain(node GraphNode, chain []GraphNode) bool {
	// Check if node depends on any node in the chain
	return false
}
