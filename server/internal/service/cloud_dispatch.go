package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/agentra-ai/agentra/server/internal/util"
	"github.com/agentra-ai/agentra/server/pkg/crypto"
	db "github.com/agentra-ai/agentra/server/pkg/db/generated"
	"github.com/agentra-ai/agentra/server/pkg/protocol"
)

const (
	cloudDispatchPollInterval = 250 * time.Millisecond
	cloudDispatchMaxAttempts  = 20
)

type cloudGatewaySender interface {
	GetGatewayForWorkspace(workspaceID string) string
	SendToGateway(gatewayID string, message []byte) error
}

// CloudDispatchWorker is the push-transport counterpart to the local
// daemon's pull claim. It allocates cloud Runs only when their workspace has a
// connected Gateway, then leases and retries one durable delivery per Run.
// The decrypted provider key exists only in the outbound frame and is never
// persisted in the delivery ledger.
type CloudDispatchWorker struct {
	tasks     *TaskService
	queries   *db.Queries
	sender    cloudGatewaySender
	jwtSecret []byte
}

func NewCloudDispatchWorker(tasks *TaskService, sender cloudGatewaySender, jwtSecret []byte) *CloudDispatchWorker {
	var queries *db.Queries
	if tasks != nil {
		queries = tasks.Queries
	}
	return &CloudDispatchWorker{tasks: tasks, queries: queries, sender: sender, jwtSecret: jwtSecret}
}

func (w *CloudDispatchWorker) Run(ctx context.Context) {
	if w == nil || w.tasks == nil || w.queries == nil || w.sender == nil {
		return
	}
	w.Drain(ctx)
	ticker := time.NewTicker(cloudDispatchPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.Drain(ctx)
		}
	}
}

func (w *CloudDispatchWorker) Drain(ctx context.Context) {
	for {
		processed, err := w.ProcessNext(ctx)
		if err != nil {
			slog.Warn("cloud dispatch delivery failed", "error", err)
			return
		}
		if !processed {
			return
		}
	}
}

// ProcessNext first terminates exhausted deliveries, then retries an existing
// delivery, and only then allocates another cloud Run. Network I/O never holds
// a database transaction or row lock; the fencing token protects late writes.
func (w *CloudDispatchWorker) ProcessNext(ctx context.Context) (bool, error) {
	if w == nil || w.tasks == nil || w.queries == nil || w.sender == nil {
		return false, ErrLifecycleUnavailable
	}
	if _, err := w.queries.RetireStaleCloudDispatchDeliveries(ctx); err != nil {
		return false, fmt.Errorf("retire stale cloud dispatch deliveries: %w", err)
	}

	exhausted, err := w.queries.GetExhaustedCloudDispatchDelivery(ctx)
	if err == nil {
		message := "cloud dispatch was not acknowledged after maximum delivery attempts"
		if exhausted.LastError.Valid && exhausted.LastError.String != "" {
			message += ": " + exhausted.LastError.String
		}
		_, failErr := w.tasks.Lifecycle.FailDispatch(ctx, RunRef{
			WorkItemID: exhausted.WorkItemID,
			RunID:      exhausted.RunID,
		}, message)
		if failErr != nil && !errors.Is(failErr, ErrStaleRun) {
			return true, fmt.Errorf("fail exhausted cloud dispatch: %w", failErr)
		}
		return true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return false, fmt.Errorf("find exhausted cloud dispatch: %w", err)
	}

	delivery, err := w.queries.ClaimCloudDispatchDelivery(ctx)
	if err == nil {
		return true, w.deliver(ctx, delivery)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return false, fmt.Errorf("claim cloud dispatch delivery: %w", err)
	}

	claimed, err := w.scheduleNext(ctx)
	if err != nil {
		return false, err
	}
	return claimed, nil
}

func (w *CloudDispatchWorker) scheduleNext(ctx context.Context) (bool, error) {
	candidates, err := w.queries.ListQueuedCloudTasks(ctx)
	if err != nil {
		return false, fmt.Errorf("list queued cloud tasks: %w", err)
	}
	for _, candidate := range candidates {
		issue, err := w.queries.GetIssue(ctx, candidate.IssueID)
		if err != nil {
			continue
		}
		if w.sender.GetGatewayForWorkspace(util.UUIDToString(issue.WorkspaceID)) == "" {
			continue
		}
		claimed, err := w.tasks.ClaimCloudTask(ctx, candidate.ID)
		if err != nil {
			return false, err
		}
		if claimed != nil {
			return true, nil
		}
	}
	return false, nil
}

func (w *CloudDispatchWorker) deliver(ctx context.Context, delivery db.CloudDispatchDelivery) error {
	message, workspaceID, err := w.buildMessage(ctx, delivery)
	if err != nil {
		return w.recordFailure(ctx, delivery, err)
	}
	gatewayID := w.sender.GetGatewayForWorkspace(workspaceID)
	if gatewayID == "" {
		updated, err := w.queries.DeferCloudDispatchDelivery(ctx, db.DeferCloudDispatchDeliveryParams{
			RunID: delivery.RunID, LockToken: delivery.LockToken,
			LastError: pgtype.Text{String: "no cloud gateway connected", Valid: true},
		})
		if err != nil {
			return fmt.Errorf("defer cloud dispatch without gateway: %w", err)
		}
		if updated == 0 {
			return w.acceptAcknowledgementRace(ctx, delivery.RunID)
		}
		return nil
	}
	if err := w.sender.SendToGateway(gatewayID, message); err != nil {
		return w.recordFailure(ctx, delivery, fmt.Errorf("send to gateway %s: %w", gatewayID, err))
	}
	updated, err := w.queries.MarkCloudDispatchSent(ctx, db.MarkCloudDispatchSentParams{
		RunID: delivery.RunID, LockToken: delivery.LockToken,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return w.acceptAcknowledgementRace(ctx, delivery.RunID)
	}
	if err != nil {
		return fmt.Errorf("mark cloud dispatch sent: %w", err)
	}
	slog.Info("cloud task delivery sent",
		"task_id", util.UUIDToString(delivery.WorkItemID),
		"run_id", util.UUIDToString(delivery.RunID),
		"gateway_id", gatewayID,
		"attempt", updated.Attempts,
	)
	return nil
}

func (w *CloudDispatchWorker) buildMessage(ctx context.Context, delivery db.CloudDispatchDelivery) ([]byte, string, error) {
	task, err := w.queries.GetAgentTask(ctx, delivery.WorkItemID)
	if err != nil || task.Status != "dispatched" || task.ActiveRunID != delivery.RunID || task.CloudRuntimeID != delivery.CloudRuntimeID {
		return nil, "", fmt.Errorf("cloud delivery no longer matches active Run")
	}
	issue, err := w.queries.GetIssue(ctx, task.IssueID)
	if err != nil {
		return nil, "", fmt.Errorf("load cloud dispatch Issue: %w", err)
	}
	runtime, err := w.queries.GetCloudRuntimeByID(ctx, delivery.CloudRuntimeID)
	if err != nil || !runtime.IsActive || runtime.WorkspaceID != issue.WorkspaceID {
		return nil, "", fmt.Errorf("cloud runtime is missing, inactive, or outside the Work Item workspace")
	}
	agent, err := w.queries.GetAgent(ctx, task.AgentID)
	if err != nil {
		return nil, "", fmt.Errorf("load cloud dispatch Agent: %w", err)
	}
	workspaceID := util.UUIDToString(issue.WorkspaceID)
	apiKey, err := crypto.DecryptAPIKey(string(runtime.EncryptedApiKey), workspaceID+string(w.jwtSecret))
	if err != nil {
		return nil, "", fmt.Errorf("decrypt cloud runtime API key: %w", err)
	}
	config := map[string]any{
		"task_id":      util.UUIDToString(task.ID),
		"agent_id":     util.UUIDToString(task.AgentID),
		"issue_id":     util.UUIDToString(task.IssueID),
		"issue_title":  issue.Title,
		"agent_name":   agent.Name,
		"instructions": agent.Instructions,
		"skills":       w.tasks.LoadAgentSkills(ctx, task.AgentID),
		"api_key":      apiKey,
		"gateway_url":  runtime.GatewayUrl.String,
		"provider":     runtime.Provider,
	}
	message, err := json.Marshal(protocol.GatewayTaskDispatchMessage{
		Type: protocol.EventTaskDispatch, TaskID: util.UUIDToString(task.ID),
		RunID: util.UUIDToString(delivery.RunID), Config: config,
	})
	if err != nil {
		return nil, "", fmt.Errorf("marshal cloud dispatch: %w", err)
	}
	return message, workspaceID, nil
}

func (w *CloudDispatchWorker) recordFailure(ctx context.Context, delivery db.CloudDispatchDelivery, cause error) error {
	updated, err := w.queries.RecordCloudDispatchFailure(ctx, db.RecordCloudDispatchFailureParams{
		RunID: delivery.RunID, LockToken: delivery.LockToken,
		LastError: pgtype.Text{String: cause.Error(), Valid: true},
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return w.acceptAcknowledgementRace(ctx, delivery.RunID)
	}
	if err != nil {
		return fmt.Errorf("record cloud dispatch failure: %v: %w", cause, err)
	}
	if updated.Attempts >= cloudDispatchMaxAttempts {
		_, failErr := w.tasks.Lifecycle.FailDispatch(ctx, RunRef{
			WorkItemID: delivery.WorkItemID,
			RunID:      delivery.RunID,
		}, "cloud dispatch delivery exhausted: "+cause.Error())
		if failErr != nil && !errors.Is(failErr, ErrStaleRun) {
			return fmt.Errorf("fail exhausted cloud dispatch: %w", failErr)
		}
		return nil
	}
	return cause
}

func (w *CloudDispatchWorker) acceptAcknowledgementRace(ctx context.Context, runID pgtype.UUID) error {
	delivery, err := w.queries.GetCloudDispatchDelivery(ctx, runID)
	if err == nil && delivery.AcknowledgedAt.Valid {
		return nil
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("cloud dispatch lease was lost before acknowledgement")
}
