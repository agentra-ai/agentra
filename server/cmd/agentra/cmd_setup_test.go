package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/agentra-ai/agentra/server/internal/cli"
)

func newSetupTestCommand() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Flags().String("server-url", "", "")
	cmd.Flags().String("profile", "", "")
	cmd.Flags().String("app-url", "", "")
	cmd.Flags().Bool("token", false, "")
	cmd.Flags().Bool("no-daemon", false, "")
	cmd.Flags().Bool("reauth", false, "")
	cmd.Flags().Duration("timeout", 5*time.Second, "")
	return cmd
}

func TestRunSetupTokenFlowWithoutDaemon(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	clearSetupEnvironmentExceptHome(t)

	const token = "mul_0123456789abcdef0123456789abcdef01234567"
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/readyz":
			w.WriteHeader(http.StatusOK)
		case "/api/me":
			if r.Header.Get("Authorization") != "Bearer "+token {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"id":"user-1","name":"Test User","email":"test@example.com"}`)
		case "/api/workspaces":
			if r.Header.Get("Authorization") != "Bearer "+token {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `[{"id":"ws-1","name":"Test Workspace"}]`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer api.Close()
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer app.Close()

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	originalStdin := os.Stdin
	os.Stdin = reader
	t.Cleanup(func() {
		os.Stdin = originalStdin
		reader.Close()
	})
	if _, err := fmt.Fprintln(writer, token); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	cmd := newSetupTestCommand()
	_ = cmd.Flags().Set("server-url", api.URL)
	_ = cmd.Flags().Set("app-url", app.URL)
	_ = cmd.Flags().Set("token", "true")
	_ = cmd.Flags().Set("no-daemon", "true")
	if err := runSetup(cmd, nil); err != nil {
		t.Fatalf("runSetup: %v", err)
	}

	cfg, err := cli.LoadCLIConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Token != token || cfg.WorkspaceID != "ws-1" {
		t.Fatalf("stored auth/workspace = (%q, %q)", cfg.Token, cfg.WorkspaceID)
	}
	if len(cfg.WatchedWorkspaces) != 1 || cfg.WatchedWorkspaces[0].ID != "ws-1" {
		t.Fatalf("watched workspaces = %+v", cfg.WatchedWorkspaces)
	}
}

func clearSetupEnvironment(t *testing.T) {
	t.Helper()
	clearSetupEnvironmentExceptHome(t)
	t.Setenv("HOME", t.TempDir())
}

func clearSetupEnvironmentExceptHome(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"AGENTRA_CLI_SERVER_URL",
		"AGENTRA_SERVER_URL",
		"AGENTRA_CLI_APP_URL",
		"NEXT_PUBLIC_SITE_URL",
		"AGENTRA_APP_URL",
		"FRONTEND_ORIGIN",
	} {
		t.Setenv(key, "")
	}
}

func TestSetupCommandRegistered(t *testing.T) {
	command, _, err := rootCmd.Find([]string{"setup"})
	if err != nil {
		t.Fatalf("find setup command: %v", err)
	}
	if command != setupCmd {
		t.Fatalf("found %q, want setup", command.Name())
	}
	for _, name := range []string{"app-url", "token", "no-daemon", "reauth", "timeout"} {
		if command.Flags().Lookup(name) == nil {
			t.Fatalf("setup flag %q is not registered", name)
		}
	}
}

func TestResolveSetupOptionsSelfHostDefaults(t *testing.T) {
	clearSetupEnvironment(t)
	cmd := newSetupTestCommand()

	opts, err := resolveSetupOptions(cmd)
	if err != nil {
		t.Fatalf("resolveSetupOptions: %v", err)
	}
	if opts.ServerURL != setupSelfHostServerURL {
		t.Fatalf("server URL = %q, want %q", opts.ServerURL, setupSelfHostServerURL)
	}
	if opts.AppURL != setupSelfHostAppURL {
		t.Fatalf("app URL = %q, want %q", opts.AppURL, setupSelfHostAppURL)
	}
}

func TestResolveSetupOptionsRejectsInsecureRemoteEndpoint(t *testing.T) {
	clearSetupEnvironment(t)
	cmd := newSetupTestCommand()
	_ = cmd.Flags().Set("server-url", "http://api.example.com")
	_ = cmd.Flags().Set("app-url", "https://app.example.com")

	_, err := resolveSetupOptions(cmd)
	if err == nil || !strings.Contains(err.Error(), "must use https") {
		t.Fatalf("error = %v, want HTTPS requirement", err)
	}
}

func TestRunSetupPreflight(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/readyz" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer api.Close()
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer app.Close()

	runtimes, err := runSetupPreflight(setupOptions{
		ServerURL: api.URL,
		AppURL:    app.URL,
		NoDaemon:  true,
		Timeout:   time.Second,
	})
	if err != nil {
		t.Fatalf("runSetupPreflight: %v", err)
	}
	if len(runtimes) != 0 {
		t.Fatalf("runtimes = %v, want none when daemon is skipped", runtimes)
	}
}

func TestRunSetupPreflightRejectsUnhealthyAPI(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "not ready", http.StatusServiceUnavailable)
	}))
	defer api.Close()

	_, err := runSetupPreflight(setupOptions{
		ServerURL: api.URL,
		AppURL:    api.URL,
		NoDaemon:  true,
		Timeout:   time.Second,
	})
	if err == nil || !strings.Contains(err.Error(), "HTTP 503") {
		t.Fatalf("error = %v, want HTTP 503", err)
	}
}

func TestSaveSetupEndpointsClearsServerBoundStateOnChange(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	original := cli.CLIConfig{
		ServerURL:   "https://old-api.example.com",
		AppURL:      "https://old-app.example.com",
		WorkspaceID: "ws-old",
		Token:       "secret-token",
		WatchedWorkspaces: []cli.WatchedWorkspace{
			{ID: "ws-old", Name: "Old"},
		},
	}
	if err := cli.SaveCLIConfigForProfile(original, "test"); err != nil {
		t.Fatal(err)
	}

	reset, err := saveSetupEndpoints("test", "https://new-api.example.com", "https://new-app.example.com")
	if err != nil {
		t.Fatalf("saveSetupEndpoints: %v", err)
	}
	if !reset {
		t.Fatal("reset = false, want true")
	}
	cfg, err := cli.LoadCLIConfigForProfile("test")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Token != "" || cfg.WorkspaceID != "" || len(cfg.WatchedWorkspaces) != 0 {
		t.Fatalf("server-bound state was not cleared: %+v", cfg)
	}
	if cfg.ServerURL != "https://new-api.example.com" || cfg.AppURL != "https://new-app.example.com" {
		t.Fatalf("endpoints = (%q, %q)", cfg.ServerURL, cfg.AppURL)
	}
}

func TestSaveSetupEndpointsPreservesStateWhenUnchanged(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	original := cli.CLIConfig{
		ServerURL:   "https://api.example.com",
		AppURL:      "https://app.example.com",
		WorkspaceID: "ws-1",
		Token:       "secret-token",
	}
	if err := cli.SaveCLIConfig(original); err != nil {
		t.Fatal(err)
	}

	reset, err := saveSetupEndpoints("", original.ServerURL, original.AppURL)
	if err != nil {
		t.Fatalf("saveSetupEndpoints: %v", err)
	}
	if reset {
		t.Fatal("reset = true, want false")
	}
	cfg, err := cli.LoadCLIConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Token != original.Token || cfg.WorkspaceID != original.WorkspaceID {
		t.Fatalf("state changed: %+v", cfg)
	}
}
