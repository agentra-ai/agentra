package gateway

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/fsouza/go-dockerclient"
)

// ContainerManager handles Docker container lifecycle
type ContainerManager struct {
	docker  *docker.Client
	baseImg string
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
	APIKey        string
	ProxyURL      string
	MemoryLimitMB int
	CPULimit      int
	Env           []string
}

func (cm *ContainerManager) CreateContainer(ctx context.Context, cfg *TaskConfig) (string, error) {
	env := []string{
		fmt.Sprintf("AGENTRA_TASK_ID=%s", cfg.TaskID),
		fmt.Sprintf("ANTHROPIC_API_KEY=%s", cfg.APIKey),
	}
	for _, e := range cfg.Env {
		env = append(env, e)
	}

	containerCfg := docker.Config{
		Image: cm.baseImg,
		Env:   env,
		Cmd:   []string{"agentra", "agent", "run"},
	}

	hostCfg := docker.HostConfig{
		NetworkMode: "bridge",
		Memory:      int64(cfg.MemoryLimitMB) * 1024 * 1024,
	}

	resp, err := cm.docker.CreateContainer(docker.CreateContainerOptions{
		Name: "",
		Config: &containerCfg,
		HostConfig: &hostCfg,
	})
	if err != nil {
		return "", fmt.Errorf("create container: %w", err)
	}
	return resp.ID, nil
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

func (cm *ContainerManager) DestroyContainer(ctx context.Context, containerID string) error {
	return cm.docker.RemoveContainer(docker.RemoveContainerOptions{
		ID:    containerID,
		Force: true,
	})
}