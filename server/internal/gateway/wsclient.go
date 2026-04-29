package gateway

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
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

	// Callbacks set by Gateway
	OnTaskDispatch func(taskID string, config map[string]any)
	OnTaskCancel   func(taskID string)
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
	// Strip /ws suffix if present - AGENTRA_SERVER_URL may include it for daemon use
	baseURL := strings.TrimSuffix(c.serverURL, "/ws")
	url := baseURL + "/api/gateway/connect?gateway_id=" + c.gatewayID
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
		taskID, _ := event["task_id"].(string)
		config, _ := event["config"].(map[string]any)
		if c.OnTaskDispatch != nil && taskID != "" {
			c.OnTaskDispatch(taskID, config)
		}
	case "task:cancel":
		taskID, _ := event["task_id"].(string)
		if c.OnTaskCancel != nil && taskID != "" {
			c.OnTaskCancel(taskID)
		}
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
	return c.SendTaskFailedWithRetry(taskID, errorMsg, true)
}

func (c *WSClient) SendTaskFailedWithRetry(taskID, errorMsg string, retryable bool) error {
	return c.send(map[string]any{
		"type":     "task:failed",
		"taskId":   taskID,
		"error":    errorMsg,
		"retryable": retryable,
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