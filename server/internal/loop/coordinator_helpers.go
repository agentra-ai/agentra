package loop

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/agentra-ai/agentra/server/internal/events"
	dbpkg "github.com/agentra-ai/agentra/server/pkg/db/generated"
)

// newTaskID returns a fresh UUID string for use as a loop-relative identifier.
func newTaskID() string { return uuid.NewString() }

// nowUTC returns the current wall-clock time in UTC. Used when stamping
// completed_at on terminal loop transitions.
func nowUTC() time.Time { return time.Now().UTC() }

// stageFromString maps a loop task_type (e.g. "loop_develop") back to the
// domain *Stage. Returns nil for unknown values so callers can treat them
// as "do not set current_stage" rather than panicking.
func stageFromString(s string) *Stage {
	switch s {
	case taskTypePlan:
		v := StagePlan
		return &v
	case taskTypeDevelop:
		v := StageDevelop
		return &v
	case taskTypeReview:
		v := StageReview
		return &v
	case taskTypeFix:
		v := StageFix
		return &v
	}
	return nil
}

// taskTypeForStage is the inverse of stageFromString: given a domain Stage,
// it returns the task_type string the agent queue uses to enqueue that
// stage's work. Returns "" for unknown stages so callers can fail loudly
// (or skip) rather than enqueueing a malformed task_type.
func taskTypeForStage(s Stage) string {
	switch s {
	case StagePlan:
		return taskTypePlan
	case StageDevelop:
		return taskTypeDevelop
	case StageReview:
		return taskTypeReview
	case StageFix:
		return taskTypeFix
	}
	return ""
}

// parseTaskResult unmarshals the raw task_run output into a TaskResult. Bad
// JSON or empty input returns nil — the coordinator must treat a missing
// result as a non-event (no review verdict) rather than a crash.
func parseTaskResult(raw []byte) *TaskResult {
	if len(raw) == 0 {
		return nil
	}
	var r TaskResult
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil
	}
	return &r
}

// eventTaskID extracts the task_id string from an event payload, which the
// bus serializes as map[string]any.
func eventTaskID(e events.Event) (string, bool) {
	m, ok := e.Payload.(map[string]any)
	if !ok {
		return "", false
	}
	id, ok := m["task_id"].(string)
	return id, ok
}

// latestTaskResult returns the parsed TaskResult of the most recent task_run
// for a task, or nil if no run exists or its output is unparseable. ListTaskRunsByTask
// is ordered by created_at DESC, so index 0 is the latest run.
func latestTaskResult(ctx context.Context, q *dbpkg.Queries, taskID pgtype.UUID) *TaskResult {
	runs, err := q.ListTaskRunsByTask(ctx, taskID)
	if err != nil || len(runs) == 0 {
		return nil
	}
	latest := runs[0]
	if !latest.Output.Valid {
		return nil
	}
	return parseTaskResult([]byte(latest.Output.String))
}
