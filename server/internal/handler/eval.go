package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/agentra-ai/agentra/server/internal/eval"
	"github.com/agentra-ai/agentra/server/internal/eval/seed"
	"github.com/agentra-ai/agentra/server/internal/middleware"
	db "github.com/agentra-ai/agentra/server/pkg/db/generated"
)

// EvalServer wraps chi.Router so the handler can mount subroutes.
type EvalServer struct {
	*chi.Mux
	Queries *db.Queries
}

// NewEvalServer mounts the eval API under /api/eval.
func NewEvalServer(q *db.Queries, workspaceRoot string) *EvalServer {
	r := chi.NewRouter()
	s := &EvalServer{Mux: r, Queries: q}

	// Seed default golden dataset (owner/admin only).
	r.With(middleware.RequireWorkspaceRole(s.Queries, "owner", "admin")).
		Post("/seed", s.handleSeed)

	// List current golden cases.
	r.With(middleware.RequireWorkspaceMember(s.Queries)).
		Get("/cases", s.handleListCases)

	// Trigger a run (owner/admin starts the benchmark).
	r.With(middleware.RequireWorkspaceRole(s.Queries, "owner", "admin")).
		Post("/run", s.handleRun)

	// Latest run result.
	r.With(middleware.RequireWorkspaceMember(s.Queries)).
		Get("/runs/latest", s.handleLatestRun)

	// Regression gate — returns 503 if latest run regressed vs previous.
	r.With(middleware.RequireWorkspaceMember(s.Queries)).
		Get("/gate", s.handleGate)

	return s
}

func (s *EvalServer) handleSeed(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	wsID := chi.URLParam(r, "workspaceId")

	// Idempotent: only seed if no cases exist.
	existing, err := s.Queries.ListEvalGoldenIssues(ctx, mustUUID(wsID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(existing) > 0 {
		writeJSON(w, http.StatusOK, map[string]any{
			"seeded": false, "count": len(existing), "message": "already seeded",
		})
		return
	}

	cases := seed.DefaultCases
	for i := range cases {
		expected := pgtype.Text{String: cases[i].ExpectedTest, Valid: true}
		_, err := s.Queries.CreateEvalGoldenIssue(ctx, db.CreateEvalGoldenIssueParams{
			Slug:        cases[i].Slug,
			Category:    cases[i].Category,
			WorkspaceID: mustUUID(wsID),
			Title:       cases[i].Title,
			Description: cases[i].Description,
			ExpectedTest: expected,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"seeded": true, "count": len(cases),
	})
}

func (s *EvalServer) handleListCases(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	wsID := chi.URLParam(r, "workspaceId")

	cases, err := s.Queries.ListEvalGoldenIssues(ctx, mustUUID(wsID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cases)
}

func (s *EvalServer) handleRun(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	wsID := chi.URLParam(r, "workspaceId")

	// Headless mode: score the golden dataset against the static repo-DNA.
	ev := eval.New(/* workspaceRoot */ "")
	ev.Headless = true

	cases := seed.DefaultCases
	evalCases := make([]eval.GoldenIssue, 0, len(cases))
	for _, c := range cases {
		evalCases = append(evalCases, eval.GoldenIssue{
			Slug: c.Slug, Category: c.Category, Title: c.Title,
			Description: c.Description, ExpectedTest: c.ExpectedTest,
		})
	}
	report := ev.RunHeadless(ctx, evalCases)

	// Persist run.
	if _, err := s.Queries.CreateEvalRun(r.Context(), db.CreateEvalRunParams{
		WorkspaceID: mustUUID(wsID),
		TotalCases:  int32(report.Total),
		Passed:      int32(report.Passed),
		Failed:      int32(report.Failed),
		Score:       pgtype.Numeric{}, // estimate via provider cost API; 0 placeholder
		Summary:     mustMarshal(report),
		Status:      "completed",
	}); err != nil {
		// non-fatal; report still returned
	}

	writeJSON(w, http.StatusCreated, report)
}

func (s *EvalServer) handleLatestRun(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	wsID := chi.URLParam(r, "workspaceId")

	run, err := s.Queries.GetLatestEvalRun(ctx, mustUUID(wsID))
	if err != nil {
		writeError(w, http.StatusNotFound, "no eval run yet")
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func (s *EvalServer) handleGate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	wsID := chi.URLParam(r, "workspaceId")

	regressed, err := s.Queries.DetectEvalRegression(ctx, mustUUID(wsID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if regressed.Regressed.Valid && regressed.Regressed.Bool {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"regressed": true,
			"message":   "eval score dropped vs previous run",
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"regressed": false})
}

// --- helpers ---

func mustUUID(s string) pgtype.UUID {
	var u pgtype.UUID
	_ = u.Scan(s)
	return u
}

func mustMarshal(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
