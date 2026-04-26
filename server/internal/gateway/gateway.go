package gateway

import (
	"context"
	"log/slog"
	"sync"
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
	TaskID      string
	ContainerID string
	CancelFunc  context.CancelFunc
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

	// Note: In real implementation, we would register callbacks for task dispatch
	// For now, the gateway handles task dispatch via wsClient

	if err := g.wsClient.Connect(ctx); err != nil {
		return err
	}

	g.logger.Info("gateway connected", "gatewayId", g.cfg.GatewayID)

	return g.wsClient.Run(ctx)
}