package gateway

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/agentra-ai/agentra/server/pkg/protocol"
	"github.com/gorilla/websocket"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestWSClientConnectUsesAuthorizationAndCanonicalProtocol(t *testing.T) {
	type requestData struct {
		authorization string
		gatewayID     string
		workspaceID   string
		queryToken    string
		message       []byte
	}
	received := make(chan requestData, 1)
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/gateway/connect", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		_, message, err := conn.ReadMessage()
		if err != nil {
			return
		}
		received <- requestData{
			authorization: r.Header.Get("Authorization"),
			gatewayID:     r.URL.Query().Get("gateway_id"),
			workspaceID:   r.URL.Query().Get("workspace_id"),
			queryToken:    r.URL.Query().Get("token"),
			message:       message,
		}
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	serverURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	client := NewWSClient(serverURL, "gateway-1", "workspace-1", "secret-token", discardLogger())
	if err := client.Connect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer client.conn.Close()
	if err := client.SendTaskCompleted("task-1", "run-1", 19, "failed"); err != nil {
		t.Fatalf("send completed: %v", err)
	}

	select {
	case got := <-received:
		if got.authorization != "Bearer secret-token" {
			t.Fatalf("authorization = %q", got.authorization)
		}
		if got.queryToken != "" {
			t.Fatalf("secret leaked in query string: %q", got.queryToken)
		}
		if got.gatewayID != "gateway-1" || got.workspaceID != "workspace-1" {
			t.Fatalf("identity query = (%q, %q)", got.gatewayID, got.workspaceID)
		}
		var message protocol.GatewayTaskCompletedMessage
		if err := json.Unmarshal(got.message, &message); err != nil {
			t.Fatal(err)
		}
		if message.TaskID != "task-1" || message.RunID != "run-1" || message.ExitCode != 19 {
			t.Fatalf("message = %+v", message)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for gateway message")
	}
}

func TestWSClientConnectFailsClosedOnMissingIdentity(t *testing.T) {
	tests := []struct {
		name        string
		gatewayID   string
		workspaceID string
		token       string
		want        string
	}{
		{name: "gateway", workspaceID: "workspace-1", token: "token", want: "gateway ID"},
		{name: "workspace", gatewayID: "gateway-1", token: "token", want: "workspace ID"},
		{name: "token", gatewayID: "gateway-1", workspaceID: "workspace-1", want: "auth token"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewWSClient("ws://127.0.0.1:1/ws", tt.gatewayID, tt.workspaceID, tt.token, discardLogger())
			err := client.Connect(context.Background())
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestWSClientRunStopsWhenContextIsCanceledWhileIdle(t *testing.T) {
	connected := make(chan struct{})
	release := make(chan struct{})
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/gateway/connect", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		close(connected)
		<-release
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	defer close(release)

	serverURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	client := NewWSClient(serverURL, "gateway-1", "workspace-1", "secret-token", discardLogger())
	if err := client.Connect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}
	<-connected

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- client.Run(ctx) }()
	cancel()

	select {
	case err := <-done:
		if err != context.Canceled {
			t.Fatalf("Run() error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run did not stop after context cancellation")
	}
}
