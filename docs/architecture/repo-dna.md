# Repo-DNA

Repositoy DNA is a structured, JSON-serialisable signal map extracted from any
Git repo. Agentra's daemon reads it at task-start and injects it into the
agent's `CLAUDE.md` / `AGENTS.md` so the agent inherits the codebase's conventions
**without prompting from the human owner**.

## Trace

```
daemon registers runtime
  │
  ├─ scanGitLog(repoRoot):
  │     ├─ conventional-commit prefix distribution (feat%, fix%, …)
  │     ├─ active scopes (landing, agent, …)
  │     ├─ body rule         ("what + why")
  │     └─ footer patterns   (issue ref, co-authored-by)
  │
  ├─ scanFiles(repoRoot):
  │     ├─ stack         (Go + Chi, Next.js, PostgreSQL, Docker Compose)
  │     ├─ test coverage (vitest, go test, playwright)
  │     ├─ dir layout    (feature-first, 19 dirs)
  │     └─ conventions   (derivded rules list)
  │
  └─ runtime_config.InjectRuntimeConfig:
        writes {workDir}/CLAUDE.md  (claude)
        writes {workDir}/AGENTS.md  (codex, opencode)
        appends "## Repo-DNA (auto-extracted)" section
              — JSON blob + human-readable highlights
```

## JSON Schema

```json
{
  "commit_style": {
    "prefix_distribution": { "feat": 0.45, "fix": 0.30, "refactor": 0.10, "test": 0.08, "docs": 0.04, "chore": 0.03 },
    "scopes_active": ["landing", "agent", "loops", "auth", "realtime", "daemon", "cli", "metrics", "handler", "server", "middleware", "ci"],
    "body_rule": "imperative: what + why, not how",
    "footer_patterns": ["issue/ticket ref", "co-authored-by"]
  },
  "stack": {
    "language_primary": "Go",
    "language_secondary": "TypeScript",
    "frontend_framework": "Next.js",
    "backend_framework": "Chi",
    "db": "PostgreSQL + pgvector",
    "deployment": "Docker Compose"
  },
  "test_coverage": {
    "frontend": { "present": true, "runner": "vitest" },
    "backend":  { "present": true, "runner": "go test" },
    "e2e":      { "present": true, "runner": "playwright" }
  },
  "directory_layout": {
    "style": "feature-first",
    "feature_dirs": ["agents", "auth", "editor", "github", "inbox", "issues",
                     "landing", "loops", "memory", "modals", "my-issues",
                     "navigation", "realtime", "runtimes", "settings", "skills",
                     "taskgraph", "traces", "workspace"]
  },
  "conventions": [
    "本仓库 commit 风格规整 (type(scope): imperative) -- 请遵循",
    "前端 import 用 @/ alias,禁止 feature↔feature 直接引用",
    "Zustand 管理 client state; React Context 仅用于 WS lifecycle",
    ...
  ]
}
```

## Key module

[`pkg/codex/dna/extractor.go`](https://github.com/agentra-ai/agentra/blob/main/server/pkg/codex/dna/extractor.go)

```
Import point: runtime_config.InjectRuntimeConfig → dna.Extract(ctx.WorkspaceRoot)
Trigger:      every time the daemon starts a new agent task
Cache:        on-disk at {workspaceRoot}/.agentra/repo-dna.json (future)
```

## Use cases

| Who | What they get |
|---|---|
| Solo dev | Agent automatically follows repo conventions (commit style, test framework, import aliases) without a `.agentra/AGENT.md`. |
| Team | Specialist templates (Frontend / Backend / Security / …) get enriched with per-repo DNA on every task, no manual sync. |
| Operator | `POST /api/repos/:id/extract-dna` (future) re-scans and bumps the persisted DNA. |
| Agentra-Eval | Golden cases can compare DNA-injected vs. plain runs to measure the convention boost. |
