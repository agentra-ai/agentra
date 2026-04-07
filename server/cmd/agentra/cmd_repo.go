package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/agentra-ai/agentra/server/internal/cli"
)

var repoCmd = &cobra.Command{
	Use:   "repo",
	Short: "Manage repositories",
}

var repoCheckoutCmd = &cobra.Command{
	Use:   "checkout <url>",
	Short: "Check out a repository into the working directory",
	Long:  "Creates a git worktree from the daemon's bare clone cache. Used by agents to check out repos on demand.",
	Args:  cobra.ExactArgs(1),
	RunE:  runRepoCheckout,
}

func init() {
	repoCmd.AddCommand(repoCheckoutCmd)
}

func runRepoCheckout(cmd *cobra.Command, args []string) error {
	repoURL := args[0]

	daemonBaseURL := strings.TrimSpace(os.Getenv("AGENTRA_DAEMON_BASE_URL"))
	if daemonBaseURL == "" {
		daemonPort := os.Getenv("AGENTRA_DAEMON_PORT")
		if daemonPort == "" {
			return fmt.Errorf("AGENTRA_DAEMON_BASE_URL or AGENTRA_DAEMON_PORT must be set (this command is intended to be run by an agent inside a daemon task)")
		}
		daemonBaseURL = cli.ResolveLocalDaemonBaseURL(daemonPort)
	}

	workspaceID := os.Getenv("AGENTRA_WORKSPACE_ID")
	agentName := os.Getenv("AGENTRA_AGENT_NAME")
	taskID := os.Getenv("AGENTRA_TASK_ID")

	// Use current working directory as the checkout target.
	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	reqBody := map[string]string{
		"url":          repoURL,
		"workspace_id": workspaceID,
		"workdir":      workDir,
		"agent_name":   agentName,
		"task_id":      taskID,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("encode request: %w", err)
	}

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Post(
		fmt.Sprintf("%s/repo/checkout", strings.TrimRight(daemonBaseURL, "/")),
		"application/json",
		bytes.NewReader(data),
	)
	if err != nil {
		return fmt.Errorf("connect to daemon: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("checkout failed: %s", string(body))
	}

	var result struct {
		Path       string `json:"path"`
		BranchName string `json:"branch_name"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}

	fmt.Fprintf(os.Stdout, "%s\n", result.Path)
	fmt.Fprintf(os.Stderr, "Checked out %s → %s (branch: %s)\n", repoURL, result.Path, result.BranchName)

	return nil
}
