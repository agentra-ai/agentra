package service

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/agentra-ai/agentra/server/internal/events"
	"github.com/agentra-ai/agentra/server/internal/util"
	"github.com/agentra-ai/agentra/server/pkg/crypto"
	db "github.com/agentra-ai/agentra/server/pkg/db/generated"
	"github.com/agentra-ai/agentra/server/pkg/protocol"
)

func TestCloudDispatchUsesDedicatedClaimRetriesAndAtomicAck(t *testing.T) {
	ctx, pool, q := lifecycleBatchPool(t)
	tasks, worker, sender, task, cleanup := createCloudDispatchFixture(t, ctx, pool, q)
	defer cleanup()

	// A local daemon polling the Agent's runtime must never receive a cloud
	// Work Item. Before this Module existed, this claim caused double execution.
	localClaim, err := tasks.ClaimTaskForRuntime(ctx, task.RuntimeID)
	if err != nil {
		t.Fatal(err)
	}
	if localClaim != nil {
		t.Fatalf("local daemon claimed cloud Work Item %s", util.UUIDToString(localClaim.Task.ID))
	}

	processed, err := worker.ProcessNext(ctx)
	if err != nil || !processed {
		t.Fatalf("schedule cloud Work Item = processed:%v err:%v", processed, err)
	}
	claimed, err := q.GetAgentTask(ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if claimed.Status != "dispatched" || !claimed.ActiveRunID.Valid {
		t.Fatalf("cloud claim = status:%q run:%v", claimed.Status, claimed.ActiveRunID)
	}
	delivery, err := q.GetCloudDispatchDelivery(ctx, claimed.ActiveRunID)
	if err != nil {
		t.Fatal(err)
	}
	if delivery.Attempts != 0 || delivery.AcknowledgedAt.Valid {
		t.Fatalf("new delivery = attempts:%d ack:%v", delivery.Attempts, delivery.AcknowledgedAt.Valid)
	}

	processed, err = worker.ProcessNext(ctx)
	if err != nil || !processed {
		t.Fatalf("send cloud delivery = processed:%v err:%v", processed, err)
	}
	first := sender.messagesSnapshot()
	if len(first) != 1 {
		t.Fatalf("gateway messages = %d, want 1", len(first))
	}
	assertCloudDispatchMessage(t, first[0], task.ID, claimed.ActiveRunID, "provider-secret")

	// Expire the acknowledgement window. The retry must retain the same Run
	// identity instead of allocating another execution attempt.
	if _, err := pool.Exec(ctx, `
		UPDATE cloud_dispatch_delivery SET available_at = now() - interval '1 second'
		WHERE run_id = $1
	`, claimed.ActiveRunID); err != nil {
		t.Fatal(err)
	}
	processed, err = worker.ProcessNext(ctx)
	if err != nil || !processed {
		t.Fatalf("retry cloud delivery = processed:%v err:%v", processed, err)
	}
	if len(sender.messagesSnapshot()) != 2 {
		t.Fatalf("gateway retry messages = %d, want 2", len(sender.messagesSnapshot()))
	}
	var runs int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM task_runs WHERE task_id = $1`, task.ID).Scan(&runs); err != nil {
		t.Fatal(err)
	}
	if runs != 1 {
		t.Fatalf("cloud delivery retry allocated %d Runs, want 1", runs)
	}

	started, err := tasks.Lifecycle.Start(ctx, RunRef{WorkItemID: task.ID, RunID: claimed.ActiveRunID})
	if err != nil {
		t.Fatal(err)
	}
	delivery, err = q.GetCloudDispatchDelivery(ctx, claimed.ActiveRunID)
	if err != nil {
		t.Fatal(err)
	}
	if started.Status != "running" || !delivery.AcknowledgedAt.Valid {
		t.Fatalf("cloud ack = task:%q acknowledged:%v", started.Status, delivery.AcknowledgedAt.Valid)
	}

	// Agent capacity is two, but the Cloud Runtime capacity is one. The second
	// Issue must remain queued until this Run reaches a terminal state.
	agent, err := q.GetAgent(ctx, task.AgentID)
	if err != nil {
		t.Fatal(err)
	}
	secondIssue, err := q.CreateIssue(ctx, db.CreateIssueParams{
		WorkspaceID: agent.WorkspaceID, Title: "cloud runtime capacity",
		Status: "todo", Priority: "high", CreatorType: "member", CreatorID: agent.OwnerID,
		Number: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := q.CreateAgentTask(ctx, db.CreateAgentTaskParams{
		AgentID: agent.ID, RuntimeID: agent.RuntimeID, IssueID: secondIssue.ID,
		Priority: 1, RuntimeType: "cloud", CloudRuntimeID: delivery.CloudRuntimeID, TaskType: "standard",
	})
	if err != nil {
		t.Fatal(err)
	}
	processed, err = worker.ProcessNext(ctx)
	if err != nil || processed {
		t.Fatalf("schedule above Cloud Runtime capacity = processed:%v err:%v", processed, err)
	}
	second, err = q.GetAgentTask(ctx, second.ID)
	if err != nil {
		t.Fatal(err)
	}
	if second.Status != "queued" {
		t.Fatalf("second cloud Work Item status = %q, want queued", second.Status)
	}
}

func TestCloudDispatchDoesNotClaimWithoutGatewayAndDeadLettersExhaustion(t *testing.T) {
	ctx, pool, q := lifecycleBatchPool(t)
	_, worker, sender, task, cleanup := createCloudDispatchFixture(t, ctx, pool, q)
	defer cleanup()

	sender.setConnected(false)
	processed, err := worker.ProcessNext(ctx)
	if err != nil || processed {
		t.Fatalf("schedule without Gateway = processed:%v err:%v", processed, err)
	}
	queued, err := q.GetAgentTask(ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if queued.Status != "queued" || queued.ActiveRunID.Valid {
		t.Fatalf("disconnected cloud task = status:%q run:%v", queued.Status, queued.ActiveRunID)
	}

	sender.setConnected(true)
	if processed, err = worker.ProcessNext(ctx); err != nil || !processed {
		t.Fatalf("schedule after Gateway reconnect = processed:%v err:%v", processed, err)
	}
	claimed, err := q.GetAgentTask(ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		UPDATE cloud_dispatch_delivery
		SET attempts = 19, available_at = now() - interval '1 second'
		WHERE run_id = $1
	`, claimed.ActiveRunID); err != nil {
		t.Fatal(err)
	}
	sender.setSendError(errors.New("gateway socket unavailable"))
	processed, err = worker.ProcessNext(ctx)
	if err != nil || !processed {
		t.Fatalf("exhaust cloud delivery = processed:%v err:%v", processed, err)
	}
	failed, err := q.GetAgentTask(ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	delivery, err := q.GetCloudDispatchDelivery(ctx, claimed.ActiveRunID)
	if err != nil {
		t.Fatal(err)
	}
	if failed.Status != "failed" || !delivery.DeadLetteredAt.Valid || delivery.Attempts != 20 {
		t.Fatalf("exhausted cloud delivery = task:%q dead:%v attempts:%d", failed.Status, delivery.DeadLetteredAt.Valid, delivery.Attempts)
	}
}

type fakeCloudGatewaySender struct {
	mu        sync.Mutex
	connected bool
	sendErr   error
	messages  [][]byte
}

func (s *fakeCloudGatewaySender) GetGatewayForWorkspace(string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.connected {
		return ""
	}
	return "gateway-test"
}

func (s *fakeCloudGatewaySender) SendToGateway(_ string, message []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sendErr != nil {
		return s.sendErr
	}
	s.messages = append(s.messages, append([]byte(nil), message...))
	return nil
}

func (s *fakeCloudGatewaySender) setConnected(connected bool) {
	s.mu.Lock()
	s.connected = connected
	s.mu.Unlock()
}

func (s *fakeCloudGatewaySender) setSendError(err error) {
	s.mu.Lock()
	s.sendErr = err
	s.mu.Unlock()
}

func (s *fakeCloudGatewaySender) messagesSnapshot() [][]byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([][]byte(nil), s.messages...)
}

func createCloudDispatchFixture(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	q *db.Queries,
) (*TaskService, *CloudDispatchWorker, *fakeCloudGatewaySender, db.AgentTaskQueue, func()) {
	t.Helper()
	agent, cleanup := createMetricTestFixture(t, ctx, pool, q)
	if _, err := pool.Exec(ctx, `UPDATE agent SET max_concurrent_tasks = 2 WHERE id = $1`, agent.ID); err != nil {
		cleanup()
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE agent_runtime SET provider = 'claude' WHERE id = $1`, agent.RuntimeID); err != nil {
		cleanup()
		t.Fatal(err)
	}
	issue := createLifecycleBatchIssue(t, ctx, q, agent)
	secret := []byte("cloud-dispatch-test-secret")
	encrypted, err := crypto.EncryptAPIKey("provider-secret", util.UUIDToString(agent.WorkspaceID)+string(secret))
	if err != nil {
		cleanup()
		t.Fatal(err)
	}
	runtime, err := q.CreateCloudRuntime(ctx, db.CreateCloudRuntimeParams{
		WorkspaceID: agent.WorkspaceID,
		GatewayUrl:  pgtype.Text{String: "https://gateway.example.test", Valid: true},
		Provider:    "anthropic", EncryptedApiKey: []byte(encrypted),
		ApiKeyHash: crypto.HashAPIKey("provider-secret"), MaxConcurrentTasks: 1,
	})
	if err != nil {
		cleanup()
		t.Fatal(err)
	}
	task, err := q.CreateAgentTask(ctx, db.CreateAgentTaskParams{
		AgentID: agent.ID, RuntimeID: agent.RuntimeID, IssueID: issue.ID,
		Priority: 1, RuntimeType: "cloud", CloudRuntimeID: runtime.ID, TaskType: "standard",
	})
	if err != nil {
		cleanup()
		t.Fatal(err)
	}
	tasks := NewTaskService(q, pool, events.New(), nil)
	sender := &fakeCloudGatewaySender{connected: true}
	worker := NewCloudDispatchWorker(tasks, sender, secret)
	return tasks, worker, sender, task, cleanup
}

func assertCloudDispatchMessage(t *testing.T, raw []byte, taskID, runID pgtype.UUID, apiKey string) {
	t.Helper()
	var message protocol.GatewayTaskDispatchMessage
	if err := json.Unmarshal(raw, &message); err != nil {
		t.Fatal(err)
	}
	if message.TaskID != util.UUIDToString(taskID) || message.RunID != util.UUIDToString(runID) || message.Config["api_key"] != apiKey {
		t.Fatalf("cloud dispatch message = task:%q run:%q api_key:%v", message.TaskID, message.RunID, message.Config["api_key"])
	}
}
