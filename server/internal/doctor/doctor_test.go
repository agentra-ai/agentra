package doctor

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestRunHealthy(t *testing.T) {
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatal("git executable is required for the runtime fixture")
	}

	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(configPath, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	repoPath := t.TempDir()
	mustRun(t, repoPath, "git", "init", "--quiet")
	bareOrigin := filepath.Join(t.TempDir(), "origin.git")
	mustRun(t, "", "git", "init", "--bare", "--quiet", bareOrigin)
	mustRun(t, repoPath, "git", "remote", "add", "origin", bareOrigin)

	var sawAuthorization atomic.Bool
	var sawTokenInQuery atomic.Bool
	upgrader := websocket.Upgrader{CheckOrigin: func(_ *http.Request) bool { return true }}
	mux := http.NewServeMux()
	mux.HandleFunc("/livez", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "live"})
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, readinessResponse{
			Status: "ready",
			Checks: map[string]readinessCheck{
				"database":  {Status: "ok"},
				"migration": {Status: "ok"},
				"storage":   {Status: "ok"},
				"scheduler": {Status: "ok"},
			},
		})
	})
	mux.HandleFunc("/api/me", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer mul_test_token" {
			sawAuthorization.Store(true)
		}
		writeJSON(w, http.StatusOK, map[string]string{"id": "user-1"})
	})
	mux.HandleFunc("/api/workspaces/ws-1", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"id": "ws-1"})
	})
	mux.HandleFunc("/daemon/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "running"})
	})
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("token") != "" {
			sawTokenInQuery.Store(true)
		}
		if r.Header.Get("Authorization") == "Bearer mul_test_token" {
			sawAuthorization.Store(true)
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		var message map[string]string
		if err := conn.ReadJSON(&message); err == nil && message["type"] == "ping" {
			_ = conn.WriteJSON(map[string]string{"type": "pong"})
		}
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	report := Run(context.Background(), Options{
		ServerURL:      server.URL,
		AppURL:         server.URL,
		WorkspaceID:    "ws-1",
		Token:          "mul_test_token",
		ConfigPath:     configPath,
		RepoPath:       repoPath,
		WorkspacesRoot: filepath.Join(t.TempDir(), "future-workspaces"),
		DaemonURL:      server.URL + "/daemon",
		Timeout:        2 * time.Second,
		RuntimeCandidates: []RuntimeCandidate{
			{Name: "codex", Path: gitPath},
		},
	})

	if report.Status != StatusPass {
		t.Fatalf("report status = %q, checks = %#v", report.Status, report.Checks)
	}
	if report.Summary.Passed != 13 || report.Summary.Failed != 0 || report.Summary.Warnings != 0 {
		t.Fatalf("unexpected summary: %#v", report.Summary)
	}
	if !sawAuthorization.Load() {
		t.Fatal("expected bearer token in Authorization header")
	}
	if sawTokenInQuery.Load() {
		t.Fatal("PAT must not be included in the WebSocket URL")
	}
}

func TestCheckReadinessReportsStorageFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusServiceUnavailable, readinessResponse{
			Status: "not_ready",
			Checks: map[string]readinessCheck{
				"database": {Status: "ok"},
				"storage":  {Status: "error"},
			},
		})
	}))
	defer server.Close()

	checks := checkReadiness(context.Background(), Options{
		ServerURL:  server.URL,
		Timeout:    time.Second,
		HTTPClient: server.Client(),
	})
	if len(checks) != 2 {
		t.Fatalf("len(checks) = %d, want 2", len(checks))
	}
	if checks[0].Status != StatusFail || checks[1].Status != StatusFail {
		t.Fatalf("checks = %#v", checks)
	}
}

func TestCheckConfigurationListsMissingValues(t *testing.T) {
	checks := checkConfiguration(context.Background(), Options{})
	if len(checks) != 1 || checks[0].Status != StatusFail {
		t.Fatalf("checks = %#v", checks)
	}
	for _, expected := range []string{"server URL", "access token", "workspace ID"} {
		if !strings.Contains(checks[0].Summary, expected) {
			t.Fatalf("summary %q does not contain %q", checks[0].Summary, expected)
		}
	}
}

func TestBuildReportPrecedence(t *testing.T) {
	report := buildReport([]Check{
		{ID: "one", Status: StatusPass},
		{ID: "two", Status: StatusWarning},
		{ID: "three", Status: StatusFail},
		{ID: "four", Status: StatusSkipped},
	})
	if report.Status != StatusFail {
		t.Fatalf("status = %q, want fail", report.Status)
	}
	if report.Summary != (Summary{Passed: 1, Warnings: 1, Failed: 1, Skipped: 1}) {
		t.Fatalf("summary = %#v", report.Summary)
	}
}

func mustRun(t *testing.T, directory, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = directory
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, output)
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
