// Package doctor provides bounded, read-only installation diagnostics for the
// Agentra CLI.
package doctor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/agentra-ai/agentra/server/pkg/agent"
	"github.com/gorilla/websocket"
)

const SchemaVersion = 1

type Status string

const (
	StatusPass    Status = "pass"
	StatusWarning Status = "warning"
	StatusFail    Status = "fail"
	StatusSkipped Status = "skipped"
)

type Check struct {
	ID          string `json:"id"`
	Status      Status `json:"status"`
	Summary     string `json:"summary"`
	Remediation string `json:"remediation,omitempty"`
}

type Summary struct {
	Passed   int `json:"passed"`
	Warnings int `json:"warnings"`
	Failed   int `json:"failed"`
	Skipped  int `json:"skipped"`
}

type Report struct {
	SchemaVersion int     `json:"schema_version"`
	Status        Status  `json:"status"`
	Checks        []Check `json:"checks"`
	Summary       Summary `json:"summary"`
}

type Options struct {
	ServerURL           string
	AppURL              string
	WorkspaceID         string
	Token               string
	Profile             string
	ConfigPath          string
	ConfigError         error
	RepoPath            string
	SkipRepoRemote      bool
	WorkspacesRoot      string
	WorkspacesRootError error
	DaemonURL           string
	Timeout             time.Duration
	HTTPClient          *http.Client
	RuntimeCandidates   []RuntimeCandidate
}

type RuntimeCandidate struct {
	Name string
	Path string
}

type readinessCheck struct {
	Status string `json:"status"`
}

type readinessResponse struct {
	Status string                    `json:"status"`
	Checks map[string]readinessCheck `json:"checks"`
}

// Run executes independent checks concurrently and returns them in a stable
// order suitable for both terminal and JSON consumers.
func Run(ctx context.Context, options Options) Report {
	if options.Timeout <= 0 {
		options.Timeout = 5 * time.Second
	}
	if options.RepoPath == "" {
		options.RepoPath = "."
	}
	if options.HTTPClient == nil {
		options.HTTPClient = &http.Client{Timeout: options.Timeout}
	}

	groups := []func(context.Context, Options) []Check{
		checkConfiguration,
		checkConfigFile,
		checkLiveness,
		checkReadiness,
		checkWebApp,
		checkAuthentication,
		checkWorkspace,
		checkRuntimes,
		checkWorkspacesRoot,
		checkRepository,
		checkDaemon,
		checkWebSocket,
	}
	results := make([][]Check, len(groups))
	var wait sync.WaitGroup
	for index, group := range groups {
		index, group := index, group
		wait.Add(1)
		go func() {
			defer wait.Done()
			results[index] = group(ctx, options)
		}()
	}
	wait.Wait()

	checks := make([]Check, 0, len(groups)+1)
	for _, group := range results {
		checks = append(checks, group...)
	}
	return buildReport(checks)
}

func buildReport(checks []Check) Report {
	report := Report{SchemaVersion: SchemaVersion, Status: StatusPass, Checks: checks}
	for _, check := range checks {
		switch check.Status {
		case StatusPass:
			report.Summary.Passed++
		case StatusWarning:
			report.Summary.Warnings++
			if report.Status == StatusPass {
				report.Status = StatusWarning
			}
		case StatusFail:
			report.Summary.Failed++
			report.Status = StatusFail
		case StatusSkipped:
			report.Summary.Skipped++
			if report.Status == StatusPass {
				report.Status = StatusWarning
			}
		}
	}
	return report
}

func checkConfiguration(_ context.Context, options Options) []Check {
	if options.ConfigError != nil {
		return []Check{{
			ID:          "configuration",
			Status:      StatusFail,
			Summary:     "CLI configuration could not be read: " + oneLine(options.ConfigError.Error()),
			Remediation: "Repair or replace the profile config, then run `agentra doctor` again.",
		}}
	}

	missing := make([]string, 0, 3)
	if strings.TrimSpace(options.ServerURL) == "" {
		missing = append(missing, "server URL")
	}
	if strings.TrimSpace(options.Token) == "" {
		missing = append(missing, "access token")
	}
	if strings.TrimSpace(options.WorkspaceID) == "" {
		missing = append(missing, "workspace ID")
	}
	if len(missing) > 0 {
		return []Check{{
			ID:          "configuration",
			Status:      StatusFail,
			Summary:     "Missing " + strings.Join(missing, ", ") + ".",
			Remediation: "Run `agentra login` or set the corresponding CLI flags/environment variables.",
		}}
	}

	label := "default profile"
	if options.Profile != "" {
		label = "profile " + options.Profile
	}
	return []Check{{ID: "configuration", Status: StatusPass, Summary: "Required values resolved for " + label + "."}}
}

func checkConfigFile(_ context.Context, options Options) []Check {
	if options.ConfigPath == "" {
		return []Check{{ID: "config_file", Status: StatusSkipped, Summary: "Config path could not be resolved."}}
	}
	info, err := os.Stat(options.ConfigPath)
	if os.IsNotExist(err) {
		return []Check{{
			ID:          "config_file",
			Status:      StatusWarning,
			Summary:     "No profile config file; values must come from flags or environment variables.",
			Remediation: "Run `agentra login` to create a secured profile config.",
		}}
	}
	if err != nil {
		return []Check{{ID: "config_file", Status: StatusFail, Summary: "Config file is not readable: " + oneLine(err.Error())}}
	}
	if !info.Mode().IsRegular() {
		return []Check{{ID: "config_file", Status: StatusFail, Summary: "Config path is not a regular file."}}
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return []Check{{
			ID:          "config_file",
			Status:      StatusWarning,
			Summary:     fmt.Sprintf("Config permissions are %04o; access tokens should be owner-only.", info.Mode().Perm()),
			Remediation: "Run `chmod 600 " + options.ConfigPath + "`.",
		}}
	}
	return []Check{{ID: "config_file", Status: StatusPass, Summary: "Config file is readable and owner-only."}}
}

func checkLiveness(ctx context.Context, options Options) []Check {
	endpoint, err := endpointURL(options.ServerURL, "/livez")
	if err != nil {
		return []Check{{ID: "server_liveness", Status: StatusFail, Summary: err.Error(), Remediation: serverURLRemediation}}
	}
	var response struct {
		Status string `json:"status"`
	}
	requestCtx, cancel := context.WithTimeout(ctx, options.Timeout)
	defer cancel()
	status, err := getJSON(requestCtx, options.HTTPClient, endpoint, "", &response)
	if err != nil {
		return []Check{{ID: "server_liveness", Status: StatusFail, Summary: "Server is unreachable: " + oneLine(err.Error()), Remediation: networkRemediation}}
	}
	if status != http.StatusOK || response.Status != "live" {
		return []Check{{ID: "server_liveness", Status: StatusFail, Summary: fmt.Sprintf("Unexpected liveness response (HTTP %d, status %q).", status, response.Status), Remediation: networkRemediation}}
	}
	return []Check{{ID: "server_liveness", Status: StatusPass, Summary: "Server process is live."}}
}

func checkReadiness(ctx context.Context, options Options) []Check {
	endpoint, err := endpointURL(options.ServerURL, "/readyz")
	if err != nil {
		return []Check{
			{ID: "server_readiness", Status: StatusFail, Summary: err.Error(), Remediation: serverURLRemediation},
			{ID: "storage", Status: StatusSkipped, Summary: "Storage check skipped because readiness is unavailable."},
		}
	}
	var response readinessResponse
	requestCtx, cancel := context.WithTimeout(ctx, options.Timeout)
	defer cancel()
	status, err := getJSON(requestCtx, options.HTTPClient, endpoint, "", &response)
	if err != nil {
		return []Check{
			{ID: "server_readiness", Status: StatusFail, Summary: "Readiness probe failed: " + oneLine(err.Error()), Remediation: "Inspect server logs and `/readyz` from the deployment network."},
			{ID: "storage", Status: StatusSkipped, Summary: "Storage check skipped because readiness is unavailable."},
		}
	}

	checks := make([]Check, 0, 2)
	if status == http.StatusOK && response.Status == "ready" {
		checks = append(checks, Check{ID: "server_readiness", Status: StatusPass, Summary: "Database, migrations, storage policy, and scheduler are ready."})
	} else {
		checks = append(checks, Check{
			ID:          "server_readiness",
			Status:      StatusFail,
			Summary:     fmt.Sprintf("Server is not ready (HTTP %d, status %q).", status, response.Status),
			Remediation: "Inspect failing `/readyz` checks and server logs before routing traffic.",
		})
	}

	storage, ok := response.Checks["storage"]
	switch {
	case !ok:
		checks = append(checks, Check{ID: "storage", Status: StatusFail, Summary: "Readiness response omitted the storage check.", Remediation: "Upgrade the Agentra server and verify `/readyz`."})
	case storage.Status == "ok":
		checks = append(checks, Check{ID: "storage", Status: StatusPass, Summary: "Configured object storage is reachable."})
	case storage.Status == "disabled":
		checks = append(checks, Check{ID: "storage", Status: StatusWarning, Summary: "Object storage is disabled; attachment features are unavailable.", Remediation: "Configure S3 or MinIO when attachment support is required."})
	default:
		checks = append(checks, Check{ID: "storage", Status: StatusFail, Summary: "Configured object storage is not ready.", Remediation: "Verify bucket existence, credentials, endpoint DNS, and TLS settings."})
	}
	return checks
}

func checkWebApp(ctx context.Context, options Options) []Check {
	if strings.TrimSpace(options.AppURL) == "" {
		return []Check{{ID: "web_app", Status: StatusWarning, Summary: "Web app URL is not configured.", Remediation: "Set app_url with `agentra config set app_url <url>`."}}
	}
	endpoint, err := endpointURL(options.AppURL, "/")
	if err != nil {
		return []Check{{ID: "web_app", Status: StatusFail, Summary: err.Error(), Remediation: "Set a valid HTTP(S) app URL."}}
	}
	requestCtx, cancel := context.WithTimeout(ctx, options.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return []Check{{ID: "web_app", Status: StatusFail, Summary: "Web app request could not be created."}}
	}
	resp, err := options.HTTPClient.Do(req)
	if err != nil {
		return []Check{{ID: "web_app", Status: StatusFail, Summary: "Web app is unreachable: " + oneLine(err.Error()), Remediation: networkRemediation}}
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode >= http.StatusBadRequest {
		return []Check{{ID: "web_app", Status: StatusFail, Summary: fmt.Sprintf("Web app returned HTTP %d.", resp.StatusCode), Remediation: "Inspect the Web container and reverse proxy."}}
	}
	return []Check{{ID: "web_app", Status: StatusPass, Summary: "Web app is reachable."}}
}

func checkAuthentication(ctx context.Context, options Options) []Check {
	if options.Token == "" {
		return []Check{{ID: "authentication", Status: StatusSkipped, Summary: "Authentication check skipped because no token is configured."}}
	}
	endpoint, err := endpointURL(options.ServerURL, "/api/me")
	if err != nil {
		return []Check{{ID: "authentication", Status: StatusSkipped, Summary: "Authentication check skipped because the server URL is invalid."}}
	}
	var response struct {
		ID string `json:"id"`
	}
	requestCtx, cancel := context.WithTimeout(ctx, options.Timeout)
	defer cancel()
	status, err := getJSON(requestCtx, options.HTTPClient, endpoint, options.Token, &response)
	if err != nil {
		return []Check{{ID: "authentication", Status: StatusFail, Summary: "Authentication request failed: " + oneLine(err.Error()), Remediation: "Check network/TLS, then run `agentra login` if the token is stale."}}
	}
	if status != http.StatusOK {
		return []Check{{ID: "authentication", Status: StatusFail, Summary: fmt.Sprintf("Token was rejected (HTTP %d).", status), Remediation: "Run `agentra login` to issue a new token."}}
	}
	return []Check{{ID: "authentication", Status: StatusPass, Summary: "Access token is valid."}}
}

func checkWorkspace(ctx context.Context, options Options) []Check {
	if options.Token == "" || options.WorkspaceID == "" {
		return []Check{{ID: "workspace", Status: StatusSkipped, Summary: "Workspace membership check skipped because token or workspace ID is missing."}}
	}
	endpoint, err := endpointURL(options.ServerURL, "/api/workspaces/"+url.PathEscape(options.WorkspaceID))
	if err != nil {
		return []Check{{ID: "workspace", Status: StatusSkipped, Summary: "Workspace check skipped because the server URL is invalid."}}
	}
	var response struct {
		ID string `json:"id"`
	}
	requestCtx, cancel := context.WithTimeout(ctx, options.Timeout)
	defer cancel()
	status, err := getJSON(requestCtx, options.HTTPClient, endpoint, options.Token, &response)
	if err != nil {
		return []Check{{ID: "workspace", Status: StatusFail, Summary: "Workspace request failed: " + oneLine(err.Error()), Remediation: "Verify the workspace ID and server connectivity."}}
	}
	if status != http.StatusOK {
		return []Check{{ID: "workspace", Status: StatusFail, Summary: fmt.Sprintf("Workspace access was rejected (HTTP %d).", status), Remediation: "Run `agentra workspace list` and select a workspace you belong to."}}
	}
	return []Check{{ID: "workspace", Status: StatusPass, Summary: "Configured workspace is accessible."}}
}

func checkRuntimes(ctx context.Context, options Options) []Check {
	candidates := options.RuntimeCandidates
	if len(candidates) == 0 {
		candidates = []RuntimeCandidate{
			{Name: "claude", Path: envOrDefault("AGENTRA_CLAUDE_PATH", "claude")},
			{Name: "codex", Path: envOrDefault("AGENTRA_CODEX_PATH", "codex")},
			{Name: "opencode", Path: envOrDefault("AGENTRA_OPENCODE_PATH", "opencode")},
		}
	}
	available := make([]string, 0, len(candidates))
	broken := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		path, err := exec.LookPath(candidate.Path)
		if err != nil {
			if len(options.RuntimeCandidates) > 0 || os.Getenv(runtimeEnvName(candidate.Name)) != "" {
				broken = append(broken, candidate.Name+" path is not executable")
			}
			continue
		}
		versionCtx, cancel := context.WithTimeout(ctx, options.Timeout)
		version, err := agent.DetectVersion(versionCtx, path)
		cancel()
		if err != nil {
			broken = append(broken, candidate.Name+" version probe failed")
			continue
		}
		available = append(available, candidate.Name+" ("+truncate(oneLine(version), 80)+")")
	}
	if len(available) == 0 {
		return []Check{{ID: "runtime_cli", Status: StatusFail, Summary: "No usable agent runtime CLI was found.", Remediation: "Install Claude Code, Codex, or OpenCode and ensure it is on PATH."}}
	}
	status := StatusPass
	summary := "Available: " + strings.Join(available, ", ") + "."
	remediation := ""
	if len(broken) > 0 {
		status = StatusWarning
		summary += " Problems: " + strings.Join(broken, ", ") + "."
		remediation = "Fix explicitly configured runtime paths or remove stale overrides."
	}
	return []Check{{ID: "runtime_cli", Status: status, Summary: summary, Remediation: remediation}}
}

func checkWorkspacesRoot(_ context.Context, options Options) []Check {
	if options.WorkspacesRootError != nil {
		return []Check{{ID: "workspace_root", Status: StatusFail, Summary: "Daemon workspace root could not be resolved: " + oneLine(options.WorkspacesRootError.Error()), Remediation: "Set AGENTRA_WORKSPACES_ROOT to a private writable directory."}}
	}
	if options.WorkspacesRoot == "" {
		return []Check{{ID: "workspace_root", Status: StatusSkipped, Summary: "Workspace root path could not be resolved."}}
	}
	if err := verifyWritablePath(options.WorkspacesRoot); err != nil {
		return []Check{{ID: "workspace_root", Status: StatusFail, Summary: "Daemon workspace root is not writable: " + oneLine(err.Error()), Remediation: "Set AGENTRA_WORKSPACES_ROOT to a private writable directory."}}
	}
	return []Check{{ID: "workspace_root", Status: StatusPass, Summary: "Daemon workspace root can be created or written."}}
}

func checkRepository(ctx context.Context, options Options) []Check {
	repoPath, err := filepath.Abs(options.RepoPath)
	if err != nil {
		return []Check{{ID: "repository", Status: StatusFail, Summary: "Repository path is invalid: " + oneLine(err.Error())}}
	}
	gitCtx, cancel := context.WithTimeout(ctx, options.Timeout)
	root, err := runGit(gitCtx, repoPath, "rev-parse", "--show-toplevel")
	cancel()
	if err != nil {
		return []Check{{ID: "repository", Status: StatusWarning, Summary: "The selected path is not a Git worktree.", Remediation: "Run from a repository or pass `agentra doctor --repo <path>`."}}
	}
	root = strings.TrimSpace(root)
	if err := verifyWritablePath(root); err != nil {
		return []Check{{ID: "repository", Status: StatusFail, Summary: "Repository is not writable.", Remediation: "Fix ownership/permissions for the repository worktree."}}
	}
	if options.SkipRepoRemote {
		return []Check{{ID: "repository", Status: StatusPass, Summary: "Git worktree is readable and writable; remote check was skipped."}}
	}
	gitCtx, cancel = context.WithTimeout(ctx, options.Timeout)
	_, err = runGit(gitCtx, root, "remote", "get-url", "origin")
	cancel()
	if err != nil {
		return []Check{{ID: "repository", Status: StatusWarning, Summary: "Git worktree is writable but has no origin remote.", Remediation: "Add an origin remote when tasks need to fetch or push code."}}
	}
	gitCtx, cancel = context.WithTimeout(ctx, options.Timeout)
	_, err = runGit(gitCtx, root, "ls-remote", "origin", "HEAD")
	cancel()
	if err != nil {
		return []Check{{ID: "repository", Status: StatusFail, Summary: "Origin remote is not reachable non-interactively.", Remediation: "Verify SSH keys, credential helper, repository access, DNS, and proxy settings."}}
	}
	return []Check{{ID: "repository", Status: StatusPass, Summary: "Git worktree is writable and origin is reachable non-interactively."}}
}

func checkDaemon(ctx context.Context, options Options) []Check {
	if options.DaemonURL == "" {
		return []Check{{ID: "local_daemon", Status: StatusSkipped, Summary: "Local daemon URL could not be resolved."}}
	}
	endpoint, err := endpointURL(options.DaemonURL, "/health")
	if err != nil {
		return []Check{{ID: "local_daemon", Status: StatusSkipped, Summary: "Local daemon URL is invalid."}}
	}
	var response struct {
		Status string `json:"status"`
	}
	requestCtx, cancel := context.WithTimeout(ctx, options.Timeout)
	defer cancel()
	status, err := getJSON(requestCtx, options.HTTPClient, endpoint, "", &response)
	if err != nil || status != http.StatusOK || response.Status != "running" {
		return []Check{{ID: "local_daemon", Status: StatusWarning, Summary: "Local daemon is not running for this profile.", Remediation: "Run `agentra daemon start` after the other checks pass."}}
	}
	return []Check{{ID: "local_daemon", Status: StatusPass, Summary: "Local daemon is running."}}
}

func checkWebSocket(ctx context.Context, options Options) []Check {
	if options.Token == "" || options.WorkspaceID == "" {
		return []Check{{ID: "websocket", Status: StatusSkipped, Summary: "WebSocket check skipped because token or workspace ID is missing."}}
	}
	endpoint, err := websocketURL(options.ServerURL, options.WorkspaceID)
	if err != nil {
		return []Check{{ID: "websocket", Status: StatusSkipped, Summary: "WebSocket check skipped because the server URL is invalid."}}
	}
	dialer := websocket.Dialer{HandshakeTimeout: options.Timeout, Proxy: http.ProxyFromEnvironment}
	header := http.Header{"Authorization": []string{"Bearer " + options.Token}}
	dialCtx, cancel := context.WithTimeout(ctx, options.Timeout)
	defer cancel()
	conn, response, err := dialer.DialContext(dialCtx, endpoint, header)
	if response != nil && response.Body != nil {
		defer response.Body.Close()
	}
	if err != nil {
		status := ""
		if response != nil {
			status = fmt.Sprintf(" (HTTP %d)", response.StatusCode)
		}
		return []Check{{ID: "websocket", Status: StatusFail, Summary: "WebSocket handshake failed" + status + ": " + oneLine(err.Error()), Remediation: "Verify reverse-proxy Upgrade headers, CORS origins, token validity, and workspace membership."}}
	}
	defer conn.Close()
	_ = conn.SetWriteDeadline(time.Now().Add(options.Timeout))
	if err := conn.WriteJSON(map[string]string{"type": "ping"}); err != nil {
		return []Check{{ID: "websocket", Status: StatusFail, Summary: "WebSocket connected but ping could not be sent.", Remediation: "Inspect reverse-proxy idle/upgrade settings and server logs."}}
	}
	_ = conn.SetReadDeadline(time.Now().Add(options.Timeout))
	var message struct {
		Type string `json:"type"`
	}
	if err := conn.ReadJSON(&message); err != nil || message.Type != "pong" {
		return []Check{{ID: "websocket", Status: StatusFail, Summary: "WebSocket connected but did not return pong.", Remediation: "Inspect realtime hub health and reverse-proxy buffering/timeouts."}}
	}
	return []Check{{ID: "websocket", Status: StatusPass, Summary: "Authenticated WebSocket ping/pong succeeded."}}
}

func getJSON(ctx context.Context, client *http.Client, endpoint, token string, output any) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(output); err != nil {
		return resp.StatusCode, fmt.Errorf("decode JSON response: %w", err)
	}
	return resp.StatusCode, nil
}

func endpointURL(baseURL, endpoint string) (string, error) {
	if strings.TrimSpace(baseURL) == "" {
		return "", fmt.Errorf("server URL is not configured")
	}
	u, err := url.Parse(baseURL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", fmt.Errorf("invalid HTTP(S) URL")
	}
	u.Path = strings.TrimRight(u.Path, "/") + endpoint
	u.RawPath = ""
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}

func websocketURL(baseURL, workspaceID string) (string, error) {
	endpoint, err := endpointURL(baseURL, "/ws")
	if err != nil {
		return "", err
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	if u.Scheme == "https" {
		u.Scheme = "wss"
	} else {
		u.Scheme = "ws"
	}
	query := u.Query()
	query.Set("workspace_id", workspaceID)
	u.RawQuery = query.Encode()
	return u.String(), nil
}

func runGit(ctx context.Context, directory string, args ...string) (string, error) {
	cmdArgs := append([]string{"-C", directory}, args...)
	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

func verifyWritablePath(target string) error {
	current := filepath.Clean(target)
	for {
		info, err := os.Stat(current)
		if err == nil {
			if !info.IsDir() {
				return fmt.Errorf("%s is not a directory", current)
			}
			file, err := os.CreateTemp(current, ".agentra-doctor-*")
			if err != nil {
				return err
			}
			name := file.Name()
			if err := file.Close(); err != nil {
				_ = os.Remove(name)
				return err
			}
			return os.Remove(name)
		}
		if !os.IsNotExist(err) {
			return err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return err
		}
		current = parent
	}
}

func envOrDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func runtimeEnvName(name string) string {
	return "AGENTRA_" + strings.ToUpper(name) + "_PATH"
}

func oneLine(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit-1] + "…"
}

const (
	serverURLRemediation = "Set --server-url, AGENTRA_CLI_SERVER_URL, or `agentra config set server_url <url>`."
	networkRemediation   = "Verify the URL, DNS, TLS certificate, reverse proxy, and server process."
)
