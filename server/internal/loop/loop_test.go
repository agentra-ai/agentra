package loop

import (
	"testing"
	"time"
)

func TestLoopStatusIsValid(t *testing.T) {
	valid := []Status{
		StatusPending, StatusRunning, StatusPaused,
		StatusDone, StatusFailed, StatusCancelled,
	}
	for _, s := range valid {
		if !s.IsValid() {
			t.Errorf("expected %q to be valid", s)
		}
	}
	if Status("garbage").IsValid() {
		t.Error("expected 'garbage' to be invalid")
	}
}

func TestStageIsValid(t *testing.T) {
	for _, s := range []Stage{StagePlan, StageDevelop, StageReview, StageFix} {
		if !s.IsValid() {
			t.Errorf("expected %q to be valid", s)
		}
	}
	if Stage("").IsValid() {
		t.Error("expected empty stage to be invalid")
	}
}

func TestLoopStructRoundtrip(t *testing.T) {
	now := time.Now()
	l := &Loop{
		ID:            "loop-1",
		IssueID:       "issue-1",
		WorkspaceID:   "ws-1",
		Status:        StatusRunning,
		CurrentStage:  StagePlanP(),
		Iteration:     2,
		MaxIterations: 5,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if l.Status != StatusRunning || *l.CurrentStage != StagePlan {
		t.Errorf("roundtrip mismatch: %+v", l)
	}
}

func TestFailureReasonString(t *testing.T) {
	if string(FailureMaxIterations) != "max_iterations_exceeded" {
		t.Errorf("expected canonical string, got %q", FailureMaxIterations)
	}
}

// StagePlanP is a small helper to make the test more readable.
func StagePlanP() *Stage { s := StagePlan; return &s }
