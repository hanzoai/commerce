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

// MonthlyNormalizedCents normalizes a plan price to a monthly figure by its
// billing PERIOD so plans on different cycles are comparable in one MRR sum.
// The read side never re-normalizes — the cents emitted here ARE the MRR.
//
// This is the ONE definition of that normalization. api/metrics and the
// hanzoai/cloud admin each used to carry a copy, each with a comment promising
// it mirrored this one; the copies are gone and both now read the number this
// package produces.
//
// The period is interval × intervalCount, and BOTH are required, because the
// price is charged in full once per period. billing/engine advancePeriod moves
// a subscription on by AddDate(0, IntervalCount, 0) and the invoice carries
// Plan.Price × seats for that whole span — so month/3 at $50 is $50 every three
// months, worth $16.67 of monthly recurring revenue. This function used to take
// only the interval, which made the multiplier structurally unreachable and
// reported such a plan at $50/mo: a 3× overstatement, and ARR = MRR × 12
// carried it straight to the revenue board. A count of 0 or less reads as 1,
// exactly as the engine reads it, so a legacy row with the field unset behaves
// as the plain every-interval plan it is.
//
// It is deliberately about a price and a period only. What a whole subscription
// contributes is SubscriptionMRRCents.
func MonthlyNormalizedCents(price int64, interval string, intervalCount int) int64 {
	if intervalCount < 1 {
		intervalCount = 1
	}
	// periods per year, so one rounded division does every interval at once
	var perYear int64
	switch strings.ToLower(strings.TrimSpace(interval)) {
	case "year", "yearly", "annual", "annually":
		perYear = 1
	case "week", "weekly":
		perYear = 52
	case "day", "daily":
		perYear = 365
	default: // month/monthly and anything unrecognized → treat as monthly
		perYear = 12
	}
	return roundDiv(price*perYear, 12*int64(intervalCount))
}

// roundDiv divides num by a positive den, rounding half away from zero.
//
// Money rounds to the nearer cent; it does not truncate. Truncating loses the
// fraction on every normalized price in the same direction, so a book of
// annual and quarterly plans reports low forever and never self-corrects.
func roundDiv(num, den int64) int64 {
	if num < 0 {
		return -((-num + den/2) / den)
	}
	return (num + den/2) / den
}

// SubscriptionMRRCents is what ONE subscription contributes to MRR: its
// interval-normalized price times the seats it bills for.
//
// Seats are part of the figure, not a footnote beside it. A 10-seat plan at
// $20/seat/month is $200 of monthly recurring revenue, and a caller that
// reports the $20 has not reported this subscription's MRR — it has reported
// the price. Every surface that says "MRR" now means this.
//
// A quantity below 1 is read as 1: a subscription that exists bills for at
// least one seat, and legacy rows written before the field existed carry 0.
//
// It reports the subscription's recurring CONTRIBUTION and deliberately does
// not look at status, because two different questions hang off that. Whether
// the contribution counts toward the run-rate right now is Status.CountsTowardMRR,
// which every summation must apply. What a canceled subscription was worth is
// this number — churn is the revenue we lost, so it needs the amount even
// though the status no longer counts.
func SubscriptionMRRCents(sub *subscription.Subscription) int64 {
	qty := sub.Quantity
	if qty < 1 {
		qty = 1
	}
	monthly := MonthlyNormalizedCents(int64(sub.Plan.Price), string(sub.Plan.Interval), sub.Plan.IntervalCount)
	return monthly * int64(qty)
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
		MRRCents:    SubscriptionMRRCents(sub),
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
