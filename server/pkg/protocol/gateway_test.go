package protocol

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestGatewayTaskCompletedMessageUsesCanonicalSnakeCase(t *testing.T) {
	data, err := json.Marshal(GatewayTaskCompletedMessage{
		Type:     EventTaskCompleted,
		TaskID:   "task-1",
		ExitCode: 17,
		Output:   "failed",
	})
	if err != nil {
		t.Fatal(err)
	}

	got := string(data)
	for _, field := range []string{`"task_id":"task-1"`, `"exit_code":17`} {
		if !strings.Contains(got, field) {
			t.Fatalf("message %s does not contain %s", got, field)
		}
	}
	if strings.Contains(got, "taskId") || strings.Contains(got, "exitCode") {
		t.Fatalf("message contains legacy camelCase fields: %s", got)
	}
}

func TestGatewayTaskLogsMessageRoundTripPreservesCursor(t *testing.T) {
	want := GatewayTaskLogsMessage{
		Type:    EventTaskLogs,
		TaskID:  "task-1",
		Seq:     42,
		Stream:  GatewayStreamStderr,
		Content: "boom\n",
	}
	data, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}

	var got GatewayTaskLogsMessage
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("round trip = %+v, want %+v", got, want)
	}
}
