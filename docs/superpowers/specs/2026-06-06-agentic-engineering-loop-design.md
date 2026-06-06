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

| 场景 | 行为 |
|------|------|
| Plan task 失败 | task retry 3 次;失败后 loop `failed` |
| Develop task 失败 | task retry;失败后 loop `failed` |
| Review task 失败 | task retry;失败后 loop `failed` |
| Fix task 失败 | task retry;重试 3 次后 loop `failed` |
| LLM 网络错误 | 现有 task retry 机制 |
| 用户暂停 | LoopCoordinator 标记 `paused`;**不再创建新 task**;已有 task 跑完后不创建下一 task |
| 用户恢复 | LoopCoordinator 恢复创建下一 task |
| 用户取消 | LoopCoordinator 取消所有进行中的 task,标记 `cancelled` |
| iteration >= max_iterations | loop `failed`,记录 `failure_reason='max_iterations_exceeded'` |
| 创建 PR 失败 | loop `failed`,记录 `failure_reason='pr_create_failed'` |
| Coordinator 进程崩溃 | 重启时从 `loops` 表加载 `running/paused` 状态的 loop,继续 |

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

| 维度 | LangGraph | CrewAI | 结论 |
|------|-----------|--------|------|
| 语言 | Python only | Python only | ❌ 都不支持 Go |
| 官方 Go port | 无 | 无 | ❌ 都没出 |
| 跟 Backend 接口兼容 | 绑定 LangChain 生态,需要适配 | 绑定自家 agent 抽象,需要适配 | ❌ 都要写 adapter 层 |
| 部署复杂度 | 加 Python 微服务 | 加 Python 微服务 | ❌ agentra 是 Go-native,混语言破坏一致性 |
| 图编排能力 | 强(复杂条件、并行、子图) | 弱(主要 sequential) | ⚠️ 我们用不到 LangGraph 的强项,CrewAI 能力相当 |
| 团队学习成本 | 中(图概念) | 低(更直观) | ⚠️ 不算决定性 |
| 维护风险 | LangChain API 经常 breaking | CrewAI 还在快速迭代 | ⚠️ 都不稳定 |

**核心判断**:我们 4 个 stage 的状态机很简单,LangGraph 的图编排能力用不上(用上反而是 over-engineering);CrewAI 的 sequential 模式我们直接实现 ~300 行就够。引入 Python 服务带来的语言混用、部署复杂度、维护风险,远超收益。

### 12.5 Go 生态编排方案对比

调研了 Go 生态的多 agent / workflow 编排方案:

| 选项 | 定位 | 优势 | 劣势 | v2 选不选 |
|------|------|------|------|-----------|
| **Temporal** | 生产级 workflow 引擎 | retries / timeouts / signals / versioning 全部现成;多语言 SDK | 加 Temporal server 依赖(独立进程);学习曲线陡;对 4 stage 杀鸡用牛刀 | ❌ MVP 不上,长期可考虑 |
| **Inngest** | Function-as-a-Service 风格 workflow | Go SDK 体验好;事件驱动;managed 服务 | 第三方 vendor;定价不透明 | ❌ MVP 不上 |
| **Cadence (Uber)** | 类似 Temporal | Go-native 早;成熟 | 社区比 Temporal 小;部署也重 | ❌ MVP 不上 |
| **Restate** | 比 Temporal 轻量 | 单 binary 部署;Go SDK 简洁 | 生态新;生产案例少 | ⚠️ 观望 |
| **Hatchet** | 分布式任务队列 + workflow | Go SDK 好;开源 | 主打任务队列,不是为 LLM 编排设计 | ❌ 场景不匹配 |
| **自定义 Coordinator + 事件总线** | 跟现有架构一致 | 零新依赖;~300 行;DB 持久化;跟现有 WS 事件打通 | 自己写 retry/timeout 边界(其实已经由 `tasks` 表 retry 提供) | ✅ **v2 选这个** |

**v2 选自定义的理由**:
1. **零新基础设施依赖**。Temporal / Inngest / Cadence 都需要独立 server,跟 agentra 的"一 docker compose 跑起来"理念冲突。
2. **跟现有事件流打通**。Coordinator 订阅 `task:completed` 事件就行,不用引入新消息总线。
3. **代码量真的不大**。状态机 4-5 个转移,~300 行;比 adapter 层(把 LangGraph 翻译成 Go)还少。
4. **后续可演进**。如果以后真要图编排,优先评估 Temporal(Go SDK 成熟,跟我们的 stack 最契合);自定义 Coordinator 不会成为障碍。

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
