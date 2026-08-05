package gateway

import (
	"context"
	"errors"
	"io"
	"testing"
)

type fakeContainerRuntime struct {
	container   ManagedContainer
	found       bool
	findErr     error
	createErr   error
	findCalls   int
	createCalls int
}

func (f *fakeContainerRuntime) CreateContainer(context.Context, *TaskConfig) (string, error) {
	f.createCalls++
	if f.createErr != nil {
		// Model Docker accepting Create while the response is lost. The next
		// discovery must adopt this identity instead of issuing Create again.
		f.found = true
		return "", f.createErr
	}
	return f.container.ID, nil
}

func (f *fakeContainerRuntime) ListManagedContainers(context.Context, string, string) ([]ManagedContainer, error) {
	if f.found {
		return []ManagedContainer{f.container}, nil
	}
	return nil, nil
}

func (f *fakeContainerRuntime) FindContainerForRun(context.Context, string, string, string, string) (ManagedContainer, bool, error) {
	f.findCalls++
	return f.container, f.found, f.findErr
}

func (*fakeContainerRuntime) StartContainer(context.Context, string) error       { return nil }
func (*fakeContainerRuntime) WaitContainer(context.Context, string) (int, error) { return 0, nil }
func (*fakeContainerRuntime) StreamContainerLogs(context.Context, string, io.Writer, io.Writer) error {
	return nil
}
func (*fakeContainerRuntime) DestroyContainer(context.Context, string) error { return nil }

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

func TestAcquireContainerAdoptsExistingRunIdentity(t *testing.T) {
	runtime := &fakeContainerRuntime{
		container: ManagedContainer{ID: "container-1", State: "running", TaskID: "task-1", RunID: "run-1"},
		found:     true,
	}
	g := &Gateway{cfg: Config{MaxRetries: 1}, logger: discardLogger(), containerMgr: runtime}
	got, err := g.acquireContainerWithRetry(context.Background(), &TaskConfig{
		TaskID: "task-1", RunID: "run-1", GatewayID: "gateway-1", WorkspaceID: "workspace-1",
	}, "task-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "container-1" || runtime.createCalls != 0 {
		t.Fatalf("acquired = %+v create calls = %d", got, runtime.createCalls)
	}
}

func TestAcquireContainerAdoptsAfterAmbiguousCreateResponse(t *testing.T) {
	runtime := &fakeContainerRuntime{
		container: ManagedContainer{ID: "container-1", State: "created", TaskID: "task-1", RunID: "run-1"},
		createErr: errors.New("response lost"),
	}
	g := &Gateway{cfg: Config{MaxRetries: 2}, logger: discardLogger(), containerMgr: runtime}
	got, err := g.acquireContainerWithRetry(context.Background(), &TaskConfig{
		TaskID: "task-1", RunID: "run-1", GatewayID: "gateway-1", WorkspaceID: "workspace-1",
	}, "task-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "container-1" || runtime.createCalls != 1 || runtime.findCalls != 2 {
		t.Fatalf("acquired = %+v find calls = %d create calls = %d", got, runtime.findCalls, runtime.createCalls)
	}
}
