package billing

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/billingevent"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/payment/processor"
	jsonhttp "github.com/hanzoai/commerce/util/json/http"
)

// HandleProviderWebhook is the single ingress for payment-provider webhooks.
// It dispatches to the matching processor in payment/router, validates the
// signature, records the event in billing_events, and — for subscription
// lifecycle events — updates the local subscription row keyed by ProviderId.
//
//	POST /api/v1/billing/webhooks/:provider
//
// The :provider path segment is informational; signature verification picks
// the right processor regardless. We pass the path segment as a lightweight
// filter so webhook endpoints are URL-scoped per-provider (easier in Stripe
// dashboard configuration).
func HandleProviderWebhook(c *gin.Context) {
	providerHint := strings.ToLower(strings.TrimSpace(c.Param("provider")))
	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		jsonhttp.Fail(c, http.StatusBadRequest, "cannot read request body", err)
		return
	}

	// Every provider puts its signature in a different header; let the router
	// try each processor with the one most likely to match.
	signature := pickSignatureHeader(c.Request.Header, providerHint)
	if signature == "" {
		jsonhttp.Fail(c, http.StatusBadRequest, "missing webhook signature header", nil)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	event, err := tryValidateWebhook(ctx, providerHint, payload, signature)
	if err != nil || event == nil {
		log.Warn("webhook signature validation failed (provider hint=%s): %v", providerHint, err)
		jsonhttp.Fail(c, http.StatusUnauthorized, "invalid webhook signature", err)
		return
	}

	// Persist the raw event so the app has an audit trail independent of
	// processor-side retention.
	org := middleware.GetOrganization(c)
	if org == nil {
		// Webhooks arrive with no session; route them to the platform org so
		// they at least get persisted. Downstream handlers may rescope.
		jsonhttp.Fail(c, http.StatusServiceUnavailable, "organization context unavailable", nil)
		return
	}
	db := datastore.New(org.Namespaced(c))

	evt := billingevent.New(db)
	evt.Type = event.Type
	evt.ObjectType = providerHint
	evt.ObjectId = event.ID
	evt.Livemode = org.Live
	if event.Data != nil {
		evt.Data = event.Data
	}
	if err := evt.Create(); err != nil {
		log.Warn("failed to persist billing event %s: %v", event.ID, err)
		// Do not 500 — event was validated; duplicate persistence is fine.
	}

	// Update local subscription state for lifecycle events.
	if strings.HasPrefix(event.Type, "subscription.") || strings.HasPrefix(event.Type, "invoice.") {
		applySubscriptionEvent(db, event)
	}

	c.JSON(http.StatusOK, gin.H{
		"received": true,
		"type":     event.Type,
		"id":       event.ID,
	})
}

// applySubscriptionEvent reconciles the local subscription row with a
// lifecycle event from the payment provider.
func applySubscriptionEvent(db *datastore.Datastore, event *processor.WebhookEvent) {
	// Payload contains the Stripe subscription object — look for "id".
	id, _ := event.Data["id"].(string)
	if id == "" {
		return
	}

	sub := subscription.New(db)
	found, err := sub.Query().Filter("ProviderId=", id).Get()
	if err != nil || !found {
		// Unknown subscription — likely created outside commerce.
		return
	}

	if status, ok := event.Data["status"].(string); ok && status != "" {
		sub.Status = subscription.Status(status)
	}
	if event.Type == "subscription.canceled" {
		sub.Canceled = true
		sub.CanceledAt = time.Now().UTC()
	}
	if err := sub.Update(); err != nil {
		log.Warn("webhook: failed to update subscription %s: %v", sub.Id(), err)
	}
}

// tryValidateWebhook walks registered payment processors looking for one that
// validates the signature. If providerHint is non-empty, we try that processor
// first (fast path); otherwise we iterate all available processors.
func tryValidateWebhook(ctx context.Context, providerHint string, payload []byte, signature string) (*processor.WebhookEvent, error) {
	registry := processor.Global()

	// Fast path: provider hint specified.
	if providerHint != "" {
		if p, err := registry.Get(processor.ProcessorType(providerHint)); err == nil {
			return p.ValidateWebhook(ctx, payload, signature)
		}
	}

	// Fallback: try every available processor until one succeeds.
	var lastErr error
	for _, p := range registry.Available(ctx) {
		evt, err := p.ValidateWebhook(ctx, payload, signature)
		if err == nil && evt != nil {
			return evt, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

// pickSignatureHeader returns the signature header for the given provider.
// We check the most common header names regardless of hint so a misconfigured
// Stripe endpoint (e.g. /webhooks/paypal) still validates correctly.
func pickSignatureHeader(h http.Header, providerHint string) string {
	candidates := []string{
		"Stripe-Signature",
		"Paypal-Transmission-Sig",
		"X-Adyen-Signature",
		"X-Paypal-Auth-Algo",
		"X-CC-Webhook-Signature", // Coinbase Commerce
		"X-Signature",
	}
	if providerHint != "" {
		// Try a provider-specific guess first.
		switch providerHint {
		case "stripe":
			if v := h.Get("Stripe-Signature"); v != "" {
				return v
			}
		case "paypal":
			if v := h.Get("Paypal-Transmission-Sig"); v != "" {
				return v
			}
		case "coinbase":
			if v := h.Get("X-CC-Webhook-Signature"); v != "" {
				return v
			}
		}
	}
	for _, name := range candidates {
		if v := h.Get(name); v != "" {
			return v
		}
	}
	return ""
}
