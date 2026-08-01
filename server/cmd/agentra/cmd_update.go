package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/agentra-ai/agentra/server/internal/buildinfo"
	"github.com/agentra-ai/agentra/server/internal/cli"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update agentra to the latest version",
	RunE:  runUpdate,
}

func runUpdate(_ *cobra.Command, _ []string) error {
	info := buildinfo.Current()
	fmt.Fprintf(os.Stderr, "Current version: %s (commit: %s)\n", info.Version, info.Commit)
	if cli.IsBrewInstall() {
		fmt.Fprintln(os.Stderr, "This binary is managed by Homebrew. Run 'brew upgrade agentra' to update it.")
		return nil
	}

	// Check latest version from GitHub.
	latest, err := cli.FetchLatestRelease()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not check latest version: %v\n", err)
	} else {
		latestVer := strings.TrimPrefix(latest.TagName, "v")
		currentVer := info.Version
		if currentVer == latestVer {
			fmt.Fprintln(os.Stderr, "Already up to date.")
			return nil
		}
		fmt.Fprintf(os.Stderr, "Latest version:  %s\n\n", latest.TagName)
	}

	if latest == nil {
		return fmt.Errorf("could not determine latest version; check https://github.com/agentra-ai/agentra/releases/latest")
	}
	targetVersion := latest.TagName
	fmt.Fprintf(os.Stderr, "Downloading %s from GitHub Releases...\n", targetVersion)
	output, err := cli.UpdateViaDownload(targetVersion)
	if err != nil {
		return fmt.Errorf("update failed: %w", err)
	}
	fmt.Fprintf(os.Stderr, "%s\nUpdate complete.\n", output)
	return nil
}
