# Agentra Integrated Enhancement Design — v3 Competitive Response

**Date**: 2026-05-10
**Status**: Approved
**Based on**: [Competitive Analysis v3](2026-05-10-competitive-analysis-v3.md) — 40+ projects, 5 dimensions
**Goal**: Close the 6 priority gaps identified in competitive analysis within 90 days

---

## Executive Summary

The v3 competitive analysis scanned 40+ projects and identified 6 gaps. This design covers the architecture, implementation, and timeline to close them. The two existential priorities (P0, 30-day target) are Goal→DAG auto-decomposition and one-command install. Without these, Agentra loses its window.

---

## 1. P0: Goal → DAG Auto-Decomposition (30 days)

### 1.1 The Gap

**Competitor**: open-multi-agent (6k★) — `runTeam(team, "Build a REST API")` → Coordinator agent auto-decomposes goal into task DAG with dependency resolution.

**Agentra current**: Task Graph exists but requires manual node/edge creation via UI. No auto-decomposition.

**Why P0**: This is open-multi-agent's entire value proposition. If they add persistence + UI (3-6 month risk window), they become a direct, lighter-weight competitor.

### 1.2 Design

**API**:
```
POST /api/issues/:id/auto-decompose
Authorization: Bearer <jwt>
X-Workspace-ID: <ws-id>

Response 200:
{
  "task_graph_id": "uuid",
  "nodes": [
    {
      "id": "uuid",
      "title": "Design API schema",
      "description": "...",
      "agent_type": "architect",
      "dependencies": [],
      "estimated_effort": "30m"
    },
    {
      "id": "uuid",
      "title": "Implement endpoints",
      "agent_type": "developer",
      "dependencies": ["prev-node-id"],
      "estimated_effort": "2h"
    }
  ],
  "edges": [
    {"from": "node-1", "to": "node-2", "type": "blocks"}
  ],
  "parallel_groups": [["node-2", "node-3"]],
  "critical_path": ["node-1", "node-2", "node-5"]
}

POST /api/issues/:id/auto-decompose/apply
# Accepts the generated DAG and creates the task graph
# Optional auto_execute flag to start immediately
```

**Planner Agent Prompt Design** (from open-multi-agent learnings):
```go
const autoDecomposePrompt = `You are a task decomposition planner. Given a goal, break it down into a DAG of subtasks.

Rules:
1. Each node must be independently executable by a single agent
2. Identify true dependencies (not artificial ordering)
3. Maximize parallelism — mark independent nodes
4. Assign agent specializations: architect, developer, tester, reviewer, devops
5. Estimate effort per node (15m, 30m, 1h, 2h, 4h, 8h)
6. Identify the critical path

Output as JSON:
{
  "nodes": [
    {"title": "...", "description": "...", "agent_type": "...", "dependencies": ["node-id"], "estimated_effort": "..."}
  ],
  "parallel_groups": [["node-id", "node-id"]],
  "critical_path": ["node-id", "..."],
  "rationale": "Why this decomposition structure"
}`
```

**WebSocket Events**:
```json
// Auto-decompose started
{"type": "task.auto_decompose.started", "issue_id": "uuid"}

// Decomposition complete, ready for review
{"type": "task.auto_decompose.completed", "issue_id": "uuid", "task_graph_id": "uuid", "node_count": 5}

// User approved (or rejected)
{"type": "task.auto_decompose.applied", "issue_id": "uuid", "task_graph_id": "uuid"}
```

**Implementation files**:
- `server/internal/handler/auto_decompose.go` — HTTP handler (already scaffolded)
- `server/internal/service/planner.go` — Planner agent logic (already scaffolded)
- `server/pkg/agent/planner_prompt.go` — Decomposition prompt template
- `apps/web/src/features/issues/components/AutoDecomposeReview.tsx` — Review UI

### 1.3 Review UI Flow

```
Issue page → "Auto-Decompose" button
  → Loading spinner ("Planner agent is decomposing...")
  → DAG preview (nodes + edges, visual graph)
  → User can: reorder, add/remove nodes, edit dependencies
  → "Apply & Create Tasks" button
  → Task Graph created, optionally auto-executed
```

### 1.4 Success Metrics
- Decomposition time: <15 seconds
- Node count: 3-12 per issue (goal-dependent)
- User acceptance rate: >80% (apply without major edits)
- Parallelism ratio: >40% of nodes in parallel groups

---

## 2. P0: One-Command Install (30 days)

### 2.1 The Gap

**Competitor**: agent-tasks (`npm install -g agent-tasks && agent-tasks init`), open-multi-agent (`npm install && export ANTHROPIC_API_KEY=...`)

**Agentra current**: Docker Compose (PostgreSQL + MinIO + server + web). Requires Docker knowledge, 5+ config steps.

**Why P0**: Every competitor installs in <1 minute. Agentra's deployment friction is the #1 adoption killer. Teams evaluating Agentra won't get past the README.

### 2.2 Design

**Three deployment tiers**:

| Tier | Target | Database | Dependencies | Install Time |
|------|--------|----------|-------------|-------------|
| **Quickstart** | Evaluation, single-user | SQLite (embedded) | None | <30s |
| **Standard** | Small teams | PostgreSQL (docker) | Docker | <2min |
| **Production** | Teams, enterprises | PostgreSQL + MinIO | Docker Compose | <5min |

**Quickstart mode** (`npx create-agentra`):
```bash
npx create-agentra my-project
cd my-project
# .env auto-generated with SQLite defaults
# Starts server + web on localhost
npm start
```

**Embedded SQLite mode**:
```go
// server/pkg/db/sqlite.go
// Conditionally use SQLite when DATABASE_URL=sqlite://agentra.db
// Same sqlc queries, different driver
// pgvector → sqlite-vec extension for embeddings
```

**Single-binary build** (Go embed):
```go
// server/cmd/standalone/main.go
//go:embed all:../../apps/web/out/*
var frontendFS embed.FS

// Single `agentra` binary that serves both API + frontend
// No Node.js required for production
```

**Homebrew**:
```ruby
# Formula: agentra.rb
brew install agentra-ai/tap/agentra
agentra init my-project
agentra start
```

### 2.3 Implementation files
- `server/pkg/db/sqlite/` — SQLite driver with sqlite-vec
- `server/cmd/standalone/` — Single-binary build target
- `packages/create-agentra/` — npm package for `npx create-agentra`
- `.github/workflows/release.yml` — Already exists; add Homebrew + npm publish

### 2.4 Success Metrics
- Quickstart time: <30 seconds to running app
- Standard time: <2 minutes to running app
- `npx create-agentra` published to npm
- `brew install agentra` available for macOS

---

## 3. P1: Git-Native Hooks (30-60 days)

### 3.1 The Gap

**Competitor**: agent-tasks — Git hooks (prepare-commit-msg auto-links task ID, post-commit links commit, post-merge auto-completes task)

**Why P1**: Differentiates from Python frameworks. Developer-native workflow that GitHub-integrated teams expect.

**Design spec**: [git-native-hooks-design.md](2026-05-10-git-native-hooks-design.md)

### 3.2 Implementation Summary

**Three hooks**:
```bash
# .git/hooks/prepare-commit-msg
# Auto-prepend issue ID to commit message from branch name
# Branch: feature/AG-123-add-auth → Commit: "AG-123: Add authentication"

# .git/hooks/post-commit
# POST /api/git/commits {sha, message, branch} → link to issue

# .git/hooks/post-merge
# If merge commit message contains "Fixes AG-123" → auto-complete task
```

**Agentra CLI command**:
```bash
agentra git init    # Installs hooks into current repo
agentra git status  # Shows linked issues/commits
agentra git link    # Manually link commit to issue
```

### 3.3 Success Metrics
- Hook latency: <100ms for prepare-commit-msg
- Auto-link accuracy: >95% (issue ID in branch name)
- Integration depth: 3 hooks + agentra CLI

---

## 4. P1: Provider Breadth (7 → 15)

### 4.1 The Gap

**Competitor**: swarmclaw (23+), wshobson/agents (185 plugins)

**Why P1 not P0**: 7 well-chosen providers cover 95% of use cases. But expanding signals platform maturity and prevents "you don't support my provider" deal-breakers.

### 4.2 Target Providers

| # | Provider | Type | Priority | Reason |
|---|----------|------|----------|--------|
| 1-7 | **Existing** (Claude, Codex, OpenCode, Anthropic API, OpenAI, OpenRouter, Ollama) | CLI + API | ✅ Done | Core coverage |
| 8 | **Google Gemini API** | API | High | Google ecosystem |
| 9 | **DeepSeek** | API | High | Cost-effective, popular in Asia |
| 10 | **Groq** | API | High | Fastest inference (LPU) |
| 11 | **Mistral** | API | Medium | EU data residency |
| 12 | **xAI Grok** | API | Medium | Growing ecosystem |
| 13 | **Together AI** | API | Medium | Open model hosting |
| 14 | **Fireworks** | API | Low | Specialized models |
| 15 | **LM Studio** | Local | Low | Local model parity with Ollama |

### 4.3 Implementation

Each new API provider requires:
```go
// ~150 lines per provider
type GeminiBackend struct {
    client  *genai.Client
    model   string
}

func (b *GeminiBackend) Execute(ctx context.Context, prompt string, opts *ExecuteOptions) (*Result, error) {
    // Provider-specific API call → unified Result
}
```

---

## 5. P2: Plugin/Skill Ecosystem (Phase 4)

### 5.1 The Lesson from wshobson/agents

wshobson/agents has 80 plugins, 185 agents, 153 skills, and 16 orchestrators with 35k stars. The key architectural insight:

**Progressive loading**: Only load plugins/agents/skills needed for the current task. Reduces token costs 60-80%.

**Agentra's path**: Skills system exists today as templates. Phase 4 evolution:
1. Package skills as installable `.agentra-skill` bundles (manifest + prompts + tools)
2. Marketplace at marketplace.agentra.ai with one-click install
3. Revenue share for premium skills (70/30 split)
4. Progressive loading inspired by wshobson/agents

---

## 6. Implementation Timeline

```
Week 1-2:  Goal→DAG Auto-Decomposition
           └── Planner agent + auto-decompose API + review UI

Week 3-4:  One-Command Install
           └── SQLite mode + create-agentra npm + single binary

Week 5-6:  Git-Native Hooks
           └── 3 git hooks + agentra git CLI + commit→issue linking

Week 7-8:  Provider Expansion (Part 1)
           └── Gemini, DeepSeek, Groq backends

Week 9-10: Provider Expansion (Part 2)
           └── Mistral, xAI, Together, Fireworks, LM Studio

Week 11-12: Integration Testing & Performance
            └── End-to-end flows, load testing, docs
```

---

## 7. Architecture Integration

```
┌─────────────────────────────────────────────────────────────┐
│                      Agentra Platform                        │
│                                                              │
│  ┌──────────────────┐  ┌──────────────┐  ┌───────────────┐ │
│  │ Auto-Decompose   │  │ Git Hooks    │  │ One-Cmd Init  │ │
│  │ (Planner Agent)  │  │ (CLI + Hook) │  │ (SQLite mode) │ │
│  └────────┬─────────┘  └──────┬───────┘  └───────┬───────┘ │
│           │                   │                   │          │
│  ┌────────▼───────────────────▼───────────────────▼───────┐ │
│  │                   Task Graph Engine                    │ │
│  │  ┌─────────┐  ┌──────────┐  ┌────────────────────┐   │ │
│  │  │ DAG     │  │ Handoff  │  │ Parallel Executor  │   │ │
│  │  │ Builder │  │ Protocol │  │                    │   │ │
│  │  └─────────┘  └──────────┘  └────────────────────┘   │ │
│  └──────────────────────┬───────────────────────────────┘ │
│                         │                                   │
│  ┌──────────────────────▼───────────────────────────────┐ │
│  │              Agent Backend Facade (15 providers)      │ │
│  │  Claude | Codex | OpenAI | Gemini | DeepSeek | ...   │ │
│  └──────────────────────┬───────────────────────────────┘ │
│                         │                                   │
│  ┌──────────────────────▼───────────────────────────────┐ │
│  │    PostgreSQL/pgvector  │  SQLite (Quickstart mode)   │ │
│  └──────────────────────────────────────────────────────┘ │
│                                                              │
│  ┌──────────────────────────────────────────────────────┐   │
│  │  WebSocket Hub  │  MCP Server  │  REST API (Chi)     │   │
│  └──────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

---

## 8. Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Auto-decomposition quality is poor | Medium | High | Human review gate; iterative prompt tuning; benchmark against open-multi-agent outputs |
| SQLite mode lacks pgvector features | Medium | Medium | sqlite-vec extension; document as "evaluation only" |
| Git hooks conflict with existing hooks | Low | Medium | `agentra git init --merge` mode; backup existing hooks |
| Provider API instability | Low | Low | Unified error handling; automatic fallback chain |
| Scope creep (adding too many providers) | Medium | Medium | Hard cap at 15; stop and reassess |

---

## 9. Success Criteria (90-day)

1. **Goal→DAG**: Auto-decomposition ships, user acceptance >80%
2. **One-command install**: `npx create-agentra` works on macOS/Linux, <30s to running app
3. **Git hooks**: 3 hooks ship, auto-link accuracy >95%
4. **Providers**: 15+ providers supported
5. **No regression**: All existing tests pass; no degradation in agent task success rate
6. **Competitive position**: No other project has auto-decomposition + persistent UI + WebSocket real-time + human-in-the-loop

---

## 10. References

- [Competitive Analysis v3](2026-05-10-competitive-analysis-v3.md) — Full 40+ project comparison
- [Goal → DAG Auto-Decomposition Design](2026-05-10-goal-to-dag-auto-decomposition-design.md)
- [One-Command Install Design](2026-05-10-one-command-install-design.md)
- [Git-Native Hooks Design](2026-05-10-git-native-hooks-design.md)
- [External MCP Registry Design](2026-05-10-external-mcp-registry-design.md)
- [ROADMAP.md](../../ROADMAP.md)
- [open-multi-agent](https://github.com/open-multi-agent/open-multi-agent) — Primary competitive reference
- [agent-tasks](https://github.com/nash-software/mcp-agent-tasks) — Git hooks reference
- [wshobson/agents](https://github.com/wshobson/agents) — Plugin ecosystem reference
