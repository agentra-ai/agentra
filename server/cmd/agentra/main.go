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

var rootCmd = &cobra.Command{
	Use:   "agentra",
	Short: "Agentra CLI — local agent runtime and management tool",
	Long:  "agentra manages local agent runtimes and provides control commands for the Agentra platform.",
	SilenceUsage:  false,
	SilenceErrors: true,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintln(os.Stderr, "Agentra CLI - AI-Native Task Management")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "用法:")
		fmt.Fprintln(os.Stderr, "  agentra [command]")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "可用命令:")
		fmt.Fprintln(os.Stderr, "  login         登录到 Agentra")
		fmt.Fprintln(os.Stderr, "  daemon        管理本地 Agent 守护进程")
		fmt.Fprintln(os.Stderr, "  workspace     管理工作区")
		fmt.Fprintln(os.Stderr, "  issue         管理 Issue")
		fmt.Fprintln(os.Stderr, "  agent         管理 Agent")
		fmt.Fprintln(os.Stderr, "  repo          管理代码仓库")
		fmt.Fprintln(os.Stderr, "  config        查看和修改配置")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "快速开始:")
		fmt.Fprintln(os.Stderr, "  1. agentra login            # 登录到 Agentra")
		fmt.Fprintln(os.Stderr, "  2. agentra daemon start     # 启动本地 Agent 守护进程")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "查看帮助:")
		fmt.Fprintln(os.Stderr, "  agentra help [command]      # 查看特定命令的帮助")
	},
}

func init() {
	rootCmd.PersistentFlags().String("server-url", "", "Agentra server URL (env: AGENTRA_SERVER_URL)")
	rootCmd.PersistentFlags().String("workspace-id", "", "Workspace ID (env: AGENTRA_WORKSPACE_ID)")
	rootCmd.PersistentFlags().String("profile", "", "Configuration profile name (e.g. dev) — isolates config, daemon state, and workspaces")

	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(daemonCmd)
	rootCmd.AddCommand(agentCmd)
	rootCmd.AddCommand(workspaceCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(issueCmd)
	rootCmd.AddCommand(attachmentCmd)
	rootCmd.AddCommand(repoCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(skillCmd)
	rootCmd.AddCommand(runtimeCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
