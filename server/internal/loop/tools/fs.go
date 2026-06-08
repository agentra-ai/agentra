package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// readFileCap is the maximum number of bytes we return from a read_file
// call before truncating with a marker. Protects the LLM context from
// accidentally large files.
const readFileCap = 10 * 1024

// searchOutputCap caps grep output. Grep matches can be voluminous; the
// LLM only needs the first few hits to reason about symbol location.
const searchOutputCap = 5 * 1024

// ReadFileTool reads a file relative to WorkDir.
type ReadFileTool struct {
	WorkDir string
}

func (t *ReadFileTool) Name() string { return "read_file" }

func (t *ReadFileTool) Description() string {
	return "Read the contents of a file at <path> relative to the task work directory. " +
		"Files larger than 10KB are truncated with a marker."
}

func (t *ReadFileTool) Schema() map[string]any {
	return map[string]any{
		"name":        "read_file",
		"description": t.Description(),
		"input_schema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Path to the file, relative to the work directory.",
				},
			},
			"required": []string{"path"},
		},
	}
}

func (t *ReadFileTool) Execute(ctx context.Context, args map[string]any) (Result, error) {
	path, _ := args["path"].(string)
	if path == "" {
		return Result{Error: "path is required"}, nil
	}
	full, err := safeJoin(t.WorkDir, path)
	if err != nil {
		return Result{Error: err.Error()}, nil
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return Result{Error: fmt.Sprintf("read %s: %v", path, err)}, nil
	}
	if len(data) > readFileCap {
		truncated := make([]byte, readFileCap)
		copy(truncated, data[:readFileCap])
		return Result{
			Content: string(truncated) + "\n... [truncated, file is " +
				fmt.Sprintf("%d bytes", len(data)) + "]",
		}, nil
	}
	return Result{Content: string(data)}, nil
}

// WriteFileTool writes content to a file relative to WorkDir, creating
// parent directories as needed.
type WriteFileTool struct {
	WorkDir string
}

func (t *WriteFileTool) Name() string { return "write_file" }

func (t *WriteFileTool) Description() string {
	return "Write <content> to a file at <path> relative to the work directory. " +
		"Parent directories are created as needed. Overwrites existing files."
}

func (t *WriteFileTool) Schema() map[string]any {
	return map[string]any{
		"name":        "write_file",
		"description": t.Description(),
		"input_schema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Destination path, relative to the work directory.",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "File content to write.",
				},
			},
			"required": []string{"path", "content"},
		},
	}
}

func (t *WriteFileTool) Execute(ctx context.Context, args map[string]any) (Result, error) {
	path, _ := args["path"].(string)
	if path == "" {
		return Result{Error: "path is required"}, nil
	}
	content, _ := args["content"].(string)
	full, err := safeJoin(t.WorkDir, path)
	if err != nil {
		return Result{Error: err.Error()}, nil
	}
	if dir := filepath.Dir(full); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return Result{Error: fmt.Sprintf("mkdir %s: %v", dir, err)}, nil
		}
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		return Result{Error: fmt.Sprintf("write %s: %v", path, err)}, nil
	}
	return Result{Content: fmt.Sprintf("wrote %d bytes to %s", len(content), path)}, nil
}

// SearchCodeTool runs grep -rn over WorkDir, returning matches.
type SearchCodeTool struct {
	WorkDir string
}

func (t *SearchCodeTool) Name() string { return "search_code" }

func (t *SearchCodeTool) Description() string {
	return "Search for <pattern> across files in the work directory. Uses grep -rn. " +
		"Output is capped at 5KB."
}

func (t *SearchCodeTool) Schema() map[string]any {
	return map[string]any{
		"name":        "search_code",
		"description": t.Description(),
		"input_schema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern": map[string]any{
					"type":        "string",
					"description": "Regex pattern to search for.",
				},
				"path": map[string]any{
					"type":        "string",
					"description": "Subdirectory to scope the search to, relative to work dir. Defaults to '.'.",
				},
			},
			"required": []string{"pattern"},
		},
	}
}

func (t *SearchCodeTool) Execute(ctx context.Context, args map[string]any) (Result, error) {
	pattern, _ := args["pattern"].(string)
	if pattern == "" {
		return Result{Error: "pattern is required"}, nil
	}
	scope, _ := args["path"].(string)
	if scope == "" {
		scope = "."
	}
	scopeFull, err := safeJoin(t.WorkDir, scope)
	if err != nil {
		return Result{Error: err.Error()}, nil
	}

	cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cctx, "grep", "-rn", "--", pattern, scopeFull)
	cmd.Dir = t.WorkDir
	stdout := &limitedBuffer{max: searchOutputCap}
	stderr := &limitedBuffer{max: searchOutputCap}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err = cmd.Run()
	out := stdout.String()
	if cctx.Err() == context.DeadlineExceeded {
		return Result{Error: "search timed out after 30s", Stderr: stderr.String()}, nil
	}
	// grep exits 1 when no matches — that is a successful search, not a
	// tool error. Surface "no matches" so the LLM can move on.
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 && out == "" {
			return Result{Content: "no matches"}, nil
		}
		return Result{Error: fmt.Sprintf("grep failed: %v", err), Stderr: stderr.String()}, nil
	}
	return Result{Content: out}, nil
}

// safeJoin resolves path against WorkDir and rejects paths that escape it.
// The check is the standard "cleaned path must start with cleaned work dir
// plus separator" pattern, which avoids the "WorkDir is /foo and path is
// /foobar" bypass that a plain HasPrefix would allow.
func safeJoin(workDir, path string) (string, error) {
	if workDir == "" {
		return "", fmt.Errorf("work directory is not set")
	}
	cleanedWork := filepath.Clean(workDir)
	full := filepath.Join(cleanedWork, path)
	cleanedFull := filepath.Clean(full)
	sep := string(os.PathSeparator)
	if cleanedFull != cleanedWork && !strings.HasPrefix(cleanedFull, cleanedWork+sep) {
		return "", fmt.Errorf("path %q escapes work directory", path)
	}
	return cleanedFull, nil
}

func init() {
	Register(&ReadFileTool{})
	Register(&WriteFileTool{})
	Register(&SearchCodeTool{})
}
