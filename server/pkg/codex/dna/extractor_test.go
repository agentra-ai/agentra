package dna

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestExtract_Agentra(t *testing.T) {
	repoRoot := findRepoRoot(t)
	if repoRoot == "" {
		t.Skip("repo root not found (running outside git repo)")
	}

	dna := Extract(repoRoot)

	if dna == nil {
		t.Fatal("Extract returned nil")
	}

	// --- required signals ---
	if dna.Stack.LanguagePrimary != "Go" {
		t.Errorf("LanguagePrimary = %q, want Go", dna.Stack.LanguagePrimary)
	}
	if dna.Stack.LanguageSecondary != "TypeScript" {
		t.Errorf("LanguageSecondary = %q, want TypeScript", dna.Stack.LanguageSecondary)
	}

	if dna.CommitStyle.PrefixDistribution == nil {
		t.Fatal("CommitStyle.PrefixDistribution is nil")
	}

	// Conventional commits (feat+fix+...) should dominate.
	conventional := float64(0)
	for _, k := range []string{"feat", "fix", "refactor", "test", "docs", "chore"} {
		conventional += dna.CommitStyle.PrefixDistribution[k]
	}
	if conventional < 0.5 {
		t.Errorf("conventional prefix share = %.2f, want >= 0.5 (dist=%v)",
			conventional, dna.CommitStyle.PrefixDistribution)
	}

	if len(dna.CommitStyle.ScopesActive) == 0 {
		t.Error("ScopesActive is empty")
	}

	// Directory layout
	if dna.DirLayout.Style != "feature-first" {
		t.Errorf("DirLayout.Style = %q, want feature-first", dna.DirLayout.Style)
	}
	if len(dna.DirLayout.FeatureDirs) == 0 {
		t.Error("FeatureDirs is empty")
	}

	// Tests
	if !dna.TestCoverage.Backend.Present {
		t.Error("Backend tests should be present")
	}
	if !dna.TestCoverage.Frontend.Present {
		t.Error("Frontend tests should be present")
	}

	// Conventions list should include the hard rule.
	found := false
	for _, c := range dna.Conventions {
		if strings.Contains(c, "兼容性层") {
			found = true
		}
	}
	if !found {
		t.Errorf("Conventions should mention '兼容性层' rule; got: %v", dna.Conventions)
	}

	// JSON serialises cleanly.
	b, err := json.MarshalIndent(dna, "", "  ")
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if len(b) < 100 {
		t.Errorf("JSON suspiciously short: %s", string(b))
	}
}

// findRepoRoot walks up from CWD until it finds .git.
func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(dir + "/.git"); err == nil {
			return dir
		}
		parent := dir[:strings.LastIndex(dir, "/")]
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
