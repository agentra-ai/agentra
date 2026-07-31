package main

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

var expectedRootCommands = []string{
	"agent",
	"attachment",
	"auth",
	"completion",
	"config",
	"conventions",
	"daemon",
	"doctor",
	"eval",
	"git",
	"help",
	"issue",
	"login",
	"loop",
	"repo",
	"runtime",
	"setup",
	"skill",
	"update",
	"version",
	"workspace",
}

func TestRootCommandInventoryIsUniqueAndDocumented(t *testing.T) {
	rootCmd.InitDefaultCompletionCmd()
	rootCmd.InitDefaultHelpCmd()

	assertUniqueCommandChildren(t, rootCmd)
	got := childCommandNames(rootCmd)
	want := append([]string(nil), expectedRootCommands...)
	sort.Strings(want)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("root commands = %v, want %v", got, want)
	}

	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	for _, path := range []string{
		filepath.Join(repoRoot, "CLI_AND_DAEMON.md"),
		filepath.Join(repoRoot, "docs", "zh", "cli.md"),
	} {
		documented := documentedRootCommands(t, path)
		if strings.Join(documented, "\n") != strings.Join(want, "\n") {
			t.Errorf("%s command inventory = %v, want %v", path, documented, want)
		}
	}
}

func TestRootWithoutArgumentsRendersGeneratedHelp(t *testing.T) {
	rootCmd.InitDefaultCompletionCmd()
	rootCmd.InitDefaultHelpCmd()

	var output bytes.Buffer
	rootCmd.SetOut(&output)
	t.Cleanup(func() { rootCmd.SetOut(nil) })
	if err := rootCmd.RunE(rootCmd, nil); err != nil {
		t.Fatalf("root RunE() error = %v", err)
	}

	help := output.String()
	for _, name := range expectedRootCommands {
		if !strings.Contains(help, name) {
			t.Errorf("generated help does not include %q", name)
		}
	}
	if strings.Contains(help, "可用命令:") {
		t.Fatal("root command still renders the removed handwritten command list")
	}
}

func assertUniqueCommandChildren(t *testing.T, command *cobra.Command) {
	t.Helper()
	seen := make(map[string]struct{})
	for _, child := range command.Commands() {
		name := child.Name()
		if _, exists := seen[name]; exists {
			t.Errorf("%s registers child command %q more than once", command.CommandPath(), name)
			continue
		}
		seen[name] = struct{}{}
		assertUniqueCommandChildren(t, child)
	}
}

func childCommandNames(command *cobra.Command) []string {
	names := make([]string, 0, len(command.Commands()))
	for _, child := range command.Commands() {
		names = append(names, child.Name())
	}
	sort.Strings(names)
	return names
}

func documentedRootCommands(t *testing.T, path string) []string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	const start = "<!-- CLI_ROOT_COMMANDS_START -->"
	const end = "<!-- CLI_ROOT_COMMANDS_END -->"
	text := string(content)
	startIndex := strings.Index(text, start)
	endIndex := strings.Index(text, end)
	if startIndex < 0 || endIndex <= startIndex {
		t.Fatalf("%s is missing the CLI command inventory markers", path)
	}

	commandPattern := regexp.MustCompile("(?m)^\\| `agentra ([a-z-]+)` \\|")
	matches := commandPattern.FindAllStringSubmatch(text[startIndex:endIndex], -1)
	names := make([]string, 0, len(matches))
	for _, match := range matches {
		names = append(names, match[1])
	}
	sort.Strings(names)
	return names
}
