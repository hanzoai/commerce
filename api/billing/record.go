package billing

// record.go — a plan the customer paid for OUTSIDE checkout.
//
// Some customers pay an invoice raised by hand in the processor's own dashboard,
// or wire the money. That payment is real and it bought a plan, but no card
// passed through Subscribe, so nothing here opened a subscription and the
// account reads free. RecordSubscription opens it from the payment's facts.
//
// It charges nothing and never will. The row is subscription.External: the
// engine never answers due for it (engine.IsDue), so no cycle invoices it and no
// collection burns the customer's credits, balance or card for it. It grants no
// credit either: the payment bought the plan, and whatever the plan includes is
// the plan's to confer, not this act's to mint. A later period arrives the same
// way the first did, by recording the payment that covers it.
//
// The PRICE is still the catalog's. The caller states what the customer paid and
// the act checks it against what the plan costs, so recording a payment against
// the wrong plan is refused rather than booked. Nothing on the wire sets a price.
//
// It is the staff path, so it may open a PRIVATE plan — one made for a single
// customer, which no self-serve path offers (plan.Assignable).

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/events"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/subscription"
	types "github.com/hanzoai/commerce/types"
)

// RecordIn is one payment collected outside checkout, and the plan it bought.
type RecordIn struct {
	// Subject is the billing subject the plan is for, already bounded to the
	// org by the caller. This act never resolves an identity.
	Subject string
	// PlanID is the plan slug. The plan authority or the embedded catalog must
	// know it and still sell it, publicly or to this customer alone.
	PlanID string
	// Interval is the period the payment covers: "" or "month", or "year".
	Interval types.Interval
	// Quantity is the seat count of a per-seat plan. 0 reads as 1.
	Quantity int
	// PriceCents is what the customer paid for one period, all seats. It is a
	// CHECK and never a price: it must equal the plan's price for that period.
	PriceCents int64
	// PeriodStart and PeriodEnd bound the period the payment covers.
	PeriodStart time.Time
	PeriodEnd   time.Time
	// Processor is who collected the money: "square", "wire", … It becomes the
	// row's provider, and nothing here ever calls it.
	Processor string
	// Reference is the processor's own ids for the payment (invoice, order,
	// payment, customer, receipt), kept on the row for reconciliation.
	Reference map[string]string
	// Terms is anything agreed alongside the plan, kept verbatim on the row.
	Terms string
	// Events is the analytics collector. nil records nothing there.
	Events *events.Client
}

// The three things recording a payment can do.
const (
	RecordCreated   = "created"   // the subject had no plan; this opened one
	RecordExtended  = "extended"  // the subject's external plan moved to the next period
	RecordUnchanged = "unchanged" // this period was already recorded; a retry changes nothing
)

// Recorded is what recording a payment did.
type Recorded struct {
	Subscription Subscription
	// Outcome is RecordCreated, RecordExtended or RecordUnchanged.
	Outcome string
}

// collected is the external collection a new row opens on.
type collected struct {
	processor  string
	start, end time.Time
}

// open shapes a freshly started row into an externally collected one: active
// from the first paid day to the last, no trial, and never due.
func (c *collected) open(sub *subscription.Subscription) {
	sub.Type = subscription.External
	sub.ProviderType = c.processor
	sub.Status = subscription.Active
	sub.TrialStart, sub.TrialEnd = time.Time{}, time.Time{}
	sub.Start, sub.PeriodStart, sub.PeriodEnd = c.start, c.start, c.end
}

// RecordSubscription opens the plan a payment collected outside checkout
// bought, or moves the subject's externally collected plan on to the period a
// new payment covers.
//
// ONE PAID SUBSCRIPTION PER SUBJECT holds here as it does at checkout. A subject
// that already pays through checkout is refused; one whose plan is already
// external on the same plan and period gets its row back unchanged, so a retry
// is harmless; a later period on the same plan extends that row. Anything else
// is refused with the reason.
func RecordSubscription(ctx context.Context, org *organization.Organization, in RecordIn) (*Recorded, error) {
	if org == nil {
		return nil, errors.New("record subscription: no organization")
	}
	subject := strings.TrimSpace(in.Subject)
	planID := strings.TrimSpace(in.PlanID)
	processor := strings.ToLower(strings.TrimSpace(in.Processor))
	reference := cleanReference(in.Reference)
	start, end := in.PeriodStart.UTC(), in.PeriodEnd.UTC()
	switch {
	case subject == "":
		return nil, saleRefusal{saleRefused, "subject is required"}
	case planID == "":
		return nil, saleRefusal{saleRefused, "planId is required"}
	case processor == "":
		return nil, saleRefusal{saleRefused, "processor is required: name who collected the payment"}
	case len(reference) == 0:
		return nil, saleRefusal{saleRefused, "reference is required: the processor's own ids for the payment"}
	case start.IsZero() || !end.After(start):
		return nil, saleRefusal{saleRefused, "periodEnd must be after periodStart"}
	case in.PriceCents <= 0:
		return nil, saleRefusal{saleRefused, "priceCents is required: what the customer paid for the period"}
	}

	db := datastore.New(org.Namespaced(ctx))
	p, err := resolveSubscriptionPlan(db, planID)
	if err != nil || !p.Assignable() {
		return nil, saleRefusal{saleMissing, fmt.Sprintf("plan %q not found", planID)}
	}
	if p, err = planAtInterval(p, in.Interval); err != nil {
		return nil, saleRefusal{saleRefused, err.Error()}
	}

	if held := billingSubscription(db, subject, org.TestMode()); held != nil {
		return extendRecorded(ctx, org, held, in, planID, p.Interval, reference, start, end)
	}

	qty := in.Quantity
	if qty < 1 {
		qty = 1
	}
	seatMult := int64(1)
	if perSeat(planID) {
		seatMult = int64(qty)
	}
	if err := priceMatches(planID, int64(p.Price)*seatMult, in.PriceCents); err != nil {
		return nil, err
	}

	p.TrialPeriodDays = 0
	sub, err := createSubscription(db, p, &createSubscriptionRequest{
		UserId:    subject,
		PlanId:    planID,
		Quantity:  qty,
		Metadata:  recordMetadata(nil, processor, reference, in.Terms, start, end),
		Test:      org.TestMode(),
		Collected: &collected{processor: processor, start: start, end: end},
	})
	if err != nil {
		return nil, err
	}
	emitSale(ctx, in.Events, org.Name, sub, nil)
	return &Recorded{Subscription: *viewSubscription(sub), Outcome: RecordCreated}, nil
}

// extendRecorded answers a payment for a subject that already holds a paid plan.
func extendRecorded(ctx context.Context, org *organization.Organization, held *subscription.Subscription, in RecordIn, planID string, interval types.Interval, reference map[string]string, start, end time.Time) (*Recorded, error) {
	heldSlug := held.Plan.Slug
	if heldSlug == "" {
		heldSlug = held.PlanId
	}
	if held.Type != subscription.External {
		return nil, saleRefusal{saleHeld, fmt.Sprintf(
			"this account already pays for the %q plan through checkout (subscription %s); change that subscription instead",
			heldSlug, held.Id())}
	}
	if heldSlug != planID || held.Plan.Interval != interval {
		return nil, saleRefusal{saleHeld, fmt.Sprintf(
			"this account already holds the %q plan by the %s (subscription %s); a payment for another plan cannot extend it",
			heldSlug, held.Plan.Interval, held.Id())}
	}
	if in.Quantity > 0 && in.Quantity != held.Quantity {
		return nil, saleRefusal{saleRefused, fmt.Sprintf(
			"subscription %s holds %d seat(s); a payment for %d cannot extend it", held.Id(), held.Quantity, in.Quantity)}
	}
	seatMult := int64(1)
	if held.Plan.PerSeat && held.Quantity > 1 {
		seatMult = int64(held.Quantity)
	}
	// The row keeps the price it was opened at, so a later payment is checked
	// against that price and not against whatever the catalog sells today.
	if err := priceMatches(heldSlug, int64(held.Plan.Price)*seatMult, in.PriceCents); err != nil {
		return nil, err
	}
	if start.Equal(held.PeriodStart.UTC()) && end.Equal(held.PeriodEnd.UTC()) {
		return &Recorded{Subscription: *viewSubscription(held), Outcome: RecordUnchanged}, nil
	}
	if !end.After(held.PeriodEnd) {
		return nil, saleRefusal{saleRefused, fmt.Sprintf(
			"subscription %s is already recorded through %s", held.Id(), held.PeriodEnd.UTC().Format(time.RFC3339))}
	}
	// A row read by query is not bound to the store, so the write goes through a
	// fresh read of the same id.
	sub, err := loadSubscription(ctx, org, held.Id())
	if err != nil {
		return nil, err
	}
	sub.PeriodStart, sub.PeriodEnd = start, end
	sub.Metadata = recordMetadata(sub.Metadata, sub.ProviderType, reference, in.Terms, start, end)
	if err := sub.Update(); err != nil {
		return nil, err
	}
	if ev := in.Events; ev != nil {
		go ev.EmitSubscriptionRenewed(context.WithoutCancel(ctx), subscriptionEvent(org.Name, sub))
	}
	return &Recorded{Subscription: *viewSubscription(sub), Outcome: RecordExtended}, nil
}

// priceMatches refuses a payment that is not what the plan costs for the period.
func priceMatches(planID string, want, paid int64) error {
	if want <= 0 {
		return saleRefusal{saleRefused, fmt.Sprintf("plan %q publishes no price for this period, so no payment can be recorded against it", planID)}
	}
	if paid != want {
		return saleRefusal{saleRefused, fmt.Sprintf("plan %q costs %d cents for this period; the payment recorded was %d", planID, want, paid)}
	}
	return nil
}

// recordMetadata carries the collection on the row: who collected it, the
// latest payment's references, the terms, and every period recorded so far.
func recordMetadata(prev types.Map, processor string, reference map[string]string, terms string, start, end time.Time) types.Map {
	m := types.Map{}
	for k, v := range prev {
		m[k] = v
	}
	ref := make(map[string]interface{}, len(reference))
	for k, v := range reference {
		ref[k] = v
	}
	m["collection"] = "external"
	m["processor"] = processor
	m["reference"] = ref
	if t := strings.TrimSpace(terms); t != "" {
		m["terms"] = t
	}
	periods, _ := m["periods"].([]interface{})
	m["periods"] = append(periods, map[string]interface{}{
		"start":     start.Format(time.RFC3339),
		"end":       end.Format(time.RFC3339),
		"reference": ref,
	})
	return m
}

// cleanReference drops blank keys and values, so an empty reference is refused
// rather than recorded as one.
func cleanReference(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		if k != "" && v != "" {
			out[k] = v
		}
	}
	return out
}
