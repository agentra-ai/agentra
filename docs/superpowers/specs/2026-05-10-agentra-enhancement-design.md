# Agentra Enhancement Design

**日期**: 2026-05-10
**状态**: Approved
**目标**: 增强 Agentra 核心竞争力，缩小与竞品差距

---

## 1. 概述

### 1.1 目标

基于竞品分析 (swarmclaw 23+ providers, hindsight memory, Overseer VCS)，增强 Agentra 五个关键功能：
1. 多 LLM Provider 支持
2. GitHub VCS 集成
3. Execution Traces
4. Memory 自动学习
5. Swarm/Agent Delegation

### 1.2 设计决策

| 功能 | 选择 | 理由 |
|------|------|------|
| Provider | C - 混合方案 | 统一接口 + provider-specific 扩展，平衡灵活性和一致性 |
| GitHub | B - GitHub App + Webhooks | 完整双向同步，需用户授权 |
| Traces | C - Hybrid | task_runs 存 summary，trace_logs 存详细 |
| Memory | A + B + C 全部 | Task completion + continuous + configurable threshold |
| Delegation | D - Hybrid | DAG 自动判断串行/并行 |

---

## 2. 架构概览

```
Agentra Enhanced
├── pkg/agent/backends/         # 多 Provider 支持
│   ├── backend.go             # 统一 Backend interface
│   ├── claude.go              # Claude Code backend
│   ├── openai.go              # OpenAI GPT backend
│   ├── gemini.go              # Google Gemini backend
│   ├── ollama.go              # Ollama (local) backend
│   ├── openrouter.go          # OpenRouter aggregate
│   └── provider_facade.go      # 统一调度 facade
│
├── pkg/github/                 # GitHub VCS 集成
│   ├── app.go                 # GitHub App OAuth
│   ├── webhooks.go            # Webhook 处理
│   ├── pr.go                  # PR operations
│   ├── commit.go              # Commit operations
│   └── sync.go                # Issue↔PR linking
│
├── pkg/traces/                 # Execution Traces
│   ├── recorder.go            # 记录 steps
│   ├── summarizer.go          # 生成 summary
│   ├── viewer.go              # Trace viewer API
│   └── types.go               # Trace structures
│
├── pkg/memory/                 # Memory 自动学习 (已存在)
│   ├── hooks/
│   │   ├── task_completion.go # A: Task completion trigger
│   │   ├── continuous.go      # B: Continuous capture
│   │   └── extractor.go       # C: Configurable threshold
│   └── service.go
│
└── pkg/taskgraph/              # Swarm Delegation
    ├── delegation.go          # DAG-based 调度决策
    ├── executor.go            # Parallel/sequential 执行
    └── container.go           # Docker 隔离
```

---

## 3. 多 Provider 支持

### 3.1 Backend Interface (统一接口)

```go
// pkg/agent/backend.go
type Backend interface {
    // 通用操作
    Execute(ctx context.Context, prompt string, opts *ExecuteOptions) (*Result, error)
    StreamingExecute(ctx context.Context, prompt string, opts *ExecuteOptions, ch chan<- string) error

    // Provider 信息
    Provider() ProviderType
    Model() string
    Capabilities() *Capabilities

    // Provider-specific 扩展
    ExtendedExecute(ctx context.Context, prompt string, opts *ExecuteOptions) (any, error)
}

type ExecuteOptions struct {
    WorkingDirectory string
    EnvVars          map[string]string
    Timeout          time.Duration
    Skill            *Skill
    Context          []MemoryEntry
}

type Result struct {
    Output       string
    ExitCode     int
    Duration     time.Duration
    TokensUsed   int
    Cost         float64
    Steps        []Step
}

type Step struct {
    Timestamp time.Time
    Action    string
    Tool      string
    Input     string
    Output    string
    Tokens    int
}
```

### 3.2 Provider 实现

| Provider | 文件 | 状态 |
|----------|------|------|
| Claude Code | `claude.go` | 已存在 |
| OpenAI GPT | `openai.go` | 新增 |
| Google Gemini | `gemini.go` | 新增 |
| Ollama (local) | `ollama.go` | 新增 |
| OpenRouter | `openrouter.go` | 新增 |
| DeepSeek | `deepseek.go` | 新增 |
| Groq | `groq.go` | 新增 |

### 3.3 Provider Facade (调度层)

```go
type ProviderFacade struct {
    backends map[ProviderType]Backend
    default  ProviderType
}

func (f *ProviderFacade) Execute(ctx context.Context, provider ProviderType, prompt string, opts *ExecuteOptions) (*Result, error) {
    backend, ok := f.backends[provider]
    if !ok {
        backend = f.backends[f.default]
    }
    return backend.Execute(ctx, prompt, opts)
}
```

### 3.4 Agent 配置扩展

```sql
ALTER TABLE agents ADD COLUMN provider TEXT NOT NULL DEFAULT 'claude';
ALTER TABLE agents ADD COLUMN model_override TEXT;
ALTER TABLE agents ADD COLUMN provider_config JSONB DEFAULT '{}';
```

---

## 4. GitHub VCS 集成

### 4.1 GitHub App OAuth Flow

```
1. User clicks "Connect GitHub"
2. Redirect to GitHub App authorization
3. User installs App on their repo
4. GitHub redirects back with installation_id
5. Store installation_id + tokens in database
6. Webhooks begin flowing
```

### 4.2 数据库 Schema

```sql
CREATE TABLE github_installations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    installation_id BIGINT NOT NULL,
    account_login TEXT NOT NULL,
    account_type TEXT NOT NULL,
    access_token TEXT NOT NULL, -- encrypted
    refresh_token TEXT, -- encrypted
    token_expires_at TIMESTAMPTZ,
    repositories JSONB DEFAULT '[]',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE github_issue_links (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    issue_id UUID NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    repository TEXT NOT NULL, -- owner/repo
    pr_number INT,
    commit_sha TEXT,
    branch_name TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX github_installations_workspace_idx ON github_installations(workspace_id);
CREATE INDEX github_issue_links_issue_idx ON github_issue_links(issue_id);
```

### 4.3 Webhook 处理

| Event | Handler | Action |
|-------|---------|--------|
| `pull_request` | `handlePR()` | Sync PR status to linked issue |
| `push` | `handlePush()` | Link commits to issues |
| `issue_comment` | `handleComment()` | Sync comments |

### 4.4 PR Operations

```go
type GitHubService struct {
    installations map[uuid.UUID]*Installation
    client        *github.Client
}

func (s *GitHubService) CreatePR(ctx context.Context, workspaceID, repo string, opts *PROptions) (*PR, error) {
    // Creates branch, opens PR, returns PR number
}

func (s *GitHubService) UpdatePRStatus(ctx context.Context, repo string, prNumber int, status string) error {
    // Update PR check status (pending/success/failure)
}

func (s *GitHubService) LinkIssueToPR(ctx context.Context, issueID, repo string, prNumber int) error {
    // Store link in github_issue_links table
}
```

### 4.5 自动 Commit on Task Complete

```go
func (s *GitHubService) CommitOnTaskComplete(ctx context.Context, task *AgentTask) error {
    // 1. Get linked issue
    // 2. Create/update branch: agentra/{issue-id}-{timestamp}
    // 3. Add commit with task result summary
    // 4. Create PR if not exists
}
```

---

## 5. Execution Traces

### 5.1 数据库 Schema

```sql
CREATE TABLE task_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL REFERENCES agent_tasks(id) ON DELETE CASCADE,
    agent_id UUID REFERENCES agents(id),
    status TEXT NOT NULL,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    duration_ms INT,
    exit_code INT,

    -- Summary fields
    total_steps INT DEFAULT 0,
    total_tokens INT DEFAULT 0,
    total_cost USD,

    -- Output
    output TEXT,
    error TEXT,

    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE trace_steps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_run_id UUID NOT NULL REFERENCES task_runs(id) ON DELETE CASCADE,
    step_number INT NOT NULL,
    timestamp TIMESTAMPTZ NOT NULL,
    action TEXT NOT NULL, -- 'tool_call', 'output', 'error', 'thinking'
    tool TEXT, -- tool name if tool_call
    input_text TEXT,
    output_text TEXT,
    tokens_used INT DEFAULT 0,
    duration_ms INT,
    metadata JSONB DEFAULT '{}',

    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX task_runs_task_id_idx ON task_runs(task_id);
CREATE INDEX task_runs_agent_id_idx ON task_runs(agent_id);
CREATE INDEX trace_steps_task_run_id_idx ON trace_steps(task_run_id);
CREATE INDEX trace_steps_timestamp_idx ON trace_steps(timestamp);
```

### 5.2 Trace Recorder

```go
type TraceRecorder struct {
    db     *pgxpool.Pool
    taskID uuid.UUID
    steps  []TraceStep
    mu     sync.Mutex
}

func (r *TraceRecorder) RecordStep(ctx context.Context, step *TraceStep) error {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.steps = append(r.steps, *step)

    // Batch write every 10 steps or on flush
    if len(r.steps) >= 10 {
        return r.flush(ctx)
    }
    return nil
}

func (r *TraceRecorder) flush(ctx context.Context) error {
    _, err := r.db.Exec(ctx, `
        INSERT INTO trace_steps (task_run_id, step_number, timestamp, action, tool, input_text, output_text, tokens_used, duration_ms, metadata)
        SELECT $1, generate_series, ...
    `, r.taskID)
    r.steps = r.steps[:0]
    return err
}
```

### 5.3 Trace Summarizer

```go
type TraceSummarizer struct{}

func (s *TraceSummarizer) Summarize(steps []TraceStep) *TaskRunSummary {
    summary := &TaskRunSummary{
        TotalSteps:  len(steps),
        TotalTokens: sumTokens(steps),
        TotalCost:   calculateCost(steps),
        Duration:    steps[len(steps)-1].Timestamp.Sub(steps[0].Timestamp),
    }

    // Categorize by tool
    summary.ToolUsage = categorizeByTool(steps)

    // Extract key actions
    summary.KeyActions = extractKeyActions(steps)

    return summary
}
```

### 5.4 Trace API

| Method | Path | 描述 |
|--------|------|------|
| GET | `/api/tasks/{id}/trace` | 获取完整 trace |
| GET | `/api/tasks/{id}/trace/summary` | 获取 summary |
| GET | `/api/agents/{id}/traces` | 获取 agent 所有 traces |
| GET | `/api/traces/analytics` | 获取 analytics 数据 |

---

## 6. Memory 自动学习

### 6.1 三种触发机制

```go
// hooks/task_completion.go (A: Task completion trigger)
func OnTaskComplete(ctx context.Context, task *AgentTask, result *TaskResult) error {
    // Extract learnings from task result
    learnings := ExtractLearnings(result.Output)

    // Store each learning as memory
    for _, learning := range learnings {
        err := memoryService.Store(ctx, &StoreInput{
            WorkspaceID: task.WorkspaceID,
            AgentID:     task.AgentID,
            Type:       "learning",
            Content:    learning,
        })
    }
    return nil
}

// hooks/continuous.go (B: Continuous capture)
func ContinuousCapture(ctx context.Context, step *TraceStep) error {
    // Evaluate if step contains valuable memory
    if ShouldCapture(step) {
        return memoryService.Store(ctx, &StoreInput{
            WorkspaceID: step.WorkspaceID,
            AgentID:     step.AgentID,
            Type:       "context",
            Content:    step.OutputText,
            Metadata:   map[string]any{"step": step.StepNumber},
        })
    }
    return nil
}

// hooks/extractor.go (C: Configurable threshold)
func ConfigurableExtractor(ctx context.Context, task *AgentTask) error {
    config := getExtractionConfig(task.WorkspaceID)

    // Check if threshold reached
    if task.TotalTokens > config.TokenThreshold ||
       task.TotalSteps > config.StepThreshold {
        return extractAndStore(task, config)
    }
    return nil
}
```

### 6.2 Learning Extractor

```go
type LearningExtractor struct {
    llmClient LLMClient
}

func (e *LearningExtractor) ExtractLearnings(output string) []string {
    prompt := fmt.Sprintf(`
Extract key learnings from this agent task output.
Focus on: decisions made, patterns discovered, caveats to remember.
Return as JSON array of strings.

Output:
%s
`, output)

    response := e.llmClient.Complete(prompt)
    return parseLearnings(response)
}
```

### 6.3 Memory 类型扩展

| Type | Trigger | Storage |
|------|---------|---------|
| `learning` | Task completion | Always stored |
| `context` | Continuous capture | When ShouldCapture() |
| `pattern` | Configurable threshold | When threshold reached |
| `task_result` | On error only | On failure |

---

## 7. Swarm/Agent Delegation

### 7.1 Delegation Scheduler

```go
type DelegationScheduler struct {
    store     *GraphStore
    executor   *Executor
    container  *ContainerManager
}

func (s *DelegationScheduler) Schedule(ctx context.Context, issueID string) error {
    // 1. Get ready nodes (dependencies satisfied)
    readyNodes, err := s.store.GetReadyNodes(ctx, issueID)
    if err != nil {
        return err
    }

    // 2. Classify by dependency
    parallelNodes := []GraphNode{}
    sequentialChains := [][]GraphNode{}

    for _, node := range readyNodes {
        if hasBlockingDependencies(node) {
            // Add to sequential chain
            addToSequentialChain(node, sequentialChains)
        } else {
            parallelNodes = append(parallelNodes, node)
        }
    }

    // 3. Execute parallel nodes concurrently
    if len(parallelNodes) > 0 {
        go s.executeParallel(parallelNodes)
    }

    // 4. Execute sequential chains
    for _, chain := range sequentialChains {
        go s.executeSequential(chain)
    }

    return nil
}
```

### 7.2 Docker Container Isolation

```go
type ContainerManager struct {
    dockerClient *docker.Client
    image        string
}

func (m *ContainerManager) Execute(ctx context.Context, node *GraphNode, prompt string) (*Result, error) {
    // 1. Start container with agent workspace
    container, err := m.dockerClient.ContainerCreate(ctx, &ContainerConfig{
        Image: m.image,
        Env:   []string{fmt.Sprintf("AGENT_PROMPT=%s", prompt)},
    })
    if err != nil {
        return nil, err
    }

    // 2. Wait for completion
    status, err := container.Wait(ctx)
    output, _ := container.Logs(ctx)

    // 3. Clean up
    container.Remove(ctx)

    return parseResult(status, output)
}
```

### 7.3 RBAC for Delegation

```sql
CREATE TABLE agent_delegation_policies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    from_agent_id UUID REFERENCES agents(id),
    to_agent_type TEXT NOT NULL, -- 'planner', 'executor', 'synthesis'
    max_depth INT DEFAULT 3,
    allow_parallel BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
```

### 7.4 Token Governance

```go
type TokenBudget struct {
    AgentID     uuid.UUID
    DailyLimit  int
    MonthlyLimit int
    UsedToday   int
    UsedMonth   int
}

func (t *TokenBudget) CanSpend(tokens int) bool {
    return t.UsedToday+tokens <= t.DailyLimit &&
           t.UsedMonth+tokens <= t.MonthlyLimit
}
```

---

## 8. API 设计

### 8.1 新增 Endpoints

| Method | Path | 描述 |
|--------|------|------|
| POST | `/api/agents/{id}/switch-provider` | 切换 agent provider |
| GET | `/api/workspaces/{id}/github/installations` | 获取 GitHub 安装列表 |
| POST | `/api/workspaces/{id}/github/connect` | 连接 GitHub App |
| DELETE | `/api/workspaces/{id}/github/disconnect` | 断开 GitHub |
| GET | `/api/issues/{id}/trace` | 获取 issue 的 trace |
| GET | `/api/traces/analytics` | Trace 分析数据 |
| POST | `/api/tasks/{id}/delegate` | 手动触发 delegation |

### 8.2 Webhook Endpoints

| Path | 描述 |
|------|------|
| POST | `/webhooks/github` - GitHub webhook receiver |

---

## 9. 前端 UI

### 9.1 Trace Viewer

**页面**: `/agents/{id}/traces`

组件:
- `TraceList.tsx` - 列表视图
- `TraceDetail.tsx` - 单个 trace 详情
- `TraceTimeline.tsx` - 可视化时间线
- `TraceAnalytics.tsx` - 统计图表

### 9.2 GitHub Integration

**页面**: `/settings/github`

组件:
- `GitHubConnect.tsx` - 连接 flow
- `RepoSelector.tsx` - 选择仓库
- `PRStatusBadge.tsx` - Issue 页面上的 PR badge

### 9.3 Provider Switcher

**页面**: `/agents/{id}/settings`

组件:
- `ProviderSelect.tsx` - 下拉选择 provider
- `ModelOverride.tsx` - 模型覆盖输入

---

## 10. 数据流

### 10.1 Multi-Provider Flow

```
User selects agent provider
    ↓
Agent.create/update with provider config
    ↓
Daemon polls for tasks with provider
    ↓
BackendFactory.create(provider) → Backend instance
    ↓
Backend.Execute(prompt) → unified Result
```

### 10.2 GitHub Sync Flow

```
GitHub App installed
    ↓
Webhook events flow to /webhooks/github
    ↓
WebhookHandler validates + parses event
    ↓
Event-specific handler (PR/Push/Comment)
    ↓
Update database + broadcast WebSocket
    ↓
Frontend receives update
```

### 10.3 Trace Recording Flow

```
Agent executes task
    ↓
TraceRecorder.RecordStep() called per step
    ↓
Batch write to trace_steps table
    ↓
Task complete → TraceSummarizer generates summary
    ↓
task_runs table updated with summary
    ↓
Frontend fetches via API
```

---

## 11. 错误处理

### 11.1 Provider Failures

```go
func (f *ProviderFacade) ExecuteWithFallback(ctx context.Context, prompt string, opts *ExecuteOptions) (*Result, error) {
    primary := f.backends[f.default]

    result, err := primary.Execute(ctx, prompt, opts)
    if err != nil {
        // Try fallback providers
        for _, fallback := range f.fallbacks {
            result, err = fallback.Execute(ctx, prompt, opts)
            if err == nil {
                return result, nil
            }
        }
        return nil, fmt.Errorf("all providers failed: %v", err)
    }
    return result, nil
}
```

### 11.2 GitHub Auth Expiry

```go
func (s *GitHubService) refreshToken(installation *Installation) error {
    // Check if token expired
    if time.Now().After(installation.TokenExpiresAt) {
        // Use refresh token to get new access token
        newToken, err := s.githubClient.UpdateToken(installation.RefreshToken)
        if err != nil {
            return fmt.Errorf("failed to refresh token: %v", err)
        }
        // Update installation
        s.store.UpdateToken(installation.ID, newToken)
    }
    return nil
}
```

### 11.3 Container Isolation Failures

```go
func (m *ContainerManager) ExecuteWithTimeout(ctx context.Context, node *GraphNode, prompt string, timeout time.Duration) (*Result, error) {
    ctx, cancel := context.WithTimeout(ctx, timeout)
    defer cancel()

    result, err := m.Execute(ctx, node, prompt)
    if err != nil {
        if ctx.Err() == context.DeadlineExceeded {
            // Clean up container and mark node failed
            s.store.TransitionNode(ctx, node.ID, StatusFailed)
            return nil, fmt.Errorf("task timed out after %v", timeout)
        }
    }
    return result, err
}
```

---

## 12. 测试策略

### 12.1 Unit Tests

| Module | Tests |
|--------|-------|
| `pkg/agent/backends/` | 每个 provider 的 Execute/Streaming 测试 |
| `pkg/github/` | App auth, webhook parsing, PR operations |
| `pkg/traces/` | Recorder, summarizer, query accuracy |
| `pkg/memory/hooks/` | Extraction logic, threshold triggers |
| `pkg/taskgraph/` | Scheduler decisions, container management |

### 12.2 Integration Tests

- Provider chain: switch provider → execute → verify result
- GitHub: install App → create PR → receive webhook → verify link
- Trace: execute task → verify steps recorded → verify summary

### 12.3 E2E Tests

- Full agent task with trace recording
- GitHub PR creation and status sync
- Multi-agent delegation with Docker containers

---

## 13. 依赖关系

```
Memory Auto-Learning (4)
    ↑ depends on
Memory Service (已存在)
    ↑ depends on
pgvector setup (已存在)

Swarm Delegation (5)
    ↑ depends on
Task Graph (已存在)
    ↑ depends on
GraphStore/Scheduler (已存在)

Multi-Provider (1)
    ↓ enables
Swarm Delegation (5) - 需要多 provider 支持

GitHub Integration (2)
    ↓ independent

Execution Traces (3)
    ↓ independent
```

---

## 14. 实现顺序

| Order | Feature | Reason |
|-------|---------|--------|
| 1 | Execution Traces | 基础，记录数据 |
| 2 | Multi-Provider | 扩展 agent 能力 |
| 3 | GitHub Integration | VCS 同步 |
| 4 | Memory Auto-Learning | 智能化 |
| 5 | Swarm Delegation | 高级编排 |

---

## 15. 竞品对齐

| Feature | Agentra | swarmclaw | Overseer | hindsight |
|---------|---------|-----------|---------|-----------|
| Multi-Provider | → 23+ | 23+ ✅ | 1 | 1 |
| VCS Integration | → ✅ | ❌ | ✅ | ❌ |
| Execution Traces | → ✅ | ❌ | ❌ | ❌ |
| Memory Auto-Learning | → ✅ | ✅ | partial | ✅ |
| Swarm Delegation | → ✅ | ✅ | ❌ | ❌ |
| Real-time WebSocket | ✅ | ❌ | ❌ | ❌ |
| Task Graph | ✅ | partial | ❌ | ❌ |

---

## 16. 结论

通过这五个增强，Agentra 将成为:
- **最灵活的 Agent 平台**: 支持 23+ providers
- **最完整的 VCS 集成**: GitHub App 双向同步
- **最可观测的系统**: 完整 execution traces
- **最智能的 Agent**: 自动 learning 和 memory
- **最协调的多 agent**: DAG-based delegation

与竞品相比，Agentra 的独特优势:
- Real-time WebSocket + Task Graph + Agent Memory 的组合
- Enterprise features (multi-workspace, RBAC, JWT)
- Open source + self-hosted