# Agentra-Eval

Agentra-Eval is the benchmark harness that measures agent reliability over time.
It runs a fixed set of **golden issues** through the agent, scores each outcome,
and detects regressions vs. the previous run.

## Trace

```
operator triggers `agentra eval run`
  │
  ├─ load 20 golden cases from eval_golden_issues
  │     (5 feature + 5 bug + 5 refactor + 3 test + 2 docs)
  │
  ├─ for each case:
  │     ├─ spawn agent with repo-DNA injected
  │     ├─ run task, capture output
  │     ├─ score:
  │     │     diff similarity   × 0.40
  │     │     tests pass       × 0.35
  │     │     lint clean       × 0.25
  │     └─ record CaseResult
  │
  ├─ aggregate → RunReport { total, passed, failed, score (0-100) }
  │
  ├─ persist to eval_runs
  │
  └─ regression gate:
        if score < prev_score → CI fails + WS alert
```

## Golden dataset (v0, 20 cases)

| Category | Cases | Example |
|---|---|---|
| feature | 5 | `feat-001-cli-status-json` — confirm `agentra daemon status --output json` returns a `version` key |
| bug | 5 | `bug-004-fk-migration` — which table does migration 039's `issue_id` reference? |
| refactor | 5 | `refactor-003-long-fn` — find a Go function > 80 lines |
| test | 3 | `test-001-unit-cover` — which pure function lacks coverage? |
| docs | 2 | `docs-003-license-file` — what SPDX identifier is in LICENSE? |

Full list: [`internal/eval/seed/seeds.go`](https://github.com/agentra-ai/agentra/blob/main/server/internal/eval/seed/seeds.go)

## CLI

```bash
agentra eval run
# → JSON report with per-case score + composite 0-100
```

## HTTP API

```
POST   /api/eval/seed           — load default golden dataset (owner/admin)
GET    /api/eval/cases           — list current golden cases
POST   /api/eval/run             — trigger a benchmark run (owner/admin)
GET    /api/eval/runs/latest     — latest run result
GET    /api/eval/gate            — 503 if latest run regressed vs previous
```

## Regression detection

```sql
-- DetectEvalRegression: compares latest vs previous run
SELECT
    l.score AS latest_score,
    p.score AS prev_score,
    (p.score IS NOT NULL AND l.score < p.score) AS regressed
FROM latest_eval_run l
LEFT JOIN previous_eval_run p ON true;
```

CI integration: `GET /api/eval/gate` returns 503 when regressed → fail the build.

## Key modules

| Module | Purpose |
|---|---|
| `internal/eval/golden.go` | DNA + CaseResult + RunReport types |
| `internal/eval/seed/seeds.go` | 20-case golden dataset |
| `internal/eval/seed/answers.go` | Headless-mode canned answers |
| `internal/handler/eval.go` | HTTP API |
| `cmd/agentra/cmd_eval.go` | `agentra eval run` CLI |
| `migrations/041_eval_system.up.sql` | `eval_golden_issues` + `eval_runs` tables |
