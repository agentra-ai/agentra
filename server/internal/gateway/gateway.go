package gateway

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/agentra-ai/agentra/server/pkg/protocol"
	"github.com/agentra-ai/agentra/server/pkg/redact"
)

type Config struct {
	ServerURL   string
	GatewayID   string
	WorkspaceID string
	AuthToken   string
	DockerHost  string
	BaseImage   string
	StateDir    string
	MaxRetries  int
}

const (
	defaultMaxRetries = 3
	baseRetryDelay    = 1 * time.Second
)

type Gateway struct {
	cfg          Config
	logger       *slog.Logger
	containerMgr containerRuntime
	wsClient     *WSClient
	spool        *terminalSpool
	tasks        sync.Map
}

type RunningTask struct {
	mu           sync.RWMutex
	TaskID       string
	RunID        string
	ContainerID  string
	CancelFunc   context.CancelFunc
	APIKey       string
	Instructions string
	Provider     string
}

func (t *RunningTask) setContainerID(containerID string) {
	t.mu.Lock()
	t.ContainerID = containerID
	t.mu.Unlock()
}

func (t *RunningTask) containerID() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.ContainerID
}

func (g *Gateway) reserveTask(candidate *RunningTask) (*RunningTask, bool) {
	actual, loaded := g.tasks.LoadOrStore(candidate.TaskID, candidate)
	if !loaded {
		return candidate, true
	}
	return actual.(*RunningTask), false
}

func New(cfg Config, logger *slog.Logger) (*Gateway, error) {
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = defaultMaxRetries
	}
	if cfg.StateDir == "" {
		cfg.StateDir = "/var/lib/agentra-gateway"
	}
	cm, err := NewContainerManager(cfg.DockerHost, cfg.BaseImage)
	if err != nil {
		return nil, err
	}
	spool, err := newTerminalSpool(cfg.StateDir)
	if err != nil {
		return nil, err
	}
	return &Gateway{
		cfg:          cfg,
		logger:       logger,
		containerMgr: cm,
		spool:        spool,
	}, nil
}

func (g *Gateway) Run(ctx context.Context) error {
	g.wsClient = NewWSClient(g.cfg.ServerURL, g.cfg.GatewayID, g.cfg.WorkspaceID, g.cfg.AuthToken, g.logger)

	// Register task dispatch callback
	g.wsClient.OnTaskDispatch = func(taskID, runID string, config map[string]any) {
		g.handleTaskDispatch(taskID, runID, config)
	}

	// Register task cancel callback
	g.wsClient.OnTaskCancel = func(taskID string) {
		g.handleTaskCancel(taskID)
	}
	g.wsClient.OnDeliveryAck = func(eventID string) {
		if err := g.spool.Ack(eventID); err != nil {
			g.logger.Error("terminal delivery ack persistence failed", "event_id", eventID, "error", err)
		}
	}

	if err := g.wsClient.Connect(ctx); err != nil {
		return err
	}

	g.logger.Info("gateway connected", "gatewayId", g.cfg.GatewayID)
	if err := g.recoverManagedContainers(ctx); err != nil {
		return fmt.Errorf("recover managed containers: %w", err)
	}
	go g.replayTerminalLoop(ctx)
	return g.wsClient.Run(ctx)
}

func (g *Gateway) handleTaskDispatch(taskID, runID string, config map[string]any) {
	if g.containerMgr == nil {
		g.logger.Error("task dispatch: no container manager", "task_id", taskID)
		g.queueTaskFailed(taskID, runID, "gateway not configured", true)
		return
	}

	// Extract config
	apiKey, _ := config["api_key"].(string)
	instructions, _ := config["instructions"].(string)
	provider, _ := config["provider"].(string)
	if provider == "" {
		provider = "anthropic"
	}

	// Get skills if present
	var skills []string
	if skillsRaw, ok := config["skills"].([]any); ok {
		for _, s := range skillsRaw {
			if skillStr, ok := s.(string); ok {
				skills = append(skills, skillStr)
			}
		}
	}

	// Build instructions for the agent
	taskInstructions := instructions
	if len(skills) > 0 {
		taskInstructions = fmt.Sprintf("%s\n\nAvailable skills: %v", instructions, skills)
	}

	g.logger.Info("task dispatch received", "task_id", taskID, "provider", provider)

	// Create task context with cancellation
	taskCtx, cancelFunc := context.WithCancel(context.Background())

	// Store running task
	runningTask := &RunningTask{
		TaskID:       taskID,
		RunID:        runID,
		APIKey:       apiKey,
		Instructions: taskInstructions,
		Provider:     provider,
		CancelFunc:   cancelFunc,
	}
	existing, reserved := g.reserveTask(runningTask)
	if !reserved {
		cancelFunc()
		if existing.RunID != runID {
			g.logger.Warn("task dispatch ignored while another Run is active",
				"task_id", taskID, "active_run_id", existing.RunID, "run_id", runID)
			return
		}
		if containerID := existing.containerID(); containerID != "" {
			if err := g.wsClient.SendTaskDispatched(taskID, runID, containerID); err != nil {
				g.logger.Error("task dispatch: failed to repeat acknowledgement", "task_id", taskID, "error", err)
			}
		}
		return
	}

	// Prepare container config
	containerCfg := &TaskConfig{
		TaskID:        taskID,
		RunID:         runID,
		GatewayID:     g.cfg.GatewayID,
		WorkspaceID:   g.cfg.WorkspaceID,
		APIKey:        apiKey,
		MemoryLimitMB: 512,
		CPULimit:      1,
		Env: []string{
			fmt.Sprintf("AGENTRA_INSTRUCTIONS=%s", taskInstructions),
			fmt.Sprintf("AGENTRA_PROVIDER=%s", provider),
		},
	}

	// Acquire by durable Run identity before creating. This covers a Gateway
	// crash and the narrower case where Docker created the container but the
	// create response was lost.
	container, err := g.acquireContainerWithRetry(taskCtx, containerCfg, taskID)
	if err != nil {
		g.logger.Error("task dispatch: container creation failed after retries", "task_id", taskID, "error", err)
		if reportErr := g.queueTaskFailed(taskID, runID, fmt.Sprintf("failed to acquire container after %d attempts: %v", g.cfg.MaxRetries, err), true); reportErr != nil {
			g.logger.Error("task dispatch: failed to report container acquisition failure", "task_id", taskID, "error", reportErr)
		}
		g.tasks.CompareAndDelete(taskID, runningTask)
		cancelFunc()
		return
	}
	containerID := container.ID
	runningTask.setContainerID(containerID)

	g.logger.Info("container acquired", "task_id", taskID, "container_id", containerID, "state", container.State)

	// Only a newly created or previously unstarted container needs Start. A
	// running/exited container is adopted and monitored without a second Start.
	if container.State == "created" {
		if err := g.startContainerWithRetry(taskCtx, containerID, taskID); err != nil {
			g.logger.Error("task dispatch: container start failed after retries", "task_id", taskID, "error", err)
			// Destroy the created container before reporting failure
			if destroyErr := g.containerMgr.DestroyContainer(context.Background(), containerID); destroyErr != nil {
				g.logger.Error("task dispatch: failed to destroy container after start failure", "task_id", taskID, "error", destroyErr)
			}
			if reportErr := g.queueTaskFailed(taskID, runID, fmt.Sprintf("failed to start container after %d attempts: %v", g.cfg.MaxRetries, err), true); reportErr != nil {
				g.logger.Error("task dispatch: failed to report container start failure", "task_id", taskID, "error", reportErr)
			}
			g.tasks.CompareAndDelete(taskID, runningTask)
			cancelFunc()
			return
		}
	} else if container.State != "running" && container.State != "exited" {
		err := fmt.Errorf("managed container is in unsupported state %q", container.State)
		g.logger.Error("task dispatch: container cannot be adopted", "task_id", taskID, "container_id", containerID, "error", err)
		if destroyErr := g.containerMgr.DestroyContainer(context.Background(), containerID); destroyErr != nil {
			g.logger.Error("task dispatch: failed to destroy unsupported container", "task_id", taskID, "container_id", containerID, "error", destroyErr)
		}
		if reportErr := g.queueTaskFailed(taskID, runID, err.Error(), true); reportErr != nil {
			g.logger.Error("task dispatch: failed to report container adoption failure", "task_id", taskID, "error", reportErr)
		}
		g.tasks.CompareAndDelete(taskID, runningTask)
		cancelFunc()
		return
	}

	// The task is running only after Docker confirms a successful start.
	if err := g.wsClient.SendTaskDispatched(taskID, runID, containerID); err != nil {
		g.logger.Error("task dispatch: failed to send dispatched", "task_id", taskID, "error", err)
	}

	g.monitorTask(taskCtx, runningTask)
}

func (g *Gateway) monitorTask(taskCtx context.Context, runningTask *RunningTask) {
	go func() {
		defer runningTask.CancelFunc()
		taskID, runID := runningTask.TaskID, runningTask.RunID
		containerID := runningTask.containerID()
		logCtx, stopLogs := context.WithCancel(taskCtx)
		defer stopLogs()
		tail := newBoundedTailBuffer(protocol.GatewayTaskResultBytes)
		emitter := &taskLogEmitter{
			sender: g.wsClient,
			taskID: taskID,
			runID:  runID,
			tail:   tail,
		}
		logsDone := make(chan error, 1)
		go func() {
			logsDone <- g.containerMgr.StreamContainerLogs(
				logCtx,
				containerID,
				emitter.writer(protocol.GatewayStreamStdout),
				emitter.writer(protocol.GatewayStreamStderr),
			)
		}()

		// Wait for container to finish
		exitCode, err := g.containerMgr.WaitContainer(taskCtx, containerID)
		if err != nil {
			stopLogs()
			g.logger.Error("task wait failed", "task_id", taskID, "error", err)
			// Container wait failure is retryable (container may be hung)
			if sendErr := g.queueTaskFailed(taskID, runID, fmt.Sprintf("wait failed: %v", err), true); sendErr != nil {
				g.logger.Error("task wait failed: report failed", "task_id", taskID, "error", sendErr)
			}
		} else {
			// Docker's follow stream normally closes immediately after container
			// exit. Bound the drain so a broken Docker connection cannot stall task
			// completion forever.
			select {
			case logErr := <-logsDone:
				if logErr != nil && !errors.Is(logErr, context.Canceled) {
					g.logger.Error("task log stream failed", "task_id", taskID, "error", logErr)
				}
			case <-time.After(5 * time.Second):
				stopLogs()
				g.logger.Warn("task log stream drain timed out", "task_id", taskID)
			}

			output := tail.String()
			if exitCode == 0 {
				g.logger.Info("task completed", "task_id", taskID, "exit_code", exitCode)
				if sendErr := g.queueTaskCompleted(taskID, runID, exitCode, output); sendErr != nil {
					g.logger.Error("task completed: report failed", "task_id", taskID, "error", sendErr)
				}
			} else {
				// Agent exit code != 0 is not retryable (agent code failed)
				g.logger.Info("task failed", "task_id", taskID, "exit_code", exitCode)
				if sendErr := g.queueTaskFailed(taskID, runID, output, false); sendErr != nil {
					g.logger.Error("task failed: report failed", "task_id", taskID, "error", sendErr)
				}
			}
		}

		// Clean up container
		if err := g.containerMgr.DestroyContainer(context.Background(), containerID); err != nil {
			g.logger.Error("task cleanup failed", "task_id", taskID, "error", err)
		}

		g.tasks.CompareAndDelete(taskID, runningTask)
	}()
}

func (g *Gateway) recoverManagedContainers(ctx context.Context) error {
	containers, err := g.containerMgr.ListManagedContainers(ctx, g.cfg.GatewayID, g.cfg.WorkspaceID)
	if err != nil {
		return err
	}
	seenTasks := make(map[string]ManagedContainer, len(containers))
	for _, container := range containers {
		if previous, exists := seenTasks[container.TaskID]; exists {
			return fmt.Errorf("multiple managed containers for task %s (Runs %s and %s)", container.TaskID, previous.RunID, container.RunID)
		}
		seenTasks[container.TaskID] = container
	}
	for _, container := range containers {
		taskCtx, cancel := context.WithCancel(ctx)
		runningTask := &RunningTask{TaskID: container.TaskID, RunID: container.RunID, CancelFunc: cancel}
		runningTask.setContainerID(container.ID)
		existing, reserved := g.reserveTask(runningTask)
		if !reserved {
			cancel()
			if existing.RunID != container.RunID {
				g.logger.Warn("managed container ignored because another Run is reserved",
					"task_id", container.TaskID, "run_id", container.RunID, "active_run_id", existing.RunID)
			}
			continue
		}
		if container.State == "created" {
			if err := g.startContainerWithRetry(taskCtx, container.ID, container.TaskID); err != nil {
				if destroyErr := g.containerMgr.DestroyContainer(context.Background(), container.ID); destroyErr != nil {
					g.logger.Error("failed to destroy recovered container after start failure", "task_id", container.TaskID, "container_id", container.ID, "error", destroyErr)
				}
				if reportErr := g.queueTaskFailed(container.TaskID, container.RunID, "failed to start recovered container: "+err.Error(), true); reportErr != nil {
					g.logger.Error("recovered container start failure report failed", "task_id", container.TaskID, "error", reportErr)
				}
				g.tasks.CompareAndDelete(container.TaskID, runningTask)
				cancel()
				continue
			}
		} else if container.State != "running" && container.State != "exited" {
			if destroyErr := g.containerMgr.DestroyContainer(context.Background(), container.ID); destroyErr != nil {
				g.logger.Error("failed to destroy recovered container in unsupported state", "task_id", container.TaskID, "container_id", container.ID, "error", destroyErr)
			}
			if reportErr := g.queueTaskFailed(container.TaskID, container.RunID, fmt.Sprintf("recovered container is in unsupported state %q", container.State), true); reportErr != nil {
				g.logger.Error("recovered container state failure report failed", "task_id", container.TaskID, "error", reportErr)
			}
			g.tasks.CompareAndDelete(container.TaskID, runningTask)
			cancel()
			continue
		}
		if err := g.wsClient.SendTaskDispatched(container.TaskID, container.RunID, container.ID); err != nil {
			g.logger.Warn("recovered container acknowledgement failed", "task_id", container.TaskID, "run_id", container.RunID, "error", err)
		}
		g.logger.Info("managed container recovered", "task_id", container.TaskID, "run_id", container.RunID, "container_id", container.ID, "state", container.State)
		g.monitorTask(taskCtx, runningTask)
	}
	return nil
}

func (g *Gateway) queueTaskCompleted(taskID, runID string, exitCode int, output string) error {
	message := protocol.GatewayTaskCompletedMessage{
		Type: protocol.EventTaskCompleted, EventID: runID, TaskID: taskID,
		RunID: runID, ExitCode: exitCode, Output: redact.Text(output),
	}
	return g.queueTerminal(runID, message)
}

func (g *Gateway) queueTaskFailed(taskID, runID, errorMessage string, retryable bool) error {
	message := protocol.GatewayTaskFailedMessage{
		Type: protocol.EventTaskFailed, EventID: runID, TaskID: taskID,
		RunID: runID, Error: redact.Text(errorMessage), Retryable: retryable,
	}
	return g.queueTerminal(runID, message)
}

func (g *Gateway) queueTerminal(eventID string, message any) error {
	if err := g.spool.Put(eventID, message); err != nil {
		return fmt.Errorf("persist terminal delivery: %w", err)
	}
	return g.wsClient.send(message)
}

func (g *Gateway) replayTerminalLoop(ctx context.Context) {
	g.replayTerminalDeliveries()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			g.replayTerminalDeliveries()
		}
	}
}

func (g *Gateway) replayTerminalDeliveries() {
	deliveries, err := g.spool.List()
	if err != nil {
		g.logger.Error("list terminal delivery spool failed", "error", err)
		return
	}
	for _, delivery := range deliveries {
		if err := g.wsClient.send(delivery.Message); err != nil {
			g.logger.Warn("terminal delivery replay failed", "event_id", delivery.EventID, "error", err)
			return
		}
	}
}

// acquireContainerWithRetry adopts the exact labeled Run before attempting a
// create. Rechecking on every attempt prevents an ambiguous Docker response
// from producing a second container.
func (g *Gateway) acquireContainerWithRetry(ctx context.Context, cfg *TaskConfig, taskID string) (ManagedContainer, error) {
	var lastErr error
	for attempt := 0; attempt < g.cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := baseRetryDelay * time.Duration(1<<(attempt-1)) // 1s, 2s, 4s...
			g.logger.Info("retrying container creation", "task_id", taskID, "attempt", attempt+1, "delay", delay)
			select {
			case <-ctx.Done():
				return ManagedContainer{}, ctx.Err()
			case <-time.After(delay):
			}
		}

		existing, found, findErr := g.containerMgr.FindContainerForRun(
			ctx, cfg.GatewayID, cfg.WorkspaceID, cfg.TaskID, cfg.RunID,
		)
		if findErr != nil {
			lastErr = findErr
			g.logger.Warn("container discovery attempt failed", "task_id", taskID, "attempt", attempt+1, "error", findErr)
			continue
		}
		if found {
			return existing, nil
		}

		containerID, err := g.containerMgr.CreateContainer(ctx, cfg)
		if err == nil {
			return ManagedContainer{ID: containerID, State: "created", TaskID: cfg.TaskID, RunID: cfg.RunID}, nil
		}
		lastErr = err
		g.logger.Warn("container creation attempt failed", "task_id", taskID, "attempt", attempt+1, "error", err)
	}
	return ManagedContainer{}, lastErr
}

// startContainerWithRetry attempts to start a container with exponential backoff.
func (g *Gateway) startContainerWithRetry(ctx context.Context, containerID, taskID string) error {
	var lastErr error
	for attempt := 0; attempt < g.cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := baseRetryDelay * time.Duration(1<<(attempt-1)) // 1s, 2s, 4s...
			g.logger.Info("retrying container start", "task_id", taskID, "attempt", attempt+1, "delay", delay)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		if err := g.containerMgr.StartContainer(ctx, containerID); err == nil {
			return nil
		} else {
			lastErr = err
			g.logger.Warn("container start attempt failed", "task_id", taskID, "attempt", attempt+1, "error", err)
		}
	}
	return lastErr
}

func (g *Gateway) handleTaskCancel(taskID string) {
	g.logger.Info("task cancel received", "task_id", taskID)

	if task, ok := g.tasks.Load(taskID); ok {
		rt := task.(*RunningTask)
		if rt.CancelFunc != nil {
			rt.CancelFunc()
		}
		if containerID := rt.containerID(); containerID != "" && g.containerMgr != nil {
			if err := g.containerMgr.DestroyContainer(context.Background(), containerID); err != nil {
				g.logger.Error("task cancel: failed to destroy container", "task_id", taskID, "error", err)
			}
		}
		g.tasks.CompareAndDelete(taskID, rt)
	}
}

// handleGatewayEvent processes events from the server (currently unused - events come through wsClient callbacks)
func (g *Gateway) handleGatewayEvent(event map[string]any) {
	g.logger.Debug("gateway event", "type", event["type"])
}

// runningTaskToMap converts a RunningTask to a map for JSON serialization
func runningTaskToMap(rt *RunningTask) map[string]any {
	return map[string]any{
		"task_id":      rt.TaskID,
		"container_id": rt.containerID(),
	}
}

// ListRunningTasks returns all currently running tasks (for debugging/admin)
func (g *Gateway) ListRunningTasks() []map[string]any {
	var tasks []map[string]any
	g.tasks.Range(func(key, value any) bool {
		rt := value.(*RunningTask)
		tasks = append(tasks, runningTaskToMap(rt))
		return true
	})
	return tasks
}
