package tools

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// All git tools run from the loop's work_dir. The branch to operate on is
// passed as an argument (default: current branch).

// GitStatusTool runs `git status` in WorkDir.
type GitStatusTool struct{ WorkDir string }

func (t *GitStatusTool) Name() string         { return "git_status" }
func (t *GitStatusTool) Description() string  { return "Run `git status` and return the output." }
func (t *GitStatusTool) Schema() map[string]any { return map[string]any{"type": "object"} }
func (t *GitStatusTool) Execute(ctx context.Context, args map[string]any) (Result, error) {
	return runGit(ctx, t.WorkDir, 10*time.Second, "status")
}

// GitDiffTool shows a unified diff in WorkDir. Pass `staged:true` for the
// index, or `file` to limit the diff to a single path.
type GitDiffTool struct{ WorkDir string }

func (t *GitDiffTool) Name() string        { return "git_diff" }
func (t *GitDiffTool) Description() string { return "Show a unified diff. Optionally limit to a single file (path arg)." }
func (t *GitDiffTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"staged": map[string]any{"type": "boolean", "default": false},
			"file":   map[string]any{"type": "string"},
		},
	}
}
func (t *GitDiffTool) Execute(ctx context.Context, args map[string]any) (Result, error) {
	gitArgs := []string{"diff"}
	if staged, _ := args["staged"].(bool); staged {
		gitArgs = append(gitArgs, "--staged")
	}
	if file, _ := args["file"].(string); file != "" {
		gitArgs = append(gitArgs, "--", file)
	}
	return runGit(ctx, t.WorkDir, 30*time.Second, gitArgs...)
}

// GitCommitTool stages all changes and commits with the given message.
// Returns the new commit SHA on success.
type GitCommitTool struct{ WorkDir string }

func (t *GitCommitTool) Name() string        { return "git_commit" }
func (t *GitCommitTool) Description() string { return "Stage all changes and commit with the given message. Returns the new SHA." }
func (t *GitCommitTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"message": map[string]any{"type": "string"},
		},
		"required": []string{"message"},
	}
}
func (t *GitCommitTool) Execute(ctx context.Context, args map[string]any) (Result, error) {
	message, _ := args["message"].(string)
	if message == "" {
		return Result{Error: "message is required"}, nil
	}
	// git add -A: stage every change in the work tree.
	if _, err := runGit(ctx, t.WorkDir, 30*time.Second, "add", "-A"); err != nil {
		return Result{Error: err.Error()}, nil
	}
	// Detect "nothing to commit" via `git diff --cached --quiet`. Exit 0 =
	// no staged changes; exit 1 = staged changes exist; exit 2+ = error.
	// This is more reliable than parsing `git diff --staged --stat`
	// because the stat output is noisy and may be empty for a brand-new
	// repo with a single empty commit.
	check, _ := runGit(ctx, t.WorkDir, 10*time.Second, "diff", "--cached", "--quiet")
	if check.ExitCode == 0 {
		return Result{Error: "no changes to commit"}, nil
	}
	if check.ExitCode > 1 {
		return Result{Error: fmt.Sprintf("git diff --cached failed: %s", check.Stderr)}, nil
	}
	// git commit. The plan's note: surface a clear error if the commit
	// itself fails (pre-commit hook, gpg signing, etc.) so the LLM can
	// react rather than retry blindly.
	if commitRes, err := runGit(ctx, t.WorkDir, 30*time.Second, "commit", "-m", message); err != nil {
		return Result{Error: fmt.Sprintf("git commit failed: %s", commitRes.Stderr)}, nil
	}
	shaRes, _ := runGit(ctx, t.WorkDir, 5*time.Second, "rev-parse", "HEAD")
	return Result{Content: strings.TrimSpace(shaRes.Content)}, nil
}

// GitPushTool pushes the given branch to the given remote (default: origin).
type GitPushTool struct{ WorkDir string }

func (t *GitPushTool) Name() string        { return "git_push" }
func (t *GitPushTool) Description() string { return "Push the current branch to the given remote (default: origin)." }
func (t *GitPushTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"remote": map[string]any{"type": "string", "default": "origin"},
			"branch": map[string]any{"type": "string"},
		},
		"required": []string{"branch"},
	}
}
func (t *GitPushTool) Execute(ctx context.Context, args map[string]any) (Result, error) {
	remote, _ := args["remote"].(string)
	if remote == "" {
		remote = "origin"
	}
	branch, _ := args["branch"].(string)
	if branch == "" {
		return Result{Error: "branch is required"}, nil
	}
	if pushRes, err := runGit(ctx, t.WorkDir, 60*time.Second, "push", remote, branch); err != nil {
		return Result{Error: fmt.Sprintf("git push failed: %s", pushRes.Stderr)}, nil
	}
	return Result{Content: fmt.Sprintf("pushed %s/%s", remote, branch)}, nil
}

// CreateBranchTool creates and checks out a new branch from the current HEAD.
type CreateBranchTool struct{ WorkDir string }

func (t *CreateBranchTool) Name() string        { return "create_branch" }
func (t *CreateBranchTool) Description() string { return "Create and check out a new branch from the current HEAD." }
func (t *CreateBranchTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{"type": "string"},
		},
		"required": []string{"name"},
	}
}
func (t *CreateBranchTool) Execute(ctx context.Context, args map[string]any) (Result, error) {
	name, _ := args["name"].(string)
	if name == "" {
		return Result{Error: "name is required"}, nil
	}
	if _, err := runGit(ctx, t.WorkDir, 5*time.Second, "checkout", "-b", name); err != nil {
		return Result{Error: err.Error()}, nil
	}
	return Result{Content: "switched to " + name}, nil
}

// GitHubPRCreateTool opens a GitHub PR via the `gh` CLI. Returns the PR
// URL in Result.PRURL (and Content) on success.
type GitHubPRCreateTool struct{ WorkDir string }

func (t *GitHubPRCreateTool) Name() string        { return "github_pr_create" }
func (t *GitHubPRCreateTool) Description() string { return "Open a GitHub PR using `gh pr create`. Returns PR URL on success." }
func (t *GitHubPRCreateTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"title": map[string]any{"type": "string"},
			"body":  map[string]any{"type": "string"},
			"base":  map[string]any{"type": "string", "default": "main"},
		},
		"required": []string{"title", "body"},
	}
}
func (t *GitHubPRCreateTool) Execute(ctx context.Context, args map[string]any) (Result, error) {
	title, _ := args["title"].(string)
	body, _ := args["body"].(string)
	base, _ := args["base"].(string)
	if base == "" {
		base = "main"
	}
	if title == "" || body == "" {
		return Result{Error: "title and body are required"}, nil
	}
	// Use gh CLI; assume it's authenticated in dogfood mode. gh writes the
	// PR URL to stdout on success and errors to stderr on failure, so
	// CombinedOutput captures both.
	cctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cctx, "gh", "pr", "create", "--title", title, "--body", body, "--base", base)
	cmd.Dir = t.WorkDir
	out, err := cmd.CombinedOutput()
	if cctx.Err() == context.DeadlineExceeded {
		return Result{Error: "gh pr create timed out after 60s"}, nil
	}
	if err != nil {
		return Result{Error: fmt.Sprintf("gh pr create: %v: %s", err, out)}, nil
	}
	// gh output: "https://github.com/owner/repo/pull/N"
	url := strings.TrimSpace(string(out))
	return Result{Content: url, PRURL: url}, nil
}

// runGit executes git in workDir with the given args and a timeout.
// The error message is intentionally generic so we don't leak the full
// command line (which can include paths or messages) back to the LLM.
func runGit(ctx context.Context, workDir string, timeout time.Duration, args ...string) (Result, error) {
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(cctx, "git", args...)
	cmd.Dir = workDir
	out, err := cmd.CombinedOutput()
	res := Result{Content: string(out), ExitCode: 0}
	if cmd.ProcessState != nil {
		res.ExitCode = cmd.ProcessState.ExitCode()
	}
	if cctx.Err() == context.DeadlineExceeded {
		return res, fmt.Errorf("git: %w", context.DeadlineExceeded)
	}
	if err != nil {
		return res, fmt.Errorf("git: %w", err)
	}
	return res, nil
}

func init() {
	Register(&GitStatusTool{})
	Register(&GitDiffTool{})
	Register(&GitCommitTool{})
	Register(&GitPushTool{})
	Register(&CreateBranchTool{})
	Register(&GitHubPRCreateTool{})
}
