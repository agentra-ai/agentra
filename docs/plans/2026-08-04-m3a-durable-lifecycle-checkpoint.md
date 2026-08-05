# M3A Durable Run-Aware Lifecycle Spine

> 日期：2026-08-05
> 状态：M3A-01a/01b 本地通过；M3A-01c 终态 consumer 分片通过
> 范围：`M3-01` 的执行事实统一，不提前实现 Work Graph scheduler

## 问题定义

当前没有唯一的 lifecycle 写入 seam：

- `agent_task_queue`、`task_runs` 和 `execution_traces` 分别保存执行状态；
- `StartTask` 先把 Work Item 写成 running，再 best-effort 创建两种 Run/Trace 记录；
- retry 复用同一 Work Item，但 daemon 的消息序号从 1 重新开始，数据库却按
  `(task_id, seq)` 去重，后一次 Run 的消息可能被静默丢弃；
- Engineering Loop 依赖进程内事件和 goroutine 推进，完成写入与 stage 推进之间崩溃时，
  启动恢复可能重复执行 stage；
- Loop 创建下一 Work Item、更新 stage、保存 PR 和完成 Loop 都是分离写入。

因此，M3 的第一步不能是合并状态枚举或给 Task Graph 增加更多按钮。必须先让执行事实拥有
明确 identity 和唯一写入路径。

## 选择的 Module 与 Seam

建立 `Durable Run-Aware Lifecycle Spine` Module：

- Work Item 表示逻辑工作和 retry policy；
- Run 表示一次执行 attempt，并以 `run_id` 作为 daemon callback 的 identity；
- Trace、message、usage 和 artifact 是 Run 的投影；
- Lifecycle Transition 使用 expected-state CAS，并与 durable event 在同一事务提交；
- Engineering Loop 作为幂等 consumer 推进，最终迁移为 Work Graph stage-template adapter。

删除测试：迁移完成后，应能删除一套重复 Run/Trace 状态、按“最新 trace”猜 attempt 的查询、
Loop 的易丢进程内推进以及“缺 task 就重跑”的恢复猜测。否则新 Module 仍然过浅。

## 分片计划

### M3A-01a — Run Identity & Attempt Isolation

- Work Item 在 claim/dispatch 时原子创建 Run 并写入 `active_run_id`，然后向
  daemon 或 Cloud Gateway 返回同一个 `run_id`；
- message、session checkpoint、complete 和 fail callback 必须携带 `run_id`；
- server 只接受当前 active Run 的 callback，旧 Run callback 明确冲突；
- message 唯一键迁移为 `(run_id, seq)`；
- execution Trace 绑定 `run_id`，停止按 task 的“最新记录”猜 attempt；
- retry 回归证明两个 Run 都能保留从 1 开始的消息序列。

实施状态（2026-08-05）：

- 已完成：claim/dispatch 原子分配 Run；`agent_task_queue.active_run_id` 成为当前
  attempt 指针；Start、Checkpoint、Complete、Fail、Retry、Cancel 进入事务型
  `RunLifecycle` Module；Work Item 与 Run 在同一事务锁定并校验 expected state；
- 已完成：local daemon 与 Cloud Gateway Adapter 的 dispatch、start、log、message、
  session、progress/stage、complete、fail、status 全链路携带并校验 `run_id`；
- 已完成：completion usage/artifact 输出与 execution Trace 精确写入指定 Run，删除
  “按 task 找最新 Run/Trace”的终态猜测；前端 dispatch、message、stage、terminal
  事件按 Run 隔离，旧 Run 的延迟事件不能清空新 Run 的 live card；
- 已覆盖：旧库重复 running Run、无 Run 的 dispatched Work Item、Run session 回填、
  active Run 唯一约束；同一 Work Item 两次 Run 各自保存 `seq=1`；Run A 的全部
  HTTP callback 返回 409 且不会改变运行中的 Run B 或写入 comment/metric 投影；
  message insert 与 terminal transition 并发时由同一 Work Item lock 串行化，终态后
  不会留下消息；
- 已验证：隔离 schema migration 历史修复、7 条核心 HTTP/Cloud Gateway 回归、
  UI Run-event 单元测试以及最终完整 `make check` 均通过。开发库没有应用未提交
  迁移；验证使用独立端口和自动销毁的临时数据库，完成后开发站点返回 HTTP 200。

M3A-01a 不再受 Docker 或网络阻塞。剩余的 durable event、幂等投影和
Engineering Loop 消费属于 M3A-01b/01c，不能混写成 M3A-01a 未完成。

### M3A-01b — Transactional Lifecycle & Durable Outbox

- 已完成：Start、Checkpoint、Complete、Fail、Cancel、Retry 通过同一个事务型
  `RunLifecycle` Interface 写入，并在同一事务追加 versioned lifecycle event；
- 已完成：runtime recovery、offline/stale sweeper、issue/agent bulk cancel 使用单条
  data-modifying CTE 原子锁定批次、关闭 active Run、更新 Work Item 并逐项追加 outbox；
  recovery event 精确绑定被中断的 Run，已由 offline transition 记录的 exhausted Work Item
  不伪造第二次 Run failure；
- 已完成：pre-claim provider capability rejection 写入无 Run 的 `work_item.rejected` fact，
  core projector 可靠生成失败 comment/realtime，Engineering Loop 以
  `invalid_configuration` 原子终止对应 stage；
- 已完成：outbox worker 按 Work Item 保序 claim，使用 lease 与 fencing token 防止旧 worker
  ack 新 lease，失败采用指数退避，并在达到上限后进入 dead letter；
- 已完成：Trace、comment 和 metric 由 core consumer 以 event ID 或 Run ID 幂等投影；
  独立 `task-derived` consumer 等 core ack 后，在自己的事务中生成终态 activity、失败
  inbox、完成评论的 subscriber/mention inbox、评论者 subscription 和 receipt；失败进入
  独立退避/dead-letter，不再被同步 Bus listener 吞掉；
- 已完成：带 durable event ID 的终态 listener 已退出数据库写入，旧 listener 仅兼容
  无持久 ID 的历史事件；所有 realtime signal 均在对应数据库事务 commit 后发布，明确是
  可重复、非权威提示，客户端可从 durable row 重新读取；
- 已覆盖：投影提交后、outbox ack 前模拟崩溃并重放，同一 comment 和 metric 只写一次，
  realtime 可以重复；worker 随后仍能确认原事件。

authoritative terminal/retry/cancel/reject Transition 及其本地数据库投影已全部收口到
durable lifecycle Seam。核心事实、Engineering Loop、task-derived 三个 consumer 各自拥有
receipt、retry 与 dead-letter，避免一个慢投影阻塞其他 consumer。Claim/dispatch 仍以即时
realtime delivery 为主的问题已在 M3A-01d 收口。

### M3A-01c — Idempotent Engineering Loop Consumer

- 已完成终态分片：Loop 使用独立 receipt/delivery 按 outbox event ID/version 幂等消费；
- 已完成：consumer 锁定 Loop 并精确读取事件中的 `run_id`，stage Work Item 创建与
  Loop transition 在一个数据库事务提交；`StartLoop` 的首个 Work Item 与 started 状态
  也改为原子提交；
- 已覆盖：两个 worker 并发 claim 只推进一次，重复 durable fact 只生成 receipt，启动恢复
  发现待消费终态事件时不会重排当前 stage；无 Run 的 pre-claim rejection 也只失败一次；
- 已删除：Loop 对终态 Bus/goroutine 的订阅、按“最新 Run”猜结果的查询，以及“缺
  in-flight task 就重排”的终态恢复路径。

进度和 stage live signal 仍有意保持 ephemeral；它们不是决定 Loop 状态的事实。
完整 Work Graph stage-template adapter 不属于本终态 consumer 分片。

### M3A-01d — Durable Cloud Dispatch Delivery

- 已完成：local daemon pull claim 明确排除 `runtime_type=cloud`，Cloud Work Item 不再依赖
  本地 daemon 顺带领取，消除同一 Work Item 同时由本地进程和 Gateway 执行的路径；
- 已完成：独立 Cloud Dispatch worker 仅在 workspace 有已连接 Gateway 时原子创建 Run、
  将 Work Item 置为 dispatched，并在同一数据库语句创建 per-Run delivery；Agent 与
  Cloud Runtime 两级并发上限均在 claim SQL 内校验；
- 已完成：delivery 使用 lease/fencing、退避和 20 次上限；WebSocket 写入后等待 Gateway
  确认容器已启动，超时以相同 `run_id` 重发；耗尽时 Run、Work Item、lifecycle fact 和
  dead-letter 在一个事务关闭；取消、重试等旁路留下的未确认 delivery 会显式归档；
- 已完成：Gateway 将 `run_id` 作为 dispatch idempotency key；同一进程收到重复帧时不会
  创建第二个容器，容器已经启动则只重发 `task:dispatched` ack；server 把 ack、Run start、
  Work Item running 和 `run.started` fact 放在同一事务；
- 已覆盖：Gateway 缺席时 Work Item 保持 queued；连接后独立 claim；ack 丢失后的重发只
  保留一个 Run；重复 ack 幂等；Cloud Runtime capacity 阻止超额 claim；发送耗尽进入
  terminal failure/dead-letter；local claim 无法取得 cloud Work Item。

边界：Gateway 进程重启后尚不能按 Docker label 接管已创建容器，terminal frame 也没有
Gateway 本地 durable spool。当前保持 `cloud-runtime=experimental`，不把 server 侧 delivery
闭环表述成完整 crash-resume 或生产隔离证据。

## 暂不进入本检查点

- Work Graph dependency scheduler、parallel barrier 和 handoff；
- 将 Engineering Loop 渲染成 Work Graph stage template；
- Issue workflow自动完成策略；
- provider 扩展和 M2A 外部证据。

这些工作依赖可靠 Run identity 和 lifecycle event，提前实现会把现有竞态扩散到更多调用者。

## M3A-01a 验收门

- claim/dispatch 与 Run 创建不可出现一半成功；
- 每个 local daemon 与 Cloud Gateway callback 都被绑定到 active `run_id`；
- stale callback 返回明确冲突且不改变 Work Item；
- 同一 Work Item 连续两个 Run 可各自保存 `seq=1..N`；
- Run A 的 session、usage、artifact 和 Trace 不进入 Run B；
- process crash/retry 关闭旧 Run，新 claim 创建新 Run；新 Run 可显式继承旧 Run
  保存的 provider session checkpoint，但不会复用旧 Run identity；
- 数据迁移、API、daemon client、跨租户和完整 `make check` 均有回归。

## 状态判定

- M2A：本地完成，等待托管 CI 与真实 provider smoke；
- M3A-01：主 callback 的 durable lifecycle spine 与 Loop 终态 consumer 本地通过；
  recovery/sweeper/bulk cancel/reject、task-derived 数据库投影和 Cloud dispatch
  delivery/ack/retry 已收口；下一执行可靠性分片是 Gateway restart container adoption 与
  terminal frame durable spool；
- 北极星 M0–M8：持续进行，不再使用单一 `blocked` 终态表达所有检查点。

最终本地验证（2026-08-05）：完整 `make check` 通过，覆盖截至 052 的隔离迁移、
Go/TypeScript 单元与集成测试及 E2E；临时数据库自动销毁，未对开发库应用迁移。
运行中的 `https://web.agentra.orb.local/` 随后返回 HTTP 200。
