package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/agentra-ai/agentra/server/internal/cli"
	"github.com/agentra-ai/agentra/server/internal/doctor"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose CLI, server, runtime, repository, and realtime connectivity",
	Long: `Run bounded, read-only diagnostics for the active Agentra profile.

Checks cover configuration security, Web and API reachability, readiness and
object storage, authentication, workspace membership, agent runtime CLIs,
daemon workspace permissions, Git remote access, the local daemon, and an
authenticated WebSocket ping/pong.`,
	Args: cobra.NoArgs,
	RunE: runDoctor,
}

func init() {
	doctorCmd.Flags().String("output", "table", "Output format: table or json")
	doctorCmd.Flags().Duration("timeout", 5*time.Second, "Timeout for each external check")
	doctorCmd.Flags().String("repo", ".", "Repository path to diagnose")
	doctorCmd.Flags().Bool("skip-repo-remote", false, "Skip non-interactive origin connectivity check")
}

func runDoctor(cmd *cobra.Command, _ []string) error {
	output, _ := cmd.Flags().GetString("output")
	if output != "table" && output != "json" {
		return fmt.Errorf("invalid output format %q (supported: table, json)", output)
	}
	timeout, _ := cmd.Flags().GetDuration("timeout")
	if timeout <= 0 {
		return fmt.Errorf("timeout must be greater than zero")
	}
	repoPath, _ := cmd.Flags().GetString("repo")
	skipRepoRemote, _ := cmd.Flags().GetBool("skip-repo-remote")
	profile := resolveProfile(cmd)
	_, configErr := cli.LoadCLIConfigForProfile(profile)
	configPath, _ := cli.CLIConfigPathForProfile(profile)
	workspacesRoot, rootErr := doctorWorkspacesRoot(profile)

	report := doctor.Run(cmd.Context(), doctor.Options{
		ServerURL:           resolveServerURL(cmd),
		AppURL:              resolveAppURL(cmd),
		WorkspaceID:         resolveWorkspaceID(cmd),
		Token:               resolveToken(cmd),
		Profile:             profile,
		ConfigPath:          configPath,
		ConfigError:         configErr,
		RepoPath:            repoPath,
		SkipRepoRemote:      skipRepoRemote,
		WorkspacesRoot:      workspacesRoot,
		WorkspacesRootError: rootErr,
		DaemonURL:           cli.ResolveLocalDaemonBaseURL(strconv.Itoa(healthPortForProfile(profile))),
		Timeout:             timeout,
	})

	if output == "json" {
		if err := cli.PrintJSON(os.Stdout, report); err != nil {
			return err
		}
	} else {
		printDoctorReport(report)
	}
	if report.Status == doctor.StatusFail {
		cmd.Root().SilenceUsage = true
		return fmt.Errorf("doctor found %d failing check(s)", report.Summary.Failed)
	}
	return nil
}

func printDoctorReport(report doctor.Report) {
	rows := make([][]string, 0, len(report.Checks))
	for _, check := range report.Checks {
		rows = append(rows, []string{doctorStatusLabel(check.Status), check.ID, check.Summary})
	}
	cli.PrintTable(os.Stdout, []string{"STATUS", "CHECK", "RESULT"}, rows)

	for _, check := range report.Checks {
		if check.Remediation != "" && check.Status != doctor.StatusPass {
			fmt.Fprintf(os.Stdout, "\nFix %s: %s\n", check.ID, check.Remediation)
		}
	}
	fmt.Fprintf(
		os.Stdout,
		"\nOverall: %s (%d passed, %d warnings, %d failed, %d skipped)\n",
		strings.ToUpper(string(report.Status)),
		report.Summary.Passed,
		report.Summary.Warnings,
		report.Summary.Failed,
		report.Summary.Skipped,
	)
}

func doctorStatusLabel(status doctor.Status) string {
	switch status {
	case doctor.StatusPass:
		return "PASS"
	case doctor.StatusWarning:
		return "WARN"
	case doctor.StatusFail:
		return "FAIL"
	default:
		return "SKIP"
	}
}

func doctorWorkspacesRoot(profile string) (string, error) {
	root := strings.TrimSpace(os.Getenv("AGENTRA_WORKSPACES_ROOT"))
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve daemon workspace root: %w", err)
		}
		name := "agentra_workspaces"
		if profile != "" {
			name += "_" + profile
		}
		root = filepath.Join(home, name)
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve daemon workspace root: %w", err)
	}
	return absolute, nil
}
