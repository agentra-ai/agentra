# M3A Durable Run-Aware Lifecycle Spine

> 日期：2026-08-04  
> 状态：设计收口；M3A-01a 第一条纵向切片已实现  
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

- Work Item 从 dispatched → running 时原子创建 Run，并向 daemon 返回 `run_id`；
- message、session checkpoint、complete 和 fail callback 必须携带 `run_id`；
- server 只接受当前 active Run 的 callback，旧 Run callback 明确冲突；
- message 唯一键迁移为 `(run_id, seq)`；
- execution Trace 绑定 `run_id`，停止按 task 的“最新记录”猜 attempt；
- retry 回归证明两个 Run 都能保留从 1 开始的消息序列。

实施状态（2026-08-04）：

- 已完成：dispatched → running 与 Run 创建原子化，Start API/daemon 传递
  `run_id`；task message 和 execution Trace 绑定 Run；消息去重改为
  `(run_id, seq)`；stale Run 消息返回冲突；前端快照和实时流按 Run 隔离；
- 已覆盖：旧库多 Trace、少 Run 的迁移回填；同一 Work Item 两次 Run 都保存
  `seq=1` 且读取只显示 active/latest Run；
- 待完成：session、progress/stage、complete、fail 和 cloud terminal callback
  携带并校验 `run_id`，usage/artifact/finalize Trace 停止按“最新 task 记录”猜 Run。

因此本次落地是 M3A-01a 的可运行第一条纵向切片，不将 M3A-01a 整体标记完成。

### M3A-01b — Transactional Lifecycle & Durable Outbox

- Start、Complete、Fail、Cancel、Retry 通过一个事务型 Lifecycle Interface 写入；
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

- Start transition 与 Run 创建不可出现一半成功；
- 每个 daemon callback 都被绑定到 active `run_id`；
- stale callback 返回明确冲突且不改变 Work Item；
- 同一 Work Item 连续两个 Run 可各自保存 `seq=1..N`；
- Run A 的 session、usage、artifact 和 Trace 不进入 Run B；
- crash-resume 继续同一 Run，普通 retry 创建新 Run；
- 数据迁移、API、daemon client、跨租户和完整 `make check` 均有回归。

## 状态判定

- M2A：本地完成，等待托管 CI 与真实 provider smoke；
- M3A-01：本地可推进，不受 M2A 外部证据阻塞；
- 北极星 M0–M8：持续进行，不再使用单一 `blocked` 终态表达所有检查点。
