package gateway

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/fsouza/go-dockerclient"
)

const (
	containerLabelManaged   = "ai.agentra.managed"
	containerLabelGateway   = "ai.agentra.gateway_id"
	containerLabelWorkspace = "ai.agentra.workspace_id"
	containerLabelTask      = "ai.agentra.task_id"
	containerLabelRun       = "ai.agentra.run_id"
)

// ContainerManager handles Docker container lifecycle
type ContainerManager struct {
	docker  *docker.Client
	baseImg string
}

type containerRuntime interface {
	CreateContainer(context.Context, *TaskConfig) (string, error)
	ListManagedContainers(context.Context, string, string) ([]ManagedContainer, error)
	FindContainerForRun(context.Context, string, string, string, string) (ManagedContainer, bool, error)
	StartContainer(context.Context, string) error
	WaitContainer(context.Context, string) (int, error)
	StreamContainerLogs(context.Context, string, io.Writer, io.Writer) error
	DestroyContainer(context.Context, string) error
}

func NewContainerManager(dockerHost, baseImage string) (*ContainerManager, error) {
	cli, err := docker.NewClient(dockerHost)
	if err != nil {
		return nil, fmt.Errorf("create docker client: %w", err)
	}
	return &ContainerManager{
		docker:  cli,
		baseImg: baseImage,
	}, nil
}

type TaskConfig struct {
	TaskID        string
	RunID         string
	GatewayID     string
	WorkspaceID   string
	APIKey        string
	ProxyURL      string
	MemoryLimitMB int
	CPULimit      int
	Env           []string
}

func (cm *ContainerManager) CreateContainer(ctx context.Context, cfg *TaskConfig) (string, error) {
	env := []string{
		fmt.Sprintf("AGENTRA_TASK_ID=%s", cfg.TaskID),
		fmt.Sprintf("AGENTRA_RUN_ID=%s", cfg.RunID),
		fmt.Sprintf("ANTHROPIC_API_KEY=%s", cfg.APIKey),
	}
	for _, e := range cfg.Env {
		env = append(env, e)
	}

	containerCfg := docker.Config{
		Image:  cm.baseImg,
		Env:    env,
		Cmd:    []string{"agentra", "agent", "run"},
		Labels: managedContainerLabels(cfg),
	}

	hostCfg := docker.HostConfig{
		NetworkMode: "bridge",
		Memory:      int64(cfg.MemoryLimitMB) * 1024 * 1024,
	}

	resp, err := cm.docker.CreateContainer(docker.CreateContainerOptions{
		Name:       "",
		Config:     &containerCfg,
		HostConfig: &hostCfg,
	})
	if err != nil {
		return "", fmt.Errorf("create container: %w", err)
	}
	return resp.ID, nil
}

func managedContainerLabels(cfg *TaskConfig) map[string]string {
	return map[string]string{
		containerLabelManaged:   "true",
		containerLabelGateway:   cfg.GatewayID,
		containerLabelWorkspace: cfg.WorkspaceID,
		containerLabelTask:      cfg.TaskID,
		containerLabelRun:       cfg.RunID,
	}
}

type ManagedContainer struct {
	ID     string
	State  string
	TaskID string
	RunID  string
}

func (cm *ContainerManager) ListManagedContainers(ctx context.Context, gatewayID, workspaceID string) ([]ManagedContainer, error) {
	containers, err := cm.docker.ListContainers(docker.ListContainersOptions{
		All: true, Context: ctx,
		Filters: map[string][]string{"label": {
			containerLabelManaged + "=true",
			containerLabelGateway + "=" + gatewayID,
			containerLabelWorkspace + "=" + workspaceID,
		}},
	})
	if err != nil {
		return nil, fmt.Errorf("list managed containers: %w", err)
	}
	result := make([]ManagedContainer, 0, len(containers))
	for _, container := range containers {
		taskID, runID := container.Labels[containerLabelTask], container.Labels[containerLabelRun]
		if taskID == "" || runID == "" {
			continue
		}
		result = append(result, ManagedContainer{ID: container.ID, State: container.State, TaskID: taskID, RunID: runID})
	}
	return result, nil
}

func (cm *ContainerManager) FindContainerForRun(ctx context.Context, gatewayID, workspaceID, taskID, runID string) (ManagedContainer, bool, error) {
	containers, err := cm.ListManagedContainers(ctx, gatewayID, workspaceID)
	if err != nil {
		return ManagedContainer{}, false, err
	}
	return findContainerForRun(containers, taskID, runID)
}

func findContainerForRun(containers []ManagedContainer, taskID, runID string) (ManagedContainer, bool, error) {
	var found ManagedContainer
	for _, container := range containers {
		if container.TaskID != taskID {
			continue
		}
		if container.RunID != runID {
			return ManagedContainer{}, false, fmt.Errorf("managed container for task %s belongs to another Run %s", taskID, container.RunID)
		}
		if found.ID != "" {
			return ManagedContainer{}, false, fmt.Errorf("multiple managed containers for task %s Run %s", taskID, runID)
		}
		found = container
	}
	return found, found.ID != "", nil
}

func (cm *ContainerManager) StartContainer(ctx context.Context, containerID string) error {
	return cm.docker.StartContainer(containerID, nil)
}

func (cm *ContainerManager) WaitContainer(ctx context.Context, containerID string) (int, error) {
	statusCh := make(chan int, 1)
	errCh := make(chan error, 1)

	go func() {
		status, err := cm.docker.WaitContainer(containerID)
		if err != nil {
			errCh <- err
			return
		}
		statusCh <- status
	}()

	select {
	case status := <-statusCh:
		return status, nil
	case err := <-errCh:
		return -1, err
	}
}

func (cm *ContainerManager) GetContainerLogs(ctx context.Context, containerID string, since time.Time) ([]byte, error) {
	var stdout, stderr bytes.Buffer

	err := cm.docker.Logs(docker.LogsOptions{
		Context:      ctx,
		Container:    containerID,
		OutputStream: &stdout,
		ErrorStream:  &stderr,
		Stdout:       true,
		Stderr:       true,
		Since:        since.Unix(),
	})
	if err != nil {
		return nil, err
	}

	result := stdout.Bytes()
	result = append(result, stderr.Bytes()...)
	return result, nil
}

// StreamContainerLogs follows a container until it exits or ctx is cancelled.
// Docker demultiplexes stdout and stderr into the supplied bounded writers.
func (cm *ContainerManager) StreamContainerLogs(ctx context.Context, containerID string, stdout, stderr io.Writer) error {
	return cm.docker.Logs(docker.LogsOptions{
		Context:      ctx,
		Container:    containerID,
		OutputStream: stdout,
		ErrorStream:  stderr,
		Follow:       true,
		Stdout:       true,
		Stderr:       true,
	})
}

func (cm *ContainerManager) DestroyContainer(ctx context.Context, containerID string) error {
	return cm.docker.RemoveContainer(docker.RemoveContainerOptions{
		ID:    containerID,
		Force: true,
	})
}
