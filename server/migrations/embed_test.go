package migrations

import "testing"

func TestLatestVersion(t *testing.T) {
	version, err := LatestVersion()
	if err != nil {
		t.Fatalf("LatestVersion() error = %v", err)
	}
	if version != "052_cloud_dispatch_delivery" {
		t.Fatalf("LatestVersion() = %q, want %q", version, "052_cloud_dispatch_delivery")
	}
}
