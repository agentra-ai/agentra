package handler

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/agentra-ai/agentra/server/internal/middleware"
	"github.com/agentra-ai/agentra/server/internal/util"
	"github.com/agentra-ai/agentra/server/pkg/stripe"
	stripego "github.com/stripe/stripe-go/v76"
	db "github.com/agentra-ai/agentra/server/pkg/db/generated"
)

type BillingHandler struct {
	Queries *db.Queries
	Stripe  *stripe.Client
}

func NewBillingHandler(q *db.Queries, sc *stripe.Client) *BillingHandler {
	return &BillingHandler{Queries: q, Stripe: sc}
}

func (h *BillingHandler) RegisterRoutes(r chi.Router) {
	r.Use(middleware.RequireWorkspaceRoleFromURL(h.Queries, "id", "owner", "admin"))
	r.Get("/subscription", h.GetSubscription)
	r.Get("/checkout", h.CreateCheckoutSession)
	r.Get("/portal", h.CreatePortalSession)
	r.Get("/invoices", h.ListInvoices)
	r.Get("/usage", h.GetUsage)
}

func (h *BillingHandler) workspaceID(w http.ResponseWriter, r *http.Request) (pgtype.UUID, bool) {
	id := chi.URLParam(r, "id")
	uid := util.ParseUUID(id)
	if !uid.Valid {
		writeError(w, http.StatusBadRequest, "invalid workspace id")
		return pgtype.UUID{}, false
	}
	return uid, true
}

func (h *BillingHandler) GetSubscription(w http.ResponseWriter, r *http.Request) {
	wsID, ok := h.workspaceID(w, r)
	if !ok {
		return
	}
	if h.Stripe.Skip() {
		writeJSON(w, http.StatusOK, map[string]any{"plan": "free", "status": "inactive"})
		return
	}
	sub, err := h.Queries.GetSubscriptionByWorkspace(r.Context(), wsID)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"plan": "free", "status": "inactive"})
		return
	}
	writeJSON(w, http.StatusOK, sub)
}

func (h *BillingHandler) CreateCheckoutSession(w http.ResponseWriter, r *http.Request) {
	wsID, ok := h.workspaceID(w, r)
	if !ok {
		return
	}
	if h.Stripe.Skip() {
		writeError(w, http.StatusServiceUnavailable, "billing not configured")
		return
	}
	email := r.Header.Get("X-User-Email")
	if email == "" {
		writeError(w, http.StatusBadRequest, "X-User-Email header required")
		return
	}

	// Seed subscription row
	if _, err := h.Queries.GetSubscriptionByWorkspace(r.Context(), wsID); err != nil {
		_, _ = h.Queries.CreateSubscription(r.Context(), db.CreateSubscriptionParams{
			WorkspaceID: wsID, Plan: "free", Seats: 0,
		})
	}

	url, err := h.Stripe.CreateCheckoutSession(r.Context(), email, util.UUIDToString(wsID))
	if err != nil {
		slog.Error("CreateCheckoutSession: stripe error", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create checkout session")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"url": url})
}

func (h *BillingHandler) CreatePortalSession(w http.ResponseWriter, r *http.Request) {
	wsID, ok := h.workspaceID(w, r)
	if !ok {
		return
	}
	if h.Stripe.Skip() {
		writeError(w, http.StatusServiceUnavailable, "billing not configured")
		return
	}
	sub, err := h.Queries.GetSubscriptionByWorkspace(r.Context(), wsID)
	if err != nil || !sub.StripeCustomerID.Valid || sub.StripeCustomerID.String == "" {
		writeError(w, http.StatusNotFound, "no active subscription")
		return
	}
	url, err := h.Stripe.CreatePortalSession(r.Context(), sub.StripeCustomerID.String)
	if err != nil {
		slog.Error("CreatePortalSession: stripe error", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create portal session")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"url": url})
}

func (h *BillingHandler) ListInvoices(w http.ResponseWriter, r *http.Request) {
	wsID, ok := h.workspaceID(w, r)
	if !ok {
		return
	}
	rows, err := h.Queries.ListInvoices(r.Context(), wsID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if rows == nil {
		rows = []db.Invoice{}
	}
	writeJSON(w, http.StatusOK, rows)
}

func (h *BillingHandler) GetUsage(w http.ResponseWriter, r *http.Request) {
	wsID, ok := h.workspaceID(w, r)
	if !ok {
		return
	}
	plan, err := h.Queries.GetWorkspacePlan(r.Context(), wsID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	used, _ := h.Queries.CountActiveMembers(r.Context(), wsID)
	writeJSON(w, http.StatusOK, map[string]any{
		"seats_used": used,
		"seats_max":  plan.MaxSeats,
		"plan":       plan.Plan,
	})
}

// StripeWebhook handles POST /api/webhooks/stripe (PUBLIC — no JWT).
func (h *BillingHandler) StripeWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "only POST allowed")
		return
	}
	const maxWebhookBytes = 1 << 20
	body, err := io.ReadAll(io.LimitReader(r.Body, maxWebhookBytes))
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read body")
		return
	}
	defer r.Body.Close()

	sig := r.Header.Get("Stripe-Signature")
	if sig == "" {
		writeError(w, http.StatusBadRequest, "missing Stripe-Signature")
		return
	}
	if h.Stripe.Skip() {
		writeError(w, http.StatusServiceUnavailable, "billing not configured")
		return
	}

	event, err := h.Stripe.ConstructEvent(body, sig)
	if err != nil {
		slog.Warn("StripeWebhook: bad signature", "error", err)
		writeError(w, http.StatusBadRequest, "invalid signature")
		return
	}
	slog.Info("StripeWebhook received", "type", event.Type)

	switch event.Type {
	case "checkout.session.completed":
		h.handleCheckoutCompleted(r.Context(), event)
	case "customer.subscription.updated":
		h.handleSubscriptionUpdated(r.Context(), event)
	case "invoice.paid":
		h.handleInvoicePaid(r.Context(), event)
	case "customer.subscription.deleted":
		h.handleSubscriptionDeleted(r.Context(), event)
	default:
		slog.Debug("StripeWebhook: unhandled type", "type", event.Type)
	}
	writeJSON(w, http.StatusOK, map[string]any{"received": true})
}

func (h *BillingHandler) handleCheckoutCompleted(ctx context.Context, event stripego.Event) {
	var sess stripego.CheckoutSession
	raw, err := json.Marshal(event.Data.Raw)
	if err != nil {
		slog.Error("handleCheckoutCompleted: marshal data.raw", "error", err)
		return
	}
	if err := json.Unmarshal(raw, &sess); err != nil {
		slog.Error("handleCheckoutCompleted: unmarshal", "error", err)
		return
	}
	wsMeta := ""
	if v, ok := sess.Metadata["workspace_id"]; ok {
		wsMeta = v
	}
	wsID := util.ParseUUID(wsMeta)
	if !wsID.Valid {
		slog.Warn("handleCheckoutCompleted: no workspace_id")
		return
	}
	var subscriptionID string
	if sess.Subscription != nil {
		subscriptionID = sess.Subscription.ID
	}
	params := db.UpdateSubscriptionParams{
		WorkspaceID:          wsID,
		StripeSubscriptionID: ptr(subscriptionID),
		Plan:                 "pro",
		Status:               "active",
		CurrentPeriodEnd:     pgtype.Timestamptz{Time: time.Now().AddDate(0, 1, 0), Valid: true},
		Seats:                0,
	}
	if _, err := h.Queries.UpdateSubscription(ctx, params); err != nil {
		slog.Error("handleCheckoutCompleted: UpdateSubscription", "error", err)
	}
}

func (h *BillingHandler) handleSubscriptionUpdated(ctx context.Context, event stripego.Event) {
	var sub stripego.Subscription
	raw, err := json.Marshal(event.Data.Raw)
	if err != nil {
		return
	}
	if err := json.Unmarshal(raw, &sub); err != nil {
		return
	}
	status := ""
	switch sub.Status {
	case stripego.SubscriptionStatusActive:
		status = "active"
	case stripego.SubscriptionStatusPastDue:
		status = "past_due"
	case stripego.SubscriptionStatusCanceled:
		status = "canceled"
	case stripego.SubscriptionStatusUnpaid:
		status = "unpaid"
	default:
		status = string(sub.Status)
	}
	_, _ = h.Queries.UpdateSubscriptionStatus(ctx, db.UpdateSubscriptionStatusParams{
		StripeSubscriptionID: ptr(sub.ID),
		Status:               status,
	})
}

func (h *BillingHandler) handleInvoicePaid(ctx context.Context, event stripego.Event) {
	var inv stripego.Invoice
	raw, err := json.Marshal(event.Data.Raw)
	if err != nil {
		return
	}
	if err := json.Unmarshal(raw, &inv); err != nil {
		return
	}
	var wsID pgtype.UUID
	if inv.Subscription != nil {
		if sub, qerr := h.Queries.GetSubscriptionByStripeID(ctx, ptr(inv.Subscription.ID)); qerr == nil {
			wsID = sub.WorkspaceID
		}
	}
	if !wsID.Valid && inv.Metadata != nil {
		if v, ok := inv.Metadata["workspace_id"]; ok && v != "" {
			wsID = util.ParseUUID(v)
		}
	}
	if !wsID.Valid {
		slog.Warn("handleInvoicePaid: cannot determine workspace", "invoice_id", inv.ID)
		return
	}
	var pstart, pend time.Time
	if inv.PeriodStart > 0 {
		pstart = time.Unix(inv.PeriodStart, 0)
	}
	if inv.PeriodEnd > 0 {
		pend = time.Unix(inv.PeriodEnd, 0)
	}
	_, _ = h.Queries.CreateInvoice(ctx, db.CreateInvoiceParams{
		WorkspaceID:      wsID,
		StripeInvoiceID:  ptr(inv.ID),
		AmountCents:      int32(max(0, int(inv.AmountPaid))),
		Status:           "paid",
		PeriodStart:      pgtype.Timestamptz{Time: pstart, Valid: !pstart.IsZero()},
		PeriodEnd:        pgtype.Timestamptz{Time: pend, Valid: !pend.IsZero()},
		HostedInvoiceUrl: ptr(inv.HostedInvoiceURL),
	})
}

func (h *BillingHandler) handleSubscriptionDeleted(ctx context.Context, event stripego.Event) {
	var sub stripego.Subscription
	raw, err := json.Marshal(event.Data.Raw)
	if err != nil {
		return
	}
	if err := json.Unmarshal(raw, &sub); err != nil {
		return
	}
	_, _ = h.Queries.UpdateSubscriptionStatus(ctx, db.UpdateSubscriptionStatusParams{
		StripeSubscriptionID: ptr(sub.ID),
		Status:               "canceled",
	})
}

// ptr returns a pgtype.Text referencing the given string.
func ptr(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: s != ""}
}

// max returns the larger of a and b.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
