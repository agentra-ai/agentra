package migrations

import "testing"

func TestLatestVersion(t *testing.T) {
	version, err := LatestVersion()
	if err != nil {
		t.Fatalf("LatestVersion() error = %v", err)
	}
	if version != "050_active_run_lifecycle" {
		t.Fatalf("LatestVersion() = %q, want %q", version, "050_active_run_lifecycle")
	}
}
