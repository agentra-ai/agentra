// Package prompts provides embed.FS access to the per-stage prompt templates
// for the Agentic Engineering Loop. Templates live as siblings to this file
// and are compiled into the binary via go:embed so the loader has no
// dependency on the current working directory.
package prompts

import "embed"

// FS holds the per-stage prompt templates. The loader in
// server/internal/loop/stages/prompts.go reads files from this FS.
//
//go:embed *.md
var FS embed.FS
