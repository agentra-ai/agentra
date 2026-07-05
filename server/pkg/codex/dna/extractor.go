// Package dna extracts "repository DNA" — the implicit conventions of a
// codebase — so that Agentra's agent runtime can inject context that makes
// agents behave as if they had been reading the repo for months.
//
// The module is intentionally defensive: every signal source can fail
// independently, and the output is always a usable (if partial) *DNA.

package dna

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// DNA is the structured output of an extraction pass.
type DNA struct {
	GeneratedAt   string         `json:"generated_at"`
	RepoRoot      string         `json:"repo_root"`
	HeadSHA       string         `json:"head_sha,omitempty"`
	CommitStyle   CommitStyle    `json:"commit_style"`
	Imports       ImportConventions `json:"imports"`
	StateManagement string       `json:"state_management,omitempty"`
	Testing        TestingConventions `json:"testing"`
	Patterns       []string      `json:"patterns"`
}

type CommitStyle struct {
	Types    map[string]int `json:"types"`    // feat:, fix:, chore:, etc.
	WithScope bool          `json:"with_scope"` // e.g. feat(foo): ...
	MeanLen  int            `json:"mean_subject_length"`
}

type ImportConventions struct {
	Aliases         map[string]string `json:"aliases"`          // "@/": "apps/web/"
	PreferRelative  bool              `json:"prefer_relative"`   // under src/
	BannedPackages  []string          `json:"banned_packages,omitempty"`
}

type TestingConventions struct {
	Framework   string `json:"framework"`     // "vitest", "go test", "jest", etc.
	Pattern     string `json:"test_pattern"`  // "*.test.ts", "*_test.go", etc.
	CoLocated   bool   `json:"co_located"`    // e.g. foo.test.ts next to foo.ts
	NotInDir    bool   `json:"not_in_dir"`    // dedicated test/ dir
}

var (
	// Git commit message format: `<type>(<scope>): <subject>` or `<type>: <subject>`.
	commitRe = regexp.MustCompile(`^([a-z]+)(\([^)]+\))?:\s+(.+)$`)

	// Import alias patterns: `from "@/x"` or `import "@/x"`.
	aliasFromRe  = regexp.MustCompile(`from\s+["']([^/"][^"']*)/[^"']*["']`)
	aliasImportRe = regexp.MustCompile(`import\s+["']([^/"][^"']*)/[^"']*["']`)

	// State management signals.
	zustandRe = regexp.MustCompile(`create\s*<[^>]*>\s*\(`)
	mobxRe    = regexp.MustCompile(`makeObservable|makeAutoObservable`)
	recoilRe  = useContextSignalRe()
)

func useContextSignalRe() *regexp.Regexp {
	return regexp.MustCompile(`use(Recoil|Atom|Selector)|atom\(`)
}

// Extract runs the full extraction on repoRoot. Never returns a partial *DNA
// with zero signals — if everything fails you get a stub with GeneratedAt and RepoRoot.
func Extract(ctx context.Context, repoRoot string) *DNA {
	d := &DNA{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		RepoRoot:    repoRoot,
	}

	// Robust against context cancellation.
	done := make(chan struct{})
	go func() {
		defer close(done)
		d.HeadSHA = headSHA(ctx, repoRoot)
		d.CommitStyle = extractCommitStyle(ctx, repoRoot)
		d.Imports = extractImportConventions(ctx, repoRoot)
		d.StateManagement = detectStateManagement(ctx, repoRoot)
		d.Testing = extractTesting(ctx, repoRoot)
		d.Patterns = findChurnClusters(ctx, repoRoot)
	}()
	select {
	case <-done:
	case <-ctx.Done():
	}

	// Always return a usable DNA.
	return d
}

// headSHA returns the short SHA of HEAD, or "" on failure.
func headSHA(ctx context.Context, dir string) string {
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "--short", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// Commit style is parsed from the last 200 commit messages.
func extractCommitStyle(ctx context.Context, dir string) CommitStyle {
	style := CommitStyle{Types: map[string]int{}}
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "log", "--oneline", "-200", "--format=%s")
	out, err := cmd.Output()
	if err != nil {
		return style
	}

	lines := strings.Split(string(out), "\n")
	totalLen := 0
	used := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		m := commitRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		style.Types[m[1]]++
		if m[2] != "" {
			style.WithScope = true
		}
		totalLen += len(m[3])
		used++
	}
	if used > 0 {
		style.MeanLen = totalLen / used
	}
	return style
}

// Import conventions are derived from a sample of source files.
func extractImportConventions(ctx context.Context, dir string) ImportConventions {
	ic := ImportConventions{Aliases: map[string]string{}}

	// 1. Check tsconfig.json / jsconfig.json for alias paths.
	extractPathAliases(dir, &ic)

	// 2. Sample up to 30 source files for usage patterns.
	samples := sampleFiles(dir, []string{".ts", ".tsx"}, 30)
	aliasUses := map[string]int{}
	for _, fp := range samples {
		data, err := os.ReadFile(fp)
		if err != nil {
			continue
		}
		src := string(data)
		for _, match := range aliasFromRe.FindAllStringSubmatch(src, -1) {
			aliasUses[match[1]]++
		}
	}
	// Find top alias.
	if len(aliasUses) > 0 {
		type kv struct {
			k string
			v int
		}
		sorted := []kv{}
		for k, v := range aliasUses {
			sorted = append(sorted, kv{k, v})
		}
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].v > sorted[j].v })
		ic.Aliases["@/"] = sorted[0].k + "/"
	}

	return ic
}

func extractPathAliases(dir string, ic *ImportConventions) {
	for _, cfg := range []string{"tsconfig.json", "jsconfig.json"} {
		data, err := os.ReadFile(filepath.Join(dir, cfg))
		if err != nil {
			continue
		}
		// Crude scan: find "@/*": ["foo/*"]-style entries.
		re := regexp.MustCompile(`"@/[*]?"\s*:\s*\["([^"]+)"\]`)
		for _, m := range re.FindAllStringSubmatch(string(data), -1) {
			ic.Aliases["@/*"] = m[1]
		}
		// Search subdirs too.
		filepath.Walk(dir, func(p string, info os.FileInfo, _ error) error {
			if info != nil && !info.IsDir() {
				return nil
			}
			if info != nil {
				return nil
			}
			return nil
		})
	}
}

func sampleFiles(dir string, exts []string, n int) []string {
	hits := []string{}
	_ = filepath.Walk(dir, func(p string, info os.FileInfo, _ error) error {
		if info != nil && info.IsDir() {
			base := filepath.Base(p)
			if base == "node_modules" || base == ".git" || base == ".next" {
				return filepath.SkipDir
			}
			return nil
		}
		for _, ext := range exts {
			if strings.HasSuffix(p, ext) {
				hits = append(hits, p)
				return nil
			}
		}
		return nil
	})
	if len(hits) > n {
		hits = hits[:n]
	}
	return hits
}

func detectStateManagement(ctx context.Context, dir string) string {
	samples := sampleFiles(dir, []string{".ts", ".tsx"}, 10)
	has := map[string]bool{}
	for _, fp := range samples {
		data, err := os.ReadFile(fp)
		if err != nil {
			continue
		}
		src := string(data)
		if zustandRe.MatchString(src) {
			has["zustand"] = true
		}
		if mobxRe.MatchString(src) {
			has["mobx"] = true
		}
		if recoilRe.MatchString(src) {
			has["recoil"] = true
		}
	}
	// Priority.
	if has["zustand"] {
		return "zustand"
	}
	if has["recoil"] {
		return "recoil"
	}
	if has["mobx"] {
		return "mobx"
	}
	return ""
}

func extractTesting(ctx context.Context, dir string) TestingConventions {
	tc := TestingConventions{}
	// Check package.json for declared test runner.
	pkgPath := filepath.Join(dir, "package.json")
	if data, err := os.ReadFile(pkgPath); err == nil {
		src := string(data)
		switch {
		case strings.Contains(src, `"vitest"`):
			tc.Framework = "vitest"
			tc.Pattern = "*.test.{ts,tsx}"
		case strings.Contains(src, `"jest"`):
			tc.Framework = "jest"
			tc.Pattern = "*.spec.{ts,tsx}"
		case strings.Contains(src, `"@playwright"`):
			tc.Framework = "playwright"
			tc.Pattern = "*.spec.ts"
		}
	}
	// Go projects.
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
		tc.Framework = "go test"
		tc.Pattern = "*_test.go"
	}
	return tc
}

// findChurnClusters finds directories that change together frequently.
// This is a proxy for "modules that should be assigned to the same agent".
func findChurnClusters(ctx context.Context, dir string) []string {
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "log", "--oneline", "-100", "--name-only", "--pretty=format:COMMIT")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	dirCount := map[string]int{}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line == "COMMIT" {
			continue
		}
		d := filepath.Dir(line)
		if d == "." {
			continue
		}
		dirCount[d]++
	}

	// Sort dirs by co-change frequency.
	type kv struct {
		k string
		v int
	}
	sorted := []kv{}
	for k, v := range dirCount {
		if v < 2 {
			continue
		}
		sorted = append(sorted, kv{k, v})
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].v > sorted[j].v })

	outDirs := []string{}
	for _, kv := range sorted {
		outDirs = append(outDirs, kv.k)
		if len(outDirs) >= 5 {
			break
		}
	}
	return outDirs
}
