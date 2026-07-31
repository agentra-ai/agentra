package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "unknown"
)

var gitCmd = &cobra.Command{
	Use:   "git",
	Short: "Git integration commands",
	Long: `Git integration for Agentra.

Commands:
  hooks    Install or uninstall git hooks
`,
}

var rootCmd = &cobra.Command{
	Use:           "agentra",
	Short:         "Agentra CLI — local agent runtime and management tool",
	Long:          "agentra manages local agent runtimes and provides control commands for the Agentra platform.",
	SilenceUsage:  false,
	SilenceErrors: true,
	Args:          cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return cmd.Help()
	},
}

func init() {
	rootCmd.PersistentFlags().String("server-url", "", "Agentra server URL (env: AGENTRA_SERVER_URL)")
	rootCmd.PersistentFlags().String("workspace-id", "", "Workspace ID (env: AGENTRA_WORKSPACE_ID)")
	rootCmd.PersistentFlags().String("profile", "", "Configuration profile name (e.g. dev) — isolates config, daemon state, and workspaces")

	rootCmd.AddCommand(setupCmd)
	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(daemonCmd)
	rootCmd.AddCommand(agentCmd)
	rootCmd.AddCommand(workspaceCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(issueCmd)
	rootCmd.AddCommand(attachmentCmd)
	rootCmd.AddCommand(repoCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(skillCmd)
	rootCmd.AddCommand(runtimeCmd)
	rootCmd.AddCommand(gitCmd)
	rootCmd.AddCommand(loopCmd)
	rootCmd.AddCommand(conventionsCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
