package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	db "github.com/agentra-ai/agentra/server/pkg/db/generated"
)

type TraceHandler struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

func NewTraceHandler(pool *pgxpool.Pool, queries *db.Queries) *TraceHandler {
	return &TraceHandler{pool: pool, queries: queries}
}

func (h *TraceHandler) RegisterRoutes(r chi.Router) {
	r.Get("/tasks/{id}/trace", h.GetTaskTrace)
	r.Get("/tasks/{id}/trace/summary", h.GetTaskTraceSummary)
	r.Get("/agents/{id}/traces", h.ListAgentTraces)
	r.Get("/traces/analytics", h.GetTraceAnalytics)
}

func (h *TraceHandler) GetTaskTrace(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "id")
	runID := r.URL.Query().Get("run_id")

	steps, err := h.queries.ListTraceSteps(r.Context(), pgtype.UUID{Bytes: uuid.MustParse(runID), Valid: true})
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	json.NewEncoder(w).Encode(map[string]any{
		"task_id": taskID,
		"run_id":  runID,
		"steps":   steps,
	})
}

func (h *TraceHandler) GetTaskTraceSummary(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "id")
	runID := r.URL.Query().Get("run_id")

	steps, err := h.queries.ListTraceSteps(r.Context(), pgtype.UUID{Bytes: uuid.MustParse(runID), Valid: true})
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	summary := buildSummary(steps)
	summary.TaskID = taskID
	summary.RunID = runID

	json.NewEncoder(w).Encode(summary)
}

func (h *TraceHandler) ListAgentTraces(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "id")
	limit := int32(50)

	runs, err := h.queries.ListTaskRuns(r.Context(), db.ListTaskRunsParams{AgentID: pgtype.UUID{Bytes: uuid.MustParse(agentID), Valid: true}, Limit: limit})
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	json.NewEncoder(w).Encode(runs)
}

func (h *TraceHandler) GetTraceAnalytics(w http.ResponseWriter, r *http.Request) {
	agentID := r.URL.Query().Get("agent_id")
	period := r.URL.Query().Get("period")

	var interval pgtype.Interval
	if period != "" {
		interval.Scan(period)
	}
	analytics, err := h.queries.GetTraceAnalytics(r.Context(), db.GetTraceAnalyticsParams{
		AgentID: pgtype.UUID{Bytes: uuid.MustParse(agentID), Valid: true},
		Column2: interval,
	})
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	json.NewEncoder(w).Encode(analytics)
}

type traceSummary struct {
	TaskID     string            `json:"task_id"`
	RunID      string            `json:"run_id"`
	TotalSteps int               `json:"total_steps"`
	ToolUsage  map[string]int   `json:"tool_usage"`
	KeyActions []string          `json:"key_actions"`
}

func buildSummary(steps []db.TraceStep) *traceSummary {
	summary := &traceSummary{
		ToolUsage:  make(map[string]int),
		KeyActions: []string{},
	}

	for _, step := range steps {
		summary.TotalSteps++
		if step.Tool.Valid {
			summary.ToolUsage[step.Tool.String]++
		}
		if step.Action == "tool_call" && step.Tool.Valid {
			summary.KeyActions = append(summary.KeyActions, step.Tool.String)
		}
	}

	return summary
}