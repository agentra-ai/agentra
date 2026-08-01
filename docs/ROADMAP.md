# Agentra Product Roadmap 2025–2026

**The platform where AI agents work alongside humans as true teammates — not just tools.**

<!-- RELEASE_METADATA_START -->
- Current Release: [v0.6.0](https://github.com/agentra-ai/agentra/releases/tag/v0.6.0) (2026-07-31, stable)
<!-- RELEASE_METADATA_END -->
- Last Updated: 31 Jul 2026
- License: Apache 2.0 Open Source

> [!IMPORTANT]
> The machine-checked [capability manifest](capabilities.json) is the source of truth for shipped support levels. Historical `Done` labels below mean an implementation milestone landed; they do not automatically mean the feature is a stable, end-to-end product contract. New capability claims must use `stable`, `beta`, `experimental`, or `planned` and include evidence in the manifest.

## Council Verdict (Jul 2026): Phase 3 Begins

Per council-of-high-intelligence deliberation (Karpathy 1.5× + Machiavelli + Watts):

**Route chosen: B (Billing-first)** — Seats → Billing → Projects sequence.
- Window to capture enterprise AI-spend budget before Linear/Jira bolt on agent-native UX: 6-12 months
- Free ≤5 seats · $6/seat base · $10/seat with agent runtime
- Projects hierarchy deferred until billing ships (retention play, not revenue unlock)

## At a Glance

| Metric | Value |
|--------|-------|
| Current Version | Derived from the release metadata above |
| Agent Integrations | Claude, Codex, OpenCode |
| LLM Providers | 23+ |
| MCP Server | Beta |
| Target Team Size | 2–10 |
| Roadmap Horizon | 18 months (Q2 2025 → Q3 2026) |

---

## Competitive Analysis Summary (Updated 2026-05-10 v3)

**Latest landscape** (40+ projects scanned across 6 categories):

| Category | Top Project | Stars | Key Insight |
|----------|-----------|-------|-------------|
| Agent Frameworks | MetaGPT, AutoGen, CrewAI | 51k–67k | Python libraries dominate mindshare; none have task UIs |
| SWE Agents | OpenHands, ChatDev, AutoGPT | 33k–184k | Code execution, not task management |
| Claude Code Ecosystem | wshobson/agents, oh-my-claudecode | 33k–35k | Plugin explosion; Teams-first UX emerging |
| **Goal → DAG** | **open-multi-agent** | **6k** | **Auto-decomposes goal into task DAG — #1 gap to close** |
| Lightweight Orchestration | openai/swarm, agency-swarm | 4k–21k | Lightweight handoff; no persistence |
| Go Orchestration | myclaude, Shannon | 1.8k–2.6k | Go-based; closest in language choice |
| Enterprise Memory | hindsight, kiwiq | 1k–12k | SOTA memory benchmarks; multi-tier storage |
| MCP Tasks | agent-tasks, EDDI, mateclaw | 338–435 | File-based + config-driven; git hooks |
| Spatial/Visual | voicetree | 826 | Obsidian-like graph for agent orchestration |
| Task Management UI | Tasuku, Overseer, crewboard-oss | 11–223 | Lightweight but no agent runtime |

**Phase 2.5 completed gaps**: ✅ Multi-Provider (7), ✅ Memory RRF (4-strategy), ✅ Task Graph Handoff, ✅ Execution Traces, ✅ MCP Server

**Current Priority Gaps (v3)**:
| Gap | Competitor | Priority | Timeframe |
|-----|------------|----------|-----------|
| Goal → DAG Auto-Decomposition | open-multi-agent | 🔴 P0 | 30 days |
| Verifiable cross-platform install (`install.sh`, PowerShell, Homebrew, `agentra setup`) | agent-tasks, open-multi-agent | 🟡 P0 | Beta; public assets on next tag |
| Git-Native Hooks | agent-tasks | 🟡 P1 | 30-60 days |
| Provider Breadth (7→15+) | swarmclaw | 🟡 P1 | 30-60 days |
| Plugin/Skill Ecosystem Scale | wshobson/agents | 🟢 P2 | Phase 4 |
| Enterprise Compliance (EU AI Act) | EDDI | 🟢 P2 | Phase 4 |

**Agentra's enduring moat**: Real-time WebSocket + persistent task UI + multi-workspace + cloud runtime + human-in-the-loop — none of the 40+ scanned projects combine all five.

**New strategic insight**: Agentra is the only platform at the intersection of "task management UI" and "agent runtime". Frameworks (CrewAI, AutoGen) lack UIs; task managers (Tasuku, agent-tasks) lack agent runtimes. But the window is closing — open-multi-agent is one persistence layer away from becoming a direct competitor.

See [competitive-analysis-v3](archive/specs/2026-05-10-competitive-analysis-v3.md) for full 40+ project, 5-dimension comparison.

---

## Executive Summary

Agentra is an open-source, AI-native task management platform built for the next generation of software teams — where AI agents are first-class team members, not bolt-on automation. Think Linear, but your issues can be autonomously claimed, worked, and completed by agents that report blockers, push code, and update statuses in real time.

The current Agentra release ships a working end-to-end loop: create an issue, assign it to an agent, watch the agent execute via Claude Code or Codex, and see live WebSocket status updates. The foundation is solid. This roadmap charts the path from promising developer tool to the definitive platform for human-agent collaboration.

---

## Strategic Pillars

| Feature | Description | Priority |
|----------|-------------|----------|
| Agent-Native UX | Every workflow is designed for agents as first-class assignees. Agent status, logs, and blockers live alongside human tasks. | P0 |
| Open & Self-Hostable | Apache 2.0, one-command Docker setup. Privacy-first teams run it on their infra. | P0 |
| Deep Agent SDK | Unified Backend interface supporting any capable AI — Claude, Codex, and beyond. | P0 |
| Real-Time Collaboration | WebSocket-first — humans and agents see the same live view. | P1 |
| Skill Ecosystem | Reusable, shareable workflows that turn best practices into team capital. | P1 |

---

## Current State

### What's Shipped

| Feature | Description | Priority | Status |
|---------|-------------|----------|--------|
| Issue Management | Full CRUD for issues with status (Open / In Progress / Done), priority, and assignments to humans or agents. | P0 | Done |
| Agent Assignment | Issues can be assigned to AI agents; daemon runtime auto-discovers Claude Code and Codex on PATH. | P0 | Done |
| Task Lifecycle | queued → claimed → started → completed / failed with real-time WebSocket broadcasts. | P0 | Done |
| Local Daemon Runtime | Go daemon that polls for tasks and executes agents locally; authenticates via pairing token. | P0 | Done |
| Skills System | Reusable workflow templates that can be applied to issues as task context. | P1 | Done |
| Comments & Mentions | Threaded comments with @mention parsing and notification routing. | P1 | Done |
| Real-time Sync | WebSocket hub broadcasts all entity changes to connected clients instantly. | P0 | Done |
| File Attachments | Upload and manage files on issues and comments (MinIO / S3-compatible). | P2 | Done |
| Inbox / Notifications | Notification center for mentions, status changes, and agent updates. | P1 | Done |
| Google OAuth + JWT | Auth via Google OAuth or email/password; JWT sessions; personal access tokens. | P1 | Done |
| Docker Compose Setup | One-command local dev: postgres, minio, server, web — all containerized. | P0 | Done |
| Multi-Agent SDK | Unified Backend interface for Claude Code, Codex, OpenCode with hot-swap capability. | P0 | Done |

### Known Gaps

| Feature | Description | Priority | Status |
|---------|-------------|----------|--------|
| Agent Memory (RAG) | pgvector-backed with 4-strategy RRF retrieval | P0 | ✅ Done |
| Agent-to-Agent Handoff | Task Graph with DAG execution + handoff protocol | P0 | ✅ Done |
| Multi-Provider Support | 7 providers (3 CLI + 4 API) via Backend Facade | P0 | ✅ Done |
| Execution Traces | execution_traces table + TaskService integration | P1 | ✅ Done |
| Goal → DAG Auto-decomposition | LLM auto-decompose issue into task graph (open-multi-agent has this) | P0 | ✅ Done ([spec](archive/specs/2026-05-10-goal-to-dag-auto-decomposition-design.md)) |
| One-Command Install | Homebrew Cask, checksum-verifying shell/PowerShell installers, and idempotent `agentra setup` | P0 | 🧪 Beta; signed assets on next tag |
| GitHub Integration | GitHub OAuth App + webhooks + PR/commit linking | P1 | 🚧 In Progress |
| Git-Native Hooks | Auto-link commits via prepare-commit-msg + post-merge hooks (agent-tasks has this) | P1 | ✅ Done ([spec](archive/specs/2026-05-10-git-native-hooks-design.md)) |
| Provider Breadth (7→15) | Add DeepSeek, Groq, Together, Mistral, xAI, Gemini API, more | P1 | ✅ Done |
| Real-Time DAG Visualization | Interactive ReactFlow DAG view on issue detail page | P2 | ✅ Done |
| Pre-Defined Specialist Agents | 6 templates (Frontend, Backend, Test, Security, DevOps, Tech Writer) | P2 | ✅ Done |
| Onboarding Wizard | Guided in-app flow from workspace to first agent task | P1 | Not Started |
| Cloud Runtime | Managed containerized agent execution | P0 | 🚧 In Progress |
| Mobile App | React Native iOS/Android for monitoring and approvals | P2 | Not Started |
| Enterprise Compliance | EU AI Act, SOC2, GDPR readiness | P2 | Not Started |

> **Competitive Analysis v3 (2026-05-10)**: 40+ projects scanned across 6 categories. `open-multi-agent` (6k★) has Goal→DAG auto-decomposition. `agent-tasks` has one-command install + git hooks. `wshobson/agents` (35k★) + `oh-my-claudecode` (33k★) show Claude Code ecosystem explosion. `CrewAI`/`AutoGen`/`MetaGPT` (51k–67k★) dominate Python framework mindshare but lack task UIs. See [competitive-analysis-v3](archive/specs/2026-05-10-competitive-analysis-v3.md).

---

## Roadmap Overview

| Timeline | Phase | Theme | Focus |
|----------|-------|-------|-------|
| Q2–Q3 2025 | Phase 1 | Solid Foundation | DX, stability, onboarding, cloud runtime |
| Q4 2025 | Phase 2 | Agent Intelligence | Agent memory, MCP, multi-agent, insights |
| Q1–Q2 2026 | Phase 3 | Team Scale | Projects, sprints, billing, mobile |
| Q3 2026 | Phase 4 | Platform Ecosystem | Marketplace, API, enterprise SSO |

---

## Phase 1 — Q2–Q3 2025 (6 months)

**Solid Foundation — IN PROGRESS**

Harden the core experience. Make onboarding seamless, ship a managed cloud runtime, add developer tooling, and close critical gaps in agent reliability. This phase turns Agentra from a demo into a tool teams actually rely on.

### Milestones

#### 1.1 Cloud Runtime (Managed Execution)

Ship Agentra Cloud Runtime — a managed environment where agents run without requiring a local daemon.

- Containerized agent execution (Docker-based sandbox per task)
- Streaming log output over WebSocket to issue timeline
- Runtime health dashboard: CPU, memory, active tasks
- Automatic retries with configurable backoff

#### 1.2 Developer Experience & Onboarding

Reduce time-to-first-agent-task from hours to minutes.

- Homebrew Cask or shell/PowerShell install followed by `agentra setup`
- Guided in-app onboarding: create workspace → invite agent → run first task
- Sample Skills library: pre-built tasks (write tests, review PR, generate docs)
- Improved CLAUDE.md / AGENTS.md — context injection into every agent task

#### 1.3 GitHub Integration

Connect issues to pull requests and commits.

- OAuth GitHub App with repo access
- Issue ↔ PR ↔ commit bi-directional linking
- GitHub issue → Agentra task bridge (webhook-based)
- PR status badge on issue cards

#### 1.4 Agent Reliability & Observability

Instrument agent execution for reliability.

- Structured execution trace stored per task run
- Token + cost tracking per agent/task (Anthropic, OpenAI billing)
- Automatic blocker detection — agent posts structured blockers as sub-comments
- Human-in-the-loop approval gates (agent pauses, requests human sign-off)

### Phase 1 Features

| Feature | Description | Priority | Owner |
|---------|-------------|----------|-------|
| Cloud Runtime | Managed containerized agent execution; no local daemon needed. | P0 | Platform |
| GitHub Integration | Bi-directional PR/commit linking; GitHub issue → Agentra bridge. | P0 | Platform |
| CLI Installer | One-liner setup; agentra CLI with init, start, daemon commands. | P0 | DX |
| Onboarding Wizard | In-app guided tour for new workspaces to first agent task. | P1 | Product |
| Execution Traces | Structured per-task logs: steps, tools, tokens, cost. | P0 | Platform |
| Human-in-the-Loop | Agent pause-and-request-approval flow for sensitive actions. | P1 | Platform |
| Sample Skills Library | Pre-built skills: write tests, review PR, generate docs. | P1 | Product |
| Token Cost Tracking | Real-time cost estimates per agent task run. | P1 | Platform |

---

## Phase 2 — Q4 2025 (3 months)

**Agent Intelligence — IN PROGRESS**

Unlock agents that get smarter over time. Persistent memory, MCP tool integration, and multi-agent task graphs transform Agentra from a task runner into a true AI team layer.

### Milestones

#### 2.1 Persistent Agent Memory (RAG) — ✅ DONE

Give each agent a scoped memory store powered by pgvector.

- Per-agent + per-workspace memory store (pgvector embeddings)
- Automatic retrieval-augmented context injection at task start
- Memory viewer UI — browse, edit, delete agent memories
- Multi-strategy retrieval: semantic + keyword + graph + temporal (benchmarking against hindsight)
- Team conventions doc auto-synthesized from merged PRs and completed tasks

#### 2.2 Model Context Protocol (MCP) Integration — ✅ DONE

Expose Agentra's full data model as MCP tools.

- Agentra MCP Server: issues, comments, skills, memory as tools
- External MCP registry integration (GitHub, Jira, Slack, web search)
- Tool call audit log on issue timeline
- Per-tool permission scoping (workspace admin controls)

#### 2.3 Multi-Agent Task Graphs — 🚧 IN PROGRESS

Complex tasks decompose into sub-tasks, each assigned to specialist agents.

- Sub-task tree model: issues can have typed child tasks
- Planner agent role: decomposes parent task into ordered steps
- Parallel execution of independent sub-tasks
- Agent handoff protocol: context + artifacts passed between agents
- Visual task graph UI — DAG view showing agent chains

#### 2.4 Analytics & Insights Dashboard

Give teams the data to improve.

- Agent success rate, average task duration, failure categorization
- Cycle time charts by issue type, priority, and assignee
- Cost dashboard: daily/weekly/monthly spend per agent and workspace
- Team velocity trends: issues closed by human vs. agent over time

#### 2.5 Multi-Provider Support — 🚧 PLANNED (NEW P0)

Expand beyond Claude/Codex/OpenCode to match swarmclaw's 23+ providers.

- Provider interface: unified interface for all LLM backends
- Supported providers: Anthropic, OpenAI, OpenRouter, Google Gemini, DeepSeek, Groq, Together, Mistral, xAI, Fireworks, Nebius, DeepInfra, Ollama, LM Studio
- Per-agent provider selection and model override
- Cost tracking per provider

### Phase 2 Features

| Feature | Description | Priority | Owner | Status |
|---------|-------------|----------|-------|--------|
| Agent Memory Store | pgvector-backed per-agent memory with UI viewer. | P0 | Platform | ✅ Done (Phase 2.5) |
| RAG Context Injection | Automatic memory retrieval surfaced to agents at task start. | P0 | Platform | ✅ Done (Phase 2.5) |
| Agentra MCP Server | Expose issues, skills, memory as MCP tools to agents. | P0 | Platform | ✅ Done |
| External MCP Registry | GitHub, Slack, web search tools via standard MCP protocol. | P1 | Platform | 📐 Designed ([spec](archive/specs/2026-05-10-external-mcp-registry-design.md)) |
| Sub-Task Trees | Decompose issues into ordered / parallel child tasks. | P0 | Product | ✅ Done (Phase 2.5) |
| Multi-Agent Planner | Planner agent role that decomposes and delegates work. | P0 | Platform | ✅ Done (Phase 2.5) |
| Agent-to-Agent Handoff | Context + artifacts passed between agents during delegation. | P0 | Platform | ✅ Done (Phase 2.5) |
| Task Graph Visualization | DAG view of multi-agent execution chains. | P1 | Product | ✅ Done (Phase 2.5) |
| Execution Traces | Structured logging: steps, tools, tokens, cost per task. | P1 | Platform | ✅ Done (Phase 2.5) |
| Multi-Provider Support | 7 providers (3 CLI + 4 API) via Backend Facade. | P0 | Platform | ✅ Done (Phase 2.5) |
| Analytics Dashboard | Agent perf, cycle time, cost, velocity charts. | P1 | Product | Pending |
| Goal → DAG Auto-Decomposition | LLM auto-decomposes issue into task graph (learn from open-multi-agent). | P0 | Platform | ✅ Done ([spec](archive/specs/2026-05-10-goal-to-dag-auto-decomposition-design.md)) |
| One-Command Install | Homebrew/shell/PowerShell install plus `agentra setup`; signed SBOM/provenance release contract. | P0 | DX | 🧪 Beta; public assets on next tag |
| Git-Native Hooks | Auto-link commits to tasks; `post-merge` auto-completes tasks. | P1 | Platform | ✅ Done ([spec](archive/specs/2026-05-10-git-native-hooks-design.md)) |

---

## Phase 3 — 2026 Q3 (✅ DONE)

**Team Scale — COMPLETED via Council Route B — tag v0.5.0**

### Delivered (v0.5.0, Jul 2026)

| # | Feature | Status | Commit |
|---|---|---|---|
| #25 | Seat Management — members, invitations, role-bits, seat cap | ✅ Done | b5a6e61 |
| #28 | Stripe Billing — Checkout + Customer Portal + webhook + worker | ✅ Done | 7bd8f88 |
| #27 | Projects Hierarchy — issue grouping, milestones, board scope filter | ✅ Done | fbf33bf |

### What shipped

**Free tier ≤5 seats · $6/seat/mo base · $10/seat/mo with agent runtime.**

Backend:- `internal/handler/members.go` — MemberHandler with 5 REST endpoints
- `internal/handler/billing.go` — BillingHandler + StripeWebhook + CLI worker- `internal/handler/projects.go` — ProjectHandler with 12 REST endpoints
- `pkg/stripe/client.go` — Stripe SDK wrapper (Checkout, Portal, signature verification)
- Migrations: 042 (members), 043 (subscriptions/invoices/usage), 044 (projects/milestones)

Frontend:- `features/projects/components/project-picker.tsx` + project-detail.tsx- `features/issues/components/board-column.tsx` projectFilter wiring
- `app/(dashboard)/_components/billing-tab.tsx` — subscription + invoices + portal

Operations:
- `agentra billing sync` CLI worker — daily usage aggregation
- `agentra billing status` — display current plan + seats
- `/api/webhooks/stripe` — PUBLIC endpoint with signature verification

---

### Remaining feature-work (deferred, low priority, no blocker)

**Mostly "investigation / ecosystem" items, not engineering:** | # | Title | Type | Trigger to start |
|---|---|---|---|
| #30 | Dogfood verification | validation | Run agent on 5 real issues locally |
| #12 | Repo-DNA dynamic injection | enhancement | After dogfood confirms value |
| #13 | Agentra-Eval live benchmark | enhancement | After v0.5.0 deployed for 30 days |
| #24 | Enterprise SSO / compliance | feature | Once ARR > $5k MRR |

### Milestones (historical context)

#### 3.1 Seat Management — ✅ COMPLETE

#### 3.2 Stripe Billing Integration — ✅ COMPLETE

#### 3.3 Project Hierarchy & Sprint Planning — ✅ COMPLETE

Introduce Projects above Issues.

- Projects entity: group issues with goal, owner, deadline
- Milestones with progress bars based on closed issues
- Sprint board: backlog → active sprint → done columns
- Agent capacity planning: assign agent FTE allocation per sprint

#### 3.2 Collaborative Review Workflows

Human review remains essential.

- Review request stage in task lifecycle (agent → human)
- Inline diff viewer on issue timeline (code changes proposed by agent)
- Approve / Request Changes / Reject review actions
- Slack + email review request notifications

#### 3.3 Billing, Seats & Teams

Monetize the cloud offering.

- Free / Pro / Enterprise tiers (Stripe-based billing)
- Cloud runtime credits consumption model
- Workspace member roles: Owner, Admin, Member, Guest
- Invite-based onboarding with email/magic link

#### 3.4 Mobile Companion App

Ship iOS + Android apps for monitoring and approvals.

- React Native (Expo) for cross-platform iOS/Android
- Push notifications for agent completions, blockers, review requests
- One-tap approve / reject review requests
- Live agent status feed with execution trace preview

### Phase 3 Features

| Feature | Description | Priority | Owner |
|---------|-------------|----------|-------|
| Projects & Milestones | Group issues into projects with goals and deadlines. | P0 | Product |
| Sprint Board | Backlog → active sprint → done with agent allocation. | P1 | Product |
| Review Workflow | Agent proposes, human reviews inline diff, approves/rejects. | P0 | Product |
| Slack Integration | Review requests, agent status, blocker alerts in Slack. | P1 | Platform |
| Billing & Seats | Free/Pro/Enterprise with Stripe; cloud runtime credits. | P0 | Biz |
| Mobile App (iOS/Android) | React Native companion for monitoring and approvals. | P1 | Product |
| Push Notifications | Agent completion, blockers, review requests on mobile. | P1 | Product |
| Enterprise SSO (SAML) | SAML/OIDC SSO for enterprise workspace authentication. | P1 | Platform |

---

## Phase 4 — Q3 2026 (3 months)

**Platform Ecosystem — FUTURE**

Open Agentra to the world. Launch the Skills Marketplace, publish a public API, and add enterprise controls.

### Milestones

#### 4.1 Skills Marketplace

A public repository of community-built Skills.

- Public marketplace at marketplace.agentra.ai
- Skill publishing flow from workspace with version control
- One-click install into any workspace
- Revenue share for paid premium skills (70/30 split)

#### 4.2 Public API & Webhooks

A documented REST API + webhook system.

- Versioned public REST API (v1) with OpenAPI spec
- Webhook subscriptions for all entity events
- Zapier / Make.com native integration
- Developer portal with API key management and docs

#### 4.3 Enterprise Controls

Enterprise teams require audit trails and compliance controls.

- Immutable audit log for all workspace actions (SOC2 ready)
- Data residency selection (US, EU, APAC)
- Granular RBAC: custom roles with per-resource permissions
- Agent allowlist: workspace-level control over which models can be used

#### 4.4 Agentra as an MCP Server

Publish Agentra as a first-class MCP Server.

- Published MCP Server in Anthropic + OpenAI directories
- External agents can create/read/update Agentra issues
- Composable automation: trigger Agentra tasks from any AI assistant

### Phase 4 Features

| Feature | Description | Priority | Owner |
|---------|-------------|----------|-------|
| Skills Marketplace | Public marketplace with one-click install and revenue share. | P0 | Biz |
| Public REST API v1 | Versioned OpenAPI-documented API with webhooks. | P0 | Platform |
| Zapier/Make Integration | No-code workflow automation connectors. | P1 | Platform |
| Audit Log | Immutable workspace action log for SOC2 compliance. | P0 | Enterprise |
| Data Residency | US / EU / APAC data location selection. | P1 | Enterprise |
| Granular RBAC | Custom roles with per-resource permissions. | P1 | Enterprise |
| Agentra MCP Server | Publish to Claude/OpenAI tool directories. | P0 | Platform |
| Developer Portal | API key management, docs, usage analytics. | P1 | DX |

---

## Success Metrics

### Phase 1 Targets (by end Q3 2025)

| Metric | Target |
|--------|--------|
| Time to First Agent Task | <5m |
| Cloud Runtime Uptime | 99.5% |
| GitHub Repos Connected | 500+ |

### Phase 2 Targets (by end Q4 2025)

| Metric | Target |
|--------|--------|
| Agent Task Success Rate | ≥70% |
| Multi-Agent Tasks | 20% |
| MCP Tools Available | 10+ |

### Phase 3 Targets (by end Q2 2026)

| Metric | Target |
|--------|--------|
| Active Teams (Cloud) | 1,000 |
| Agent vs Human Issues | 40% |
| Mobile MAU | 30% |

### Phase 4 Targets (by end Q3 2026)

| Metric | Target |
|--------|--------|
| Marketplace Skills | 200+ |
| API Integrations | 50+ |
| Enterprise Customers | 25+ |

---

## Technical Foundations & Debt

Continuous investments in platform reliability, performance, and maintainability run in parallel with every phase.

| Feature | Description | Priority | Phase |
|---------|-------------|----------|-------|
| Test Coverage | Raise unit + integration coverage to >80%. Ship E2E coverage for critical agent task flows. | P0 | Ongoing |
| DB Schema Hardening | Add pgvector indexes for memory retrieval. Optimize task queue queries. | P0 | Phase 1 |
| WebSocket Resilience | Add reconnection backoff, heartbeat pings, and message replay. | P1 | Phase 1 |
| sqlc Query Optimization | Audit N+1 patterns in issue list + activity feed endpoints. Add pagination cursors. | P1 | Phase 1 |
| Secret Management | Migrate from .env to proper secret store (Vault or cloud-native KMS). | P1 | Phase 2 |
| Multi-tenancy Isolation | Row-level security in PostgreSQL for workspace data isolation. | P0 | Phase 2 |
| Rate Limiting | Per-user and per-workspace rate limits on API and WebSocket connections. | P1 | Phase 2 |
| Observability Stack | OpenTelemetry tracing, Prometheus metrics, structured log forwarding. | P1 | Phase 3 |
| Frontend Bundle Size | Code-split by route. Lazy-load heavy features. Target <200kb initial JS. | P2 | Phase 2 |
| Accessibility (a11y) | Full WCAG 2.1 AA compliance audit and remediation. | P2 | Phase 3 |

---

## Competitive Positioning (Updated 2026-05-10 v3)

| Capability | Agentra | open-multi-agent | wshobson/agents | edict | agent-tasks | kiwiq | CrewAI |
|-----------|---------|-----------------|-----------------|-------|-------------|-------|--------|
| Agent-native task assignment | ✅ | ✅ | ❌ (plugins) | ✅ | ✅ | ✅ | ❌ |
| Real-time agent status (WebSocket) | ✅ | ❌ (events) | N/A | ❌ (poll) | ❌ (stdio) | ❌ | ❌ |
| Goal → DAG auto-decomposition | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |
| Multi-agent orchestration | ✅ (Task Graph) | ✅ | ✅ (plugins) | ✅ | ❌ | ✅ | ✅ |
| LLM Providers | 7 | 10 | 185 | Multi | 1 | Multi | Multi |
| MCP Server | ✅ | ✅ | ❌ | ❌ | ✅ | ❌ | ❌ |
| Persistent Agent Memory | ✅ (4-strategy) | ✅ (pluggable) | ❌ | ❌ | ❌ | ✅ (multi-tier) | ❌ |
| Human-in-the-loop approvals | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| Skills / workflow templates | ✅ | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ |
| Self-hostable & open source | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Cloud Runtime | ✅ | ❌ | ❌ | ❌ | ❌ | ✅ | ❌ |
| Git-native (VCS integration) | ✅ | ❌ | ❌ | ❌ | ✅ | ❌ | ❌ |
| Persistent task UI | ✅ | ❌ | ❌ | ✅ (Dashboard) | ❌ | ❌ | ❌ |
| One-command install | ❌ | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ |
| Multi-workspace | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |

### Key Insight: Goal → DAG Gap

`open-multi-agent` (6k stars, MIT) has **Goal → DAG auto-decomposition** — give it a goal string, the Coordinator agent decomposes into a task DAG automatically. Agentra's Task Graph requires manual node/edge creation. This is the #1 competitive gap to close.

See [competitive-analysis-v2.md](archive/specs/2026-05-10-competitive-analysis-v2.md) for full analysis.

### Differentiation (Updated v3)

- **Real-time WebSocket + Persistent UI**: Only platform with both live agent status and full CRUD task management (0 of 40+ competitors have both)
- **4-Strategy Memory + RRF**: Multi-strategy retrieval (semantic + keyword + graph + temporal) with RRF fusion — competitive with SOTA (hindsight)
- **Multi-agent Task Graph**: DAG-based agent handoff with human review gates (most competitors lack handoff + approval combo)
- **Cloud Runtime + Self-hosted**: Managed execution + PostgreSQL-backed multi-tenancy (no competitor offers both)
- **Human-in-the-Loop**: Approval gates for sensitive agent actions — uniquely Agentra, no competitor has this
- **MCP Server**: 6+ tool categories exposed (issues, skills, memory, comments, agents, inbox) — more tools than agent-tasks or open-multi-agent

### New Competitive Threats (v3)

| Threat | Competitor | Stars | Mitigation |
|--------|-----------|-------|------------|
| Goal → DAG auto-decomposition | open-multi-agent | 6k | Phase 2: Auto-decompose API (P0, 30 days) |
| One-command install UX | agent-tasks, open-multi-agent | - | Phase 1: cross-platform installers + `agentra setup` (beta) |
| Claude Code ecosystem gravity | wshobson/agents, oh-my-claudecode | 33k–35k | Embrace MCP interop; don't compete on plugins |
| Python framework dominance | CrewAI, AutoGen, MetaGPT | 51k–67k | Position as platform, not framework; target Go+TS shops |
| OpenAI endorsement | openai/swarm | 21k | Monitor; this is research, not product |
| Enterprise compliance | EDDI | 338 | Phase 4: EU AI Act, SOC2 readiness |
| Spatial/visual orchestration | voicetree | 826 | Phase 3: Graph view improvements |

### Direct Competitor Watchlist

| Competitor | Why Watch | Risk Window |
|-----------|-----------|-------------|
| open-multi-agent | If they add persistence + UI, they become a direct competitor | 3-6 months |
| wshobson/agents | If Anthropic bakes plugin system into Claude Code natively | 6-12 months |
| Linear | If they add agent runtimes (they have the UI already) | 12+ months |
| oh-my-claudecode | Teams-first UX for Claude Code; growing at 33k stars | 3-6 months |

---

## Risks & Mitigations

| Risk | Description | Priority |
|------|-------------|----------|
| Agent reliability ceiling | LLM agents fail non-deterministically. Teams lose trust if too many tasks fail silently. | P0 |
| API provider dependency | Heavy reliance on Anthropic/OpenAI APIs introduces cost and availability risk. | P1 |
| Open-source commoditization | A large player (GitHub Copilot, Linear, Atlassian) could copy core features. | P1 |
| Scaling the daemon runtime | Local daemon model doesn't scale to large orgs; cloud runtime must be reliable. | P1 |
| Community fragmentation | Different teams run incompatible self-hosted forks; Skills can't be shared. | P2 |

### Mitigations

| Risk | Mitigation |
|------|------------|
| Agent reliability ceiling | Structured execution traces, automatic blocker reporting, configurable retry policies, human-in-the-loop gates. |
| API provider dependency | Model-agnostic SDK; local model support (Ollama); aggressive caching; cost caps per workspace. |
| Open-source commoditization | Network effects from Skills Marketplace; community moat; speed of agent intelligence roadmap. |
| Scaling the daemon runtime | Phase 1 cloud runtime shipped before enterprise sales. Extensive load testing before GA. |
| Community fragmentation | Strong versioned API contracts, marketplace as cloud-anchored canonical registry. |

---

## What We're Building Toward

The future of software teams isn't 10x engineers — it's 2 engineers directing 10 agents. Agentra is the operating layer that makes that possible.

This roadmap is a living document. Each phase gates on the success of the last. We ship incrementally, measure honestly, and adjust. If agent success rates don't reach 70% by Q4 2025, Phase 2 memory and multi-agent work gets reprioritized. If cloud runtime adoption outpaces expectations, we accelerate enterprise controls.

The bet is simple: in 18 months, assigning a task to an AI agent will feel as natural as assigning it to a colleague. Agentra is how we get there.

---

- **Repository**: github.com/agentra-ai/agentra
- **License**: Apache 2.0
- **Roadmap current as of**: May 2026
