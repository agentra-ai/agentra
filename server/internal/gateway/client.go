package gateway

import (
	"encoding/json"
	"log/slog"
	"sync"
	"time"

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
	mu          sync.Mutex
}

// Hub manages all gateway connections
type Hub struct {
	mu       sync.RWMutex
	clients  map[string]*Client  // gatewayID -> Client
	workspace map[string]string  // workspaceID -> gatewayID

	// Task dispatch callbacks
	OnTaskDispatch func(gatewayID, taskID string, config map[string]any)
	OnTaskComplete func(gatewayID, taskID string, exitCode int, output string)
	OnTaskFail     func(gatewayID, taskID string, error string)
	OnTaskLogs     func(gatewayID, taskID string, logs string)
	OnTaskCancel   func(gatewayID, taskID string)
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
	defer h.mu.Unlock()
	h.clients[client.ID] = client
	h.workspace[client.WorkspaceID] = client.ID
}

// Unregister removes a gateway connection
func (h *Hub) Unregister(gatewayID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if client, ok := h.clients[gatewayID]; ok {
		delete(h.workspace, client.WorkspaceID)
		delete(h.clients, gatewayID)
		close(client.Send)
	}
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
	client, ok := h.clients[gatewayID]
	h.mu.RUnlock()

	if !ok {
		return ErrGatewayNotConnected
	}

	select {
	case client.Send <- msg:
		return nil
	default:
		// Buffer full, gateway is unresponsive - remove it
		go func() {
			h.Unregister(gatewayID)
		}()
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
		c.Hub.Unregister(c.ID)
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

		var event map[string]any
		if err := json.Unmarshal(message, &event); err != nil {
			continue
		}

		c.handleEvent(event)
	}
}

func (c *Client) handleEvent(event map[string]any) {
	switch event["type"] {
	case "gateway:heartbeat":
		// Respond with heartbeat
		msg, _ := json.Marshal(map[string]any{"type": "gateway:heartbeat"})
		c.Send <- msg

	case "task:completed":
		if c.Hub.OnTaskComplete != nil {
			gatewayID := c.ID
			taskID, _ := event["task_id"].(string)
			exitCode, _ := event["exit_code"].(int)
			output, _ := event["output"].(string)
			c.Hub.OnTaskComplete(gatewayID, taskID, exitCode, output)
		}

	case "task:failed":
		if c.Hub.OnTaskFail != nil {
			gatewayID := c.ID
			taskID, _ := event["task_id"].(string)
			errStr, _ := event["error"].(string)
			c.Hub.OnTaskFail(gatewayID, taskID, errStr)
		}

	case "task:logs":
		if c.Hub.OnTaskLogs != nil {
			gatewayID := c.ID
			taskID, _ := event["task_id"].(string)
			logs, _ := event["logs"].(string)
			c.Hub.OnTaskLogs(gatewayID, taskID, logs)
		}
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
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}