package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

type Config struct {
	ServerURL   string
	GatewayID   string
	WorkspaceID string
	AuthToken   string
	DockerHost  string
	BaseImage   string
	MaxRetries  int
}

const (
	defaultMaxRetries = 3
	baseRetryDelay    = 1 * time.Second
)

type Gateway struct {
	cfg          Config
	logger       *slog.Logger
	containerMgr *ContainerManager
	wsClient    *WSClient
	tasks       sync.Map
}

type RunningTask struct {
	TaskID       string
	ContainerID  string
	CancelFunc   context.CancelFunc
	APIKey       string
	Instructions string
	Provider     string
}

func New(cfg Config, logger *slog.Logger) *Gateway {
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = defaultMaxRetries
	}
	cm, _ := NewContainerManager(cfg.DockerHost, cfg.BaseImage)
	return &Gateway{
		cfg:          cfg,
		logger:       logger,
		containerMgr: cm,
	}
}

func (g *Gateway) Run(ctx context.Context) error {
	g.wsClient = NewWSClient(g.cfg.ServerURL, g.cfg.GatewayID, g.cfg.AuthToken, g.logger)

	// Register task dispatch callback
	g.wsClient.OnTaskDispatch = func(taskID string, config map[string]any) {
		g.handleTaskDispatch(taskID, config)
	}

	// Register task cancel callback
	g.wsClient.OnTaskCancel = func(taskID string) {
		g.handleTaskCancel(taskID)
	}

	if err := g.wsClient.Connect(ctx); err != nil {
		return err
	}

	g.logger.Info("gateway connected", "gatewayId", g.cfg.GatewayID)

	return g.wsClient.Run(ctx)
}

func (g *Gateway) handleTaskDispatch(taskID string, config map[string]any) {
	if g.containerMgr == nil {
		g.logger.Error("task dispatch: no container manager", "task_id", taskID)
		g.wsClient.SendTaskFailed(taskID, "gateway not configured")
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
		APIKey:       apiKey,
		Instructions: taskInstructions,
		Provider:     provider,
		CancelFunc:   cancelFunc,
	}
	g.tasks.Store(taskID, runningTask)

	// Prepare container config
	containerCfg := &TaskConfig{
		TaskID:        taskID,
		APIKey:        apiKey,
		MemoryLimitMB: 512,
		CPULimit:      1,
		Env: []string{
			fmt.Sprintf("AGENTRA_INSTRUCTIONS=%s", taskInstructions),
			fmt.Sprintf("AGENTRA_PROVIDER=%s", provider),
		},
	}

	// Create container with retry
	containerID, err := g.createContainerWithRetry(taskCtx, containerCfg, taskID)
	if err != nil {
		g.logger.Error("task dispatch: container creation failed after retries", "task_id", taskID, "error", err)
		g.wsClient.SendTaskFailedWithRetry(taskID, fmt.Sprintf("failed to create container after %d attempts: %v", g.cfg.MaxRetries, err), false)
		g.tasks.Delete(taskID)
		return
	}
	runningTask.ContainerID = containerID

	g.logger.Info("container created", "task_id", taskID, "container_id", containerID)

	// Notify server that container is running
	if err := g.wsClient.SendTaskDispatched(taskID, containerID); err != nil {
		g.logger.Error("task dispatch: failed to send dispatched", "task_id", taskID, "error", err)
	}

	// Start container with retry
	if err := g.startContainerWithRetry(taskCtx, containerID, taskID); err != nil {
		g.logger.Error("task dispatch: container start failed after retries", "task_id", taskID, "error", err)
		// Destroy the created container before reporting failure
		if destroyErr := g.containerMgr.DestroyContainer(context.Background(), containerID); destroyErr != nil {
			g.logger.Error("task dispatch: failed to destroy container after start failure", "task_id", taskID, "error", destroyErr)
		}
		g.wsClient.SendTaskFailedWithRetry(taskID, fmt.Sprintf("failed to start container after %d attempts: %v", g.cfg.MaxRetries, err), false)
		g.tasks.Delete(taskID)
		return
	}

	// Run goroutine to wait for completion and clean up
	go func() {
		// Wait for container to finish
		exitCode, err := g.containerMgr.WaitContainer(taskCtx, containerID)
		if err != nil {
			g.logger.Error("task wait failed", "task_id", taskID, "error", err)
			// Container wait failure is retryable (container may be hung)
			g.wsClient.SendTaskFailedWithRetry(taskID, fmt.Sprintf("wait failed: %v", err), true)
		} else {
			// Get logs
			logs, err := g.containerMgr.GetContainerLogs(context.Background(), containerID, time.Time{})
			if err != nil {
				g.logger.Error("task logs failed", "task_id", taskID, "error", err)
			}

			output := string(logs)
			if exitCode == 0 {
				g.logger.Info("task completed", "task_id", taskID, "exit_code", exitCode)
				g.wsClient.SendTaskCompleted(taskID, exitCode, output)
			} else {
				// Agent exit code != 0 is not retryable (agent code failed)
				g.logger.Info("task failed", "task_id", taskID, "exit_code", exitCode)
				g.wsClient.SendTaskFailedWithRetry(taskID, output, false)
			}
		}

		// Clean up container
		if err := g.containerMgr.DestroyContainer(context.Background(), containerID); err != nil {
			g.logger.Error("task cleanup failed", "task_id", taskID, "error", err)
		}

		g.tasks.Delete(taskID)
	}()
}

// createContainerWithRetry attempts to create a container with exponential backoff.
func (g *Gateway) createContainerWithRetry(ctx context.Context, cfg *TaskConfig, taskID string) (string, error) {
	var lastErr error
	for attempt := 0; attempt < g.cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := baseRetryDelay * time.Duration(1<<(attempt-1)) // 1s, 2s, 4s...
			g.logger.Info("retrying container creation", "task_id", taskID, "attempt", attempt+1, "delay", delay)
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(delay):
			}
		}

		containerID, err := g.containerMgr.CreateContainer(ctx, cfg)
		if err == nil {
			return containerID, nil
		}
		lastErr = err
		g.logger.Warn("container creation attempt failed", "task_id", taskID, "attempt", attempt+1, "error", err)
	}
	return "", lastErr
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
		if rt.ContainerID != "" && g.containerMgr != nil {
			if err := g.containerMgr.DestroyContainer(context.Background(), rt.ContainerID); err != nil {
				g.logger.Error("task cancel: failed to destroy container", "task_id", taskID, "error", err)
			}
		}
		g.tasks.Delete(taskID)
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
		"container_id": rt.ContainerID,
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
