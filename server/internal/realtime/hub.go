package realtime

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"github.com/agentra-ai/agentra/server/internal/auth"
	"github.com/agentra-ai/agentra/server/internal/gateway"
)

// MembershipChecker verifies a user belongs to a workspace.
type MembershipChecker interface {
	IsMember(ctx context.Context, userID, workspaceID string) bool
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// TODO: Restrict origins in production
		return true
	},
}

// Client represents a single WebSocket connection with identity.
type Client struct {
	hub         *Hub
	conn        *websocket.Conn
	send        chan []byte
	userID      string
	workspaceID string
	rooms       map[string]bool // workspaceID -> subscribed
	mu          sync.RWMutex
}

func (c *Client) joinRoom(workspaceID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rooms[workspaceID] = true
}

func (c *Client) leaveRoom(workspaceID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.rooms, workspaceID)
}

func (c *Client) isInRoom(workspaceID string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.rooms[workspaceID]
}

// Hub manages WebSocket connections organized by workspace rooms.
type Hub struct {
	rooms      map[string]map[*Client]bool // workspaceID -> clients
	broadcast  chan []byte                  // global broadcast (daemon events)
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex

	// GatewayHub manages Cloud Runtime Gateway connections
	GatewayHub *gateway.Hub
}

// NewHub creates a new Hub instance.
func NewHub() *Hub {
	return &Hub{
		rooms:      make(map[string]map[*Client]bool),
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		GatewayHub: gateway.NewHub(),
	}
}

// Run starts the hub event loop.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			room := client.workspaceID
			if h.rooms[room] == nil {
				h.rooms[room] = make(map[*Client]bool)
			}
			h.rooms[room][client] = true
			total := 0
			for _, r := range h.rooms {
				total += len(r)
			}
			h.mu.Unlock()
			slog.Info("ws client connected", "workspace_id", room, "total_clients", total)

		case client := <-h.unregister:
			h.mu.Lock()
			room := client.workspaceID
			if clients, ok := h.rooms[room]; ok {
				if _, exists := clients[client]; exists {
					delete(clients, client)
					close(client.send)
					if len(clients) == 0 {
						delete(h.rooms, room)
					}
				}
			}
			total := 0
			for _, r := range h.rooms {
				total += len(r)
			}
			h.mu.Unlock()
			slog.Info("ws client disconnected", "workspace_id", room, "total_clients", total)

		case message := <-h.broadcast:
			// Global broadcast for daemon events (no workspace filtering)
			h.mu.RLock()
			var slow []*Client
			for _, clients := range h.rooms {
				for client := range clients {
					select {
					case client.send <- message:
					default:
						slow = append(slow, client)
					}
				}
			}
			h.mu.RUnlock()
			if len(slow) > 0 {
				h.mu.Lock()
				for _, client := range slow {
					room := client.workspaceID
					if clients, ok := h.rooms[room]; ok {
						if _, exists := clients[client]; exists {
							delete(clients, client)
							close(client.send)
							if len(clients) == 0 {
								delete(h.rooms, room)
							}
						}
					}
				}
				h.mu.Unlock()
			}
		}
	}
}

// BroadcastToWorkspace sends a message only to clients in the given workspace.
func (h *Hub) BroadcastToWorkspace(workspaceID string, message []byte) {
	h.mu.RLock()
	var slow []*Client // clients to remove from room
	for _, clients := range h.rooms {
		for client := range clients {
			if client.workspaceID == workspaceID && client.isInRoom(workspaceID) {
				select {
				case client.send <- message:
				default:
					slow = append(slow, client)
				}
			}
		}
	}
	h.mu.RUnlock()

	// Asynchronously remove slow clients to avoid blocking callers
	if len(slow) > 0 {
		go func() {
			h.mu.Lock()
			defer h.mu.Unlock()
			for _, client := range slow {
				if room, ok := h.rooms[workspaceID]; ok {
					if _, exists := room[client]; exists {
						delete(room, client)
						close(client.send)
						if len(room) == 0 {
							delete(h.rooms, workspaceID)
						}
					}
				}
			}
		}()
	}
}

// SendToUser sends a message to all connections belonging to a specific user,
// regardless of which workspace room they are in. Connections in excludeWorkspace
// are skipped (they already receive the message via BroadcastToWorkspace).
func (h *Hub) SendToUser(userID string, message []byte, excludeWorkspace ...string) {
	exclude := ""
	if len(excludeWorkspace) > 0 {
		exclude = excludeWorkspace[0]
	}

	h.mu.RLock()
	type target struct {
		client      *Client
		workspaceID string
	}
	var targets []target
	for wsID, clients := range h.rooms {
		if wsID == exclude {
			continue
		}
		for client := range clients {
			if client.userID == userID {
				targets = append(targets, target{client, wsID})
			}
		}
	}
	h.mu.RUnlock()

	var slow []target
	for _, t := range targets {
		select {
		case t.client.send <- message:
		default:
			slow = append(slow, t)
		}
	}

	// Asynchronously remove slow clients to avoid blocking BroadcastToWorkspace
	if len(slow) > 0 {
		go func() {
			h.mu.Lock()
			defer h.mu.Unlock()
			for _, t := range slow {
				if room, ok := h.rooms[t.workspaceID]; ok {
					if _, exists := room[t.client]; exists {
						delete(room, t.client)
						close(t.client.send)
						if len(room) == 0 {
							delete(h.rooms, t.workspaceID)
						}
					}
				}
			}
		}()
	}
}

// Broadcast sends a message to all connected clients (used for daemon events).
func (h *Hub) Broadcast(message []byte) {
	h.broadcast <- message
}

// HandleWebSocket upgrades an HTTP connection to WebSocket with JWT auth.
func HandleWebSocket(hub *Hub, mc MembershipChecker, w http.ResponseWriter, r *http.Request) {
	tokenStr := r.URL.Query().Get("token")
	workspaceID := r.URL.Query().Get("workspace_id")

	if tokenStr == "" || workspaceID == "" {
		http.Error(w, `{"error":"token and workspace_id required"}`, http.StatusUnauthorized)
		return
	}

	// Validate JWT
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return auth.JWTSecret(), nil
	})
	if err != nil || !token.Valid {
		http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		http.Error(w, `{"error":"invalid claims"}`, http.StatusUnauthorized)
		return
	}

	userID, ok := claims["sub"].(string)
	if !ok || strings.TrimSpace(userID) == "" {
		http.Error(w, `{"error":"invalid claims"}`, http.StatusUnauthorized)
		return
	}

	// Verify user is a member of the workspace
	if !mc.IsMember(r.Context(), userID, workspaceID) {
		http.Error(w, `{"error":"not a member of this workspace"}`, http.StatusForbidden)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("websocket upgrade failed", "error", err)
		return
	}

	client := &Client{
		hub:         hub,
		conn:        conn,
		send:        make(chan []byte, 256),
		userID:      userID,
		workspaceID: workspaceID,
		rooms:       make(map[string]bool),
	}
	// Auto-join the workspace room on connect
	client.joinRoom(workspaceID)
	hub.register <- client

	go client.writePump()
	go client.readPump()
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				slog.Debug("websocket read error", "error", err, "user_id", c.userID, "workspace_id", c.workspaceID)
			}
			break
		}

		var event map[string]any
		if err := json.Unmarshal(message, &event); err != nil {
			slog.Debug("ws message unmarshal failed", "user_id", c.userID, "error", err)
			continue
		}

		c.handleInboundEvent(event)
	}
}

func (c *Client) handleInboundEvent(event map[string]any) {
	switch event["type"] {
	case "ping":
		msg, _ := json.Marshal(map[string]any{"type": "pong"})
		c.send <- msg
	case "subscribe":
		workspaceID, _ := event["workspace_id"].(string)
		if workspaceID == "" {
			return
		}
		c.joinRoom(workspaceID)
		slog.Debug("client subscribed to room", "user_id", c.userID, "workspace_id", workspaceID)
	case "unsubscribe":
		workspaceID, _ := event["workspace_id"].(string)
		if workspaceID == "" {
			return
		}
		c.leaveRoom(workspaceID)
		slog.Debug("client unsubscribed from room", "user_id", c.userID, "workspace_id", workspaceID)
	default:
		slog.Debug("ws inbound message ignored", "user_id", c.userID, "type", event["type"])
	}
}

func (c *Client) writePump() {
	defer c.conn.Close()

	for message := range c.send {
		if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
			slog.Warn("websocket write error", "error", err)
			return
		}
	}
}

// HandleGatewayWebSocket upgrades an HTTP connection to WebSocket for Cloud Runtime Gateway connections.
func HandleGatewayWebSocket(hub *Hub, w http.ResponseWriter, r *http.Request) {
	gatewayID := r.URL.Query().Get("gateway_id")
	if gatewayID == "" {
		http.Error(w, `{"error":"gateway_id required"}`, http.StatusBadRequest)
		return
	}

	// Validate auth token if provided
	token := r.URL.Query().Get("token")
	if token != "" {
		// For now, tokens are not validated - gateways are trusted within the network
		// In production, validate against a shared secret or JWT
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("gateway websocket upgrade failed", "error", err, "gateway_id", gatewayID)
		return
	}

	gatewayClient := &gateway.Client{
		ID:   gatewayID,
		Conn: conn,
		Hub:  hub.GatewayHub,
		Send: make(chan []byte, 256),
	}

	hub.GatewayHub.Register(gatewayClient)

	go gatewayClient.WritePump()
	go gatewayClient.ReadPump()

	slog.Info("gateway connected", "gateway_id", gatewayID)
}
