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
	"github.com/hanzoai/commerce/models/billingevent"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/payment/processor"
	// Blank-import the provider barrel so every provider's init() registers
	// with processor.Global() before HandleProviderWebhook runs. Without this
	// the global registry is empty and tryValidateWebhook can never reach any
	// provider's ValidateWebhook (the per-org payment.ProcessorsForOrg path is
	// separate and unaffected). This is the single owning import for the
	// generic webhook dispatcher; do not scatter barrel imports elsewhere.
	_ "github.com/hanzoai/commerce/payment/providers"
	jsonhttp "github.com/hanzoai/commerce/util/json/http"
)

// HandleProviderWebhook is the single ingress for payment-provider webhooks.
// It dispatches to the matching processor in payment/router, validates the
// signature, records the event in billing_events, and — for subscription
// lifecycle events — updates the local subscription row keyed by ProviderId.
//
//	POST /v1/billing/webhooks/:provider
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
	//
	// Webhooks arrive with no session, so the auth middleware never sets an
	// organization — the org must be derived from the VALIDATED event itself.
	// We resolve it by matching the event's provider object id against the
	// local subscription row that carries the owning org's namespace.
	org, ok := orgForEvent(c, event)
	if !ok || org == nil {
		// The signature is valid but the event maps to no known org (e.g. a
		// subscription created outside commerce, or a non-lifecycle event with
		// nothing to reconcile). Acknowledge so the provider stops retrying,
		// but never persist into a blank/default namespace.
		log.Info("webhook %s (%s) validated but maps to no org; acknowledging without persist", event.ID, event.Type)
		c.JSON(http.StatusAccepted, gin.H{
			"received": true,
			"skipped":  "no matching organization",
			"type":     event.Type,
			"id":       event.ID,
		})
		return
	}
	db := datastore.New(org.Namespaced(c))

	// Idempotency: key the billing event deterministically by the provider's
	// event id (under the shared synckey parent so ListBillingEvents' ancestor
	// query still finds it). Create is an upsert, so a redelivery — including
	// anything inside the replay window — overwrites the same row instead of
	// appending a duplicate. We probe by key first only to report duplicate=true;
	// the keyed write is what actually guarantees idempotency.
	parent := db.NewKey("synckey", "", 1, nil)
	if event.ID != "" {
		probe := billingevent.New(db)
		if err := probe.GetById(billingEventKey(event.ID)); err == nil {
			c.JSON(http.StatusOK, gin.H{
				"received":  true,
				"duplicate": true,
				"type":      event.Type,
				"id":        event.ID,
			})
			return
		}
	}

	evt := billingevent.New(db)
	if event.ID != "" {
		evt.MustSetKey(db.NewKey("billing-event", billingEventKey(event.ID), 0, parent))
	}
	evt.Type = event.Type
	evt.ObjectType = providerHint
	evt.ObjectId = event.ID
	evt.Livemode = org.Live
	if event.Data != nil {
		evt.Data = event.Data
	}
	if err := evt.Create(); err != nil {
		log.Warn("failed to persist billing event %s: %v", event.ID, err)
		// Do not 500 — event was validated; the keyed upsert is idempotent.
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

// orgForEvent resolves the organization that owns a validated webhook event.
// It is a package var so tests can substitute a deterministic resolver for the
// handler's branch logic; production points at resolveOrgForEvent.
var orgForEvent = resolveOrgForEvent

// resolveOrgForEvent is the real org resolver.
//
// Webhooks are unauthenticated, so the owning org is not in request context;
// it must come from the event. Providers identify subscriptions/customers by
// their own ids, and commerce stores those on the local subscription row
// (Subscription.ProviderId) inside the owning org's namespace. We therefore
// enumerate orgs (which live in the global namespace) and, for each, look for a
// subscription whose ProviderId matches an id carried by the event. The org
// whose namespace holds the match is the owner.
//
// This is the same enumerate-then-rescope pattern the billing cycle uses
// (RunBillingCycleAllOrgs), and it keys on the same ProviderId field that
// applySubscriptionEvent reconciles against — one resolution path, no new
// per-event index. Returns ok=false when no org claims the event.
func resolveOrgForEvent(c *gin.Context, event *processor.WebhookEvent) (*organization.Organization, bool) {
	ids := providerRefs(event)
	if len(ids) == 0 {
		return nil, false
	}

	rootDb := datastore.New(c)
	orgs := make([]*organization.Organization, 0)
	if _, err := organization.Query(rootDb).GetAll(&orgs); err != nil {
		log.Error("webhook: failed to enumerate organizations for event %s: %v", event.ID, err)
		return nil, false
	}

	for _, org := range orgs {
		odb := datastore.New(org.Namespaced(c))
		for _, id := range ids {
			sub := subscription.New(odb)
			if found, err := sub.Query().Filter("ProviderId=", id).Get(); err == nil && found {
				return org, true
			}
		}
	}
	return nil, false
}

// billingEventKey derives the deterministic datastore id for a provider event.
// Keying on the provider's own event id makes persistence idempotent: a
// redelivery upserts the same row. The "evt_" prefix keeps these ids distinct
// from any other billing-event id scheme and is stable across providers because
// provider event ids are globally unique within a provider and the row already
// records ObjectType (the provider).
func billingEventKey(providerEventID string) string {
	return "evt_" + providerEventID
}

// providerRefs returns the provider-side identifiers an event may be keyed on,
// most specific first. The subscription object's own id (data.object.id) is the
// primary key applySubscriptionEvent reconciles against; the customer id is a
// fallback for events whose object is not the subscription itself (e.g. an
// invoice that references its subscription/customer).
func providerRefs(event *processor.WebhookEvent) []string {
	if event == nil || event.Data == nil {
		return nil
	}
	out := make([]string, 0, 3)
	add := func(v interface{}) {
		if s, ok := v.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	add(event.Data["id"])
	add(event.Data["subscription"])
	add(event.Data["customer"])
	return out
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
		"X-Square-Hmacsha256-Signature", // Square (HMAC-SHA256 over notificationURL+body)
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
		case "square":
			if v := h.Get("X-Square-Hmacsha256-Signature"); v != "" {
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
