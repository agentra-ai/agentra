package tools

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// initRepo creates a temp dir with a git repo, one commit, and a
// remote (local path). It detects the actual default branch name
// (master, main, or whatever the system git is configured to use)
// so the test is portable across systems with different
// `init.defaultBranch` settings.
func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bareDir := filepath.Join(t.TempDir(), "bare.git")

	gitCmd(t, "", "init", "--bare", bareDir)
	gitCmd(t, dir, "init")
	gitCmd(t, dir, "config", "user.email", "test@x")
	gitCmd(t, dir, "config", "user.name", "Test")
	gitCmd(t, dir, "remote", "add", "origin", bareDir)
	gitCmd(t, dir, "commit", "--allow-empty", "-m", "init")
	// Detect the actual default branch name (master/main/whatever).
	branchOut, err := exec.Command("git", "-C", dir, "rev-parse", "--abbrev-ref", "HEAD").CombinedOutput()
	if err != nil {
		t.Fatalf("detect branch: %v: %s", err, branchOut)
	}
	branch := strings.TrimSpace(string(branchOut))
	gitCmd(t, dir, "push", "-u", "origin", branch)
	return dir
}

// gitCmd runs `git` with the given args. dir may be empty to mean
// the current working directory. Fails the test on any error.
func gitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	c := exec.Command("git", args...)
	if dir != "" {
		c.Dir = dir
	}
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

func TestGitStatus_Empty(t *testing.T) {
	dir := initRepo(t)
	tool := &GitStatusTool{WorkDir: dir}
	res, err := tool.Execute(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Error != "" {
		t.Fatalf("tool error: %s", res.Error)
	}
	// "nothing to commit" or "no changes added" both indicate a
	// clean tree.
	if !strings.Contains(res.Content, "nothing to commit") &&
		!strings.Contains(res.Content, "no changes added") {
		t.Errorf("expected clean status, got: %s", res.Content)
	}
}

func TestCreateBranch(t *testing.T) {
	dir := initRepo(t)
	tool := &CreateBranchTool{WorkDir: dir}
	res, err := tool.Execute(context.Background(), map[string]any{"name": "feature-x"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Error != "" {
		t.Fatalf("tool error: %s", res.Error)
	}
	stRes, _ := (&GitStatusTool{WorkDir: dir}).Execute(context.Background(), nil)
	if !strings.Contains(stRes.Content, "feature-x") {
		t.Errorf("expected branch in status, got: %s", stRes.Content)
	}
}

// TestGitDiff_HappyPath confirms a tracked, modified file appears in
// the unified diff output.
func TestGitDiff_HappyPath(t *testing.T) {
	dir := initRepo(t)
	// Create + commit a tracked file.
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, dir, "add", "a.txt")
	gitCmd(t, dir, "commit", "-m", "add a.txt")
	// Modify it.
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tool := &GitDiffTool{WorkDir: dir}
	res, err := tool.Execute(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Error != "" {
		t.Fatalf("tool error: %s", res.Error)
	}
	if !strings.Contains(res.Content, "-v1") || !strings.Contains(res.Content, "+v2") {
		t.Errorf("expected diff to contain -v1 and +v2, got: %s", res.Content)
	}
}

// TestGitDiff_WithFile confirms the `file` arg scopes the diff to
// just that file (so changes to other files are excluded).
func TestGitDiff_WithFile(t *testing.T) {
	dir := initRepo(t)
	// Create + commit two tracked files.
	for _, name := range []string{"a.txt", "b.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name+":v1\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gitCmd(t, dir, "add", "a.txt", "b.txt")
	gitCmd(t, dir, "commit", "-m", "add a and b")
	// Modify both.
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a:v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b:v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tool := &GitDiffTool{WorkDir: dir}
	res, _ := tool.Execute(context.Background(), map[string]any{"file": "a.txt"})
	if res.Error != "" {
		t.Fatalf("tool error: %s", res.Error)
	}
	if !strings.Contains(res.Content, "a.txt") {
		t.Errorf("expected diff to mention a.txt, got: %s", res.Content)
	}
	if strings.Contains(res.Content, "b.txt") {
		t.Errorf("expected diff NOT to mention b.txt, got: %s", res.Content)
	}
}

// TestGitDiff_PathTraversal confirms a `file` arg that escapes the
// work directory is rejected as a tool error before git is invoked.
func TestGitDiff_PathTraversal(t *testing.T) {
	dir := initRepo(t)
	tool := &GitDiffTool{WorkDir: dir}
	res, _ := tool.Execute(context.Background(), map[string]any{"file": "../etc/passwd"})
	if res.Error == "" {
		t.Error("expected error for path traversal")
	}
	if !strings.Contains(res.Error, "escapes") {
		t.Errorf("expected 'escapes' in error, got %q", res.Error)
	}
}

// TestGitCommit_HappyPath confirms a staged file is committed and
// the new SHA is returned in Content.
func TestGitCommit_HappyPath(t *testing.T) {
	dir := initRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tool := &GitCommitTool{WorkDir: dir}
	res, err := tool.Execute(context.Background(), map[string]any{"message": "add a"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Error != "" {
		t.Fatalf("tool error: %s", res.Error)
	}
	sha := strings.TrimSpace(res.Content)
	if len(sha) < 7 {
		t.Fatalf("expected SHA, got %q", res.Content)
	}
	// Verify the commit actually exists.
	out, err := exec.Command("git", "-C", dir, "cat-file", "-t", sha).CombinedOutput()
	if err != nil {
		t.Fatalf("cat-file: %v: %s", err, out)
	}
	if strings.TrimSpace(string(out)) != "commit" {
		t.Errorf("expected commit, got %s", out)
	}
}

// TestGitCommit_NothingToCommit confirms a clean work tree produces
// a non-error result with "nothing to commit" in Content — per the
// Result.Error semantics in tool.go: a normal git exit is not a
// tool error, and "nothing to commit" is a routine outcome the LLM
// should reason about, not a transport failure.
func TestGitCommit_NothingToCommit(t *testing.T) {
	dir := initRepo(t)
	tool := &GitCommitTool{WorkDir: dir}
	res, _ := tool.Execute(context.Background(), map[string]any{"message": "no-op"})
	if res.Error != "" {
		t.Errorf("expected empty Error for 'nothing to commit', got %q", res.Error)
	}
	if !strings.Contains(res.Content, "nothing to commit") {
		t.Errorf("expected 'nothing to commit' in Content, got %q", res.Content)
	}
}

// TestGitCommit_HookFailure confirms that when git itself fails (we
// force a non-zero exit via a pre-commit hook that exits 1) the
// result populates Result.Stderr but does NOT set Result.Error.
// This is the core behavior change in Issue 1: non-zero exit codes
// are not tool errors, so the LLM runtime's retry/classification
// logic isn't tripped by routine git failures. We use a hook
// (rather than e.g. omitting user.email/user.name) because the
// test environment may have a global git user identity that would
// otherwise let `git commit` succeed.
func TestGitCommit_HookFailure(t *testing.T) {
	dir := t.TempDir()
	gitCmd(t, dir, "init")
	gitCmd(t, dir, "config", "user.email", "test@x")
	gitCmd(t, dir, "config", "user.name", "Test")
	// Install a pre-commit hook that always fails so the commit
	// step is guaranteed to exit non-zero. The hook writes to
	// stderr so we can verify Result.Stderr captures it.
	hookPath := filepath.Join(dir, ".git", "hooks", "pre-commit")
	if err := os.WriteFile(hookPath, []byte("#!/bin/sh\necho 'pre-commit hook failed' 1>&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tool := &GitCommitTool{WorkDir: dir}
	res, _ := tool.Execute(context.Background(), map[string]any{"message": "x"})
	if res.Error != "" {
		t.Errorf("expected empty Error on non-zero git exit, got %q", res.Error)
	}
	if res.ExitCode == 0 {
		t.Error("expected non-zero exit code")
	}
	if !strings.Contains(res.Stderr, "pre-commit hook failed") {
		t.Errorf("expected 'pre-commit hook failed' in stderr, got %q", res.Stderr)
	}
}

// TestGitPush_NoOp confirms a push of the already-tracked branch to
// its existing upstream succeeds against a local bare repo (no
// network required).
func TestGitPush_NoOp(t *testing.T) {
	dir := initRepo(t)
	branchOut, err := exec.Command("git", "-C", dir, "rev-parse", "--abbrev-ref", "HEAD").CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}
	branch := strings.TrimSpace(string(branchOut))
	tool := &GitPushTool{WorkDir: dir}
	res, _ := tool.Execute(context.Background(), map[string]any{"branch": branch})
	if res.Error != "" {
		t.Fatalf("tool error: %s", res.Error)
	}
	if res.ExitCode != 0 {
		t.Errorf("expected exit 0, got %d, stderr=%s", res.ExitCode, res.Stderr)
	}
	if !strings.Contains(res.Content, "pushed origin/"+branch) {
		t.Errorf("expected 'pushed origin/%s' in Content, got %q", branch, res.Content)
	}
}

// TestCreateBranch_AlreadyExists confirms a duplicate branch name
// produces a non-zero exit with stderr populated, but does NOT set
// Result.Error (per Issue 1's semantics: a non-zero exit is not a
// tool error).
func TestCreateBranch_AlreadyExists(t *testing.T) {
	dir := initRepo(t)
	tool := &CreateBranchTool{WorkDir: dir}
	if _, err := tool.Execute(context.Background(), map[string]any{"name": "dup"}); err != nil {
		t.Fatal(err)
	}
	// Switch back so we can try to create the same branch again.
	gitCmd(t, dir, "checkout", "-")
	res, _ := tool.Execute(context.Background(), map[string]any{"name": "dup"})
	if res.Error != "" {
		t.Errorf("expected empty Error on non-zero git exit, got %q", res.Error)
	}
	if res.ExitCode == 0 {
		t.Error("expected non-zero exit code")
	}
	if !strings.Contains(res.Stderr, "already exists") {
		t.Errorf("expected 'already exists' in stderr, got %q", res.Stderr)
	}
}
