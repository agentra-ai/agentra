package gateway

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/agentra-ai/agentra/server/pkg/protocol"
	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512 * 1024
)

// Client manages a WebSocket connection to a Cloud Runtime Gateway
type Client struct {
	ID          string
	WorkspaceID string
	Conn        *websocket.Conn
	Hub         *Hub
	Send        chan []byte
}

// Hub manages all gateway connections
type Hub struct {
	mu        sync.RWMutex
	clients   map[string]*Client // gatewayID -> Client
	workspace map[string]string  // workspaceID -> gatewayID

	// Gateway-to-server task lifecycle callbacks.
	OnTaskDispatched func(gatewayID, workspaceID, taskID, runID, containerID string)
	OnTaskComplete   func(gatewayID, workspaceID, taskID, runID string, exitCode int, output string)
	OnTaskFail       func(gatewayID, workspaceID, taskID, runID string, error string, retryable bool)
	OnTaskLogs       func(gatewayID, workspaceID, taskID, runID string, seq int, stream, content string)
}

func NewHub() *Hub {
	return &Hub{
		clients:   make(map[string]*Client),
		workspace: make(map[string]string),
	}
}

// Register registers a gateway connection
func (h *Hub) Register(client *Client) {
	h.mu.Lock()

	// Exactly one live socket may own a gateway ID or workspace. Close the old
	// socket before publishing the replacement; Unregister uses client identity
	// so a late cleanup from the old socket cannot remove the new connection.
	var replaced []*Client
	if existing := h.clients[client.ID]; existing != nil && existing != client {
		replaced = append(replaced, existing)
	}
	if existingID := h.workspace[client.WorkspaceID]; existingID != "" && existingID != client.ID {
		if existing := h.clients[existingID]; existing != nil {
			replaced = append(replaced, existing)
			delete(h.clients, existingID)
		}
	}
	h.clients[client.ID] = client
	h.workspace[client.WorkspaceID] = client.ID
	h.mu.Unlock()

	for _, existing := range replaced {
		_ = existing.Conn.Close()
	}
}

// Unregister removes a gateway connection
func (h *Hub) Unregister(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	current, ok := h.clients[client.ID]
	if !ok || current != client {
		return
	}
	if h.workspace[client.WorkspaceID] == client.ID {
		delete(h.workspace, client.WorkspaceID)
	}
	delete(h.clients, client.ID)
	close(client.Send)
}

// GetGatewayForWorkspace returns the gateway ID for a workspace
func (h *Hub) GetGatewayForWorkspace(workspaceID string) string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.workspace[workspaceID]
}

// SendToGateway sends a message to a specific gateway
func (h *Hub) SendToGateway(gatewayID string, msg []byte) error {
	h.mu.RLock()
	defer h.mu.RUnlock()
	client, ok := h.clients[gatewayID]

	if !ok {
		return ErrGatewayNotConnected
	}

	select {
	case client.Send <- msg:
		return nil
	default:
		// Buffer full, gateway is unresponsive - remove it
		go h.Unregister(client)
		return ErrGatewaySendFailed
	}
}

// ErrGatewayNotConnected is returned when gateway is not connected
var ErrGatewayNotConnected = &gatewayError{"gateway not connected"}

// ErrGatewaySendFailed is returned when send buffer is full
var ErrGatewaySendFailed = &gatewayError{"gateway send failed"}

type gatewayError struct{ msg string }

func (e *gatewayError) Error() string { return e.msg }

// ReadPump pumps messages from the WebSocket connection to the hub
func (c *Client) ReadPump() {
	defer func() {
		c.Hub.Unregister(c)
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(maxMessageSize)
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				slog.Error("websocket unexpected close", "gateway", c.ID, "error", err)
			}
			break
		}

		if err := c.handleMessage(message); err != nil {
			slog.Warn("gateway message rejected", "gateway_id", c.ID, "workspace_id", c.WorkspaceID, "error", err)
			continue
		}
	}
}

func (c *Client) handleMessage(message []byte) error {
	var envelope protocol.GatewayEnvelope
	if err := json.Unmarshal(message, &envelope); err != nil {
		return fmt.Errorf("decode envelope: %w", err)
	}

	switch envelope.Type {
	case protocol.EventGatewayHeartbeat:
		// Respond with heartbeat
		msg, _ := json.Marshal(protocol.GatewayHeartbeatMessage{Type: protocol.EventGatewayHeartbeat})
		select {
		case c.Send <- msg:
			return nil
		default:
			return ErrGatewaySendFailed
		}

	case protocol.EventTaskCompleted:
		var event protocol.GatewayTaskCompletedMessage
		if err := json.Unmarshal(message, &event); err != nil {
			return fmt.Errorf("decode task completed: %w", err)
		}
		if strings.TrimSpace(event.TaskID) == "" || strings.TrimSpace(event.RunID) == "" {
			return fmt.Errorf("task completed: task_id and run_id are required")
		}
		if c.Hub.OnTaskComplete != nil {
			c.Hub.OnTaskComplete(c.ID, c.WorkspaceID, event.TaskID, event.RunID, event.ExitCode, event.Output)
		}
		return nil

	case protocol.EventTaskDispatched:
		var event protocol.GatewayTaskDispatchedMessage
		if err := json.Unmarshal(message, &event); err != nil {
			return fmt.Errorf("decode task dispatched: %w", err)
		}
		if strings.TrimSpace(event.TaskID) == "" || strings.TrimSpace(event.RunID) == "" || strings.TrimSpace(event.ContainerID) == "" {
			return fmt.Errorf("task dispatched: task_id, run_id, and container_id are required")
		}
		if c.Hub.OnTaskDispatched != nil {
			c.Hub.OnTaskDispatched(c.ID, c.WorkspaceID, event.TaskID, event.RunID, event.ContainerID)
		}
		return nil

	case protocol.EventTaskFailed:
		var event protocol.GatewayTaskFailedMessage
		if err := json.Unmarshal(message, &event); err != nil {
			return fmt.Errorf("decode task failed: %w", err)
		}
		if strings.TrimSpace(event.TaskID) == "" || strings.TrimSpace(event.RunID) == "" {
			return fmt.Errorf("task failed: task_id and run_id are required")
		}
		if c.Hub.OnTaskFail != nil {
			c.Hub.OnTaskFail(c.ID, c.WorkspaceID, event.TaskID, event.RunID, event.Error, event.Retryable)
		}
		return nil

	case protocol.EventTaskLogs:
		var event protocol.GatewayTaskLogsMessage
		if err := json.Unmarshal(message, &event); err != nil {
			return fmt.Errorf("decode task logs: %w", err)
		}
		if strings.TrimSpace(event.TaskID) == "" || strings.TrimSpace(event.RunID) == "" {
			return fmt.Errorf("task logs: task_id and run_id are required")
		}
		if event.Seq <= 0 || int64(event.Seq) > 2147483647 {
			return fmt.Errorf("task logs: seq must be between 1 and 2147483647")
		}
		if event.Stream != protocol.GatewayStreamStdout && event.Stream != protocol.GatewayStreamStderr {
			return fmt.Errorf("task logs: unsupported stream %q", event.Stream)
		}
		if event.Content == "" {
			return fmt.Errorf("task logs: content is required")
		}
		if len(event.Content) > protocol.GatewayLogChunkBytes {
			return fmt.Errorf("task logs: content exceeds %d bytes", protocol.GatewayLogChunkBytes)
		}
		if c.Hub.OnTaskLogs != nil {
			c.Hub.OnTaskLogs(c.ID, c.WorkspaceID, event.TaskID, event.RunID, event.Seq, event.Stream, event.Content)
		}
		return nil

	default:
		return fmt.Errorf("unsupported event type %q", envelope.Type)
	}
}

// WritePump pumps messages from the hub to the WebSocket connection
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			if err := c.Conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				return
			}
			if !ok {
				_ = c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			if _, err := w.Write(message); err != nil {
				_ = w.Close()
				return
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			if err := c.Conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				return
			}
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
