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
	var gotRunID string
	var gotExitCode int
	client.Hub.OnTaskComplete = func(_, _, taskID, runID string, exitCode int, _ string) bool {
		gotTaskID = taskID
		gotRunID = runID
		gotExitCode = exitCode
		return true
	}

	message, err := json.Marshal(protocol.GatewayTaskCompletedMessage{
		Type:     protocol.EventTaskCompleted,
		TaskID:   "task-1",
		RunID:    "run-1",
		ExitCode: 23,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.handleMessage(message); err != nil {
		t.Fatalf("handleMessage: %v", err)
	}
	if gotTaskID != "task-1" || gotRunID != "run-1" || gotExitCode != 23 {
		t.Fatalf("callback = (%q, %d), want (task-1, 23)", gotTaskID, gotExitCode)
	}
	select {
	case raw := <-client.Send:
		var ack protocol.GatewayDeliveryAckMessage
		if err := json.Unmarshal(raw, &ack); err != nil {
			t.Fatal(err)
		}
		if ack.Type != protocol.EventGatewayDeliveryAck || ack.EventID != "run-1" {
			t.Fatalf("ack = %+v", ack)
		}
	default:
		t.Fatal("durable terminal callback did not enqueue an ack")
	}
}

func TestClientHandleMessagePropagatesWorkspaceAndLogCursor(t *testing.T) {
	client := protocolTestClient()
	var got []any
	client.Hub.OnTaskLogs = func(gatewayID, workspaceID, taskID, runID string, seq int, stream, content string) {
		got = []any{gatewayID, workspaceID, taskID, runID, seq, stream, content}
	}

	message, err := json.Marshal(protocol.GatewayTaskLogsMessage{
		Type:    protocol.EventTaskLogs,
		TaskID:  "task-1",
		RunID:   "run-1",
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

	want := []any{"gateway-1", "workspace-1", "task-1", "run-1", 9, "stderr", "failure\n"}
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
	client.Hub.OnTaskComplete = func(_, _, _, _ string, _ int, _ string) bool {
		called = true
		return true
	}

	err := client.handleMessage([]byte(`{"type":"task:completed","taskId":"task-1","exitCode":7}`))
	if err == nil || !strings.Contains(err.Error(), "task_id and run_id are required") {
		t.Fatalf("error = %v, want missing task_id and run_id", err)
	}
	if called {
		t.Fatal("callback ran for a legacy camelCase message")
	}
}

func TestClientHandleMessageDoesNotAckRejectedTerminalDelivery(t *testing.T) {
	client := protocolTestClient()
	client.Hub.OnTaskFail = func(_, _, _, _ string, _ string, _ bool) bool { return false }
	message, err := json.Marshal(protocol.GatewayTaskFailedMessage{
		Type: protocol.EventTaskFailed, EventID: "run-1", TaskID: "task-1", RunID: "run-1", Error: "failed",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.handleMessage(message); err != nil {
		t.Fatalf("handleMessage: %v", err)
	}
	select {
	case ack := <-client.Send:
		t.Fatalf("unexpected ack: %s", ack)
	default:
	}
}

func TestClientHandleMessageRejectsTerminalEventIDForAnotherRun(t *testing.T) {
	client := protocolTestClient()
	client.Hub.OnTaskComplete = func(_, _, _, _ string, _ int, _ string) bool { return true }
	message, err := json.Marshal(protocol.GatewayTaskCompletedMessage{
		Type: protocol.EventTaskCompleted, EventID: "run-2", TaskID: "task-1", RunID: "run-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	err = client.handleMessage(message)
	if err == nil || !strings.Contains(err.Error(), "event_id must match run_id") {
		t.Fatalf("error = %v", err)
	}
}

func TestClientHandleMessageRejectsOversizedLogFrame(t *testing.T) {
	client := protocolTestClient()
	message, err := json.Marshal(protocol.GatewayTaskLogsMessage{
		Type:    protocol.EventTaskLogs,
		TaskID:  "task-1",
		RunID:   "run-1",
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
