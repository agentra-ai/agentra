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
			Messages []TaskMessageData `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		batchSizes = append(batchSizes, len(body.Messages))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	messages := make([]TaskMessageData, protocol.TaskMessageBatchSize*2+3)
	for i := range messages {
		messages[i] = TaskMessageData{Seq: i + 1, Type: "text", Content: "line"}
	}
	if err := client.ReportTaskMessages(context.Background(), "task-1", messages); err != nil {
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
