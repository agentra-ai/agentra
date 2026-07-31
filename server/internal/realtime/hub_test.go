package realtime

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

const testWorkspaceID = "test-workspace"
const testUserID = "test-user"

// mockMembershipChecker always returns true.
type mockMembershipChecker struct{}

func (m *mockMembershipChecker) IsMember(_ context.Context, _, _ string) bool {
	return true
}

type rejectMembershipChecker struct{}

func (m *rejectMembershipChecker) IsMember(_ context.Context, _, _ string) bool {
	return false
}

type mockUserAuthenticator struct{}

func (a *mockUserAuthenticator) Authenticate(_ context.Context, token string) (string, error) {
	if token == "test-token" || token == "mul_test_pat" {
		return testUserID, nil
	}
	return "", errors.New("invalid token")
}

func newTestHub(t *testing.T) (*Hub, *httptest.Server) {
	t.Helper()
	hub := NewHub()
	go hub.Run()

	mc := &mockMembershipChecker{}
	authenticator := &mockUserAuthenticator{}
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		HandleWebSocket(hub, authenticator, mc, w, r)
	})
	server := httptest.NewServer(mux)

	// The default wsUpgrader has a nil allowList (fail-closed). Let the test
	// server's own origin through so the DefaultDialer (which always sends
	// Origin via gorilla/websocket) can connect. httptest's server.URL is
	// already an absolute "http://host:port" origin.
	SetWSAllowedOrigins([]string{server.URL})

	return hub, server
}

func connectWS(t *testing.T, server *httptest.Server) *websocket.Conn {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws?token=test-token&workspace_id=" + testWorkspaceID
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect WebSocket: %v", err)
	}
	return conn
}

func newGatewayTestServer(t *testing.T, mc MembershipChecker) (*Hub, *httptest.Server) {
	t.Helper()
	hub := NewHub()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/gateway/connect", func(w http.ResponseWriter, r *http.Request) {
		HandleGatewayWebSocket(hub, &mockUserAuthenticator{}, mc, w, r)
	})
	return hub, httptest.NewServer(mux)
}

func gatewayWSURL(server *httptest.Server) string {
	return "ws" + strings.TrimPrefix(server.URL, "http") + "/api/gateway/connect?gateway_id=gateway-1&workspace_id=" + testWorkspaceID
}

func TestGatewayWebSocketRequiresAuthorizationHeader(t *testing.T) {
	_, server := newGatewayTestServer(t, &mockMembershipChecker{})
	defer server.Close()

	for _, rawURL := range []string{
		gatewayWSURL(server),
		gatewayWSURL(server) + "&token=test-token",
	} {
		conn, response, err := websocket.DefaultDialer.Dial(rawURL, nil)
		if conn != nil {
			conn.Close()
		}
		if response != nil && response.Body != nil {
			response.Body.Close()
		}
		if err == nil {
			t.Fatalf("gateway connected without authorization header: %s", rawURL)
		}
		if response == nil || response.StatusCode != http.StatusUnauthorized {
			t.Fatalf("status = %v, want 401", response)
		}
	}
}

func TestGatewayWebSocketBindsAuthenticatedWorkspace(t *testing.T) {
	hub, server := newGatewayTestServer(t, &mockMembershipChecker{})
	defer server.Close()

	header := http.Header{"Authorization": []string{"Bearer test-token"}}
	conn, _, err := websocket.DefaultDialer.Dial(gatewayWSURL(server), header)
	if err != nil {
		t.Fatalf("connect gateway: %v", err)
	}
	defer conn.Close()

	if got := hub.GatewayHub.GetGatewayForWorkspace(testWorkspaceID); got != "gateway-1" {
		t.Fatalf("workspace gateway = %q, want gateway-1", got)
	}
}

func TestGatewayWebSocketRejectsNonMember(t *testing.T) {
	_, server := newGatewayTestServer(t, &rejectMembershipChecker{})
	defer server.Close()

	header := http.Header{"Authorization": []string{"Bearer test-token"}}
	conn, response, err := websocket.DefaultDialer.Dial(gatewayWSURL(server), header)
	if conn != nil {
		conn.Close()
	}
	if response != nil && response.Body != nil {
		defer response.Body.Close()
	}
	if err == nil {
		t.Fatal("non-member gateway unexpectedly connected")
	}
	if response == nil || response.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %v, want 403", response)
	}
}

func TestHub_AcceptsPATFromAuthorizationHeader(t *testing.T) {
	_, server := newTestHub(t)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws?workspace_id=" + testWorkspaceID
	header := http.Header{"Authorization": []string{"Bearer mul_test_pat"}}
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("connect with PAT authorization header: %v", err)
	}
	defer conn.Close()

	if err := conn.WriteJSON(map[string]string{"type": "ping"}); err != nil {
		t.Fatalf("write ping: %v", err)
	}
	var response map[string]string
	if err := conn.ReadJSON(&response); err != nil {
		t.Fatalf("read pong: %v", err)
	}
	if response["type"] != "pong" {
		t.Fatalf("response type = %q, want pong", response["type"])
	}
}

func TestHub_RejectsPATInQueryString(t *testing.T) {
	_, server := newTestHub(t)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws?token=mul_test_pat&workspace_id=" + testWorkspaceID
	conn, response, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if conn != nil {
		conn.Close()
	}
	if response != nil && response.Body != nil {
		defer response.Body.Close()
	}
	if err == nil {
		t.Fatal("query-string PAT unexpectedly connected")
	}
	if response == nil || response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %v, want 401", response)
	}
}

// totalClients counts all clients across all rooms.
func totalClients(hub *Hub) int {
	hub.mu.RLock()
	defer hub.mu.RUnlock()
	count := 0
	for _, clients := range hub.rooms {
		count += len(clients)
	}
	return count
}

func TestHub_ClientRegistration(t *testing.T) {
	hub, server := newTestHub(t)
	defer server.Close()

	conn := connectWS(t, server)
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)

	count := totalClients(hub)
	if count != 1 {
		t.Fatalf("expected 1 client, got %d", count)
	}
}

func TestHub_Broadcast(t *testing.T) {
	hub, server := newTestHub(t)
	defer server.Close()

	conn1 := connectWS(t, server)
	defer conn1.Close()
	conn2 := connectWS(t, server)
	defer conn2.Close()

	time.Sleep(50 * time.Millisecond)

	msg := []byte(`{"type":"issue:created","data":"test"}`)
	hub.Broadcast(msg)

	conn1.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, received1, err := conn1.ReadMessage()
	if err != nil {
		t.Fatalf("client 1 read error: %v", err)
	}
	if string(received1) != string(msg) {
		t.Fatalf("client 1: expected %s, got %s", msg, received1)
	}

	conn2.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, received2, err := conn2.ReadMessage()
	if err != nil {
		t.Fatalf("client 2 read error: %v", err)
	}
	if string(received2) != string(msg) {
		t.Fatalf("client 2: expected %s, got %s", msg, received2)
	}
}

func TestHub_ClientDisconnect(t *testing.T) {
	hub, server := newTestHub(t)
	defer server.Close()

	conn := connectWS(t, server)

	time.Sleep(50 * time.Millisecond)

	countBefore := totalClients(hub)
	if countBefore != 1 {
		t.Fatalf("expected 1 client before disconnect, got %d", countBefore)
	}

	conn.Close()
	time.Sleep(100 * time.Millisecond)

	countAfter := totalClients(hub)
	if countAfter != 0 {
		t.Fatalf("expected 0 clients after disconnect, got %d", countAfter)
	}
}

func TestHub_BroadcastToMultipleClients(t *testing.T) {
	hub, server := newTestHub(t)
	defer server.Close()

	const numClients = 5
	conns := make([]*websocket.Conn, numClients)
	for i := 0; i < numClients; i++ {
		conns[i] = connectWS(t, server)
		defer conns[i].Close()
	}

	time.Sleep(50 * time.Millisecond)

	count := totalClients(hub)
	if count != numClients {
		t.Fatalf("expected %d clients, got %d", numClients, count)
	}

	msg := []byte(`{"type":"test","count":5}`)
	hub.Broadcast(msg)

	for i, conn := range conns {
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, received, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("client %d read error: %v", i, err)
		}
		if string(received) != string(msg) {
			t.Fatalf("client %d: expected %s, got %s", i, msg, received)
		}
	}
}

func TestHub_MultipleBroadcasts(t *testing.T) {
	hub, server := newTestHub(t)
	defer server.Close()

	conn := connectWS(t, server)
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)

	messages := []string{
		`{"type":"issue:created"}`,
		`{"type":"issue:updated"}`,
		`{"type":"issue:deleted"}`,
	}

	for _, msg := range messages {
		hub.Broadcast([]byte(msg))
	}

	for i, expected := range messages {
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, received, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("message %d read error: %v", i, err)
		}
		if string(received) != expected {
			t.Fatalf("message %d: expected %s, got %s", i, expected, received)
		}
	}
}
