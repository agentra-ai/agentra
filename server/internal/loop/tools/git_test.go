package tools

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// initRepo creates a temp dir with a git repo, one commit, and a remote
// (local path). It detects the actual default branch name (master, main,
// or whatever the system git is configured to use) so the test is
// portable across systems with different `init.defaultBranch` settings.
func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bareDir := filepath.Join(t.TempDir(), "bare.git")

	runGit := func(dir string, args ...string) {
		c := exec.Command("git", args...)
		if dir != "" {
			c.Dir = dir
		}
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}

	runGit("", "init", "--bare", bareDir)
	runGit(dir, "init")
	runGit(dir, "config", "user.email", "test@x")
	runGit(dir, "config", "user.name", "Test")
	runGit(dir, "remote", "add", "origin", bareDir)
	runGit(dir, "commit", "--allow-empty", "-m", "init")
	// Detect the actual default branch name (master/main/whatever)
	branchOut, err := exec.Command("git", "-C", dir, "rev-parse", "--abbrev-ref", "HEAD").CombinedOutput()
	if err != nil {
		t.Fatalf("detect branch: %v: %s", err, branchOut)
	}
	branch := strings.TrimSpace(string(branchOut))
	runGit(dir, "push", "-u", "origin", branch)
	return dir
}

func TestGitStatus_Empty(t *testing.T) {
	dir := initRepo(t)
	tool := &GitStatusTool{WorkDir: dir}
	res, err := tool.Execute(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	// "nothing to commit" or "no changes added" both indicate a clean tree
	if !strings.Contains(res.Content, "nothing to commit") &&
		!strings.Contains(res.Content, "no changes added") {
		t.Errorf("expected clean status, got: %s", res.Content)
	}
}

func TestCreateBranch(t *testing.T) {
	dir := initRepo(t)
	tool := &CreateBranchTool{WorkDir: dir}
	_, err := tool.Execute(context.Background(), map[string]any{"name": "feature-x"})
	if err != nil {
		t.Fatal(err)
	}
	res, _ := (&GitStatusTool{WorkDir: dir}).Execute(context.Background(), nil)
	if !strings.Contains(res.Content, "feature-x") {
		t.Errorf("expected branch in status, got: %s", res.Content)
	}
}
