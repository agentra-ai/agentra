package buildinfo

import "strings"

// These values are overridden by release build ldflags. Direct go run builds
// remain explicitly identifiable as development binaries.
var (
	Version = "dev"
	Commit  = "unknown"
)

type Info struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
}

func Current() Info {
	version := strings.TrimSpace(Version)
	version = strings.TrimPrefix(version, "v")
	if version == "" {
		version = "dev"
	}
	commit := strings.TrimSpace(Commit)
	if commit == "" {
		commit = "unknown"
	}
	return Info{Version: version, Commit: commit}
}
