package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/agentra-ai/agentra/server/internal/cli"
)

var loopCmd = &cobra.Command{
	Use:   "loop",
	Short: "Manage Agentic Engineering Loops",
}

var loopStartCmd = &cobra.Command{
	Use:   "start <issue-id>",
	Short: "Start a new agentic engineering loop on an issue",
	Args:  cobra.ExactArgs(1),
	RunE:  runLoopStart,
}

var loopStatusCmd = &cobra.Command{
	Use:   "status <loop-id>",
	Short: "Get the status of a loop",
	Args:  cobra.ExactArgs(1),
	RunE:  runLoopStatus,
}

var loopPauseCmd = &cobra.Command{
	Use:   "pause <loop-id>",
	Short: "Pause a running loop",
	Args:  cobra.ExactArgs(1),
	RunE:  runLoopPause,
}

var loopResumeCmd = &cobra.Command{
	Use:   "resume <loop-id>",
	Short: "Resume a paused loop",
	Args:  cobra.ExactArgs(1),
	RunE:  runLoopResume,
}

var loopCancelCmd = &cobra.Command{
	Use:   "cancel <loop-id>",
	Short: "Cancel a loop",
	Args:  cobra.ExactArgs(1),
	RunE:  runLoopCancel,
}

var loopListCmd = &cobra.Command{
	Use:   "list",
	Short: "List loops in the workspace",
	RunE:  runLoopList,
}

func init() {
	loopCmd.AddCommand(loopStartCmd)
	loopCmd.AddCommand(loopStatusCmd)
	loopCmd.AddCommand(loopPauseCmd)
	loopCmd.AddCommand(loopResumeCmd)
	loopCmd.AddCommand(loopCancelCmd)
	loopCmd.AddCommand(loopListCmd)

	// loop start
	loopStartCmd.Flags().Int("max-iterations", 5, "Maximum fix iterations before failing the loop")
	loopStartCmd.Flags().String("agent", "", "Agent ID to use for all stages of the loop (fallback when --stages omits a stage)")
	loopStartCmd.Flags().StringSlice("stages", nil,
		"Per-stage agent overrides. Repeat or comma-separate, e.g. --stages plan=AGENT-UUID,develop=AGENT-UUID. Valid stages: plan, develop, review, fix.")
	loopStartCmd.Flags().String("output", "json", "Output format: table or json")

	// loop status
	loopStatusCmd.Flags().String("output", "table", "Output format: table or json")

	// loop pause/resume/cancel
	loopPauseCmd.Flags().String("output", "table", "Output format: table or json")
	loopResumeCmd.Flags().String("output", "table", "Output format: table or json")
	loopCancelCmd.Flags().String("output", "table", "Output format: table or json")

	// loop list
	loopListCmd.Flags().String("output", "table", "Output format: table or json")
	loopListCmd.Flags().String("status", "", "Filter by status (pending, running, paused, done, failed, cancelled)")
	loopListCmd.Flags().String("issue-id", "", "Filter by issue ID")
	loopListCmd.Flags().Int("limit", 50, "Maximum number of loops to return")
}

// ---------------------------------------------------------------------------
// Loop commands
// ---------------------------------------------------------------------------

func runLoopStart(cmd *cobra.Command, args []string) error {
	if err := requireAuth(cmd); err != nil {
		return err
	}
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	body := map[string]any{"issue_id": args[0]}
	if maxIter, _ := cmd.Flags().GetInt("max-iterations"); maxIter > 0 {
		body["max_iterations"] = maxIter
	}
	if agent, _ := cmd.Flags().GetString("agent"); agent != "" {
		body["agent_id"] = agent
	}
	stagesRaw, _ := cmd.Flags().GetStringSlice("stages")
	if len(stagesRaw) > 0 {
		stageAgents, err := parseStageAgents(stagesRaw)
		if err != nil {
			return err
		}
		body["stage_agents"] = stageAgents
	}

	var result map[string]any
	if err := client.PostJSON(ctx, "/api/loops", body, &result); err != nil {
		return fmt.Errorf("start loop: %w", err)
	}

	loopID := strVal(result, "id")
	fmt.Fprintf(os.Stderr, "started loop %s on issue %s\n", loopID, args[0])

	output, _ := cmd.Flags().GetString("output")
	if output == "json" {
		return cli.PrintJSON(os.Stdout, result)
	}
	return nil
}

func runLoopStatus(cmd *cobra.Command, args []string) error {
	if err := requireAuth(cmd); err != nil {
		return err
	}
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var loop map[string]any
	if err := client.GetJSON(ctx, "/api/loops/"+args[0], &loop); err != nil {
		return fmt.Errorf("get loop: %w", err)
	}

	output, _ := cmd.Flags().GetString("output")
	if output == "json" {
		return cli.PrintJSON(os.Stdout, loop)
	}

	headers := []string{"ID", "STATUS", "STAGE", "ITERATION", "PR_URL"}
	rows := [][]string{{
		truncateID(strVal(loop, "id")),
		formatLoopStatus(loop),
		formatLoopStage(loop),
		strVal(loop, "iteration"),
		strVal(loop, "pr_url"),
	}}
	cli.PrintTable(os.Stdout, headers, rows)
	return nil
}

func runLoopPause(cmd *cobra.Command, args []string) error {
	return runLoopTransition(cmd, args[0], "pause", "paused", "/api/loops/"+args[0]+"/pause")
}

func runLoopResume(cmd *cobra.Command, args []string) error {
	return runLoopTransition(cmd, args[0], "resume", "resumed", "/api/loops/"+args[0]+"/resume")
}

func runLoopCancel(cmd *cobra.Command, args []string) error {
	return runLoopTransition(cmd, args[0], "cancel", "cancelled", "/api/loops/"+args[0]+"/cancel")
}

func runLoopTransition(cmd *cobra.Command, loopID, verb, pastTense, path string) error {
	if err := requireAuth(cmd); err != nil {
		return err
	}
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var result map[string]any
	if err := client.PostJSON(ctx, path, nil, &result); err != nil {
		return fmt.Errorf("%s loop: %w", verb, err)
	}

	fmt.Fprintf(os.Stderr, "%s loop %s\n", pastTense, loopID)

	output, _ := cmd.Flags().GetString("output")
	if output == "json" {
		return cli.PrintJSON(os.Stdout, result)
	}
	return nil
}

func runLoopList(cmd *cobra.Command, _ []string) error {
	if err := requireAuth(cmd); err != nil {
		return err
	}
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	path := "/api/loops"
	if query := buildLoopListQuery(cmd, client.WorkspaceID); query != "" {
		path += "?" + query
	}

	var result map[string]any
	if err := client.GetJSON(ctx, path, &result); err != nil {
		return fmt.Errorf("list loops: %w", err)
	}

	loopsRaw, _ := result["loops"].([]any)

	output, _ := cmd.Flags().GetString("output")
	if output == "json" {
		return cli.PrintJSON(os.Stdout, loopsRaw)
	}

	headers := []string{"ID", "ISSUE", "STATUS", "STAGE", "ITERATION", "PR_URL"}
	rows := make([][]string, 0, len(loopsRaw))
	for _, raw := range loopsRaw {
		loop, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		rows = append(rows, []string{
			truncateID(strVal(loop, "id")),
			truncateID(strVal(loop, "issue_id")),
			formatLoopStatus(loop),
			formatLoopStage(loop),
			strVal(loop, "iteration"),
			strVal(loop, "pr_url"),
		})
	}
	cli.PrintTable(os.Stdout, headers, rows)
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// formatLoopStatus returns a human-readable status. When a loop failed or was
// cancelled, the failure reason (if present) is appended in parentheses.
func formatLoopStatus(loop map[string]any) string {
	status := strVal(loop, "status")
	if status == "" {
		return "-"
	}
	if reason := strVal(loop, "failure_reason"); reason != "" {
		return fmt.Sprintf("%s (%s)", status, reason)
	}
	return status
}

// formatLoopStage returns the current stage of a loop, or "-" when unset
// (e.g. for a brand-new loop that hasn't picked a stage yet).
func formatLoopStage(loop map[string]any) string {
	stage := strVal(loop, "current_stage")
	if stage == "" {
		return "-"
	}
	return stage
}

// buildLoopListQuery assembles the query string for GET /api/loops from the
// command's flag values. It is exported-style for testability — see
// cmd_loop_test.go.
func buildLoopListQuery(cmd *cobra.Command, workspaceID string) string {
	params := url.Values{}
	if workspaceID != "" {
		params.Set("workspace_id", workspaceID)
	}
	if v, _ := cmd.Flags().GetString("status"); v != "" {
		params.Set("status", v)
	}
	if v, _ := cmd.Flags().GetString("issue-id"); v != "" {
		params.Set("issue_id", v)
	}
	if v, _ := cmd.Flags().GetInt("limit"); v > 0 {
		params.Set("limit", fmt.Sprintf("%d", v))
	}
	return params.Encode()
}

// validLoopStages is the closed set of stages the loop coordinator knows
// about. Anything else from --stages is rejected client-side so users see
// a friendly error before the request goes over the wire.
var validLoopStages = map[string]struct{}{
	"plan": {}, "develop": {}, "review": {}, "fix": {},
}

// parseStageAgents turns the raw --stages flag values (one per repetition
// OR comma-separated) into a stage→agent map. Each entry must be
// "stage=agent-id"; duplicates and unknown stages are rejected so the
// loop doesn't silently start with a misconfigured pipeline.
func parseStageAgents(raw []string) (map[string]string, error) {
	out := map[string]string{}
	for _, entry := range raw {
		for _, kv := range strings.Split(entry, ",") {
			kv = strings.TrimSpace(kv)
			if kv == "" {
				continue
			}
			eq := strings.IndexByte(kv, '=')
			if eq <= 0 || eq == len(kv)-1 {
				return nil, fmt.Errorf("--stages: expected stage=agent-id, got %q", kv)
			}
			stage := strings.TrimSpace(kv[:eq])
			agent := strings.TrimSpace(kv[eq+1:])
			if _, ok := validLoopStages[stage]; !ok {
				return nil, fmt.Errorf("--stages: unknown stage %q (valid: plan, develop, review, fix)", stage)
			}
			if _, dup := out[stage]; dup {
				return nil, fmt.Errorf("--stages: stage %q specified more than once", stage)
			}
			out[stage] = agent
		}
	}
	return out, nil
}
