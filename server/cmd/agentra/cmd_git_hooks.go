package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var gitHooksInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install git hooks for Agentra",
	Long: `Install git hooks that automatically link commits and PRs to Agentra issues.

Detects issue ID from branch name (e.g., agentra/AGENTRA-123/feature → AGENTRA-123).

Hooks installed:
  prepare-commit-msg  — Prepends [AGENTRA-123] to commit messages
  post-commit         — Links commit SHA to the issue
  post-merge         — Auto-transitions issue to Done on PR merge

Requires:
  - Git repository
  - AGENTRA_API_URL environment variable (defaults to http://localhost:8080)
  - AGENTRA_API_TOKEN for authentication (optional for link-commit)

Usage:
  agentra git hooks install
  agentra git hooks install --dir /path/to/repo
`,
	RunE: runGitHooksInstall,
}

var hooksInstallDir string

func init() {
	gitHooksInstallCmd.Flags().StringVar(&hooksInstallDir, "dir", ".", "Repository directory (default: current directory)")
	gitHooksCmd.AddCommand(gitHooksInstallCmd)
}

func runGitHooksInstall(cmd *cobra.Command, args []string) error {
	repoDir, err := filepath.Abs(hooksInstallDir)
	if err != nil {
		return fmt.Errorf("invalid directory: %w", err)
	}

	hooksDir := filepath.Join(repoDir, ".git", "hooks")

	// Check if .git directory exists
	if _, err := os.Stat(filepath.Join(repoDir, ".git")); os.IsNotExist(err) {
		return fmt.Errorf("not a git repository: %s", repoDir)
	}

	// Create hooks directory if it doesn't exist
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return fmt.Errorf("create hooks dir: %w", err)
	}

	// Hook scripts to install
	hooks := []struct {
		name    string
		content string
	}{
		{
			name: "prepare-commit-msg",
			content: `#!/usr/bin/env node
// prepare-commit-msg: auto-prepend [AGENTRA-123] to commit message

const { execSync } = require('child_process');

try {
  const branch = execSync('git branch --show-current', { encoding: 'utf8' }).trim();
  const match = branch.match(/(AGENTRA-\d+)/i);
  if (!match) process.exit(0);

  const taskId = match[1].toUpperCase();
  const msgFile = process.argv[2];
  const commitSource = process.argv[3]; // merge|squash|commit|template

  if (['merge', 'squash'].includes(commitSource)) process.exit(0);

  const fs = require('fs');
  let msg = fs.readFileSync(msgFile, 'utf8');
  if (msg.startsWith('[' + taskId + ']')) process.exit(0);

  fs.writeFileSync(msgFile, '[' + taskId + '] ' + msg);
} catch (e) {
  process.exit(0); // never block git
}
`,
		},
		{
			name: "post-commit",
			content: `#!/usr/bin/env node
// post-commit: link commit SHA to Agentra issue

const { execSync } = require('child_process');

try {
  const branch = execSync('git branch --show-current', { encoding: 'utf8' }).trim();
  const match = branch.match(/(AGENTRA-\d+)/i);
  if (!match) process.exit(0);

  const taskId = match[1].toUpperCase();
  const logOutput = execSync('git log -1 --format="%H %s" HEAD', { encoding: 'utf8' }).trim();
  const spaceIdx = logOutput.indexOf(' ');
  const sha = logOutput.substring(0, spaceIdx);
  const message = logOutput.substring(spaceIdx + 1);

  const apiUrl = process.env.AGENTRA_API_URL || 'http://localhost:8080';
  const token = process.env.AGENTRA_API_TOKEN;

  const body = JSON.stringify({ issueId: taskId, sha, message, branch });

  const headers = { 'Content-Type': 'application/json' };
  if (token) headers['Authorization'] = 'Bearer ' + token;

  fetch(apiUrl + '/api/git/link-commit', {
    method: 'POST',
    headers: headers,
    body: body,
  }).catch(() => {}); // fire-and-forget, never block
} catch (e) {
  process.exit(0);
}
`,
		},
		{
			name: "post-merge",
			content: `#!/usr/bin/env node
// post-merge: auto-transition issue to Done on PR merge

const { execSync } = require('child_process');

try {
  const mergeMsg = execSync('git log -1 --format="%s%n%b" HEAD', { encoding: 'utf8' });
  const prMatch = mergeMsg.match(/#(\d+)/);
  if (!prMatch) process.exit(0);

  const prNumber = prMatch[1];
  let prData;
  try {
    prData = JSON.parse(execSync(
      'gh pr view ' + prNumber + ' --json number,url,state,title,body,mergedAt,headRefName',
      { encoding: 'utf8' }
    ));
  } catch (e) {
    process.exit(0); // not a PR context or gh not available
  }

  const allText = [prData.title, prData.body, prData.headRefName].join(' ');
  const issueMatch = allText.match(/(AGENTRA-\d+)/i);
  if (!issueMatch) process.exit(0);

  const apiUrl = process.env.AGENTRA_API_URL || 'http://localhost:8080';
  const token = process.env.AGENTRA_API_TOKEN;

  const body = JSON.stringify({
    issueId: issueMatch[1].toUpperCase(),
    prNumber, prUrl: prData.url,
    prState: prData.state.toLowerCase(),
    prTitle: prData.title,
    mergedAt: prData.mergedAt || null
  });

  const headers = { 'Content-Type': 'application/json' };
  if (token) headers['Authorization'] = 'Bearer ' + token;

  fetch(apiUrl + '/api/git/link-pr', {
    method: 'POST',
    headers: headers,
    body: body,
  }).catch(() => {});
} catch (e) {
  process.exit(0);
}
`,
		},
	}

	installed := 0
	for _, hook := range hooks {
		hookPath := filepath.Join(hooksDir, hook.name)
		if err := os.WriteFile(hookPath, []byte(hook.content), 0755); err != nil {
			return fmt.Errorf("write hook %s: %w", hook.name, err)
		}
		installed++
		fmt.Printf("Installed: .git/hooks/%s\n", hook.name)
	}

	fmt.Printf("\n✓ %d hooks installed in %s\n", installed, repoDir)
	fmt.Println("\nEnvironment variables:")
	fmt.Println("  AGENTRA_API_URL    API server URL (default: http://localhost:8080)")
	fmt.Println("  AGENTRA_API_TOKEN  API token for authentication (optional)")

	return nil
}

var gitHooksUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall git hooks",
	Long:  `Remove Agentra git hooks from the repository.`,
	RunE:  runGitHooksUninstall,
}

func init() {
	gitHooksUninstallCmd.Flags().StringVar(&hooksInstallDir, "dir", ".", "Repository directory")
	gitHooksCmd.AddCommand(gitHooksUninstallCmd)
}

func runGitHooksUninstall(cmd *cobra.Command, args []string) error {
	repoDir, err := filepath.Abs(hooksInstallDir)
	if err != nil {
		return fmt.Errorf("invalid directory: %w", err)
	}

	hooksDir := filepath.Join(repoDir, ".git", "hooks")
	hookNames := []string{"prepare-commit-msg", "post-commit", "post-merge"}

	removed := 0
	for _, name := range hookNames {
		hookPath := filepath.Join(hooksDir, name)
		if err := os.Remove(hookPath); err == nil {
			fmt.Printf("Removed: .git/hooks/%s\n", name)
			removed++
		}
	}

	fmt.Printf("\n✓ %d hooks removed from %s\n", removed, repoDir)
	return nil
}

var gitHooksCmd = &cobra.Command{
	Use:   "hooks",
	Short: "Manage git hooks for Agentra integration",
	Long:  `Install or uninstall git hooks that link commits/PRs to Agentra issues.

Examples:
  agentra git hooks install              # Install hooks in current repo
  agentra git hooks install --dir /path/to/repo
  agentra git hooks uninstall           # Remove hooks from current repo
`,
}

func init() {
	gitHooksCmd.AddCommand(gitHooksInstallCmd)
	gitHooksCmd.AddCommand(gitHooksUninstallCmd)
	gitCmd.AddCommand(gitHooksCmd)
}