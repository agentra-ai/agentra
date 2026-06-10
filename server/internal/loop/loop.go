// Package loop implements the Agentic Engineering Loop: a state machine that
// drives an issue from open to mergeable PR by chaining Plan → Develop →
// Review → Fix stages implemented as agent_task_queue rows.
package loop

import (
	"encoding/json"
	"time"
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusPaused    Status = "paused"
	StatusDone      Status = "done"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
)

func (s Status) IsValid() bool {
	switch s {
	case StatusPending, StatusRunning, StatusPaused,
		StatusDone, StatusFailed, StatusCancelled:
		return true
	}
	return false
}

type Stage string

const (
	StagePlan    Stage = "plan"
	StageDevelop Stage = "develop"
	StageReview  Stage = "review"
	StageFix     Stage = "fix"
)

func (s Stage) IsValid() bool {
	switch s {
	case StagePlan, StageDevelop, StageReview, StageFix:
		return true
	}
	return false
}

type FailureReason string

const (
	FailureMaxIterations   FailureReason = "max_iterations_exceeded"
	FailureLoopTimeout     FailureReason = "loop_timeout"
	FailureStageTimeout    FailureReason = "stage_timeout"
	FailurePRCreateFailed  FailureReason = "pr_create_failed"
	FailureContextExceeded FailureReason = "context_exceeded"
	FailureContentFilter   FailureReason = "content_filter"
	FailureUnrecoverable   FailureReason = "unrecoverable_error"
)

// Loop is the top-level state record for one engineering cycle on an issue.
type Loop struct {
	ID            string     `json:"id"`
	IssueID       string     `json:"issue_id"`
	WorkspaceID   string     `json:"workspace_id"`
	Status        Status     `json:"status"`
	CurrentStage  *Stage     `json:"current_stage,omitempty"`
	Iteration     int        `json:"iteration"`
	MaxIterations int        `json:"max_iterations"`
	PRURL         *string    `json:"pr_url,omitempty"`
	PRNumber      *int       `json:"pr_number,omitempty"`
	BranchName    *string    `json:"branch_name,omitempty"`
	AgentID       *string    `json:"agent_id,omitempty"`
	Config        []byte     `json:"config"`
	FailureReason *string    `json:"failure_reason,omitempty"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// LoopConfig is the typed shape we expect inside the loops.config JSONB
// column. It is intentionally a subset: unknown keys round-trip untouched
// because we only marshal it back on writes, and reads go through the raw
// Loop.Config bytes. StageAgents is a stage-name → agent-id map; entries
// override Loop.AgentID for that one stage. Missing keys fall through to
// Loop.AgentID.
type LoopConfig struct {
	StageAgents map[string]string `json:"stage_agents,omitempty"`
}

// ParseConfig decodes Loop.Config into a typed LoopConfig. A nil/empty
// blob or any decode error returns the zero value — the loop is still
// usable, callers just get fallback behavior (no per-stage overrides).
// This is deliberately forgiving: corrupt config should not break the
// state machine.
func (l *Loop) ParseConfig() LoopConfig {
	var cfg LoopConfig
	if len(l.Config) == 0 {
		return cfg
	}
	_ = json.Unmarshal(l.Config, &cfg)
	return cfg
}

// StageAgent returns the agent id to use for a particular stage, honoring
// any per-stage override in Loop.Config.stage_agents. Returns nil if no
// override applies AND Loop.AgentID is nil — the coordinator treats that
// as "use no agent" and the CreateAgentTask call will fail the NOT NULL
// runtime_id check, which is the right loud failure mode.
func (l *Loop) StageAgent(stage Stage) *string {
	cfg := l.ParseConfig()
	if cfg.StageAgents != nil {
		if id, ok := cfg.StageAgents[string(stage)]; ok && id != "" {
			return &id
		}
	}
	return l.AgentID
}
