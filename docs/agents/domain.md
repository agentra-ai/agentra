# Domain Docs

Single-context layout: one `CONTEXT.md` at the repo root, with ADRs in `docs/adr/`.

## Consumer Rules

The following skills read domain context:

- `improve-codebase-architecture` — reads `CONTEXT.md` for domain language and `docs/adr/` for past decisions
- `diagnose` — reads `CONTEXT.md` for background context
- `tdd` — reads `CONTEXT.md` to understand domain concepts before writing tests

## File Locations

| File | Purpose |
|---|---|
| `CONTEXT.md` | Project domain language, key concepts, and background |
| `docs/adr/` | Architectural decision records (ADRs) |

## Writing Context

When writing or updating `CONTEXT.md`:
- Keep it focused on domain concepts, not implementation details
- Include the project's core entities and relationships
- Note any important architectural constraints or patterns

## ADR Format

Use standard ADR format: Title, Status, Context, Decision, Consequences.
Store in `docs/adr/NNNN-title.md` with ISO date in filename.
