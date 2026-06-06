# Agentic Engineering Loop 设计规格

**日期**: 2026-06-06
**状态**: Draft v2
**作者**: brainstorm with user
**目标读者**: Agentra 维护者 + 后续实现 plan 的 agent

---

## 1. 概述

为 Agentra 添加 Agentic Engineering Loop 能力:接受一个 issue id,自动按 Plan → Develop → Review → Fix 循环推进,直到产出可合并的 PR。

**v2 的核心思路**:**Loop 不是一个新子系统,而是现有 task 系统的一种新使用模式**。每个 stage 是 `tasks` 表里的一条记录,supervisor 只是一个监听 task 完成事件、决定下一步的轻量级 service。

v1 把它做成了独立的"loop"子系统(新包 + 3 张新表 + 新 worker + 新 supervisor 状态机),代码量约 3000 行。v2 直接复用现有 task graph / daemon / REST / WS / UI 基础设施,估计 ~1100 行。功能等价,集成度更高。

### 1.1 为什么 v2 复用 task 系统

| 现有能力 | v2 怎么用 |
|----------|-----------|
| `tasks` 表 | 存每个 stage,带 `task_type` 区分 |
| `task_artifacts` 表 | 存 plan / diff / review 内容 |
| `internal/daemon/` 任务 worker | 复用,加 4 个 routing rule |
| `pkg/agent.Backend` 接口 | 不动,每个 stage 传不同的 system prompt + tools |
| `internal/events` 事件总线 | LoopCoordinator 订阅 `task:completed` 事件 |
| WS 事件 | 现有 `task:*` 事件自动推送给前端,loop 进度=issue 下的 tasks 进度 |
| `internal/cli/` | 加 `loop` 子命令,复用 `cli.APIClient` |
| `internal/handler/` | 加 loop 相关 REST endpoint |
| Issue 页面 UI | 现有 tasks 视图直接展示 loop 进度,**零 UI 工作量** |

### 1.2 关键设计决策

| 决策 | 选择 | 理由 |
|------|------|------|
| Loop 状态存储 | 1 张新表 `loops` | 跟踪 iteration / status / PR metadata;塞 task metadata 会污染语义 |
| Stage 实现 | `tasks` 表 + 4 个新 task_type | 复用 daemon、artifacts、status 流转 |
| 状态机 | `LoopCoordinator` 监听 task:completed 事件 | 事件驱动比轮询更省,跟现有事件流一致 |
| 包位置 | `server/internal/loop/` | 这是 Agentra 内部实现细节,不是 reusable 库 |
| 触发 | 新 CLI 子命令 + REST endpoint | 与现有 `agentra` CLI 一致 |
| UI | 复用 issue 页面的 tasks/comments 视图 | 零 UI 工作量,loop 任务自然作为 issue 子任务展示 |
| 触发器 | 一次 `loop start` 启动,后台异步跑 | 一次 loop 几分钟到几小时,不能阻塞 CLI |

---

## 2. 目标 / 非目标

### 2.1 目标 (MVP)

- 接受 issue id,自动跑 Plan → Develop → Review → (Fix → Review)* 循环
- Review 通过时自动创建 PR
- Loop 状态可查、可暂停、可恢复、可取消
- 每个 stage 复用现有 `pkg/agent.Backend` 接口
- 与现有 web UI 集成:issue 页面里能看到 loop 进度
- Dogfooding:用这个 loop 开发 Agentra 自己

### 2.2 非目标 (out of MVP)

- Architect / QA / Deploy 角色
- 跨 workspace 的 loop
- Loop 编排可视化 UI(用现有 issue/task 视图)
- 多 LLM provider 在同一 loop 中混用
- Loop self-healing / 自动回滚
- Loop template(不同 issue type 配不同 stage 组合)

---

## 3. 架构

### 3.1 总体形态

```
┌──────────────┐
│ CLI / Web    │  agentra loop start ISSUE-123
└──────┬───────┘
       │ POST /api/loops
       ▼
┌──────────────────┐
│ LoopCoordinator  │  server/internal/loop/coordinator.go
│  (state machine) │
└──────┬───────────┘
       │ creates → tasks table (loop_plan/loop_develop/loop_review/loop_fix)
       │
       ▼
┌──────────────────┐
│  Daemon          │  internal/daemon/ (现有)
│  Task Workers    │  routes by task_type
└──────┬───────────┘
       │ pkg/agent.Backend.Execute()
       ▼
┌──────────────────┐
│  pkg/agent.      │  现有
│  Backend         │
└──────────────────┘
       │ 完成后发
       ▼
┌──────────────────┐
│ task:completed   │  现有事件
│   event          │
└──────┬───────────┘
       │ 订阅
       ▼
┌──────────────────┐
│ LoopCoordinator  │  决定下一 stage,创建下一 task
└──────────────────┘
       │
       ▼  (重复 plan → develop → review → fix*)
       │
       ▼ review.approved
┌──────────────────┐
│  gh pr create    │  创建 PR
└──────────────────┘
```

### 3.2 与 task 系统的边界

- LoopCoordinator **只** 写 `loops` 表和 `tasks` 表(创建任务)
- LoopCoordinator **只** 读 `tasks.status` 和 `task_artifacts`
- LoopCoordinator **不** 直接调 LLM,不直接动文件系统
- 真正干活的是 daemon task workers(已有)
- LoopCoordinator 订阅 `task:completed` 事件来推进状态机

---

## 4. 状态机

### 4.1 状态

`loops.status`:

| 状态 | 含义 |
|------|------|
| `pending` | 已创建,等待启动 |
| `running` | 至少一个 stage 在执行或可执行 |
| `paused` | 用户暂停;不再创建新 task;已有 task 跑完不创建下一 task |
| `done` | review.approved=true,PR 创建成功 |
| `failed` | review.approved=false 且 iteration >= max_iterations;或 unrecoverable error |
| `cancelled` | 用户取消 |

`loops.current_stage`:

| 值 | 含义 |
|----|------|
| `plan` \| `develop` \| `review` \| `fix` | 当前 stage |
| `NULL` | done / failed / cancelled |

`loops.iteration`: 已经走完的 fix 次数(0 = 还没 fix 过)。

### 4.2 转移

```
[pending]
  start() → [running, current_stage=plan]

[running, current_stage=plan]
  plan task done → [running, current_stage=develop]

[running, current_stage=develop]
  develop task done → [running, current_stage=review]

[running, current_stage=review]
  review.approved = true
    → create_pr() → [done]
  review.approved = false, iteration < max_iterations
    → [running, current_stage=fix, iteration++]
  review.approved = false, iteration >= max_iterations
    → [failed]

[running, current_stage=fix]
  fix task done → [running, current_stage=review]

[running, current_stage=*]
  unrecoverable error → [failed]
  pause() → [paused]
  cancel() → [cancelled]

[paused]
  resume() → [running, current_stage=<previous>]
```

### 4.3 失败与重试边界

- **Stage 内失败**:`tasks` 表自带的 retry(默认 3 次),不计入 loop iteration
- **Stage 彻底失败**:loop 进入 `failed`
- **Review 找到 issues**:计入 loop iteration(每次 fix 算一次)
- **max_iterations**:默认 5,创建 loop 时可指定

---

## 5. 数据模型

### 5.1 新表:loops

```sql
CREATE TABLE loops (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  issue_id UUID NOT NULL REFERENCES issues(id),
  workspace_id UUID NOT NULL REFERENCES workspaces(id),

  status TEXT NOT NULL DEFAULT 'pending',
  current_stage TEXT,                              -- plan/develop/review/fix
  iteration INT NOT NULL DEFAULT 0,
  max_iterations INT NOT NULL DEFAULT 5,

  -- 输出
  pr_url TEXT,
  pr_number INT,
  branch_name TEXT,

  -- 配置
  agent_id UUID REFERENCES agents(id),             -- 用哪个 LLM provider
  config JSONB NOT NULL DEFAULT '{}',              -- 后续:parallel/timeout 等

  started_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_loops_issue_id ON loops(issue_id);
CREATE INDEX idx_loops_status ON loops(status)
  WHERE status IN ('pending', 'running', 'paused');
```

**v1 的 `loop_stages` 表 v2 删掉**:每个 stage 就是 `tasks` 表里的一行,带 `task_type` 区分。Stage 状态通过 `tasks.status` 跟踪。

**v1 的 `loop_artifacts` 表 v2 删掉**:复用 `task_artifacts`,artifact `kind` 区分 `plan` / `diff` / `review` / `fix_diff`。

### 5.2 tasks 表加 1 列

```sql
ALTER TABLE tasks ADD COLUMN loop_id UUID REFERENCES loops(id);
```

`(可选)`:也可以用 `tasks.metadata` JSONB 存 `loop_id`,不破坏 schema。本 spec 默认用列,查询更高效。

### 5.3 4 个新 task_type

注册到现有 task_type 体系(假设是 enum 或 text):

| task_type | system prompt 模板要点 | 工具 | 输出 artifact (`kind`) |
|-----------|------------------------|------|--------------------------|
| `loop_plan` | "你是需求分析师。读 issue,产出结构化 plan:目标、涉及文件、步骤、验收。" | read_file, search_code | `plan` (markdown) |
| `loop_develop` | "你是开发者。读 plan,改文件,跑测试,commit 到分支 loop/<issue-id>-<n>。" | read_file, write_file, run_command, git_commit, git_push | `diff` (unified diff) |
| `loop_review` | "你是代码审查员。读 diff 和原 plan,产出 JSON: `{approved: bool, issues: [{file, line, severity, message}]}`。" | read_file, read_diff | `review` (JSON) |
| `loop_fix` | "你是开发者。读 review.issues,改文件,跑测试,新 commit。" | read_file, write_file, run_command, git_commit, git_push | `fix_diff` (unified diff) |

System prompt 模板存在 `server/internal/loop/prompts/*.md`(纯文本,易调试,后续可热更新)。

### 5.4 涉及文件改动总览

| 文件 | 动作 | 用途 |
|------|------|------|
| `migrations/0XX_loops.up.sql` | 新建 | loops 表 + tasks.loop_id 列 |
| `server/internal/loop/coordinator.go` | 新建 | 状态机,事件订阅,~300 行 |
| `server/internal/loop/stages/{plan,develop,review,fix}.go` | 新建 | 4 个 stage executor,~150 行/个 |
| `server/internal/loop/prompts/{plan,develop,review,fix}.md` | 新建 | system prompt 模板 |
| `server/internal/loop/loop.go` | 新建 | `Loop` struct,公共类型 |
| `server/internal/handler/loop.go` | 新建 | REST endpoint,~80 行 |
| `server/internal/cli/loop.go` | 新建 | CLI 子命令,~100 行 |
| `server/internal/daemon/routing.go` | 修改 | 加 4 个 task_type 的 routing rule |
| `server/internal/events/handlers.go` | 修改 | 注册 `LoopCoordinator` 订阅 `task:completed` |

新文件 ~9 个,修改文件 ~2 个。**对比 v1 的 5 张新表 + 1 个新 worker + 1 个新 supervisor,改动量小一档。**

---

## 6. 组件

### 6.1 LoopCoordinator

文件:`server/internal/loop/coordinator.go`

**职责**:
- `CreateLoop(ctx, issueID, opts) (*Loop, error)` — 创建 loop,初始 task=`loop_plan`
- `HandleTaskCompleted(ctx, task) error` — 订阅 `task:completed` 事件,推进状态机
- `Pause / Resume / Cancel(ctx, loopID)` — 用户操作
- `GetLoop(ctx, loopID) (*Loop, error)` — 查询

**不职责**:
- 不执行 task(daemon 干)
- 不直接调 LLM(Backend 干)
- 不动 `issues` / `comments` 表(只读)
- 不感知 task 内容(只读 status / artifacts 类型)

**关键设计**:Coordinator 启动时从 `loops` 表加载所有 `status='running' | 'paused'` 的 loop,监听事件;不维护内存状态,DB 是 single source of truth。这样 Coordinator 进程崩溃重启后状态不丢。

### 6.2 Stage Executors

文件:`server/internal/loop/stages/*.go`

每个 stage 一个函数:

```go
type StageExecutor func(ctx context.Context, task *Task) ([]Artifact, error)
```

返回的 artifacts 由 task 系统自动写入 `task_artifacts`。

**例:plan stage**(~100 行)

```go
package stages

func PlanExecutor(ctx context.Context, task *Task) ([]Artifact, error) {
    issue, err := getIssue(ctx, task.IssueID)
    if err != nil { return nil, err }

    session := &agent.Session{
        SystemPrompt: loadPrompt("plan.md"),
        Messages: []agent.Message{{
            Role: "user",
            Content: fmt.Sprintf("Issue: %s\n\n%s", issue.Title, issue.Description),
        }},
        Tools: []agent.Tool{readFileTool, searchCodeTool},
    }

    result, err := backend.Execute(ctx, session)
    if err != nil { return nil, err }

    return []Artifact{{Kind: "plan", Content: result.Text}}, nil
}
```

其他 3 个 stage 类似,只是 system prompt + tools + 输出处理不同。

### 6.3 Daemon Routing

`server/internal/daemon/routing.go` 加 4 个 case(伪代码):

```go
func routeTask(task *Task) (StageExecutor, bool) {
    switch task.Type {
    case "loop_plan":     return stages.PlanExecutor, true
    case "loop_develop":  return stages.DevelopExecutor, true
    case "loop_review":   return stages.ReviewExecutor, true
    case "loop_fix":      return stages.FixExecutor, true
    default:              return nil, false
    }
}
```

具体形态取决于现有 daemon 的 routing 机制(可能已经是 dispatcher 模式,只需注册)。**不需要新 worker**。

### 6.4 CLI

`server/internal/cli/loop.go`:

```bash
agentra loop start <issue-id> [--max-iterations=5] [--agent=<agent-id>]
agentra loop status <loop-id>
agentra loop pause <loop-id>
agentra loop resume <loop-id>
agentra loop cancel <loop-id>
agentra loop list [--status=running]
```

复用 `cli.APIClient` 调 REST,所有子命令输出用现有 formatter。

### 6.5 REST API

`server/internal/handler/loop.go`:

| Method | Path | Body | 用途 |
|--------|------|------|------|
| POST | /api/loops | `{issue_id, max_iterations?, agent_id?}` | 创建 loop,返回 `loop_id` |
| GET | /api/loops/:id | — | 查询 loop 状态 |
| GET | /api/loops | `?status=running&issue_id=X` | 列表查询 |
| POST | /api/loops/:id/pause | — | 暂停 |
| POST | /api/loops/:id/resume | — | 恢复 |
| POST | /api/loops/:id/cancel | — | 取消 |

### 6.6 WebSocket 事件

复用现有 `task:*` 事件(loop 任务也是 task,自动推送)。**可选**新增 1 个:

- `loop:status_changed` — loop 整体状态/阶段切换时广播,UI 可以快速响应

事件 payload 包含 `loop_id`,前端过滤。

### 6.7 Tool System Design

Stage 需要调用工具(读文件、写文件、跑命令、git 操作)。这里定义工具的接口、清单、按 stage 的作用域、沙箱策略。

#### 6.7.1 Tool 接口

```go
// server/internal/loop/tools/tool.go
type Tool interface {
    Name() string
    Description() string
    Schema() ToolSchema              // 给 LLM 的 JSON schema(走 tool_use 协议)
    Execute(ctx context.Context, args json.RawMessage) (Result, error)
}

type ToolSchema struct {
    Name        string                 `json:"name"`
    Description string                 `json:"description"`
    Parameters  map[string]interface{} `json:"parameters"`  // JSON schema
}

type Result struct {
    Content string  // 文本结果给 LLM
    Error   string  // 非空代表失败
    Stderr  string  // run_command 用,跟 stdout 分开
    ExitCode int    // run_command 用
}
```

工具是**纯函数**:`args → Result`,无状态,无副作用靠 Execute 自己管。每次调用都打日志(`tool_call` / `tool_result` / `tool_error` 事件)。

#### 6.7.2 Tool 清单(MVP)

| Tool | 用在哪些 stage | 关键参数 | 返回 |
|------|----------------|----------|------|
| `read_file` | Plan, Develop, Review, Fix | `path` | 文件内容(>10KB 截断) |
| `search_code` | Plan, Develop, Fix | `query`, `path?` | grep 结果 |
| `write_file` | Develop, Fix | `path`, `content` | void(失败返 error) |
| `run_command` | Develop, Fix | `cmd`, `timeout_sec?` | stdout/stderr/exit_code |
| `run_test` | Develop, Fix | `cmd?`(默认 `go test ./...` 或 `pnpm test`) | 测试结果摘要 |
| `git_status` | Develop, Fix | — | git status 输出 |
| `git_diff` | Develop, Review, Fix | `staged?`, `file?` | unified diff |
| `git_commit` | Develop, Fix | `message` | commit SHA |
| `git_push` | Develop, Fix | `remote`, `branch` | void |
| `create_branch` | Develop | `name` | void |
| `github_pr_create` | Develop(最后) | `title`, `body`, `base?` | PR URL + number |

Plan / Review 是只读 stage,只用 `read_file` / `search_code` / `git_diff`。Develop / Fix 是写 stage,有完整工具集。

#### 6.7.3 Tool 按 stage 分配

```go
// server/internal/loop/stages/tools.go
var toolsByStage = map[string][]string{
    "loop_plan":    {"read_file", "search_code"},
    "loop_develop": {"read_file", "search_code", "write_file", "run_command", "run_test",
                     "git_status", "git_diff", "git_commit", "git_push",
                     "create_branch", "github_pr_create"},
    "loop_review":  {"read_file", "git_diff"},
    "loop_fix":     {"read_file", "search_code", "write_file", "run_command", "run_test",
                     "git_status", "git_diff", "git_commit", "git_push"},
}
```

#### 6.7.4 Tool 安全策略

| 维度 | dogfood 模式(MVP) | 生产模式(post-MVP) |
|------|---------------------|---------------------|
| 文件系统 | 全权限(loop 跑在开发者本机) | 限制在 loop 专属 worktree |
| `run_command` | 任何命令,默认 timeout 5 分钟 | 命令白名单 + timeout 强制 |
| `git_*` | 开发者本机 git 配置 | 专用 bot account + token |
| `github_pr_create` | 开发者本机 `gh` 认证 | 专用 GitHub App token |
| 网络 | 任意 | 出站白名单 |

dogfood 模式下,工具直接走开发者本机环境(因为 loop 跑在开发者的机器上,改 Agentra 自己);生产模式(post-MVP)需要 worktree 隔离、bot account、沙箱命令。

#### 6.7.5 Tool 错误处理

- `read_file` 文件不存在:返回 error(message 含 "file not found, do not retry")
- `write_file` 写只读文件:返回 error
- `run_command` timeout:返回 partial output + timeout error
- `run_command` exit_code != 0:**不** 当作 tool 错误,return Result 包含 exit_code,让 LLM 看 stderr 自己判断
- `git_commit` 啥都没改:返回 error "no changes to commit"
- `git_push` 冲突:返回 error 包含 "merge conflict",stage fail
- `github_pr_create` 已经存在同标题 PR:返回 error 包含 PR URL(stage 仍然 fail,用户决定)

每个 tool 错误都会被 LLM 看到,LLM 可以决定怎么应对(改 prompt / 换思路 / 放弃)。Coordinator 不替 LLM 决定"放弃"。

#### 6.7.6 工具调用日志

每次工具调用都通过 `pkg/logger` 打:

```
[loop_id=xxx] tool=read_file args={path: "server/internal/loop/coordinator.go"}
[loop_id=xxx] tool=read_file result=1453_chars duration=12ms
[loop_id=xxx] tool=run_command args={cmd: "go test ./...", timeout: 300}
[loop_id=xxx] tool=run_command result={exit_code: 0, stdout: "...", stderr: ""} duration=2.3s
[loop_id=xxx] tool=github_pr_create args={title: "Fix AGENTRA-1", body: "..."}
[loop_id=xxx] tool=github_pr_create error="PR already exists: https://..."
```

日志带 `loop_id` / `issue_id` / `stage` / `iteration`,可以从 `internal/loop/loop.log` 反查任何一次 loop 跑的所有 tool 调用。Args 脱敏(不记 file content,只记 path)。

---

## 7. 集成点

### 7.1 现有 task 系统

- 新增 4 个 task_type,每个对应一个 StageExecutor
- `tasks` 表加 `loop_id` 列(可空)
- 复用 `task_artifacts` 存 plan / diff / review
- 复用 `tasks.status` 流转
- 复用 task 的 retry 机制(stage 内重试 3 次)

### 7.2 现有 daemon

- 现有 task worker 的 routing 加 4 个 case
- 不需要新 worker、不需要新进程

### 7.3 现有事件总线

- `internal/events` 已有 task:completed 事件
- LoopCoordinator 在启动时注册订阅
- (可选)新增 `loop:status_changed` 事件

### 7.4 现有 issue UI

- Issue 详情页已经展示 subagent 任务树 / 关联任务
- Loop 任务作为 `tasks` 表里带 `loop_id` 的行,自动展示
- 用户能看到"Plan in progress"、"Review found 2 issues" 等状态(就是 task status 翻译)
- **零 UI 代码改动**

### 7.5 现有 CLI

- `internal/cli/` 加 `loop.go` 子命令
- 复用 `cli.APIClient` 调 REST
- 复用 `cli.Formatter` 输出 JSON / table

### 7.6 现有 Backend 接口

- **不动** `pkg/agent.Backend`
- 每个 stage 调用 `Backend.Execute(ctx, session)` 时,传不同的 system prompt + tools
- session 上下文从 task metadata / issue 读

### 7.7 现有 git 集成

- 用 `gh` CLI 推 branch + 创建 PR
- branch 命名:`loop/<issue-id>-<iteration>`
- dogfood 模式下,loop 直接读写 `/Users/doug/ai/system/agentra` checkout
- 后续要做 sandbox 隔离(每个 loop 跑在独立 worktree / Docker),但 MVP 不做

---

## 8. 失败处理

### 8.1 失败分类

| 错误类别 | 例子 | 默认行为 |
|----------|------|----------|
| LLM 网络错误 | timeout, connection reset | stage retry 3 次(指数 backoff 1s/4s/16s) |
| LLM rate limit | 429 | stage retry 3 次(指数 backoff 10s/40s/160s) |
| LLM context exceeded | prompt too long | stage fail,loop fail(retry 不解决问题,需修 prompt) |
| LLM content filter | blocked output | stage fail,loop fail |
| Tool 错误 - 文件 | not found, permission denied | stage retry 1 次(LLM 可能自动改路径),然后 fail |
| Tool 错误 - git | conflict, no remote | stage fail,loop fail(让用户手动处理) |
| Tool 错误 - test | test failed | **不 fail**,记入 develop artifact,让 review stage 看到 |
| Tool 错误 - PR 创建 | gh 没装,token 失效 | loop fail,记录 `failure_reason='pr_create_failed'`,用户可 resume |
| 用户暂停 | CLI / REST / UI 操作 | LoopCoordinator 标记 `paused`;**不再创建新 task**;已有 task 跑完后停在原地 |
| 用户恢复 | 同上 | LoopCoordinator 恢复创建下一 task(从 `current_stage` 继续) |
| 用户取消 | 同上 | LoopCoordinator 取消所有进行中的 task,标记 `cancelled` |
| iteration >= max_iterations | review 一直找到 issues | loop `failed`,记录 `failure_reason='max_iterations_exceeded'` |
| Loop 总时长超时 | 单 loop 跑超过 24h | loop `failed`,记录 `failure_reason='loop_timeout'` |
| Stage 超时 | 单 stage 跑超过 30min | stage fail,loop fail(默认) |
| Coordinator 进程崩溃 | OOM, panic | 重启时从 DB 恢复(见 8.4) |

### 8.2 Timeout 配置

| 资源 | 默认值 | 配置项 |
|------|--------|--------|
| 单次 LLM call | 5 分钟 | `loops.config.llm_timeout_sec` |
| 单个 stage | 30 分钟 | `loops.config.stage_timeout_sec` |
| 整个 loop | 24 小时 | `loops.config.loop_timeout_sec` |
| LLM call 间空闲(轮询) | 不适用(Backend 是 streaming) | — |
| 单次 `run_command` | 5 分钟 | `run_command` args 里的 `timeout_sec`(默认 300) |
| 单次 `run_test` | 10 分钟 | 固定(测试可能长) |

创建 loop 时可覆盖默认值;`max_iterations` 默认 5,可指定 1-20。

### 8.3 Retry 策略

| 资源 | 重试次数 | backoff | 重试计数 |
|------|----------|---------|----------|
| LLM call(网络错误) | 3 | 指数 1s/4s/16s | **不** 计入 loop iteration |
| LLM call(rate limit) | 3 | 指数 10s/40s/160s | **不** 计入 |
| Tool 错误(文件) | 1 | 立即 | **不** 计入 |
| Tool 错误(git) | 0 | — | 直接 fail |
| Stage 失败 | 0(由 task 系统 retry) | — | 取决于 task 系统 |
| Loop 失败 | **0**(不自动重试 loop) | — | 用户主动 `agentra loop resume` |

**关键不变量**:**LLM call 的 retry 不计入 loop iteration**。iteration 只在 "Review 找到 issues → 进入 Fix" 时 +1。LLM 抽风不算,review 真的找到问题才算。

### 8.4 Coordinator 崩溃恢复

Coordinator 进程崩溃 / OOM / 重启时,启动逻辑:

1. 加载 `loops.status IN ('running', 'paused')` 的所有 loop
2. 对每个 loop,加载 `tasks.status='running'` 的 task:
   - 如果 task 超过 `stage_timeout_sec` → 标为 `failed`,触发 Coordinator 走 fail-loop 逻辑
   - 如果 task 未超时 → 标为 `interrupted`,让 daemon 重试(task 系统自带 retry 机制)
3. 重新订阅 `task:completed` 事件,从 `current_stage` 继续推进
4. 对 `loops.status='paused'` 的 loop,只订阅,不主动推进

**DB 是 single source of truth**。Coordinator 进程无内存状态需要恢复,所有数据从 `loops` / `tasks` / `task_artifacts` 表读。

### 8.5 Token 成本跟踪

每个 stage 完成后,把 LLM call 的 token 用量写入 `task_artifacts.metadata`:

```sql
UPDATE task_artifacts
SET metadata = metadata || jsonb_build_object(
  'input_tokens', $1,
  'output_tokens', $2,
  'wallclock_ms', $3,
  'cost_usd', $4
)
WHERE id = $5;
```

Loop 结束/失败时,Coordinator 汇总所有 stage 的 token 用量,写入 `loops.config.total_cost`(M2 起记录):

```json
{
  "total_cost": {
    "input_tokens": 1240000,
    "output_tokens": 320000,
    "estimated_usd": 12.40,
    "by_stage": {
      "plan":    { "input": 5000, "output": 2000 },
      "develop": { "input": 800000, "output": 200000 },
      "review":  { "input": 100000, "output": 5000 },
      "fix":     { "input": 335000, "output": 113000 }
    }
  }
}
```

用户在 issue 页面 / `agentra loop status` 里能看到这个 loop 花了多少 token。**M2 起实现**;M0/M1 先不实现,占位字段准备好。

---

## 9. 验收

### 9.1 功能验收 (M1)

- [ ] `agentra loop start AGENTRA-1` 创建一个 loop,返回 `loop_id`
- [ ] Loop 自动跑 plan → develop → review → (fix)* → done
- [ ] Review approved 时自动创建 PR,`loops.pr_url` 写入
- [ ] Review 找到 issues 时自动 fix 然后再 review
- [ ] `agentra loop status <loop_id>` 实时显示状态
- [ ] Issue 页面能看到 loop 进度(tasks 视图里展示)
- [ ] `agentra loop pause/resume/cancel` 生效
- [ ] Loop 失败时 `failure_reason` 字段记录具体原因

### 9.2 Dogfood 验收 (M2)

- [ ] 用这个 loop 修复一个真实 Agentra bug
- [ ] 修复 PR 由 loop 自动创建,可合并
- [ ] Loop 至少处理一次 review → fix → review 迭代
- [ ] Loop 创建的 PR 通过 CI

### 9.3 非功能验收

- [ ] `loop start` CLI 启动后**立即返回**,loop 后台跑
- [ ] Loop 状态实时可查(WS 推送)
- [ ] 失败有明确 `failure_reason`
- [ ] Coordinator 进程崩溃后重启能继续(无状态丢失)
- [ ] 单元测试覆盖 Coordinator 状态机所有转移
- [ ] 集成测试覆盖 plan → develop → review → done 全流程(mock LLM)

### 9.4 分阶段实现计划

6 个 milestone 推进,每个 milestone 都有可验证的产出和验收。**总计 ~9 个工作日**(单人,~2 周)。

#### M0: 数据模型 + REST 骨架 (1 天)

**产出**:
- `migrations/0XX_loops.up.sql`:`loops` 表 + `tasks.loop_id` 列
- `server/internal/loop/loop.go`:`Loop` struct + CRUD 函数(DB 层)
- `server/internal/handler/loop.go`:REST endpoint(GET / POST / list)
- 不创建实际 task,不订阅事件,不调 Backend

**验收**:
- `POST /api/loops {issue_id: X}` 创建 status=pending 的 loop
- `GET /api/loops/:id` 返回 loop
- `GET /api/loops?status=running` 列表查询
- `pnpm typecheck` + `go test ./...` 通过

#### M1: LoopCoordinator + Plan stage (2 天)

**产出**:
- `server/internal/loop/coordinator.go`:Coordinator 骨架 + `CreateLoop` + `HandleTaskCompleted`
- `server/internal/loop/stages/plan.go`:PlanExecutor(只读 issue,产出 plan markdown)
- `server/internal/loop/events.go`:事件订阅注册
- `server/internal/daemon/routing.go`:加 `loop_plan` case
- 4 个 system prompt 模板(只 plan 实际用,其他占位)

**验收**:
- `agentra loop start ISSUE-1` 创建 loop
- Loop 自动跑 Plan stage,产出 `plan` artifact
- 完成后 loop 状态 `running, current_stage=plan`(无 develop 转移,停在 plan)
- 用户能看到 plan 内容
- Coordinator 单元测试覆盖 `decideNextStage` 所有分支

#### M2: Develop stage (2 天)

**产出**:
- `server/internal/loop/stages/develop.go`:DevelopExecutor
- `server/internal/loop/tools/*.go`:11 个 tool 实现(read_file, write_file, run_command, run_test, git_*, github_pr_create)
- 状态机加 `plan → develop → done`(无 review,直接 done)

**验收**:
- Loop 跑 plan → develop → done
- Develop stage 真的改了 Agentra 代码,创建了 commit
- 完成后有 `diff` artifact
- Develop stage 失败时 loop fail,有 `failure_reason`
- 11 个 tool 各自单元测试覆盖(happy path + 关键 error path)

#### M3: Review stage (2 天)

**产出**:
- `server/internal/loop/stages/review.go`:ReviewExecutor
- 状态机加 `develop → review → done`(review 默认 approved,MVP 简化)
- Review JSON 解析逻辑

**验收**:
- Loop 跑 plan → develop → review → done
- Review 产出 `review` artifact(JSON: `{approved, issues[]}`)
- 默认 approved,loop 完成
- Review 单元测试覆盖(approved 路径 + 强制 issues 路径用 mock LLM)

#### M4: Fix stage + iteration (2 天)

**产出**:
- `server/internal/loop/stages/fix.go`:FixExecutor
- 状态机加 `review → fix → review` 转移
- `iteration` 计数,max_iterations 检查
- Loop 失败时 `failure_reason` 字段写入

**验收**:
- Review 找到 issues 时,loop 自动 fix 然后再 review
- iteration 计数正确(每次 fix +1)
- iteration >= max_iterations 后 loop `failed`,`failure_reason='max_iterations_exceeded'`
- 集成测试覆盖 review→fix→review 完整流程(mock LLM 强制 review 找 issues)

#### M5: CLI + UI 验证 (1 天)

**产出**:
- `server/internal/cli/loop.go`:`start` / `status` / `pause` / `resume` / `cancel` / `list` 子命令
- Web UI 验证:issue 页面能看到 loop 进度(走现有 task 视图,无新代码)
- WS 事件:`loop:status_changed`(可选)

**验收**:
- 所有 CLI 子命令工作
- Issue 页面显示 loop 状态(手动验证)
- E2E test:从 CLI 启动 loop,UI 看到进度,loop 完成

#### M6: 第一次 dogfood (1 天)

**产出**:
- 选一个真实的 Agentra bug(优先简单明确,影响小,无外部依赖)
- `agentra loop start <issue-id>` 启动 loop
- Loop 跑完整 plan→develop→review→fix→review→done 流程
- 创建 PR

**验收**:
- PR 由 loop 自动创建,可手动 review 后合并
- 至少一次 review→fix 迭代被触发
- Loop 创建的 PR 通过 CI
- 整个流程 token 成本 < $20(粗略上限)

**总计:M0-M6 约 9 个工作日**(单人)。可以并行 M0+M1 的部分工作(比如 schema 设计可以先做)。

### 9.5 测试策略

| 测试类型 | 目标覆盖率 | 工具 | 关键测试 |
|----------|------------|------|----------|
| **单元测试** | Coordinator 90%,Stage Executors 80%,Tools 100% | `go test` + `testify/assert` | `decideNextStage` 所有分支、tool 错误路径、JSON 解析 |
| **集成测试** | 关键流程 100% 覆盖 | `go test` + mock LLM(`httptest`) | plan→develop→review→done 全流程;review→fix→review;max_iterations 触发;Coordinator 崩溃恢复 |
| **E2E test** | MVP 验收 100% 覆盖 | Playwright + 真实 Backend(staging env) | CLI 启动 loop → UI 看状态 → loop 完成 → PR 创建 |
| **手动 dogfood** | 至少 1 次成功 | 开发者本机 | 用 loop 修复一个真实 bug,合并 PR |

**Mock LLM 模式**:
- 用 `httptest` 起一个 mock HTTP server 模拟 Anthropic API
- 按 test case 注入 response(approved / issues / error)
- 不需要真 LLM,CI 跑得快

**集成测试位置**:`server/internal/loop/integration_test.go`,需要数据库(test DB,M0 起就建)。

**E2E test 位置**:`e2e/tests/loop.spec.ts`,需要 backend + frontend + 测试 issue 准备好。

### 9.6 Observability

#### 9.6.1 OTel Traces

每个 stage 一个 root span,内含 LLM call 和 tool call 子 span:

```
[loop: AGENTRA-1, issue: 1, iter: 0]
  └── [stage: plan]
       └── [llm_call] input_tokens=4500, output_tokens=2200
       └── [tool: read_file] path=docs/specs/loop.md
  └── [stage: develop]
       └── [llm_call] input_tokens=8000, output_tokens=3000
       └── [tool: read_file] path=server/internal/loop/coordinator.go
       └── [tool: write_file] path=server/internal/loop/coordinator.go
       └── [tool: git_commit] sha=abc1234
       └── [tool: github_pr_create] url=https://...
```

Span attributes 至少包含:`loop_id`, `issue_id`, `iteration`, `stage_type`, `agent_id`, `duration_ms`, `token_input`, `token_output`。

#### 9.6.2 Metrics

通过现有 Prometheus exporter(若已有)暴露:

| Metric | Type | Labels |
|--------|------|--------|
| `loops_created_total` | counter | `workspace_id` |
| `loops_completed_total` | counter | `workspace_id`, `iterations` |
| `loops_failed_total` | counter | `workspace_id`, `failure_reason` |
| `loops_duration_seconds` | histogram | `workspace_id`, `status` |
| `loops_iterations_to_completion` | histogram | `workspace_id` |
| `stages_duration_seconds` | histogram | `stage_type`, `status` |
| `stages_llm_input_tokens_total` | counter | `stage_type`, `model` |
| `stages_llm_output_tokens_total` | counter | `stage_type`, `model` |
| `tools_call_total` | counter | `tool_name`, `status` |
| `tools_duration_seconds` | histogram | `tool_name` |

M0 起就埋点,不需要等 M6。

#### 9.6.3 Logs

结构化日志(JSON),带 `loop_id` / `issue_id` / `stage` / `iteration` / `tool` 字段。

关键事件:
- `loop.created` / `loop.started` / `loop.paused` / `loop.resumed` / `loop.cancelled`
- `loop.stage_started` / `loop.stage_completed` / `loop.stage_failed`
- `loop.iteration_incremented` / `loop.completed` / `loop.failed`
- `tool.called` / `tool.succeeded` / `tool.failed`

LLM call 的 args 脱敏:**不记** user 消息全文,只记长度;不记 file content,只记 path。

#### 9.6.4 UI 集成

不需要新 UI 代码——现有 `task:completed` 事件已经推送给前端,issue 页面里的 task 列表自动展示 loop 进度。可选新增 `loop:status_changed` 事件给前端做"loop 状态变化"的快闪。

---

## 10. v1 → v2 变更说明

| 维度 | v1 (用户不满意) | v2 (本文) | 简化效果 |
|------|-----------------|-----------|----------|
| 新表数量 | 3 (`loops` / `loop_stages` / `loop_artifacts`) | 1 (`loops`) | -2 表,-1 个 migration 文件 |
| Stage 数据 | 自定义 `loop_stages` 表 | 复用 `tasks` 表 + `task_type` 字段 | 减少 schema 重复 |
| Artifact 存储 | 自定义 `loop_artifacts` 表 | 复用 `task_artifacts` | 减少 schema 重复 |
| 包位置 | `server/pkg/loop/` | `server/internal/loop/` | 内部化,降低 API 表面 |
| 状态机宿主 | 自定义 Supervisor 协程 + 轮询 | LoopCoordinator 订阅 `task:completed` 事件 | 复用现有事件流,无轮询 |
| 新 worker | 独立的 loop worker 协程 | 复用 daemon task worker(加 routing rule) | -1 个 worker 实现 |
| Role 模型 | 4 个独立 Role 对象 + 接口 | 4 个 task_type + StageExecutor 函数 | 没有不必要的抽象 |
| 估算代码量 | ~3000 行 | ~1100 行 | -63% |
| 与 UI 集成 | 需要新写 loop 进度视图 | 复用现有 task 视图,零 UI 工作 | -N UI 文件 |
| 复用 task 重试 | 自己实现 | 复用 `tasks` 表 retry 机制 | 减少代码 |

**v2 的核心改进**:不把 loop 当成"新东西",而是"task 的一种用法"。所有 loop 行为都是普通 task 行为,只是被 Coordinator 串起来。

---

## 11. 未来工作 (out of MVP)

- Architect / QA / Deploy 阶段(在 task_type 体系里加 4 个 type + 1 个 state 转移)
- Loop 编排可视化 UI(用 `loops` 表数据画流程图)
- 多 LLM provider 在同一 loop 中混用(每 stage 可指定 agent_id)
- Loop self-healing(develop 失败时自动 revert 到上一稳定 commit)
- Workspace-level loop 策略(模板、并发上限)
- Loop template(不同 issue type 配不同 stage 组合 + 不同 system prompt)
- Sandbox 隔离(每个 loop 跑在独立 worktree 或 Docker)
- Loop 的成本统计(token 用量、wallclock)

---

## 12. 编排框架模式借鉴 (LangGraph / CrewAI)

LangGraph 和 CrewAI 是当下多 agent 编排的两条主流路径。本节把它们的抽象拉出来,跟 v2 的设计做一一对应,说明 v2 借鉴了什么、丢掉了什么、为什么没直接用它们的 Python 实现。

### 12.1 LangGraph 的核心抽象

LangGraph 是 LangChain 出的图编排框架,核心模型是显式 StateGraph:

| 抽象 | 含义 | v2 怎么对应 |
|------|------|-------------|
| **StateGraph** | 全局状态 + 转移规则的图 | `loops` 表 (`status` / `current_stage` / `iteration`) |
| **Node** | 读 state 改 state 的纯函数 | 4 个 `StageExecutor` (`server/internal/loop/stages/*.go`) |
| **Edge** | 节点间转移,可以条件分支 | Coordinator 里的 if/else(单一函数,无 DSL) |
| **Checkpointing** | 每个 node 完成后 state 持久化,可中断/恢复 | `tasks.status` 流转 + DB 是 single source of truth |
| **Human-in-the-loop** | 图运行到某点暂停,等外部信号 | `loops.status='paused'` + `pause/resume` REST |
| **Streaming** | 节点执行过程实时推给前端 | 现有 `task:*` WS 事件自动覆盖 |
| **Conditional edges** | 边条件可以是任意表达式 | Coordinator 里固定 4-5 个分支(plan→develop→review→fix/done/failed) |

### 12.2 CrewAI 的核心抽象

CrewAI 把多 agent 协作抽象成"剧组":

| 抽象 | 含义 | v2 怎么对应 |
|------|------|-------------|
| **Crew** | 一组 agent + 一个 process | 1 个 `Loop`(1 张 loops 表行) |
| **Agent** | role + goal + backstory + tools | 1 个 `task_type` + 1 份 system prompt(plan/develop/review/fix 各一份) |
| **Task** | 分配给某个 agent 的具体工作,带 expected_output | `tasks` 表里 1 行(带 `task_type` + `expected_artifact_kind`) |
| **Process** | 任务编排方式:sequential / hierarchical / consensual | v2 MVP 只有 sequential(状态机显式驱动) |
| **Memory** | 短/长/实体/上下文 4 类记忆 | 不引入。stage 间上下文通过 `task_artifacts` 显式传递(可追溯、可调试) |
| **Tools** | agent 可调用的函数 | 复用现有 `pkg/agent` tool 机制,每个 stage 的 tool 列表在 stage 注册时声明 |

### 12.3 v2 借鉴 vs 不借鉴的清单

| 模式 | 来源 | v2 处理 | 理由 |
|------|------|---------|------|
| 显式状态机 | LangGraph | ✅ 借鉴 | 我们 stage 之间的转移是有条件分支,显式状态机比 DAG 边条件更可读、可观测 |
| Stage 是纯函数 | LangGraph | ✅ 借鉴 | StageExecutor 是 `(ctx, task) → (artifacts, error)`,无副作用靠 daemon 包装 |
| 状态持久化 | LangGraph | ✅ 借鉴 | DB 是 single source of truth,Coordinator 崩了重启能恢复 |
| Conditional edge | LangGraph | ⚠️ 简化 | 不用 LangGraph 的边条件 DSL,Coordinator 里就 4-5 个 if/else 分支,可读性更好 |
| Human-in-the-loop | LangGraph | ✅ 借鉴 | pause/resume 命令对应 interrupt + resume 信号 |
| Streaming | LangGraph | ✅ 借鉴 | 复用现有 `task:*` 事件,不加新事件类型 |
| Role-based agent | CrewAI | ✅ 借鉴 | 每个 stage 一份 system prompt,角色差异显式可见 |
| Sequential process | CrewAI | ✅ 借鉴 | MVP 单线,4 个 stage 顺序跑 |
| Hierarchical / consensual process | CrewAI | ❌ 不借鉴 | MVP 单一 Coordinator,不需要 manager/worker 分层 |
| Memory layers | CrewAI | ❌ 不借鉴 | 引入会增加复杂度,stage 间上下文靠 `task_artifacts` 显式传递 |
| Tool registry | CrewAI | ⚠️ 简化 | 不引入独立的 tool registry,tools 在每个 stage 的 prompt 模板附近直接声明 |
| 边的可视化 | LangGraph Studio / CrewAI Studio | ❌ 不借鉴 | 状态机只有 4-5 个状态,文本展示就够;后续可加简单流程图 |

### 12.4 为什么不用 LangGraph / CrewAI 直接

**先纠正 v2 初稿的说法**:调研后确认,**LangGraph 有多个非官方 Go 端口**,CrewAI 没有 Go 端口但生态位上有 Go-native 替代品。

| 维度 | LangGraph (Python) | CrewAI (Python) | 结论 |
|------|--------------------|-----------------|------|
| 语言 | Python only | Python only | ⚠️ Python 生态 |
| 官方 Go port | ❌ 无 | ❌ 无 | LangGraph 没有官方 Go port |
| **非官方 Go port** | ✅ 有多个(见 12.5) | ⚠️ 仅 1 个 1⭐ 玩具项目 | LangGraph 侧有可选项,CrewAI 侧无 |
| 跟 Backend 接口兼容 | 绑定 LangChain 生态,要 adapter | 绑定自家 agent 抽象,要 adapter | ⚠️ 都要适配层 |
| 部署复杂度 | 加 Python 微服务(若直接用) | 加 Python 微服务(若直接用) | ⚠️ 混语言 = 一致性损失 |
| 图编排能力 | 强(复杂条件、并行、子图) | 弱(主要 sequential) | 我们用不到 LangGraph 强项 |
| 团队学习成本 | 中(图概念) | 低(更直观) | ⚠️ 不算决定性 |
| 维护风险 | LangChain API 经常 breaking | CrewAI 还在快速迭代 | ⚠️ 都不算稳 |

**核心判断**:
1. **不直接用 LangGraph/CrewAI 的 Python 实现**:理由仍是语言不一致 + 部署复杂度。
2. **不强制复用其 Go port**:非官方 Go port 都有碎片化、stars 低、API 跟 LangGraph Python 不完全对齐等风险,直接拿我们的 Backend 接口包一个"伪 LangGraph"是给将来挖坑。
3. **借鉴抽象、状态机、sequential/loop 模式**:这部分放在我们的自定义 Coordinator 里实现,300 行能说清楚。

### 12.4.1 但要承认:Go-native 多 agent 框架在 2026 已经不少了

调研时发现 Go 生态有 **3 类候选**值得放进 12.5 对比表(下面 12.5 详述):
- **直接对标 LangGraph 的 Go port**(`smallnest/langgraphgo` 261⭐、`dshills/langgraph-go` 8⭐)
- **OpenAI Agents Go SDK 的社区 port**(`nlpodyssey/openai-agents-go` 258⭐)
- **Go-native 多 agent 编排框架**(`AgenticGoKit` 155⭐——**这个最值得评估**)

具体对比见 12.5。结论不变:custom Coordinator 是 MVP 的正确选择,AgenticGoKit 是 post-MVP 演进的强候选。

### 12.5 Go 生态编排方案对比

调研了 Go 生态的多 agent / workflow 编排方案,**包括非官方 LangGraph Go port、Go-native 多 agent 框架、通用 workflow 引擎三类**:

#### 12.5.1 A 类:LangGraph / CrewAI 风格多 agent 框架

| 选项 | ⭐ | 维护 | License | 关键特性 | 适配度 |
|------|-----|------|---------|----------|--------|
| **`AgenticGoKit`** (`AgenticGoKit/AgenticGoKit`) | 155 | 活跃(2026-05-26 推过) | Apache 2.0 | **Sequential / Parallel / DAG / Loop** 4 种编排模式;Anthropic / OpenAI / Ollama 等多 LLM;**MCP tool**;OpenTelemetry;Beta | ⭐⭐⭐⭐ |
| **`nlpodyssey/openai-agents-go`** | 258 | 活跃(2026-03-26) | Apache 2.0 | OpenAI Python Agents SDK 的 Go port;Agent + Handoffs + Guardrails 3 个核心概念;examples 完整 | ⭐⭐⭐ |
| **`smallnest/langgraphgo`** | 261 | 2026-02-24 推过 | MIT | LangGraph Go port;功能强,但 137MB(可能含模型/样例资源) | ⭐⭐ |
| **`dshills/langgraph-go`** | 8 | 2025-11-18 推过 | MIT | 自称 "production-looking":stateful graph + deterministic replay + checkpointing + observability + Anthropic/OpenAI/Google/Bedrock 适配 | ⭐⭐ (stars 太低) |
| **`gocrewwai`** (`stealthrocket/gocrewwai`) | 9 | 2026-05-10 | (待查) | "Enterprise-grade CrewAI alternative for Go" — LangGraph-style Flow + A2A + MCP + HITL | ⭐⭐ |
| `crewai-go` (`captain-corgi-hub`) | 1 | 2024-10 起未推 | (待查) | "Generative AI Multi Agents in Go inspire by CrewAI" — 几乎无更新 | ⭐ |

**`AgenticGoKit` 是这类别里最值得评估的**:
- **DAG / Loop pattern** 是我们 v2 状态机想要的(Loop 模式直接对应 plan→develop→review→fix)
- **MCP tool 集成**正好对得上 agentra 已有的 `pkg/mcp` server
- **Anthropic 支持**是必需的(我们 Backend 主要调 Claude)
- **Apache 2.0** 允许商用
- **风险**:155 stars + "Beta" 状态,生产可用性需要自己验证

#### 12.5.2 B 类:通用 workflow 引擎(非 LLM 专用)

| 选项 | ⭐ | 维护 | License | 关键特性 | 适配度 |
|------|-----|------|---------|----------|--------|
| **Temporal** (`temporalio/temporal`) | 20778 | 活跃(2026-06-06) | MIT | 生产级 workflow 引擎:retries / timeouts / signals / versioning 全有;多语言 SDK | ⭐⭐⭐ |
| **Inngest** (`inngest/inngest`) | 5451 | 活跃(2026-06-05) | NOASSERTION | FaaS 风格 workflow;Go SDK 好;事件驱动;managed 或 self-host | ⭐⭐ |
| Cadence (Uber) | 5k+ | 活跃 | Apache 2.0 | Temporal 前身;Go-native 早;成熟 | ⭐⭐ |
| Restate | 1k+ | 活跃 | (待查) | 比 Temporal 轻量;单 binary | ⭐ |
| Hatchet | 1k+ | 活跃 | (待查) | 分布式任务队列 + workflow;非 LLM 专用 | ⭐ |
| Argo Workflows (`argoproj/argo-workflows`) | (high) | 活跃 | Apache 2.0 | K8s-native;非 LLM 专用;引入 K8s 依赖过重 | ⭐ |

Temporal / Inngest 是 Go 生态最成熟的 workflow 引擎,但**它们解决的是通用业务工作流**(订单处理、跨服务 saga、定时任务),不是 LLM agent 编排。LLM stage 的特殊性(非确定性、token 成本、人工 review、tool calling 错误处理)它们不直接管,得自己包一层。

#### 12.5.3 C 类:其他 Go AI/agent 工具(参考)

| 选项 | ⭐ | 备注 |
|------|-----|------|
| `gastown` (`stealthrocket/gastown`) | 15755 | "multi-agent workspace manager" — 大型平台,跟我们 MVP 不匹配 |
| `mudler/nib` | 21 | "tiny zero-dependency LLM agent harness" — 终端工具,非编排框架 |
| `superfly/contextwindow` | (low) | 低层 LLM agent 库 |
| `AgenticGoKit/agk` | - | AgenticGoKit 的 CLI(独立 sibling repo) |

#### 12.5.4 v2 选哪个

| 候选 | MVP | 理由 |
|------|-----|------|
| **`AgenticGoKit`** | ❌ 不上 | 关键风险:155 stars + "Beta" 状态,breaking change 概率高;集成要适配 MCP 和 Backend 接口;学习新抽象 |
| **`nlpodyssey/openai-agents-go`** | ❌ 不上 | 主要是 OpenAI API 绑定(虽然自称 provider-agnostic),不直接支持 Anthropic 为一等公民 |
| **`smallnest/langgraphgo`** | ❌ 不上 | 137MB 代码量,stars 跟 AgenticGoKit 相当但活跃度差;跟 LangGraph Python API 同步压力 |
| **`Temporal`** | ❌ 不上 | 引入 Temporal server 部署,跟"一 docker compose"理念冲突;杀鸡用牛刀 |
| **`Inngest`** | ❌ 不上 | 第三方 vendor;定价不透明 |
| **自定义 Coordinator** | ✅ **MVP 选这个** | 见下面理由 |

**v2 选自定义 Coordinator 的理由**:
1. **零新基础设施依赖**。Temporal / Inngest / Cadence / Restate 都需要独立 server,跟 agentra 的"一 docker compose 跑起来"理念冲突。
2. **零新代码依赖**。AgenticGoKit / nlpodyssey 都要拉新库、适配 API,学习新抽象;我们 4 stage 状态机 ~300 行说清楚。
3. **跟现有事件流打通**。Coordinator 订阅 `task:completed` 事件就行,不用引入新消息总线。
4. **API stability**。自己写的代码我们自己控制,不会因为 framework breaking change 拖累 release。
5. **后续可演进**。如果以后真要换,优先评估 AgenticGoKit(MCP + Anthropic + DAG/Loop pattern 最贴合),次选 Temporal(Go SDK 成熟)。Custom Coordinator 不会成为障碍,Stage 是单一函数,容易映射到外部框架的 Node。

### 12.6 实现细节:Coordinator 怎么消费 LangGraph-style 模式

Coordinator 关键代码骨架(伪 Go):

```go
// server/internal/loop/coordinator.go
package loop

type Coordinator struct {
    db        *sql.DB
    events    *events.Bus
    taskSvc   *service.TaskService
    backend   agent.Backend  // 现有 Backend 接口
}

func (c *Coordinator) HandleTaskCompleted(ctx context.Context, evt events.TaskCompleted) error {
    task, err := c.taskSvc.GetTask(ctx, evt.TaskID)
    if err != nil { return err }

    loop, err := c.getLoopByTaskID(ctx, task.LoopID)
    if err != nil { return err }
    if loop.Status != "running" { return nil }  // 用户暂停/取消后不再推进

    next, err := c.decideNextStage(loop, task)  // 状态机转移
    if err != nil { return c.failLoop(ctx, loop, err) }

    switch next.action {
    case "create_task":
        return c.taskSvc.CreateTask(ctx, next.taskSpec)
    case "complete":
        return c.completeLoop(ctx, loop, next.prURL)
    case "fail":
        return c.failLoop(ctx, loop, next.reason)
    }
    return nil
}

func (c *Coordinator) decideNextStage(loop *Loop, lastTask *Task) (Decision, error) {
    switch loop.CurrentStage {
    case "plan":
        return Decision{action: "create_task", taskSpec: developTask(loop)}, nil
    case "develop":
        return Decision{action: "create_task", taskSpec: reviewTask(loop)}, nil
    case "review":
        review := parseReviewArtifact(lastTask.Artifacts)
        if review.Approved {
            return Decision{action: "complete", prURL: ...}, nil
        }
        if loop.Iteration >= loop.MaxIterations {
            return Decision{action: "fail", reason: "max_iterations_exceeded"}, nil
        }
        return Decision{action: "create_task", taskSpec: fixTask(loop, review.Issues)}, nil
    case "fix":
        return Decision{action: "create_task", taskSpec: reviewTask(loop)}, nil
    }
    return Decision{}, fmt.Errorf("unknown stage: %s", loop.CurrentStage)
}
```

**这个 Coordinator 是 v2 借鉴 LangGraph 状态机 + CrewAI sequential process 的产物**,但用 Go 直接写,集成进现有 `internal/events` 订阅和 `internal/service.TaskService`,没有任何外部框架依赖。

### 12.7 演进路径:什么时候切换到 AgenticGoKit

如果以后出现以下信号,重新评估并可能切换到 `AgenticGoKit`:

| 触发条件 | 理由 |
|----------|------|
| AgenticGoKit 达到 1k+ stars | 社区验证足够,降低"项目死了"风险 |
| AgenticGoKit 发布 v1.0 (脱离 Beta) | API stability 有保证 |
| Loop stage 数量超过 6 个 | 状态机管理成本上升,框架抽象收益变高 |
| Loop 需要并行 stage / sub-loop | 我们当前的 flat state machine 表达力不够 |
| Loop 需要可视化编排 UI | AgenticGoKit 自带 Mermaid 流程图生成,自己写成本高 |
| AgenticGoKit 支持 Anthropic 优先 + 我们的 Backend 接口 | 集成摩擦降低 |

**触发后**:
- Coordinator 退化成 AgenticGoKit 的一个 Workflow 定义文件
- StageExecutor 退化成 AgenticGoKit 的 Agent / Tool
- 我们的 `loops` 表替换成 AgenticGoKit 自己的 state 表
- 估计需要 2-3 周迁移工作

**触发前**:保持自定义 Coordinator,代码量小、依赖少、可控。

---

## 13. 待确认 (open questions)

### 12.6 实现细节:Coordinator 怎么消费 LangGraph-style 模式

Coordinator 关键代码骨架(伪 Go):

```go
// server/internal/loop/coordinator.go
package loop

type Coordinator struct {
    db        *sql.DB
    events    *events.Bus
    taskSvc   *service.TaskService
    backend   agent.Backend  // 现有 Backend 接口
}

func (c *Coordinator) HandleTaskCompleted(ctx context.Context, evt events.TaskCompleted) error {
    task, err := c.taskSvc.GetTask(ctx, evt.TaskID)
    if err != nil { return err }

    loop, err := c.getLoopByTaskID(ctx, task.LoopID)
    if err != nil { return err }
    if loop.Status != "running" { return nil }  // 用户暂停/取消后不再推进

    next, err := c.decideNextStage(loop, task)  // 状态机转移
    if err != nil { return c.failLoop(ctx, loop, err) }

    switch next.action {
    case "create_task":
        return c.taskSvc.CreateTask(ctx, next.taskSpec)
    case "complete":
        return c.completeLoop(ctx, loop, next.prURL)
    case "fail":
        return c.failLoop(ctx, loop, next.reason)
    }
    return nil
}

func (c *Coordinator) decideNextStage(loop *Loop, lastTask *Task) (Decision, error) {
    switch loop.CurrentStage {
    case "plan":
        return Decision{action: "create_task", taskSpec: developTask(loop)}, nil
    case "develop":
        return Decision{action: "create_task", taskSpec: reviewTask(loop)}, nil
    case "review":
        review := parseReviewArtifact(lastTask.Artifacts)
        if review.Approved {
            return Decision{action: "complete", prURL: ...}, nil
        }
        if loop.Iteration >= loop.MaxIterations {
            return Decision{action: "fail", reason: "max_iterations_exceeded"}, nil
        }
        return Decision{action: "create_task", taskSpec: fixTask(loop, review.Issues)}, nil
    case "fix":
        return Decision{action: "create_task", taskSpec: reviewTask(loop)}, nil
    }
    return Decision{}, fmt.Errorf("unknown stage: %s", loop.CurrentStage)
}
```

**这个 Coordinator 是 v2 借鉴 LangGraph 状态机 + CrewAI sequential process 的产物**,但用 Go 直接写,集成进现有 `internal/events` 订阅和 `internal/service.TaskService`,没有任何外部框架依赖。

---

## 13. 待确认 (open questions)

实现 plan 阶段需要确认:

1. 现有 `task` 系统是否已经支持自定义 `task_type` + executor routing?还是要扩展?
2. 现有 `task` 系统是否已经发 `task:completed` 事件?如果没有,事件怎么发?
3. 现有 daemon 的 routing 机制是 dispatcher / registry / 还是 if-else 链?这影响 stage executor 的注册方式。
4. `internal/events` 事件订阅是同步回调还是消息队列?决定 Coordinator 是否能 hot-reload 状态。
5. dogfood 模式下,loop worker 跟 daemon 是不是跑在同一个进程?如果不是,需要新进程间协调。

这些确认后,implementation plan 阶段可以开始拆 task。
