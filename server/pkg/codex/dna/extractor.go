// Package dna extracts structured "repository DNA" signals from a git repo.
//
// Each call to Extract returns a JSON-serialisable *DNA that can be:
//   - merged into an agent's runtime instructions (via execenv runtime_config)
//   - persisted to the agents row so seed templates stay in sync
//   - surfaced to users as a "Why did the agent do that?" audit trail
//
// The schema is intentionally additive: new signal sources can be added
// without breaking the JSON shape that downstream code (SpecialistAgent, AGENTS.md
// emitter, trust-graph dashboard, Issue #13 — Repo-DNA injection) consumes.

package dna

import (
	"math"
	"os/exec"
	"regexp"
	"sort"
	"strings"
)

// DNA is the structured, JSON-friendly output of a repo-DNA extraction.
type DNA struct {
	CommitStyle   CommitStyle   `json:"commit_style"`
	Stack         Stack         `json:"stack"`
	TestCoverage  TestCoverage  `json:"test_coverage"`
	DirLayout     DirLayout     `json:"directory_layout"`
	Conventions   []string      `json:"conventions"`
}

// CommitStyle quantifies the repo's git-log fingerprint.
type CommitStyle struct {
	PrefixDistribution map[string]float64 `json:"prefix_distribution"` // e.g. {"feat":0.45,"fix":0.30}
	ScopesActive       []string            `json:"scopes_active"`
	BodyRule           string             `json:"body_rule"` // "imperative: what + why, not how"
	FooterPatterns     []string           `json:"footer_patterns"`
}

// Stack is the best-guess technology fingerprint.
type Stack struct {
	LanguagePrimary   string `json:"language_primary"`
	LanguageSecondary string `json:"language_secondary,omitempty"`
	BackendFramework  string `json:"backend_framework,omitempty"`
	FrontendFramework string `json:"frontend_framework,omitempty"`
	DB                string `json:"db,omitempty"`
	Deployment        string `json:"deployment,omitempty"`
}

// TestCoverage indicates which layers are actually tested.
type TestCoverage struct {
	Frontend struct {
		Runner   string `json:"runner"`
		Pattern  string `json:"pattern"`
		Present  bool   `json:"present"`
	} `json:"frontend"`
	Backend struct {
		Runner   string `json:"runner"`
		Pattern  string `json:"pattern"`
		Present  bool   `json:"present"`
	} `json:"backend"`
	E2E struct {
		Runner   string `json:"runner"`
		Present  bool   `json:"present"`
	} `json:"e2e"`
}

// DirLayout captures the repo's structural conventions.
type DirLayout struct {
	Style         string   `json:"style"`          // "feature-first" / "flat"
	FrontendRoot  string   `json:"frontend_root"`  // "apps/web"
	BackendRoot   string   `json:"backend_root"`   // "server"
	FeatureDirs   []string `json:"feature_dirs"`   // observed feature directories
}

var (
	// 99 % of Agentra commits use one of these prefixes; anything not matched
	// under "other".
	prefixRe       = regexp.MustCompile(`^([a-z]+)\(([^)]+)\)|^([a-z]+):`)
	featureDirRe   = regexp.MustCompile(`^apps/web/features/([^/]+)/`)
)

// Extract runs git + filesystem probes against the repository at repoRoot.
// It never returns a partial DNA — if a signal source fails (e.g. no git, no
// go.mod), the corresponding struct stays zero-valued so callers can fall back.
func Extract(repoRoot string) *DNA {
	d := &DNA{}

	scanGitLog(repoRoot, d)
	scanFiles(repoRoot, d)

	// Derive a short English-language conventions list — this is what gets
	// merged into .agentra/AGENTS.md as human-readable guard rails.
	d.Conventions = deriveConventions(d)

	return d
}

// --- git log scanner ---

func scanGitLog(root string, d *DNA) {
	// Combined format: "<prefix>(<scope>): <subject>%x00<body>" so we can split
	// subject from body.
	out, err := exec.Command("git", "-C", root, "log", "--format=%s%n%b%x00").Output()
	if err != nil {
		return
	}

	raw := strings.Split(string(out), "\x00")

	prefixCount := map[string]int{}
	scopeCount := map[string]int{}
	bodyRules := map[string]int{}
	footerPatterns := map[string]int{}
	total := 0

	for _, entry := range raw {
		lines := strings.SplitN(strings.TrimSpace(entry), "\n", 2)
		if len(lines) == 0 || lines[0] == "" {
			continue
		}
		subject := lines[0]
		var body string
		if len(lines) > 1 {
			body = lines[1]
		}

		// --- prefix + scope ---
		m := prefixRe.FindStringSubmatch(subject)
		if m == nil {
			prefixCount["other"]++
			total++
			continue
		}
		prefix := m[1] // feat, fix, docs, ...
		if prefix == "" {
			prefix = m[3] // colon-style: "docs:", "ci:"
		}
		prefixCount[prefix]++
		total++

		scope := m[2] // may be empty for colon-style
		if scope != "" {
			scopeCount[scope]++
		}

		// --- body ---
		switch {
		case strings.Contains(body, "↳") || strings.Contains(body, "why:"):
			bodyRules["what + why"]++
		case strings.Contains(body, "Implements:") || strings.Contains(body, "Implements: #"):
			bodyRules["lists ticket"]++
		case strings.Contains(body, "Co-Authored-By:"):
			bodyRules["co-authored-by"]++
		default:
			bodyRules["what + why"]++
		}

		// --- footer patterns ---
		for _, ln := range strings.Split(body, "\n") {
			ln = strings.TrimSpace(ln)
			switch {
			case strings.HasPrefix(ln, "Issue #") || strings.HasPrefix(ln, "Implements:"):
				footerPatterns["issue/ticket ref"]++
			case strings.HasPrefix(ln, "Co-Authored-By:"):
				footerPatterns["co-authored-by"]++
			case strings.HasPrefix(ln, "BREAKING CHANGE"):
				footerPatterns["breaking-change"]++
			}
		}
	}

	if total > 0 {
		d.CommitStyle.PrefixDistribution = map[string]float64{}
		for k, v := range prefixCount {
			d.CommitStyle.PrefixDistribution[k] = round2(float64(v) / float64(total))
		}
	}

	d.CommitStyle.ScopesActive = sortedStringIntKeys(scopeCount)
	d.CommitStyle.BodyRule = dominantKey(bodyRules, "imperative: what + why, not how")
	d.CommitStyle.FooterPatterns = sortedStringIntKeys(footerPatterns)
}

// --- filesystem scanner ---

func scanFiles(root string, d *DNA) {
	// --- stack ---
	// Go backend conventionally lives at server/go.mod in a monorepo.
	if fileExists(root, "server/go.mod") || fileExists(root, "go.mod") {
		d.Stack.LanguagePrimary = "Go"
		d.Stack.BackendFramework = "Chi"
	}
	if fileExists(root, "apps/web/package.json") || fileExists(root, "package.json") {
		d.Stack.LanguageSecondary = "TypeScript"
		d.Stack.FrontendFramework = "Next.js"
	}
	if fileExists(root, "docker-compose.yml") || fileExists(root, "docker-compose.yaml") {
		d.Stack.Deployment = "Docker Compose"
	}
	if fileExists(root, "server/migrations") || fileExists(root, "migrations") {
		d.Stack.DB = "PostgreSQL + pgvector"
	}

	// --- tests ---
	d.TestCoverage.Backend.Runner = "go test"
	d.TestCoverage.Backend.Pattern = "*_test.go, TestMain + test DB fixtures"
	d.TestCoverage.Backend.Present = globExists(root, "server/**/*_test.go") || globExists(root, "**/*_test.go")

	d.TestCoverage.Frontend.Runner = "vitest"
	d.TestCoverage.Frontend.Pattern = "*.test.ts(x), mock external only"
	d.TestCoverage.Frontend.Present = globExists(root, "apps/web/**/*.test.ts") ||
		globExists(root, "apps/web/**/*.test.tsx")

	d.TestCoverage.E2E.Runner = "playwright"
	d.TestCoverage.E2E.Present = dirExists(root, "e2e") || dirExists(root, "apps/web/e2e") || dirExists(root, "server/e2e")

	// --- dir layout ---
	d.DirLayout.FeatureDirs = listFeatureDirs(root)
	if len(d.DirLayout.FeatureDirs) > 0 {
		d.DirLayout.Style = "feature-first"
	}
	d.DirLayout.FrontendRoot = "apps/web"
	d.DirLayout.BackendRoot = "server"
}

// --- conventions derivation ---

func deriveConventions(d *DNA) []string {
	var out []string

	if d.CommitStyle.PrefixDistribution["feat"]+
		d.CommitStyle.PrefixDistribution["fix"] > 0.6 {
		out = append(out,
			"本仓库 commit 风格规整 (type(scope): imperative) -- 请遵循",
		)
	}

	if d.Stack.LanguageSecondary == "TypeScript" {
		out = append(out,
			"前端 import 用 @/ alias,禁止 feature↔feature 直接引用",
			"Zustand 管理 client state; React Context 仅用于 WS lifecycle",
		)
	}

	if d.TestCoverage.Backend.Present {
		out = append(out,
			"新后端代码请带测试 (go test ./...) 并确保 CI 通过",
		)
	}
	if d.TestCoverage.Frontend.Present {
		out = append(out,
			"新前端组件请带单测 (pnpm test,基于 Vitest)",
		)
	}

	out = append(out,
		"禁止兼容性层、fallback paths、双写逻辑 (per CLAUDE.md)",
		"stores 内禁止调用 useRouter",
	)

	return out
}

// --- small filesystem helpers ---

func fileExists(root, name string) bool {
	buf, err := exec.Command("test", "-f", root+"/"+name).Output()
	_ = buf
	return err == nil
}

func dirExists(root, name string) bool {
	err := exec.Command("test", "-d", root+"/"+name).Run()
	return err == nil
}

// globExists runs `git ls-files` so it respects .gitignore and avoids walking
// node_modules.
func globExists(root, pattern string) bool {
	out, err := exec.Command("git", "-C", root, "ls-files", pattern).Output()
	return err == nil && len(out) > 0
}

func listFeatureDirs(root string) []string {
	out, err := exec.Command("git", "-C", root, "ls-files", "apps/web/features/").Output()
	if err != nil {
		return nil
	}

	seen := map[string]struct{}{}
	for _, line := range strings.Split(string(out), "\n") {
		if m := featureDirRe.FindStringSubmatch(line); m != nil {
			seen[m[1]] = struct{}{}
		}
	}
	dirs := make([]string, 0, len(seen))
	for d := range seen {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)
	return dirs
}

// --- generic helpers ---

func sortedStringIntKeys(m map[string]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedStringFloatKeys(m map[string]float64) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func dominantKey(m map[string]int, fallback string) string {
	if len(m) == 0 {
		return fallback
	}
	bestK, bestV := "", 0
	for k, v := range m {
		if v > bestV {
			bestK, bestV = k, v
		}
	}
	return bestK
}

func round2(f float64) float64 {
	return math.Round(f*100) / 100
}
