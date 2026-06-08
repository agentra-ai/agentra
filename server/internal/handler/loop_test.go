package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	looppkg "github.com/agentra-ai/agentra/server/internal/loop"
)

// createLoopTestAgent returns the id of a seeded agent in the test workspace.
// agent_id is now a required field for CreateLoop, so any test that needs
// a happy-path loop must supply one.
func createLoopTestAgent(t *testing.T) string {
	t.Helper()
	w := httptest.NewRecorder()
	req := newRequest("GET", "/api/agents?workspace_id="+testWorkspaceID, nil)
	testHandler.ListAgents(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("setup: ListAgents: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var agents []AgentResponse
	if err := json.NewDecoder(w.Body).Decode(&agents); err != nil {
		t.Fatalf("decode agents: %v", err)
	}
	if len(agents) == 0 {
		t.Fatal("setup: expected at least 1 seeded agent")
	}
	return agents[0].ID
}

// createLoopTestIssue creates an issue in the test workspace and returns its
// id. The issue is cleaned up when the test ends.
func createLoopTestIssue(t *testing.T, title string) string {
	t.Helper()
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
		"title": title,
	})
	testHandler.CreateIssue(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateIssue: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var issue IssueResponse
	if err := json.NewDecoder(w.Body).Decode(&issue); err != nil {
		t.Fatalf("decode issue: %v", err)
	}
	t.Cleanup(func() {
		w := httptest.NewRecorder()
		req := newRequest("DELETE", "/api/issues/"+issue.ID, nil)
		req = withURLParam(req, "id", issue.ID)
		testHandler.DeleteIssue(w, req)
	})
	return issue.ID
}

// createLoopTestWorkspace creates a second workspace with the test user as
// owner, returns its id, and cleans it up when the test ends. Used to verify
// cross-tenant isolation.
func createLoopTestWorkspace(t *testing.T) string {
	t.Helper()
	wsID := uuid.NewString()
	slug := "loop-isolation-" + wsID[:8]
	_, err := testPool.Exec(context.Background(),
		`INSERT INTO workspace (id, name, slug, issue_prefix) VALUES ($1, $2, $3, $4)`,
		wsID, "Loop Isolation "+wsID[:8], slug, "ISO")
	if err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	_, err = testPool.Exec(context.Background(),
		`INSERT INTO member (workspace_id, user_id, role) VALUES ($1, $2, 'owner')`,
		wsID, testUserID)
	if err != nil {
		t.Fatalf("seed member: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM workspace WHERE id = $1`, wsID)
	})
	return wsID
}

// decodeLoop decodes a loop response into a generic map for field-by-field
// inspection in tests.
func decodeLoop(t *testing.T, body *bytes.Buffer) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.NewDecoder(body).Decode(&out); err != nil {
		t.Fatalf("decode loop: %v", err)
	}
	return out
}

func TestCreateLoop_HappyPath(t *testing.T) {
	issueID := createLoopTestIssue(t, "loop happy path")
	agentID := createLoopTestAgent(t)

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/loops?workspace_id="+testWorkspaceID, map[string]any{
		"issue_id":       issueID,
		"max_iterations": 7,
		"agent_id":       agentID,
	})
	testHandler.CreateLoop(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateLoop: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	loop := decodeLoop(t, w.Body)
	if loop["status"] != "pending" {
		t.Errorf("expected status=pending, got %v", loop["status"])
	}
	if int(loop["max_iterations"].(float64)) != 7 {
		t.Errorf("expected max_iterations=7, got %v", loop["max_iterations"])
	}
	if loop["issue_id"] != issueID {
		t.Errorf("expected issue_id=%s, got %v", issueID, loop["issue_id"])
	}
	if loop["id"] == nil || loop["id"] == "" {
		t.Error("expected non-empty loop id")
	}
}

// TestCreateLoop_StartsLoopWhenCoordinatorWired verifies the production
// handler-to-coordinator wiring: when the Handler has a LoopCoordinator
// attached, CreateLoop must call StartLoop, transitioning the loop from
// pending to running and stamping started_at. This is the fix for the gap
// where freshly created loops sat in 'pending' status forever because the
// event-driven state machine has no preceding task to fire on for the
// plan stage.
func TestCreateLoop_StartsLoopWhenCoordinatorWired(t *testing.T) {
	// Build a Handler that mirrors the global testHandler but with a real
	// Coordinator wired. Reusing testHandler is not safe: it is a shared
	// singleton, and writing started_at on a loop created from one test
	// would race with reads from the rest of the suite.
	wiredHandler := *testHandler
	coord := looppkg.NewCoordinator(testHandler.Queries, testHandler.Bus)
	wiredHandler.SetLoopCoordinator(coord)

	// Look up the seeded "Handler Test Agent" — StartLoop enqueues a
	// loop_plan task that requires a non-null agent_id (NOT NULL FK to
	// agent_task_queue.agent_id).
	listW := httptest.NewRecorder()
	listReq := newRequest("GET", "/api/agents?workspace_id="+testWorkspaceID, nil)
	testHandler.ListAgents(listW, listReq)
	if listW.Code != http.StatusOK {
		t.Fatalf("setup: ListAgents: expected 200, got %d: %s", listW.Code, listW.Body.String())
	}
	var seededAgents []AgentResponse
	if err := json.NewDecoder(listW.Body).Decode(&seededAgents); err != nil {
		t.Fatalf("decode agents: %v", err)
	}
	if len(seededAgents) == 0 {
		t.Fatal("setup: expected at least 1 seeded agent")
	}
	agentID := seededAgents[0].ID

	issueID := createLoopTestIssue(t, "loop auto-start")

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/loops?workspace_id="+testWorkspaceID, map[string]any{
		"issue_id": issueID,
		"agent_id": agentID,
	})
	wiredHandler.CreateLoop(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateLoop: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	loop := decodeLoop(t, w.Body)
	if loop["status"] != "running" {
		t.Errorf("expected status=running after StartLoop, got %v", loop["status"])
	}
	if loop["current_stage"] != "plan" {
		t.Errorf("expected current_stage=plan after StartLoop, got %v", loop["current_stage"])
	}
	if loop["started_at"] == nil || loop["started_at"] == "" {
		t.Error("expected started_at set after StartLoop")
	}
}

func TestCreateLoop_MissingIssueID(t *testing.T) {
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/loops?workspace_id="+testWorkspaceID, map[string]any{})
	testHandler.CreateLoop(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("CreateLoop (missing issue_id): expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// TestCreateLoop_RequiresAgentID verifies that CreateLoop rejects requests
// missing agent_id. The agent_task_queue.runtime_id column is NOT NULL, so
// a loop created without an agent would be unable to enqueue its first
// plan-stage task and would be stuck in 'pending' forever. The handler must
// fail fast with a clear 400.
func TestCreateLoop_RequiresAgentID(t *testing.T) {
	issueID := createLoopTestIssue(t, "loop missing agent")

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/loops?workspace_id="+testWorkspaceID, map[string]any{
		"issue_id": issueID,
	})
	testHandler.CreateLoop(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("CreateLoop (missing agent_id): expected 400, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "agent_id") {
		t.Errorf("expected error body to mention agent_id, got: %s", w.Body.String())
	}
}

func TestCreateLoop_MissingWorkspace(t *testing.T) {
	issueID := createLoopTestIssue(t, "loop no workspace")

	w := httptest.NewRecorder()
	body := map[string]any{"issue_id": issueID}
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(body)
	req := httptest.NewRequest("POST", "/api/loops", &buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", testUserID)
	testHandler.CreateLoop(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("CreateLoop (missing workspace): expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetLoop_HappyPath(t *testing.T) {
	issueID := createLoopTestIssue(t, "loop get happy")
	agentID := createLoopTestAgent(t)

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/loops?workspace_id="+testWorkspaceID, map[string]any{
		"issue_id": issueID,
		"agent_id": agentID,
	})
	testHandler.CreateLoop(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("setup: CreateLoop: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	created := decodeLoop(t, w.Body)
	loopID := created["id"].(string)

	w = httptest.NewRecorder()
	req = newRequest("GET", "/api/loops/"+loopID, nil)
	req = withURLParam(req, "id", loopID)
	testHandler.GetLoop(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GetLoop: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	got := decodeLoop(t, w.Body)
	if got["id"] != loopID {
		t.Errorf("expected id=%s, got %v", loopID, got["id"])
	}
	if got["status"] != "pending" {
		t.Errorf("expected status=pending, got %v", got["status"])
	}
}

func TestGetLoop_NotFound(t *testing.T) {
	bogus := uuid.NewString()
	w := httptest.NewRecorder()
	req := newRequest("GET", "/api/loops/"+bogus, nil)
	req = withURLParam(req, "id", bogus)
	testHandler.GetLoop(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("GetLoop: expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetLoop_WrongWorkspace(t *testing.T) {
	issueID := createLoopTestIssue(t, "loop cross ws")
	otherWS := createLoopTestWorkspace(t)

	// Create a loop in the other workspace via direct SQL (bypassing the
	// handler's workspace check, which would 404 a request from the caller).
	loopID := uuid.NewString()
	_, err := testPool.Exec(context.Background(),
		`INSERT INTO loops (id, issue_id, workspace_id) VALUES ($1, $2, $3)`,
		loopID, issueID, otherWS)
	if err != nil {
		t.Fatalf("seed loop: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM loops WHERE id = $1`, loopID)
	})

	w := httptest.NewRecorder()
	req := newRequest("GET", "/api/loops/"+loopID, nil)
	req = withURLParam(req, "id", loopID)
	testHandler.GetLoop(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("GetLoop (cross-ws): expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListLoops_OnlyOwnWorkspace(t *testing.T) {
	issueA := createLoopTestIssue(t, "loop list A")
	agentID := createLoopTestAgent(t)
	otherWS := createLoopTestWorkspace(t)

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/loops?workspace_id="+testWorkspaceID, map[string]any{
		"issue_id": issueA,
		"agent_id": agentID,
	})
	testHandler.CreateLoop(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("setup: CreateLoop (A): expected 201, got %d", w.Code)
	}

	otherLoopID := uuid.NewString()
	_, err := testPool.Exec(context.Background(),
		`INSERT INTO loops (id, issue_id, workspace_id) VALUES ($1, $2, $3)`,
		otherLoopID, issueA, otherWS)
	if err != nil {
		t.Fatalf("seed loop: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM loops WHERE id = $1`, otherLoopID)
	})

	w = httptest.NewRecorder()
	req = newRequest("GET", "/api/loops?workspace_id="+testWorkspaceID, nil)
	testHandler.ListLoops(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ListLoops: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var listResp struct {
		Loops []map[string]any `json:"loops"`
		Total int              `json:"total"`
	}
	if err := json.NewDecoder(w.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	for _, l := range listResp.Loops {
		if l["id"] == otherLoopID {
			t.Errorf("ListLoops leaked loop from other workspace: %v", l)
		}
	}
}

func TestPauseLoop_UpdatesStatus(t *testing.T) {
	issueID := createLoopTestIssue(t, "loop pause")
	agentID := createLoopTestAgent(t)

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/loops?workspace_id="+testWorkspaceID, map[string]any{
		"issue_id": issueID,
		"agent_id": agentID,
	})
	testHandler.CreateLoop(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("setup: CreateLoop: expected 201, got %d", w.Code)
	}
	loopID := decodeLoop(t, w.Body)["id"].(string)

	// Move it to running so pause is a valid state transition.
	running := looppkg.StatusRunning
	if _, err := testHandler.LoopStore.UpdateStatus(context.Background(), loopID, looppkg.UpdateStatusInput{
		Status: &running,
	}); err != nil {
		t.Fatalf("setup: UpdateStatus: %v", err)
	}

	w = httptest.NewRecorder()
	req = newRequest("POST", "/api/loops/"+loopID+"/pause", nil)
	req = withURLParam(req, "id", loopID)
	testHandler.PauseLoop(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("PauseLoop: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	got := decodeLoop(t, w.Body)
	if got["status"] != "paused" {
		t.Errorf("expected status=paused, got %v", got["status"])
	}
}

func TestResumeLoop_UpdatesStatus(t *testing.T) {
	issueID := createLoopTestIssue(t, "loop resume")
	agentID := createLoopTestAgent(t)

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/loops?workspace_id="+testWorkspaceID, map[string]any{
		"issue_id": issueID,
		"agent_id": agentID,
	})
	testHandler.CreateLoop(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("setup: CreateLoop: expected 201, got %d", w.Code)
	}
	loopID := decodeLoop(t, w.Body)["id"].(string)

	// Move it to paused first so resume is a valid transition.
	paused := looppkg.StatusPaused
	if _, err := testHandler.LoopStore.UpdateStatus(context.Background(), loopID, looppkg.UpdateStatusInput{
		Status: &paused,
	}); err != nil {
		t.Fatalf("setup: UpdateStatus: %v", err)
	}

	w = httptest.NewRecorder()
	req = newRequest("POST", "/api/loops/"+loopID+"/resume", nil)
	req = withURLParam(req, "id", loopID)
	testHandler.ResumeLoop(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ResumeLoop: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	got := decodeLoop(t, w.Body)
	if got["status"] != "running" {
		t.Errorf("expected status=running, got %v", got["status"])
	}
}

func TestTransitionLoop_PreservesIterationAndStage(t *testing.T) {
	issueID := createLoopTestIssue(t, "loop preserve iteration")
	agentID := createLoopTestAgent(t)

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/loops?workspace_id="+testWorkspaceID, map[string]any{
		"issue_id": issueID,
		"agent_id": agentID,
	})
	testHandler.CreateLoop(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("setup: CreateLoop: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	loopID := decodeLoop(t, w.Body)["id"].(string)

	// Move to running and set iteration to 3 via direct DB update to simulate
	// a loop that has been through multiple coordinator cycles.
	running := looppkg.StatusRunning
	if _, err := testHandler.LoopStore.UpdateStatus(context.Background(), loopID, looppkg.UpdateStatusInput{
		Status:    &running,
		Iteration: intPtr(3),
	}); err != nil {
		t.Fatalf("setup: UpdateStatus to running: %v", err)
	}
	stage := looppkg.StageDevelop
	if _, err := testHandler.LoopStore.UpdateStatus(context.Background(), loopID, looppkg.UpdateStatusInput{
		Status:       &running,
		Iteration:    intPtr(3),
		CurrentStage: &stage,
	}); err != nil {
		t.Fatalf("setup: set current_stage: %v", err)
	}

	// Pause the loop — must preserve iteration and current_stage.
	w = httptest.NewRecorder()
	req = newRequest("POST", "/api/loops/"+loopID+"/pause", nil)
	req = withURLParam(req, "id", loopID)
	testHandler.PauseLoop(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("PauseLoop: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	paused := decodeLoop(t, w.Body)
	if paused["status"] != "paused" {
		t.Errorf("expected status=paused, got %v", paused["status"])
	}
	if int(paused["iteration"].(float64)) != 3 {
		t.Errorf("PauseLoop: expected iteration=3, got %v", paused["iteration"])
	}
	if paused["current_stage"] != "develop" {
		t.Errorf("PauseLoop: expected current_stage=develop, got %v", paused["current_stage"])
	}

	// Resume the loop — must still preserve iteration and current_stage.
	w = httptest.NewRecorder()
	req = newRequest("POST", "/api/loops/"+loopID+"/resume", nil)
	req = withURLParam(req, "id", loopID)
	testHandler.ResumeLoop(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ResumeLoop: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resumed := decodeLoop(t, w.Body)
	if resumed["status"] != "running" {
		t.Errorf("expected status=running, got %v", resumed["status"])
	}
	if int(resumed["iteration"].(float64)) != 3 {
		t.Errorf("ResumeLoop: expected iteration=3, got %v", resumed["iteration"])
	}
	if resumed["current_stage"] != "develop" {
		t.Errorf("ResumeLoop: expected current_stage=develop, got %v", resumed["current_stage"])
	}
}

func intPtr(v int) *int { return &v }

func TestCancelLoop_UpdatesStatus(t *testing.T) {
	issueID := createLoopTestIssue(t, "loop cancel")
	agentID := createLoopTestAgent(t)

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/loops?workspace_id="+testWorkspaceID, map[string]any{
		"issue_id": issueID,
		"agent_id": agentID,
	})
	testHandler.CreateLoop(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("setup: CreateLoop: expected 201, got %d", w.Code)
	}
	loopID := decodeLoop(t, w.Body)["id"].(string)

	w = httptest.NewRecorder()
	req = newRequest("POST", "/api/loops/"+loopID+"/cancel", nil)
	req = withURLParam(req, "id", loopID)
	testHandler.CancelLoop(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("CancelLoop: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	got := decodeLoop(t, w.Body)
	if got["status"] != "cancelled" {
		t.Errorf("expected status=cancelled, got %v", got["status"])
	}
}
