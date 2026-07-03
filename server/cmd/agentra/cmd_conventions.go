package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var conventionsCmd = &cobra.Command{
	Use:   "conventions",
	Short: "Manage .agentra/AGENT.md project convention manifests",
}

var conventionsInitCmd = &cobra.Command{
	Use:   "init-agent-conventions [path]",
	Short: "Scaffold .agentra/AGENT.md from current repo signals",
	Long: `Scan the working directory for conventional signals — README, package.json,
go.mod, Cargo.toml, pyproject.toml — and write a starter .agentra/AGENT.md
that the daemon will auto-inject into agent prompts at task start.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runConventionsInit,
}

var conventionsValidateCmd = &cobra.Command{
	Use:   "validate-agent-conventions [path]",
	Short: "Validate an existing .agentra/AGENT.md against the spec",
	Args:  cobra.MaximumNArgs(1),
	RunE:   runConventionsValidate,
}

func init() {
	conventionsCmd.AddCommand(conventionsInitCmd, conventionsValidateCmd)
}

func runConventionsInit(cmd *cobra.Command, args []string) error {
	target := "."
	if len(args) > 0 {
		target = args[0]
	}
	target, err := filepath.Abs(target)
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}

	out := filepath.Join(target, ".agentra", "AGENT.md")
	if _, err := os.Stat(out); err == nil {
		return fmt.Errorf("%s already exists — delete it first if you want to regenerate", out)
	}

	conventions := scanRepoSignals(target)

	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return fmt.Errorf("create .agentra/: %w", err)
	}
	if err := os.WriteFile(out, []byte(conventions), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", out, err)
	}

	fmt.Printf("Scaffold written → %s\n", out)
	fmt.Println("Edit it to taste, then re-run `agentra validate-agent-conventions` to verify structure.")
	return nil
}

func scanRepoSignals(root string) string {
	var (
		hasGo        = exists(root, "go.mod")
		hasNode      = exists(root, "package.json")
		hasPython    = exists(root, "pyproject.toml") || exists(root, "requirements.txt")
		hasCargo     = exists(root, "Cargo.toml")
		hasReadme    = exists(root, "README.md")
		hasDocker    = exists(root, "docker-compose.yml") || exists(root, "docker-compose.yaml")
		hasMigration = hasDir(root, "migrations")
		hasFeatures  = hasDir(root, "features")
		hasApps      = hasDir(root, "apps")
		hasServer    = hasDir(root, "server")
		hasTests     = hasDir(root, "e2e") || hasDir(root, "tests")
	)

	var sb strings.Builder
	sb.WriteString("# .agentra/AGENT.md — Project Knowledge Manifest\n\n")

	// ----- Stack -----
	sb.WriteString("## Stack\n")
	switch {
	case hasGo && hasNode:
		sb.WriteString("- Language: Go (backend) + TypeScript (frontend)\n")
		sb.WriteString("- Database: PostgreSQL + pgvector\n")
		if hasDocker {
			sb.WriteString("- Deployment: Docker Compose\n")
		}
	case hasGo:
		sb.WriteString("- Language: Go\n")
		if hasMigration {
			sb.WriteString("- Database: PostgreSQL\n")
		}
	case hasNode:
		sb.WriteString("- Language: TypeScript\n")
	case hasPython:
		sb.WriteString("- Language: Python\n")
	case hasCargo:
		sb.WriteString("- Language: Rust\n")
	default:
		sb.WriteString("- Language: <add primary language>\n")
	}
	sb.WriteString("\n")

	// ----- Import Conventions -----
	sb.WriteString("## Import Conventions\n")
	switch {
	case hasGo && hasNode:
		sb.WriteString("- Frontend: `@/` alias maps to `apps/web/`\n")
		sb.WriteString("- Backend: Go module path `github.com/<org>/<repo>/server/internal/...`\n")
	case hasFeatures:
		sb.WriteString("- Use feature-first layout; never cross-import between feature boundaries\n")
	case hasApps:
		sb.WriteString("- Apps are self-contained — import via `@/` alias, not relative cross-app paths\n")
	}
	sb.WriteString("\n")

	// ----- State Management -----
	sb.WriteString("## State Management\n")
	switch {
	case hasNode && hasFeatures:
		sb.WriteString("- Zustand for client state\n- React Context only for connection lifecycle\n- Local useState for component-scoped UI only\n")
	default:
		sb.WriteString("- Add how state is managed in this codebase\n")
	}
	sb.WriteString("\n")

	// ----- Forbidden Patterns -----
	sb.WriteString("## Forbidden Patterns\n")
	sb.WriteString("- Don't add compatibility layers or fallback paths (per CLAUDE.md)\n")
	sb.WriteString("- Don't dual-write to old + new paths\n")
	if hasGo {
		sb.WriteString("- Don't call `useRouter` from zustand stores (React rule)\n")
	}
	sb.WriteString("\n")

	// ----- Preferred Patterns -----
	sb.WriteString("## Preferred Patterns\n")
	if hasFeatures {
		sb.WriteString("- Feature-first: `features/<domain>/{components,hooks,stores,config}`\n")
	}
	sb.WriteString("- Optimistic mutations with rollback on failure\n")
	sb.WriteString("- WebSocket sync = invalidate + refetch; never dual-write to SOT\n")
	sb.WriteString("\n")

	// ----- Testing -----
	sb.WriteString("## Testing\n")
	switch {
	case hasNode && hasGo && hasTests:
		sb.WriteString("- Frontend: Vitest (mock external deps only)\n")
		sb.WriteString("- Backend: `go test ./...` (fixtures in test DB)\n")
		sb.WriteString("- E2E: Playwright (requires full stack running)\n")
	case hasNode:
		sb.WriteString("- Frontend: Vitest (mock external deps only)\n")
	case hasGo:
		sb.WriteString("- Backend: `go test ./...` (fixtures in test DB)\n")
	}
	sb.WriteString("\n")

	// ----- Architecture -----
	if hasServer && hasApps {
		sb.WriteString("## Architecture Decisions\n")
		sb.WriteString("- Standalone Go Chi backend + self-contained Next.js frontend (no shared package deps)\n")
		sb.WriteString("- Daemon executes agent CLIs locally; WebSocket hub broadcasts events in real time\n")
	}

	if hasReadme {
		sb.WriteString("\n## References\n")
		sb.WriteString("- README.md — overview + quick start\n")
	}
	if hasDocker {
		sb.WriteString("- docker-compose.yml — full stack topology\n")
	}

	return sb.String()
}

func runConventionsValidate(cmd *cobra.Command, args []string) error {
	path := ".agentra/AGENT.md"
	if len(args) > 0 {
		path = args[0]
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	var errors []string
	sections := []string{
		"Stack", "Import Conventions", "State Management",
		"Forbidden Patterns", "Preferred Patterns", "Testing",
	}
	content := string(data)

	if !strings.Contains(content, "## Stack") {
		errors = append(errors, "missing required section: ## Stack")
	}
	for _, s := range sections[1:] {
		if strings.Contains(content, "## "+s) {
			fmt.Printf("  ✓ %s\n", s)
		} else {
			fmt.Printf("  ○ %s (optional)\n", s)
		}
	}

	if len(errors) > 0 {
		fmt.Println("Validation failed:")
		for _, e := range errors {
			fmt.Printf("  ✗ %s\n", e)
		}
		os.Exit(1)
	}
	fmt.Printf("✓ %s passes spec validation\n", path)
	return nil
}

// --- small file-system helpers ---

func exists(path, name string) bool {
	fi, err := os.Stat(filepath.Join(path, name))
	return err == nil && !fi.IsDir()
}

func hasDir(root, name string) bool {
	fi, err := os.Stat(filepath.Join(root, name))
	return err == nil && fi.IsDir()
}

// unused but makes the package self-contained when conventions live in
// their own subcommand tree.
var _ = context.Background
