package billing

import (
	"context"
	"strings"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/events"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/subscription"
)

// Analytics emit — the customer-activity spine. Each billing money-path handler
// fires ONE best-effort event to the analytics collector (which lands it in
// commerce.events) so the fleet read side (admin.hanzo.ai) can aggregate
// billing/subscription/usage across every tenant with NO per-org fan-out. This
// file holds the ONE fire-and-forget wiring + the model→event builders so the
// handlers stay one line and DRY. Money is USD cents end to end.

// fireEvent runs fn with the request-scoped analytics client on a DETACHED
// context (fire-and-forget) when a collector is wired — best-effort, so it can
// never block or fail the money path. No-op when no events client is present
// (the local is injected root-wide by installEventsLocal). Mirrors the
// order_completed emit in api/checkout/authorize.go.
func fireEvent(c *zip.Ctx, fn func(context.Context, *events.Client)) {
	client := c.Locals("events")
	if client == nil {
		return
	}
	ev, ok := client.(*events.Client)
	if !ok || ev == nil {
		return
	}
	// WithoutCancel (not Background): survive client disconnect but keep trace values.
	ctx := context.WithoutCancel(c.Context())
	go fn(ctx, ev)
}

// monthlyNormalizedCents normalizes a plan price to a monthly figure by its
// billing interval so annual and monthly plans are comparable in one MRR sum.
// The read side never re-normalizes — the cents emitted here ARE the MRR.
func monthlyNormalizedCents(price int64, interval string) int64 {
	switch strings.ToLower(strings.TrimSpace(interval)) {
	case "year", "yearly", "annual", "annually":
		return price / 12
	case "week", "weekly":
		return price * 52 / 12
	case "day", "daily":
		return price * 365 / 12
	default: // month/monthly and anything unrecognized → treat as monthly
		return price
	}
}

// planCategoryForSlug resolves a plan's mix category from the static catalog
// (the same source bundledPlansForSlug reads). Empty when unknown — honest, the
// read side buckets it under "".
func planCategoryForSlug(slug string) string {
	for i := range hanzoPlans {
		if hanzoPlans[i].Slug == slug {
			return hanzoPlans[i].Category
		}
	}
	return ""
}

// rfc3339 formats a time as an RFC3339 UTC string; zero → "" (honest empty).
func rfc3339(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// subscriptionEvent maps a subscription (+ its owning org) onto the collector
// event, resolving the plan key, category, and monthly-normalized MRR.
func subscriptionEvent(orgName string, sub *subscription.Subscription) *events.Subscription {
	plan := strings.TrimSpace(sub.Plan.Slug)
	if plan == "" {
		plan = strings.TrimSpace(sub.PlanId)
	}
	interval := string(sub.Plan.Interval)
	price := int64(sub.Plan.Price)
	status := string(sub.Status)
	trial := strings.EqualFold(strings.TrimSpace(status), "trialing")
	return &events.Subscription{
		ID:          sub.Id(),
		OrgID:       orgName,
		UserID:      sub.UserId,
		Plan:        plan,
		PlanName:    sub.Plan.Name,
		Category:    planCategoryForSlug(plan),
		Status:      status,
		Interval:    interval,
		PriceCents:  price,
		MRRCents:    monthlyNormalizedCents(price, interval),
		Seats:       sub.Quantity,
		Trial:       trial,
		PeriodStart: rfc3339(sub.PeriodStart),
		PeriodEnd:   rfc3339(sub.PeriodEnd),
	}
}

// invoiceEvent maps an invoice (+ its owning org) onto the collector event.
func invoiceEvent(orgName string, inv *billinginvoice.BillingInvoice) *events.Invoice {
	return &events.Invoice{
		ID:              inv.Id(),
		Number:          inv.NumberStr,
		OrgID:           orgName,
		UserID:          inv.UserId,
		Status:          string(inv.Status),
		AmountCents:     int64(inv.AmountDue),
		AmountPaidCents: int64(inv.AmountPaid),
		Currency:        string(inv.Currency),
		SubscriptionID:  inv.SubscriptionId,
		Issued:          rfc3339(inv.CreatedAt),
		Due:             rfc3339(inv.DueDate),
	}
}

// ── per-handler emit wrappers (same-package, so the call sites import nothing) ──

func emitSubscriptionCreated(c *zip.Ctx, orgName string, sub *subscription.Subscription) {
	fireEvent(c, func(ctx context.Context, ev *events.Client) {
		ev.EmitSubscriptionCreated(ctx, subscriptionEvent(orgName, sub))
	})
}

func emitSubscriptionRenewed(c *zip.Ctx, orgName string, sub *subscription.Subscription) {
	fireEvent(c, func(ctx context.Context, ev *events.Client) {
		ev.EmitSubscriptionRenewed(ctx, subscriptionEvent(orgName, sub))
	})
}

func emitSubscriptionPlanChanged(c *zip.Ctx, orgName string, sub *subscription.Subscription) {
	fireEvent(c, func(ctx context.Context, ev *events.Client) {
		ev.EmitSubscriptionPlanChanged(ctx, subscriptionEvent(orgName, sub))
	})
}

func emitSubscriptionCanceled(c *zip.Ctx, orgName string, sub *subscription.Subscription) {
	fireEvent(c, func(ctx context.Context, ev *events.Client) {
		ev.EmitSubscriptionCanceled(ctx, subscriptionEvent(orgName, sub))
	})
}

func emitInvoiceFinalized(c *zip.Ctx, orgName string, inv *billinginvoice.BillingInvoice) {
	fireEvent(c, func(ctx context.Context, ev *events.Client) {
		ev.EmitInvoiceFinalized(ctx, invoiceEvent(orgName, inv))
	})
}

func emitInvoicePaid(c *zip.Ctx, orgName string, inv *billinginvoice.BillingInvoice) {
	fireEvent(c, func(ctx context.Context, ev *events.Client) {
		ev.EmitInvoicePaid(ctx, invoiceEvent(orgName, inv))
	})
}

func emitInvoiceVoid(c *zip.Ctx, orgName string, inv *billinginvoice.BillingInvoice) {
	fireEvent(c, func(ctx context.Context, ev *events.Client) {
		ev.EmitInvoiceVoid(ctx, invoiceEvent(orgName, inv))
	})
}

// emitAPIUsageDebit fires the metered-usage debit. It reads the same-package
// usageRequest directly (amountCents/effMicros are the post-rounding debit).
func emitAPIUsageDebit(c *zip.Ctx, orgName string, req *usageRequest, amountCents, amountMicros int64) {
	fireEvent(c, func(ctx context.Context, ev *events.Client) {
		ev.EmitAPIUsageDebit(ctx, &events.APIUsage{
			OrgID:        orgName,
			UserID:       req.User,
			AmountCents:  amountCents,
			AmountMicros: amountMicros,
			Model:        req.Model,
			Provider:     req.Provider,
			Project:      req.Project,
			Service:      req.Service,
			RequestID:    req.RequestID,
			TotalTokens:  req.TotalTokens,
			Status:       req.Status,
		})
	})
}
