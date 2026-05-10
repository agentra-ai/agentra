# Competitive Analysis v3 — 40+ Projects, 5 Dimensions

**Date**: 2026-05-10
**Status**: Final
**Scope**: 40+ projects across 6 categories, compared on technical architecture, core principles, implementation, interaction design, and product positioning

---

## Executive Summary

Agentra occupies a unique but threatened position. It is the **only** platform combining real-time WebSocket task management with persistent agent lifecycle, multi-workspace support, and cloud runtime. However, the competitive landscape has expanded dramatically since the last analysis:

- **Agent frameworks** (CrewAI 51k★, AutoGen 57k★, MetaGPT 67k★) dominate mindshare but lack task management UIs
- **Goal→DAG** (open-multi-agent 6k★) has auto-decomposition Agentra lacks
- **Claude Code ecosystem** (wshobson/agents 35k★, oh-my-claudecode 33k★) is exploding with plugin-based approaches
- **MCP-native tools** (agent-tasks, EDDI, mateclaw) show file-based and config-driven alternatives
- **SWE agents** (OpenHands 73k★, ChatDev 33k★) overlap on code execution but not task management

**The big takeaway**: Agentra's core differentiator (real-time task management + agent runtime) has no direct competitor yet. But closing the Goal→DAG gap and one-command install are existential priorities.

---

## 1. Competitive Landscape — All 40+ Projects

### 1.1 Categorization Matrix

```
                     ┌── Task Management UI ──┐
                     │                         │
                     │    ★ Agentra ★          │
                     │    agent-tasks          │
                     │    moo-tasks            │
                     │    crewboard-oss        │
                     │    clawbot-kanban       │
                     └───────────┬─────────────┘
                                 │
            ┌────────────────────┼────────────────────┐
            │                    │                    │
   ┌────────▼────────┐  ┌───────▼───────┐  ┌────────▼────────┐
   │ Agent Frameworks │  │ Orchestration │  │   SWE Agents    │
   │                  │  │               │  │                 │
   │ CrewAI    (51k)  │  │ wshobson/agts │  │ OpenHands (73k) │
   │ AutoGen   (57k)  │  │ oh-my-claude  │  │ ChatDev   (33k) │
   │ MetaGPT   (67k)  │  │ openai/swarm  │  │ AutoGPT  (184k) │
   │ LangGraph (31k)  │  │ edict   (15k) │  │ SuperAGI  (17k) │
   │ swarms     (6k)  │  │ open-multi-ag │  │                 │
   │ agency-swm (4k)  │  │ myclaude (2k) │  │                 │
   │ TaskWeaver (6k)  │  │ Shannon  (1k) │  │                 │
   │ maestr-orch (406)│  │ overstory(1k) │  │                 │
   └──────────────────┘  │ kiwiq    (1k) │  └──────────────────┘
                         │ voicetree(826)│
                         │ EDDI     (338)│
                         │ mateclaw (435)│
                         └───────────────┘
```

### 1.2 Full Project Catalog

| # | Project | Stars | Language | Category | Key Differentiator |
|---|---------|-------|----------|----------|-------------------|
| 1 | **AutoGPT** | 184,124 | Python | SWE Agent | Vision of accessible AI |
| 2 | **OpenHands** | 73,006 | Python | SWE Agent | AI-driven development |
| 3 | **MetaGPT** | 67,835 | Python | Agent Framework | "First AI software company" |
| 4 | **AutoGen** | 57,868 | Python | Agent Framework | Programming framework for agentic AI |
| 5 | **CrewAI** | 51,037 | Python | Agent Framework | Role-playing autonomous agents |
| 6 | **wshobson/agents** | 35,093 | Python | Orchestration | 80 plugins + 185 agents |
| 7 | **oh-my-claudecode** | 33,217 | TypeScript | Orchestration | Teams-first for Claude Code |
| 8 | **ChatDev** | 33,038 | Python | SWE Agent | Multi-agent software dev |
| 9 | **LangGraph** | 31,634 | Python | Agent Framework | Agents as graphs |
| 10 | **openai/swarm** | 21,465 | Python | Orchestration | Lightweight OpenAI orchestration |
| 11 | **SuperAGI** | 17,512 | Python | SWE Agent | Dev-first autonomous agents |
| 12 | **edict** | 15,678 | Python | Orchestration | 9-role Chinese bureaucracy |
| 13 | **hindsight** | 12,763 | Python/Go | Memory | SOTA biomimetic memory |
| 14 | **swarms** | 6,657 | Python | Agent Framework | Enterprise-grade framework |
| 15 | **TaskWeaver** | 6,161 | Python | Agent Framework | Code-first analytics agent |
| 16 | **open-multi-agent** | 6,086 | TypeScript | Orchestration | **Goal → DAG auto-decomposition** |
| 17 | **agency-swarm** | 4,340 | Python | Orchestration | Reliable multi-agent |
| 18 | **myclaude** | 2,638 | Go | Orchestration | Multi-agent Go workflow |
| 19 | **Shannon** | 1,833 | Go | Orchestration | Production Go orchestration |
| 20 | **overstory** | 1,284 | TypeScript | Orchestration | Coding agent orchestration |
| 21 | **make-it-heavy** | 1,112 | Python | Orchestration | Grok-like parallel agents |
| 22 | **kiwiq** | 1,043 | Python | Orchestration | JSON agents + multi-tier memory |
| 23 | **voicetree** | 826 | TypeScript | Orchestration | Spatial IDE graph view |
| 24 | **swarmclaw** | 472 | Node.js | Orchestration | 23+ providers + Electron |
| 25 | **mateclaw** | 435 | Java | Orchestration | MCP + Skills + Memory |
| 26 | **opencode-workspace** | 418 | TypeScript | Orchestration | OpenCode harness |
| 27 | **maestro-orchestrate** | 406 | JavaScript | Orchestration | 39 specialists + parallel |
| 28 | **orchestrator-supaconductor** | 348 | Python | Orchestration | Board of Directors model |
| 29 | **EDDI** | 338 | Java | Orchestration | Config-driven + MCP/A2A |
| 30 | **mentis** | 295 | Python | Orchestration | LangGraph-based |
| 31 | **Overseer** | 223 | Rust | Task Mgmt | VCS-native (jj/git) |
| 32 | **Tasuku** | 63 | Go | Task Mgmt | Git-friendly MCP tasks |
| 33 | **Beekeeper** | 52 | Python | Orchestration | Supervisor pattern |
| 34 | **AgentsBoard** | 11 | TypeScript | Task Mgmt | Kanban for agents |
| 35 | **agent-tasks** | - | TypeScript | Task Mgmt | **MCP CLI + git hooks** |
| 36 | **mcp-agent-tasks** | - | TypeScript | Task Mgmt | File-based MCP tasks |
| 37 | **crewboard-oss** | - | TypeScript | Task Mgmt | Kanban + AI agents |
| 38 | **mem0-mcp** | - | Python | Memory | MCP memory server |
| 39 | **agentmemo-mcp** | - | Python | Memory | MCP + approval gateway |
| 40 | **AI-teams-controller** | 7 | Python | Orchestration | tmux teams + memory |
| 41 | **clawbot-kanban** | - | TypeScript | Task Mgmt | Kanban for Clawbot |

---

## 2. Five-Dimension Comparison

### 2.1 Technical Architecture

| Dimension | Agentra | CrewAI | AutoGen | MetaGPT | LangGraph | open-multi-agent |
|-----------|---------|--------|---------|---------|-----------|-----------------|
| **Language** | Go + TypeScript | Python | Python | Python | Python | TypeScript |
| **Runtime** | Daemon + Cloud | Python process | Python process | Python process | Python process | Node.js process |
| **Database** | PostgreSQL + pgvector | None (in-memory) | None | None | Checkpointer | None (in-memory) |
| **Real-time** | WebSocket | None | None | None | None | onProgress events |
| **Multi-tenant** | Workspace isolation | N/A | N/A | N/A | N/A | N/A |
| **Agent Interface** | CLI Backend unified | Python classes | Python agents | Python roles | Graph nodes | TypeScript agents |
| **MCP Support** | ✅ Native server | ❌ | ❌ | ❌ | ❌ | ✅ connectMCPTools |
| **Deployment** | Docker Compose | `pip install` | `pip install` | `pip install` | `pip install` | `npm install` |

**Key insight**: Agentra is the only platform with enterprise infrastructure (PostgreSQL, WebSocket, multi-tenant, MCP server). Frameworks like CrewAI/AutoGen are libraries, not platforms. This is both a strength (enterprise readiness) and weakness (deployment complexity).

### 2.2 Core Principles — How They Decompose Work

| Approach | Project | Mechanism | Agentra Comparison |
|----------|---------|-----------|-------------------|
| **Role-based** | CrewAI, MetaGPT | Pre-defined roles (PM, Engineer, QA) | Skills system (more flexible) |
| **Goal→DAG** | open-multi-agent | LLM auto-decomposes goal into task DAG | ❌ Manual task graph (gap) |
| **Supervisor** | AutoGen, Beekeeper | Central agent delegates to workers | Daemon poll model |
| **Graph-based** | LangGraph | Explicit state graph with edges | Task Graph (similar model) |
| **Hierarchical** | edict | 9 fixed roles (三省六部制) | Dynamic agent assignment |
| **Plugin-based** | wshobson/agents | 185 agents as plugins | Skills marketplace (planned) |
| **Swarm** | openai/swarm | Lightweight agent handoffs | Task Graph handoff ✅ |
| **File-native** | agent-tasks | Markdown files + git hooks | PostgreSQL + WS (different paradigm) |
| **Config-driven** | kiwiq, EDDI | JSON agent definitions | SQL agent configs |

**The Goal→DAG gap is the #1 architectural weakness.** open-multi-agent proves the model: `runTeam(team, "Build a REST API")` → Coordinator generates:
```
design-api (architect) → implement (developer) ┐
                                               ├→ review-code (reviewer)
scaffold-tests (tester) ───────────────────────┘
```

### 2.3 Implementation — Provider Support

| Project | Provider Count | Model Types | Multi-modal |
|---------|---------------|-------------|-------------|
| swarmclaw | 23+ | CLI + API + OpenClaw | Yes |
| wshobson/agents | 185 (plugins) | Claude Code plugins | Yes |
| oh-my-claudecode | Multi | Claude Code + extensions | Yes |
| open-multi-agent | 10 | API (Anthropic, OpenAI, Gemini, DeepSeek) | No |
| kiwiq | Multi | API-based | No |
| **Agentra** | **7** | 3 CLI + 4 API | No |
| CrewAI | Multi | Any LangChain LLM | No |
| AutoGen | Multi | Any Python LLM client | No |
| LangGraph | Multi | Any LangChain LLM | No |

**Provider breadth isn't the urgent gap.** 7 well-chosen providers cover 95% of team needs. The bigger issue is deployment friction.

### 2.4 Interaction Design

| Project | Primary UI | Real-time | Learning Curve | Onboarding |
|---------|-----------|-----------|---------------|------------|
| **Agentra** | Next.js Web App | ✅ WebSocket | 5 min (Docker) | Manual config |
| open-multi-agent | HTML dashboard | ❌ events | 1 min | `npm install` |
| edict | Dashboard + Kanban | ❌ poll (5-10s) | 3 min | Clone + config |
| wshobson/agents | Claude Code (CLI) | N/A | 0 (inside CC) | Plugin install |
| CrewAI | None (library) | N/A | Library-level | `pip install` |
| AutoGen | None (library) | N/A | Library-level | `pip install` |
| agent-tasks | CLI + MCP | ❌ stdio | 30s | `npm install -g` |
| swarmclaw | Electron + Web | ✅ WebSocket | 1 min | Desktop app |
| voicetree | Spatial graph IDE | ❌ | 5 min | Web app |

**Agentra has the best persistent UI but the worst onboarding.** The gap between "5 minutes with Docker" and "30 seconds with npm" is existential for adoption.

### 2.5 Product Positioning

```
                         Persistent UI
                              │
                 agent-tasks   │   ★ Agentra ★
                 (MCP CLI)     │   (Platform)
                              │
     Light ──────────────────┼────────────────── Heavy
                              │
     open-multi-agent         │        edict
     (Library)                │        (System)
     CrewAI/AutoGen           │        MetaGPT
     (Frameworks)             │
                              │
                         No UI / API-only
```

**Agentra's unique position**: The only platform with both persistent task management UI AND agent runtime. But it sits alone in the "heavy" quadrant—every competitor is lighter to adopt.

| Project | Target User | Team Size | Business Model |
|---------|------------|-----------|----------------|
| **Agentra** | AI-native teams | 2-10 | Open source + Cloud |
| CrewAI | Python developers | Any | Open source + Enterprise |
| AutoGen | Enterprise developers | Any | MIT open source |
| MetaGPT | AI researchers | Any | MIT open source |
| LangGraph | LangChain ecosystem | Any | Open source + LangSmith |
| open-multi-agent | TypeScript devs | 1-5 | MIT open source |
| agent-tasks | Claude Code users | 1-3 | MIT open source |
| OpenHands | Individual devs | 1 | MIT open source |
| kiwiq | Enterprise AI teams | 10+ | Open source + Cloud |

---

## 3. Threat Matrix

### 3.1 Direct Threats (Task Management + Agent Runtime)

| Threat | Project | Stars | Overlap | Urgency |
|--------|---------|-------|---------|---------|
| Goal→DAG auto-decomposition | open-multi-agent | 6k | 90% | 🔴 P0 |
| One-command install | agent-tasks | - | 60% | 🔴 P0 |
| 23+ providers + Desktop | swarmclaw | 472 | 80% | 🟡 P1 |
| MCP CLI + git hooks | agent-tasks | - | 50% | 🟡 P1 |
| Enterprise memory + multi-tier | kiwiq | 1k | 30% | 🟢 P2 |

### 3.2 Indirect Threats (Could Pivot)

| Threat | Project | Stars | Risk | Likelihood |
|--------|---------|-------|------|------------|
| Framework adds task UI | CrewAI, AutoGen | 50k+ | High | Medium (libraries, not products) |
| LangGraph adds persistence | LangGraph | 31k | Medium | Low (LangSmith monetization) |
| Claude Code adds task mgmt | Anthropic | - | High | Low (platform, not vertical) |
| Linear adds agent runtime | Linear | - | Very High | Medium (different DNA) |
| GitHub Copilot adds issues | GitHub | - | Very High | Low (code-focused) |

### 3.3 Ecosystem Threats

| Threat | Project | Stars | What They Do Better |
|--------|---------|-------|---------------------|
| Plugin ecosystem momentum | wshobson/agents | 35k | 80 plugins, community gravity |
| Teams UX for Claude Code | oh-my-claudecode | 33k | Teams-first design, growing fast |
| OpenAI blessing | openai/swarm | 21k | Official OpenAI project |
| Enterprise Java ecosystem | EDDI, mateclaw | 700+ | EU AI Act compliance, Quarkus/Spring |

---

## 4. Agentra's Enduring Moat

After scanning 40+ projects, these capabilities remain **unique to Agentra**:

### 4.1 Real-time WebSocket + Persistent Task UI

No competitor has both:
- **open-multi-agent**: Has onProgress events but no persistent UI
- **edict**: Has dashboard but polling-based (5-10s delay)
- **swarmclaw**: Has WebSocket but no multi-workspace task CRUD
- **agent-tasks**: Has MCP tools but no Web UI at all

**Moat depth**: Deep. This is architecturally hard to retrofit into frameworks.

### 4.2 Agent Lifecycle with Human-in-the-Loop

Agentra's `queued → claimed → started → completed/failed` lifecycle with approval gates has no equivalent. Frameworks execute and return; they don't manage lifecycle state.

### 4.3 Multi-Workspace + Cloud Runtime

No competitor supports multiple workspaces or offers a managed cloud runtime for agent execution. This is enterprise table stakes that frameworks ignore.

### 4.4 4-Strategy Memory + RRF Fusion

Agentra's memory system (semantic + keyword + graph + temporal with RRF fusion) is competitive with hindsight's SOTA model. The only gap is auto-learning from task completion (already designed).

### 4.5 MCP Server + MCP Tool Integration

Agentra exposes issues, skills, memory, comments, agents, and inbox as MCP tools. Only agent-tasks and open-multi-agent also do MCP, but with fewer tools.

---

## 5. Priority Gaps to Close

### 5.1 🔴 P0: Goal → DAG Auto-Decomposition

**Competitor**: open-multi-agent (6k★)
**Current**: Manual task graph node/edge creation
**Target**: `POST /api/issues/:id/auto-decompose` → Planner agent auto-generates DAG

**Why urgent**: open-multi-agent is growing fast (6k stars with <1 year). If they add persistence + UI, they become a direct competitor. Agentra must ship auto-decomposition first.

**Design spec**: [goal-to-dag-auto-decomposition-design.md](2026-05-10-goal-to-dag-auto-decomposition-design.md)

**Implementation path**:
1. Planner agent receives issue title + description
2. LLM decomposes into typed child tasks with dependencies
3. Returns DAG (nodes + edges) for user review/approval
4. On approval, creates task graph and optionally starts execution
5. WebSocket pushes DAG state changes in real-time

### 5.2 🔴 P0: One-Command Install

**Competitor**: agent-tasks (`npm install -g`), open-multi-agent (`npm install`)
**Current**: Docker Compose (PostgreSQL + MinIO + server + web)
**Target**: `npx create-agentra` or `brew install agentra`

**Why urgent**: Every competitor installs in <1 minute. Agentra takes 5+ minutes and requires Docker knowledge. This kills adoption.

**Design spec**: [one-command-install-design.md](2026-05-10-one-command-install-design.md)

**Implementation path**:
1. Ship embedded SQLite mode for single-user/small-team (zero deps)
2. `npx create-agentra` CLI that scaffolds project
3. Single-binary Go build with embedded frontend
4. Docker Compose preserved for production deployments
5. `brew install agentra` for macOS users

### 5.3 🟡 P1: Git-Native Hooks

**Competitor**: agent-tasks (prepare-commit-msg, post-commit, post-merge)
**Current**: No git integration
**Target**: Git hooks that auto-link commits to tasks

**Why P1**: Differentiates from framework competitors (CrewAI, AutoGen) who can't do VCS integration.

**Design spec**: [git-native-hooks-design.md](2026-05-10-git-native-hooks-design.md)

### 5.4 🟡 P1: Provider Breadth Expansion

**Competitor**: swarmclaw (23+), wshobson/agents (185 plugins)
**Current**: 7 providers (3 CLI + 4 API)
**Target**: 15+ providers covering all major LLM APIs

**Why P1, not P0**: 7 providers cover 95% of use cases. Breadth is a marketing advantage, not a functional one yet. But as teams standardize on specific providers, missing one becomes a deal-breaker.

### 5.5 🟢 P2: Plugin/Skill Ecosystem

**Competitor**: wshobson/agents (80 plugins, 185 agents, 153 skills)
**Current**: Skills system (basic templates)
**Target**: Marketplace with installable, versioned skill packages

**Why P2**: Requires community first. Plugin ecosystem without users is empty shelves. Build after 1k active teams.

---

## 6. Architecture Lessons from Competitors

### 6.1 From open-multi-agent: Goal-First API Design

```typescript
// open-multi-agent pattern — Agentra should adopt
const result = await runTeam(team, "Build a REST API for todo list");
// → Coordinator decomposes goal into DAG
// → Executes with onProgress events
// → Returns synthesized result
```

Agentra equivalent:
```go
POST /api/workspaces/:id/execute
{
  "goal": "Build a REST API for todo list",
  "team": ["architect", "developer", "reviewer"],
  "auto_execute": true
}
// → Returns task graph ID + WebSocket streaming progress
```

### 6.2 From agent-tasks: File-Native Mode

```bash
# agent-tasks pattern
agent-tasks init              # Creates .mcp-tasks/
agent-tasks create "Add auth" # Creates task file
# Git hooks auto-link commits
```

Agentra equivalent: Optional Markdown export + git hook mode for teams that prefer file-native workflows alongside the Web UI.

### 6.3 From kiwiq: JSON Agent Definitions

```json
{
  "agent": {
    "name": "code-reviewer",
    "role": "Review code for security and quality",
    "memory_tiers": ["working", "episodic", "semantic"],
    "tools": ["github", "code-analysis"],
    "model": "claude-sonnet-4-6"
  }
}
```

Agentra already has agent configs in SQL; the lesson is making them portable and shareable as JSON/YAML.

### 6.4 From wshobson/agents: Progressive Plugin Loading

Only load the plugins/agents/skills needed for the current task. This reduces token costs by 60-80% compared to loading all context. Agentra's Skills system should adopt this pattern.

### 6.5 From EDDI: EU AI Act Compliance

EDDI (338★) has built-in compliance for EU AI Act, GDPR, HIPAA. As Agentra targets enterprise, compliance becomes table stakes. This is Phase 4 territory but worth tracking.

---

## 7. Revised Competitive Positioning

### 7.1 Updated Comparison Table

| Capability | Agentra | open-multi-agent | wshobson/agents | edict | agent-tasks | kiwiq | CrewAI |
|-----------|---------|-----------------|-----------------|-------|-------------|-------|--------|
| Agent-native task assignment | ✅ | ✅ | ❌ (plugins) | ✅ | ✅ | ✅ | ❌ (framework) |
| Real-time WebSocket | ✅ | ❌ (events) | N/A | ❌ (poll) | ❌ (stdio) | ❌ | ❌ |
| Goal → DAG auto-decomp | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |
| Multi-agent orchestration | ✅ (Task Graph) | ✅ | ✅ (plugins) | ✅ | ❌ | ✅ | ✅ |
| Persistent task UI | ✅ | ❌ | ❌ | ✅ (Dashboard) | ❌ | ❌ | ❌ |
| LLM Providers | 7 | 10 | 185 (plugins) | Multi | 1 (MCP) | Multi | Multi (LangChain) |
| MCP Server | ✅ | ✅ | ❌ | ❌ | ✅ | ❌ | ❌ |
| Agent Memory (4-strategy) | ✅ | ✅ (pluggable) | ❌ | ❌ | ❌ | ✅ (multi-tier) | ❌ |
| Human-in-the-loop | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| Skills/workflow templates | ✅ | ❌ | ✅ (plugins) | ❌ | ❌ | ❌ | ❌ |
| Cloud Runtime | ✅ | ❌ | ❌ | ❌ | ❌ | ✅ | ❌ |
| Git-native hooks | ❌ | ❌ | ❌ | ❌ | ✅ | ❌ | ❌ |
| Self-hostable | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| One-command install | ❌ | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ |
| Multi-workspace | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| Enterprise compliance | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |

### 7.2 The "Why Agentra Wins" Narrative

1. **Only platform with UI + runtime**: Frameworks execute code but don't manage tasks. Task managers don't run agents. Agentra does both.
2. **Real-time collaboration**: WebSocket means teams see agent progress live. No other platform offers this.
3. **Enterprise-ready infrastructure**: PostgreSQL, JWT, multi-workspace, RBAC — not bolt-ons, built from day one.
4. **Open source with cloud option**: Self-host for privacy, cloud for convenience. No vendor lock-in.
5. **Memory that learns**: 4-strategy RRF fusion means agents get smarter per workspace over time.

### 7.3 The "Why Not Agentra" Risks

1. **Too heavy to start**: Docker + PostgreSQL + MinIO vs `npm install`
2. **Missing Goal→DAG**: Competitor has the killer feature
3. **Go backend limits community**: Python/TypeScript dominate the AI ecosystem
4. **No mobile app**: Modern teams expect mobile

---

## 8. Strategic Recommendations

### 8.1 Immediate (Next 30 Days)

| Action | Priority | Impact | Effort |
|--------|----------|--------|--------|
| Ship Goal→DAG auto-decomposition API | P0 | High | Medium (design exists) |
| Ship `npx create-agentra` with SQLite mode | P0 | High | Medium (design exists) |
| Add open-multi-agent style `execute` endpoint | P0 | High | Low (wraps Task Graph) |

### 8.2 Short-term (30-60 Days)

| Action | Priority | Impact | Effort |
|--------|----------|--------|--------|
| Git-native hooks (prepare-commit-msg, post-merge) | P1 | Medium | Low |
| Add 5 more providers (DeepSeek, Groq, Together, Mistral, xAI) | P1 | Medium | Medium |
| Agent-to-agent handoff protocol polish | P1 | High | Low |

### 8.3 Medium-term (60-90 Days)

| Action | Priority | Impact | Effort |
|--------|----------|--------|--------|
| Plugin/skill packaging format (learn from wshobson/agents) | P2 | Medium | Medium |
| Analytics dashboard (agent performance, cost, velocity) | P1 | Medium | Large |
| Mobile companion app (React Native) | P2 | Medium | Large |

### 8.4 Long-term (Phase 4)

| Action | Priority | Impact | Effort |
|--------|----------|--------|--------|
| Public Skills Marketplace | P2 | High | Very Large |
| Enterprise SSO + compliance (EU AI Act, SOC2) | P2 | Medium | Large |
| TypeScript SDK (complement Go backend) | P2 | Medium | Medium |

---

## 9. Conclusion

Agentra is uniquely positioned as the only platform combining real-time task management with an agent runtime. But the window is closing: open-multi-agent has Goal→DAG, agent-tasks has frictionless install, and the Claude Code plugin ecosystem is exploding with wshobson/agents and oh-my-claudecode.

**The two existential priorities** are Goal→DAG auto-decomposition and one-command install. Ship these within 30 days, and Agentra has no direct competitor for the "AI-native task management platform" category. Delay longer, and the frameworks will add UIs before Agentra adds their killer features.

**The enduring bet**: Teams don't want another Python framework. They want a platform where agents are teammates with assigned tasks, real-time status, persistent memory, and human oversight. Agentra is the only project building that.

---

## References

- [CrewAI](https://github.com/crewAIInc/crewAI) (51,037★) — Role-playing autonomous agents
- [AutoGen](https://github.com/microsoft/autogen) (57,868★) — Programming framework for agentic AI
- [MetaGPT](https://github.com/FoundationAgents/MetaGPT) (67,835★) — Multi-agent meta-programming
- [LangGraph](https://github.com/langchain-ai/langgraph) (31,634★) — Agents as graphs
- [ChatDev](https://github.com/OpenBMB/ChatDev) (33,038★) — LLM-powered multi-agent collaboration
- [OpenHands](https://github.com/OpenHands/OpenHands) (73,006★) — AI-driven development
- [AutoGPT](https://github.com/Significant-Gravitas/AutoGPT) (184,124★) — Autonomous AI agents
- [SuperAGI](https://github.com/TransformerOptimus/SuperAGI) (17,512★) — Autonomous agent framework
- [openai/swarm](https://github.com/openai/swarm) (21,465★) — Lightweight agent orchestration
- [wshobson/agents](https://github.com/wshobson/agents) (35,093★) — Claude Code plugin ecosystem
- [oh-my-claudecode](https://github.com/Yeachan-Heo/oh-my-claudecode) (33,217★) — Teams-first Claude Code
- [edict](https://github.com/cft0808/edict) (15,678★) — 三省六部制 orchestration
- [open-multi-agent](https://github.com/open-multi-agent/open-multi-agent) (6,086★) — Goal→DAG
- [agency-swarm](https://github.com/VRSEN/agency-swarm) (4,340★) — Reliable multi-agent
- [myclaude](https://github.com/stellarlinkco/myclaude) (2,638★) — Go multi-agent workflow
- [Shannon](https://github.com/Kocoro-lab/Shannon) (1,833★) — Production Go orchestration
- [overstory](https://github.com/jayminwest/overstory) (1,284★) — Coding agent orchestration
- [kiwiq](https://github.com/rcortx/kiwiq) (1,043★) — Enterprise agent platform
- [swarmclaw](https://github.com/swarmclawai/swarmclaw) (472★) — 23+ providers + Electron
- [agent-tasks](https://github.com/nash-software/mcp-agent-tasks) — MCP task management + git hooks
- [EDDI](https://github.com/labsai/EDDI) (338★) — Config-driven + MCP/A2A
- [TaskWeaver](https://github.com/microsoft/TaskWeaver) (6,161★) — Code-first agent framework
- [voicetree](https://github.com/voicetreelab/voicetree) (826★) — Spatial IDE for multi-agent
- [mateclaw](https://github.com/matevip/mateclaw) (435★) — MCP + Skills + Memory
- [hindsight](https://github.com/vectorize-io/hindsight) (12,763★) — SOTA agent memory
- [Overseer](https://github.com/dmmulroy/overseer) (223★) — VCS-native task management
- [Tasuku](https://github.com/iheanyi/tasuku) (63★) — Git-friendly MCP tasks
- [Agentra ROADMAP.md](../ROADMAP.md)
