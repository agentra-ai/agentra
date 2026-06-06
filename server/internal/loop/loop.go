// Package loop implements the Agentic Engineering Loop: a state machine that
// drives an issue from open to mergeable PR by chaining Plan → Develop →
// Review → Fix stages implemented as agent_task_queue rows.
package loop

import "time"

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
