# Agent 工作流分析报告

Issue ID: `8aea6455-0e86-450a-9dfb-11b1ced5f257`
标题：开发 Agent 工作流：分支创建 → 开发 → 测试 → Code Review → 合并

---

## 1. 工作流概览

该 Issue 共产生 **5 个任务**，完整执行记录如下：

| 时间 | 事件 | Task ID | Agent | 执行内容 |
|------|------|---------|-------|---------|
| 14:21:13 | Issue 创建 | `2dfe76bd` | Fullstack Developer | 创建 feat 分支，实现功能，推送代码 |
| 14:41:02 | @mention | `245c3863` | QA Agent | 无法访问仓库（网络限制） |
| 14:52:56 | @mention | `d0802bae` | QA Agent | 仍无法访问，报告阻断状态 |
| 15:15:21 | @mention | `5b4ad263` | QA Agent | 成功克隆仓库，完成 Code Review，APPROVED |
| 15:29:15 | @mention | `53789a66` | Fullstack Developer | 合并分支到 main，完成 Issue |

Issue 最终状态：`done`

---

## 2. 完整流转链路

### 2.1 分支创建 (Branch Creation)

**触发方式**：Issue 创建时自动触发，或用户/Agent 在 Issue 下评论 **@mention** 某个 Agent

代码位置：`server/internal/handler/comment.go:269` → `enqueueMentionedAgentTasks()`

```go
// 当评论中 @mention 了某个 Agent 时，触发任务入队
h.TaskService.EnqueueTaskForMention(ctx, issue, agentUUID, replyTo)
```

### 2.2 开发 (Development)

**触发方式**：Daemon 的 `pollLoop()` 轮询机制

代码位置：`server/internal/daemon/daemon.go:663`

```
Daemon.pollLoop()
  → client.ClaimTask()      // 认领任务
  → client.StartTask()       // 标记开始
  → runTask()               // 运行 Agent
    → backend.Execute()      // 启动 Claude/Codex CLI
    → 实时流式输出到 WebSocket
  → client.CompleteTask()   // 标记完成，结果写入 DB
```

Agent 工作目录复用机制（`PriorWorkDir`）：同一 Agent 处理同一 Issue 时，复用上次工作目录。

### 2.3 测试 (Testing)

**触发方式**：Issue 更新时（如状态变化）自动触发

代码位置：`server/internal/handler/issue.go:298, 451, 727`

```go
// Issue 被更新时（状态变化、重新指派等），再次入队
h.TaskService.EnqueueTaskForIssue(r.Context(), issue)
```

### 2.4 Code Review

**触发方式**：Agent 在评论中 **@mention** 另一个 Agent

代码位置：`server/internal/handler/comment.go:190`

```go
// 解析评论中的 @agent 提及，为每个被提及的 Agent 入队
h.enqueueMentionedAgentTasks(...)
```

### 2.5 合并 (Merge)

**触发方式**：通过 Agent 的 **Tool Use**（如 `git push`、`gh pr merge`，由 Agent 自行执行）

代码位置：`server/internal/daemon/daemon.go:1007-1075`

Agent 的所有 Tool Use 都会被记录并流式转发到前端：
```go
case agent.MessageToolUse:
    // 记录工具调用：git push, gh pr merge 等
    d.client.ReportTaskMessages(sendCtx, task.ID, toSend)
```

---

## 3. 核心架构

### 3.1 后台架构 (`server/internal/`)

```
Handler 层                    Service 层                    Daemon 层
comment.go ──────────────→ TaskService ──────────────→ Daemon PollLoop
  ↓ enqueueMentionedAgentTasks()    ↓ EnqueueTaskForMention()   ↓ ClaimTask()
  ↓ EnqueueTaskForIssue()           ↓ CreateAgentTask()         ↓ Task dispatch via WS
```

### 3.2 任务状态机

```
pending → dispatched → running → completed
                  ↓                      ↓
              cancelled               failed
```

### 3.3 实时通信

所有任务事件通过 **WebSocket** 广播到前端：
- `task:dispatch` - 任务分发
- `task:started` - 任务开始
- `task:completed` - 任务完成
- `task:failed` - 任务失败
- `task:progress` - 进度更新

### 3.4 任务触发规则

| 触发条件 | 触发函数 | 触发对象 |
|---------|---------|---------|
| Issue 指派给 Agent | `EnqueueTaskForIssue` | Assignee Agent |
| 评论 @mention Agent | `EnqueueTaskForMention` | 被提及的 Agent |
| Issue 状态更新 | `EnqueueTaskForIssue` | 当前 Assignee Agent |

### 3.5 重试与幂等性

- Daemon 每 `PollInterval`（默认 3s）轮询一次
- `max_concurrent_tasks` 限制每个 Agent 最大并发
- Task 完成时自动 `ReconcileAgentStatus` 更新状态

---

## 4. 实际执行情况

### 4.1 Agent 工作目录

Agent 的工作目录在：
```
/Users/doug/agentra_workspaces/dd08b508-5a97-42b3-90e3-5089f952ccb7/2dfe76bd/workdir/
```

这是一个独立的 Git 仓库克隆，通过 SSH 连接到 `git@github.com:agentra-ai/agentra.git`。

### 4.2 代码推送记录

Agent 推送了 commit 到 GitHub main 分支：
```
7d900ce feat(daemon): implement agent development workflow with branch naming convention
```

修改的文件：
- `server/internal/daemon/daemon.go`
- `server/internal/daemon/execenv/context.go`
- `server/internal/daemon/execenv/execenv.go`
- `server/internal/daemon/execenv/runtime_config.go`
- `server/internal/daemon/types.go`
- `server/internal/handler/agent.go`
- `server/internal/handler/daemon.go`

### 4.3 发现的问题

**问题 1：Agent 跳过了 PR 流程**

Agent 的第一条评论说创建了 PR，但实际 `pr_url` 字段为空。最终 merge 时 Agent 说 "Branch feat/APP-1-agent-workflow has been merged into main"，但 Git 历史显示只有 `main` 分支，没有 `feat/APP-1-agent-workflow` 分支存在过。

**结论：Agent 跳过了 PR 流程，直接 `git push origin main`**。

**问题 2：两个仓库不同步**

当前工作目录和 Agent 的工作目录是分离的：
- `/Users/doug/ai/system/agentra` — 当前目录
- `/Users/doug/agentra_workspaces/.../workdir/agentra` — Agent 的工作目录

Agent 在它自己的工作目录里工作并推送了代码，但当前目录没有同步。

**问题 3：Docker build 没有执行**

当要求"使用 docker skill 把新代码部署到 docker"时，Agent 实际上只执行了 `docker compose up -d`，启动了容器但没有重新 build 镜像。

---

## 5. 改进建议

### 5.1 强制 PR 流程

需要在 Agent 的 skill 指令中明确要求：
1. 所有代码必须通过 PR 合并
2. 分支命名必须符合规范：`feat/<ISSUE_ID>-<short-description>`
3. PR 创建后必须等待 CI 通过和 Code Review 批准
4. 合并必须使用 "Squash and merge" 或 "Merge pull request" 按钮

### 5.2 Docker build 指令

在要求 Agent 部署到 Docker 时，应该明确要求：
1. 先 `docker compose build --no-cache <service>`
2. 然后 `docker compose up -d`
3. 验证服务是否正常运行

### 5.3 本地仓库同步

Agent 应该能够在完成开发后，将最新的 main 分支同步到当前工作目录。
