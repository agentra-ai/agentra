package migrations

import "testing"

func TestLatestVersion(t *testing.T) {
	version, err := LatestVersion()
	if err != nil {
		t.Fatalf("LatestVersion() error = %v", err)
	}
	if version != "049_run_identity" {
		t.Fatalf("LatestVersion() = %q, want %q", version, "049_run_identity")
	}
}
