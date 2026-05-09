package taskgraph

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
)

type ContainerManager struct {
	docker      *client.Client
	image       string
	networkName string
}

func NewContainerManager(image, networkName string) (*ContainerManager, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, err
	}

	return &ContainerManager{
		docker:      cli,
		image:       image,
		networkName: networkName,
	}, nil
}

func (m *ContainerManager) Execute(ctx context.Context, node *GraphNode, prompt string) (*ExecutionResult, error) {
	// Create container
	resp, err := m.docker.ContainerCreate(ctx, &container.Config{
		Image: m.image,
		Cmd:   []string{"agent", "execute", "--prompt", prompt},
		Env: []string{
			fmt.Sprintf("AGENT_TYPE=%s", node.NodeType),
			fmt.Sprintf("NODE_ID=%s", node.ID),
		},
	}, nil, &network.NetworkingConfig{}, nil, "")
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %w", err)
	}

	defer m.docker.ContainerRemove(ctx, resp.ID, container.RemoveOptions{})

	// Wait for completion with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	statusCh, errCh := m.docker.ContainerWait(timeoutCtx, resp.ID, container.WaitConditionNotRunning)

	select {
	case result := <-statusCh:
		outReader, _ := m.docker.ContainerLogs(ctx, resp.ID, container.LogsOptions{ShowStdout: true})
		output, _ := io.ReadAll(outReader)

		return &ExecutionResult{
			ExitCode: int(result.StatusCode),
			Output:   string(output),
		}, nil
	case err := <-errCh:
		return nil, fmt.Errorf("container wait failed: %w", err)
	}
}

type ExecutionResult struct {
	ExitCode int
	Output   string
	Cost     float64
	Duration time.Duration
}
