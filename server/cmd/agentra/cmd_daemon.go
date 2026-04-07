package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/agentra-ai/agentra/server/internal/cli"
	"github.com/agentra-ai/agentra/server/internal/daemon"
	logger_pkg "github.com/agentra-ai/agentra/server/internal/logger"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Manage the local agent runtime daemon",
}

var daemonStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the local agent runtime daemon",
	Long:  "Start the daemon process that polls for tasks and executes them using local agent CLIs (Claude, Codex).\nRuns in the background by default. Use --foreground to run in the current terminal.",
	RunE:  runDaemonStart,
}

var daemonStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the running daemon",
	RunE:  runDaemonStop,
}

var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show daemon status",
	RunE:  runDaemonStatus,
}

var daemonLogsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Show daemon logs",
	RunE:  runDaemonLogs,
}

func init() {
	f := daemonStartCmd.Flags()
	f.Bool("foreground", false, "Run in the foreground instead of background")
	f.String("daemon-id", "", "Unique daemon identifier (env: AGENTRA_DAEMON_ID)")
	f.String("device-name", "", "Human-readable device name (env: AGENTRA_DAEMON_DEVICE_NAME)")
	f.String("runtime-name", "", "Runtime display name (env: AGENTRA_AGENT_RUNTIME_NAME)")
	f.Duration("poll-interval", 0, "Task poll interval (env: AGENTRA_DAEMON_POLL_INTERVAL)")
	f.Duration("heartbeat-interval", 0, "Heartbeat interval (env: AGENTRA_DAEMON_HEARTBEAT_INTERVAL)")
	f.Duration("agent-timeout", 0, "Per-task timeout (env: AGENTRA_AGENT_TIMEOUT)")
	f.Int("max-concurrent-tasks", 0, "Max tasks running in parallel (env: AGENTRA_DAEMON_MAX_CONCURRENT_TASKS)")

	daemonLogsCmd.Flags().BoolP("follow", "f", false, "Follow log output")
	daemonLogsCmd.Flags().IntP("lines", "n", 50, "Number of lines to show")

	daemonStatusCmd.Flags().String("output", "table", "Output format: table or json")

	daemonCmd.AddCommand(daemonStartCmd)
	daemonCmd.AddCommand(daemonStopCmd)
	daemonCmd.AddCommand(daemonStatusCmd)
	daemonCmd.AddCommand(daemonLogsCmd)
}

// daemonDirForProfile returns the state directory for the given profile.
// Empty profile → ~/.agentra/, named profile → ~/.agentra/profiles/<name>/.
func daemonDirForProfile(profile string) string {
	dir, err := cli.ProfileDir(profile)
	if err != nil {
		return ""
	}
	return dir
}

func daemonPIDPathForProfile(profile string) string {
	return daemonDirForProfile(profile) + "/daemon.pid"
}

func daemonLogPathForProfile(profile string) string {
	return daemonDirForProfile(profile) + "/daemon.log"
}

// healthPortForProfile returns the health check port for the given profile.
// Default profile uses the standard port (19514). Named profiles get a
// deterministic offset derived from the profile name.
func healthPortForProfile(profile string) int {
	if profile == "" {
		return daemon.DefaultHealthPort
	}
	// Simple hash: sum of bytes mod 1000, offset from base+1.
	var h int
	for _, b := range []byte(profile) {
		h += int(b)
	}
	return daemon.DefaultHealthPort + 1 + (h % 1000)
}

// --- daemon start ---

func runDaemonStart(cmd *cobra.Command, _ []string) error {
	foreground, _ := cmd.Flags().GetBool("foreground")
	if foreground {
		return runDaemonForeground(cmd)
	}
	return runDaemonBackground(cmd)
}

func runDaemonBackground(cmd *cobra.Command) error {
	profile := resolveProfile(cmd)
	healthPort := healthPortForProfile(profile)
	desiredServerURL := resolveServerURL(cmd)

	// Check if daemon is already running.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	health := checkDaemonHealthOnPort(ctx, healthPort)
	if health["status"] == "running" {
		if shouldReplaceLegacyDaemon(health) {
			pidText := daemonPIDText(health)
			if pidText == "" {
				pidText = "unknown"
			}
			fmt.Fprintf(os.Stderr, "Stopping legacy multica daemon on port %d (pid %s) before starting agentra...\n", healthPort, pidText)
			if err := stopRunningDaemon(healthPort, health); err != nil {
				return err
			}
		} else if err := daemonStartConflictError(profile, health, desiredServerURL); err != nil {
			return err
		} else {
			label := "daemon"
			if profile != "" {
				label = fmt.Sprintf("daemon [%s]", profile)
			}
			return fmt.Errorf("%s is already running (pid %v)", label, health["pid"])
		}
	}

	// Resolve current executable.
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable path: %w", err)
	}

	// Build child args: daemon start --foreground + forwarded flags.
	args := buildDaemonStartArgs(cmd)

	// Ensure daemon directory exists.
	dir := daemonDirForProfile(profile)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create daemon directory: %w", err)
	}

	logPath := daemonLogPathForProfile(profile)
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open log file %s: %w", logPath, err)
	}

	child := exec.Command(exePath, args...)
	child.Stdout = logFile
	child.Stderr = logFile
	child.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := child.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("start daemon: %w", err)
	}
	childPID := child.Process.Pid
	logFile.Close()

	// Detach: we don't Wait() on the child — it runs independently.
	child.Process.Release()

	// Write PID file.
	pidPath := daemonPIDPathForProfile(profile)
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(childPID)), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not write PID file: %v\n", err)
	}

	// Give the daemon enough time to authenticate, register runtimes,
	// and warm any repo cache before declaring startup failed.
	health = waitForDaemonHealth(healthPort, 20*time.Second)
	if health["status"] != "running" {
		fmt.Fprintf(os.Stderr, "Daemon may not have started successfully. Check logs:\n  %s\n", logPath)
		return nil
	}

	if profile != "" {
		fmt.Fprintf(os.Stderr, "Daemon [%s] started (pid %d, version %s)\n", profile, childPID, version)
	} else {
		fmt.Fprintf(os.Stderr, "Daemon started (pid %d, version %s)\n", childPID, version)
	}
	fmt.Fprintf(os.Stderr, "Logs: %s\n", logPath)
	return nil
}

// buildDaemonStartArgs constructs args for the background child process.
func buildDaemonStartArgs(cmd *cobra.Command) []string {
	args := []string{"daemon", "start", "--foreground"}

	if v := flagString(cmd, "daemon-id"); v != "" {
		args = append(args, "--daemon-id", v)
	}
	if v := flagString(cmd, "device-name"); v != "" {
		args = append(args, "--device-name", v)
	}
	if v := flagString(cmd, "runtime-name"); v != "" {
		args = append(args, "--runtime-name", v)
	}
	if d, _ := cmd.Flags().GetDuration("poll-interval"); d > 0 {
		args = append(args, "--poll-interval", d.String())
	}
	if d, _ := cmd.Flags().GetDuration("heartbeat-interval"); d > 0 {
		args = append(args, "--heartbeat-interval", d.String())
	}
	if d, _ := cmd.Flags().GetDuration("agent-timeout"); d > 0 {
		args = append(args, "--agent-timeout", d.String())
	}
	if n, _ := cmd.Flags().GetInt("max-concurrent-tasks"); n > 0 {
		args = append(args, "--max-concurrent-tasks", strconv.Itoa(n))
	}

	// Forward global persistent flags.
	if v, _ := cmd.Flags().GetString("server-url"); v != "" {
		args = append(args, "--server-url", v)
	}
	if v := resolveProfile(cmd); v != "" {
		args = append(args, "--profile", v)
	}

	return args
}

func runDaemonForeground(cmd *cobra.Command) error {
	profile := resolveProfile(cmd)

	serverURL := cli.FlagOrEnv(cmd, "server-url", "AGENTRA_SERVER_URL", "")
	if serverURL == "" {
		if c, err := cli.LoadCLIConfigForProfile(profile); err == nil && c.ServerURL != "" {
			serverURL = c.ServerURL
		}
	}
	overrides := daemon.Overrides{
		ServerURL:   serverURL,
		DaemonID:    flagString(cmd, "daemon-id"),
		DeviceName:  flagString(cmd, "device-name"),
		RuntimeName: flagString(cmd, "runtime-name"),
		Profile:     profile,
		HealthPort:  healthPortForProfile(profile),
	}
	if d, _ := cmd.Flags().GetDuration("poll-interval"); d > 0 {
		overrides.PollInterval = d
	}
	if d, _ := cmd.Flags().GetDuration("heartbeat-interval"); d > 0 {
		overrides.HeartbeatInterval = d
	}
	if d, _ := cmd.Flags().GetDuration("agent-timeout"); d > 0 {
		overrides.AgentTimeout = d
	}
	if n, _ := cmd.Flags().GetInt("max-concurrent-tasks"); n > 0 {
		overrides.MaxConcurrentTasks = n
	}

	cfg, err := daemon.LoadConfig(overrides)
	if err != nil {
		return err
	}
	cfg.CLIVersion = version

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger := logger_pkg.NewLogger("daemon")
	d := daemon.New(cfg, logger)

	// Write PID file so "daemon stop" can find us.
	if dir := daemonDirForProfile(profile); dir != "" {
		os.MkdirAll(dir, 0o755)
		os.WriteFile(daemonPIDPathForProfile(profile), []byte(strconv.Itoa(os.Getpid())), 0o644)
	}
	defer os.Remove(daemonPIDPathForProfile(profile))

	if err := d.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}

	// Check if the daemon needs to restart after a CLI update.
	if restartBin := d.RestartBinary(); restartBin != "" {
		logger.Info("restarting daemon with updated binary", "path", restartBin)

		args := buildDaemonStartArgs(cmd)
		child := exec.Command(restartBin, args...)

		logPath := daemonLogPathForProfile(profile)
		logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			logger.Error("failed to open log file for restart", "error", err)
			return nil
		}
		child.Stdout = logFile
		child.Stderr = logFile
		child.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

		if err := child.Start(); err != nil {
			logFile.Close()
			logger.Error("failed to start new daemon", "error", err)
			return nil
		}
		childPID := child.Process.Pid
		logFile.Close()
		child.Process.Release()

		// Write new PID file.
		pidPath := daemonPIDPathForProfile(profile)
		os.WriteFile(pidPath, []byte(strconv.Itoa(childPID)), 0o644)

		logger.Info("new daemon started", "pid", childPID)
	}

	return nil
}

// --- daemon stop ---

func runDaemonStop(cmd *cobra.Command, _ []string) error {
	profile := resolveProfile(cmd)
	healthPort := healthPortForProfile(profile)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	health := checkDaemonHealthOnPort(ctx, healthPort)
	if health["status"] != "running" {
		label := "Daemon"
		if profile != "" {
			label = fmt.Sprintf("Daemon [%s]", profile)
		}
		fmt.Fprintf(os.Stderr, "%s is not running.\n", label)
		return nil
	}

	pid, ok := health["pid"].(float64)
	if !ok || pid == 0 {
		return fmt.Errorf("could not determine daemon PID from health endpoint")
	}

	process, err := os.FindProcess(int(pid))
	if err != nil {
		return fmt.Errorf("find process %d: %w", int(pid), err)
	}

	if err := process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("stop daemon (pid %d): %w", int(pid), err)
	}

	fmt.Fprintf(os.Stderr, "Stopping daemon (pid %d)...\n", int(pid))

	// Poll health endpoint until daemon is gone.
	for i := 0; i < 10; i++ {
		time.Sleep(500 * time.Millisecond)
		ctx2, cancel2 := context.WithTimeout(context.Background(), 1*time.Second)
		h := checkDaemonHealthOnPort(ctx2, healthPort)
		cancel2()
		if h["status"] != "running" {
			os.Remove(daemonPIDPathForProfile(profile))
			fmt.Fprintln(os.Stderr, "Daemon stopped.")
			return nil
		}
	}

	fmt.Fprintln(os.Stderr, "Daemon is still stopping. It may be finishing a running task.")
	return nil
}

// --- daemon status ---

func runDaemonStatus(cmd *cobra.Command, _ []string) error {
	profile := resolveProfile(cmd)
	healthPort := healthPortForProfile(profile)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	health := checkDaemonHealthOnPort(ctx, healthPort)

	output, _ := cmd.Flags().GetString("output")
	if output == "json" {
		return cli.PrintJSON(os.Stdout, health)
	}

	label := "Daemon"
	if profile != "" {
		label = fmt.Sprintf("Daemon [%s]", profile)
	}

	if health["status"] != "running" {
		fmt.Fprintf(os.Stdout, "%s: stopped\n", label)
		return nil
	}

	fmt.Fprintf(os.Stdout, "%s:      running (pid %v, uptime %v)\n", label, health["pid"], health["uptime"])
	if serverURL := daemonHealthString(health, "server_url"); serverURL != "" {
		fmt.Fprintf(os.Stdout, "Server URL:  %s\n", serverURL)
	}
	if agents, ok := health["agents"].([]any); ok && len(agents) > 0 {
		parts := make([]string, len(agents))
		for i, a := range agents {
			parts[i] = fmt.Sprint(a)
		}
		fmt.Fprintf(os.Stdout, "Agents:      %s\n", strings.Join(parts, ", "))
	}
	if ws, ok := health["workspaces"].([]any); ok {
		fmt.Fprintf(os.Stdout, "Workspaces:  %d\n", len(ws))
	}
	return nil
}

// --- daemon logs ---

func runDaemonLogs(cmd *cobra.Command, _ []string) error {
	profile := resolveProfile(cmd)
	logPath := daemonLogPathForProfile(profile)
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		return fmt.Errorf("no log file found at %s\nThe daemon may not have been started in background mode", logPath)
	}

	follow, _ := cmd.Flags().GetBool("follow")
	lines, _ := cmd.Flags().GetInt("lines")

	args := []string{"-n", strconv.Itoa(lines)}
	if follow {
		args = append(args, "-f")
	}
	args = append(args, logPath)

	tail := exec.Command("tail", args...)
	tail.Stdout = os.Stdout
	tail.Stderr = os.Stderr
	return tail.Run()
}

// checkDaemonHealthOnPort calls the daemon's local health endpoint on the given port.
func checkDaemonHealthOnPort(ctx context.Context, port int) map[string]any {
	addr := fmt.Sprintf("http://127.0.0.1:%d/health", port)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, addr, nil)
	if err != nil {
		return map[string]any{"status": "stopped"}
	}

	httpClient := &http.Client{Timeout: 2 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return map[string]any{"status": "stopped"}
	}
	defer resp.Body.Close()

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return map[string]any{"status": "stopped"}
	}
	return result
}

// flagString returns a string flag value or empty string.
func flagString(cmd *cobra.Command, name string) string {
	val, _ := cmd.Flags().GetString(name)
	return val
}

func daemonHealthString(health map[string]any, key string) string {
	raw, _ := health[key].(string)
	return strings.TrimSpace(raw)
}

func daemonPIDFromHealth(health map[string]any) (int, bool) {
	raw, ok := health["pid"]
	if !ok {
		return 0, false
	}

	switch pid := raw.(type) {
	case float64:
		if pid <= 0 {
			return 0, false
		}
		return int(pid), true
	case int:
		if pid <= 0 {
			return 0, false
		}
		return pid, true
	case int64:
		if pid <= 0 {
			return 0, false
		}
		return int(pid), true
	default:
		return 0, false
	}
}

func daemonPIDText(health map[string]any) string {
	if pid, ok := daemonPIDFromHealth(health); ok {
		return strconv.Itoa(pid)
	}
	return ""
}

func shouldReplaceLegacyDaemon(health map[string]any) bool {
	if health["status"] != "running" {
		return false
	}
	return strings.Contains(strings.ToLower(daemonHealthString(health, "server_url")), "multica")
}

func daemonStartConflictError(profile string, health map[string]any, desiredServerURL string) error {
	if health["status"] != "running" {
		return nil
	}

	runningServerURL := daemonHealthString(health, "server_url")
	if runningServerURL == "" || desiredServerURL == "" || runningServerURL == desiredServerURL {
		return nil
	}

	label := "daemon"
	if profile != "" {
		label = fmt.Sprintf("daemon [%s]", profile)
	}

	pidText := daemonPIDText(health)
	if pidText == "" {
		return fmt.Errorf(
			"%s health port is occupied by a daemon targeting %s; this CLI is configured for %s. Stop the conflicting daemon or use a named profile",
			label,
			runningServerURL,
			desiredServerURL,
		)
	}

	return fmt.Errorf(
		"%s health port is occupied by a daemon targeting %s (pid %s); this CLI is configured for %s. Stop the conflicting daemon or use a named profile",
		label,
		runningServerURL,
		pidText,
		desiredServerURL,
	)
}

func stopRunningDaemon(healthPort int, health map[string]any) error {
	pid, ok := daemonPIDFromHealth(health)
	if !ok {
		return fmt.Errorf("could not determine daemon PID from health endpoint")
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find process %d: %w", pid, err)
	}

	if err := process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("stop daemon (pid %d): %w", pid, err)
	}

	for i := 0; i < 20; i++ {
		time.Sleep(250 * time.Millisecond)

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		current := checkDaemonHealthOnPort(ctx, healthPort)
		cancel()

		if current["status"] != "running" {
			return nil
		}

		currentPID, ok := daemonPIDFromHealth(current)
		if ok && currentPID != pid {
			return nil
		}
	}

	return fmt.Errorf("daemon (pid %d) did not stop within 5s", pid)
}

func waitForDaemonHealth(healthPort int, timeout time.Duration) map[string]any {
	deadline := time.Now().Add(timeout)

	for {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		health := checkDaemonHealthOnPort(ctx, healthPort)
		cancel()

		if health["status"] == "running" {
			return health
		}
		if time.Now().After(deadline) {
			return health
		}

		time.Sleep(250 * time.Millisecond)
	}
}
