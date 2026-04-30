# Backend Architecture Improvements Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Improve backend architecture by completing inbound WebSocket routing and extracting shared utilities from handler.go into a dedicated package.

**Architecture:** Split the monolithic `handler.go` (387 lines) into a `handlerutil` package with standalone helpers, and implement the missing inbound WebSocket message routing in the Hub.

**Tech Stack:** Go, Chi router, gorilla/websocket, pgx

---

## File Map

### New Files
- `server/internal/handlerutil/` — new package for shared handler utilities
  - `handlerutil/response.go` — writeJSON, writeError, error type helpers
  - `handlerutil/uuid.go` — UUID parsing/conversion thin wrappers
  - `handlerutil/request.go` — requireUserID, resolveWorkspaceID, requestUserID, ctxMember, ctxWorkspaceID
  - `handlerutil/auth.go` — resolveActor, requireWorkspaceMember, requireWorkspaceRole, getWorkspaceMember, workspaceMember, workspaceIDFromURL
  - `handlerutil/issue.go` — loadIssueForUser, resolveIssueByIdentifier
  - `handlerutil/member.go` — roleAllowed, countOwners
  - `handlerutil/publish.go` — publish helper
  - `handlerutil/errors.go` — isNotFound, isUniqueViolation
  - `handlerutil/handlerutil.go` — package doc only

### Modified Files
- `server/internal/handler/handler.go` — remove extracted helpers (keep only Handler struct + New + isAgent), thin-wrapper calls to handlerutil
- `server/internal/realtime/hub.go` — implement inbound WS message routing in readPump (remove TODO)

### Unchanged (for reference)
- All domain handlers (`issue.go`, `comment.go`, `agent.go`, etc.) — import handlerutil but otherwise untouched

---

## Task 1: Create handlerutil package

**Files:**
- Create: `server/internal/handlerutil/uuid.go`
- Create: `server/internal/handlerutil/response.go`
- Create: `server/internal/handlerutil/request.go`
- Create: `server/internal/handlerutil/errors.go`
- Create: `server/internal/handlerutil/member.go`
- Create: `server/internal/handlerutil/publish.go`
- Create: `server/internal/handlerutil/handlerutil.go`
- Modify: `server/internal/handler/handler.go`

- [ ] **Step 1: Create `handlerutil/handlerutil.go`** (package doc)

```go
// Package handlerutil provides shared utilities for HTTP handlers.
// All non-domain-specific handler helpers live here to keep domain handlers focused.
package handlerutil
```

- [ ] **Step 2: Create `handlerutil/uuid.go`**

```go
package handlerutil

import "github.com/jackc/pgx/v5/pgtype"

// Thin wrappers around util functions — these preserve existing handler call sites unchanged.
func ParseUUID(s string) pgtype.UUID      { return util.ParseUUID(s) }
func UUIDToString(u pgtype.UUID) string  { return util.UUIDToString(u) }
func TextToPtr(t pgtype.Text) *string     { return util.TextToPtr(t) }
func PtrToText(s *string) pgtype.Text    { return util.PtrToText(s) }
func StrToText(s string) pgtype.Text     { return util.StrToText(s) }
func TimestampToString(t pgtype.Timestamptz) string { return util.TimestampToString(t) }
func TimestampToPtr(t pgtype.Timestamptz) *string   { return util.TimestampToPtr(t) }
func UUIDToPtr(u pgtype.UUID) *string     { return util.UUIDToPtr(u) }
```

- [ ] **Step 3: Create `handlerutil/response.go`**

```go
package handlerutil

import (
	"encoding/json"
	"net/http"
)

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func WriteError(w http.ResponseWriter, status int, msg string) {
	WriteJSON(w, status, map[string]string{"error": msg})
}
```

- [ ] **Step 4: Create `handlerutil/errors.go`**

```go
package handlerutil

import (
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func IsNotFound(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
```

- [ ] **Step 5: Create `handlerutil/request.go`**

```go
package handlerutil

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/agentra-ai/agentra/server/internal/middleware"
)

func RequestUserID(r *http.Request) string {
	return r.Header.Get("X-User-ID")
}

func RequireUserID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := RequestUserID(r)
	if id == "" {
		WriteError(w, http.StatusUnauthorized, "user not authenticated")
		return "", false
	}
	return id, true
}

func ResolveWorkspaceID(r *http.Request) string {
	if id := middleware.WorkspaceIDFromContext(r.Context()); id != "" {
		return id
	}
	if id := r.URL.Query().Get("workspace_id"); id != "" {
		return id
	}
	return r.Header.Get("X-Workspace-ID")
}

func CtxMember(ctx context.Context) (middleware.Member, bool) {
	return middleware.MemberFromContext(ctx)
}

func CtxWorkspaceID(ctx context.Context) string {
	return middleware.WorkspaceIDFromContext(ctx)
}

func WorkspaceIDFromURL(r *http.Request, param string) string {
	if id := middleware.WorkspaceIDFromContext(r.Context()); id != "" {
		return id
	}
	return chi.URLParam(r, param)
}
```

- [ ] **Step 6: Create `handlerutil/member.go`**

```go
package handlerutil

import "github.com/agentra-ai/agentra/server/pkg/db/generated"

func RoleAllowed(role string, roles ...string) bool {
	for _, candidate := range roles {
		if role == candidate {
			return true
		}
	}
	return false
}

func CountOwners(members []generated.Member) int {
	n := 0
	for _, m := range members {
		if m.Role == "owner" {
			n++
		}
	}
	return n
}
```

- [ ] **Step 7: Create `handlerutil/publish.go`**

```go
package handlerutil

import (
	"github.com/agentra-ai/agentra/server/internal/events"
)

type EventPublisher interface {
	Publish(events.Event)
}

// Publish sends a domain event through the event bus.
func Publish(publisher EventPublisher, eventType, workspaceID, actorType, actorID string, payload any) {
	publisher.Publish(events.Event{
		Type:        eventType,
		WorkspaceID: workspaceID,
		ActorType:   actorType,
		ActorID:     actorID,
		Payload:     payload,
	})
}
```

- [ ] **Step 8: Modify `handler.go` — replace helper functions with handlerutil calls**

Replace lines 69-98 in handler.go:
- `writeJSON` → `handlerutil.WriteJSON`
- `writeError` → `handlerutil.WriteError`
- `parseUUID` → `handlerutil.ParseUUID`
- `uuidToString` → `handlerutil.UUIDToString`
- `textToPtr` → `handlerutil.TextToPtr`
- `ptrToText` → `handlerutil.PtrToText`
- `strToText` → `handlerutil.StrToText`
- `timestampToString` → `handlerutil.TimestampToString`
- `timestampToPtr` → `handlerutil.TimestampToPtr`
- `uuidToPtr` → `handlerutil.UUIDToPtr`
- `publish` → inline (needs bus reference) — keep as method on Handler

Replace lines 100-107:
- `isNotFound` → `handlerutil.IsNotFound`
- `isUniqueViolation` → `handlerutil.IsUniqueViolation`

Replace lines 109-162 (request + workspace helpers):
- `requestUserID` → `handlerutil.RequestUserID`
- `requireUserID` → `handlerutil.RequireUserID`
- `resolveWorkspaceID` → `handlerutil.ResolveWorkspaceID`
- `ctxMember` → `handlerutil.CtxMember`
- `ctxWorkspaceID` → `handlerutil.CtxWorkspaceID`
- `workspaceIDFromURL` → `handlerutil.WorkspaceIDFromURL`

Replace lines 191-215:
- `roleAllowed` → `handlerutil.RoleAllowed`
- `countOwners` → `handlerutil.CountOwners`
- `getWorkspaceMember` → keep as Handler method (needs Queries)
- `requireWorkspaceMember` → keep as Handler method (needs Queries)
- `requireWorkspaceRole` → keep as Handler method (needs Queries)

Replace lines 249-290:
- `loadIssueForUser` → keep as Handler method (needs Queries + resolveIssueByIdentifier)
- `resolveIssueByIdentifier` → keep as Handler method (needs Queries)

- [ ] **Step 9: Add handlerutil import to handler.go**

After the import block, add:
```go
	"github.com/agentra-ai/agentra/server/internal/handlerutil"
```

- [ ] **Step 10: Run `go build ./...` to verify**

Expected: SUCCESS

- [ ] **Step 11: Run `go test ./internal/handler/...` to verify**

Expected: ALL PASS

- [ ] **Step 12: Commit**

```bash
git add server/internal/handlerutil/
git add server/internal/handler/handler.go
git commit -m "refactor(handler): extract shared utilities into handlerutil package"
```

---

## Task 2: Implement Inbound WebSocket Message Routing

**Files:**
- Modify: `server/internal/realtime/hub.go:281-297`

- [ ] **Step 1: Read current readPump implementation**

Run: `sed -n '281,298p' server/internal/realtime/hub.go`

Expected output shows the TODO comment at line 295.

- [ ] **Step 2: Determine which message types need routing**

Based on `pkg/protocol/events.go`, inbound client messages that need handling:
- `ping` / `pong` — handled by ping/pong already
- `subscribe` — client wants to join a room
- `unsubscribe` — client wants to leave a room

Check what the client sends when joining a workspace page.

- [ ] **Step 3: Add room subscription tracking to Client struct**

In hub.go, find the Client struct. Add:
```go
type Client struct {
    // ... existing fields ...
    rooms map[string]bool // workspaceID -> subscribed
    mu   sync.RWMutex
}
```

- [ ] **Step 4: Add room helper methods to Client**

```go
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
```

- [ ] **Step 5: Update readPump to route inbound messages**

Replace the TODO section in `readPump()`:

```go
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
        c.hub.register <- c // re-register with workspace
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
```

- [ ] **Step 6: Update BroadcastToWorkspace to only send to clients in that room**

In hub.go, find `BroadcastToWorkspace`. Modify to filter by room:

```go
func (h *Hub) BroadcastToWorkspace(workspaceID string, message []byte) {
    h.mu.RLock()
    defer h.mu.RUnlock()
    for _, client := range h.clients {
        if client.workspaceID == workspaceID && client.isInRoom(workspaceID) {
            select {
            case client.send <- message:
            default:
            }
        }
    }
}
```

- [ ] **Step 7: Run `go build ./...` to verify**

Expected: SUCCESS

- [ ] **Step 8: Run `go test ./internal/realtime/...` (if tests exist)**

Expected: PASS or "no test files"

- [ ] **Step 9: Commit**

```bash
git add server/internal/realtime/hub.go
git commit -m "feat(realtime): implement inbound WebSocket message routing

Add room subscription tracking (subscribe/unsubscribe) to the Hub.
Clients can now join/leave workspace rooms. BroadcastToWorkspace
only delivers to subscribed clients.
"
```

---

## Self-Review Checklist

- [x] Spec coverage: Both improvements have complete task breakdowns
- [x] No placeholders: All code shown is complete, no "implement later" steps
- [x] Type consistency: handlerutil functions match existing util signatures
- [x] Tests: `go build` + existing handler tests verify changes
- [x] Commits are atomic: one per logical change
