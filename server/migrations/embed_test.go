package migrations

import "testing"

func TestLatestVersion(t *testing.T) {
	version, err := LatestVersion()
	if err != nil {
		t.Fatalf("LatestVersion() error = %v", err)
	}
	if version != "051_lifecycle_outbox" {
		t.Fatalf("LatestVersion() = %q, want %q", version, "051_lifecycle_outbox")
	}
}
