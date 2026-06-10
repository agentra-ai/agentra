package loop

import (
	"encoding/json"
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

func TestStageAgent_NoOverride_NoDefault(t *testing.T) {
	l := &Loop{} // no AgentID, no Config
	if got := l.StageAgent(StageDevelop); got != nil {
		t.Fatalf("expected nil agent id, got %q", *got)
	}
}

func TestStageAgent_FallsBackToAgentID(t *testing.T) {
	defaultID := "00000000-0000-0000-0000-000000000001"
	l := &Loop{AgentID: &defaultID}
	for _, s := range []Stage{StagePlan, StageDevelop, StageReview, StageFix} {
		got := l.StageAgent(s)
		if got == nil {
			t.Errorf("expected fallback to %q for %s, got nil", defaultID, s)
			continue
		}
		if *got != defaultID {
			t.Errorf("expected fallback %q for %s, got %q", defaultID, s, *got)
		}
	}
}

func TestStageAgent_PerStageOverrideWins(t *testing.T) {
	defaultID := "00000000-0000-0000-0000-000000000001"
	developID := "00000000-0000-0000-0000-000000000002"
	reviewID := "00000000-0000-0000-0000-000000000003"
	cfg, err := json.Marshal(LoopConfig{StageAgents: map[string]string{
		"develop": developID,
		"review":  reviewID,
	}})
	if err != nil {
		t.Fatal(err)
	}
	l := &Loop{AgentID: &defaultID, Config: cfg}

	if got := l.StageAgent(StagePlan); got == nil || *got != defaultID {
		t.Errorf("plan should fall through to default, got %v", got)
	}
	if got := l.StageAgent(StageDevelop); got == nil || *got != developID {
		t.Errorf("develop should use override, got %v", got)
	}
	if got := l.StageAgent(StageReview); got == nil || *got != reviewID {
		t.Errorf("review should use override, got %v", got)
	}
	if got := l.StageAgent(StageFix); got == nil || *got != defaultID {
		t.Errorf("fix (no override) should fall through, got %v", got)
	}
}

func TestStageAgent_CorruptConfig_FallsBack(t *testing.T) {
	// The state machine must not break because somebody hand-edited a
	// loops.config row. ParseConfig is forgiving; StageAgent sees an
	// empty map and returns AgentID.
	defaultID := "00000000-0000-0000-0000-000000000001"
	l := &Loop{AgentID: &defaultID, Config: []byte("not json{")}
	if got := l.StageAgent(StagePlan); got == nil || *got != defaultID {
		t.Fatalf("expected fallback on corrupt config, got %v", got)
	}
}
