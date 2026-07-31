package migrations

import "testing"

func TestLatestVersion(t *testing.T) {
	version, err := LatestVersion()
	if err != nil {
		t.Fatalf("LatestVersion() error = %v", err)
	}
	if version != "048_execution_trace_lifecycle_fks" {
		t.Fatalf("LatestVersion() = %q, want %q", version, "048_execution_trace_lifecycle_fks")
	}
}
