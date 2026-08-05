package gateway

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/agentra-ai/agentra/server/pkg/protocol"
)

func TestTerminalSpoolSurvivesReopenAndRequiresAck(t *testing.T) {
	stateDir := t.TempDir()
	spool, err := newTerminalSpool(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	message := protocol.GatewayTaskCompletedMessage{
		Type: protocol.EventTaskCompleted, EventID: "run-1", TaskID: "task-1", RunID: "run-1", ExitCode: 0,
	}
	if err := spool.Put("run-1", message); err != nil {
		t.Fatal(err)
	}
	// A duplicate terminal callback for the same Run cannot replace the first
	// immutable outcome.
	if err := spool.Put("run-1", protocol.GatewayTaskFailedMessage{
		Type: protocol.EventTaskFailed, EventID: "run-1", TaskID: "task-1", RunID: "run-1", Error: "late",
	}); err != nil {
		t.Fatal(err)
	}

	reopened, err := newTerminalSpool(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	deliveries, err := reopened.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(deliveries) != 1 || deliveries[0].EventID != "run-1" {
		t.Fatalf("reopened deliveries = %#v", deliveries)
	}
	var got protocol.GatewayTaskCompletedMessage
	if err := json.Unmarshal(deliveries[0].Message, &got); err != nil {
		t.Fatal(err)
	}
	if got.Type != protocol.EventTaskCompleted || got.TaskID != "task-1" {
		t.Fatalf("spooled terminal = %+v", got)
	}
	if err := reopened.Ack("run-1"); err != nil {
		t.Fatal(err)
	}
	deliveries, err = reopened.List()
	if err != nil || len(deliveries) != 0 {
		t.Fatalf("deliveries after ack = %d err:%v", len(deliveries), err)
	}
}

func TestGatewayRedactsTerminalPayloadBeforeDurableSpool(t *testing.T) {
	spool, err := newTerminalSpool(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	g := &Gateway{spool: spool, wsClient: &WSClient{}}
	secret := "Authorization: Bearer eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.abc123"
	// The network send is expected to fail for this disconnected unit fixture;
	// persistence must already have completed.
	if err := g.queueTaskCompleted("task-1", "run-1", 0, secret); err == nil {
		t.Fatal("queueTaskCompleted unexpectedly sent on a disconnected client")
	}
	deliveries, err := spool.List()
	if err != nil || len(deliveries) != 1 {
		t.Fatalf("deliveries = %d err:%v", len(deliveries), err)
	}
	if strings.Contains(string(deliveries[0].Message), "eyJhbGci") {
		t.Fatalf("durable terminal payload contains secret: %s", deliveries[0].Message)
	}
}
