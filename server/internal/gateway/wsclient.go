package gateway

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"

	"github.com/gorilla/websocket"
)

type WSClient struct {
	serverURL string
	gatewayID string
	authToken string
	conn      *websocket.Conn
	mu        sync.Mutex
	logger    *slog.Logger
}

func NewWSClient(serverURL, gatewayID, authToken string, logger *slog.Logger) *WSClient {
	return &WSClient{
		serverURL: serverURL,
		gatewayID: gatewayID,
		authToken: authToken,
		logger:    logger,
	}
}

func (c *WSClient) Connect(ctx context.Context) error {
	url := c.serverURL + "/api/gateway/connect?gateway_id=" + c.gatewayID
	if c.authToken != "" {
		url += "&token=" + c.authToken
	}

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, url, nil)
	if err != nil {
		return err
	}
	c.conn = conn

	c.send(map[string]any{
		"type":       "gateway:register",
		"gatewayId":  c.gatewayID,
	})

	return nil
}

func (c *WSClient) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			_, msg, err := c.conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					c.logger.Error("websocket error", "error", err)
				}
				return err
			}

			var event map[string]any
			if err := json.Unmarshal(msg, &event); err != nil {
				continue
			}

			c.handleEvent(event)
		}
	}
}

func (c *WSClient) handleEvent(event map[string]any) {
	switch event["type"] {
	case "task:dispatch":
		// Handled by gateway
	case "task:cancel":
		// Handled by gateway
	case "gateway:heartbeat":
		c.send(map[string]any{"type": "gateway:heartbeat"})
	}
}

func (c *WSClient) SendTaskDispatched(taskID, containerID string) error {
	return c.send(map[string]any{
		"type":        "task:dispatched",
		"taskId":      taskID,
		"containerId": containerID,
	})
}

func (c *WSClient) SendTaskLogs(taskID, logs string) error {
	return c.send(map[string]any{
		"type":   "task:logs",
		"taskId": taskID,
		"logs":   logs,
	})
}

func (c *WSClient) SendTaskCompleted(taskID string, exitCode int, output string) error {
	return c.send(map[string]any{
		"type":     "task:completed",
		"taskId":   taskID,
		"exitCode": exitCode,
		"output":   output,
	})
}

func (c *WSClient) SendTaskFailed(taskID, errorMsg string) error {
	return c.send(map[string]any{
		"type":   "task:failed",
		"taskId": taskID,
		"error":  errorMsg,
	})
}

func (c *WSClient) send(msg map[string]any) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return c.conn.WriteMessage(websocket.TextMessage, data)
}