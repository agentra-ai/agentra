package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestReadFile_HappyPath confirms a relative path resolves inside the
// work dir and the file content is returned verbatim.
func TestReadFile_HappyPath(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	t1 := &ReadFileTool{WorkDir: dir}
	res, err := t1.Execute(context.Background(), map[string]any{"path": "a.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Error != "" {
		t.Fatalf("tool error: %s", res.Error)
	}
	if res.Content != "hello" {
		t.Errorf("got %q, want %q", res.Content, "hello")
	}
}

// TestReadFile_PathTraversal confirms an attempt to read a sibling
// (../etc/passwd) is rejected as a tool error, not a read error.
func TestReadFile_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	t1 := &ReadFileTool{WorkDir: dir}
	res, _ := t1.Execute(context.Background(), map[string]any{"path": "../etc/passwd"})
	if res.Error == "" {
		t.Error("expected error for path traversal")
	}
	if !strings.Contains(res.Error, "escapes") {
		t.Errorf("expected 'escapes' in error, got %q", res.Error)
	}
}

// TestWriteFile_Roundtrip confirms a write creates parent directories
// and the file content round-trips.
func TestWriteFile_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	t1 := &WriteFileTool{WorkDir: dir}
	if _, err := t1.Execute(context.Background(), map[string]any{"path": "sub/a.txt", "content": "x"}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "sub", "a.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "x" {
		t.Errorf("got %q, want %q", got, "x")
	}
}
