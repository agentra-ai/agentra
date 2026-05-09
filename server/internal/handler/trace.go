package handler

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/agentra-ai/agentra/server/pkg/traces"
	"github.com/go-chi/chi/v5"
)

// GetTraceByTask returns the execution trace for a task.
// GET /api/traces/:taskId
func (h *Handler) GetTraceByTask(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")

	trace, err := h.TraceService.GetTraceByTask(r.Context(), taskID)
	if err != nil {
		slog.Warn("get trace by task: not found", "task_id", taskID, "error", err)
		writeJSON(w, http.StatusOK, map[string]any{"trace": nil})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"trace": trace})
}

// ListTracesByIssue returns all execution traces for an issue.
// GET /api/issues/:id/traces
func (h *Handler) ListTracesByIssue(w http.ResponseWriter, r *http.Request) {
	issueID := chi.URLParam(r, "id")

	result, err := h.TraceService.ListTraces(r.Context(), issueID)
	if err != nil {
		slog.Warn("list traces by issue: failed", "issue_id", issueID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list traces")
		return
	}

	if result == nil {
		result = []*traces.ExecutionTrace{}
	}

	writeJSON(w, http.StatusOK, map[string]any{"traces": result})
}

// GetTrace returns a single execution trace by ID.
// GET /api/traces/detail/:traceId
func (h *Handler) GetTrace(w http.ResponseWriter, r *http.Request) {
	traceID := chi.URLParam(r, "traceId")

	trace, err := h.TraceService.GetTrace(r.Context(), traceID)
	if err != nil {
		slog.Warn("get trace: not found", "trace_id", traceID, "error", err)
		writeError(w, http.StatusNotFound, "trace not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"trace": trace})
}

// recordTraceMessage records a task message as a trace step or tool call.
// This is called from ReportTaskMessages on a best-effort basis.
func (h *Handler) recordTraceMessage(ctx context.Context, traceID string, msg TaskMessageRequest) {
	if traceID == "" || h.TraceService == nil || h.TraceService.TraceService == nil {
		return
	}

	switch msg.Type {
	case "text", "thinking", "status", "error":
		step := traces.TraceStep{
			Step:    msg.Seq,
			Type:    mapMessageType(msg.Type),
			Content: msg.Content,
		}
		if msg.Type == "error" {
			errStr := msg.Content
			step.Error = &errStr
		}
		_ = h.TraceService.RecordStep(ctx, traceID, step)

	case "tool-use":
		tc := traces.ToolCall{
			Tool:  msg.Tool,
			Input: msg.Input,
		}
		_ = h.TraceService.RecordToolCall(ctx, traceID, tc)

	case "tool-result":
		// Tool results are recorded as tool output updates.
		// For simplicity, we record a new tool call entry with just the output.
		tc := traces.ToolCall{
			Tool:   msg.Tool,
			Output: msg.Output,
		}
		_ = h.TraceService.RecordToolCall(ctx, traceID, tc)
	}
}

// mapMessageType maps internal message types to trace step types.
func mapMessageType(msgType string) string {
	switch msgType {
	case "text", "thinking":
		return "assistant"
	case "tool-use", "tool-result":
		return "tool"
	case "error":
		return "system"
	case "status":
		return "system"
	default:
		return "assistant"
	}
}
