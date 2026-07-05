package stripe

import (
	"context"
	"fmt"

	"github.com/stripe/stripe-go/v76"
	portalsess "github.com/stripe/stripe-go/v76/billingportal/session"
	checkoutsession "github.com/stripe/stripe-go/v76/checkout/session"
	"github.com/stripe/stripe-go/v76/webhook"
)

// Price environment variable names. The raw price IDs are read from the
// environment at startup and passed into NewClient.
const (
	PriceBaseSeat     = "PRICE_BASE_SEAT"
	PriceAgentRuntime = "PRICE_AGENT_RUNTIME"
)

// Production endpoints. Kept as constants so tests can build a Client without
// consulting the network.
const (
	CheckoutSuccessBase = "https://agentra.orb.local/settings/billing?session_id={CHECKOUT_SESSION_ID}"
	CheckoutCancelBase  = "https://agentra.orb.local/settings/billing"
	PortalReturnURL     = "https://agentra.orb.local/settings/billing"
)

// Client holds a Stripe API key and the price IDs for the current deployment.
type Client struct {
	secretKey     string
	webhookSecret string
	priceBaseSeat string
	priceAddon    string
}

// NewClient constructs a Client and sets stripe.Key. The zero value for a
// price ID means "not configured" — the matching call will fail noisily if
// invoked, which is the desired behaviour in dev.
func NewClient(secretKey, webhookSecret, priceBase, priceAddon string) *Client {
	stripe.Key = secretKey
	return &Client{
		secretKey:     secretKey,
		webhookSecret: webhookSecret,
		priceBaseSeat: priceBase,
		priceAddon:    priceAddon,
	}
}

// WebhookSecret returns the configured signing secret.
func (c *Client) WebhookSecret() string { return c.webhookSecret }

// CreateCheckoutSession provisions a Stripe Checkout Session and returns the
// URL the browser should navigate to. The workspace ID is stashed in metadata
// so the webhook handler can reconcile the completed session back to a
// workspace.
func (c *Client) CreateCheckoutSession(ctx context.Context, customerEmail, workspaceID string) (string, error) {
	params := &stripe.CheckoutSessionParams{
		CustomerEmail: stripe.String(customerEmail),
		Mode:          stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		SuccessURL:    stripe.String(CheckoutSuccessBase),
		CancelURL:     stripe.String(CheckoutCancelBase),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(c.priceBaseSeat),
				Quantity: stripe.Int64(1),
			},
		},
		Metadata: map[string]string{
			"workspace_id": workspaceID,
		},
	}
	sess, err := checkoutsession.New(params)
	if err != nil {
		return "", fmt.Errorf("stripe checkout session: %w", err)
	}
	return sess.URL, nil
}

// CreatePortalSession provisions a Stripe Billing Portal Session and returns
// the URL the browser should navigate to.
func (c *Client) CreatePortalSession(ctx context.Context, stripeCustomerID string) (string, error) {
	params := &stripe.BillingPortalSessionParams{
		Customer:  stripe.String(stripeCustomerID),
		ReturnURL: stripe.String(PortalReturnURL),
	}
	sess, err := portalsess.New(params)
	if err != nil {
		return "", fmt.Errorf("stripe portal session: %w", err)
	}
	return sess.URL, nil
}

// ConstructEvent verifies the Stripe-Signature header against the webhook
// secret and unmarshals the envelope into a typed stripe.Event.
func (c *Client) ConstructEvent(payload []byte, sigStr string) (stripe.Event, error) {
	return webhook.ConstructEvent(payload, sigStr, c.webhookSecret)
}

// Skip returns true when billing is unconfigured.
func (c *Client) Skip() bool {
	return c.secretKey == ""
}
