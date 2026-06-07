package tools

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

// shellCap is the per-stream cap (stdout and stderr) for run_command /
// run_test. A test run can easily produce >50KB of output, but the LLM
// only needs the tail (failures, panics) — the head is usually noise.
const shellCap = 50 * 1024

// limitedBuffer is an io.Writer that drops bytes past `max`. It is not
// safe for concurrent use; os/exec never writes concurrently to the
// same stream.
type limitedBuffer struct {
	max  int
	buf  []byte
	dropped int
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	room := b.max - len(b.buf)
	if room <= 0 {
		b.dropped += len(p)
		return len(p), nil
	}
	if len(p) > room {
		b.dropped += len(p) - room
		b.buf = append(b.buf, p[:room]...)
		return len(p), nil
	}
	b.buf = append(b.buf, p...)
	return len(p), nil
}

func (b *limitedBuffer) String() string {
	if b.dropped == 0 {
		return string(b.buf)
	}
	return string(b.buf) + fmt.Sprintf("\n... [%d bytes dropped]", b.dropped)
}

// RunCommandTool runs a shell command inside WorkDir.
type RunCommandTool struct {
	WorkDir        string
	DefaultTimeout time.Duration
}

func (t *RunCommandTool) Name() string { return "run_command" }

func (t *RunCommandTool) Description() string {
	return "Run a shell command in the task work directory via `sh -c`. " +
		"Default timeout is 5 minutes; override with timeout_sec. " +
		"Non-zero exit codes are reported in exit_code and stderr — they " +
		"are not tool errors."
}

func (t *RunCommandTool) Schema() map[string]any {
	return map[string]any{
		"name":        "run_command",
		"description": t.Description(),
		"input_schema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"cmd": map[string]any{
					"type":        "string",
					"description": "Shell command to run.",
				},
				"timeout_sec": map[string]any{
					"type":        "number",
					"description": "Override the default 5-minute timeout, in seconds.",
				},
			},
			"required": []string{"cmd"},
		},
	}
}

func (t *RunCommandTool) Execute(ctx context.Context, args map[string]any) (Result, error) {
	cmdStr, _ := args["cmd"].(string)
	if cmdStr == "" {
		return Result{Error: "cmd is required"}, nil
	}
	timeout := t.DefaultTimeout
	if v, ok := args["timeout_sec"].(float64); ok && v > 0 {
		timeout = time.Duration(v) * time.Second
	}
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cctx, "sh", "-c", cmdStr)
	cmd.Dir = t.WorkDir
	stdout := &limitedBuffer{max: shellCap}
	stderr := &limitedBuffer{max: shellCap}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	_ = cmd.Run()

	res := Result{ExitCode: 0, Stderr: stderr.String()}
	if cmd.ProcessState != nil {
		res.ExitCode = cmd.ProcessState.ExitCode()
	}
	res.Content = stdout.String()

	if cctx.Err() == context.DeadlineExceeded {
		res.Error = fmt.Sprintf("command timed out after %s", timeout)
		return res, nil
	}
	// A non-zero exit is NOT a tool error: leave Error empty, the LLM
	// sees the failure in Stderr + ExitCode and decides what to do.
	return res, nil
}

// RunTestTool runs the project's test suite. It is a thin specialization
// of RunCommandTool — same shape, different default command and longer
// default timeout.
type RunTestTool struct {
	WorkDir string
	Cmd     string
	// DefaultTimeout is the per-run timeout. Zero falls back to 10 min.
	DefaultTimeout time.Duration
}

func (t *RunTestTool) Name() string { return "run_test" }

func (t *RunTestTool) Description() string {
	cmd := t.Cmd
	if cmd == "" {
		cmd = "go test ./..."
	}
	return "Run the project test suite. Equivalent to run_command with the " +
		"test command as `cmd`. Default command: `" + cmd + "`. " +
		"Default timeout is 10 minutes."
}

func (t *RunTestTool) Schema() map[string]any {
	cmd := t.Cmd
	if cmd == "" {
		cmd = "go test ./..."
	}
	return map[string]any{
		"name":        "run_test",
		"description": t.Description(),
		"input_schema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"cmd": map[string]any{
					"type":        "string",
					"description": "Override the test command. Defaults to `" + cmd + "`.",
				},
				"timeout_sec": map[string]any{
					"type":        "number",
					"description": "Override the default 10-minute timeout, in seconds.",
				},
			},
		},
	}
}

func (t *RunTestTool) Execute(ctx context.Context, args map[string]any) (Result, error) {
	cmdStr, _ := args["cmd"].(string)
	if cmdStr == "" {
		cmdStr = t.Cmd
	}
	if cmdStr == "" {
		cmdStr = "go test ./..."
	}
	timeout := t.DefaultTimeout
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	if v, ok := args["timeout_sec"].(float64); ok && v > 0 {
		timeout = time.Duration(v) * time.Second
	}
	rc := &RunCommandTool{
		WorkDir:        t.WorkDir,
		DefaultTimeout: timeout,
	}
	return rc.Execute(ctx, map[string]any{
		"cmd":        cmdStr,
		"timeout_sec": float64(timeout / time.Second),
	})
}

func init() {
	Register(&RunCommandTool{DefaultTimeout: 5 * time.Minute})
	Register(&RunTestTool{Cmd: "go test ./..."})
}
