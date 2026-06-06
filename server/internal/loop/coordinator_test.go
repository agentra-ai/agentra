package loop

import "testing"

func TestDecideNextStage_PlanToDevelop(t *testing.T) {
	c := &Coordinator{}
	l := &Loop{Status: StatusRunning, CurrentStage: sp(StagePlan), Iteration: 0, MaxIterations: 5}
	d := c.decideNextStage(l, nil)
	if d.action != "create_task" {
		t.Errorf("action = %q, want %q", d.action, "create_task")
	}
	if d.taskType != "loop_develop" {
		t.Errorf("taskType = %q, want %q", d.taskType, "loop_develop")
	}
}

func TestDecideNextStage_DevelopToReview(t *testing.T) {
	c := &Coordinator{}
	l := &Loop{Status: StatusRunning, CurrentStage: sp(StageDevelop), Iteration: 0, MaxIterations: 5}
	result := &TaskResult{PRURL: "https://github.com/agentra-ai/agentra/pull/42", PRNumber: pint(42), BranchName: "loop/issue-1"}
	d := c.decideNextStage(l, result)
	if d.action != "create_task" {
		t.Errorf("action = %q, want %q", d.action, "create_task")
	}
	if d.taskType != "loop_review" {
		t.Errorf("taskType = %q, want %q", d.taskType, "loop_review")
	}
}

func TestDecideNextStage_ReviewApprovedToComplete(t *testing.T) {
	c := &Coordinator{}
	l := &Loop{Status: StatusRunning, CurrentStage: sp(StageReview), Iteration: 0, MaxIterations: 5}
	result := &TaskResult{
		ReviewApproved: bptr(true),
		PRURL:          "https://github.com/agentra-ai/agentra/pull/42",
		PRNumber:       pint(42),
		BranchName:     "loop/issue-1",
	}
	d := c.decideNextStage(l, result)
	if d.action != "complete" {
		t.Errorf("action = %q, want %q", d.action, "complete")
	}
	if d.prURL != "https://github.com/agentra-ai/agentra/pull/42" {
		t.Errorf("prURL = %q", d.prURL)
	}
	if d.prNumber != 42 {
		t.Errorf("prNumber = %d, want 42", d.prNumber)
	}
	if d.branchName != "loop/issue-1" {
		t.Errorf("branchName = %q", d.branchName)
	}
}

func TestDecideNextStage_ReviewRejectedToFix(t *testing.T) {
	c := &Coordinator{}
	l := &Loop{Status: StatusRunning, CurrentStage: sp(StageReview), Iteration: 1, MaxIterations: 5}
	result := &TaskResult{ReviewApproved: bptr(false), ReviewIssues: "tests failing"}
	d := c.decideNextStage(l, result)
	if d.action != "create_task" {
		t.Errorf("action = %q, want %q", d.action, "create_task")
	}
	if d.taskType != "loop_fix" {
		t.Errorf("taskType = %q, want %q", d.taskType, "loop_fix")
	}
	if d.iterationBump != 1 {
		t.Errorf("iterationBump = %d, want 1", d.iterationBump)
	}
}

func TestDecideNextStage_ReviewRejectedAtMaxIterationsToFail(t *testing.T) {
	c := &Coordinator{}
	l := &Loop{Status: StatusRunning, CurrentStage: sp(StageReview), Iteration: 5, MaxIterations: 5}
	result := &TaskResult{ReviewApproved: bptr(false), ReviewIssues: "still broken"}
	d := c.decideNextStage(l, result)
	if d.action != "fail" {
		t.Errorf("action = %q, want %q", d.action, "fail")
	}
	if d.reason != FailureMaxIterations {
		t.Errorf("reason = %q, want %q", d.reason, FailureMaxIterations)
	}
}

func TestDecideNextStage_FixToReview(t *testing.T) {
	c := &Coordinator{}
	l := &Loop{Status: StatusRunning, CurrentStage: sp(StageFix), Iteration: 2, MaxIterations: 5}
	result := &TaskResult{}
	d := c.decideNextStage(l, result)
	if d.action != "create_task" {
		t.Errorf("action = %q, want %q", d.action, "create_task")
	}
	if d.taskType != "loop_review" {
		t.Errorf("taskType = %q, want %q", d.taskType, "loop_review")
	}
}

func TestDecideNextStage_PausedIsNoop(t *testing.T) {
	c := &Coordinator{}
	l := &Loop{Status: StatusPaused, CurrentStage: sp(StageReview), Iteration: 1, MaxIterations: 5}
	result := &TaskResult{ReviewApproved: bptr(false)}
	d := c.decideNextStage(l, result)
	if d.action != "noop" {
		t.Errorf("action = %q, want %q", d.action, "noop")
	}
}

func sp(s Stage) *Stage { return &s }
func bptr(b bool) *bool { return &b }
func pint(i int) *int   { return &i }
