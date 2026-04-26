package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/agentra-ai/agentra/server/internal/gateway"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg := gateway.Config{
		ServerURL:   getEnv("AGENTRA_SERVER_URL", "ws://localhost:8080/ws"),
		GatewayID:   getEnv("GATEWAY_ID", "gateway-1"),
		WorkspaceID: getEnv("GATEWAY_WORKSPACE_ID", ""),
		AuthToken:   getEnv("AGENTRA_AUTH_TOKEN", ""),
		DockerHost:  getEnv("DOCKER_HOST", "unix:///var/run/docker.sock"),
		BaseImage:   getEnv("BASE_IMAGE", "agentra/agent-runtime:latest"),
	}

	g := gateway.New(cfg, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		logger.Info("shutting down gateway")
		cancel()
	}()

	if err := g.Run(ctx); err != nil {
		logger.Error("gateway error", "error", err)
		os.Exit(1)
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}