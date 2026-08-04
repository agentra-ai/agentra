# Agentra 超越 Multica 的成熟开源项目计划

> 日期：2026-07-30
> 对比基线：Agentra `0d7ce5839adcd70f45b1514a80e74a723d11c953`；Multica `32ab1e77dc3a3268b95261cc72af64ca06684ffb`
> 目标：不是复制 Multica 的功能数量，而是把 Agentra 做成更可信、更易安装、更可扩展、具备更强智能编排闭环的开源 human-agent work platform。

## 1. 执行摘要

Agentra 与 Multica 已处于同一产品赛道，但当前成熟度差异很清楚：

- Agentra 的潜在技术上限更高：已有 Goal → DAG、Engineering Loop、Memory、MCP Server、Eval、Execution Trace、API LLM Provider、Cloud Gateway 等差异化模块。
- Multica 的产品完成度和工程收口更强：17 个 coding CLI 的能力矩阵、Squads、Autopilots、Chat、Projects、桌面端、移动端、文档站、Helm、安装器、跨平台 CI，以及大量围绕并发、权限、升级、恢复和边界条件的回归测试。
- Agentra 当前最大问题不是缺少想法，而是“存在源码”与“端到端可用”没有严格区分。部分已宣称完成的能力存在路由未挂载、前后端路径不一致、入口缺失或占位实现。
- 因此第一阶段必须先建立产品真实性和发布纪律；之后再吸收 Multica 的成熟能力；最后用 Agentra 的 DAG + Loop + Memory + Eval 形成真正难以复制的智能工程闭环。

战略顺序：

1. **先可信**：所有公开声明都由可运行路径和自动化验收支撑。
2. **再好用**：十分钟完成安装、登录、连接 runtime、执行第一项任务。
3. **再兼容**：用统一 conformance contract 扩展 coding agent，而不是堆脆弱适配器。
4. **再智能**：把 DAG、Loop、Teams、Autopilot、Memory 和 Eval 收敛成一条执行主线。
5. **再扩平台**：项目管理、VCS、消息渠道、桌面/移动、企业能力建立在稳定核心之上。

## 2. 证据基线

### 2.1 Agentra 当前事实

源码级检查确认：

- 908 个 tracked files，其中 `apps/web` 323 个、`server` 473 个。
- 55 个 up migrations，88 个 Go/TypeScript/E2E 测试文件。
- Web 已有 Issues、Board、Inbox、My Issues、Agents、Runtimes、Skills、Loops、Settings 等入口。
- Backend 已有 Issues、Comments、Reactions、Subscribers、Attachments、Agents、Runtimes、Cloud Runtime、Billing、Memory、Projects、Git hooks、Loops、Auto Decompose 等 handler 或 route。
- 独有能力源码包括：Task Graph、Goal auto-decomposition、Engineering Loop、Memory CRUD/BM25、MCP Server、Eval fixture contract、Execution Trace、Repo-DNA、API-provider facade。

同时确认的收口问题：

- Task Graph 前端调用 `/api/issues/:id/graph` 和 `/api/graph/nodes/:id`，主 Router 未挂载这些路径。
- Trace 前端调用 `/api/tasks/:id/trace` 等路径，主 Router 暴露的是另一组路径。
- 初始审计发现 Metrics handler 未挂载、Eval handler 未挂载且返回 canned score；前者已正式挂载，后者已删除。
- Projects 路由在主 Router 中重复声明，其中一组嵌套路径不正确；Web 只有少量 project components，没有完整页面入口。
- Gateway log callback 仍是 TODO，README 宣称的实时日志需要端到端证明。
- README、Roadmap、package version、tag 和实际可用能力之间存在版本与状态漂移。
- CI 没有运行仓库定义的完整 `make check`，缺少 E2E、race、跨平台 runtime 和 installer gate。
- 自托管默认仍要求手工修改固定 secret；只有简单 `/health`，缺少 readiness、迁移健康、升级验证和 Helm。
- 缺少 `SECURITY.md`、PR template、支持策略、治理说明和正式版本化能力矩阵。

这些问题必须作为 P0，而不是继续被新功能掩盖。

### 2.2 Multica 当前事实

对 Multica 当前公开 main 源码树和官方文档的检查确认：

- 3,980 个 tracked files，281 个 up migrations，约 1,019 个 Go/TypeScript 测试文件；规模本身不代表质量，但显示了明显更深的边缘条件覆盖。
- 独立的 Web、Electron Desktop、Expo Mobile、Fumadocs Docs 应用，以及共享 `core`、`ui`、`views` packages。
- 17 个 coding tools，并明确记录 session resume、MCP、skill injection、model discovery 等能力差异。
- Squads 是 first-class assignee；leader 负责路由，并有防循环、去重、归档转移和权限规则。
- Autopilots 支持 cron、manual、webhook；包含 durable delivery、idempotency、event filtering、payload limit、rate limit、恢复 worker 和 run history。
- Projects、Resources、Chat、Invitations、Onboarding、Labels、Custom Properties、Search、Pins、Usage、Quick Actions 等产品面已形成完整入口。
- 自托管包含随机 secret、loopback-only 暴露、自动迁移、`/readyz`、SMTP/Resend、Helm、Kubernetes probe 和升级说明。
- CI 有 path filtering、Turbo cache、Go race、Redis integration、Helm test、Windows process/runtime regression、macOS/Linux installer matrix，以及独立桌面/移动验证。
- Release 同时产出 CLI、Homebrew、多架构 backend/web images 和 desktop assets。

### 2.3 对比数字的使用原则

文件数、migration 数、star、commit 和测试文件数只作为生态与维护投入信号，不作为产品优劣结论。真正的成熟度由以下证据决定：

- 用户能否从干净环境完成完整任务；
- 失败、重试、取消、恢复、升级、降级和多租户隔离是否有明确语义；
- 每项公开能力是否有 API contract、权限规则、回归测试、文档和可观测性；
- 发布产物是否可重复、可验证、可安全升级。

## 3. 取长补短矩阵

| 领域 | Agentra 当前优势 | Multica 值得吸收 | Agentra 的超越方向 | 优先级 |
|---|---|---|---|---|
| 核心任务管理 | Polymorphic assignee、实时协作、丰富 issue timeline | Projects、labels、properties、saved views、search、pins 的完整产品闭环 | 把 issue/project/goal/DAG 统一成可追踪 work graph | P0/P1 |
| Agent runtime | CLI + API provider 两种 backend、Cloud Gateway | 17 个 CLI 的 capability matrix、resume、MCP、skill path、跨平台回归 | 建立 Runtime Adapter Conformance Kit，让第三方 provider 可插拔且可验证 | P0/P1 |
| 智能编排 | Goal → DAG、Loop、Memory、Eval 是明显领先资产 | Squads 的稳定路由、Autopilot 的事件入口 | Teams + DAG + Loop + Policy + Budget 的统一编排内核 | P1 |
| 执行可观测性 | Trace、runtime usage、live card 已有基础 | 完整 transcript、usage attribution、failure taxonomy、恢复语义 | 可回放 execution ledger + cost/budget + eval feedback loop | P0/P1 |
| 人机协作 | Issues/comments/mentions/reactions | Chat、channel bot、quick action、leader re-trigger | Chat 可一键升级为 issue/goal/DAG，所有执行保持审计链 | P1/P2 |
| 安装与自托管 | Compose、MinIO、local daemon、gateway | Homebrew/script/PowerShell、随机 secret、readyz、Helm、升级守卫 | 单命令安装 + signed artifacts + preflight/doctor + migration rehearsal | P0 |
| 多端体验 | Web 功能基础较强 | Desktop、Mobile、Docs app，共享 core/ui/views | 先完成 responsive PWA，再用薄壳复用核心；避免三套业务逻辑 | P2 |
| 集成生态 | GitHub、git hooks、MCP server 已有基础 | VCS abstraction、Slack/Lark、Composio、webhook worker | Provider-neutral VCS/channel/tool adapters + scoped token policy | P1/P2 |
| 安全与企业 | workspace scope、RBAC、PAT、SSO schema | 更细权限测试、fail-closed attribution、secret handling、自托管安全默认值 | tenant-isolation proof suite、audit ledger、OIDC/SAML/SCIM、policy engine | P0/P2 |
| 工程质量 | `make check` 包含 unit/Go/E2E | race、Windows/macOS、installer、Helm、migration upgrade、千级回归测试 | 风险分层 CI + conformance + soak + N-1 upgrade gate | P0 |
| 开源治理 | Apache-2.0、CONTRIBUTING、issue templates | PR template、文档站、Discord、完整 release pipeline | SECURITY、governance、support policy、RFC/ADR、roadmap truthfulness | P0/P1 |

## 4. 应借鉴、应改进、不应复制

### 4.1 直接借鉴的模式

1. **Provider capability matrix**
   - 每个 adapter 明确 resume、stream、tool event、MCP、skills、model discovery、cancel、platform support。
   - 文档从 contract test 结果生成，避免人工表格漂移。

2. **Squad/Team 作为稳定路由目标**
   - 用户分配给团队而不是硬编码某个 agent。
   - leader、member、archive、anti-loop、dedup、permissions 都必须有确定语义。

3. **Durable Autopilot ingress**
   - cron、webhook、manual 和 VCS event 使用同一 admission/run model。
   - idempotency、rate limit、payload cap、recovery worker 和 run history 是基础要求。

4. **成熟自托管默认值**
   - 自动生成 secrets，默认只监听 loopback，显式 TLS/reverse-proxy 指南。
   - liveness 与 readiness 分离，readiness 校验 DB 和 migration state。
   - Helm secret 引用，不把 secret 模板化进 values。

5. **风险导向 CI**
   - Go race、Windows process tree、installer matrix、Helm render、migration upgrade、desktop/mobile smoke 独立分层。
   - path filter 只用于昂贵任务，required status 始终有明确结果。

### 4.2 在借鉴基础上做得更好

1. **从 17 个手工 adapter 升级为 conformance ecosystem**
   - 核心仓库只内置优先 provider。
   - 第三方 adapter 通过 versioned protocol、capability negotiation、fixture kit 和 certification 加入。
   - UI 只展示 contract 证明可用的能力，不接受 silent ignore。

2. **从 Squad 路由升级为智能 Team runtime**
   - leader 可依据技能、容量、成本、历史成功率、repo access 和 policy 选择 worker。
   - 支持并行 DAG、stage barrier、reviewer、human gate 和自动回退。
   - 每次路由决策可解释、可审计、可评测。

3. **从定时任务升级为可治理 Automation**
   - 默认创建可见 issue；run-only 也必须进入 execution ledger，不产生“不可见工作”。
   - webhook v1 即支持 HMAC、secret rotation、dead-letter、retry policy、budget 和 notification。
   - 每个 automation 有版本、发布状态、dry run 和 rollback。

4. **从 transcript 升级为可回放证据链**
   - prompt/context/tool/artifact/approval/cost/result 形成 append-only execution ledger。
   - 敏感字段结构化 redaction；可导出、可复现、可作为 Eval 输入。

5. **从静态 Eval 升级为真实回归门**
   - golden issue 在隔离 repo/environment 中真正执行。
   - 评分包含 acceptance tests、diff quality、安全、token/cost、时延和人工 review。
   - provider、prompt、skill 或 runtime 变更必须通过基准差异门。

### 4.3 不应复制的做法

- 不把“支持 provider”定义为代码中存在一个文件；必须通过 capability contract。
- 不接受配置被 provider 静默忽略；unsupported capability 必须在保存或执行前失败。
- 不复制 bearer-only webhook 作为长期正式方案；正式版本必须有 HMAC 和 rotation。
- 不让 run-only 任务脱离统一审计、通知和成本归因。
- 不在 Web 核心未稳定前维护三套重复业务逻辑；Desktop/Mobile 应复用共享 domain/core。
- 不为尚未发布的旧路径增加 compatibility shim；直接收敛到一个清晰 contract。
- 不继续堆积“已设计/已写文件但未接通”的功能；任何 Done 都需要端到端验收。
- 不用 migration 数量证明成熟。v1 前应在可行时整理 baseline，之后执行严格升级策略。

## 5. 目标产品模型

Agentra 最终应围绕一个统一工作闭环设计：

```text
Goal / Issue / Event
        ↓
Plan（human 或 planner）
        ↓
Work Graph（DAG + dependencies + artifacts + gates）
        ↓
Team Router（skills + capacity + policy + budget + history）
        ↓
Runtime Adapter（local / cloud；Claude / Codex / others）
        ↓
Execution Ledger（trace + tool + approval + cost + artifact）
        ↓
Review / Verify / Merge
        ↓
Memory + Eval + Analytics feedback
```

关键原则：

- Issue 是人类可理解的责任单位。
- Task Run 是一次可重试、可恢复的执行尝试。
- Work Graph 表达复杂工作的依赖和并行关系。
- Team 是稳定分配对象，Agent 是可替换执行者。
- Runtime 是计算边界，Provider Adapter 是能力边界。
- Execution Ledger 是审计与学习的事实来源。
- Memory 必须带来源、作用域、TTL 和置信度，不能是不可解释的全局向量池。
- Eval 必须测真实工作结果，而不是只测模型回答。

## 6. 分阶段实施路线图

### M0 — Truthful Core：把“存在”变成“可用”

目标：建立可信基线，停止文档、路由、UI 和版本状态漂移。

工作包：

- `M0-01` 建立版本化 capability manifest：`experimental` / `beta` / `stable` / `planned`。
- `M0-02` 自动生成并校验 backend route inventory；检查所有 frontend API path 都有唯一 backend contract。
- `M0-03` 接通或删除 Task Graph 的缺失 route，补 workspace/RBAC、error handling 和 E2E。
- `M0-04` 统一 Trace API；删除旧路径，不保留双路径兼容层。
- `M0-05` 正确挂载 Metrics；Eval 在接入真实执行和评分前保持 experimental，不进入 stable Web/API surface。
- `M0-06` 收敛重复 Projects route，完成单一后端入口；在 UI 未完成前禁止宣称 Done。
- `M0-07` 接通 Gateway → Web 的日志流；定义 backpressure、ring buffer、reconnect 和 redaction。
- `M0-08` 注册遗漏 CLI command，或删除未完成命令；CLI help、docs 和实际 root commands 一致。
- `M0-09` 统一版本来源：tag、CLI、API、Web、README、Roadmap 从 release metadata 派生。
- `M0-10` 清理明显 dead code、placeholder score、silent fallback 和未生效配置。

验收门：

- 自动 contract test 证明 frontend 使用的每个 API path 存在且 method/schema 一致。
- 主 Router 没有重复 route；所有 handler 明确为 mounted 或 experimental-only。
- README 的每个 shipped feature 都有至少一条 E2E 或 integration evidence。
- `make check` 全绿，并加入 CI required checks。
- capability manifest 与 release notes 自动校验，无手工状态漂移。

当前实施进度（2026-08-01）：

- 已建立 Chi route inventory contract test；数据库不可用时纯路由测试仍会真实执行。
- 已接通 Task Graph 的读取、节点更新和节点删除 API，并增加 workspace membership 与实体归属双重校验。
- 已挂载 owner/admin Metrics API，handler 只取经过 middleware 授权的 workspace 上下文，query 参数不能绕过成员与角色校验。
- 已清理 Projects 重复挂载和 Agent Memory 的错误 API 前缀。
- 已确认 `cmd_eval.go` 会在自身 `init()` 中注册 CLI command；此前“CLI root 未注册”的判断不成立。
- 已完成 `M0-08` CLI command truth contract：`agentra` 无参数和 `--help` 统一由 Cobra 注册树生成，英文/中文顶层命令清单与 21 个真实命令做精确集合校验；同时移除了 `git hooks install/uninstall` 的重复注册。
- 已完成 `M0-09` 统一 release metadata contract：`release/metadata.json` 是版本、tag、CLI/API/Web build info、package manifests、README 与 Roadmap 的单一来源；tag workflow 会在发布前拒绝元数据不一致，直接开发构建明确标识为 `dev/unknown`。
- `M0-10` 已删除从未挂载却宣称存在的 Eval HTTP handler、placeholder score 与未使用 evaluator；CLI 只保留 25-case 静态 fixture contract 校验，并在机器输出中固定 `quality_gate=false`，真实 agent benchmark 留待可回放 execution ledger 完成后实现。
- `M0-10` 已删除完全没有生产调用的 Memory service/hooks/semantic+graph+temporal/RRF 平行架构及其专用 SQL；保留真实可达的 agent/team CRUD、Web viewer 与 workspace BM25 搜索，并把 embedding/RAG/provenance 明确列为未完成。
- `M0-10` 已删除无法构建且未接入认证的 MCP 嵌套模块：原入口从未调用 authenticator，工具上下文为空，issue 读写可接受任意 workspace/resource ID，且实际只注册 issue 工具却宣称六类工具。MCP 重新标记为 planned，正式实现由 `M6-05` 的 scoped PAT、resource authorization、tool audit 和 policy contract 驱动。
- `M0-10` 已删除零生产调用的 Task Graph 伪执行链：旧 executor 不调用任何 agent backend 却返回 `executed`，随后把节点写成 `completed`；dependency classifier 与 handoff/container/scheduler 同样未挂载。保留真实可达的 Goal→DAG、图持久化、workspace-scoped CRUD 与 Web 可视化，并把自动 dispatch、依赖调度、恢复和 handoff 明确列为 planned。
- `M0-10` 已删除零生产引用、无测试且自身依赖声明不完整的 `server/pkg/github` 嵌套模块；真实可达的 GitHub OAuth、workspace installation 与 issue git-link 路由继续保留在主 server 模块，PR/webhook durable sync 仍由 `M6-01`–`M6-03` 实现。
- `M0-10` 已删除非 production 环境固定接受 `888888` 的 OTP master code；本地 `console` 邮件模式继续返回每次随机生成的真实 OTP，development 环境也必须通过数据库中的限时验证码与尝试次数校验。
- `M0-10` 已删除 Engineering Loop 的错误 prompt 回退：review/fix 缺失真实 branch/iteration、stage template 构建失败或出现未知 `loop_*` task type 时，daemon 现在明确失败队列任务，不再猜测分支或降级成 standard prompt 后报告阶段成功。
- `M0-10` 已把 runtime config 注入改为执行前置条件：写入 `CLAUDE.md`/`AGENTS.md` 失败或 provider 不受支持时任务明确失败，不再在缺失 agent identity、workspace/repo context 与 Agentra CLI 约束的情况下继续执行。
- 已删除未接入产品且调用旧 API 的 Trace Web 死代码；后端 Trace API 保留为唯一现有 contract，正式 Web viewer 后续基于它重建。
- 已完成 `M0-07` Gateway → Web 日志流：Gateway 只接受 Authorization Header 的 JWT/PAT，并要求身份是目标 workspace 的 owner/admin，再把连接绑定到该单一 workspace；协议统一为 snake_case 强类型帧，以每任务单调 `seq`、stdout/stderr 和 32 KiB content 为契约。
- cloud task 的 dispatched/log/completed/failed 事件都会再次验证 task、cloud runtime 与 Gateway workspace 的一致性；跨租户 ID 不作为存在性 oracle。日志在服务端脱敏后写入带 `(task_id, seq)` 唯一约束的 `task_message`，重复帧成功幂等但不重复广播。
- 容器日志使用 Docker follow 流同步回压，不建立无界队列；terminal event 只携带 256 KiB 诊断尾部。Web 初始加载与重连均读取最多 5,000 条持久化快照，live timeline、dedup set 和 terminal 都保持同一内存上限。
- 数据库回归新增 trace lifecycle FK：删除 issue 会级联清理 execution trace，删除 agent 不再被历史 trace 阻塞；`make check` 默认使用进程级隔离的临时数据库并在退出时精确删除，失败运行不会污染下一次验证或开发数据库。
- Gateway/Web realtime 仍保持 beta：Hub fanout 目前是单进程内存实现，多副本 sticky-free fanout 留在 `M7-02`。
- 已接通 Memory 的 workspace 搜索、team/agent 新增与删除路径，并为 Settings 中的新增、搜索、删除交互补齐真实状态流。
- 已修正 Compose 与宿主机 `make setup/dev/check` 的数据库连接矛盾：PostgreSQL 仅绑定 loopback，不对局域网或公网开放。
- 已恢复 Next.js 16 的 ESLint flat config，将 lint 接入本地完整检查与 CI，并清理生产 API client 的显式 `any` 类型债务。
- 已完成 `/projects` 正式 Web 入口：项目创建/编辑/删除、里程碑状态、issue 分配/移除和只读成员视图均使用当前 workspace store；后端拒绝通过伪造 workspace URL 访问其他 workspace 的 project ID。

### M1 — Installable & Operable：十分钟跑通第一项任务

目标：从“开发者能搭起来”升级为“普通开源用户能安全运行”。

工作包：

- `M1-01` 实现 `agentra setup`：Cloud 与 self-host 两种 profile；preflight、login、workspace、daemon 一次完成。
- `M1-02` 发布 Homebrew、install.sh、PowerShell installer；安装器用 stub fixture 做 macOS/Linux/Windows 回归。
- `M1-03` self-host bootstrap 自动生成 JWT/Postgres/MinIO secrets，默认不暴露 DB/Adminer/MinIO console。
- `M1-04` 增加 `/livez`、`/readyz`；readiness 校验 DB、migration、storage 和 scheduler critical dependency。
- `M1-05` backend 启动自动执行安全迁移或明确 init container；提供 N-1 → N upgrade rehearsal。
- `M1-06` 支持 SMTP + Resend，增加 signup allowlist、disable signup、disable workspace creation。
- `M1-07` 新增 `agentra doctor`：网络、auth、runtime CLI、repo permission、storage、WebSocket 诊断。
- `M1-08` 建立版本化 docs app，覆盖 cloud/self-host、CLI、providers、permissions、troubleshooting、upgrade。
- `M1-09` 发布多架构 GHCR images、checksums、SBOM、provenance 和签名。

验收门：

- 新 macOS、Ubuntu、Windows 环境在 10 分钟内完成 first agent task。
- self-host 默认配置没有已知 default secret 或公网数据库暴露。
- CI 从前一稳定 tag 的真实数据库执行 upgrade，并完成关键读写 smoke。
- `agentra doctor` 能对最常见安装失败给出可操作结论。

当前实施进度（2026-07-31）：

- 已实现 `agentra setup` 的 cloud/self-host 引导：先校验 Web 与 `/readyz`、远程端点强制 HTTPS、检查 Claude/Codex/OpenCode CLI，再复用现有认证、workspace watch 与 daemon 启动完成一站式配置；重复运行会复用有效凭证与匹配的 daemon。
- self-host 默认使用安全 Compose 的 loopback 端口；在公开托管服务发布前，cloud profile 要求显式端点，不写入虚假的生产默认地址。端点切换会清除属于旧服务器的 token 与 workspace 选择。
- 已建立 macOS/Linux `install.sh`、Windows `install.ps1` 与 Homebrew cask 发布契约；安装器检测平台、校验 `checksums.txt`、暂存替换并执行版本 smoke，CI 在 Ubuntu、macOS、Windows 使用本地 release fixture 回归。
- GoReleaser 已纳入 Windows amd64/arm64 zip、两个安装器和官方 tap cask；CLI daemon 的后台进程、停止信号和日志读取已移除 Unix-only 假设，Windows amd64 交叉编译通过。
- 签名、SBOM、provenance 仍属于 M1-09；Homebrew cask/Windows 公共资产必须等下一次 tag 真正发布，因此 M1-02 保持 beta 而非宣称已上线。
- 已新增 `/livez` 与 `/readyz`；readiness 验证 PostgreSQL、嵌入二进制的最新 migration、已配置对象存储 bucket 和 loop scheduler。
- `/health` 继续作为 CLI 的轻量进程检查；Compose 和新 worktree 环境使用 `/readyz` 作为流量就绪门。
- readiness 尚未覆盖 realtime fanout、邮件和 gateway capacity，继续保持 beta。
- 已实现 `agentra doctor`：并发、分项超时地检查配置权限、Web/API、readiness/storage、认证、workspace、Runtime CLI、工作目录、Git origin、本地 daemon 与 WebSocket ping/pong；支持版本化 JSON 和失败退出码。
- CLI WebSocket 诊断通过 `Authorization` Header 使用 PAT，服务端与 REST 复用同一 JWT/PAT 认证逻辑，避免长期 token 出现在 URL/代理日志。
- doctor 的对象存储检查仍依赖 `/readyz`，Git 权限仅验证 fetch/read 而不执行 push，因此继续标记 beta。
- 已实现安全 self-host bootstrap：一次生成互相独立的 PostgreSQL/JWT/MinIO 随机凭据、以 `0600` 原子写入 `.env`、拒绝覆盖和拒绝已知默认值，并为该契约加入自动化回归。
- 默认 Compose 不再发布 PostgreSQL/MinIO，不再启动 Adminer 或挂载 Docker socket 的 gateway；前后端只绑定 loopback，Adminer/gateway 分别通过 `debug` / `cloud-runtime` profile 显式启用。
- 已删除未进入 workspace、依赖不完整且宣称不存在 SQLite 路径的旧 `scripts/create-agentra` 脚手架；跨平台安装统一留给后续 `agentra setup` 与 installer 工作包，不保留虚假入口。
- 已将容器发布从个人 Docker Hub 凭据收敛到官方 `ghcr.io/agentra-ai/agentra` package；server、gateway、web 每个 tag 均声明 `linux/amd64` 与 `linux/arm64` manifest，并附带 BuildKit SBOM/max provenance、Cosign keyless digest 签名和 GitHub registry attestation。
- CLI release 现在为 6 个归档生成 SPDX 2.3 SBOM，`checksums.txt` 同时覆盖归档、SBOM 与两份安装器；checksum 与每份 SBOM 都由 GitHub OIDC 身份生成 Sigstore bundle，所有 checksummed subject 再写入 GitHub provenance attestation。
- 真实 GoReleaser + Syft snapshot 已证明 6 个归档和 6 个 SBOM 可生成；正式 OIDC 签名、GHCR push 与 attestation 必须等下一次 tag 在 GitHub Actions 中执行，因此供应链能力保持 beta。macOS notarization/code signing 与 Windows Authenticode 仍是后续平台信任工作。
- 已实现 `M1-06` 的服务端策略闭环：OTP 邮件支持 Resend API 与 SMTP（STARTTLS、隐式 TLS、无认证内网 relay），生产环境没有真实 provider 时失败关闭，不再把开发验证码返回浏览器。
- 新用户注册可按精确邮箱或 `@domain` allowlist 限制，也可完全关闭；已被邀请/预置的用户仍能登录。关闭 workspace 创建后，新用户必须已被邀请或匹配 workspace 声明域名，显式创建 API 同样返回 403，避免生成无 workspace 的孤儿账号。
- 邮件与访问策略目前是进程级环境配置，SMTP 尚无 provider-container CI，readiness 也未探测出站邮件，因此保持 beta。

### M2 — Runtime Conformance：少而精地扩 provider

目标：先把 3 个核心 backend 做到稳定，再以协议化方式扩展。

执行拆分：M2 是长期路线中的一个阶段，不作为单次、无限范围的执行目标。当前只验收
`M2A Core Conformance`（`M2-01` 至 `M2-05`）；`M2-06`/`M2-08` 进入后续
`M2B Isolation & SDK`，`M2-07` 进入 `M2C Provider Expansion`。当前门槛、证据和
授权边界见 [M2 Runtime Conformance 检查点](./2026-08-04-m2-runtime-conformance-checkpoint.md)。

工作包：

- `M2-01` 定义 Runtime Adapter v1：discover、models、execute、stream、resume、cancel、skills、MCP、usage、artifacts。
- `M2-02` 建立 conformance fixture kit：stdout/stderr、partial JSON、hung child、cancel、resume miss、token usage、secret redaction。
- `M2-03` Claude、Codex、OpenCode 全量通过 Linux/macOS/Windows contract。
- `M2-04` 每个 adapter 声明 capability；server/UI 在 dispatch 前验证，禁止 silent fallback。
- `M2-05` 支持 process-tree cancellation、timeout、bounded cleanup 和 crash resume。
- `M2-06` 建立 per-task home/worktree/credential overlay，防止不同任务泄漏 memory、skills、session 或 token。
- `M2-07` 第二批优先接入 Cursor、Copilot、Qwen、Kimi、Trae/CodeBuddy；按用户需求和 conformance 完成度发布。
- `M2-08` 开放外部 adapter SDK/manifest，建立 certified/community/experimental 等级。

验收门：

- adapter matrix 由测试结果生成。
- 所有 stable adapter 支持 cancel、bounded cleanup、resume 语义和明确 MCP/skills 行为。
- 同一 conformance suite 可在本地和 CI 运行；Windows 有真实 shell/process regression。
- provider 不支持某能力时，配置保存或 dispatch 返回明确错误。

当前实施进度（2026-08-04）：

- 已落地 `M2-01` 的 Runtime Adapter v1 类型契约和首个自动化 conformance matrix：Claude、Codex、OpenCode 必须完整声明 14 项能力、保持稳定 provider 顺序，并由测试锁定 native/adapter/unsupported 等级。
- 三个 backend 的 descriptor 与中心 registry 由回归测试逐一比对；model listing 等未支持能力必须返回结构化 `UnsupportedCapabilityError`，descriptor 返回值也已验证无法反向修改 registry。
- 执行参数在子进程查找和启动前统一校验；Codex/OpenCode 对 `max_turns` 和 `tool_restrictions` 的拒绝路径已有 pre-launch 回归，负 timeout、负 max turns 和空工具名返回结构化参数错误。
- 已启动 `M2-02`：加入由测试期间原生编译的 hermetic fake CLI，同一个 fixture 覆盖 Claude JSONL、Codex JSON-RPC 和 OpenCode JSONL，并故意将每帧拆成多次 pipe write，验证 partial JSON framing。
- 三个 adapter 已通过真实子进程 success、stderr、非零退出、bounded timeout、主动 cancel 和 resume miss 回归；Codex 成功结果现在保留可恢复的 thread ID，adapter stderr 在进入结构化日志前执行 secret redaction。
- hermetic runtime contract 已加入独立的 Ubuntu、macOS、Windows CI matrix，Linux 额外启用 race detector；真实 hosted runner 结果仍须在提交推送后确认。
- Linux/macOS runtime 现在运行在独立 process group，Windows 使用系统 tree termination；三种 adapter 均通过 heartbeat 孙进程回归证明 cancel 后不再继续执行，等待上限固定为 2 秒。Windows hosted runner 结果仍须在推送后确认。
- 已启动 `M2-04` 的服务端闭环：daemon 只允许注册已声明的本地 provider；Agent 保存时从 runtime 派生 provider 并拒绝冲突值；Engineering Loop 创建前验证 stage 所需的 max-turn/tool 能力；daemon ping 与 task launch 都按 descriptor 校验选项。
- 已完成 crash-resume 的本地闭环：三个 adapter 在 provider session 建立后、执行结束前发出 checkpoint；server 只允许 workspace 成员为运行中 task 持久化 session/workdir；同一稳定 daemon identity 的新进程 instance 重新注册时在 retry budget 内自动重排队，同一 instance 的重复注册保持幂等，并在重新 claim 同一 task 时优先恢复该 checkpoint。adapter、client、数据库恢复和跨租户拒绝均有回归。
- 已完成 token usage 与 artifact wire contract：Claude assistant usage、Codex `thread/tokenUsage/updated`（并兼容 legacy token_count）和 OpenCode step-finish tokens 都进入统一 Result；daemon 将 duration/usage/artifacts 传至 task result、trace 与 metrics。负 usage、无定位符或错误 digest 的 artifact 失败关闭；三个 CLI 尚无可靠 provider-declared artifact，因此 capability 继续明确标为 unsupported，不扫描 worktree 猜测。
- 真实 provider binary、通用 pre-claim 过滤和 UI capability 展示尚未完成，因此 `M2-03` 至 `M2-04` 仍处于进行中，不能将三个 adapter 提升为 stable。

### M3 — Agentra Intelligence：形成真正差异化的编排闭环

目标：让 Goal → Plan → Parallel Work → Review → Learning 成为 Agentra 的核心产品，而不是孤立 demo。

工作包：

- `M3-01` 统一 Issue、Task Graph、Loop 和 Task Run domain model；只保留一条 lifecycle state machine。
- `M3-02` Planner 生成带 acceptance criteria、artifact contract、risk 和 human gate 的 DAG。
- `M3-03` 将 Engineering Loop 变为 graph stage template：Plan → Develop → Review → Fix → Verify。
- `M3-04` 实现 first-class Teams：leader、members、roles、archive、visibility、capacity、routing policy。
- `M3-05` Team Router 根据 skills、repo access、capacity、cost、success rate、latency 和 policy 选择 worker。
- `M3-06` durable scheduler 支持并行节点、stage barrier、retry、dead-letter、cancel propagation 和 resume。
- `M3-07` Human Gate：高风险 tool、生产变更、超预算、merge/deploy 前暂停审批。
- `M3-08` Memory 增加 provenance、scope、TTL、confidence、PII policy；每次检索在 trace 中可见。
- `M3-09` Eval 执行真实 golden repos/issues；对 planner、provider、skill、memory 策略做差异回归。
- `M3-10` 形成 outcome analytics：成功率、返工率、review rejection、cycle time、cost per accepted change。

验收门：

- 一个 goal 可自动生成 DAG，并行执行，汇总 artifact，通过 human review，产生可追踪结果。
- coordinator 重启不会重复执行已完成节点或丢失待执行节点。
- 所有路由与 planner 决策都能解释“为何选择此 agent”。
- Eval 分数来自真实测试与 diff，不使用 placeholder/headless canned result 作为 stable gate。

### M4 — Automation & Human Collaboration

目标：吸收 Autopilot、Chat 和渠道协作，但保持所有工作可见、可治理。

工作包：

- `M4-01` Automation model：cron、manual、webhook、VCS、API trigger 统一 admission。
- `M4-02` webhook 首版即支持 HMAC、rotation、idempotency、event filters、payload limit、rate limit。
- `M4-03` durable delivery worker、retry policy、dead-letter、alert、replay 和 per-trigger budget。
- `M4-04` 默认 create-issue；direct run 也写入 execution ledger、inbox 和 usage，不允许隐形工作。
- `M4-05` Automation version/draft/publish/dry-run/rollback。
- `M4-06` Private Agent Chat；可选择 workspace/project/repo context，默认不自动继承敏感上下文。
- `M4-07` Chat 一键 promote 为 issue、goal 或 DAG，保留来源链和选择性上下文。
- `M4-08` quick actions 和 structured blocker/approval cards。

验收门：

- 三副本 scheduler 下同一 cron/webhook 只产生一次 authoritative run。
- webhook 重放、服务重启和 worker crash 不丢请求、不重复 side effect。
- 用户可以从 automation/chat 一路追溯到 issue、task run、artifact 和 cost。

### M5 — Product Completeness：达到成熟任务管理基线

目标：不只比 agent runtime，而是让 2–10 人团队愿意每天用它管理真实工作。

工作包：

- `M5-01` 完成 Projects Web 闭环：列表、详情、lead、status、priority、dates、progress、resources、milestones。
- `M5-02` Labels + typed custom properties + filter/group/sort/save view。
- `M5-03` 全局 search，支持 issue/project/agent/skill/comment/artifact。
- `M5-04` invitations、member lifecycle、role matrix、ownership transfer。
- `M5-05` notification preferences、digest、archive/read semantics、delivery channel。
- `M5-06` issue relationships：parent/child、blocks/blocked-by、duplicate、related。
- `M5-07` keyboard navigation、command palette、bulk action、undo for reversible actions。
- `M5-08` accessibility WCAG 2.2 AA、responsive layouts、overflow/long-content/slow-network states。
- `M5-09` i18n contributor pipeline；英文和中文是 release gate，其余语言按社区维护。

验收门：

- 核心 project/issue/agent workflow 在 desktop-size 和 mobile web E2E 全覆盖。
- workspace role matrix 每个敏感 mutation 都有 allow/deny integration test。
- 100k issues/workspace 的标准筛选和搜索 p95 达到定义的性能预算。

### M6 — Integrations & Open Platform

目标：让 Agentra 成为 agent work system of record，而不是封闭应用。

工作包：

- `M6-01` provider-neutral VCS domain：GitHub 首发，随后 GitLab/Gitea。
- `M6-02` repo/resource、PR、commit、branch、check、review、merge 的统一 link model。
- `M6-03` GitHub App webhook durable worker；签名验证、delivery dedup、replay 和 rate limit。
- `M6-04` Slack/Lark/Feishu channel adapter：identity binding、workspace membership、audit、live result card。
- `M6-05` 完成 MCP Server 的 remote deployment、scoped PAT、tool audit 和 permission policy。
- `M6-06` public REST/OpenAPI + webhook/outbox；generated SDK 至少覆盖 TypeScript/Go。
- `M6-07` Skill/Template registry：版本、依赖、签名、来源、审查、workspace pin。
- `M6-08` tool integration adapter 可接 Composio 或原生 MCP，但 authorization 由 Agentra policy 控制。

验收门：

- 所有 inbound event 都可验证来源、幂等处理并追踪到 delivery/run。
- token 具备 workspace/resource/action scope，可吊销且有 last-used audit。
- 一个外部开发者能按文档实现 adapter/skill 并通过 conformance。

### M7 — Deployment, Scale & Clients

目标：在不破坏核心一致性的前提下扩展运行形态和客户端。

工作包：

- `M7-01` 官方 Helm chart：external Postgres/object storage/Redis、Ingress、Secret refs、probes、resources。
- `M7-02` HA realtime：Redis/NATS adapter、sticky-free WS fanout、presence TTL。
- `M7-03` backup/restore、object lifecycle、data export/delete、retention policy。
- `M7-04` observability：OpenTelemetry traces/metrics/logs、Prometheus rules、dashboard 和 SLO。
- `M7-05` PWA/offline read/notification 先行，验证移动需求。
- `M7-06` Desktop 采用薄壳并复用 shared core；必须支持 daemon lifecycle、auto-update、self-host profile。
- `M7-07` 原生 Mobile 仅在 PWA 需求被证明后进入；复用 schema/query/realtime packages。

验收门：

- 单实例与三副本部署通过相同 conformance 和故障恢复测试。
- 提供经过演练的 backup/restore 和 N-1 upgrade runbook。
- Desktop/Mobile 不复制 domain rules；共享 contract drift test 必须通过。

### M8 — Security, Enterprise & Open-source Governance

目标：同时建立企业可信度和健康开源社区。

工作包：

- `M8-01` `SECURITY.md`、漏洞披露 SLA、supported versions、security advisory 流程。
- `M8-02` threat model：tenant isolation、daemon trust、repo credentials、prompt injection、artifact supply chain。
- `M8-03` tenant-isolation proof suite，覆盖 IDOR、header spoof、WS subscription、storage URL、background worker。
- `M8-04` append-only audit log、retention/export、admin investigation UI。
- `M8-05` OIDC/SAML、SCIM、service account、policy-as-code、BYOK secrets。
- `M8-06` `CODE_OF_CONDUCT.md`、`GOVERNANCE.md`、`SUPPORT.md`、PR template、maintainer guide。
- `M8-07` RFC/ADR 流程、public roadmap、good-first-issue、triage SLA、release cadence。
- `M8-08` demo repo、sample agents/teams/automations、benchmark results 和公开 architecture docs。

验收门：

- 安全敏感 release 有 threat-model delta、tenant tests、SBOM 和 signed artifacts。
- 新贡献者仅凭文档能在 30 分钟内启动开发环境并提交通过 CI 的改动。
- Roadmap 的 Done 状态只来自 capability manifest + release evidence。

## 7. 前 90 天建议切片

### 第 1–2 周：真实性与契约

- 完成 M0 全部工作。
- 把 broken/dead routes、版本漂移和 docs overclaim 清零。
- 在 CI 中运行完整核心检查。

### 第 3–4 周：安装、自托管与安全默认值

- 完成 `agentra setup`、installers、doctor、readyz、随机 secrets。
- 新增 SECURITY、support policy、PR template。
- 建立多架构发布与 migration upgrade smoke。

### 第 5–8 周：Runtime Adapter v1

- Claude/Codex/OpenCode conformance。
- Windows/macOS/Linux cancellation、resume、skills、MCP、usage 回归。
- 选择 2–3 个第二批 provider，不追求一次性凑数量。

### 第 9–12 周：智能编排 Alpha

- 统一 DAG + Loop lifecycle。
- Teams Alpha + capacity/skill routing。
- Automation Alpha：cron/manual/signed webhook + durable delivery。
- 真实 Eval golden repo gate。

90 天结束的外部可见成果应是：

1. 安装和自托管明显优于当前版本；
2. 核心三 provider 的可靠性和透明度达到或超过 Multica；
3. Goal → DAG → Team → Loop → Review 是可演示、可恢复、可评测的真实闭环；
4. README 中不存在无法被测试证明的“Done”。

## 8. 统一质量门

每个 stable capability 必须同时满足：

### 功能

- Domain contract、API schema、权限矩阵、UI empty/loading/error/offline 状态完整。
- 至少一条用户旅程 E2E 和一组后端 integration tests。
- CLI/API/Web 对相同动作具有一致语义。

### 可靠性

- create/claim/start/complete/fail/cancel/retry/resume 全状态测试。
- 幂等 key、并发竞争、重复 delivery、进程 crash、server restart 有回归覆盖。
- 长任务支持 heartbeat、stale detection 和 safe recovery。

### 安全

- workspace scope 和角色 allow/deny 测试。
- secrets 不进入 log、trace、prompt、artifact 或 client bundle。
- external ingress 具备认证、限流、大小限制和审计。

### 性能

- API、query、WS fanout、scheduler、large transcript 定义并执行基准。
- 为 100k issues、10k task runs、large artifact/timeline 建立性能 fixture。
- 性能回归进入 release gate，而不是只依赖人工体验。

### 发布

- `make check`、race、migration upgrade、installer、platform runtime、image smoke 全绿。
- artifacts 有 checksum、SBOM、provenance/signature。
- changelog、capability manifest、docs 和 version 一致。

### 文档

- 用户文档说明 happy path、权限、失败/恢复、限制和 self-host 差异。
- “不支持”必须明确，不允许 silent fallback 或模糊宣传。

## 9. 架构决策清单

以下决策应在实现前形成 ADR：

1. Issue/Task/Graph/Loop 的统一 lifecycle 和 authoritative state owner。
2. Runtime Adapter v1 transport、versioning 和 third-party loading model。
3. Team 是否扩展 `assignee_type`，以及 leader/member 的权限和归因模型。
4. Automation delivery/outbox/scheduler 的一致性模型。
5. Execution Ledger 的存储、redaction、retention 和 replay 语义。
6. Memory provenance/scope/TTL 与 prompt-injection 防护。
7. API schema 来源：OpenAPI-first、code-first 或 shared schema generation。
8. Realtime HA adapter 与 delivery guarantee。
9. Desktop 技术选型和 shared core 边界。
10. v1 前 migration baseline 与 v1 后兼容/升级政策。

## 10. 执行纪律

- 每个里程碑先写 vertical-slice issue：用户结果、domain change、API、UI、tests、docs、migration、rollout。
- 不以“代码文件已创建”关闭 issue；必须通过该切片的验收门。
- 每个阶段先完成一个 tracer bullet，再并行扩展。
- 不引入与目标无关的 broad refactor。
- 新 provider、新 client、新 integration 必须复用已定义 contract，不复制 domain logic。
- 所有兼容性代码都需要明确的真实用户/数据迁移理由；产品未上线的旧路径直接删除。
- Roadmap 每次 release 后从 capability evidence 更新，不手工维持多套状态。

## 11. 完成定义

“超越 Multica”不能靠一次版本宣称完成。达到以下状态，才可以认为 Agentra 成为更成熟的开源选择：

- **核心可信度**：公开 stable 能力 100% 有自动化端到端证据。
- **首次体验**：跨平台十分钟 first task，自托管安全默认值可直接使用。
- **runtime 质量**：核心 provider 的 resume/cancel/MCP/skills/usage/cross-platform 行为透明且通过 conformance。
- **智能差异化**：Goal → DAG → Team → Loop → Review → Memory/Eval 是统一、可靠、可恢复的产品主线。
- **平台完整性**：成熟 project/issue 管理、VCS/channel/API/MCP、usage/audit、安全和运维能力。
- **开源可持续性**：清晰治理、贡献路径、版本政策、安全响应、文档与可验证 release pipeline。

在此之前，目标保持为持续实施状态；每次迭代都应优先提高“可证明的真实能力”，而不是功能计数。

## 12. 外部一手资料

- Multica repository and README: <https://github.com/multica-ai/multica>
- Provider capability matrix: <https://raw.githubusercontent.com/multica-ai/multica/main/apps/docs/content/docs/providers.mdx>
- Squads: <https://raw.githubusercontent.com/multica-ai/multica/main/apps/docs/content/docs/squads.mdx>
- Autopilots: <https://raw.githubusercontent.com/multica-ai/multica/main/apps/docs/content/docs/autopilots.mdx>
- Projects: <https://raw.githubusercontent.com/multica-ai/multica/main/apps/docs/content/docs/projects.mdx>
- Desktop and Mobile: <https://raw.githubusercontent.com/multica-ai/multica/main/apps/docs/content/docs/desktop-app.mdx>, <https://raw.githubusercontent.com/multica-ai/multica/main/apps/docs/content/docs/mobile-app.mdx>
- Self-host quickstart and advanced operations: <https://raw.githubusercontent.com/multica-ai/multica/main/apps/docs/content/docs/self-host-quickstart.mdx>, <https://raw.githubusercontent.com/multica-ai/multica/main/SELF_HOSTING_ADVANCED.md>
- CI and release workflows: <https://raw.githubusercontent.com/multica-ai/multica/main/.github/workflows/ci.yml>, <https://raw.githubusercontent.com/multica-ai/multica/main/.github/workflows/release.yml>
