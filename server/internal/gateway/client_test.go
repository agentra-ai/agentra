package gateway

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/agentra-ai/agentra/server/pkg/protocol"
)

func protocolTestClient() *Client {
	return &Client{
		ID:          "gateway-1",
		WorkspaceID: "workspace-1",
		Hub:         NewHub(),
		Send:        make(chan []byte, 1),
	}
}

func TestClientHandleMessageDecodesIntegerExitCode(t *testing.T) {
	client := protocolTestClient()
	var gotTaskID string
	var gotExitCode int
	client.Hub.OnTaskComplete = func(_, _, taskID string, exitCode int, _ string) {
		gotTaskID = taskID
		gotExitCode = exitCode
	}

	message, err := json.Marshal(protocol.GatewayTaskCompletedMessage{
		Type:     protocol.EventTaskCompleted,
		TaskID:   "task-1",
		ExitCode: 23,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.handleMessage(message); err != nil {
		t.Fatalf("handleMessage: %v", err)
	}
	if gotTaskID != "task-1" || gotExitCode != 23 {
		t.Fatalf("callback = (%q, %d), want (task-1, 23)", gotTaskID, gotExitCode)
	}
}

func TestClientHandleMessagePropagatesWorkspaceAndLogCursor(t *testing.T) {
	client := protocolTestClient()
	var got []any
	client.Hub.OnTaskLogs = func(gatewayID, workspaceID, taskID string, seq int, stream, content string) {
		got = []any{gatewayID, workspaceID, taskID, seq, stream, content}
	}

	message, err := json.Marshal(protocol.GatewayTaskLogsMessage{
		Type:    protocol.EventTaskLogs,
		TaskID:  "task-1",
		Seq:     9,
		Stream:  protocol.GatewayStreamStderr,
		Content: "failure\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.handleMessage(message); err != nil {
		t.Fatalf("handleMessage: %v", err)
	}

	want := []any{"gateway-1", "workspace-1", "task-1", 9, "stderr", "failure\n"}
	if len(got) != len(want) {
		t.Fatalf("callback = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("callback[%d] = %#v, want %#v", i, got[i], want[i])
		}
	}
}

func TestClientHandleMessageRejectsLegacyCamelCase(t *testing.T) {
	client := protocolTestClient()
	called := false
	client.Hub.OnTaskComplete = func(_, _, _ string, _ int, _ string) { called = true }

	err := client.handleMessage([]byte(`{"type":"task:completed","taskId":"task-1","exitCode":7}`))
	if err == nil || !strings.Contains(err.Error(), "task_id is required") {
		t.Fatalf("error = %v, want missing task_id", err)
	}
	if called {
		t.Fatal("callback ran for a legacy camelCase message")
	}
}

func TestClientHandleMessageRejectsOversizedLogFrame(t *testing.T) {
	client := protocolTestClient()
	message, err := json.Marshal(protocol.GatewayTaskLogsMessage{
		Type:    protocol.EventTaskLogs,
		TaskID:  "task-1",
		Seq:     1,
		Stream:  protocol.GatewayStreamStdout,
		Content: strings.Repeat("x", protocol.GatewayLogChunkBytes+1),
	})
	if err != nil {
		t.Fatal(err)
	}

	err = client.handleMessage(message)
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("error = %v, want size rejection", err)
	}
}
