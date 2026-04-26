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
}

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

	// Create container
	containerID, err := g.containerMgr.CreateContainer(taskCtx, containerCfg)
	if err != nil {
		g.logger.Error("task dispatch: failed to create container", "task_id", taskID, "error", err)
		g.wsClient.SendTaskFailed(taskID, fmt.Sprintf("failed to create container: %v", err))
		g.tasks.Delete(taskID)
		return
	}
	runningTask.ContainerID = containerID

	g.logger.Info("container created", "task_id", taskID, "container_id", containerID)

	// Notify server that container is running
	if err := g.wsClient.SendTaskDispatched(taskID, containerID); err != nil {
		g.logger.Error("task dispatch: failed to send dispatched", "task_id", taskID, "error", err)
	}

	// Start container
	if err := g.containerMgr.StartContainer(taskCtx, containerID); err != nil {
		g.logger.Error("task dispatch: failed to start container", "task_id", taskID, "error", err)
		g.wsClient.SendTaskFailed(taskID, fmt.Sprintf("failed to start container: %v", err))
		g.tasks.Delete(taskID)
		return
	}

	// Run goroutine to wait for completion and clean up
	go func() {
		// Wait for container to finish
		exitCode, err := g.containerMgr.WaitContainer(taskCtx, containerID)
		if err != nil {
			g.logger.Error("task wait failed", "task_id", taskID, "error", err)
			g.wsClient.SendTaskFailed(taskID, fmt.Sprintf("wait failed: %v", err))
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
				g.logger.Info("task failed", "task_id", taskID, "exit_code", exitCode)
				g.wsClient.SendTaskFailed(taskID, output)
			}
		}

		// Clean up container
		if err := g.containerMgr.DestroyContainer(context.Background(), containerID); err != nil {
			g.logger.Error("task cleanup failed", "task_id", taskID, "error", err)
		}

		g.tasks.Delete(taskID)
	}()
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
