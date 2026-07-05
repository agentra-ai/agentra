-- Billing queries for Stripe subscriptions, invoices, and usage records (Issue #22).

-- name: CreateSubscription :one
INSERT INTO subscriptions (workspace_id, stripe_subscription_id, plan, current_period_start, current_period_end, seats)
VALUES ($1, $2, $3, $4, $5, $6) RETURNING *;

-- name: GetSubscriptionByWorkspace :one
SELECT * FROM subscriptions WHERE workspace_id = $1 AND status = 'active' LIMIT 1;

-- name: GetSubscriptionByStripeID :one
SELECT * FROM subscriptions WHERE stripe_subscription_id = $1 LIMIT 1;

-- name: UpdateSubscription :one
UPDATE subscriptions
SET stripe_subscription_id = $2, plan = $3, status = $4, current_period_end = $5, seats = $6, updated_at = now()
WHERE workspace_id = $1 RETURNING *;

-- name: UpdateSubscriptionStatus :one
UPDATE subscriptions SET status = $2, updated_at = now()
WHERE stripe_subscription_id = $1 RETURNING *;

-- name: CreateInvoice :one
INSERT INTO invoices (workspace_id, stripe_invoice_id, amount_cents, status, period_start, period_end, hosted_invoice_url)
VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING *;

-- name: GetInvoiceByStripeID :one
SELECT * FROM invoices WHERE stripe_invoice_id = $1 LIMIT 1;

-- name: UpdateInvoiceStatus :one
UPDATE invoices SET status = $2 WHERE stripe_invoice_id = $1 RETURNING *;

-- name: GetLatestInvoice :one
SELECT * FROM invoices WHERE workspace_id = $1 ORDER BY created_at DESC LIMIT 1;

-- name: ListInvoices :many
SELECT * FROM invoices WHERE workspace_id = $1 ORDER BY created_at DESC LIMIT 50;

-- name: CreateUsageRecord :one
INSERT INTO usage_records (workspace_id, metric, quantity) VALUES ($1, $2, $3) RETURNING *;

-- name: GetUsageForPeriod :many
SELECT * FROM usage_records
WHERE workspace_id = $1 AND recorded_at >= $2 AND recorded_at < $3
ORDER BY recorded_at DESC;

-- name: SetStripeCustomerBySubscription :exec
UPDATE subscriptions SET stripe_customer_id = $1, updated_at = now()
WHERE stripe_subscription_id = $2;
