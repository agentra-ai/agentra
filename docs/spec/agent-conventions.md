# Agent Conventions Spec — `.agentra/AGENT.md`

> Standard format for injecting repository-specific knowledge into coding agents.
> Implements issue [#10](https://github.com/agentra-ai/agentra/issues/10).

## Problem

Today, Agentra's `SpecialistAgentTemplate` hardcodes this project's own coding conventions for 6 roles (Frontend, Backend, Test, Security, DevOps, Tech Writer) inside the React component. Two issues:

1. Every other OSS project must repeat this work from scratch.
2. Conventions are static — they rot as the codebase evolves.

## Solution

Define a **portable convention file** format that any project can drop into `.agentra/AGENT.md`. Agentra's daemon reads it at task start and injects it into the agent's prompt.

## Format

```markdown
# .agentra/AGENT.md — Agent Knowledge Manifest

## Stack
- Language: TypeScript 5.x
- Framework: React 18 with App Router
- State: Zustand (client) + sqlc (server queries)
- Database: PostgreSQL 17 + pgvector
- Runtime: Next.js 16 standalone + Go Chi backend

## Import Conventions
- Frontend uses `@/` alias (maps to `apps/web/`)
- Backend uses module path `github.com/agentra-ai/agentra/server/internal/...`
- Never relative-import across feature boundaries

## State Management
- Zustand for client state
- React Context only for connection lifecycle (WSProvider)
- Local useState for component-scoped UI state

## Forbidden Patterns
- Don't use `any` in new code (strict mode)
- Don't call `useRouter` from zustand stores
- Don't dual-write (no fallback paths, compatibility layers)

## Preferred Patterns
- Feature-first directory layout: `features/<domain>/{store,components,hooks,config}`
- Optimistic updates with rollback for all mutations
- WS sync = invalidate + refetch; never dual-write

## Testing
- Frontend: Vitest (mock external deps only)
- Backend: `go test` (fixtures in test DB)
- E2E: Playwright (requires full stack running)

## Agent-Specific Tools
- Frontend template: nextjs.org/docs, shadcn/ui
- Backend template: chirouter documentation, sqlc.yaml
- MCP tools: issues, comments, skills, memory
```

## Field Spec

| Field | Required | Type | Description |
|-------|----------|------|-------------|
| `Stack` | Yes | H2 section | Runtime + framework identifiers |
| `Import Conventions` | No | H2 section | Module resolution rules |
| `State Management` | No | H2 section | Where state lives and how it flows |
| `Forbidden Patterns` | No | H2 section | Never-do-this rules |
| `Preferred Patterns` | No | H2 section | Always-do-this rules |
| `Testing` | No | H2 section | Test framework + fixture strategy |
| `Agent-Specific Tools` | No | H2 section | URLs / CLI paths the agent should know |
| `Architecture Decisions` | No | H2 section | ADR-style context (why we chose X over Y) |

## Resolution Rules

At task start, Agentra's prompt builder:

1. Reads `.agentra/AGENT.md` from the repo root (or `docs/AGENT.md`, or `CLAUDE.md` as fallback).
2. Strips the H1 title line, keeps all H2+ sections as-is.
3. Prepends a header: `# Project Context (auto-injected from .agentra/AGENT.md)`.
4. Appends to the agent prompt *after* the role-specific template but *before* the task description.
5. Cache: parsed AST is memoized per `repo_head_sha` for 1 hour to avoid re-reads.

Example prompt assembly:

```
{agent_role_template}
---
{project_convention_md}
---
Task: {issue.title}
Description: {issue.description}
```

## Lifecycle

```
make setup ─┐
            ├─► .agentra/AGENT.md exists? ──Yes─► daemon reads at task start
            │                                  └─► fallback: auto-extract via repo-DNA (#12)
            └── No ──► CLI offers: agentra init-agent-conventions
                           └─► AI-assisted scaffold from README + package.json + directory scan
```

## CLI Integration

```bash
# Scaffold a starter AGENT.md from current repo
agentra init-agent-conventions

# Validate an existing file against the spec
agentra validate-agent-conventions
```

## Next Steps (issue #12 follow-up)

The static `.agentra/AGENT.md` is the v0 primitive. **Repo-DNA dynamic injection** (issue #12) extends this by:

- Scanning `git log` for commit message patterns and common change clusters.
- Scanning existing PRs for review preferences.
- Scanning test files for testing conventions.
- Merging extracted signals into `repo-dna.json` → runtime prompt.

The static file serves as **override layer** — human-authoritative rules always win over auto-extracted ones.

---

*Companion issues: #11 (telemetry to measure convention effectiveness), #12 (dynamic injection).*
