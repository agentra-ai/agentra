package gateway

import (
	"strings"
	"testing"
)

func TestManagedContainerLabelsBindContainerToGatewayWorkspaceAndRun(t *testing.T) {
	labels := managedContainerLabels(&TaskConfig{
		TaskID: "task-1", RunID: "run-1", GatewayID: "gateway-1", WorkspaceID: "workspace-1",
	})
	want := map[string]string{
		containerLabelManaged: "true", containerLabelGateway: "gateway-1",
		containerLabelWorkspace: "workspace-1", containerLabelTask: "task-1", containerLabelRun: "run-1",
	}
	for key, value := range want {
		if labels[key] != value {
			t.Fatalf("label %q = %q, want %q", key, labels[key], value)
		}
	}
}

func TestFindContainerForRunRejectsAnotherRunForSameTask(t *testing.T) {
	_, _, err := findContainerForRun([]ManagedContainer{
		{ID: "container-old", State: "running", TaskID: "task-1", RunID: "run-old"},
	}, "task-1", "run-new")
	if err == nil || !strings.Contains(err.Error(), "another Run") {
		t.Fatalf("error = %v", err)
	}
}
