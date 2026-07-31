package dna

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestExtract(t *testing.T) {
	repoRoot := t.TempDir()
	writeFixtureFile(t, repoRoot, "package.json", `{"devDependencies":{"vitest":"latest"}}`)
	writeFixtureFile(t, repoRoot, "tsconfig.json", `{"compilerOptions":{"paths":{"@/*":["src/*"]}}}`)
	writeFixtureFile(t, repoRoot, "src/store.ts", `
import { create } from "zustand";
import { api } from "@/shared/api";
export const useStore = create<{ ready: boolean }>(() => ({ ready: true }));
void api;
`)
	writeFixtureFile(t, repoRoot, "src/store.test.ts", `import "./store";`)
	initFixtureRepository(t, repoRoot)

	result := Extract(context.Background(), repoRoot)

	if result == nil {
		t.Fatal("Extract returned nil")
	}
	if result.RepoRoot != repoRoot {
		t.Fatalf("RepoRoot = %q, want %q", result.RepoRoot, repoRoot)
	}
	if result.GeneratedAt == "" || result.HeadSHA == "" {
		t.Fatalf("expected generated timestamp and head SHA, got %#v", result)
	}
	if result.CommitStyle.Types["feat"] != 1 {
		t.Fatalf("commit types = %#v, want one feat commit", result.CommitStyle.Types)
	}
	if !result.CommitStyle.WithScope {
		t.Fatal("expected scoped commit style")
	}
	if result.Imports.Aliases["@/*"] != "src/*" {
		t.Fatalf("path aliases = %#v, want @/* -> src/*", result.Imports.Aliases)
	}
	if result.StateManagement != "zustand" {
		t.Fatalf("StateManagement = %q, want zustand", result.StateManagement)
	}
	if result.Testing.Framework != "vitest" || result.Testing.Pattern != "*.test.{ts,tsx}" {
		t.Fatalf("testing conventions = %#v, want vitest", result.Testing)
	}
	if _, err := json.Marshal(result); err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
}

func writeFixtureFile(t *testing.T, root, relativePath, contents string) {
	t.Helper()
	path := filepath.Join(root, relativePath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create fixture directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write fixture %s: %v", relativePath, err)
	}
}

func initFixtureRepository(t *testing.T, root string) {
	t.Helper()
	commands := [][]string{
		{"init"},
		{"config", "user.email", "repo-dna-test@agentra.ai"},
		{"config", "user.name", "Repo DNA Test"},
		{"add", "."},
		{"commit", "-m", "feat(web): add fixture store"},
	}
	for _, args := range commands {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
}
