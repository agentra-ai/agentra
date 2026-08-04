// runtimefixture is a hermetic fake coding-agent CLI used by the Runtime
// Adapter conformance suite. It speaks the minimum Claude, Codex, and OpenCode
// protocols needed to exercise real process and stream behavior.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	scenarioEnv  = "AGENTRA_RUNTIME_FIXTURE_SCENARIO"
	heartbeatEnv = "AGENTRA_RUNTIME_FIXTURE_HEARTBEAT"
	secret       = "sk-agentra-runtime-fixture-secret-1234567890"
)

func main() {
	if hasArg("--version") {
		fmt.Println("agentra-runtime-fixture 1.0.0")
		return
	}
	if hasArg("--fixture-child") {
		runHeartbeatChild()
		return
	}

	scenario := os.Getenv(scenarioEnv)
	if scenario == "" {
		scenario = "success"
	}
	if scenario == "exit_error" {
		fmt.Fprintf(os.Stderr, "fixture failed with token=%s\n", secret)
		os.Exit(17)
	}

	switch provider() {
	case "codex":
		runCodex(scenario)
	case "opencode":
		runOpenCode(scenario)
	default:
		runClaude(scenario)
	}
}

func provider() string {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "app-server":
			return "codex"
		case "run":
			return "opencode"
		}
	}
	return "claude"
}

func runClaude(scenario string) {
	fmt.Fprintf(os.Stderr, "fixture stderr token=%s\n", secret)
	startDescendantIfRequested(scenario)
	writeJSON(map[string]any{
		"type":       "system",
		"session_id": "fixture-claude-session",
	})

	if scenario == "hang" || scenario == "hang_with_child" {
		hang()
	}
	if scenario == "resume_miss" {
		writeJSON(map[string]any{
			"type":       "result",
			"is_error":   true,
			"result":     "session not found",
			"session_id": argValue("--resume"),
		})
		return
	}

	writeJSON(map[string]any{
		"type": "assistant",
		"message": map[string]any{
			"role": "assistant",
			"usage": map[string]any{
				"input_tokens": 101, "output_tokens": 11,
				"cache_read_input_tokens": 5, "cache_creation_input_tokens": 3,
			},
			"content": []map[string]any{
				{"type": "text", "text": "fixture output"},
			},
		},
	})
	writeJSON(map[string]any{
		"type":       "result",
		"result":     "fixture output",
		"session_id": "fixture-claude-session",
	})
}

func runOpenCode(scenario string) {
	fmt.Fprintf(os.Stderr, "fixture stderr token=%s\n", secret)
	startDescendantIfRequested(scenario)
	writeJSON(map[string]any{
		"type":      "step_start",
		"sessionID": "fixture-opencode-session",
	})

	if scenario == "hang" || scenario == "hang_with_child" {
		hang()
	}
	if scenario == "resume_miss" {
		writeJSON(map[string]any{
			"type":      "error",
			"sessionID": argValue("--session"),
			"error": map[string]any{
				"name": "SessionNotFoundError",
				"data": map[string]any{"message": "session not found"},
			},
		})
		return
	}

	writeJSON(map[string]any{
		"type":      "text",
		"sessionID": "fixture-opencode-session",
		"part":      map[string]any{"text": "fixture output"},
	})
	writeJSON(map[string]any{
		"type":      "step_finish",
		"sessionID": "fixture-opencode-session",
		"part": map[string]any{
			"tokens": map[string]any{
				"input": 202, "output": 22, "reasoning": 2,
				"cache": map[string]any{"read": 7, "write": 4},
			},
		},
	})
}

func runCodex(scenario string) {
	fmt.Fprintf(os.Stderr, "fixture stderr token=%s\n", secret)
	startDescendantIfRequested(scenario)

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var request struct {
			ID     int    `json:"id"`
			Method string `json:"method"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &request); err != nil {
			continue
		}

		switch request.Method {
		case "initialize":
			writeRPCResult(request.ID, map[string]any{})
		case "thread/start":
			writeRPCResult(request.ID, map[string]any{
				"thread": map[string]any{"id": "fixture-codex-thread"},
			})
		case "turn/start", "turn/continue":
			if scenario == "resume_miss" && request.Method == "turn/continue" {
				writeRPCError(request.ID, -32001, "session not found")
				return
			}

			writeRPCResult(request.ID, map[string]any{
				"turn": map[string]any{"id": "fixture-codex-turn"},
			})
			writeJSON(map[string]any{
				"jsonrpc": "2.0",
				"method":  "turn/started",
				"params": map[string]any{
					"turn": map[string]any{"id": "fixture-codex-turn", "status": "inProgress"},
				},
			})
			if scenario == "hang" || scenario == "hang_with_child" {
				hang()
			}
			writeJSON(map[string]any{
				"jsonrpc": "2.0",
				"method":  "item/completed",
				"params": map[string]any{
					"item": map[string]any{
						"id":    "fixture-message",
						"type":  "agentMessage",
						"phase": "final_answer",
						"text":  "fixture output",
					},
				},
			})
			writeJSON(map[string]any{
				"jsonrpc": "2.0",
				"method":  "thread/tokenUsage/updated",
				"params": map[string]any{
					"threadId": "fixture-codex-thread",
					"turnId":   "fixture-codex-turn",
					"tokenUsage": map[string]any{
						"total": map[string]any{
							"inputTokens": 303, "outputTokens": 33,
							"reasoningOutputTokens": 3, "cachedInputTokens": 9,
							"cacheWriteInputTokens": 6, "totalTokens": 339,
						},
						"last": map[string]any{
							"inputTokens": 303, "outputTokens": 33,
							"reasoningOutputTokens": 3, "cachedInputTokens": 9,
							"cacheWriteInputTokens": 6, "totalTokens": 339,
						},
					},
				},
			})
			writeJSON(map[string]any{
				"jsonrpc": "2.0",
				"method":  "turn/completed",
				"params": map[string]any{
					"turn": map[string]any{"id": "fixture-codex-turn", "status": "completed"},
				},
			})
		}
	}
}

func writeRPCResult(id int, result any) {
	writeJSON(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	})
}

func writeRPCError(id, code int, message string) {
	writeJSON(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	})
}

// writeJSON deliberately splits each JSON line across two writes. The adapter
// must tolerate partial pipe reads and wait for the newline-delimited frame.
func writeJSON(value any) {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	middle := len(data) / 2
	_, _ = os.Stdout.Write(data[:middle])
	time.Sleep(2 * time.Millisecond)
	_, _ = os.Stdout.Write(append(data[middle:], '\n'))
}

func hasArg(name string) bool {
	for _, arg := range os.Args[1:] {
		if arg == name {
			return true
		}
	}
	return false
}

func argValue(name string) string {
	for i, arg := range os.Args[1:] {
		if arg == name && i+2 < len(os.Args) {
			return strings.TrimSpace(os.Args[i+2])
		}
	}
	return ""
}

func startDescendantIfRequested(scenario string) {
	if scenario != "hang_with_child" {
		return
	}
	executable, err := os.Executable()
	if err != nil {
		panic(err)
	}
	child := exec.Command(executable, "--fixture-child")
	child.Env = os.Environ()
	child.Stdout = os.Stderr
	child.Stderr = os.Stderr
	if err := child.Start(); err != nil {
		panic(err)
	}
}

func runHeartbeatChild() {
	path := os.Getenv(heartbeatEnv)
	if path == "" {
		panic("runtime fixture heartbeat path is required")
	}
	for {
		value := fmt.Sprintf("%d", time.Now().UnixNano())
		if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
			panic(err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func hang() {
	for {
		time.Sleep(time.Hour)
	}
}
