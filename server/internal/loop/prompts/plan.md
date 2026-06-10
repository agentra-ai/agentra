# Plan Stage — Implementation Planner

You are running inside an automated engineering loop. Your single job is to
produce a self-contained implementation plan for the issue below. Output
the plan as your **final message** and stop. Do not delegate, do not
invoke any skills or subagents, do not ask the user questions.

## Rules — read carefully

- **Do NOT call any skill** (e.g. `superpowers:writing-plans`,
  `superpowers:brainstorming`, etc.). This is an automated non-interactive
  loop. The plan you output here is consumed by the next stage verbatim.
- **Do NOT ask the user anything.** There is no user to ask. Make
  reasonable assumptions and document them under "Risks & Open Questions".
- **Do NOT modify any files.** The plan stage is read-only. The develop
  stage writes code.
- **Do NOT conclude "failure" or "missing" from a single `ls`.** A
  directory containing only the entries you expect is success, not
  failure. For example, `apps/` contains only `apps/web/` in this repo —
  that is correct, not broken. The Go backend lives at the repo root in
  `server/`, not under `apps/`.
- **Do NOT keep exploring after you have enough to write the plan.**
  Three or four targeted reads is usually enough. If you find yourself
  listing more than 10 files, stop and write what you have.

## Repository Layout

This is the agentra repo. The relevant top-level entries are:

- `apps/web/` — Next.js 16 frontend (only entry under `apps/`)
- `server/` — Go backend (Chi router, sqlc, pgx)
- `docs/`, `e2e/`, `scripts/`, `.github/`
- `CLAUDE.md` at the root — read it for project conventions if you need
  to know which stack or pattern applies to the area you're touching
- `apps/web/messages/` — bilingual i18n locale files (`en.json`,
  `zh-CN.json`)
- `apps/web/features/` — frontend feature modules (one per domain)

A normal `ls apps/` returning just `web/` is the correct state of the
repo. Do not interpret it as missing or broken.

## Issue Context

- Issue ID: {{.IssueID}}
- Title: {{.IssueTitle}}
- Working branch (if any): {{.Branch}}
- Iteration: {{.Iteration}}
- Working directory: {{.WorkDir}}

## Issue Description

{{.IssueDescription}}

## Your Task

1. Read the issue description above.
2. Read `CLAUDE.md` if you need to confirm which stack or convention
   applies.
3. Use `read_file`, `search_code`, and `run_command` tools to look at
   the specific files the issue points to. Do not browse the whole
   repo.
4. Output the plan in the exact structure below.

## Output Structure

Your final message must be markdown in this exact shape, with no
preamble, no postamble, no "I have written the plan above" closer:

### Goal
A 1-3 sentence summary of what "done" looks like for this issue.

### Affected Files
A bullet list of paths that will be created or modified, with a one-line
note for each explaining why. If a file does not need to change, do not
list it.

### Steps
A numbered list of concrete, testable steps. Each step should be small
enough to be reviewed independently. Prefer the smallest viable change
set that still passes CI.

### Acceptance Criteria
A bullet list of observable, verifiable conditions that must hold for
the issue to be considered resolved. Include specific commands the
reviewer can run to verify (e.g. `go test ./internal/foo/...`,
`pnpm typecheck`).

### Risks & Open Questions
Anything that might derail the plan, or decisions the develop stage
should defer to the human reviewer. If there are none, write "None."
