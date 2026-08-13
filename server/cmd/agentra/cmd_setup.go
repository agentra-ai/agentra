package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/agentra-ai/agentra/server/internal/buildinfo"
	"github.com/agentra-ai/agentra/server/internal/cli"
)

const (
	setupSelfHostServerURL = "http://127.0.0.1:8080"
	setupSelfHostAppURL    = "http://127.0.0.1:3000"
)

type setupOptions struct {
	ServerURL string
	AppURL    string
	NoDaemon  bool
	Reauth    bool
	Timeout   time.Duration
}

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Connect this machine to Agentra",
	Long: `Configure a self-hosted Agentra endpoint, run preflight checks,
authenticate, discover workspaces, and start the local agent daemon.

Setup defaults to the loopback ports published by docker compose. Use
--no-daemon on machines that should only use the management CLI.`,
	Args: cobra.NoArgs,
	RunE: runSetup,
}

func init() {
	f := setupCmd.Flags()
	f.String("app-url", "", "Agentra Web app URL")
	f.Bool("token", false, "Authenticate by pasting a personal access token")
	f.Bool("no-daemon", false, "Configure authentication and workspaces without starting a local daemon")
	f.Bool("reauth", false, "Authenticate again even if the stored token is valid")
	f.Duration("timeout", 5*time.Second, "Timeout for each preflight request")
}

func runSetup(cmd *cobra.Command, _ []string) error {
	opts, err := resolveSetupOptions(cmd)
	if err != nil {
		return err
	}

	fmt.Fprintln(os.Stderr, "[1/4] Checking Agentra endpoints and local runtime...")
	runtimes, err := runSetupPreflight(opts)
	if err != nil {
		return fmt.Errorf("preflight failed: %w", err)
	}
	fmt.Fprintf(os.Stderr, "  API: %s\n  Web: %s\n", opts.ServerURL, opts.AppURL)
	if opts.NoDaemon {
		fmt.Fprintln(os.Stderr, "  Local daemon: skipped")
	} else {
		fmt.Fprintf(os.Stderr, "  Agent CLIs: %s\n", strings.Join(runtimes, ", "))
	}

	profile := resolveProfile(cmd)
	resetAuth, err := saveSetupEndpoints(profile, opts.ServerURL, opts.AppURL)
	if err != nil {
		return err
	}
	if resetAuth {
		fmt.Fprintln(os.Stderr, "  Endpoint changed; cleared credentials and workspace selections from the previous server.")
	}

	fmt.Fprintln(os.Stderr, "[2/4] Authenticating...")
	if !opts.Reauth && setupTokenValid(cmd, opts.Timeout) {
		fmt.Fprintln(os.Stderr, "  Existing access token is valid.")
	} else if err := runAuthLogin(cmd, nil); err != nil {
		return fmt.Errorf("authenticate: %w", err)
	}

	fmt.Fprintln(os.Stderr, "[3/4] Discovering workspaces...")
	if err := autoWatchWorkspaces(cmd); err != nil {
		return fmt.Errorf("configure workspaces: %w", err)
	}

	if opts.NoDaemon {
		fmt.Fprintln(os.Stderr, "[4/4] Local daemon skipped (--no-daemon).")
		fmt.Fprintln(os.Stderr, "Setup complete. Run 'agentra doctor' to inspect this profile.")
		return nil
	}

	fmt.Fprintln(os.Stderr, "[4/4] Starting local daemon...")
	if err := ensureSetupDaemon(cmd); err != nil {
		return fmt.Errorf("start daemon: %w", err)
	}
	fmt.Fprintln(os.Stderr, "Setup complete. This machine is ready to execute Agentra tasks.")
	return nil
}

func resolveSetupOptions(cmd *cobra.Command) (setupOptions, error) {
	serverURL := resolveServerURL(cmd)
	appURL, _ := cmd.Flags().GetString("app-url")
	appURL = strings.TrimSpace(appURL)
	if appURL == "" {
		appURL = resolveAppURL(cmd)
	}
	if serverURL == "" {
		serverURL = setupSelfHostServerURL
	}
	if appURL == "" {
		appURL = setupSelfHostAppURL
	}

	serverURL, err := normalizeSetupEndpoint("server", serverURL)
	if err != nil {
		return setupOptions{}, err
	}
	appURL, err = normalizeSetupEndpoint("app", appURL)
	if err != nil {
		return setupOptions{}, err
	}

	noDaemon, _ := cmd.Flags().GetBool("no-daemon")
	reauth, _ := cmd.Flags().GetBool("reauth")
	timeout, _ := cmd.Flags().GetDuration("timeout")
	if timeout <= 0 {
		return setupOptions{}, fmt.Errorf("--timeout must be greater than zero")
	}

	return setupOptions{
		ServerURL: serverURL,
		AppURL:    appURL,
		NoDaemon:  noDaemon,
		Reauth:    reauth,
		Timeout:   timeout,
	}, nil
}

func normalizeSetupEndpoint(label, raw string) (string, error) {
	normalized := strings.TrimRight(strings.TrimSpace(raw), "/")
	parsed, err := url.Parse(normalized)
	if err != nil || parsed.Host == "" {
		return "", fmt.Errorf("invalid %s URL %q", label, raw)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("%s URL must use http or https", label)
	}
	if parsed.User != nil {
		return "", fmt.Errorf("%s URL must not contain credentials", label)
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("%s URL must not contain a query or fragment", label)
	}
	if parsed.Scheme == "http" && !isLoopbackSetupHost(parsed.Hostname()) {
		return "", fmt.Errorf("remote %s URL must use https", label)
	}
	return normalized, nil
}

func isLoopbackSetupHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func runSetupPreflight(opts setupOptions) ([]string, error) {
	client := &http.Client{Timeout: opts.Timeout}
	if err := checkSetupEndpoint(client, opts.ServerURL+"/readyz", "Agentra API readiness"); err != nil {
		return nil, err
	}
	if err := checkSetupEndpoint(client, opts.AppURL, "Agentra Web app"); err != nil {
		return nil, err
	}
	if opts.NoDaemon {
		return nil, nil
	}

	runtimes := discoverSetupRuntimes()
	if len(runtimes) == 0 {
		return nil, fmt.Errorf("no supported agent CLI found; install claude, codex, or opencode, or use --no-daemon")
	}
	return runtimes, nil
}

func checkSetupEndpoint(client *http.Client, endpoint, label string) error {
	ctx, cancel := context.WithTimeout(context.Background(), client.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("%s: %w", label, err)
	}
	req.Header.Set("User-Agent", "agentra-setup/"+buildinfo.Current().Version)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("%s at %s is unreachable: %w", label, endpoint, err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return fmt.Errorf("%s at %s returned HTTP %d", label, endpoint, resp.StatusCode)
	}
	return nil
}

func discoverSetupRuntimes() []string {
	var found []string
	for _, name := range []string{"claude", "codex", "opencode"} {
		path := strings.TrimSpace(os.Getenv("AGENTRA_" + strings.ToUpper(name) + "_PATH"))
		if path == "" {
			path = name
		}
		if _, err := exec.LookPath(path); err == nil {
			found = append(found, name)
		}
	}
	return found
}

func saveSetupEndpoints(profile, serverURL, appURL string) (bool, error) {
	cfg, err := cli.LoadCLIConfigForProfile(profile)
	if err != nil {
		return false, err
	}
	changedServer := cfg.ServerURL != serverURL
	changedApp := cfg.AppURL != appURL
	resetAuth := (changedServer || changedApp) && (cfg.Token != "" || cfg.WorkspaceID != "" || len(cfg.WatchedWorkspaces) > 0)
	if changedServer || changedApp {
		cfg.Token = ""
		cfg.WorkspaceID = ""
		cfg.WatchedWorkspaces = nil
	}
	cfg.ServerURL = serverURL
	cfg.AppURL = appURL
	if err := cli.SaveCLIConfigForProfile(cfg, profile); err != nil {
		return false, fmt.Errorf("save setup configuration: %w", err)
	}
	return resetAuth, nil
}

func setupTokenValid(cmd *cobra.Command, timeout time.Duration) bool {
	token := resolveToken(cmd)
	if token == "" {
		return false
	}
	client := cli.NewAPIClient(resolveServerURL(cmd), "", token)
	client.HTTPClient.Timeout = timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	var me struct {
		ID string `json:"id"`
	}
	return client.GetJSON(ctx, "/api/me", &me) == nil
}

func ensureSetupDaemon(cmd *cobra.Command) error {
	profile := resolveProfile(cmd)
	healthPort := healthPortForProfile(profile)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	health := checkDaemonHealthOnPort(ctx, healthPort)
	if health["status"] == "running" && !shouldReplaceLegacyDaemon(health) {
		if err := daemonStartConflictError(profile, health, resolveServerURL(cmd)); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "  Daemon is already running on port %d.\n", healthPort)
		return nil
	}
	return runDaemonBackground(cmd)
}
