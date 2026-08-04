# M3A Durable Run-Aware Lifecycle Spine

> 日期：2026-08-05
> 状态：M3A-01a 本地验收通过；M3A-01b outbox 待实施
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

- Start、Checkpoint、Complete、Fail、Cancel、Retry 已通过一个事务型 Lifecycle
  Interface 写入；runtime recovery、timeout 和 bulk cancel 目前使用等价的原子 SQL，
  后续收口到同一个 durable-event Seam；
- 每个成功 transition 同事务追加 versioned outbox event；
- realtime、metrics、Trace 和 notifications 改为幂等投影；
- 注入状态提交后、事件投递前的崩溃，证明重启后事件不会丢失。

### M3A-01c — Idempotent Engineering Loop Consumer

- Loop 按 outbox event ID/version 幂等消费；
- stage Work Item 创建与 Loop transition 原子化或使用唯一幂等键；
- 注入完成后崩溃、消费中崩溃和重复投递，证明不会重复 stage；
- 删除 Loop 对易丢 goroutine 事件和“缺 in-flight task 就重排”的依赖。

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
- M3A-01：本地可推进，不受 M2A 外部证据阻塞；
- 北极星 M0–M8：持续进行，不再使用单一 `blocked` 终态表达所有检查点。
