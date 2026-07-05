package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/agentra-ai/agentra/server/internal/middleware"
	"github.com/agentra-ai/agentra/server/internal/util"
	db "github.com/agentra-ai/agentra/server/pkg/db/generated"
)

type BillingHandler struct {
	Queries *db.Queries
}

func NewBillingHandler(q *db.Queries) *BillingHandler {
	return &BillingHandler{Queries: q}
}

func (h *BillingHandler) RegisterRoutes(r chi.Router) {
	r.Use(middleware.RequireWorkspaceRoleFromURL(h.Queries, "workspaceId", "owner", "admin"))
	r.Get("/subscription", h.GetSubscription)
	r.Post("/checkout", h.CreateCheckoutSession)
	r.Post("/portal", h.CreatePortalSession)
	r.Get("/invoices", h.ListInvoices)
	r.Get("/usage", h.GetUsage)
	r.Post("/webhook", h.StripeWebhook) // public in practice; overlay in router
}

func (h *BillingHandler) workspaceID(w http.ResponseWriter, r *http.Request) (pgtype.UUID, bool) {
	// Read workspace id from chi path or query
	// Walk up to find workspace from outer route (simplified: query param)
	ws := chi.URLParam(r, "workspaceId")
	if ws == "" {
		ws = r.URL.Query().Get("workspace_id")
	}
	uid := util.ParseUUID(ws)
	if !uid.Valid {
		writeError(w, http.StatusBadRequest, "workspace_id required")
		return pgtype.UUID{}, false
	}
	return uid, true
}

func (h *BillingHandler) GetSubscription(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "TODO")
}

func (h *BillingHandler) CreateCheckoutSession(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "TODO")
}

func (h *BillingHandler) CreatePortalSession(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "TODO")
}

func (h *BillingHandler) ListInvoices(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "TODO")
}

func (h *BillingHandler) GetUsage(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "TODO")
}

func (h *BillingHandler) StripeWebhook(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "TODO")
}
