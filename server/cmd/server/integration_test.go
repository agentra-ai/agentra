package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/agentra-ai/agentra/pkg/taskgraph"
	"github.com/agentra-ai/agentra/server/internal/auth"
	"github.com/agentra-ai/agentra/server/internal/events"
	"github.com/agentra-ai/agentra/server/internal/realtime"
	"github.com/agentra-ai/agentra/server/internal/service"
	"github.com/agentra-ai/agentra/server/internal/util"
	db "github.com/agentra-ai/agentra/server/pkg/db/generated"
	"github.com/agentra-ai/agentra/server/pkg/protocol"
	stripelib "github.com/agentra-ai/agentra/server/pkg/stripe"
)

var (
	testServer          *httptest.Server
	testPool            *pgxpool.Pool
	testHub             *realtime.Hub
	testToken           string
	testUserID          string
	testWorkspaceID     string
	testLifecycleWorker *service.LifecycleOutboxWorker
	testTaskDerived     *service.TaskDerivedLifecycleProjector
)

// jwtSecret is resolved at runtime via auth.JWTSecret() so it respects
// the JWT_SECRET env var (set in .env) and stays in sync with the server.

const (
	integrationTestEmail         = "integration-test@agentra.ai"
	integrationTestName          = "Integration Tester"
	integrationTestWorkspaceSlug = "integration-tests"
)

func TestMain(m *testing.M) {
	// Ensure a deterministic dev JWT secret in clean CI environments that
	// don't set JWT_SECRET. auth.JWTSecret() panics without one; this lets
	// the suite run in GitHub Actions / fresh checkouts without an env file.
	if os.Getenv("JWT_SECRET") == "" {
		os.Setenv("JWT_SECRET", "test-secret-that-is-at-least-32-bytes-for-dev-only")
	}
	auth.ResetSecretForTesting()

	ctx := context.Background()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://agentra:agentra@localhost:5432/agentra?sslmode=disable"
	}

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		fmt.Printf("Integration database unavailable; database-backed tests will skip: %v\n", err)
		os.Exit(m.Run())
	}
	if err := pool.Ping(ctx); err != nil {
		fmt.Printf("Integration database unavailable; database-backed tests will skip: %v\n", err)
		pool.Close()
		os.Exit(m.Run())
	}

	testPool = pool
	testUserID, testWorkspaceID, err = setupIntegrationTestFixture(ctx, pool)
	if err != nil {
		fmt.Printf("Failed to set up integration test fixture: %v\n", err)
		pool.Close()
		os.Exit(1)
	}

	hub := realtime.NewHub()
	go hub.Run()
	testHub = hub

	bus := events.New()
	registerListeners(bus, hub)
	testLifecycleWorker = service.NewLifecycleOutboxWorker(db.New(pool), bus, service.NewTraceServiceFromPool(pool))
	testTaskDerived = service.NewTaskDerivedLifecycleProjector(pool, db.New(pool), bus)
	stripeClient := stripelib.NewClient("", "", "", "")
	router := NewRouter(pool, hub, bus, stripeClient)
	testServer = httptest.NewServer(router)
	// Allow the test server's own loopback origin for WebSocket upgrades.
	// NewRouter wires corsconfig.AllowedOrigins() which reads FRONTEND_ORIGIN
	// (set to web.agentra.orb.local in .env) — that doesn't match the
	// 127.0.0.1:PORT test server, so we override the allowList here.
	// httptest's testServer.URL is already an absolute "http://host:port" origin.
	realtime.SetWSAllowedOrigins([]string{testServer.URL})

	// Generate a JWT token directly for the test user
	testToken, err = generateTestJWT(testUserID, integrationTestEmail, integrationTestName)
	if err != nil {
		fmt.Printf("Failed to generate test JWT: %v\n", err)
		testServer.Close()
		pool.Close()
		os.Exit(1)
	}

	code := m.Run()

	if err := cleanupIntegrationTestFixture(context.Background(), pool); err != nil {
		fmt.Printf("Failed to clean up integration test fixture: %v\n", err)
		if code == 0 {
			code = 1
		}
	}
	testServer.Close()
	pool.Close()
	os.Exit(code)
}

func requireIntegrationDB(t *testing.T) {
	t.Helper()
	if testPool == nil || testServer == nil {
		t.Skip("integration database is not available")
	}
}

func drainLifecycleOutbox(t *testing.T) {
	t.Helper()
	for {
		processed, err := testLifecycleWorker.ProcessNext(context.Background())
		if err != nil {
			t.Fatalf("drain lifecycle outbox: %v", err)
		}
		if !processed {
			break
		}
	}
	for {
		processed, err := testTaskDerived.ProcessNext(context.Background())
		if err != nil {
			t.Fatalf("drain task-derived lifecycle projection: %v", err)
		}
		if !processed {
			return
		}
	}
}

func setupIntegrationTestFixture(ctx context.Context, pool *pgxpool.Pool) (string, string, error) {
	if err := cleanupIntegrationTestFixture(ctx, pool); err != nil {
		return "", "", err
	}

	var userID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO "user" (name, email)
		VALUES ($1, $2)
		RETURNING id
	`, integrationTestName, integrationTestEmail).Scan(&userID); err != nil {
		return "", "", err
	}

	var workspaceID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO workspace (name, slug, description)
		VALUES ($1, $2, $3)
		RETURNING id
	`, "Integration Tests", integrationTestWorkspaceSlug, "Temporary workspace for router integration tests").Scan(&workspaceID); err != nil {
		return "", "", err
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO member (workspace_id, user_id, role)
		VALUES ($1, $2, 'owner')
	`, workspaceID, userID); err != nil {
		return "", "", err
	}

	var runtimeID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO agent_runtime (
			workspace_id, daemon_id, name, runtime_mode, provider, status, device_info, metadata, last_seen_at
		)
		VALUES ($1, NULL, $2, 'cloud', $3, 'online', $4, '{}'::jsonb, now())
		RETURNING id
	`, workspaceID, "Integration Test Runtime", "integration_test_runtime", "Integration test runtime").Scan(&runtimeID); err != nil {
		return "", "", err
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO agent (
			workspace_id, name, description, runtime_mode, runtime_config,
			runtime_id, visibility, max_concurrent_tasks, owner_id, tools, triggers
		)
		VALUES ($1, $2, '', 'cloud', '{}'::jsonb, $3, 'workspace', 1, $4, '[]'::jsonb, '[]'::jsonb)
	`, workspaceID, "Integration Test Agent", runtimeID, userID); err != nil {
		return "", "", err
	}

	return userID, workspaceID, nil
}

func cleanupIntegrationTestFixture(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, `DELETE FROM workspace WHERE slug = $1`, integrationTestWorkspaceSlug); err != nil {
		return err
	}
	if _, err := pool.Exec(ctx, `DELETE FROM "user" WHERE email = $1`, integrationTestEmail); err != nil {
		return err
	}
	return nil
}

// Helper to make authenticated requests
func authRequest(t *testing.T, method, path string, body any) *http.Response {
	t.Helper()
	var bodyReader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, testServer.URL+path, bodyReader)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+testToken)
	req.Header.Set("X-Workspace-ID", testWorkspaceID)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	return resp
}

func readJSON(t *testing.T, resp *http.Response, v any) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
}

func generateTestJWT(userID, email, name string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":   userID,
		"email": email,
		"name":  name,
		"exp":   time.Now().Add(72 * time.Hour).Unix(),
		"iat":   time.Now().Unix(),
	})
	return token.SignedString(auth.JWTSecret())
}

func createTaskMessageFixture(t *testing.T, status, runtimeType, cloudRuntimeID string) (issueID, taskID string) {
	t.Helper()
	var agentID, runtimeID string
	if err := testPool.QueryRow(context.Background(), `
		SELECT id, runtime_id FROM agent
		WHERE workspace_id = $1
		ORDER BY created_at ASC
		LIMIT 1
	`, testWorkspaceID).Scan(&agentID, &runtimeID); err != nil {
		t.Fatalf("load task fixture agent: %v", err)
	}
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO issue (workspace_id, title, status, priority, creator_type, creator_id, assignee_type, assignee_id)
		VALUES ($1, 'Task message fixture', 'in_progress', 'medium', 'member', $2, 'agent', $3)
		RETURNING id
	`, testWorkspaceID, testUserID, agentID).Scan(&issueID); err != nil {
		t.Fatalf("create task fixture issue: %v", err)
	}
	t.Cleanup(func() {
		// Completed tasks create both trace models. Keep this fixture compatible
		// with databases that have not yet applied lifecycle FK migration 048.
		_, _ = testPool.Exec(context.Background(), `DELETE FROM execution_traces WHERE task_id = $1`, taskID)
		_, _ = testPool.Exec(context.Background(), `DELETE FROM task_runs WHERE task_id = $1`, taskID)
		_, _ = testPool.Exec(context.Background(), `DELETE FROM issue WHERE id = $1`, issueID)
	})
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO agent_task_queue (
			agent_id, issue_id, status, runtime_id, runtime_type, cloud_runtime_id,
			dispatched_at, started_at
		)
		VALUES (
			$1, $2, $3, $4, $5, NULLIF($6, '')::uuid,
			CASE WHEN $3 IN ('dispatched', 'running') THEN now() ELSE NULL END,
			CASE WHEN $3 = 'running' THEN now() ELSE NULL END
		)
		RETURNING id
	`, agentID, issueID, status, runtimeID, runtimeType, cloudRuntimeID).Scan(&taskID); err != nil {
		t.Fatalf("create task fixture: %v", err)
	}
	if status == "dispatched" || status == "running" {
		var runID string
		if err := testPool.QueryRow(context.Background(), `
			INSERT INTO task_runs (task_id, agent_id, status, started_at)
			VALUES ($1, $2, $3, now())
			RETURNING id
		`, taskID, agentID, status).Scan(&runID); err != nil {
			t.Fatalf("create task fixture run: %v", err)
		}
		if _, err := testPool.Exec(context.Background(), `
			UPDATE agent_task_queue SET active_run_id = $2 WHERE id = $1
		`, taskID, runID); err != nil {
			t.Fatalf("link task fixture run: %v", err)
		}
	}
	return issueID, taskID
}

func activeRunIDForTask(t *testing.T, taskID string) string {
	t.Helper()
	var runID string
	if err := testPool.QueryRow(context.Background(), `
		SELECT active_run_id FROM agent_task_queue WHERE id = $1
	`, taskID).Scan(&runID); err != nil {
		t.Fatalf("load active run: %v", err)
	}
	return runID
}

func dispatchNewRunForTask(t *testing.T, taskID string) string {
	t.Helper()
	var runID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO task_runs (task_id, agent_id, status, started_at)
		SELECT id, agent_id, 'dispatched', now()
		FROM agent_task_queue
		WHERE id = $1
		RETURNING id
	`, taskID).Scan(&runID); err != nil {
		t.Fatalf("create retry run: %v", err)
	}
	if _, err := testPool.Exec(context.Background(), `
		UPDATE agent_task_queue
		SET status = 'dispatched', active_run_id = $2,
		    started_at = NULL, completed_at = NULL, error = NULL
		WHERE id = $1
	`, taskID, runID); err != nil {
		t.Fatalf("dispatch retry run: %v", err)
	}
	return runID
}

func startTaskFixture(t *testing.T, taskID string) string {
	t.Helper()
	runID := activeRunIDForTask(t, taskID)
	resp := authRequest(t, http.MethodPost, "/api/daemon/tasks/"+taskID+"/start", map[string]any{"run_id": runID})
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("start task: status = %d: %s", resp.StatusCode, body)
	}
	var started struct {
		RunID string `json:"run_id"`
	}
	readJSON(t, resp, &started)
	if started.RunID != runID {
		t.Fatalf("start task returned run_id %q, want %q", started.RunID, runID)
	}
	return started.RunID
}

// ---- Health ----

func TestHealth(t *testing.T) {
	requireIntegrationDB(t)

	resp, err := http.Get(testServer.URL + "/health")
	if err != nil {
		t.Fatalf("health check failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if result["status"] != "ok" {
		t.Fatalf("expected status ok, got %s", result["status"])
	}
}

// ---- Auth ----

func TestSendCodeAndVerify(t *testing.T) {
	requireIntegrationDB(t)

	const email = "integration-sendcode@agentra.ai"
	ctx := context.Background()

	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM verification_code WHERE email = $1`, email)
		var userID string
		err := testPool.QueryRow(ctx, `SELECT id FROM "user" WHERE email = $1`, email).Scan(&userID)
		if err == nil {
			rows, queryErr := testPool.Query(ctx, `
				SELECT w.id FROM workspace w JOIN member m ON m.workspace_id = w.id WHERE m.user_id = $1
			`, userID)
			if queryErr == nil {
				defer rows.Close()
				for rows.Next() {
					var wsID string
					if rows.Scan(&wsID) == nil {
						testPool.Exec(ctx, `DELETE FROM workspace WHERE id = $1`, wsID)
					}
				}
			}
		}
		testPool.Exec(ctx, `DELETE FROM "user" WHERE email = $1`, email)
	})

	// Step 1: Send code
	body, _ := json.Marshal(map[string]string{"email": email})
	resp, err := http.Post(testServer.URL+"/auth/send-code", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("send-code failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("send-code: expected 200, got %d", resp.StatusCode)
	}
	var sendCodeResp struct {
		Message string  `json:"message"`
		DevCode *string `json:"dev_code"`
	}
	readJSON(t, resp, &sendCodeResp)
	if sendCodeResp.Message == "" {
		t.Fatal("expected non-empty send-code message")
	}
	if sendCodeResp.DevCode == nil || *sendCodeResp.DevCode == "" {
		t.Fatal("expected dev_code in development mode")
	}

	// Read code from DB
	var code string
	err = testPool.QueryRow(ctx, `SELECT code FROM verification_code WHERE email = $1 ORDER BY created_at DESC LIMIT 1`, email).Scan(&code)
	if err != nil {
		t.Fatalf("failed to read code from DB: %v", err)
	}

	// Step 2: Verify code
	body, _ = json.Marshal(map[string]string{"email": email, "code": code})
	resp, err = http.Post(testServer.URL+"/auth/verify-code", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("verify-code failed: %v", err)
	}
	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("verify-code: expected 200, got %d: %s", resp.StatusCode, respBody)
	}

	var loginResp struct {
		Token string `json:"token"`
		User  struct {
			Email string `json:"email"`
		} `json:"user"`
	}
	readJSON(t, resp, &loginResp)

	if loginResp.Token == "" {
		t.Fatal("expected non-empty token")
	}
	if loginResp.User.Email != email {
		t.Fatalf("expected email '%s', got '%s'", email, loginResp.User.Email)
	}

	// Verify the token works with /api/me
	req, _ := http.NewRequest("GET", testServer.URL+"/api/me", nil)
	req.Header.Set("Authorization", "Bearer "+loginResp.Token)
	meResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("getMe failed: %v", err)
	}
	if meResp.StatusCode != 200 {
		t.Fatalf("getMe: expected 200, got %d", meResp.StatusCode)
	}
	meResp.Body.Close()
}

func TestVerifyCodeCreatesWorkspaceForNewUser(t *testing.T) {
	requireIntegrationDB(t)

	const email = "new-integration-verify@agentra.ai"
	ctx := context.Background()

	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM verification_code WHERE email = $1`, email)
		var userID string
		err := testPool.QueryRow(ctx, `SELECT id FROM "user" WHERE email = $1`, email).Scan(&userID)
		if err == nil {
			rows, queryErr := testPool.Query(ctx, `
				SELECT w.id FROM workspace w JOIN member m ON m.workspace_id = w.id WHERE m.user_id = $1
			`, userID)
			if queryErr == nil {
				defer rows.Close()
				for rows.Next() {
					var wsID string
					if rows.Scan(&wsID) == nil {
						testPool.Exec(ctx, `DELETE FROM workspace WHERE id = $1`, wsID)
					}
				}
			}
		}
		testPool.Exec(ctx, `DELETE FROM "user" WHERE email = $1`, email)
	})

	testPool.Exec(ctx, `DELETE FROM "user" WHERE email = $1`, email)

	// Send code
	body, _ := json.Marshal(map[string]string{"email": email})
	resp, err := http.Post(testServer.URL+"/auth/send-code", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("send-code failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("send-code: expected 200, got %d", resp.StatusCode)
	}
	var sendCodeResp struct {
		Message string  `json:"message"`
		DevCode *string `json:"dev_code"`
	}
	readJSON(t, resp, &sendCodeResp)
	if sendCodeResp.Message == "" {
		t.Fatal("expected non-empty send-code message")
	}
	if sendCodeResp.DevCode == nil || *sendCodeResp.DevCode == "" {
		t.Fatal("expected dev_code in development mode")
	}

	// Read code from DB
	var code string
	err = testPool.QueryRow(ctx, `SELECT code FROM verification_code WHERE email = $1 ORDER BY created_at DESC LIMIT 1`, email).Scan(&code)
	if err != nil {
		t.Fatalf("failed to read code from DB: %v", err)
	}

	// Verify code
	body, _ = json.Marshal(map[string]string{"email": email, "code": code})
	resp, err = http.Post(testServer.URL+"/auth/verify-code", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("verify-code failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("verify-code: expected 200, got %d", resp.StatusCode)
	}

	var loginResp struct {
		Token string `json:"token"`
	}
	readJSON(t, resp, &loginResp)

	// Check workspace was created
	req, _ := http.NewRequest("GET", testServer.URL+"/api/workspaces", nil)
	req.Header.Set("Authorization", "Bearer "+loginResp.Token)
	workspacesResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("listWorkspaces failed: %v", err)
	}
	defer workspacesResp.Body.Close()

	if workspacesResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", workspacesResp.StatusCode)
	}

	var workspaces []struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	}
	readJSON(t, workspacesResp, &workspaces)

	if len(workspaces) != 1 {
		t.Fatalf("expected 1 workspace, got %d", len(workspaces))
	}
	if !strings.Contains(workspaces[0].Name, "Workspace") {
		t.Fatalf("expected workspace name containing 'Workspace', got %q", workspaces[0].Name)
	}
}

func TestProtectedRoutesRequireAuth(t *testing.T) {
	requireIntegrationDB(t)

	paths := []string{"/api/me", "/api/issues", "/api/agents", "/api/inbox", "/api/workspaces"}

	for _, path := range paths {
		resp, err := http.Get(testServer.URL + path)
		if err != nil {
			t.Fatalf("request to %s failed: %v", path, err)
		}
		resp.Body.Close()
		if resp.StatusCode != 401 {
			t.Fatalf("%s: expected 401, got %d", path, resp.StatusCode)
		}
	}
}

func TestInvalidJWT(t *testing.T) {
	requireIntegrationDB(t)

	cases := []struct {
		name  string
		token string
	}{
		{"garbage token", "not-a-jwt"},
		{"empty token", ""},
		{"wrong secret", func() string {
			claims := jwt.MapClaims{"sub": "test", "exp": time.Now().Add(time.Hour).Unix()}
			t, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("wrong"))
			return t
		}()},
		{"expired token", func() string {
			claims := jwt.MapClaims{"sub": "test", "exp": time.Now().Add(-time.Hour).Unix()}
			t, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(auth.JWTSecret())
			return t
		}()},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", testServer.URL+"/api/me", nil)
			if tc.token != "" {
				req.Header.Set("Authorization", "Bearer "+tc.token)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			resp.Body.Close()
			if resp.StatusCode != 401 {
				t.Fatalf("expected 401, got %d", resp.StatusCode)
			}
		})
	}
}

// ---- Issues CRUD through full router ----

func TestIssuesCRUDThroughRouter(t *testing.T) {
	requireIntegrationDB(t)

	// Create
	resp := authRequest(t, "POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
		"title":    "Integration test issue",
		"status":   "todo",
		"priority": "high",
	})
	if resp.StatusCode != 201 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("CreateIssue: expected 201, got %d: %s", resp.StatusCode, body)
	}

	var created map[string]any
	readJSON(t, resp, &created)
	issueID := created["id"].(string)
	if created["title"] != "Integration test issue" {
		t.Fatalf("expected title 'Integration test issue', got '%s'", created["title"])
	}

	// Get
	resp = authRequest(t, "GET", "/api/issues/"+issueID, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("GetIssue: expected 200, got %d", resp.StatusCode)
	}
	var fetched map[string]any
	readJSON(t, resp, &fetched)
	if fetched["id"] != issueID {
		t.Fatalf("expected id %s, got %s", issueID, fetched["id"])
	}

	// Update status only — should preserve title
	resp = authRequest(t, "PUT", "/api/issues/"+issueID, map[string]any{
		"status": "in_progress",
	})
	if resp.StatusCode != 200 {
		t.Fatalf("UpdateIssue: expected 200, got %d", resp.StatusCode)
	}
	var updated map[string]any
	readJSON(t, resp, &updated)
	if updated["status"] != "in_progress" {
		t.Fatalf("expected status 'in_progress', got '%s'", updated["status"])
	}
	if updated["title"] != "Integration test issue" {
		t.Fatalf("title should be preserved, got '%s'", updated["title"])
	}

	// Update title only — should preserve status
	resp = authRequest(t, "PUT", "/api/issues/"+issueID, map[string]any{
		"title": "Renamed integration issue",
	})
	if resp.StatusCode != 200 {
		t.Fatalf("UpdateIssue title: expected 200, got %d", resp.StatusCode)
	}
	var updated2 map[string]any
	readJSON(t, resp, &updated2)
	if updated2["title"] != "Renamed integration issue" {
		t.Fatalf("expected title 'Renamed integration issue', got '%s'", updated2["title"])
	}
	if updated2["status"] != "in_progress" {
		t.Fatalf("status should be preserved, got '%s'", updated2["status"])
	}

	// List
	resp = authRequest(t, "GET", "/api/issues?workspace_id="+testWorkspaceID, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("ListIssues: expected 200, got %d", resp.StatusCode)
	}
	var listResp map[string]any
	readJSON(t, resp, &listResp)
	total := listResp["total"].(float64)
	if total < 1 {
		t.Fatal("expected at least 1 issue")
	}

	// Delete
	resp = authRequest(t, "DELETE", "/api/issues/"+issueID, nil)
	resp.Body.Close()
	if resp.StatusCode != 204 {
		t.Fatalf("DeleteIssue: expected 204, got %d", resp.StatusCode)
	}

	// Verify deleted
	resp = authRequest(t, "GET", "/api/issues/"+issueID, nil)
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("GetIssue after delete: expected 404, got %d", resp.StatusCode)
	}
}

func TestTaskGraphThroughRouter(t *testing.T) {
	requireIntegrationDB(t)

	resp := authRequest(t, "POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
		"title": "Task graph integration test",
	})
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("CreateIssue: expected 201, got %d: %s", resp.StatusCode, body)
	}
	var issue map[string]any
	readJSON(t, resp, &issue)
	issueID := issue["id"].(string)
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM issue WHERE id = $1`, issueID)
	})

	graphStore := taskgraph.NewGraphStore(testPool)
	node, err := graphStore.CreateNode(
		context.Background(),
		testWorkspaceID,
		issueID,
		taskgraph.NodeTypeExecutor,
		0,
		[]byte(`{"description":"Implement the contract"}`),
	)
	if err != nil {
		t.Fatalf("create graph node: %v", err)
	}

	resp = authRequest(t, "GET", "/api/issues/"+issueID+"/graph", nil)
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("GetTaskGraph: expected 200, got %d: %s", resp.StatusCode, body)
	}
	var graph struct {
		Nodes []taskgraph.GraphNode `json:"nodes"`
		Edges []taskgraph.GraphEdge `json:"edges"`
	}
	readJSON(t, resp, &graph)
	if len(graph.Nodes) != 1 || graph.Nodes[0].ID != node.ID {
		t.Fatalf("expected graph node %s, got %#v", node.ID, graph.Nodes)
	}

	resp = authRequest(t, "PATCH", "/api/graph/nodes/"+node.ID, map[string]any{
		"status":     "running",
		"position_x": 42,
	})
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("UpdateTaskGraphNode: expected 200, got %d: %s", resp.StatusCode, body)
	}
	var updated taskgraph.GraphNode
	readJSON(t, resp, &updated)
	if updated.Status != taskgraph.StatusRunning || updated.PositionX != 42 {
		t.Fatalf("unexpected updated node: %#v", updated)
	}

	resp = authRequest(t, "PATCH", "/api/graph/nodes/"+node.ID, map[string]any{
		"status": "not-a-status",
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("UpdateTaskGraphNode invalid status: expected 400, got %d", resp.StatusCode)
	}

	resp = authRequest(t, "DELETE", "/api/graph/nodes/"+node.ID, nil)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("DeleteTaskGraphNode: expected 204, got %d", resp.StatusCode)
	}
}

func TestMetricsThroughRouterUsesAuthorizedWorkspace(t *testing.T) {
	requireIntegrationDB(t)

	resp := authRequest(t, "GET", "/api/admin/metrics/summary?days=7", nil)
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("Metrics Summary: expected 200, got %d: %s", resp.StatusCode, body)
	}
	var summary struct {
		Days int `json:"days"`
	}
	readJSON(t, resp, &summary)
	if summary.Days != 7 {
		t.Fatalf("expected 7 day window, got %d", summary.Days)
	}

	// A query parameter can select a workspace only through the same owner/admin
	// middleware. It must not bypass authorization inside the metrics handler.
	resp = authRequest(t, "GET", "/api/admin/metrics/summary?workspace_id=00000000-0000-0000-0000-000000000000&days=7", nil)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("Metrics Summary unauthorized workspace: expected 404, got %d", resp.StatusCode)
	}
}

func TestTeamMemoryThroughRouter(t *testing.T) {
	requireIntegrationDB(t)

	basePath := "/api/workspaces/" + testWorkspaceID + "/memories"
	resp := authRequest(t, "POST", basePath, map[string]any{
		"memory_type": "learning",
		"content":     "  Router contract memory  ",
	})
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("CreateTeamMemory: expected 201, got %d: %s", resp.StatusCode, body)
	}
	var created map[string]any
	readJSON(t, resp, &created)
	memoryID := created["id"].(string)
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM team_memory WHERE id = $1`, memoryID)
	})
	if created["content"] != "Router contract memory" {
		t.Fatalf("expected trimmed content, got %#v", created["content"])
	}

	resp = authRequest(t, "GET", basePath+"/search?q=contract", nil)
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("SearchMemories: expected 200, got %d: %s", resp.StatusCode, body)
	}
	var search struct {
		Memories []map[string]any `json:"memories"`
	}
	readJSON(t, resp, &search)
	if len(search.Memories) == 0 || search.Memories[0]["workspace_id"] != testWorkspaceID {
		t.Fatalf("expected workspace-scoped search result, got %#v", search.Memories)
	}

	resp = authRequest(t, "POST", basePath, map[string]any{
		"memory_type": "unsupported",
		"content":     "invalid",
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("CreateTeamMemory invalid type: expected 400, got %d", resp.StatusCode)
	}

	resp = authRequest(t, "DELETE", basePath+"/"+memoryID, nil)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("DeleteTeamMemory: expected 204, got %d", resp.StatusCode)
	}
}

// ---- Comments through full router ----

func TestCommentsThroughRouter(t *testing.T) {
	requireIntegrationDB(t)

	// Create issue
	resp := authRequest(t, "POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
		"title": "Comment integration test",
	})
	var issue map[string]any
	readJSON(t, resp, &issue)
	issueID := issue["id"].(string)

	// Create comment
	resp = authRequest(t, "POST", "/api/issues/"+issueID+"/comments", map[string]any{
		"content": "Integration test comment",
		"type":    "comment",
	})
	if resp.StatusCode != 201 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("CreateComment: expected 201, got %d: %s", resp.StatusCode, body)
	}
	var comment map[string]any
	readJSON(t, resp, &comment)
	if comment["content"] != "Integration test comment" {
		t.Fatalf("expected content 'Integration test comment', got '%s'", comment["content"])
	}

	// Create second comment
	resp = authRequest(t, "POST", "/api/issues/"+issueID+"/comments", map[string]any{
		"content": "Second comment",
		"type":    "comment",
	})
	resp.Body.Close()

	// List comments
	resp = authRequest(t, "GET", "/api/issues/"+issueID+"/comments", nil)
	if resp.StatusCode != 200 {
		t.Fatalf("ListComments: expected 200, got %d", resp.StatusCode)
	}
	var comments []map[string]any
	readJSON(t, resp, &comments)
	if len(comments) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(comments))
	}

	// Cleanup
	resp = authRequest(t, "DELETE", "/api/issues/"+issueID, nil)
	resp.Body.Close()
}

// ---- Agents through full router ----

func TestAgentsThroughRouter(t *testing.T) {
	requireIntegrationDB(t)

	// List
	resp := authRequest(t, "GET", "/api/agents?workspace_id="+testWorkspaceID, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("ListAgents: expected 200, got %d", resp.StatusCode)
	}
	var agents []map[string]any
	readJSON(t, resp, &agents)
	if len(agents) < 1 {
		t.Fatal("expected at least 1 agent")
	}

	// Get
	agentID := agents[0]["id"].(string)
	resp = authRequest(t, "GET", "/api/agents/"+agentID, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("GetAgent: expected 200, got %d", resp.StatusCode)
	}
	var agent map[string]any
	readJSON(t, resp, &agent)
	if agent["id"] != agentID {
		t.Fatalf("expected agent id %s, got %s", agentID, agent["id"])
	}

	// Update status
	resp = authRequest(t, "PUT", "/api/agents/"+agentID, map[string]any{
		"status": "idle",
	})
	if resp.StatusCode != 200 {
		t.Fatalf("UpdateAgent: expected 200, got %d", resp.StatusCode)
	}
	var updated map[string]any
	readJSON(t, resp, &updated)
	if updated["status"] != "idle" {
		t.Fatalf("expected status 'idle', got '%s'", updated["status"])
	}
	// Name should be preserved
	if updated["name"] != agents[0]["name"] {
		t.Fatalf("name should be preserved, got '%s'", updated["name"])
	}
}

// ---- Workspaces through full router ----

func TestWorkspacesThroughRouter(t *testing.T) {
	requireIntegrationDB(t)

	// List
	resp := authRequest(t, "GET", "/api/workspaces", nil)
	if resp.StatusCode != 200 {
		t.Fatalf("ListWorkspaces: expected 200, got %d", resp.StatusCode)
	}
	var workspaces []map[string]any
	readJSON(t, resp, &workspaces)
	if len(workspaces) < 1 {
		t.Fatal("expected at least 1 workspace")
	}

	// Get
	wsID := workspaces[0]["id"].(string)
	resp = authRequest(t, "GET", "/api/workspaces/"+wsID, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("GetWorkspace: expected 200, got %d", resp.StatusCode)
	}
	var ws map[string]any
	readJSON(t, resp, &ws)
	if ws["id"] != wsID {
		t.Fatalf("expected workspace id %s, got %s", wsID, ws["id"])
	}

	// Update
	resp = authRequest(t, "PUT", "/api/workspaces/"+wsID, map[string]any{
		"description": "Integration test update",
	})
	if resp.StatusCode != 200 {
		t.Fatalf("UpdateWorkspace: expected 200, got %d", resp.StatusCode)
	}
	var updated map[string]any
	readJSON(t, resp, &updated)
	if updated["description"] != "Integration test update" {
		t.Fatalf("expected description 'Integration test update', got '%v'", updated["description"])
	}
	// Name should be preserved
	if updated["name"] != ws["name"] {
		t.Fatalf("name should be preserved")
	}

	// Members
	resp = authRequest(t, "GET", "/api/workspaces/"+wsID+"/members", nil)
	if resp.StatusCode != 200 {
		t.Fatalf("ListMembers: expected 200, got %d", resp.StatusCode)
	}
	var members []map[string]any
	readJSON(t, resp, &members)
	if len(members) < 1 {
		t.Fatal("expected at least 1 member")
	}
	// Verify member has user info
	if members[0]["email"] == nil || members[0]["email"] == "" {
		t.Fatal("member should have email field")
	}
	if members[0]["role"] == nil || members[0]["role"] == "" {
		t.Fatal("member should have role field")
	}
}

// ---- Inbox through full router ----

func TestInboxThroughRouter(t *testing.T) {
	requireIntegrationDB(t)

	resp := authRequest(t, "GET", "/api/inbox", nil)
	if resp.StatusCode != 200 {
		t.Fatalf("ListInbox: expected 200, got %d", resp.StatusCode)
	}
	var items []map[string]any
	readJSON(t, resp, &items)
	// Inbox may be empty, just verify it returns valid JSON array
	if items == nil {
		t.Fatal("expected non-nil inbox items array")
	}
}

// ---- 404 for non-existent resources ----

func TestNonExistentResources(t *testing.T) {
	requireIntegrationDB(t)

	fakeUUID := "00000000-0000-0000-0000-000000000000"

	cases := []struct {
		name string
		path string
	}{
		{"issue", "/api/issues/" + fakeUUID},
		{"agent", "/api/agents/" + fakeUUID},
		{"workspace", "/api/workspaces/" + fakeUUID},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := authRequest(t, "GET", tc.path, nil)
			resp.Body.Close()
			if resp.StatusCode != 404 {
				t.Fatalf("expected 404, got %d", resp.StatusCode)
			}
		})
	}
}

// ---- Invalid request bodies ----

func TestInvalidRequestBodies(t *testing.T) {
	requireIntegrationDB(t)

	resp := authRequest(t, "POST", "/api/issues?workspace_id="+testWorkspaceID, nil)
	defer resp.Body.Close()
	// Sending nil body should fail with 400
	if resp.StatusCode != 400 {
		// Some handlers may return 500 for nil body, that's acceptable too
		if resp.StatusCode != 500 {
			t.Fatalf("expected 400 or 500, got %d", resp.StatusCode)
		}
	}
}

// ---- Durable task message stream ----

func TestTaskMessagesAreRedactedIdempotentAndCursorBounded(t *testing.T) {
	requireIntegrationDB(t)
	_, taskID := createTaskMessageFixture(t, "dispatched", "local", "")
	runID := startTaskFixture(t, taskID)
	path := "/api/daemon/tasks/" + taskID + "/messages"

	resp := authRequest(t, http.MethodPost, path, map[string]any{
		"run_id": runID,
		"messages": []map[string]any{
			{"seq": 1, "type": "text", "content": "OPENAI_API_KEY=super-secret"},
			{"seq": 2, "type": "text", "content": "second"},
		},
	})
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("report messages: status = %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	// Replaying a cursor is a successful no-op and must not replace or duplicate
	// the original durable message.
	resp = authRequest(t, http.MethodPost, path, map[string]any{
		"run_id":   runID,
		"messages": []map[string]any{{"seq": 2, "type": "text", "content": "duplicate"}},
	})
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("replay message: status = %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	resp = authRequest(t, http.MethodGet, path+"?since=1&limit=1", nil)
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("list messages: status = %d: %s", resp.StatusCode, body)
	}
	var messages []map[string]any
	readJSON(t, resp, &messages)
	if len(messages) != 1 || messages[0]["seq"] != float64(2) || messages[0]["content"] != "second" {
		t.Fatalf("cursor response = %#v", messages)
	}

	var count int
	var firstContent string
	if err := testPool.QueryRow(context.Background(), `
		SELECT count(*), min(content) FILTER (WHERE seq = 1)
		FROM task_message WHERE task_id = $1
	`, taskID).Scan(&count, &firstContent); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("persisted messages = %d, want 2", count)
	}
	if strings.Contains(firstContent, "super-secret") || !strings.Contains(firstContent, "REDACTED") {
		t.Fatalf("secret was not redacted: %q", firstContent)
	}
}

func TestTaskRunIdentityIsolatesRetryMessages(t *testing.T) {
	requireIntegrationDB(t)
	issueID, taskID := createTaskMessageFixture(t, "dispatched", "local", "")
	path := "/api/daemon/tasks/" + taskID + "/messages"

	run1 := startTaskFixture(t, taskID)
	resp := authRequest(t, http.MethodPost, path, map[string]any{
		"run_id":   run1,
		"messages": []map[string]any{{"seq": 1, "type": "text", "content": "first attempt"}},
	})
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("report first-run message: status = %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	resp = authRequest(t, http.MethodPost, "/api/daemon/tasks/"+taskID+"/fail", map[string]any{
		"run_id": run1,
		"error":  "retry fixture",
	})
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("fail first run: status = %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	ctx := context.Background()
	dispatchedRun2 := dispatchNewRunForTask(t, taskID)
	run2 := startTaskFixture(t, taskID)
	if run2 != dispatchedRun2 {
		t.Fatalf("started run %q, want dispatched run %q", run2, dispatchedRun2)
	}
	if run2 == run1 {
		t.Fatalf("retry reused run_id %q", run1)
	}
	resp = authRequest(t, http.MethodPost, path, map[string]any{
		"run_id":   run2,
		"messages": []map[string]any{{"seq": 1, "type": "text", "content": "second attempt"}},
	})
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("report second-run message: status = %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	resp = authRequest(t, http.MethodPost, "/api/daemon/tasks/"+taskID+"/session", map[string]any{
		"run_id": run2, "session_id": "fresh-session", "work_dir": "/tmp/fresh-run",
	})
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("checkpoint second run: status = %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	var commentsBeforeStale, metricsBeforeStale, lifecycleEventsBeforeStale int
	if err := testPool.QueryRow(ctx, `SELECT count(*) FROM comment WHERE issue_id = $1`, issueID).Scan(&commentsBeforeStale); err != nil {
		t.Fatal(err)
	}
	if err := testPool.QueryRow(ctx, `SELECT count(*) FROM agent_task_metrics WHERE task_id = $1`, taskID).Scan(&metricsBeforeStale); err != nil {
		t.Fatal(err)
	}
	if err := testPool.QueryRow(ctx, `SELECT count(*) FROM lifecycle_outbox WHERE work_item_id = $1`, taskID).Scan(&lifecycleEventsBeforeStale); err != nil {
		t.Fatal(err)
	}

	assertConflict := func(method, requestPath string, body any) {
		t.Helper()
		staleResp := authRequest(t, method, requestPath, body)
		defer staleResp.Body.Close()
		if staleResp.StatusCode != http.StatusConflict {
			responseBody, _ := io.ReadAll(staleResp.Body)
			t.Fatalf("stale callback %s %s = %d, want 409: %s", method, requestPath, staleResp.StatusCode, responseBody)
		}
	}

	// Every attempt-scoped Adapter callback must reject the superseded Run,
	// not only durable message frames.
	assertConflict(http.MethodPost, "/api/daemon/tasks/"+taskID+"/start", map[string]any{"run_id": run1})
	assertConflict(http.MethodPost, "/api/daemon/tasks/"+taskID+"/progress", map[string]any{
		"run_id": run1, "summary": "stale progress", "step": 1, "total": 1,
	})
	assertConflict(http.MethodPost, "/api/daemon/tasks/"+taskID+"/stage", map[string]any{
		"run_id": run1, "stage": "testing",
	})
	assertConflict(http.MethodPost, "/api/daemon/tasks/"+taskID+"/session", map[string]any{
		"run_id": run1, "session_id": "stale-session", "work_dir": "/tmp/stale-run",
	})
	assertConflict(http.MethodPost, "/api/daemon/tasks/"+taskID+"/complete", map[string]any{
		"run_id": run1, "output": "stale completion", "duration_ms": 1,
	})
	assertConflict(http.MethodPost, "/api/daemon/tasks/"+taskID+"/fail", map[string]any{
		"run_id": run1, "error": "stale failure",
	})
	assertConflict(http.MethodGet, "/api/daemon/tasks/"+taskID+"/status?run_id="+run1, nil)

	// A delayed frame from the previous process must not contaminate the active
	// attempt, even though its cursor is valid within that older Run.
	resp = authRequest(t, http.MethodPost, path, map[string]any{
		"run_id":   run1,
		"messages": []map[string]any{{"seq": 2, "type": "text", "content": "stale frame"}},
	})
	if resp.StatusCode != http.StatusConflict {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("stale run status = %d, want 409: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	resp = authRequest(t, http.MethodGet, path, nil)
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("list active-run messages: status = %d: %s", resp.StatusCode, body)
	}
	var messages []protocol.TaskMessagePayload
	readJSON(t, resp, &messages)
	if len(messages) != 1 || messages[0].RunID != run2 || messages[0].Content != "second attempt" {
		t.Fatalf("active-run messages = %#v", messages)
	}

	var persisted int
	if err := testPool.QueryRow(ctx, `
		SELECT count(*) FROM task_message WHERE task_id = $1 AND seq = 1
	`, taskID).Scan(&persisted); err != nil {
		t.Fatal(err)
	}
	if persisted != 2 {
		t.Fatalf("persisted retry messages = %d, want 2", persisted)
	}

	var taskStatus, activeRunID, sessionID, workDir string
	if err := testPool.QueryRow(ctx, `
		SELECT status, active_run_id, session_id, work_dir
		FROM agent_task_queue WHERE id = $1
	`, taskID).Scan(&taskStatus, &activeRunID, &sessionID, &workDir); err != nil {
		t.Fatal(err)
	}
	if taskStatus != "running" || activeRunID != run2 || sessionID != "fresh-session" || workDir != "/tmp/fresh-run" {
		t.Fatalf("active lifecycle = status:%q run:%q session:%q work:%q", taskStatus, activeRunID, sessionID, workDir)
	}

	var run1Status, run2Status, run2Session, run2WorkDir string
	if err := testPool.QueryRow(ctx, `
		SELECT old.status, current.status, current.session_id, current.work_dir
		FROM task_runs old
		JOIN task_runs current ON current.id = $2
		WHERE old.id = $1
	`, run1, run2).Scan(&run1Status, &run2Status, &run2Session, &run2WorkDir); err != nil {
		t.Fatal(err)
	}
	if run1Status != "failed" || run2Status != "running" || run2Session != "fresh-session" || run2WorkDir != "/tmp/fresh-run" {
		t.Fatalf("run states = old:%q current:%q session:%q work:%q", run1Status, run2Status, run2Session, run2WorkDir)
	}

	var commentsAfterStale, metricsAfterStale, lifecycleEventsAfterStale int
	if err := testPool.QueryRow(ctx, `SELECT count(*) FROM comment WHERE issue_id = $1`, issueID).Scan(&commentsAfterStale); err != nil {
		t.Fatal(err)
	}
	if err := testPool.QueryRow(ctx, `SELECT count(*) FROM agent_task_metrics WHERE task_id = $1`, taskID).Scan(&metricsAfterStale); err != nil {
		t.Fatal(err)
	}
	if err := testPool.QueryRow(ctx, `SELECT count(*) FROM lifecycle_outbox WHERE work_item_id = $1`, taskID).Scan(&lifecycleEventsAfterStale); err != nil {
		t.Fatal(err)
	}
	if commentsAfterStale != commentsBeforeStale || metricsAfterStale != metricsBeforeStale {
		t.Fatalf("stale callbacks wrote projections: comments %d->%d metrics %d->%d", commentsBeforeStale, commentsAfterStale, metricsBeforeStale, metricsAfterStale)
	}
	if lifecycleEventsAfterStale != lifecycleEventsBeforeStale {
		t.Fatalf("stale callbacks wrote lifecycle events: %d->%d", lifecycleEventsBeforeStale, lifecycleEventsAfterStale)
	}
}

func TestTaskMessageInsertCannotCrossTerminalTransition(t *testing.T) {
	requireIntegrationDB(t)
	_, taskID := createTaskMessageFixture(t, "dispatched", "local", "")
	runID := startTaskFixture(t, taskID)
	ctx := context.Background()

	terminalTx, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer terminalTx.Rollback(ctx)
	if _, err := terminalTx.Exec(ctx, `
		UPDATE task_runs
		SET status = 'completed', completed_at = now()
		WHERE id = $1
	`, runID); err != nil {
		t.Fatal(err)
	}
	if _, err := terminalTx.Exec(ctx, `
		UPDATE agent_task_queue
		SET status = 'completed', completed_at = now(), active_run_id = NULL
		WHERE id = $1
	`, taskID); err != nil {
		t.Fatal(err)
	}

	type insertResult struct {
		outcome db.CreateTaskMessageRow
		err     error
	}
	started := make(chan struct{})
	done := make(chan insertResult, 1)
	go func() {
		close(started)
		outcome, insertErr := db.New(testPool).CreateTaskMessage(ctx, db.CreateTaskMessageParams{
			TaskID:  util.ParseUUID(taskID),
			RunID:   util.ParseUUID(runID),
			Seq:     1,
			Type:    "text",
			Content: pgtype.Text{String: "must not cross terminal commit", Valid: true},
		})
		done <- insertResult{outcome: outcome, err: insertErr}
	}()
	<-started
	if err := terminalTx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	result := <-done
	if result.err != nil {
		t.Fatalf("conditional message insert: %v", result.err)
	}
	if result.outcome.Active || result.outcome.Inserted {
		t.Fatalf("message insert crossed terminal transition: %#v", result.outcome)
	}
	var messageCount int
	if err := testPool.QueryRow(ctx, `
		SELECT count(*) FROM task_message WHERE task_id = $1
	`, taskID).Scan(&messageCount); err != nil {
		t.Fatal(err)
	}
	if messageCount != 0 {
		t.Fatalf("post-terminal messages = %d, want 0", messageCount)
	}
}

func TestTaskSessionCheckpointSurvivesRuntimeRecovery(t *testing.T) {
	requireIntegrationDB(t)
	_, taskID := createTaskMessageFixture(t, "running", "local", "")
	runID := activeRunIDForTask(t, taskID)

	resp := authRequest(t, http.MethodPost, "/api/daemon/tasks/"+taskID+"/session", map[string]string{
		"run_id":     runID,
		"session_id": "session-before-crash",
		"work_dir":   "/tmp/agentra-checkpoint-worktree",
	})
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("checkpoint session: status = %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	var runtimeID, sessionID, workDir string
	if err := testPool.QueryRow(context.Background(), `
		SELECT runtime_id, session_id, work_dir
		FROM agent_task_queue
		WHERE id = $1
	`, taskID).Scan(&runtimeID, &sessionID, &workDir); err != nil {
		t.Fatal(err)
	}
	if sessionID != "session-before-crash" || workDir != "/tmp/agentra-checkpoint-worktree" {
		t.Fatalf("checkpoint = (%q, %q)", sessionID, workDir)
	}
	// Reproduce the long-crash path: after three missed heartbeats, the runtime
	// sweeper marks the orphan failed before the daemon can register again.
	if _, err := testPool.Exec(context.Background(), `
		UPDATE agent_task_queue
		SET status = 'failed', completed_at = now(), error = 'runtime went offline'
		WHERE id = $1
	`, taskID); err != nil {
		t.Fatal(err)
	}
	if _, err := testPool.Exec(context.Background(), `
		UPDATE agent_runtime
		SET daemon_id = 'crash-resume-daemon', provider = 'claude',
		    metadata = '{"instance_id":"previous-instance"}'::jsonb
		WHERE id = $1
	`, runtimeID); err != nil {
		t.Fatal(err)
	}

	queries := db.New(testPool)
	registration := map[string]any{
		"workspace_id": testWorkspaceID,
		"daemon_id":    "crash-resume-daemon",
		"instance_id":  "replacement-instance",
		"device_name":  "Crash Resume Test",
		"cli_version":  "test",
		"runtimes": []map[string]string{{
			"name": "Claude Crash Resume Test", "type": "claude", "version": "test", "status": "online",
		}},
	}
	resp = authRequest(t, http.MethodPost, "/api/daemon/register", registration)
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("register replacement daemon: status = %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	recovered, err := queries.GetAgentTask(context.Background(), util.ParseUUID(taskID))
	if err != nil {
		t.Fatalf("load recovered task: %v", err)
	}
	if recovered.Status != "queued" || recovered.RetryCount != 1 {
		t.Fatalf("recovered status = %q, retry_count = %d", recovered.Status, recovered.RetryCount)
	}
	if recovered.SessionID.String != "session-before-crash" || recovered.WorkDir.String != "/tmp/agentra-checkpoint-worktree" {
		t.Fatalf("recovery lost checkpoint: session = %q, work_dir = %q", recovered.SessionID.String, recovered.WorkDir.String)
	}

	resp = authRequest(t, http.MethodPost, "/api/daemon/runtimes/"+runtimeID+"/tasks/claim", map[string]any{})
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("claim recovered task: status = %d: %s", resp.StatusCode, body)
	}
	var claim struct {
		Task struct {
			ID             string `json:"id"`
			PriorSessionID string `json:"prior_session_id"`
			PriorWorkDir   string `json:"prior_work_dir"`
		} `json:"task"`
	}
	readJSON(t, resp, &claim)
	if claim.Task.ID != taskID || claim.Task.PriorSessionID != "session-before-crash" || claim.Task.PriorWorkDir != "/tmp/agentra-checkpoint-worktree" {
		t.Fatalf("claim did not resume current task checkpoint: %#v", claim.Task)
	}

	// Retrying registration from the same process instance is idempotent and
	// must not steal the task back from its active execution.
	resp = authRequest(t, http.MethodPost, "/api/daemon/register", registration)
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("repeat daemon registration: status = %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()
	idempotent, err := queries.GetAgentTask(context.Background(), util.ParseUUID(taskID))
	if err != nil {
		t.Fatal(err)
	}
	if idempotent.Status != "dispatched" || idempotent.RetryCount != 1 {
		t.Fatalf("repeat registration changed task: status = %q, retry_count = %d", idempotent.Status, idempotent.RetryCount)
	}

	// Recovery is bounded: once the retry budget is exhausted, another crash
	// fails closed instead of creating an infinite restart loop.
	if _, err := testPool.Exec(context.Background(), `
		UPDATE agent_task_queue
		SET status = 'running', retry_count = max_retries, started_at = now()
		WHERE id = $1
	`, taskID); err != nil {
		t.Fatal(err)
	}
	lifecycle := service.NewRunLifecycle(testPool, queries)
	requeued, failed, err := lifecycle.RecoverTasksForRuntime(context.Background(), util.ParseUUID(runtimeID))
	if err != nil {
		t.Fatalf("recover exhausted runtime task: %v", err)
	}
	if requeued != 0 || failed != 1 {
		t.Fatalf("exhausted recovery = requeued:%d failed:%d", requeued, failed)
	}
	exhaustedTask, err := queries.GetAgentTask(context.Background(), util.ParseUUID(taskID))
	if err != nil {
		t.Fatal(err)
	}
	if exhaustedTask.Status != "failed" || exhaustedTask.RetryCount != exhaustedTask.MaxRetries {
		t.Fatalf("exhausted recovery task = %#v", exhaustedTask)
	}
	if exhaustedTask.Error.String != "runtime restarted and retry budget was exhausted" {
		t.Fatalf("exhausted recovery error = %q", exhaustedTask.Error.String)
	}
}

func TestRuntimeClaimRejectsIncompatibleTaskBeforeDispatch(t *testing.T) {
	requireIntegrationDB(t)
	ctx := context.Background()

	var runtimeID, agentID, loopIssueID, standardIssueID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO agent_runtime (workspace_id, name, runtime_mode, provider, status)
		VALUES ($1, 'Pre-claim Codex Runtime', 'local', 'codex', 'online')
		RETURNING id
	`, testWorkspaceID).Scan(&runtimeID); err != nil {
		t.Fatalf("create runtime: %v", err)
	}
	if err := testPool.QueryRow(ctx, `
		INSERT INTO agent (workspace_id, name, runtime_mode, runtime_id, owner_id, provider, max_concurrent_tasks)
		VALUES ($1, 'Pre-claim Codex Agent', 'local', $2, $3, 'codex', 1)
		RETURNING id
	`, testWorkspaceID, runtimeID, testUserID).Scan(&agentID); err != nil {
		t.Fatalf("create agent: %v", err)
	}
	for title, target := range map[string]*string{
		"Incompatible loop task":   &loopIssueID,
		"Compatible standard task": &standardIssueID,
	} {
		var issueNumber int
		if err := testPool.QueryRow(ctx, `
			UPDATE workspace SET issue_counter = issue_counter + 1
			WHERE id = $1
			RETURNING issue_counter
		`, testWorkspaceID).Scan(&issueNumber); err != nil {
			t.Fatalf("reserve issue number for %q: %v", title, err)
		}
		if err := testPool.QueryRow(ctx, `
			INSERT INTO issue (workspace_id, title, status, priority, creator_type, creator_id, assignee_type, assignee_id, number)
			VALUES ($1, $2, 'in_progress', 'medium', 'member', $3, 'agent', $4, $5)
			RETURNING id
		`, testWorkspaceID, title, testUserID, agentID, issueNumber).Scan(target); err != nil {
			t.Fatalf("create issue %q: %v", title, err)
		}
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM issue WHERE id IN ($1, $2)`, loopIssueID, standardIssueID)
		_, _ = testPool.Exec(context.Background(), `DELETE FROM agent WHERE id = $1`, agentID)
		_, _ = testPool.Exec(context.Background(), `DELETE FROM agent_runtime WHERE id = $1`, runtimeID)
	})

	var incompatibleTaskID, compatibleTaskID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO agent_task_queue (agent_id, runtime_id, issue_id, status, priority, runtime_type, task_type)
		VALUES ($1, $2, $3, 'queued', 10, 'local', 'loop_plan')
		RETURNING id
	`, agentID, runtimeID, loopIssueID).Scan(&incompatibleTaskID); err != nil {
		t.Fatalf("create incompatible task: %v", err)
	}
	if err := testPool.QueryRow(ctx, `
		INSERT INTO agent_task_queue (agent_id, runtime_id, issue_id, status, priority, runtime_type, task_type)
		VALUES ($1, $2, $3, 'queued', 0, 'local', 'standard')
		RETURNING id
	`, agentID, runtimeID, standardIssueID).Scan(&compatibleTaskID); err != nil {
		t.Fatalf("create compatible task: %v", err)
	}

	resp := authRequest(t, http.MethodPost, "/api/daemon/runtimes/"+runtimeID+"/tasks/claim", map[string]any{})
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("claim task: status = %d: %s", resp.StatusCode, body)
	}
	var claim struct {
		Task struct {
			ID    string `json:"id"`
			RunID string `json:"run_id"`
		} `json:"task"`
	}
	readJSON(t, resp, &claim)
	if claim.Task.ID != compatibleTaskID {
		t.Fatalf("claimed task = %q, want compatible task %q", claim.Task.ID, compatibleTaskID)
	}
	if claim.Task.RunID == "" {
		t.Fatal("claimed task did not allocate a dispatch Run")
	}
	var claimedStatus, claimedActiveRunID, claimedRunStatus string
	if err := testPool.QueryRow(ctx, `
		SELECT atq.status, atq.active_run_id, tr.status
		FROM agent_task_queue atq
		JOIN task_runs tr ON tr.id = atq.active_run_id
		WHERE atq.id = $1
	`, compatibleTaskID).Scan(&claimedStatus, &claimedActiveRunID, &claimedRunStatus); err != nil {
		t.Fatal(err)
	}
	if claimedStatus != "dispatched" || claimedRunStatus != "dispatched" || claimedActiveRunID != claim.Task.RunID {
		t.Fatalf("claimed lifecycle = task:%q run:%q active:%q response:%q", claimedStatus, claimedRunStatus, claimedActiveRunID, claim.Task.RunID)
	}

	var incompatibleStatus, incompatibleError string
	var incompatibleDispatchedAt *time.Time
	if err := testPool.QueryRow(ctx, `
		SELECT status, error, dispatched_at
		FROM agent_task_queue WHERE id = $1
	`, incompatibleTaskID).Scan(&incompatibleStatus, &incompatibleError, &incompatibleDispatchedAt); err != nil {
		t.Fatal(err)
	}
	if incompatibleStatus != "failed" || incompatibleDispatchedAt != nil || !strings.Contains(incompatibleError, "max_turns") {
		t.Fatalf("incompatible task = status %q, error %q, dispatched_at %v", incompatibleStatus, incompatibleError, incompatibleDispatchedAt)
	}
	drainLifecycleOutbox(t)
	var rejectionEvents, rejectionComments int
	if err := testPool.QueryRow(ctx, `
		SELECT count(*) FROM lifecycle_outbox
		WHERE work_item_id = $1 AND event_type = 'work_item.rejected' AND processed_at IS NOT NULL
	`, incompatibleTaskID).Scan(&rejectionEvents); err != nil {
		t.Fatal(err)
	}
	if err := testPool.QueryRow(ctx, `
		SELECT count(*) FROM comment c
		JOIN lifecycle_outbox event ON event.id = c.lifecycle_event_id
		WHERE event.work_item_id = $1 AND event.event_type = 'work_item.rejected'
	`, incompatibleTaskID).Scan(&rejectionComments); err != nil {
		t.Fatal(err)
	}
	if rejectionEvents != 1 || rejectionComments != 1 {
		t.Fatalf("rejection projection = events:%d comments:%d, want 1/1", rejectionEvents, rejectionComments)
	}
}

func TestTaskCompletionPersistsUsageArtifactsAndMetrics(t *testing.T) {
	requireIntegrationDB(t)
	_, taskID := createTaskMessageFixture(t, "dispatched", "local", "")

	runID := startTaskFixture(t, taskID)

	usage := map[string]any{
		"input_tokens": 100, "output_tokens": 50, "reasoning_output_tokens": 10,
		"cache_read_tokens": 25, "cache_write_tokens": 5,
	}
	artifact := map[string]any{
		"kind": "report", "path": "artifacts/report.json",
		"media_type": "application/json", "sha256": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
	resp := authRequest(t, http.MethodPost, "/api/daemon/tasks/"+taskID+"/complete", map[string]any{
		"run_id": runID, "output": "completed with usage", "duration_ms": 1234,
		"token_usage": usage, "artifacts": []map[string]any{artifact},
	})
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("complete task: status = %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()
	drainLifecycleOutbox(t)

	var resultJSON []byte
	if err := testPool.QueryRow(context.Background(), `SELECT result FROM agent_task_queue WHERE id = $1`, taskID).Scan(&resultJSON); err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal(resultJSON, &result); err != nil {
		t.Fatal(err)
	}
	resultUsage, ok := result["token_usage"].(map[string]any)
	if !ok || resultUsage["input_tokens"] != float64(100) || resultUsage["reasoning_output_tokens"] != float64(10) {
		t.Fatalf("persisted token usage = %#v", result["token_usage"])
	}
	resultArtifacts, ok := result["artifacts"].([]any)
	if !ok || len(resultArtifacts) != 1 {
		t.Fatalf("persisted artifacts = %#v", result["artifacts"])
	}

	var durationMs, tokenInput, tokenOutput int
	if err := testPool.QueryRow(context.Background(), `
		SELECT duration_ms, token_input, token_output
		FROM agent_task_metrics
		WHERE task_id = $1
	`, taskID).Scan(&durationMs, &tokenInput, &tokenOutput); err != nil {
		t.Fatal(err)
	}
	if durationMs != 1234 || tokenInput != 100 || tokenOutput != 60 {
		t.Fatalf("metric usage = duration:%d input:%d output:%d", durationMs, tokenInput, tokenOutput)
	}

	var taskStatus string
	var activeRunID *string
	if err := testPool.QueryRow(context.Background(), `
		SELECT status, active_run_id::text FROM agent_task_queue WHERE id = $1
	`, taskID).Scan(&taskStatus, &activeRunID); err != nil {
		t.Fatal(err)
	}
	if taskStatus != "completed" || activeRunID != nil {
		t.Fatalf("completed work item = status:%q active_run_id:%v", taskStatus, activeRunID)
	}

	var runStatus, runOutput string
	var runDurationMs, totalTokens int
	if err := testPool.QueryRow(context.Background(), `
		SELECT status, output, duration_ms, total_tokens
		FROM task_runs WHERE id = $1 AND task_id = $2
	`, runID, taskID).Scan(&runStatus, &runOutput, &runDurationMs, &totalTokens); err != nil {
		t.Fatal(err)
	}
	if runStatus != "completed" || runDurationMs != 1234 || totalTokens != 160 || !strings.Contains(runOutput, "completed with usage") {
		t.Fatalf("completed run = status:%q duration:%d tokens:%d output:%q", runStatus, runDurationMs, totalTokens, runOutput)
	}
}

func TestTaskCompletionRejectsInvalidUsageAndArtifacts(t *testing.T) {
	requireIntegrationDB(t)
	_, taskID := createTaskMessageFixture(t, "running", "local", "")
	runID := activeRunIDForTask(t, taskID)
	path := "/api/daemon/tasks/" + taskID + "/complete"

	resp := authRequest(t, http.MethodPost, path, map[string]any{
		"run_id": runID, "output": "invalid usage", "token_usage": map[string]any{"input_tokens": -1},
	})
	if resp.StatusCode != http.StatusBadRequest {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("negative usage status = %d, want 400: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	resp = authRequest(t, http.MethodPost, path, map[string]any{
		"run_id": runID, "output": "invalid artifact", "artifacts": []map[string]any{{"kind": "report"}},
	})
	if resp.StatusCode != http.StatusBadRequest {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("unlocated artifact status = %d, want 400: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	var status string
	if err := testPool.QueryRow(context.Background(), `SELECT status FROM agent_task_queue WHERE id = $1`, taskID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "running" {
		t.Fatalf("invalid completion mutated task to %q", status)
	}
}

func TestTaskMessagesRejectCrossWorkspaceReads(t *testing.T) {
	requireIntegrationDB(t)
	var otherWorkspaceID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO workspace (name, slug, description)
		VALUES ('Task Message Isolation', $1, '')
		RETURNING id
	`, "task-message-isolation-"+strings.ToLower(fmt.Sprintf("%d", time.Now().UnixNano()))).Scan(&otherWorkspaceID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM workspace WHERE id = $1`, otherWorkspaceID)
	})

	var agentID, runtimeID, issueID, taskID string
	if err := testPool.QueryRow(context.Background(), `SELECT id, runtime_id FROM agent WHERE workspace_id = $1 LIMIT 1`, testWorkspaceID).Scan(&agentID, &runtimeID); err != nil {
		t.Fatal(err)
	}
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO issue (workspace_id, title, creator_type, creator_id)
		VALUES ($1, 'Foreign task', 'member', $2)
		RETURNING id
	`, otherWorkspaceID, testUserID).Scan(&issueID); err != nil {
		t.Fatal(err)
	}
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO agent_task_queue (agent_id, issue_id, status, runtime_id)
		VALUES ($1, $2, 'running', $3)
		RETURNING id
	`, agentID, issueID, runtimeID).Scan(&taskID); err != nil {
		t.Fatal(err)
	}

	resp := authRequest(t, http.MethodGet, "/api/daemon/tasks/"+taskID+"/messages", nil)
	if resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("cross-workspace read status = %d, want 404: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	resp = authRequest(t, http.MethodPost, "/api/daemon/tasks/"+taskID+"/session", map[string]string{
		"run_id":     "11111111-1111-1111-1111-111111111111",
		"session_id": "foreign-session",
		"work_dir":   "/tmp/foreign-worktree",
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("cross-workspace checkpoint status = %d, want 404: %s", resp.StatusCode, body)
	}
}

func TestGatewayLogsFlowThroughDurableTaskMessageLedger(t *testing.T) {
	requireIntegrationDB(t)
	if testHub == nil {
		t.Fatal("test gateway hub is not configured")
	}

	var cloudRuntimeID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO cloud_runtimes (
			workspace_id, provider, encrypted_api_key, api_key_hash, max_concurrent_tasks
		)
		VALUES ($1, 'anthropic', $2, $3, 1)
		RETURNING id
	`, testWorkspaceID, []byte("encrypted-test-key"), "gateway-test-"+fmt.Sprintf("%d", time.Now().UnixNano())).Scan(&cloudRuntimeID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM cloud_runtimes WHERE id = $1`, cloudRuntimeID)
	})
	_, taskID := createTaskMessageFixture(t, "dispatched", "cloud", cloudRuntimeID)
	runID := activeRunIDForTask(t, taskID)

	testHub.GatewayHub.OnTaskDispatched("gateway-1", testWorkspaceID, taskID, runID, "container-1")
	testHub.GatewayHub.OnTaskLogs("gateway-evil", "00000000-0000-0000-0000-000000000000", taskID, runID, 1, "stdout", "cross-tenant")
	testHub.GatewayHub.OnTaskLogs("gateway-1", testWorkspaceID, taskID, runID, 1, "stdout", "AUTH_TOKEN=very-secret")
	testHub.GatewayHub.OnTaskLogs("gateway-1", testWorkspaceID, taskID, runID, 1, "stdout", "duplicate")

	var status, content string
	var count int
	if err := testPool.QueryRow(context.Background(), `
		SELECT atq.status, count(tm.id), min(tm.content)
		FROM agent_task_queue atq
		LEFT JOIN task_message tm ON tm.task_id = atq.id
		WHERE atq.id = $1
		GROUP BY atq.status
	`, taskID).Scan(&status, &count, &content); err != nil {
		t.Fatal(err)
	}
	if status != "running" || count != 1 {
		t.Fatalf("gateway task = status %q, messages %d", status, count)
	}
	if strings.Contains(content, "very-secret") || !strings.Contains(content, "REDACTED") {
		t.Fatalf("gateway content was not redacted: %q", content)
	}

	testHub.GatewayHub.OnTaskComplete("gateway-1", testWorkspaceID, taskID, runID, 0, "completed")
	if err := testPool.QueryRow(context.Background(), `SELECT status FROM agent_task_queue WHERE id = $1`, taskID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "completed" {
		t.Fatalf("gateway task status = %q, want completed", status)
	}
}

func TestGatewayRetryClosesRunBeforeRequeue(t *testing.T) {
	requireIntegrationDB(t)
	if testHub == nil {
		t.Fatal("test gateway hub is not configured")
	}

	var cloudRuntimeID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO cloud_runtimes (
			workspace_id, provider, encrypted_api_key, api_key_hash, max_concurrent_tasks
		)
		VALUES ($1, 'anthropic', $2, $3, 1)
		RETURNING id
	`, testWorkspaceID, []byte("encrypted-retry-key"), "gateway-retry-"+fmt.Sprintf("%d", time.Now().UnixNano())).Scan(&cloudRuntimeID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM cloud_runtimes WHERE id = $1`, cloudRuntimeID)
	})
	_, taskID := createTaskMessageFixture(t, "dispatched", "cloud", cloudRuntimeID)
	run1 := activeRunIDForTask(t, taskID)

	testHub.GatewayHub.OnTaskDispatched("gateway-1", testWorkspaceID, taskID, run1, "container-1")
	testHub.GatewayHub.OnTaskFail("gateway-1", testWorkspaceID, taskID, run1, "transient gateway failure", true)
	drainLifecycleOutbox(t)

	var taskStatus, run1Status, trace1Status string
	var activeRunID *string
	var retryCount int
	if err := testPool.QueryRow(context.Background(), `
		SELECT atq.status, atq.active_run_id::text, atq.retry_count, tr.status, et.status
		FROM agent_task_queue atq
		JOIN task_runs tr ON tr.id = $2
		JOIN execution_traces et ON et.run_id = tr.id
		WHERE atq.id = $1
	`, taskID, run1).Scan(&taskStatus, &activeRunID, &retryCount, &run1Status, &trace1Status); err != nil {
		t.Fatal(err)
	}
	if taskStatus != "queued" || activeRunID != nil || retryCount != 1 || run1Status != "failed" || trace1Status != "failed" {
		t.Fatalf("retried lifecycle = task:%q active:%v retry:%d run:%q trace:%q", taskStatus, activeRunID, retryCount, run1Status, trace1Status)
	}

	run2 := dispatchNewRunForTask(t, taskID)
	testHub.GatewayHub.OnTaskDispatched("gateway-1", testWorkspaceID, taskID, run2, "container-2")
	// A terminal frame already in flight from container-1 cannot complete the
	// newly running container-2 attempt.
	testHub.GatewayHub.OnTaskComplete("gateway-1", testWorkspaceID, taskID, run1, 0, "stale completion")
	if err := testPool.QueryRow(context.Background(), `
		SELECT status, active_run_id::text FROM agent_task_queue WHERE id = $1
	`, taskID).Scan(&taskStatus, &activeRunID); err != nil {
		t.Fatal(err)
	}
	if taskStatus != "running" || activeRunID == nil || *activeRunID != run2 {
		t.Fatalf("stale gateway terminal changed current run: status:%q active:%v want:%q", taskStatus, activeRunID, run2)
	}

	testHub.GatewayHub.OnTaskComplete("gateway-1", testWorkspaceID, taskID, run2, 0, "current completion")
	if err := testPool.QueryRow(context.Background(), `
		SELECT status, active_run_id::text FROM agent_task_queue WHERE id = $1
	`, taskID).Scan(&taskStatus, &activeRunID); err != nil {
		t.Fatal(err)
	}
	if taskStatus != "completed" || activeRunID != nil {
		t.Fatalf("current gateway terminal = status:%q active:%v", taskStatus, activeRunID)
	}
}

// ---- WebSocket integration through full router ----

func TestWebSocketIntegration(t *testing.T) {
	requireIntegrationDB(t)

	// Connect WebSocket client
	wsURL := "ws" + strings.TrimPrefix(testServer.URL, "http") + "/ws?token=" + testToken + "&workspace_id=" + testWorkspaceID
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("WebSocket connection failed: %v", err)
	}
	defer conn.Close()

	// Allow Hub goroutine to process the register and add client to room
	time.Sleep(100 * time.Millisecond)

	// Create an issue — this should trigger a WebSocket broadcast
	resp := authRequest(t, "POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
		"title":  "WebSocket test issue",
		"status": "todo",
	})
	var issue map[string]any
	readJSON(t, resp, &issue)
	issueID := issue["id"].(string)

	// Read the WebSocket message
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("WebSocket read error: %v", err)
	}

	// Verify the message contains the issue event
	var wsMsg map[string]any
	if err := json.Unmarshal(msg, &wsMsg); err != nil {
		t.Fatalf("failed to parse WebSocket message: %v", err)
	}
	if wsMsg["type"] != "issue:created" {
		t.Fatalf("expected type 'issue:created', got '%s'", wsMsg["type"])
	}

	// Update the issue — should trigger another broadcast
	resp = authRequest(t, "PUT", "/api/issues/"+issueID, map[string]any{
		"status": "in_progress",
	})
	resp.Body.Close()

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, msg, err = conn.ReadMessage()
	if err != nil {
		t.Fatalf("WebSocket read error on update: %v", err)
	}
	var updateMsg map[string]any
	json.Unmarshal(msg, &updateMsg)
	if updateMsg["type"] != "issue:updated" {
		t.Fatalf("expected type 'issue:updated', got '%s'", updateMsg["type"])
	}

	// Delete the issue — should trigger another broadcast
	resp = authRequest(t, "DELETE", "/api/issues/"+issueID, nil)
	resp.Body.Close()

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, msg, err = conn.ReadMessage()
	if err != nil {
		t.Fatalf("WebSocket read error on delete: %v", err)
	}
	var deleteMsg map[string]any
	json.Unmarshal(msg, &deleteMsg)
	if deleteMsg["type"] != "issue:deleted" {
		t.Fatalf("expected type 'issue:deleted', got '%s'", deleteMsg["type"])
	}
}

func TestWebSocketAcceptsPATAuthorizationHeader(t *testing.T) {
	requireIntegrationDB(t)

	rawToken := "mul_0123456789abcdef0123456789abcdef01234567"
	tokenHash := auth.HashToken(rawToken)
	if _, err := testPool.Exec(context.Background(), `
		INSERT INTO personal_access_token (user_id, name, token_hash, token_prefix)
		VALUES ($1, 'WebSocket integration', $2, 'mul_01234567')
	`, testUserID, tokenHash); err != nil {
		t.Fatalf("create PAT fixture: %v", err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM personal_access_token WHERE token_hash = $1`, tokenHash)
	})

	wsURL := "ws" + strings.TrimPrefix(testServer.URL, "http") + "/ws?workspace_id=" + testWorkspaceID
	header := http.Header{"Authorization": []string{"Bearer " + rawToken}}
	conn, response, err := websocket.DefaultDialer.Dial(wsURL, header)
	if response != nil && response.Body != nil {
		defer response.Body.Close()
	}
	if err != nil {
		t.Fatalf("WebSocket PAT connection failed: %v", err)
	}
	defer conn.Close()

	if err := conn.WriteJSON(map[string]string{"type": "ping"}); err != nil {
		t.Fatalf("write ping: %v", err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	var message map[string]string
	if err := conn.ReadJSON(&message); err != nil {
		t.Fatalf("read pong: %v", err)
	}
	if message["type"] != "pong" {
		t.Fatalf("message type = %q, want pong", message["type"])
	}
}
