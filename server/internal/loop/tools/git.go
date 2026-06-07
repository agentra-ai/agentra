package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// All git tools run from the loop's work_dir. The branch to operate
// on is passed as an argument (default: current branch).

// GitStatusTool runs `git status` in WorkDir.
type GitStatusTool struct{ WorkDir string }

func (t *GitStatusTool) Name() string        { return "git_status" }
func (t *GitStatusTool) Description() string { return "Run `git status` and return the output." }
func (t *GitStatusTool) Schema() map[string]any {
	return map[string]any{"type": "object"}
}
func (t *GitStatusTool) Execute(ctx context.Context, args map[string]any) (Result, error) {
	return runGitOK(ctx, t.WorkDir, 10*time.Second, "status")
}

// GitDiffTool shows a unified diff in WorkDir. Pass `staged:true` for
// the index, or `file` to limit the diff to a single path. The `file`
// arg is validated against WorkDir so the `--` separator is not a
// path-traversal bypass.
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
		// Validate the user-supplied path resolves inside WorkDir
		// before passing it to git, so `--` isn't a path-traversal
		// bypass.
		if _, err := safeJoin(t.WorkDir, file); err != nil {
			return Result{Error: err.Error()}, nil
		}
		gitArgs = append(gitArgs, "--", file)
	}
	return runGitOK(ctx, t.WorkDir, 30*time.Second, gitArgs...)
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
	// `git add -A`: stage every change in the work tree. A non-zero
	// exit propagates as-is so the LLM can read the actual git
	// stderr via Result.Stderr.
	if res, err := runGit(ctx, t.WorkDir, 30*time.Second, "add", "-A"); err != nil || res.ExitCode != 0 {
		return toErrResult(res, err), nil
	}
	// Detect "nothing to commit" via `git diff --cached --quiet`.
	// Exit 0 = no staged changes; exit 1 = staged changes exist;
	// exit 2+ = error. More reliable than parsing stat output,
	// which is noisy and may be empty for a brand-new repo with a
	// single empty commit.
	check, err := runGit(ctx, t.WorkDir, 10*time.Second, "diff", "--cached", "--quiet")
	if err != nil {
		return toErrResult(check, err), nil
	}
	if check.ExitCode == 0 {
		// "nothing to commit" is a normal outcome, not a tool
		// error. The LLM sees this in Content and decides whether
		// to retry or move on.
		return Result{Content: "nothing to commit, working tree clean"}, nil
	}
	if check.ExitCode > 1 {
		return check, nil
	}
	// git commit. A non-zero exit (pre-commit hook, gpg signing,
	// etc.) is not a tool error — the LLM reads git's stderr.
	commitRes, err := runGit(ctx, t.WorkDir, 30*time.Second, "commit", "-m", message)
	if err != nil || commitRes.ExitCode != 0 {
		return toErrResult(commitRes, err), nil
	}
	shaRes, _ := runGit(ctx, t.WorkDir, 5*time.Second, "rev-parse", "HEAD")
	if shaRes.Error != "" {
		// Workdir was unset / process spawn failed mid-way through
		// a successful commit; surface it so the LLM can retry.
		return shaRes, nil
	}
	return Result{Content: strings.TrimSpace(shaRes.Content)}, nil
}

// GitPushTool pushes the given branch to the given remote (default:
// origin).
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
	pushRes, err := runGit(ctx, t.WorkDir, 60*time.Second, "push", remote, branch)
	if err != nil {
		return toErrResult(pushRes, err), nil
	}
	if pushRes.ExitCode != 0 {
		// Push failure is a normal outcome (no upstream, non-
		// fast-forward, auth failure, etc.). The LLM reads
		// stderr to decide.
		return pushRes, nil
	}
	return Result{Content: fmt.Sprintf("pushed %s/%s", remote, branch)}, nil
}

// CreateBranchTool creates and checks out a new branch from the
// current HEAD.
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
	coRes, _ := runGitOK(ctx, t.WorkDir, 5*time.Second, "checkout", "-b", name)
	if coRes.Error != "" || coRes.ExitCode != 0 {
		return coRes, nil
	}
	return Result{Content: "switched to " + name}, nil
}

// GitHubPRCreateTool opens a GitHub PR via the `gh` CLI. Returns the
// PR URL in Result.PRURL (and Content) on success.
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
	cctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	// Prefer --json url so URL extraction is robust against stderr
	// noise and warning banners that `gh` may emit. stdout/stderr are
	// captured separately and capped via limitedBuffer to protect
	// the LLM context.
	cmd := exec.CommandContext(cctx, "gh", "pr", "create",
		"--title", title, "--body", body, "--base", base, "--json", "url")
	cmd.Dir = t.WorkDir
	stdout := &limitedBuffer{max: shellCap}
	stderr := &limitedBuffer{max: shellCap}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	_ = cmd.Run()
	if cctx.Err() == context.DeadlineExceeded {
		return Result{Error: "gh pr create timed out after 60s", Stderr: stderr.String()}, nil
	}
	res := Result{Stderr: stderr.String()}
	if cmd.ProcessState != nil {
		res.ExitCode = cmd.ProcessState.ExitCode()
	}
	if res.ExitCode != 0 {
		// `gh` failed (auth, no remote, not a git repo, etc.).
		// Surface the actual stderr so the LLM can see why.
		res.Error = "gh pr create failed"
		return res, nil
	}
	// Parse JSON; fall back to scanning stdout for an http URL if
	// the installed `gh` doesn't support --json.
	out := stdout.String()
	var payload struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err == nil {
		url := strings.TrimSpace(payload.URL)
		if strings.HasPrefix(url, "http") {
			return Result{Content: url, PRURL: url}, nil
		}
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "http") {
			return Result{Content: line, PRURL: line}, nil
		}
	}
	res.Error = "gh pr create: could not parse PR URL from output"
	return res, nil
}

// runGit executes git in workDir with the given args and a timeout.
// stdout and stderr are captured separately, each capped at shellCap
// bytes (50KB) to protect the LLM context from huge diffs/status
// outputs. A non-zero git exit is NOT returned as a Go error: the
// function returns the captured output in Content/Stderr with a
// populated ExitCode. The Go error is reserved for actual tool-level
// failures (workdir unset, process spawn failure, timeout).
func runGit(ctx context.Context, workDir string, timeout time.Duration, args ...string) (Result, error) {
	if workDir == "" {
		return Result{}, fmt.Errorf("git: work directory is not set")
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(cctx, "git", args...)
	cmd.Dir = workDir
	stdout := &limitedBuffer{max: shellCap}
	stderr := &limitedBuffer{max: shellCap}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	runErr := cmd.Run()
	res := Result{Content: stdout.String(), Stderr: stderr.String()}
	if cmd.ProcessState != nil {
		res.ExitCode = cmd.ProcessState.ExitCode()
	}
	if cctx.Err() == context.DeadlineExceeded {
		return res, fmt.Errorf("git: timed out after %s", timeout)
	}
	// ProcessState is nil only when the process never started
	// (binary not found, etc.). For a non-zero exit, cmd.Run()
	// returns *exec.ExitError but ProcessState is still set —
	// those are normal git exits and not tool errors, so the
	// caller inspects Result.ExitCode/Stderr instead.
	if cmd.ProcessState == nil {
		return res, fmt.Errorf("git: %w", runErr)
	}
	return res, nil
}

// toErrResult converts a runGit (Result, error) return into the tool's
// standard Result shape. A non-nil Go error becomes Result.Error;
// otherwise the result is returned untouched. Used to short-circuit
// "either tool error or non-zero exit" branches without duplicating
// the conversion logic at every call site.
func toErrResult(res Result, err error) Result {
	if err != nil {
		res.Error = err.Error()
	}
	return res
}

// runGitOK is a thin convenience wrapper around runGit that converts
// any tool-level error into Result.Error. Use runGit directly when
// the caller needs to inspect ExitCode for special cases (e.g.
// "nothing to commit").
func runGitOK(ctx context.Context, workDir string, timeout time.Duration, args ...string) (Result, error) {
	res, err := runGit(ctx, workDir, timeout, args...)
	if err != nil {
		return Result{Error: err.Error(), Stderr: res.Stderr, ExitCode: res.ExitCode}, nil
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
