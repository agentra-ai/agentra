package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/agentra-ai/agentra/server/pkg/protocol"
	"github.com/gorilla/websocket"
)

type WSClient struct {
	serverURL   string
	gatewayID   string
	workspaceID string
	authToken   string
	conn        *websocket.Conn
	mu          sync.Mutex
	logger      *slog.Logger

	// Callbacks set by Gateway
	OnTaskDispatch func(taskID string, config map[string]any)
	OnTaskCancel   func(taskID string)
}

func NewWSClient(serverURL, gatewayID, workspaceID, authToken string, logger *slog.Logger) *WSClient {
	return &WSClient{
		serverURL:   serverURL,
		gatewayID:   gatewayID,
		workspaceID: workspaceID,
		authToken:   authToken,
		logger:      logger,
	}
}

func (c *WSClient) Connect(ctx context.Context) error {
	if strings.TrimSpace(c.gatewayID) == "" {
		return fmt.Errorf("gateway ID is required")
	}
	if strings.TrimSpace(c.workspaceID) == "" {
		return fmt.Errorf("gateway workspace ID is required")
	}
	if strings.TrimSpace(c.authToken) == "" {
		return fmt.Errorf("gateway auth token is required")
	}

	// Strip /ws suffix if present - AGENTRA_SERVER_URL may include it for daemon use
	baseURL := strings.TrimSuffix(c.serverURL, "/ws")
	endpoint, err := url.Parse(baseURL + "/api/gateway/connect")
	if err != nil {
		return fmt.Errorf("parse gateway server URL: %w", err)
	}
	query := endpoint.Query()
	query.Set("gateway_id", c.gatewayID)
	query.Set("workspace_id", c.workspaceID)
	endpoint.RawQuery = query.Encode()
	header := http.Header{"Authorization": []string{"Bearer " + c.authToken}}

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, endpoint.String(), header)
	if err != nil {
		return err
	}
	c.conn = conn
	return nil
}

func (c *WSClient) Run(ctx context.Context) error {
	if c.conn == nil {
		return fmt.Errorf("gateway websocket is not connected")
	}

	// ReadMessage blocks while the connection is idle. Closing the socket on
	// context cancellation makes shutdown prompt without requiring traffic from
	// the server.
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			_ = c.conn.Close()
		case <-done:
		}
	}()

	for {
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.logger.Error("websocket error", "error", err)
			}
			return err
		}

		if err := c.handleMessage(msg); err != nil {
			c.logger.Warn("gateway server message rejected", "error", err)
			continue
		}
	}
}

func (c *WSClient) handleMessage(message []byte) error {
	var envelope protocol.GatewayEnvelope
	if err := json.Unmarshal(message, &envelope); err != nil {
		return fmt.Errorf("decode envelope: %w", err)
	}

	switch envelope.Type {
	case protocol.EventTaskDispatch:
		var event protocol.GatewayTaskDispatchMessage
		if err := json.Unmarshal(message, &event); err != nil {
			return fmt.Errorf("decode task dispatch: %w", err)
		}
		if strings.TrimSpace(event.TaskID) == "" {
			return fmt.Errorf("task dispatch: task_id is required")
		}
		if c.OnTaskDispatch != nil {
			c.OnTaskDispatch(event.TaskID, event.Config)
		}
		return nil
	case protocol.EventTaskCancel:
		var event protocol.GatewayTaskCancelMessage
		if err := json.Unmarshal(message, &event); err != nil {
			return fmt.Errorf("decode task cancel: %w", err)
		}
		if strings.TrimSpace(event.TaskID) == "" {
			return fmt.Errorf("task cancel: task_id is required")
		}
		if c.OnTaskCancel != nil {
			c.OnTaskCancel(event.TaskID)
		}
		return nil
	case protocol.EventGatewayHeartbeat:
		return c.send(protocol.GatewayHeartbeatMessage{Type: protocol.EventGatewayHeartbeat})
	default:
		return fmt.Errorf("unsupported event type %q", envelope.Type)
	}
}

func (c *WSClient) SendTaskDispatched(taskID, containerID string) error {
	return c.send(protocol.GatewayTaskDispatchedMessage{
		Type:        protocol.EventTaskDispatched,
		TaskID:      taskID,
		ContainerID: containerID,
	})
}

func (c *WSClient) SendTaskLogs(taskID string, seq int, stream, content string) error {
	return c.send(protocol.GatewayTaskLogsMessage{
		Type:    protocol.EventTaskLogs,
		TaskID:  taskID,
		Seq:     seq,
		Stream:  stream,
		Content: content,
	})
}

func (c *WSClient) SendTaskCompleted(taskID string, exitCode int, output string) error {
	return c.send(protocol.GatewayTaskCompletedMessage{
		Type:     protocol.EventTaskCompleted,
		TaskID:   taskID,
		ExitCode: exitCode,
		Output:   output,
	})
}

func (c *WSClient) SendTaskFailed(taskID, errorMsg string) error {
	return c.SendTaskFailedWithRetry(taskID, errorMsg, true)
}

func (c *WSClient) SendTaskFailedWithRetry(taskID, errorMsg string, retryable bool) error {
	return c.send(protocol.GatewayTaskFailedMessage{
		Type:      protocol.EventTaskFailed,
		TaskID:    taskID,
		Error:     errorMsg,
		Retryable: retryable,
	})
}

func (c *WSClient) send(msg any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return fmt.Errorf("gateway websocket is not connected")
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	if err := c.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
		return err
	}
	return c.conn.WriteMessage(websocket.TextMessage, data)
}
