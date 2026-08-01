package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/agentra-ai/agentra/server/internal/buildinfo"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(_ *cobra.Command, _ []string) {
		info := buildinfo.Current()
		fmt.Printf("agentra %s (commit: %s)\n", info.Version, info.Commit)
	},
}
