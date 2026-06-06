package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	looppkg "github.com/agentra-ai/agentra/server/internal/loop"
)

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

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/loops?workspace_id="+testWorkspaceID, map[string]any{
		"issue_id":       issueID,
		"max_iterations": 7,
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

func TestCreateLoop_MissingIssueID(t *testing.T) {
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/loops?workspace_id="+testWorkspaceID, map[string]any{})
	testHandler.CreateLoop(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("CreateLoop (missing issue_id): expected 400, got %d: %s", w.Code, w.Body.String())
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

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/loops?workspace_id="+testWorkspaceID, map[string]any{
		"issue_id": issueID,
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
	otherWS := createLoopTestWorkspace(t)

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/loops?workspace_id="+testWorkspaceID, map[string]any{
		"issue_id": issueA,
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

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/loops?workspace_id="+testWorkspaceID, map[string]any{
		"issue_id": issueID,
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

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/loops?workspace_id="+testWorkspaceID, map[string]any{
		"issue_id": issueID,
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

func TestCancelLoop_UpdatesStatus(t *testing.T) {
	issueID := createLoopTestIssue(t, "loop cancel")

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/loops?workspace_id="+testWorkspaceID, map[string]any{
		"issue_id": issueID,
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
