package traces

import (
	"time"
)

type Summarizer struct{}

func (s *Summarizer) Summarize(steps []TraceStep) *TaskRunSummary {
	summary := &TaskRunSummary{
		ToolUsage:  make(map[string]int),
		KeyActions: []string{},
	}

	var firstTimestamp, lastTimestamp time.Time
	for i, step := range steps {
		if i == 0 {
			firstTimestamp, _ = time.Parse(time.RFC3339, step.Timestamp)
		}
		lastTimestamp, _ = time.Parse(time.RFC3339, step.Timestamp)

		summary.TotalTokens += step.TokensUsed
		summary.DurationMs += int64(step.DurationMs)

		if step.Tool != "" {
			summary.ToolUsage[step.Tool]++
		}

		if step.Action == "tool_call" {
			summary.KeyActions = append(summary.KeyActions, step.Tool)
		}
	}

	summary.DurationMs = lastTimestamp.Sub(firstTimestamp).Milliseconds()
	return summary
}