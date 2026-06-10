package handler

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/agentra-ai/agentra/server/internal/agent/seed"
)

// SeedSpecialistsResponse is the JSON shape returned by
// POST /api/workspaces/{id}/seed-specialists. Created/Skipped hold slugs of
// the default specialist templates; either is empty when the call was a
// no-op (no runtime registered yet, or nothing to seed).
type SeedSpecialistsResponse struct {
	Created []string `json:"created"`
	Skipped []string `json:"skipped"`
}

// SeedSpecialists installs the default specialist agent templates into the
// named workspace. It mirrors what the daemon-register path does on first
// connection — this endpoint exists so operators can backfill workspaces
// that already had daemons running before the auto-seed was wired up, or
// recover after a failed seed run.
//
// Authorization: any workspace member can call this. The seed only inserts
// new agents; it does not modify or delete anything, so membership is the
// right gate (matches the spirit of "any member can create agents").
//
// Behavior:
//   - Idempotent: re-running is a no-op once all templates are present.
//   - If the workspace has no agent runtime registered (no daemon has
//     connected), returns 200 with empty arrays — there is no runtime to
//     attach the agents to. The seed function itself short-circuits on a
//     zero runtimeID.
//   - On per-template insert failures, seeding continues with the remaining
//     templates and the first error is surfaced as 500.
func (h *Handler) SeedSpecialists(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "id")
	wsUUID := parseUUID(workspaceID)
	if _, ok := h.requireWorkspaceMember(w, r, workspaceID, "workspace not found"); !ok {
		return
	}

	// Find any registered runtime for this workspace. The daemon-register
	// path uses the just-registered runtime; for an explicit backfill we
	// pick any active runtime — specialists are runtime-agnostic.
	runtimes, err := h.Queries.ListAgentRuntimes(r.Context(), wsUUID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusInternalServerError, "failed to list runtimes")
		return
	}
	var runtimeID pgtype.UUID
	if len(runtimes) > 0 {
		runtimeID = runtimes[0].ID
	}

	owner := firstWorkspaceMember(r.Context(), h.Queries, wsUUID)
	res, err := seed.SeedForWorkspace(r.Context(), h.Queries, wsUUID, owner, runtimeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "seed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, SeedSpecialistsResponse{
		Created: res.Created,
		Skipped: res.Skipped,
	})
}
