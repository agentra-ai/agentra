package daemon

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agentra-ai/agentra/server/pkg/protocol"
)

func TestReportTaskMessagesSplitsBoundedBatches(t *testing.T) {
	var batchSizes []int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			RunID    string            `json:"run_id"`
			Messages []TaskMessageData `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		batchSizes = append(batchSizes, len(body.Messages))
		if body.RunID != "run-1" {
			t.Errorf("run_id = %q, want run-1", body.RunID)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	messages := make([]TaskMessageData, protocol.TaskMessageBatchSize*2+3)
	for i := range messages {
		messages[i] = TaskMessageData{Seq: i + 1, Type: "text", Content: "line"}
	}
	if err := client.ReportTaskMessages(context.Background(), "task-1", "run-1", messages); err != nil {
		t.Fatal(err)
	}

	want := []int{protocol.TaskMessageBatchSize, protocol.TaskMessageBatchSize, 3}
	if len(batchSizes) != len(want) {
		t.Fatalf("batch sizes = %v, want %v", batchSizes, want)
	}
	for i := range want {
		if batchSizes[i] != want[i] {
			t.Fatalf("batch sizes = %v, want %v", batchSizes, want)
		}
	}
}

func TestStartTaskReturnsRunID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"task-1","run_id":"run-1"}`))
	}))
	defer server.Close()

	runID, err := NewClient(server.URL).StartTask(context.Background(), "task-1")
	if err != nil {
		t.Fatal(err)
	}
	if runID != "run-1" {
		t.Fatalf("run_id = %q, want run-1", runID)
	}
}

func TestCheckpointTaskSessionRequest(t *testing.T) {
	var path string
	var body map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	if err := client.CheckpointTaskSession(context.Background(), "task-1", "session-1", "/tmp/worktree-1"); err != nil {
		t.Fatal(err)
	}
	if path != "/api/daemon/tasks/task-1/session" {
		t.Fatalf("path = %q", path)
	}
	if body["session_id"] != "session-1" || body["work_dir"] != "/tmp/worktree-1" {
		t.Fatalf("body = %#v", body)
	}
}

func TestCompleteTaskIncludesUsageAndArtifacts(t *testing.T) {
	var body struct {
		DurationMs int64                    `json:"duration_ms"`
		TokenUsage *protocol.TaskTokenUsage `json:"token_usage"`
		Artifacts  []protocol.TaskArtifact  `json:"artifacts"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	usage := &protocol.TaskTokenUsage{
		InputTokens: 100, OutputTokens: 50, ReasoningOutputTokens: 10,
		CacheReadTokens: 25, CacheWriteTokens: 5,
	}
	artifacts := []protocol.TaskArtifact{{Kind: "report", Path: "artifacts/report.json", MediaType: "application/json", SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}
	if err := client.CompleteTask(context.Background(), "task-1", "done", "branch", "session", "/tmp/worktree", 1234, usage, artifacts); err != nil {
		t.Fatal(err)
	}
	if body.DurationMs != 1234 || body.TokenUsage == nil || *body.TokenUsage != *usage {
		t.Fatalf("completion usage = %#v, duration = %d", body.TokenUsage, body.DurationMs)
	}
	if len(body.Artifacts) != 1 || body.Artifacts[0] != artifacts[0] {
		t.Fatalf("completion artifacts = %#v", body.Artifacts)
	}
}
