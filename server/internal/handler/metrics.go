package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/agentra-ai/agentra/server/pkg/db/generated"
)

// MetricsHandler exposes agent_task_metrics aggregates to workspace owners/admins.
type MetricsHandler struct {
	Queries *db.Queries
}

func NewMetricsHandler(q *db.Queries) *MetricsHandler {
	return &MetricsHandler{Queries: q}
}

func (h *MetricsHandler) RegisterRoutes(r chi.Router) {
	r.Get("/summary", h.Summary)
	r.Get("/per-issue/{issueId}", h.PerIssue)
}

// Summary: provider × task_type × window aggregate.
func (h *MetricsHandler) Summary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	days, _ := strconv.Atoi(r.URL.Query().Get("days"))
	if days <= 0 {
		days = 30
	}

	wsID, ok := h.workspaceID(w, r)
	if !ok {
		return
	}

	rows, err := h.Queries.GetMetricsSummary(ctx, db.GetMetricsSummaryParams{
		WorkspaceID: wsID,
		Column2:     int32(days),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"days":   days,
		"providers": rows,
	})
}

// PerIssue: single issue's runs.
func (h *MetricsHandler) PerIssue(w http.ResponseWriter, r *http.Request) {
	issueID := parseUUID(chi.URLParam(r, "issueId"))
	if !issueID.Valid {
		writeError(w, http.StatusBadRequest, "invalid issue id")
		return
	}
	rows, err := h.Queries.GetMetricsByIssue(r.Context(), issueID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

func (h *MetricsHandler) workspaceID(w http.ResponseWriter, r *http.Request) (pgtype.UUID, bool) {
	ws := r.URL.Query().Get("workspace_id")
	uid := parseUUID(ws)
	if !uid.Valid {
		writeError(w, http.StatusBadRequest, "workspace_id query param required")
		return pgtype.UUID{}, false
	}
	return uid, true
}
