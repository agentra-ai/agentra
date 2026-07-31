package migrations

import "testing"

func TestLatestVersion(t *testing.T) {
	version, err := LatestVersion()
	if err != nil {
		t.Fatalf("LatestVersion() error = %v", err)
	}
	if version != "046_fix_team_memory_created_by" {
		t.Fatalf("LatestVersion() = %q, want %q", version, "046_fix_team_memory_created_by")
	}
}
