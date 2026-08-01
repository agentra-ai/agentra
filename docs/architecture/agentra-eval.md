# Agentra-Eval: current contract and target architecture

Agentra-Eval is an experimental dataset contract, not a benchmark or release
quality gate yet. The current repository contains 25 proposed cases and a CLI
validator that proves each expected-test regular expression is valid and can
match its deterministic fixture string.

The validator does **not** execute an agent, inspect a code diff, run repository
tests, persist a benchmark result, compare releases, or expose an HTTP API.

## Current supported surface

```bash
agentra eval validate-fixtures
```

The JSON response identifies itself explicitly:

```json
{
  "mode": "fixture_contract",
  "quality_gate": false,
  "valid": true,
  "total": 25,
  "valid_patterns": 25,
  "fixture_matches": 25,
  "failures": 0
}
```

`valid: true` means only that the static fixture contract is internally
consistent. It is not an agent quality score.

```text
FixtureCases (25 proposed prompts + regex patterns)
                         │
                         ▼
            compile every expected-test regex
                         │
                         ▼
       match it against one deterministic fixture answer
                         │
                         ▼
 fixture_contract report with quality_gate=false
```

The cases are evenly split across five categories:

| Category | Cases | Example |
|---|---:|---|
| feature | 5 | Confirm daemon status JSON contains a version key |
| bug | 5 | Inspect a migration foreign key |
| refactor | 5 | Identify an oversized Go function |
| test | 5 | Inspect a test or fixture convention |
| docs | 5 | Check repository documentation structure |

## Deliberately unsupported

There is no `/api/eval` route. An earlier unmounted handler returned canned
answers, persisted a placeholder score, and was described as a working API; it
was removed because an unreachable fake benchmark is worse than an explicit
experimental boundary.

Migration 041 still defines historical experimental `eval_golden_issues` and
`eval_runs` tables. They are not mounted as a product surface and do not make
evaluation an API capability. Their final schema will be decided with the real
execution ledger rather than treated as a compatibility contract.

## Requirements for a real benchmark

Agentra-Eval can become a quality gate only after all of these are implemented:

1. Every case runs through a selected runtime adapter in a clean, pinned
   repository fixture.
2. The outcome records commit/diff, tests, lint, duration, token usage, cost,
   runtime identity, model, and failure taxonomy.
3. Scoring is derived from observable outcomes, not canned text.
4. Runs and case attempts are durable, workspace-scoped, idempotent, and
   replayable from an execution ledger.
5. Regression thresholds use a versioned baseline and have enough repeated
   samples to distinguish regressions from model variance.
6. Authenticated API and Web surfaces have tenant-isolation and end-to-end
   regression coverage.

Until then, the capability remains `experimental` in
[`docs/capabilities.json`](../capabilities.json).

## Key modules

| Module | Purpose |
|---|---|
| `server/internal/eval/seed/seeds.go` | 25 proposed fixture cases |
| `server/internal/eval/seed/answers.go` | Deterministic regex-match fixtures |
| `server/cmd/agentra/cmd_eval.go` | Truthful fixture-contract validator |
| `server/cmd/agentra/cmd_eval_test.go` | Prevents fake benchmark/gate claims from returning |
