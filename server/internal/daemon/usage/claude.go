package usage

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// scanClaude reads Claude Code JSONL session logs from ~/.config/claude/projects/**/*.jsonl
// and extracts token usage from "assistant" message lines.
func (s *Scanner) scanClaude() []Record {
	roots := claudeLogRoots()
	if len(roots) == 0 {
		return nil
	}

	var allRecords []Record
	seen := make(map[string]bool) // dedup by "messageId:requestId"

	for _, root := range roots {
		files, err := filepath.Glob(filepath.Join(root, "**", "*.jsonl"))
		if err != nil {
			s.logger.Debug("claude glob error", "root", root, "error", err)
			continue
		}
		// Also glob one level deeper for subagent logs
		deeper, _ := filepath.Glob(filepath.Join(root, "**", "**", "*.jsonl"))
		files = append(files, deeper...)

		for _, f := range files {
			records := s.parseClaudeFile(f, seen)
			allRecords = append(allRecords, records...)
		}
	}

	return mergeRecords(allRecords)
}

// claudeLogRoots returns the directories to scan for Claude JSONL logs.
func claudeLogRoots() []string {
	var roots []string

	// Check CLAUDE_CONFIG_DIR env var
	if configDir := os.Getenv("CLAUDE_CONFIG_DIR"); configDir != "" {
		for _, dir := range strings.Split(configDir, ",") {
			dir = strings.TrimSpace(dir)
			if dir != "" {
				roots = append(roots, filepath.Join(dir, "projects"))
			}
		}
	}

	// Standard locations
	home, err := os.UserHomeDir()
	if err != nil {
		return roots
	}

	candidates := []string{
		filepath.Join(home, ".config", "claude", "projects"),
		filepath.Join(home, ".claude", "projects"),
	}
	for _, dir := range candidates {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			roots = append(roots, dir)
		}
	}

	return roots
}

// claudeLine represents the subset of a Claude JSONL line we care about.
type claudeLine struct {
	Type      string    `json:"type"`
	Timestamp string    `json:"timestamp"`
	RequestID string    `json:"requestId"`
	Message   *struct {
		ID    string `json:"id"`
		Model string `json:"model"`
		Usage *struct {
			InputTokens              int64 `json:"input_tokens"`
			OutputTokens             int64 `json:"output_tokens"`
			CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
			CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

func (s *Scanner) parseClaudeFile(path string, seen map[string]bool) []Record {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var records []Record
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024) // up to 1MB lines

	for scanner.Scan() {
		line := scanner.Bytes()

		// Fast pre-filter: skip lines that can't contain what we need
		if !bytesContains(line, `"type":"assistant"`) && !bytesContains(line, `"type": "assistant"`) {
			continue
		}
		if !bytesContains(line, `"usage"`) {
			continue
		}

		var entry claudeLine
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}
		if entry.Type != "assistant" || entry.Message == nil || entry.Message.Usage == nil {
			continue
		}

		// Dedup: Claude streaming produces multiple lines with same message.id + requestId
		// with cumulative token counts. Take only the first occurrence.
		dedupKey := entry.Message.ID + ":" + entry.RequestID
		if dedupKey != ":" && seen[dedupKey] {
			continue
		}
		if dedupKey != ":" {
			seen[dedupKey] = true
		}

		// Parse timestamp to get date
		ts, err := time.Parse(time.RFC3339Nano, entry.Timestamp)
		if err != nil {
			ts, err = time.Parse(time.RFC3339, entry.Timestamp)
			if err != nil {
				continue
			}
		}

		model := entry.Message.Model
		if model == "" {
			model = "unknown"
		}

		records = append(records, Record{
			Date:             ts.Local().Format("2006-01-02"),
			Provider:         "claude",
			Model:            normalizeClaudeModel(model),
			InputTokens:      entry.Message.Usage.InputTokens,
			OutputTokens:     entry.Message.Usage.OutputTokens,
			CacheReadTokens:  entry.Message.Usage.CacheReadInputTokens,
			CacheWriteTokens: entry.Message.Usage.CacheCreationInputTokens,
		})
	}

	return records
}

// normalizeClaudeModel strips common prefixes/suffixes from model names.
func normalizeClaudeModel(model string) string {
	// Strip "anthropic." prefix
	model = strings.TrimPrefix(model, "anthropic.")
	// Strip Vertex AI prefixes like "us.anthropic."
	if idx := strings.LastIndex(model, "anthropic."); idx >= 0 {
		model = model[idx+len("anthropic."):]
	}
	return model
}

func bytesContains(data []byte, substr string) bool {
	return strings.Contains(string(data), substr)
}
