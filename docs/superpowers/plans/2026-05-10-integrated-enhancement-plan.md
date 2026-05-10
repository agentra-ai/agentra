# Agentra Integrated Enhancement Plan

**Date**: 2026-05-10
**Status**: Draft
**Based on**: [Competitive Analysis v3](../specs/2026-05-10-competitive-analysis-v3.md) — 40+ projects
**Goal**: Close 6 priority gaps identified in competitive analysis within 90 days

---

## Executive Summary

v3 competitive analysis of 40+ projects reveals Agentra occupies a **unique but time-limited position**:

- **Only platform** combining real-time WebSocket task management + persistent agent lifecycle + multi-workspace + cloud runtime
- **Two existential gaps**: Goal→DAG auto-decomposition (open-multi-agent has it) and one-command install (agent-tasks/nanostack have it)
- **Window**: 3-6 months before open-multi-agent adds persistence or Linear adds agent runtimes

**Strategy**: Ship P0 gaps in 30 days, P1 in 60 days, P2 in 90 days. Preserve Agentra's unique moat (UI+runtime+multi-workspace) while closing functional gaps.

---

## Competitive Gaps Summary

| Gap | Competitor | Stars | Urgency | Status |
|-----|------------|-------|---------|--------|
| Goal → DAG auto-decomposition | open-multi-agent | 6k | 🔴 P0 | 📐 Designed |
| One-command install | agent-tasks, nanostack | - | 🔴 P0 | 📐 Designed |
| Git-native hooks | agent-tasks | - | 🟡 P1 | 📐 Designed |
| Provider breadth (7→15+) | swarmclaw | 472 | 🟡 P1 | Not started |
| Goal-first API | open-multi-agent | 6k | 🟡 P1 | Not started |
| Real-time DAG visualization | agent-flow | 898 | 🟢 P2 | Not started |

---

## P0: 30-Day Sprint (Existential Priorities)

### P0.1: Goal → DAG Auto-Decomposition

**Gap**: open-multi-agent's `runTeam(team, "Build REST API")` auto-decomposes into task DAG. Agentra requires manual node/edge creation.

**Implementation**:
```
POST /api/issues/:id/auto-decompose
  → Planner Agent analyzes issue description
  → Returns DAG structure (nodes + dependencies)
  → User reviews, edits, confirms
  → Task Graph created and queued
```

**Files to modify**:
- `server/internal/handler/auto_decompose.go` (new)
- `server/internal/service/planner.go` (exists, extend)
- `server/pkg/agent/providers/` (use for LLM calls)

**Spec**: [goal-to-dag-auto-decomposition-design.md](../specs/2026-05-10-goal-to-dag-auto-decomposition-design.md)

**Acceptance criteria**:
- [ ] `POST /api/issues/:id/auto-decompose` returns valid DAG JSON
- [ ] DAG includes node titles, agent types, dependencies, estimates
- [ ] Human review modal in frontend before execution
- [ ] Execution via existing Task Graph infrastructure

---

### P0.2: One-Command Install

**Gap**: Agentra requires Docker + PostgreSQL + MinIO. Competitors: `npm install` or `git clone + shell`.

**Implementation**:
```bash
# Option 1: npx
npx create-agentra@latest

# Option 2: brew
brew install agentra-ai/tap/agentra

# Option 3: Embedded SQLite (zero-dep dev mode)
agentra dev --sqlite
```

**Three modes**:
1. **Full (Docker Compose)**: PostgreSQL + MinIO + server + web
2. **Standard (bare metal)**: External PostgreSQL, embedded binary
3. **Dev (SQLite)**: Zero-dep local development

**Files to create/modify**:
- `server/cmd/agentra/main.go` (CLI entry)
- `scripts/create-agentra/` (scaffold templates)
- `Makefile` (update targets)
- `.env.example` (simplify for SQLite mode)

**Spec**: [one-command-install-design.md](../specs/2026-05-10-one-command-install-design.md)

**Acceptance criteria**:
- [ ] `npx create-agentra@latest` bootstraps complete workspace
- [ ] `agentra dev --sqlite` runs without Docker
- [ ] `agentra dev` (full) works with single docker compose up
- [ ] Onboarding wizard completes in <5 minutes

---

## P1: 60-Day Sprint (Feature Parity)

### P1.1: Git-Native Hooks

**Gap**: agent-tasks auto-links commits to issues via `prepare-commit-msg` hook. Agentra has no VCS integration.

**Implementation**:
```bash
# Install git hook template
agentra git-hook install

# Generates: .git/hooks/prepare-commit-msg
# Detects issue ID from branch name (agentra/ISSUE-123/...)
# Auto-inserts: [AGT-123] 
```

**Features**:
- `prepare-commit-msg`: Auto-insert issue reference
- `post-merge`: Auto-transition issue to "In Review" on merge
- `post-checkout`: Show current issue context

**Files to create**:
- `server/pkg/git/hooks.go` (hook installer)
- `server/internal/handler/git_hook.go` (webhook receiver)
- `migrations/037_git_hooks.up.sql` (schema)

**Spec**: [git-native-hooks-design.md](../specs/2026-05-10-git-native-hooks-design.md)

**Acceptance criteria**:
- [ ] `agentra git-hook install` sets up hooks
- [ ] Commit messages include issue ID from branch
- [ ] Merged PRs auto-transition linked issues

---

### P1.2: Provider Breadth (7 → 15+)

**Gap**: swarmclaw supports 23+ providers. Agentra supports 7 (3 CLI + 4 API).

**Implementation**:
Add API-based providers:
- DeepSeek (deepseek-chat)
- Groq (llama, mixtral)
- Together (mistral, llama)
- Mistral (mistral-large)
- xAI (grok-beta)
- Fireworks (llama)
- DeepInfra (mixtral, llama)

**Files to modify**:
- `server/pkg/agent/providers/` (add new files)
- `server/pkg/agent/backend.go` (facade already exists)

**Acceptance criteria**:
- [ ] All new providers respond to `Backend.Execute()`
- [ ] Provider selection in agent settings UI
- [ ] Cost tracking per provider

---

### P1.3: Goal-First API

**Gap**: open-multi-agent has `runTeam(team, goal)` — one endpoint for full workflow.

**Implementation**:
```json
POST /api/workspaces/:id/execute
{
  "goal": "Build a REST API for user management",
  "team": ["architect", "developer", "reviewer"],
  "constraints": {
    "language": "Go",
    "framework": "chi"
  }
}

→ Returns: { "task_graph_id": "...", "stream": "ws://..." }
```

**Files to create**:
- `server/internal/handler/execute.go` (new endpoint)
- `server/internal/service/executor.go` (orchestrates planner + task graph)

**Acceptance criteria**:
- [ ] Single API call creates full task graph
- [ ] WebSocket streams real-time progress
- [ ] Human approval gate before execution

---

## P2: 90-Day Sprint (Differentiation)

### P2.1: Real-Time DAG Visualization

**Gap**: agent-flow (898★) shows real-time Claude Code agent orchestration. Agentra's DAG view is static.

**Implementation**:
- WebSocket pushes graph state changes
- Frontend renders animated DAG with agent status nodes
- Show node-level progress (thinking → acting → blocked → done)
- Integrate agent-flow's visualization concepts

**Files to modify**:
- `server/internal/realtime/hub.go` (broadcast graph events)
- `apps/web/features/issues/components/DAGView.tsx` (enhance)
- `server/pkg/taskgraph/` (emit events on state changes)

**Acceptance criteria**:
- [ ] DAG nodes animate on state change
- [ ] Real-time agent progress visible without refresh
- [ ] Blockers highlighted in red

---

### P2.2: Pre-Defined Specialist Agents

**Gap**: maestro-orchestrate has 39 specialists, edict has 9 roles. Agentra has no pre-configured agent types.

**Implementation**:
Pre-built Skill templates for common roles:
- `architect` — System design, API spec
- `developer` — Implementation, tests
- `reviewer` — Code review, security scan
- `devops` — CI/CD, deployment
- `docs` — Documentation, README
- `qa` — Testing, edge cases

**Files to create**:
- `server/pkg/skills/specialists/` (skill templates)
- `apps/web/features/skills/SpecialistPicker.tsx` (UI)

**Acceptance criteria**:
- [ ] 10+ specialist skills pre-installed
- [ ] One-click apply to issue
- [ ] Each has system prompt, tools, memory config

---

## Implementation Order

```
Week 1-2:   P0.1 Goal→DAG API + frontend modal
Week 3-4:   P0.2 One-command install (SQLite mode)
Week 5-6:   P1.1 Git-native hooks
Week 7-8:   P1.2 Provider breadth (5 new providers)
Week 9-10:  P1.3 Goal-first API
Week 11-12: P2.1 Real-time DAG visualization
Week 13-14: P2.2 Pre-defined specialists
```

---

## Files Reference

### Existing specs (reuse):
- [goal-to-dag-auto-decomposition-design.md](../specs/2026-05-10-goal-to-dag-auto-decomposition-design.md)
- [one-command-install-design.md](../specs/2026-05-10-one-command-install-design.md)
- [git-native-hooks-design.md](../specs/2026-05-10-git-native-hooks-design.md)
- [agentra-enhancement-v3.md](../specs/2026-05-10-agentra-enhancement-v3.md)

### Key existing code:
- `server/internal/service/planner.go` — Planner logic
- `server/internal/handler/auto_decompose.go` — Auto-decompose handler (new)
- `server/pkg/agent/backend.go` — Provider facade
- `server/pkg/agent/providers/` — Provider implementations
- `server/internal/realtime/hub.go` — WebSocket hub

---

## Success Metrics

| Metric | Target | By |
|--------|--------|-----|
| Time to first agent task | <5 min | Week 4 |
| Goal→DAG API latency | <10s | Week 2 |
| Providers supported | 15+ | Week 8 |
| Git hook adoption | 50% workspaces | Week 12 |
| Real-time DAG engagement | +30% task graph usage | Week 14 |
