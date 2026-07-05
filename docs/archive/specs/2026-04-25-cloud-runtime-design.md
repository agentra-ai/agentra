# Cloud Runtime Design

**Phase 1 Milestone 1.1 — Agent-as-a-Service Cloud Runtime**

## Overview

Cloud Runtime enables agents to execute tasks in cloud-hosted containers without requiring a local daemon. Users provide their own API keys (Anthropic/OpenAI); Agentra provides the execution infrastructure, streaming logs, and container lifecycle management.

**Mode**: Agent-as-a-Service — users bring their own API keys; Agentra manages the execution environment.

---

## Architecture

### Components

| Component | Responsibility | Location |
|-----------|----------------|----------|
| Task Service | Task lifecycle management, queue, dispatch | Server |
| Cloud Runtime Gateway | Container lifecycle, WebSocket hub, task routing | Agentra Hosted (Basic) / Self-Hosted (Pro) |
| Proxy Service | API key injection, outbound traffic, audit logging | Cloud Runtime Gateway |
| Container Pool | Per-task Docker containers, dynamically created | Cloud Runtime Gateway |
| Workspace Settings | API key storage, runtime configuration | Server DB |

### Communication Flow

```
1. User enables Cloud Runtime in workspace settings (uploads API key)
2. Server registers workspace as "cloud-enabled" with runtime type
3. When task is assigned to agent:
   a. Server sends task via WebSocket to Cloud Runtime Gateway
   b. Gateway creates new Docker container
   c. Gateway injects API key via Proxy
   d. Agent inside container executes using agentra CLI
   e. Logs stream back via WebSocket → Server → Client
   f. On completion, container is destroyed
```

---

## User Configuration Flow

### Basic Tier (Agentra Hosted)

1. User navigates to Workspace Settings → Runtime
2. Selects "Cloud Runtime" tab
3. Enters their Anthropic/OpenAI API key
4. System validates key with a minimal test call
5. Cloud Runtime enabled — no infrastructure to manage

### Professional Tier (Self-Hosted)

1. Workspace admin deploys Cloud Runtime Gateway via Docker
2. Connects Gateway to Agentra Server with auth token
3. Configures API keys and resource limits
4. Workspace settings point to self-hosted Gateway URL

---

## Container Lifecycle

### Startup (On-Demand)

1. Gateway receives task dispatch via WebSocket
2. Creates new Docker container from base image
3. Mounts workspace volume (repos cached)
4. Injects environment: API keys via Proxy, task context
5. Starts agent process inside container
6. Streams logs via WebSocket

### Execution

- Agent runs using standard agentra CLI
- Repo checkout via `agentra repo checkout <url>` (existing protocol)
- All outbound API calls routed through Proxy
- Proxy injects API key, logs usage, enforces quotas

### Shutdown

1. Task completes (success/fail/cancel)
2. Gateway captures final logs
3. Container destroyed after graceful timeout (5s)
4. Metrics recorded: duration, tokens used, cost estimate

---

## Security Model

### Sandboxed with Proxy

```
Container (no direct network)
    │
    │ All outbound HTTP/HTTPS
    ▼
Proxy Service
    ├── Inject API Key
    ├── Log request (method, endpoint, timestamp)
    ├── Quota enforcement
    └── Route to external API
```

### API Key Storage

- Keys encrypted at rest using AES-256-GCM
- Keys never exposed to frontend
- Keys injected into container at startup via environment
- Proxy validates key before forwarding requests

### Container Isolation

- Each task gets isolated container
- Containers cannot access each other's filesystem
- No inbound connections to container allowed
- Container network mode: restricted egress only

---

## Data Flow

### Task Dispatch

```
Server → WebSocket → Cloud Runtime Gateway
         ↓
    Validate workspace subscription
         ↓
    Create container
         ↓
    Inject API key via Proxy config
         ↓
    Start agent process
```

### Log Streaming

```
Container stdout/stderr → Gateway log buffer → WebSocket → Server Hub → All WS Clients
                                         ↓
                                   Issue Timeline (stored)
```

### Task Completion

```
Container exits → Gateway captures exit code
    ↓
Send completion event via WebSocket to Server
    ↓
Server updates task status, broadcasts to clients
    ↓
Gateway destroys container
```

---

## Database Schema

### New Tables

#### `cloud_runtimes`

| Column | Type | Description |
|--------|------|-------------|
| id | UUID | Primary key |
| workspace_id | UUID | FK to workspaces |
| gateway_url | VARCHAR(500) | Self-hosted gateway URL (nullable for basic tier) |
| provider | VARCHAR(50) | "anthropic" / "openai" |
| encrypted_api_key | BYTEA | AES-256-GCM encrypted |
| is_active | BOOLEAN | Runtime enabled |
| max_concurrent_tasks | INT | Self-hosted only |
| created_at | TIMESTAMP | Creation time |
| updated_at | TIMESTAMP | Last update |

#### `cloud_runtime_tasks`

| Column | Type | Description |
|--------|------|-------------|
| id | UUID | Primary key |
| cloud_runtime_id | UUID | FK to cloud_runtimes |
| task_id | UUID | FK to tasks |
| container_id | VARCHAR(100) | Docker container ID |
| started_at | TIMESTAMP | Container start time |
| completed_at | TIMESTAMP | Container end time |
| exit_code | INT | Container exit code |
| token_usage | JSONB | Token count by type |
| cost_estimate | DECIMAL | Estimated cost in USD |

### Modified Tables

#### `tasks`

- Add `runtime_type` ENUM: 'local' | 'cloud'
- Add `cloud_runtime_id` UUID (FK, nullable)

#### `agents`

- Add `preferred_runtime` ENUM: 'local' | 'cloud' | 'any'

---

## API Endpoints

### Gateway → Server

| Method | Path | Description |
|--------|------|-------------|
| POST | /api/gateway/register | Register self-hosted gateway |
| WS | /api/gateway/tasks | Task dispatch WebSocket |
| POST | /api/gateway/tasks/{id}/complete | Report task completion |
| POST | /api/gateway/tasks/{id}/fail | Report task failure |
| POST | /api/gateway/tasks/{id}/logs | Stream logs |
| GET | /api/gateway/health | Gateway health check |

### Server → Gateway (Internal)

| Method | Path | Description |
|--------|------|-------------|
| WS | /api/gateway/connect | Gateway connects for task dispatch |

---

## WebSocket Protocol

### Gateway → Server Events

```typescript
// Gateway registers and maintains connection
{ type: "gateway:register", gatewayId: string, capabilities: {...} }
{ type: "gateway:heartbeat" }

// Task lifecycle
{ type: "task:dispatched", taskId: string, containerId: string }
{ type: "task:logs", taskId: string, logs: string }
{ type: "task:completed", taskId: string, exitCode: number, output: string }
{ type: "task:failed", taskId: string, error: string }
```

### Server → Gateway Events

```typescript
// Server dispatches task
{ type: "task:start", taskId: string, config: {...} }
{ type: "task:cancel", taskId: string }
```

---

## Frontend Changes

### Workspace Settings Page

New "Runtime" tab:

- **Runtime Type Selector**: Local Daemon / Cloud Runtime
- **Cloud Runtime Card** (when selected):
  - Provider dropdown: Anthropic / OpenAI
  - API Key input (masked)
  - "Test Connection" button
  - Usage stats (tasks run, cost this month)
- **Advanced Settings** (Pro tier):
  - Gateway URL input
  - Max concurrent tasks slider

### Agent Card

- Badge showing runtime type: "Local" / "Cloud"
- Status indicator: idle / running (with container info)

### Issue Detail Page

- Task timeline shows "Running in Cloud" with container icon
- Live log viewer (collapsible, existing component)
- Runtime info tooltip on hover

---

## Error Handling

| Scenario | Handling |
|----------|----------|
| API key invalid | Show error in settings, pause cloud tasks |
| Container creation timeout | Retry 3x, then mark task failed |
| Container OOM/killed | Capture exit code, report as failed |
| Gateway unreachable | Server marks tasks as pending, retry on reconnect |
| Proxy rate limit hit | Queue requests, retry with backoff |
| Task cancelled | Gateway receives cancel event, terminates container |

---

## Cost Estimation

Proxy tracks all API calls:

```json
{
  "provider": "anthropic",
  "model": "claude-sonnet-4-20250514",
  "input_tokens": 1234,
  "output_tokens": 5678,
  "cost_usd": 0.0234,
  "timestamp": "2025-04-25T10:00:00Z"
}
```

Cost displayed per-task and aggregated per-workspace.

---

## Implementation Phases

### Phase 1 (This Spec)
- Basic tier: Agentra-hosted Gateway
- WebSocket-based task dispatch
- Proxy with API key injection
- On-demand container creation
- Log streaming to issue timeline

### Phase 2
- Pro tier: Self-hosted Gateway
- Advanced quota management
- Container reuse optimization

---

## Success Metrics

| Metric | Target |
|--------|--------|
| Container cold start time | < 30s |
| Log streaming latency | < 500ms |
| Task success rate | > 95% |
| API key never exposed to frontend | 100% |
