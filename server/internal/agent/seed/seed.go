package seed

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"

	"github.com/jackc/pgx/v5/pgtype"

	dbpkg "github.com/agentra-ai/agentra/server/pkg/db/generated"
)

// Q is the minimal query surface SeedForWorkspace needs. Both *dbpkg.Queries
// and a transactional wrapper (dbpkg.Queries.WithTx) satisfy it, so callers
// can run seeding inside an existing transaction (workspace create) or
// stand-alone (daemon register, backfill).
type Q interface {
	ListAgents(ctx context.Context, workspaceID pgtype.UUID) ([]dbpkg.Agent, error)
	CreateAgent(ctx context.Context, arg dbpkg.CreateAgentParams) (dbpkg.Agent, error)
}

// SeedResult reports what SeedForWorkspace did. Created holds slugs of
// agents inserted; Skipped holds slugs that were already present. The
// fields are empty when SeedForWorkspace returns early (no runtime).
type SeedResult struct {
	Created []string
	Skipped []string
}

// SeedForWorkspace idempotently installs DefaultTemplates into the given
// workspace. Agents are matched by name (not by metadata slug — the agent
// table has no slug column), so re-running is a no-op once all templates
// are present.
//
// If runtimeID is the zero UUID, the function returns an empty SeedResult
// and a nil error: the workspace has no runtime yet (the daemon hasn't
// connected), so there's nothing we can attach the agent to. Callers
// should call SeedForWorkspace again from the daemon-register path once
// a runtime is available.
//
// On any per-template error, seeding continues with the remaining
// templates and the first error is returned. This avoids a partial
// workspace where some specialists exist and others don't — better to
// insert the rest and report the failure than abort halfway.
func SeedForWorkspace(ctx context.Context, q Q, workspaceID, ownerID, runtimeID pgtype.UUID) (SeedResult, error) {
	res := SeedResult{}
	if !runtimeID.Valid {
		return res, nil
	}

	existing, err := q.ListAgents(ctx, workspaceID)
	if err != nil {
		return res, fmt.Errorf("list existing agents: %w", err)
	}
	byName := make(map[string]bool, len(existing))
	for _, a := range existing {
		byName[a.Name] = true
	}

	// Sort by slug so Created/Skipped come back in deterministic order
	// — useful for tests and for the CLI backfill output.
	sorted := make([]Template, len(DefaultTemplates))
	copy(sorted, DefaultTemplates)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Slug < sorted[j].Slug })

	var firstErr error
	for _, t := range sorted {
		if byName[t.Name] {
			res.Skipped = append(res.Skipped, t.Slug)
			continue
		}
		toolsJSON, _ := json.Marshal(t.Tools)
		triggersJSON, _ := json.Marshal(t.Triggers)
		runtimeConfig, _ := json.Marshal(map[string]any{})
		_, err := q.CreateAgent(ctx, dbpkg.CreateAgentParams{
			WorkspaceID:        workspaceID,
			Name:               t.Name,
			Description:        t.Description,
			AvatarUrl:          pgtype.Text{},
			RuntimeMode:        "local",
			RuntimeConfig:      runtimeConfig,
			RuntimeID:          runtimeID,
			Visibility:         "workspace",
			MaxConcurrentTasks: 6,
			OwnerID:            ownerID,
			Tools:              toolsJSON,
			Triggers:           triggersJSON,
			Instructions:       t.Instructions,
			Provider:           "",
			ModelOverride:      pgtype.Text{},
			ProviderConfig:     []byte("{}"),
		})
		if err != nil {
			slog.Warn("seed: failed to create agent", "slug", t.Slug, "name", t.Name, "error", err)
			if firstErr == nil {
				firstErr = fmt.Errorf("create agent %q: %w", t.Name, err)
			}
			continue
		}
		res.Created = append(res.Created, t.Slug)
	}
	return res, firstErr
}
