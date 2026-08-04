package agent

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

const (
	fixtureScenarioEnv = "AGENTRA_RUNTIME_FIXTURE_SCENARIO"
	fixtureSecret      = "sk-agentra-runtime-fixture-secret-1234567890"
)

var runtimeFixturePath string

func TestMain(m *testing.M) {
	tempDir, err := os.MkdirTemp("", "agentra-runtime-fixture-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "create runtime fixture directory: %v\n", err)
		os.Exit(1)
	}

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		fmt.Fprintln(os.Stderr, "resolve runtime conformance test path")
		_ = os.RemoveAll(tempDir)
		os.Exit(1)
	}
	binaryName := "runtimefixture"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	runtimeFixturePath = filepath.Join(tempDir, binaryName)

	cmd := exec.Command("go", "build", "-o", runtimeFixturePath, "./testdata/runtimefixture")
	cmd.Dir = filepath.Dir(currentFile)
	if output, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "build runtime fixture: %v\n%s", err, output)
		_ = os.RemoveAll(tempDir)
		os.Exit(1)
	}

	exitCode := m.Run()
	if err := os.RemoveAll(tempDir); err != nil {
		fmt.Fprintf(os.Stderr, "remove runtime fixture directory: %v\n", err)
		if exitCode == 0 {
			exitCode = 1
		}
	}
	os.Exit(exitCode)
}

func TestRuntimeFixtureDiscovery(t *testing.T) {
	t.Parallel()

	for _, provider := range []ProviderType{ProviderClaude, ProviderCodex, ProviderOpenCode} {
		provider := provider
		t.Run(string(provider), func(t *testing.T) {
			t.Parallel()

			backend := newFixtureBackend(t, provider, "success", nil)
			discovery, err := backend.Discover(context.Background())
			if err != nil {
				t.Fatalf("Discover() error: %v", err)
			}
			if discovery.Provider != provider {
				t.Errorf("provider = %q, want %q", discovery.Provider, provider)
			}
			if discovery.Executable != runtimeFixturePath {
				t.Errorf("executable = %q, want %q", discovery.Executable, runtimeFixturePath)
			}
			if discovery.Version != "agentra-runtime-fixture 1.0.0" {
				t.Errorf("version = %q", discovery.Version)
			}
		})
	}
}

func TestRuntimeFixturePartialStreamSuccess(t *testing.T) {
	t.Parallel()

	expectedSessionIDs := map[ProviderType]string{
		ProviderClaude:   "fixture-claude-session",
		ProviderCodex:    "fixture-codex-thread",
		ProviderOpenCode: "fixture-opencode-session",
	}

	for _, provider := range []ProviderType{ProviderClaude, ProviderCodex, ProviderOpenCode} {
		provider := provider
		t.Run(string(provider), func(t *testing.T) {
			t.Parallel()

			var logs bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
			backend := newFixtureBackend(t, provider, "success", logger)
			session, err := backend.Execute(context.Background(), "fixture prompt", ExecOptions{})
			if err != nil {
				t.Fatalf("Execute() error: %v", err)
			}

			messages, result := collectSession(t, session)
			if result.Status != "completed" {
				t.Errorf("status = %q, error = %q", result.Status, result.Error)
			}
			if result.Output != "fixture output" {
				t.Errorf("output = %q, want fixture output", result.Output)
			}
			if result.SessionID != expectedSessionIDs[provider] {
				t.Errorf("session ID = %q, want %q", result.SessionID, expectedSessionIDs[provider])
			}
			assertMessage(t, messages, MessageStatus, "running")
			assertMessage(t, messages, MessageText, "fixture output")

			logOutput := logs.String()
			if strings.Contains(logOutput, fixtureSecret) {
				t.Fatalf("stderr log leaked fixture secret: %s", logOutput)
			}
			if !strings.Contains(logOutput, "[REDACTED CREDENTIAL]") {
				t.Fatalf("stderr log did not contain redaction marker: %s", logOutput)
			}
		})
	}
}

func TestRuntimeFixtureNonZeroExitFails(t *testing.T) {
	t.Parallel()

	for _, provider := range []ProviderType{ProviderClaude, ProviderCodex, ProviderOpenCode} {
		provider := provider
		t.Run(string(provider), func(t *testing.T) {
			t.Parallel()

			backend := newFixtureBackend(t, provider, "exit_error", nil)
			session, err := backend.Execute(context.Background(), "fixture prompt", ExecOptions{})
			if err != nil {
				t.Fatalf("Execute() returned a launch error: %v", err)
			}

			_, result := collectSession(t, session)
			if result.Status != "failed" {
				t.Fatalf("status = %q, want failed (error = %q)", result.Status, result.Error)
			}
			if result.Error == "" {
				t.Fatal("failed result did not include an error")
			}
		})
	}
}

func TestRuntimeFixtureTimeoutTerminatesProcess(t *testing.T) {
	t.Parallel()

	for _, provider := range []ProviderType{ProviderClaude, ProviderCodex, ProviderOpenCode} {
		provider := provider
		t.Run(string(provider), func(t *testing.T) {
			t.Parallel()

			backend := newFixtureBackend(t, provider, "hang", nil)
			started := time.Now()
			session, err := backend.Execute(context.Background(), "fixture prompt", ExecOptions{Timeout: 500 * time.Millisecond})
			if err != nil {
				t.Fatalf("Execute() error: %v", err)
			}

			_, result := collectSession(t, session)
			if result.Status != "timeout" {
				t.Fatalf("status = %q, want timeout (error = %q)", result.Status, result.Error)
			}
			if elapsed := time.Since(started); elapsed > 5*time.Second {
				t.Fatalf("timeout cleanup took %s, want under 5s", elapsed)
			}
		})
	}
}

func TestRuntimeFixtureCancellationTerminatesProcess(t *testing.T) {
	t.Parallel()

	for _, provider := range []ProviderType{ProviderClaude, ProviderCodex, ProviderOpenCode} {
		provider := provider
		t.Run(string(provider), func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			backend := newFixtureBackend(t, provider, "hang", nil)
			session, err := backend.Execute(ctx, "fixture prompt", ExecOptions{Timeout: 10 * time.Second})
			if err != nil {
				t.Fatalf("Execute() error: %v", err)
			}

			messages := waitForRunning(t, session)
			started := time.Now()
			cancel()
			remaining, result := collectSession(t, session)
			messages = append(messages, remaining...)
			if result.Status != "aborted" {
				t.Fatalf("status = %q, want aborted (error = %q, messages = %#v)", result.Status, result.Error, messages)
			}
			if elapsed := time.Since(started); elapsed > 5*time.Second {
				t.Fatalf("cancellation cleanup took %s, want under 5s", elapsed)
			}
		})
	}
}

func TestRuntimeFixtureResumeMissFailsExplicitly(t *testing.T) {
	t.Parallel()

	for _, provider := range []ProviderType{ProviderClaude, ProviderCodex, ProviderOpenCode} {
		provider := provider
		t.Run(string(provider), func(t *testing.T) {
			t.Parallel()

			backend := newFixtureBackend(t, provider, "resume_miss", nil)
			session, err := backend.Execute(context.Background(), "fixture prompt", ExecOptions{
				ResumeSessionID: "missing-session",
			})
			if err != nil {
				t.Fatalf("Execute() returned a launch error: %v", err)
			}

			_, result := collectSession(t, session)
			if result.Status != "failed" {
				t.Fatalf("status = %q, want failed (error = %q)", result.Status, result.Error)
			}
			if !strings.Contains(strings.ToLower(result.Error), "session not found") {
				t.Fatalf("error = %q, want explicit session-not-found failure", result.Error)
			}
		})
	}
}

func newFixtureBackend(t *testing.T, provider ProviderType, scenario string, logger *slog.Logger) Backend {
	t.Helper()

	backend, err := New(string(provider), Config{
		ExecutablePath: runtimeFixturePath,
		Env:            map[string]string{fixtureScenarioEnv: scenario},
		Logger:         logger,
	})
	if err != nil {
		t.Fatalf("New(%q) error: %v", provider, err)
	}
	return backend
}

func collectSession(t *testing.T, session *Session) ([]Message, Result) {
	t.Helper()

	messageChannel := session.Messages
	resultChannel := session.Result
	messages := make([]Message, 0, 4)
	var result Result
	resultReceived := false
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()

	for messageChannel != nil || resultChannel != nil {
		select {
		case message, ok := <-messageChannel:
			if !ok {
				messageChannel = nil
				continue
			}
			messages = append(messages, message)
		case value, ok := <-resultChannel:
			if !ok {
				resultChannel = nil
				continue
			}
			if resultReceived {
				t.Fatal("session emitted more than one result")
			}
			result = value
			resultReceived = true
		case <-timer.C:
			t.Fatal("session channels did not close within 10s")
		}
	}

	if !resultReceived {
		t.Fatal("session did not emit a result")
	}
	return messages, result
}

func waitForRunning(t *testing.T, session *Session) []Message {
	t.Helper()

	messages := make([]Message, 0, 2)
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for {
		select {
		case message, ok := <-session.Messages:
			if !ok {
				t.Fatal("message channel closed before running status")
			}
			messages = append(messages, message)
			if message.Type == MessageStatus && message.Status == "running" {
				return messages
			}
		case result, ok := <-session.Result:
			if !ok {
				t.Fatal("result channel closed before running status")
			}
			t.Fatalf("session completed before running status: %#v", result)
		case <-timer.C:
			t.Fatal("session did not emit running status within 5s")
		}
	}
}

func assertMessage(t *testing.T, messages []Message, messageType MessageType, value string) {
	t.Helper()

	for _, message := range messages {
		if message.Type != messageType {
			continue
		}
		if messageType == MessageStatus && message.Status == value {
			return
		}
		if messageType == MessageText && message.Content == value {
			return
		}
	}
	t.Errorf("messages did not include %s %q: %#v", messageType, value, messages)
}
