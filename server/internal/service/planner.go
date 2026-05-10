package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/agentra-ai/agentra/server/internal/util"
	agenttypes "github.com/agentra-ai/agentra/server/pkg/agent/types"
	agentproviders "github.com/agentra-ai/agentra/server/pkg/agent/providers"
	db "github.com/agentra-ai/agentra/server/pkg/db/generated"
	"github.com/agentra-ai/agentra/pkg/taskgraph"
)

// ErrNoDescription is returned when an issue has no description.
var ErrNoDescription = errors.New("issue has no description")

// ErrGraphExists is returned when a task graph already exists for an issue.
var ErrGraphExists = errors.New("task graph already exists for this issue")

// ErrInvalidDAG is returned when the LLM returns an invalid DAG structure.
var ErrInvalidDAG = errors.New("invalid DAG structure returned by planner")

// PlannerService decomposes goal issues into task DAGs via LLM.
type PlannerService struct {
	queries    *db.Queries
	graphStore *taskgraph.GraphStore
}

// NewPlannerService creates a new PlannerService.
func NewPlannerService(q *db.Queries, gs *taskgraph.GraphStore) *PlannerService {
	return &PlannerService{queries: q, graphStore: gs}
}

// DecomposeOptions configures the auto-decomposition request.
type DecomposeOptions struct {
	Provider          string
	Model             string
	MaxNodes          int
	AdditionalContext string
	Force             bool
}

// defaultDecomposeOpts fills defaults for unset options.
func defaultDecomposeOpts(opts DecomposeOptions) DecomposeOptions {
	if opts.Provider == "" {
		opts.Provider = "anthropic"
	}
	if opts.Model == "" {
		opts.Model = "claude-sonnet-4-20250514"
	}
	if opts.MaxNodes <= 0 {
		opts.MaxNodes = 10
	}
	return opts
}

// DecomposeResult holds the full decomposition output.
type DecomposeResult struct {
	Nodes      []taskgraph.GraphNode `json:"nodes"`
	Edges      []taskgraph.GraphEdge `json:"edges"`
	Plan       string                `json:"plan"`
	TokenUsage map[string]any        `json:"token_usage,omitempty"`
}

// plannerNode is the raw node from the LLM JSON response.
type plannerNode struct {
	NodeType     string         `json:"node_type"`
	Context      plannerContext `json:"context"`
	Depth        int            `json:"depth"`
	Dependencies []int          `json:"dependencies"`
}

type plannerContext struct {
	Description        string   `json:"description"`
	SuggestedAgent     string   `json:"suggested_agent"`
	EstimatedEffort    string   `json:"estimated_effort"`
	Deliverable        string   `json:"deliverable"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
}

// plannerResponse is the top-level JSON the LLM returns.
type plannerResponse struct {
	Plan  string        `json:"plan"`
	Nodes []plannerNode `json:"nodes"`
}

// DecomposeIssue decomposes a goal issue into a task DAG.
func (s *PlannerService) DecomposeIssue(ctx context.Context, workspaceID, issueID string, opts DecomposeOptions) (*DecomposeResult, error) {
	opts = defaultDecomposeOpts(opts)

	// 1. Load the issue.
	issue, err := s.queries.GetIssue(ctx, util.ParseUUID(issueID))
	if err != nil {
		return nil, fmt.Errorf("load issue: %w", err)
	}
	if !issue.Description.Valid || strings.TrimSpace(issue.Description.String) == "" {
		return nil, ErrNoDescription
	}

	// 2. Check if graph already exists.
	if !opts.Force {
		existing, err := s.graphStore.ListNodesByIssue(ctx, issueID)
		if err != nil {
			return nil, fmt.Errorf("check existing graph: %w", err)
		}
		if len(existing) > 0 {
			return nil, ErrGraphExists
		}
	}

	// 3. Load available workspace agents.
	agents, err := s.queries.ListAgents(ctx, util.ParseUUID(workspaceID))
	if err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}

	// 4. Build prompts.
	agentList := formatAgentList(agents)
	systemPrompt := buildSystemPrompt(opts.MaxNodes)
	userPrompt := buildUserPrompt(issue.Title, issue.Description.String, agentList, opts.AdditionalContext)

	// 5. Call the LLM provider.
	apiKey := resolveAPIKey(opts.Provider)
	provider, err := agentproviders.NewProvider(opts.Provider, agentproviders.APIConfig{
		APIKey:      apiKey,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return nil, fmt.Errorf("create provider: %w", err)
	}

	execOpts := agentproviders.ExecOptions{
		Model:        opts.Model,
		SystemPrompt: systemPrompt,
		MaxTurns:     1,
		Timeout:      60 * time.Second,
	}

	session, err := provider.Execute(ctx, userPrompt, execOpts)
	if err != nil {
		return nil, fmt.Errorf("execute planner: %w", err)
	}

	// Wait for result with context cancellation.
	var llmResult agenttypes.Result
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("planner execution cancelled: %w", ctx.Err())
	case res, ok := <-session.Result:
		if !ok {
			return nil, fmt.Errorf("planner session closed unexpectedly")
		}
		llmResult = res
	}

	if llmResult.Status != "completed" {
		truncated := truncateForError(llmResult.Error)
		return nil, fmt.Errorf("planner LLM failed (status=%s): %s", llmResult.Status, truncated)
	}

	// 6. Parse the JSON response.
	plan, err := parsePlannerJSON(llmResult.Output)
	if err != nil {
		truncated := truncateForError(llmResult.Output)
		return nil, fmt.Errorf("%w: %v (raw output: %s)", ErrInvalidDAG, err, truncated)
	}

	// 7. Validate DAG structure.
	if err := validateDAG(plan, opts.MaxNodes); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidDAG, err)
	}

	// 8. Resolve agent names to agent IDs.
	assignments := resolveAgentAssignments(plan.Nodes, agents)

	// 9. Create nodes in the graph store.
	nodes, err := s.createDAGNodes(ctx, workspaceID, issueID, plan, assignments)
	if err != nil {
		return nil, fmt.Errorf("create nodes: %w", err)
	}

	// 10. Create edges.
	edges, err := s.createDAGEdges(ctx, plan, nodes)
	if err != nil {
		return nil, fmt.Errorf("create edges: %w", err)
	}

	tokenUsage := make(map[string]any)
	if llmResult.TokenUsage != nil {
		tokenUsage["input_tokens"] = llmResult.TokenUsage.InputTokens
		tokenUsage["output_tokens"] = llmResult.TokenUsage.OutputTokens
	}

	return &DecomposeResult{
		Nodes:      nodes,
		Edges:      edges,
		Plan:       plan.Plan,
		TokenUsage: tokenUsage,
	}, nil
}

// createDAGNodes creates all nodes in the graph store, assigns agent IDs, and computes layout.
func (s *PlannerService) createDAGNodes(ctx context.Context, workspaceID, issueID string, plan *plannerResponse, assignments map[int]string) ([]taskgraph.GraphNode, error) {
	// Group nodes by depth for layout.
	nodesByDepth := make(map[int]int)
	maxDepth := 0
	for i := range plan.Nodes {
		d := plan.Nodes[i].Depth
		nodesByDepth[d]++
		if d > maxDepth {
			maxDepth = d
		}
	}

	// Compute per-depth index for position_x.
	perDepthIndex := make(map[int]int)

	created := make([]taskgraph.GraphNode, len(plan.Nodes))
	for i, pn := range plan.Nodes {
		d := pn.Depth

		// Compute position.
		depthCount := nodesByDepth[d]
		idx := perDepthIndex[d]
		posX := computePositionX(idx, depthCount)
		posY := float64(d * 150)
		perDepthIndex[d]++

		// Build context JSON.
		ctxMap := map[string]any{
			"description":         pn.Context.Description,
			"suggested_agent":     pn.Context.SuggestedAgent,
			"estimated_effort":    pn.Context.EstimatedEffort,
			"deliverable":         pn.Context.Deliverable,
			"acceptance_criteria": pn.Context.AcceptanceCriteria,
		}
		contextJSON, err := json.Marshal(ctxMap)
		if err != nil {
			return nil, fmt.Errorf("marshal context for node %d: %w", i, err)
		}

		nodeType := taskgraph.NodeType(pn.NodeType)

		node, err := s.graphStore.CreateNode(ctx, workspaceID, issueID, nodeType, d, contextJSON)
		if err != nil {
			return nil, fmt.Errorf("create node %d: %w", i, err)
		}

		// Assign agent and position via update.
		var agentIDStr *string
		if agentID, ok := assignments[i]; ok {
			agentIDStr = &agentID
		}

		updated, err := s.graphStore.UpdateNode(ctx, node.ID, &taskgraph.UpdateNodeParams{
			AgentID:   agentIDStr,
			PositionX: &posX,
			PositionY: &posY,
		})
		if err != nil {
			return nil, fmt.Errorf("update node %d: %w", i, err)
		}
		created[i] = *updated
	}
	return created, nil
}

// createDAGEdges creates dependency edges for all nodes.
func (s *PlannerService) createDAGEdges(ctx context.Context, plan *plannerResponse, nodes []taskgraph.GraphNode) ([]taskgraph.GraphEdge, error) {
	var edges []taskgraph.GraphEdge
	for i, pn := range plan.Nodes {
		for _, depIdx := range pn.Dependencies {
			toNode := nodes[i]
			fromNode := nodes[depIdx]

			edge, err := s.graphStore.CreateEdge(ctx, fromNode.ID, toNode.ID, taskgraph.EdgeDependsOn, nil)
			if err != nil {
				return nil, fmt.Errorf("create edge %d->%d: %w", depIdx, i, err)
			}
			edges = append(edges, *edge)
		}
	}
	return edges, nil
}

// computePositionX computes the x position for a node within its depth layer.
// Nodes are centered around 0 with 300px spacing.
func computePositionX(index, total int) float64 {
	center := float64(total-1) / 2.0
	return (float64(index) - center) * 300.0
}

// systemPromptTemplate is the planner's system prompt.
var systemPromptTemplate = `You are a task decomposition planner for Agentra, an AI-native task management platform.

Your job: Given a goal (issue title + description) and a team of available agents, produce a Task DAG that decomposes the goal into executable subtasks with dependencies.

## Rules
1. Each node must be a concrete, independently executable task.
2. Edges represent "depends_on" relationships. Node B depends on Node A if B cannot start until A completes.
3. Nodes with no mutual dependencies should be at the same depth for parallel execution.
4. Assign each node a node_type: "executor" (work task) or "synthesis" (combines results).
5. Suggest an agent from the available team for each node.
6. Prefer parallelism over long chains. Minimize critical path length.
7. Maximum nodes: %d.

## Output Format
Respond with ONLY a single JSON object (no markdown wrappers, no code fences):
{
  "plan": "markdown execution plan summary",
  "nodes": [
    {
      "node_type": "executor|synthesis",
      "context": {
        "description": "what this node does",
        "suggested_agent": "name of recommended agent",
        "estimated_effort": "low|medium|high",
        "deliverable": "concrete output",
        "acceptance_criteria": ["criterion"]
      },
      "depth": 0,
      "dependencies": []
    }
  ]
}

dependencies array contains 0-based indices of upstream nodes. First node = index 0. Nodes at depth 0 have empty dependencies.`

func buildSystemPrompt(maxNodes int) string {
	return fmt.Sprintf(systemPromptTemplate, maxNodes)
}

// userPromptTemplate is the user prompt for the planner.
var userPromptTemplate = `## Goal
**Title**: %s
**Description**: %s

## Available Agents
%s

## Additional Context
%s

Decompose this goal into a task DAG. Return the JSON.`

func buildUserPrompt(title, description, agentList, additionalContext string) string {
	if additionalContext == "" {
		additionalContext = "(none)"
	}
	return fmt.Sprintf(userPromptTemplate, title, description, agentList, additionalContext)
}

// formatAgentList formats agents for the LLM prompt.
func formatAgentList(agents []db.Agent) string {
	if len(agents) == 0 {
		return "(no agents available)"
	}
	var b strings.Builder
	for i, a := range agents {
		fmt.Fprintf(&b, "%d. **%s**", i+1, a.Name)
		if a.Description != "" {
			fmt.Fprintf(&b, " - %s", a.Description)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// parsePlannerJSON extracts a plannerResponse from LLM output, stripping markdown fences.
func parsePlannerJSON(raw string) (*plannerResponse, error) {
	cleaned := strings.TrimSpace(raw)

	// Strip markdown code fences if present.
	if strings.HasPrefix(cleaned, "```") {
		// Find the opening fence end.
		newline := strings.Index(cleaned, "\n")
		if newline >= 0 {
			cleaned = cleaned[newline+1:]
		}
		// Strip closing fence.
		if strings.HasSuffix(cleaned, "```") {
			cleaned = cleaned[:len(cleaned)-3]
		}
		cleaned = strings.TrimSpace(cleaned)
	}

	// Try to find JSON within the string (handles stray markdown or prose).
	start := strings.Index(cleaned, "{")
	if start < 0 {
		return nil, errors.New("no JSON object found in response")
	}
	cleaned = cleaned[start:]

	// Find matching closing brace.
	end := strings.LastIndex(cleaned, "}")
	if end < 0 {
		return nil, errors.New("no closing brace found in response")
	}
	cleaned = cleaned[:end+1]

	var resp plannerResponse
	if err := json.Unmarshal([]byte(cleaned), &resp); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}
	return &resp, nil
}

// validateDAG validates the parsed DAG structure.
func validateDAG(plan *plannerResponse, maxNodes int) error {
	if len(plan.Nodes) == 0 {
		return errors.New("at least 1 node is required")
	}
	if len(plan.Nodes) > maxNodes {
		return fmt.Errorf("node count %d exceeds maximum %d", len(plan.Nodes), maxNodes)
	}

	totalNodes := len(plan.Nodes)
	for i, node := range plan.Nodes {
		// Validate node_type.
		switch node.NodeType {
		case "executor", "synthesis":
			// Valid.
		default:
			return fmt.Errorf("node %d: invalid node_type %q (must be executor or synthesis)", i, node.NodeType)
		}

		// Validate dependencies.
		for _, depIdx := range node.Dependencies {
			if depIdx < 0 || depIdx >= totalNodes {
				return fmt.Errorf("node %d: dependency index %d out of range (0-%d)", i, depIdx, totalNodes-1)
			}
			if depIdx == i {
				return fmt.Errorf("node %d: cannot depend on itself", i)
			}
		}

		// Validate context has at least a description.
		if strings.TrimSpace(node.Context.Description) == "" {
			return fmt.Errorf("node %d: context.description is required", i)
		}
	}
	return nil
}

// resolveAgentAssignments matches suggested agent names to real agent IDs.
// Uses case-insensitive word-level substring matching.
func resolveAgentAssignments(nodes []plannerNode, agents []db.Agent) map[int]string {
	assignments := make(map[int]string)

	type candidate struct {
		idx     int
		name    string
		agentID string
	}

	var candidates []candidate
	for i, node := range nodes {
		suggested := strings.TrimSpace(node.Context.SuggestedAgent)
		if suggested == "" {
			continue
		}
		candidates = append(candidates, candidate{idx: i, name: suggested})
	}

	if len(candidates) == 0 || len(agents) == 0 {
		return assignments
	}

	// Precompute lowercase agent names.
	type agentEntry struct {
		id          string
		name        string
		nameLower   string
		wordsLower  []string // split by spaces, hyphens, underscores
	}
	agentEntries := make([]agentEntry, len(agents))
	for i, a := range agents {
		nameLower := strings.ToLower(a.Name)
		agentEntries[i] = agentEntry{
			id:        util.UUIDToString(a.ID),
			name:      a.Name,
			nameLower: nameLower,
			wordsLower: splitNameWords(nameLower),
		}
	}

	for _, c := range candidates {
		suggestedLower := strings.ToLower(c.name)
		suggestedWords := splitNameWords(suggestedLower)

		bestID := ""
		bestScore := 0

		for _, ae := range agentEntries {
			score := matchScore(suggestedLower, suggestedWords, ae.nameLower, ae.wordsLower)
			if score > bestScore {
				bestScore = score
				bestID = ae.id
			}
		}

		if bestID != "" {
			assignments[c.idx] = bestID
		}
	}

	return assignments
}

// splitNameWords splits a name into words by common delimiters.
func splitNameWords(name string) []string {
	// Use FieldsFunc for space and common separators.
	delim := func(c rune) bool {
		return c == ' ' || c == '-' || c == '_' || c == '.'
	}
	return strings.FieldsFunc(name, delim)
}

// matchScore computes how well a suggested name matches an agent name.
// Higher score = better match.
func matchScore(suggestedLower string, suggestedWords []string, agentLower string, agentWords []string) int {
	// Exact match gets highest score.
	if suggestedLower == agentLower {
		return 1000
	}

	score := 0

	// Full suggested name is contained in agent name or vice versa.
	if strings.Contains(agentLower, suggestedLower) {
		score += 200
	} else if strings.Contains(suggestedLower, agentLower) {
		score += 200
	}

	// Word-level matching.
	for _, sw := range suggestedWords {
		if len(sw) < 2 {
			continue // skip short noise words
		}
		for _, aw := range agentWords {
			if sw == aw {
				score += 50
			} else if strings.Contains(aw, sw) {
				score += 30
			} else if strings.Contains(sw, aw) {
				score += 20
			}
		}
	}

	return score
}

// resolveAPIKey returns the API key for a provider from environment variables.
func resolveAPIKey(provider string) string {
	switch provider {
	case "anthropic":
		return os.Getenv("ANTHROPIC_API_KEY")
	case "openai":
		return os.Getenv("OPENAI_API_KEY")
	case "openrouter":
		return os.Getenv("OPENROUTER_API_KEY")
	case "ollama":
		return "" // ollama is local, no key needed
	default:
		return ""
	}
}

// truncateForError truncates a string for error messages (500 char max).
func truncateForError(s string) string {
	const maxLen = 500
	if len(s) <= maxLen {
		return s
	}
	suffix := "... (truncated)"
	return s[:maxLen-len(suffix)] + suffix
}
