package loop

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
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

// parseTaskResult unmarshals the raw task_run output into a TaskResult. It is
// forgiving of the common LLM habit of wrapping JSON in markdown fences or
// surrounding it with a sentence of preamble. The fast path is a direct
// json.Unmarshal; the slow path scans for the first balanced JSON object and
// unmarshals that substring. Returns nil only when no parseable JSON object
// can be found, so the coordinator can treat a missing result as a non-event
// (no review verdict) rather than a crash.
func parseTaskResult(raw []byte) *TaskResult {
	if len(raw) == 0 {
		return nil
	}
	var r TaskResult
	if err := json.Unmarshal(raw, &r); err == nil {
		return &r
	}
	// Fall back: extract the first balanced JSON object from the text,
	// respecting string boundaries so braces inside string fields don't
	// confuse the depth counter.
	s := string(raw)
	start := strings.Index(s, "{")
	if start < 0 {
		return nil
	}
	end := -1
	depth := 0
	inString := false
	escape := false
	for i := start; i < len(s); i++ {
		c := s[i]
		if escape {
			escape = false
			continue
		}
		if c == '\\' && inString {
			escape = true
			continue
		}
		if c == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		if c == '{' {
			depth++
		} else if c == '}' {
			depth--
			if depth == 0 {
				end = i
				break
			}
		}
	}
	if end < 0 {
		return nil
	}
	if err := json.Unmarshal([]byte(s[start:end+1]), &r); err != nil {
		return nil
	}
	return &r
}
