package usage

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestParseCodexFile(t *testing.T) {
	// Create a temp directory structure: sessions/YYYY/MM/DD/file.jsonl
	tmp := t.TempDir()
	sessionsDir := filepath.Join(tmp, "sessions", "2026", "01", "14")
	if err := os.MkdirAll(sessionsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Real Codex JSONL format with turn_context and token_count events
	content := `{"timestamp":"2026-01-13T17:41:31.666Z","type":"turn_context","payload":{"cwd":"/tmp","model":"gpt-5.2-codex","effort":"high"}}
{"timestamp":"2026-01-13T17:41:32.916Z","type":"event_msg","payload":{"type":"token_count","info":null,"rate_limits":{"primary":{"used_percent":24.0}}}}
{"timestamp":"2026-01-13T17:44:06.217Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":328894,"cached_input_tokens":287872,"output_tokens":3071,"reasoning_output_tokens":960,"total_tokens":331965},"last_token_usage":{"input_tokens":24525,"cached_input_tokens":3200,"output_tokens":1815,"reasoning_output_tokens":960,"total_tokens":26340},"model_context_window":258400},"rate_limits":{"primary":{"used_percent":26.0}}}}
`

	filePath := filepath.Join(sessionsDir, "rollout-test.jsonl")
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewScanner(slog.Default())
	record := s.parseCodexFile(filePath)

	if record == nil {
		t.Fatal("expected non-nil record")
	}

	if record.Date != "2026-01-14" {
		t.Errorf("date = %q, want %q", record.Date, "2026-01-14")
	}
	if record.Provider != "codex" {
		t.Errorf("provider = %q, want %q", record.Provider, "codex")
	}
	if record.Model != "gpt-5.2-codex" {
		t.Errorf("model = %q, want %q", record.Model, "gpt-5.2-codex")
	}
	if record.InputTokens != 328894 {
		t.Errorf("input_tokens = %d, want %d", record.InputTokens, 328894)
	}
	// output_tokens + reasoning_output_tokens
	if record.OutputTokens != 3071+960 {
		t.Errorf("output_tokens = %d, want %d", record.OutputTokens, 3071+960)
	}
	if record.CacheReadTokens != 287872 {
		t.Errorf("cache_read_tokens = %d, want %d", record.CacheReadTokens, 287872)
	}
}

func TestParseCodexFile_NullInfo(t *testing.T) {
	// When all token_count events have info:null, should return nil
	tmp := t.TempDir()
	sessionsDir := filepath.Join(tmp, "sessions", "2026", "01", "14")
	if err := os.MkdirAll(sessionsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	content := `{"timestamp":"2026-01-13T17:41:32.916Z","type":"event_msg","payload":{"type":"token_count","info":null}}
`
	filePath := filepath.Join(sessionsDir, "rollout-test.jsonl")
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewScanner(slog.Default())
	record := s.parseCodexFile(filePath)

	if record != nil {
		t.Errorf("expected nil record for null info, got %+v", record)
	}
}

func TestParseCodexFile_LastTokenUsageFallback(t *testing.T) {
	// When total_token_usage is absent but last_token_usage exists
	tmp := t.TempDir()
	sessionsDir := filepath.Join(tmp, "sessions", "2026", "03", "27")
	if err := os.MkdirAll(sessionsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	content := `{"timestamp":"2026-03-27T10:00:00Z","type":"turn_context","payload":{"model":"gpt-5"}}
{"timestamp":"2026-03-27T10:01:00Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":1000,"cached_input_tokens":200,"output_tokens":500}}}}
`
	filePath := filepath.Join(sessionsDir, "rollout-test.jsonl")
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewScanner(slog.Default())
	record := s.parseCodexFile(filePath)

	if record == nil {
		t.Fatal("expected non-nil record")
	}
	if record.InputTokens != 1000 {
		t.Errorf("input_tokens = %d, want %d", record.InputTokens, 1000)
	}
	if record.OutputTokens != 500 {
		t.Errorf("output_tokens = %d, want %d", record.OutputTokens, 500)
	}
	if record.CacheReadTokens != 200 {
		t.Errorf("cache_read_tokens = %d, want %d", record.CacheReadTokens, 200)
	}
}

func TestParseCodexFile_CacheReadInputTokens(t *testing.T) {
	// Test the alternative field name cache_read_input_tokens
	tmp := t.TempDir()
	sessionsDir := filepath.Join(tmp, "sessions", "2026", "03", "27")
	if err := os.MkdirAll(sessionsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	content := `{"timestamp":"2026-03-27T10:00:00Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":5000,"cache_read_input_tokens":3000,"output_tokens":800},"model":"gpt-5.2-codex"}}}
`
	filePath := filepath.Join(sessionsDir, "rollout-test.jsonl")
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewScanner(slog.Default())
	record := s.parseCodexFile(filePath)

	if record == nil {
		t.Fatal("expected non-nil record")
	}
	if record.CacheReadTokens != 3000 {
		t.Errorf("cache_read_tokens = %d, want %d", record.CacheReadTokens, 3000)
	}
	if record.Model != "gpt-5.2-codex" {
		t.Errorf("model = %q, want %q", record.Model, "gpt-5.2-codex")
	}
}
