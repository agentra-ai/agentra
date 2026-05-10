package service

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/agentra-ai/agentra/server/pkg/db/generated"
)

func TestParsePlannerJSON_Valid(t *testing.T) {
	input := `{
		"plan": "Execute a 3-step greeting pipeline.",
		"nodes": [
			{
				"node_type": "executor",
				"context": {
					"description": "Generate a greeting message",
					"suggested_agent": "greeter-bot",
					"estimated_effort": "low",
					"deliverable": "A greeting string",
					"acceptance_criteria": ["Must be friendly"]
				},
				"depth": 0,
				"dependencies": []
			},
			{
				"node_type": "executor",
				"context": {
					"description": "Translate greeting to Spanish",
					"suggested_agent": "translator",
					"estimated_effort": "low",
					"deliverable": "Spanish greeting",
					"acceptance_criteria": ["Accurate translation"]
				},
				"depth": 1,
				"dependencies": [0]
			},
			{
				"node_type": "synthesis",
				"context": {
					"description": "Deliver final greeting response",
					"suggested_agent": "delivery-bot",
					"estimated_effort": "medium",
					"deliverable": "Final response",
					"acceptance_criteria": ["Contains both greetings", "Properly formatted"]
				},
				"depth": 2,
				"dependencies": [0, 1]
			}
		]
	}`

	result, err := parsePlannerJSON(input)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if result.Plan == "" {
		t.Error("plan should not be empty")
	}
	if len(result.Nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(result.Nodes))
	}

	// Check node 0 (root)
	n0 := result.Nodes[0]
	if n0.NodeType != "executor" {
		t.Errorf("node 0 type: expected executor, got %s", n0.NodeType)
	}
	if n0.Depth != 0 {
		t.Errorf("node 0 depth: expected 0, got %d", n0.Depth)
	}
	if len(n0.Dependencies) != 0 {
		t.Errorf("node 0 dependencies: expected 0, got %d", len(n0.Dependencies))
	}
	if n0.Context.Description != "Generate a greeting message" {
		t.Errorf("node 0 description mismatch")
	}

	// Check node 1
	n1 := result.Nodes[1]
	if n1.NodeType != "executor" {
		t.Errorf("node 1 type: expected executor, got %s", n1.NodeType)
	}
	if len(n1.Dependencies) != 1 || n1.Dependencies[0] != 0 {
		t.Errorf("node 1 dependencies: expected [0], got %v", n1.Dependencies)
	}

	// Check node 2
	n2 := result.Nodes[2]
	if n2.NodeType != "synthesis" {
		t.Errorf("node 2 type: expected synthesis, got %s", n2.NodeType)
	}
	if len(n2.Dependencies) != 2 || n2.Dependencies[0] != 0 || n2.Dependencies[1] != 1 {
		t.Errorf("node 2 dependencies: expected [0, 1], got %v", n2.Dependencies)
	}
}

func TestParsePlannerJSON_MarkdownFences(t *testing.T) {
	input := "```json\n" + `{
		"plan": "Simple test",
		"nodes": [
			{
				"node_type": "executor",
				"context": {
					"description": "Test node",
					"suggested_agent": "",
					"estimated_effort": "low",
					"deliverable": "",
					"acceptance_criteria": []
				},
				"depth": 0,
				"dependencies": []
			}
		]
	}` + "\n```"

	result, err := parsePlannerJSON(input)
	if err != nil {
		t.Fatalf("expected no error with markdown fences, got: %v", err)
	}
	if len(result.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(result.Nodes))
	}
}

func TestParsePlannerJSON_NoContent(t *testing.T) {
	_, err := parsePlannerJSON("just some text, no json")
	if err == nil {
		t.Error("expected error for input with no JSON")
	}
}

func TestParsePlannerJSON_InvalidJSON(t *testing.T) {
	_, err := parsePlannerJSON("{broken: json}")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestValidateDAG_EmptyNodes(t *testing.T) {
	err := validateDAG(&plannerResponse{Nodes: []plannerNode{}}, 10)
	if err == nil {
		t.Error("expected error for empty nodes")
	}
}

func TestValidateDAG_TooManyNodes(t *testing.T) {
	nodes := make([]plannerNode, 5)
	for i := range nodes {
		nodes[i] = plannerNode{
			NodeType: "executor",
			Context:  plannerContext{Description: "node"},
		}
	}
	err := validateDAG(&plannerResponse{Nodes: nodes}, 3)
	if err == nil {
		t.Error("expected error for exceeding max_nodes")
	}
}

func TestValidateDAG_InvalidNodeType(t *testing.T) {
	err := validateDAG(&plannerResponse{
		Nodes: []plannerNode{
			{
				NodeType: "root",
				Context:  plannerContext{Description: "test"},
			},
		},
	}, 10)
	if err == nil {
		t.Error("expected error for invalid node_type 'root'")
	}
}

func TestValidateDAG_DependencyOutOfRange(t *testing.T) {
	err := validateDAG(&plannerResponse{
		Nodes: []plannerNode{
			{
				NodeType:    "executor",
				Context:     plannerContext{Description: "node 0"},
				Dependencies: []int{3},
			},
		},
	}, 10)
	if err == nil {
		t.Error("expected error for out-of-range dependency index")
	}
}

func TestValidateDAG_SelfDependency(t *testing.T) {
	err := validateDAG(&plannerResponse{
		Nodes: []plannerNode{
			{
				NodeType:    "executor",
				Context:     plannerContext{Description: "node 0"},
				Dependencies: []int{0},
			},
		},
	}, 10)
	if err == nil {
		t.Error("expected error for self-dependency")
	}
}

func TestValidateDAG_MissingDescription(t *testing.T) {
	err := validateDAG(&plannerResponse{
		Nodes: []plannerNode{
			{
				NodeType: "executor",
				Context:  plannerContext{Description: ""},
			},
		},
	}, 10)
	if err == nil {
		t.Error("expected error for missing context description")
	}
}

func TestValidateDAG_ValidSynthesisNode(t *testing.T) {
	err := validateDAG(&plannerResponse{
		Nodes: []plannerNode{
			{
				NodeType: "synthesis",
				Context:  plannerContext{Description: "combine results"},
			},
		},
	}, 10)
	if err != nil {
		t.Errorf("unexpected error for valid synthesis node: %v", err)
	}
}

func TestValidateDAG_ValidMultiDepthChain(t *testing.T) {
	err := validateDAG(&plannerResponse{
		Nodes: []plannerNode{
			{
				NodeType: "executor",
				Context:  plannerContext{Description: "node 0"},
				Depth:    0,
			},
			{
				NodeType:    "executor",
				Context:     plannerContext{Description: "node 1"},
				Depth:       1,
				Dependencies: []int{0},
			},
			{
				NodeType:    "synthesis",
				Context:     plannerContext{Description: "node 2"},
				Depth:       2,
				Dependencies: []int{1},
			},
		},
	}, 10)
	if err != nil {
		t.Errorf("unexpected error for valid multi-depth chain: %v", err)
	}
}

// validUUID creates a non-zero pgtype.UUID for test purposes.
func validUUID() pgtype.UUID {
	id := uuid.New()
	return pgtype.UUID{Bytes: [16]byte(id), Valid: true}
}

func TestResolveAgentAssignments_ExactMatch(t *testing.T) {
	agents := []db.Agent{
		{ID: validUUID(), Name: "Greeter Bot"},
		{ID: validUUID(), Name: "Translator"},
	}
	nodes := []plannerNode{
		{Context: plannerContext{SuggestedAgent: "Greeter Bot"}},
		{Context: plannerContext{SuggestedAgent: "Translator"}},
	}
	assignments := resolveAgentAssignments(nodes, agents)
	if len(assignments) != 2 {
		t.Errorf("expected 2 assignments, got %d", len(assignments))
	}
}

func TestResolveAgentAssignments_CaseInsensitive(t *testing.T) {
	agents := []db.Agent{
		{ID: validUUID(), Name: "My Agent"},
	}
	nodes := []plannerNode{
		{Context: plannerContext{SuggestedAgent: "my agent"}},
	}
	assignments := resolveAgentAssignments(nodes, agents)
	if len(assignments) != 1 {
		t.Errorf("expected 1 assignment, got %d", len(assignments))
	}
}

func TestResolveAgentAssignments_SubstringMatch(t *testing.T) {
	agents := []db.Agent{
		{ID: validUUID(), Name: "Dev Claude Agent"},
	}
	nodes := []plannerNode{
		{Context: plannerContext{SuggestedAgent: "Claude"}},
	}
	assignments := resolveAgentAssignments(nodes, agents)
	if len(assignments) != 1 {
		t.Errorf("expected 1 assignment for substring match, got %d", len(assignments))
	}
}

func TestResolveAgentAssignments_NoMatch(t *testing.T) {
	agents := []db.Agent{
		{ID: validUUID(), Name: "Alpha"},
		{ID: validUUID(), Name: "Beta"},
	}
	nodes := []plannerNode{
		{Context: plannerContext{SuggestedAgent: "Gamma"}},
	}
	assignments := resolveAgentAssignments(nodes, agents)
	if len(assignments) != 0 {
		t.Errorf("expected 0 assignments for no match, got %d", len(assignments))
	}
}

func TestResolveAgentAssignments_EmptySuggested(t *testing.T) {
	agents := []db.Agent{
		{ID: validUUID(), Name: "Some Agent"},
	}
	nodes := []plannerNode{
		{Context: plannerContext{SuggestedAgent: ""}},
	}
	assignments := resolveAgentAssignments(nodes, agents)
	if len(assignments) != 0 {
		t.Errorf("expected 0 assignments for empty suggestion, got %d", len(assignments))
	}
}

func TestResolveAgentAssignments_NoAgents(t *testing.T) {
	var agents []db.Agent
	nodes := []plannerNode{
		{Context: plannerContext{SuggestedAgent: "anyone"}},
	}
	assignments := resolveAgentAssignments(nodes, agents)
	if len(assignments) != 0 {
		t.Errorf("expected 0 assignments with no agents, got %d", len(assignments))
	}
}

func TestResolveAgentAssignments_HyphenatedNames(t *testing.T) {
	agents := []db.Agent{
		{ID: validUUID(), Name: "code-reviewer-bot"},
	}
	nodes := []plannerNode{
		{Context: plannerContext{SuggestedAgent: "Code Reviewer"}},
	}
	assignments := resolveAgentAssignments(nodes, agents)
	if len(assignments) != 1 {
		t.Errorf("expected 1 assignment for hyphenated name match, got %d", len(assignments))
	}
}

func TestComputePositionX(t *testing.T) {
	tests := []struct {
		index, total int
		want         float64
	}{
		{0, 1, 0},
		{0, 2, -150},
		{1, 2, 150},
		{0, 3, -300},
		{1, 3, 0},
		{2, 3, 300},
	}

	for _, tt := range tests {
		got := computePositionX(tt.index, tt.total)
		if got != tt.want {
			t.Errorf("computePositionX(%d, %d) = %f, want %f", tt.index, tt.total, got, tt.want)
		}
	}
}

func TestTruncateForError(t *testing.T) {
	short := "hello"
	if truncateForError(short) != short {
		t.Error("short string should not be truncated")
	}

	long := make([]byte, 600)
	for i := range long {
		long[i] = 'a'
	}
	truncated := truncateForError(string(long))
	if len(truncated) != 500 {
		t.Errorf("truncated length %d, expected 500 (including suffix)", len(truncated))
	}
	if !strings.Contains(truncated, "... (truncated)") {
		t.Error("truncated string should contain suffix")
	}
}

func TestResolveAPIKey(t *testing.T) {
	// Just verify we get empty strings for unknown providers and non-empty env vars.
	_ = resolveAPIKey("unknown")
	// This test is primarily about not panicking.
}
