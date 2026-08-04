# M2 Runtime Conformance 检查点

> 日期：2026-08-04  
> 状态：本地门槛已完成；等待外部证据  
> 范围：`M2A Core Conformance`（长期计划中的 `M2-01` 至 `M2-05`）

## 为什么需要这个检查点

“超越 Multica”是长期产品方向，不是可以用一次 `complete`/`blocked` 判定的有限任务。
如果把 M0–M8、外部发布和所有 provider 扩展放在同一个执行目标里，任何尚未授权的外部
动作都会让已能继续的本地工作看起来受阻。

从现在起采用两层状态：

- 北极星：`M0–M8` 路线图，只用于排序和取舍，不直接宣告完成。
- 执行检查点：有固定范围、验收门和证据。本检查点只关闭 M2A；M2B、M2C 在 M2A
  关闭后分别建立检查点。

## 当前结论

Docker、站点和本地验证链已经恢复，因此它们不是当前阻力。旧的 `blocked` 状态是历史
终态记录，不代表北极星目标不可推进。M2A 的本地实现已经完成；当前未解除的门槛只剩
把提交送到托管环境后才能取得的三平台证据，以及需要真实 provider binary/凭据的 smoke。

## 阻塞账本

| 门槛 | 当前状态 | 所需证据 | 解除动作 | 是否需要用户授权 |
|---|---|---|---|---|
| Docker/本地站点 | 已解除 | Compose 服务运行；`https://web.agentra.orb.local/` 返回 200 | 无 | 否 |
| 仓库完整本地检查 | 已解除 | `make check` 退出码 0 | 每批变更继续回归 | 否 |
| M2A 本地实现 | 已解除 | pre-claim 在 dispatch 前拒绝不兼容任务，并继续 claim 兼容任务 | 无 | 否 |
| Linux/macOS/Windows 托管矩阵 | 外部门槛 | GitHub Actions 三平台全部通过 | 推送提交并观察 CI | **是** |
| 真实 Claude/Codex/OpenCode smoke | 外部门槛 | 版本、命令、成功/取消/恢复结果留档 | 在具备 binary 与凭据的隔离环境运行 | 需要环境/凭据授权 |

“外部门槛”不等于整个北极星目标受阻。M2A 现在等待外部证据，但 M3 等不依赖该证据的
本地工作仍可继续；不要把一个检查点的等待状态重新扩大成 M0–M8 的全局 `blocked`。

## M2A 验收门

以下条件全部满足后，才可以关闭 M2A：

- Runtime Adapter v1 对 Claude、Codex、OpenCode 的能力声明和结构化错误由同一套
  conformance suite 锁定。
- fixture 覆盖 partial frame、stderr redaction、non-zero exit、timeout、cancel、
  process-tree cleanup、resume miss、crash resume、token usage 和 artifacts。
- server 在保存配置和 dispatch 前验证 provider 与 capability；Web 能展示能力和拒绝原因，
  不做 silent fallback。
- 相同 conformance suite 在 Linux、macOS、Windows 的托管 runner 上通过；Windows
  使用真实 shell/process 行为而非仅交叉编译。
- 三个真实 provider binary 至少各完成一次成功执行；声明支持 resume 的 provider 还须完成
  恢复验证。版本与结果写入证据记录。
- 不支持的 MCP、skills、usage 或 artifact 行为保持显式 `unsupported`，不得用空结果伪装
  成已支持。

## 执行顺序

1. 已完成通用 pre-claim capability 过滤：队列使用与 daemon 相同的 stage policy；不兼容任务在 dispatch 前原子失败，同一次 claim 继续选择兼容任务。
2. 已跑 `make check`，保持本地门槛为绿色。
3. 经用户授权后推送当前提交，取得三平台托管 CI 证据并修复平台差异。
4. 在隔离环境运行真实 provider smoke，记录版本、能力和偏差。
5. 证据齐全后关闭 M2A；随后分别开启 M2B（隔离与 SDK）和 M2C（provider 扩展）。

## 状态判定规则

- `in progress`：仍有无需新增授权即可推进的实现或验证。
- `blocked`：连续复核后，剩余路径全部依赖缺失授权、凭据或外部状态。
- `complete`：本节全部验收门均已有可复查证据；本地绿色不能替代托管或真实 provider 证据。

当前判定：M2A **本地完成、外部证据待办**；北极星目标仍是 **in progress**。下一项外部
动作是经用户授权推送并观察托管 CI；如果暂不推送，则继续不依赖该证据的 M3 本地检查点。
