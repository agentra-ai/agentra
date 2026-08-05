package gateway

import "testing"

func TestReserveTaskMakesRunDispatchIdempotent(t *testing.T) {
	g := &Gateway{}
	first := &RunningTask{TaskID: "task-1", RunID: "run-1"}
	got, reserved := g.reserveTask(first)
	if !reserved || got != first {
		t.Fatalf("first reservation = reserved:%v task:%p, want true %p", reserved, got, first)
	}

	duplicate := &RunningTask{TaskID: "task-1", RunID: "run-1"}
	got, reserved = g.reserveTask(duplicate)
	if reserved || got != first {
		t.Fatalf("duplicate reservation = reserved:%v task:%p, want false %p", reserved, got, first)
	}

	newRun := &RunningTask{TaskID: "task-1", RunID: "run-2"}
	got, reserved = g.reserveTask(newRun)
	if reserved || got.RunID != "run-1" {
		t.Fatalf("overlapping Run reservation = reserved:%v run:%q, want false run-1", reserved, got.RunID)
	}
}
