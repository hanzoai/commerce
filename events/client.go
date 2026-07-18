// Package events provides a thin HTTP client for the analytics collector.
//
// Commerce fires events via HTTP to the analytics-collector sidecar
// rather than writing directly to ClickHouse. This decouples analytics
// from the commerce binary.
package events

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Client sends events to the analytics-collector via HTTP.
type Client struct {
	endpoint   string
	httpClient *http.Client
}

// NewClient creates a new analytics client.
// Endpoint should be the analytics-collector base URL (e.g., "http://analytics-collector.hanzo.svc:8091").
func NewClient(endpoint string) *Client {
	return &Client{
		endpoint: endpoint,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// Order represents a Commerce order for event emission.
type Order struct {
	ID       string
	UserID   string
	Email    string
	Total    float64
	Currency string
	Items    []OrderItem
	Status   string
	OrgID    string
}

// OrderItem represents an item in an order.
type OrderItem struct {
	ProductID   string
	ProductName string
	SKU         string
	Quantity    int
	Price       float64
}

// EmitOrderCompleted sends an order completed event to the collector.
func (c *Client) EmitOrderCompleted(ctx context.Context, order *Order) error {
	items := make([]map[string]interface{}, len(order.Items))
	var totalQuantity int
	for i, item := range order.Items {
		items[i] = map[string]interface{}{
			"product_id":   item.ProductID,
			"product_name": item.ProductName,
			"sku":          item.SKU,
			"quantity":     item.Quantity,
			"price":        item.Price,
		}
		totalQuantity += item.Quantity
	}

	itemsJSON, _ := json.Marshal(items)

	return c.EmitRaw(ctx, map[string]interface{}{
		"event":           "order_completed",
		"distinct_id":     order.UserID,
		"organization_id": order.OrgID,
		"order_id":        order.ID,
		"revenue":         order.Total,
		"quantity":        totalQuantity,
		"properties": map[string]interface{}{
			"currency":   order.Currency,
			"items":      string(itemsJSON),
			"item_count": len(order.Items),
			"status":     order.Status,
			"email":      order.Email,
		},
	})
}

// EmitReferralLinkCreated sends a referral link creation event to the collector.
func (c *Client) EmitReferralLinkCreated(ctx context.Context, orgID, userID, referralCode, referralURL string) error {
	return c.EmitRaw(ctx, map[string]interface{}{
		"event":           "referral_link_created",
		"distinct_id":     userID,
		"organization_id": orgID,
		"properties": map[string]interface{}{
			"referral_code": referralCode,
			"referral_url":  referralURL,
		},
	})
}

// EmitReferralClaimed sends a referral claimed event to the collector.
func (c *Client) EmitReferralClaimed(ctx context.Context, orgID, referrerID, refereeID, referralCode string) error {
	return c.EmitRaw(ctx, map[string]interface{}{
		"event":           "referral_claimed",
		"distinct_id":     refereeID,
		"organization_id": orgID,
		"properties": map[string]interface{}{
			"referrer_id":   referrerID,
			"referee_id":    refereeID,
			"referral_code": referralCode,
		},
	})
}

// EmitReferralCreditGranted sends a referral credit granted event to the collector.
func (c *Client) EmitReferralCreditGranted(ctx context.Context, orgID, userID, role string, amount float64, currency string) error {
	return c.EmitRaw(ctx, map[string]interface{}{
		"event":           "referral_credit_granted",
		"distinct_id":     userID,
		"organization_id": orgID,
		"revenue":         amount,
		"properties": map[string]interface{}{
			"role":     role,
			"amount":   amount,
			"currency": currency,
		},
	})
}

// EmitReferralCommissionEarned sends a referral commission event to the collector.
func (c *Client) EmitReferralCommissionEarned(ctx context.Context, orgID, referrerID, orderID string, commission float64, currency string) error {
	return c.EmitRaw(ctx, map[string]interface{}{
		"event":           "referral_commission_earned",
		"distinct_id":     referrerID,
		"organization_id": orgID,
		"order_id":        orderID,
		"revenue":         commission,
		"properties": map[string]interface{}{
			"order_id":   orderID,
			"commission": commission,
			"currency":   currency,
		},
	})
}

// EmitReferralTierUpgraded sends a referral tier upgrade event to the collector.
func (c *Client) EmitReferralTierUpgraded(ctx context.Context, orgID, userID, previousTier, newTier string, referralCount int) error {
	return c.EmitRaw(ctx, map[string]interface{}{
		"event":           "referral_tier_upgraded",
		"distinct_id":     userID,
		"organization_id": orgID,
		"properties": map[string]interface{}{
			"previous_tier":  previousTier,
			"new_tier":       newTier,
			"referral_count": referralCount,
		},
	})
}

// EmitContributorRegistered sends a contributor registration event to the collector.
func (c *Client) EmitContributorRegistered(ctx context.Context, orgID, userID, githubUsername string) error {
	return c.EmitRaw(ctx, map[string]interface{}{
		"event":           "contributor_registered",
		"distinct_id":     userID,
		"organization_id": orgID,
		"properties": map[string]interface{}{
			"github_username": githubUsername,
		},
	})
}

// EmitContributorPayoutCalculated sends a payout calculation event to the collector.
func (c *Client) EmitContributorPayoutCalculated(ctx context.Context, orgID, userID, periodMonth string, amount float64, currency string) error {
	return c.EmitRaw(ctx, map[string]interface{}{
		"event":           "contributor_payout_calculated",
		"distinct_id":     userID,
		"organization_id": orgID,
		"revenue":         amount,
		"properties": map[string]interface{}{
			"period_month": periodMonth,
			"amount":       amount,
			"currency":     currency,
		},
	})
}

// EmitContributorPayoutSent sends a payout sent event to the collector.
func (c *Client) EmitContributorPayoutSent(ctx context.Context, orgID, userID, payoutID, periodMonth string, amount float64, currency string) error {
	return c.EmitRaw(ctx, map[string]interface{}{
		"event":           "contributor_payout_sent",
		"distinct_id":     userID,
		"organization_id": orgID,
		"revenue":         amount,
		"properties": map[string]interface{}{
			"payout_id":    payoutID,
			"period_month": periodMonth,
			"amount":       amount,
			"currency":     currency,
		},
	})
}

// ── billing / subscription / usage customer-activity spine ───────────────────
//
// These mirror EmitOrderCompleted EXACTLY: each posts the same
// {event, distinct_id, organization_id, revenue, properties} envelope the
// collector lands in commerce.events. The money that MOVES in the event is the
// top-level revenue (USD, mirroring order.Total); the exact integer cents plus
// the structured billing fields ride in properties so the fleet read side
// (admin.hanzo.ai) can aggregate them precisely with ClickHouse JSON functions.
// All best-effort: EmitRaw no-ops when no collector is configured, and callers
// fire them fire-and-forget so analytics can never block the money path.

// Subscription is a subscription-lifecycle event for the collector. Money is USD
// cents (exact). MRRCents is the monthly-normalized recurring revenue so annual
// and monthly plans are comparable in one fleet sum.
type Subscription struct {
	ID          string
	OrgID       string
	UserID      string
	Plan        string // plan slug / id — the byPlan / byCategory key
	PlanName    string // human plan name
	Category    string // plan category — the byCategory bucket
	Status      string // active | trialing | canceled | past_due | ...
	Interval    string // month | year | ...
	PriceCents  int64  // raw plan price (USD cents)
	MRRCents    int64  // monthly-normalized recurring revenue (USD cents)
	Seats       int
	Trial       bool
	PeriodStart string // RFC3339
	PeriodEnd   string // RFC3339
}

// EmitSubscriptionCreated sends a subscription_created event to the collector.
func (c *Client) EmitSubscriptionCreated(ctx context.Context, s *Subscription) error {
	return c.emitSubscription(ctx, "subscription_created", s)
}

// EmitSubscriptionRenewed sends a subscription_renewed event to the collector.
func (c *Client) EmitSubscriptionRenewed(ctx context.Context, s *Subscription) error {
	return c.emitSubscription(ctx, "subscription_renewed", s)
}

// EmitSubscriptionPlanChanged sends a subscription_plan_changed event to the collector.
func (c *Client) EmitSubscriptionPlanChanged(ctx context.Context, s *Subscription) error {
	return c.emitSubscription(ctx, "subscription_plan_changed", s)
}

// EmitSubscriptionCanceled sends a subscription_canceled event to the collector.
func (c *Client) EmitSubscriptionCanceled(ctx context.Context, s *Subscription) error {
	return c.emitSubscription(ctx, "subscription_canceled", s)
}

// emitSubscription builds the one subscription envelope shared by every
// lifecycle event. revenue is 0 — a subscription state change moves no cash at
// this instant (the charge is realized on its invoice); the MRR rides in
// properties as exact cents.
func (c *Client) emitSubscription(ctx context.Context, event string, s *Subscription) error {
	return c.EmitRaw(ctx, map[string]interface{}{
		"event":           event,
		"distinct_id":     s.UserID,
		"organization_id": s.OrgID,
		"revenue":         0,
		"properties": map[string]interface{}{
			"subscription_id": s.ID,
			"plan":            s.Plan,
			"plan_name":       s.PlanName,
			"category":        s.Category,
			"status":          s.Status,
			"interval":        s.Interval,
			"price_cents":     s.PriceCents,
			"mrr_cents":       s.MRRCents,
			"seats":           s.Seats,
			"trial":           s.Trial,
			"period_start":    s.PeriodStart,
			"period_end":      s.PeriodEnd,
		},
	})
}

// Invoice is an invoice-lifecycle event for the collector. AmountCents is the
// amount due, AmountPaidCents what was actually collected; money is USD cents.
type Invoice struct {
	ID              string
	Number          string
	OrgID           string
	UserID          string
	Status          string
	AmountCents     int64
	AmountPaidCents int64
	Currency        string
	SubscriptionID  string
	Issued          string // RFC3339
	Due             string // RFC3339
}

// EmitInvoiceFinalized sends an invoice_finalized event to the collector.
func (c *Client) EmitInvoiceFinalized(ctx context.Context, in *Invoice) error {
	return c.emitInvoice(ctx, "invoice_finalized", 0, in)
}

// EmitInvoicePaid sends an invoice_paid event to the collector. revenue is the
// amount actually paid (USD) — realized cash, mirroring order.Total.
func (c *Client) EmitInvoicePaid(ctx context.Context, in *Invoice) error {
	return c.emitInvoice(ctx, "invoice_paid", float64(in.AmountPaidCents)/100.0, in)
}

// EmitInvoiceVoid sends an invoice_void event to the collector.
func (c *Client) EmitInvoiceVoid(ctx context.Context, in *Invoice) error {
	return c.emitInvoice(ctx, "invoice_void", 0, in)
}

// emitInvoice builds the one invoice envelope shared by every lifecycle event.
func (c *Client) emitInvoice(ctx context.Context, event string, revenue float64, in *Invoice) error {
	return c.EmitRaw(ctx, map[string]interface{}{
		"event":           event,
		"distinct_id":     in.UserID,
		"organization_id": in.OrgID,
		"revenue":         revenue,
		"properties": map[string]interface{}{
			"invoice_id":        in.ID,
			"number":            in.Number,
			"status":            in.Status,
			"amount_cents":      in.AmountCents,
			"amount_paid_cents": in.AmountPaidCents,
			"currency":          in.Currency,
			"subscription_id":   in.SubscriptionID,
			"issued":            in.Issued,
			"due":               in.Due,
		},
	})
}

// APIUsage is a metered API-usage debit event for the collector. AmountCents is
// the debited spend (USD cents); AmountMicros carries the exact sub-cent debit.
type APIUsage struct {
	OrgID        string
	UserID       string
	AmountCents  int64
	AmountMicros int64
	Model        string
	Provider     string
	Project      string
	Service      string
	RequestID    string
	TotalTokens  int
	Status       string
}

// EmitAPIUsageDebit sends an api_usage_debit event to the collector. revenue is
// the debited spend (USD) — realized consumption, mirroring order.Total.
func (c *Client) EmitAPIUsageDebit(ctx context.Context, u *APIUsage) error {
	return c.EmitRaw(ctx, map[string]interface{}{
		"event":           "api_usage_debit",
		"distinct_id":     u.UserID,
		"organization_id": u.OrgID,
		"revenue":         float64(u.AmountCents) / 100.0,
		"properties": map[string]interface{}{
			"amount_cents":  u.AmountCents,
			"amount_micros": u.AmountMicros,
			"model":         u.Model,
			"provider":      u.Provider,
			"project":       u.Project,
			"service":       u.Service,
			"request_id":    u.RequestID,
			"total_tokens":  u.TotalTokens,
			"status":        u.Status,
		},
	})
}

// EmitRaw sends a raw event to the collector.
func (c *Client) EmitRaw(ctx context.Context, event map[string]interface{}) error {
	if c.endpoint == "" {
		return nil // No collector configured, silently skip
	}

	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+"/event", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send event: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("collector error: status %d", resp.StatusCode)
	}
	return nil
}

// Flush is a no-op for the HTTP client (collector handles batching).
func (c *Client) Flush() error { return nil }

// Close is a no-op for the HTTP client.
func (c *Client) Close() error { return nil }
