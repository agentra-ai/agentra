# Agentra Domain Context

Agentra is an AI-native task management platform where people and Agents work
through Issues. The product serves small AI-native teams and supports both
local and cloud Runtime execution.

## Core concepts

### Issue

An Issue is the durable unit of team responsibility. Its status describes the
human workflow (`backlog`, `todo`, `in_progress`, `in_review`, `done`,
`blocked`, or `cancelled`); it is not an execution-attempt status.

### Agent

An Agent is an assignable actor with instructions, skills, policy, and a
selected Runtime. Assigning an Agent to an Issue may enqueue a Work Item.

### Runtime

A Runtime is the execution environment selected by an Agent. A local Runtime
is served by a daemon and a Runtime Adapter; a cloud Runtime is served through
the Cloud Gateway.

### Work Item

A Work Item is one logical request for an Agent to perform work for an Issue.
It owns queueing, dispatch, retry policy, cancellation, and the final logical
outcome. The current persistence record is `agent_task_queue`.

A Work Item may have more than one Run. Retrying a Work Item must not overwrite
or merge the observations from an earlier Run.

### Run

A Run is one execution attempt of a Work Item. Its stable `run_id` is allocated
when the Work Item is dispatched, before a local daemon or Cloud Gateway starts
provisioning; execution begins when both records transition to running.
Messages, provider session checkpoints, usage, artifacts, and Trace data belong
to that Run. A Run has exactly one terminal outcome.

### Trace

A Trace is the observability projection of a Run: ordered messages, tool calls,
token usage, cost, timing, and errors. Trace state does not independently drive
the Work Item lifecycle.

### Work Graph

A Work Graph is an Issue-scoped DAG of planned work. Its nodes are currently
planning artifacts; they are not yet executable Work Items. Making them
executable requires an explicit node-to-Work-Item relationship and durable
dependency scheduling.

### Engineering Loop

An Engineering Loop is the current Plan → Develop → Review → Fix orchestration
for an Issue. It creates Work Items for its stages. The migration target is a
Work Graph stage-template adapter, not a second permanent execution lifecycle.

### Lifecycle Transition

A Lifecycle Transition is an expected-state change to a Work Item or Run. The
authoritative transition must atomically persist its state and a durable event;
realtime delivery, Trace updates, metrics, and Engineering Loop advancement are
projections of that persisted fact.

## Lifecycle invariants

- A Work Item is the logical request; a Run is one attempt.
- Every dispatched or running Work Item has exactly one active Run, and their
  statuses agree.
- Local daemon and Cloud Gateway callbacks must identify the Run they belong
  to.
- Message sequence numbers are unique within a Run, not across all retries of a
  Work Item.
- A terminal callback for an old Run cannot complete or fail a newer Run.
- Trace and metrics failures cannot change the authoritative Work Item outcome.
- Issue workflow status is not inferred directly from a Run outcome.
- Engineering Loop advancement must be idempotent and recoverable after a
  process crash.
