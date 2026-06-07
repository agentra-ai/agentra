package main

import (
	"net/url"
	"testing"

	"github.com/spf13/cobra"
)

func TestFormatLoopStatus(t *testing.T) {
	tests := []struct {
		name string
		loop map[string]any
		want string
	}{
		{
			name: "missing status",
			loop: map[string]any{},
			want: "-",
		},
		{
			name: "nil status",
			loop: map[string]any{"status": nil},
			want: "-",
		},
		{
			name: "running status",
			loop: map[string]any{"status": "running"},
			want: "running",
		},
		{
			name: "done status",
			loop: map[string]any{"status": "done"},
			want: "done",
		},
		{
			name: "failed with reason",
			loop: map[string]any{
				"status":         "failed",
				"failure_reason": "max_iterations_exceeded",
			},
			want: "failed (max_iterations_exceeded)",
		},
		{
			name: "cancelled with reason",
			loop: map[string]any{
				"status":         "cancelled",
				"failure_reason": "user_requested",
			},
			want: "cancelled (user_requested)",
		},
		{
			name: "empty failure reason ignored",
			loop: map[string]any{
				"status":         "done",
				"failure_reason": "",
			},
			want: "done",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatLoopStatus(tt.loop)
			if got != tt.want {
				t.Errorf("formatLoopStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatLoopStage(t *testing.T) {
	tests := []struct {
		name string
		loop map[string]any
		want string
	}{
		{
			name: "missing stage",
			loop: map[string]any{},
			want: "-",
		},
		{
			name: "nil stage",
			loop: map[string]any{"current_stage": nil},
			want: "-",
		},
		{
			name: "plan stage",
			loop: map[string]any{"current_stage": "plan"},
			want: "plan",
		},
		{
			name: "develop stage",
			loop: map[string]any{"current_stage": "develop"},
			want: "develop",
		},
		{
			name: "review stage",
			loop: map[string]any{"current_stage": "review"},
			want: "review",
		},
		{
			name: "fix stage",
			loop: map[string]any{"current_stage": "fix"},
			want: "fix",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatLoopStage(tt.loop)
			if got != tt.want {
				t.Errorf("formatLoopStage() = %q, want %q", got, tt.want)
			}
		})
	}
}

// freshLoopListCmd returns a fresh loop list command (with flags) so each
// subtest gets an isolated flag set.
func freshLoopListCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "list"}
	cmd.Flags().String("output", "table", "")
	cmd.Flags().String("status", "", "")
	cmd.Flags().String("issue-id", "", "")
	cmd.Flags().Int("limit", 50, "")
	return cmd
}

func TestLoopListCmd_BuildsQuery(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		workspace string
		wantPairs [][2]string
	}{
		{
			name:      "no filters uses defaults",
			args:      []string{},
			workspace: "ws-1",
			wantPairs: [][2]string{
				{"workspace_id", "ws-1"},
				{"limit", "50"},
			},
		},
		{
			name:      "with status filter",
			args:      []string{"--status", "running"},
			workspace: "ws-1",
			wantPairs: [][2]string{
				{"workspace_id", "ws-1"},
				{"status", "running"},
				{"limit", "50"},
			},
		},
		{
			name:      "with issue-id filter",
			args:      []string{"--issue-id", "issue-42"},
			workspace: "ws-1",
			wantPairs: [][2]string{
				{"workspace_id", "ws-1"},
				{"issue_id", "issue-42"},
				{"limit", "50"},
			},
		},
		{
			name:      "with limit override",
			args:      []string{"--limit", "10"},
			workspace: "ws-1",
			wantPairs: [][2]string{
				{"workspace_id", "ws-1"},
				{"limit", "10"},
			},
		},
		{
			name: "all filters",
			args: []string{
				"--status", "failed",
				"--issue-id", "issue-99",
				"--limit", "5",
			},
			workspace: "ws-2",
			wantPairs: [][2]string{
				{"workspace_id", "ws-2"},
				{"status", "failed"},
				{"issue_id", "issue-99"},
				{"limit", "5"},
			},
		},
		{
			name:      "no workspace omits workspace_id",
			args:      []string{"--status", "running"},
			workspace: "",
			wantPairs: [][2]string{
				{"status", "running"},
				{"limit", "50"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := freshLoopListCmd()
			if err := cmd.ParseFlags(tt.args); err != nil {
				t.Fatalf("ParseFlags(%v) error = %v", tt.args, err)
			}

			got := buildLoopListQuery(cmd, tt.workspace)

			if len(tt.wantPairs) == 0 {
				if got != "" {
					t.Fatalf("expected empty query, got %q", got)
				}
				return
			}

			values, err := url.ParseQuery(got)
			if err != nil {
				t.Fatalf("ParseQuery(%q) error = %v", got, err)
			}

			if len(values) != len(tt.wantPairs) {
				t.Fatalf("got %d params, want %d (params=%v)", len(values), len(tt.wantPairs), values)
			}

			for _, pair := range tt.wantPairs {
				if got := values.Get(pair[0]); got != pair[1] {
					t.Errorf("param %q = %q, want %q", pair[0], got, pair[1])
				}
			}
		})
	}
}

func TestLoopListCmd_LimitDefaultIncluded(t *testing.T) {
	// The CLI default of 50 is always sent (matches the issue list pattern).
	cmd := freshLoopListCmd()
	if err := cmd.ParseFlags([]string{}); err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	got := buildLoopListQuery(cmd, "ws-1")
	values, err := url.ParseQuery(got)
	if err != nil {
		t.Fatalf("ParseQuery: %v", err)
	}
	if got := values.Get("limit"); got != "50" {
		t.Errorf("limit = %q, want %q", got, "50")
	}
}
