package buildinfo

import "testing"

func TestCurrentNormalizesInjectedBuildMetadata(t *testing.T) {
	originalVersion, originalCommit := Version, Commit
	t.Cleanup(func() {
		Version = originalVersion
		Commit = originalCommit
	})

	Version = " v0.6.0 "
	Commit = " abc123 "
	got := Current()
	if got.Version != "0.6.0" || got.Commit != "abc123" {
		t.Fatalf("Current() = %+v", got)
	}

	Version = ""
	Commit = ""
	got = Current()
	if got.Version != "dev" || got.Commit != "unknown" {
		t.Fatalf("Current() fallback = %+v", got)
	}
}
