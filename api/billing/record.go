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
//
// ONE PROCESSOR IS COMMERCE ITSELF: "balance". The customer paid in advance — a
// wire recorded as prepaid credit — and the plan is paid from that money. The
// period is drawn from the subject's prepaid money now, through the same draw a
// self-serve purchase makes, and the row is a REGULAR one: the engine collects
// every later period from the same money, and when it runs out the row goes
// past due. The ledger entry and the invoice the draw paid are the reference, so
// the caller need not name one.

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/events"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/idempotencykey"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/types/currency"
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
	// row's provider, and nothing here ever calls it. "balance" is the one
	// exception: the period is paid now from the subject's prepaid money and the
	// engine renews it from there.
	Processor string
	// Reference is the processor's own ids for the payment (invoice, order,
	// payment, customer, receipt), kept on the row for reconciliation. Optional
	// for "balance", whose ledger entry and invoice are recorded instead.
	Reference map[string]string
	// Terms is anything agreed alongside the plan, kept verbatim on the row.
	Terms string
	// Events is the analytics collector. nil records nothing there.
	Events *events.Client
}

// The four things recording a payment can do.
const (
	RecordCreated   = "created"   // the subject had no plan; this opened one
	RecordExtended  = "extended"  // the subject's external plan moved to the next period
	RecordUnchanged = "unchanged" // this period was already recorded; a retry changes nothing
	// RecordUnapplied: the external plan the payment is for was replaced by a
	// checkout purchase before it arrived. The payment is recorded for a refund
	// review and moves no plan.
	RecordUnapplied = "unapplied"
)

// Recorded is what recording a payment did.
type Recorded struct {
	Subscription Subscription
	// Outcome is RecordCreated, RecordExtended or RecordUnchanged.
	Outcome string
}

// collected is the payment a new row opens on: who took it and the period it
// paid. An external one was taken outside commerce, and its later periods
// arrive the same way; otherwise commerce took it, and the engine collects the
// periods after it.
type collected struct {
	processor  string
	start, end time.Time
	external   bool
}

// open shapes a freshly started row onto the paid period: active from its first
// day, no trial, and — when collected externally — never due.
func (c *collected) open(sub *subscription.Subscription) {
	if c.external {
		sub.Type = subscription.External
	}
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
	case len(reference) == 0 && processor != processorBalance:
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
	if processor == processorBalance {
		return recordFromBalance(ctx, org, db, p, in, subject, planID, reference, start, end)
	}

	if held := billingSubscription(db, subject, org.TestMode()); held != nil {
		if old := replacedExternal(db, subject, planID, org.TestMode()); old != nil && held.Type != subscription.External {
			return recordUnapplied(ctx, org, db, old, held, in, processor, reference, start, end)
		}
		return extendRecorded(ctx, org, held, in, planID, p.Interval, reference, start, end)
	}
	// A payment recorded after its external plan lapsed extends that plan.
	if held := lapsedExternal(db, subject, org.TestMode()); held != nil {
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
		Metadata:  recordMetadata(nil, collectionExternal, processor, reference, in.Terms, start, end),
		Test:      org.TestMode(),
		Collected: &collected{processor: processor, start: start, end: end, external: true},
	})
	if err != nil {
		return nil, err
	}
	retireLapsed(ctx, org, db, sub, in.Events)
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
	sub.Metadata = recordMetadata(sub.Metadata, collectionExternal, sub.ProviderType, reference, in.Terms, start, end)
	if err := sub.Update(); err != nil {
		return nil, err
	}
	if ev := in.Events; ev != nil {
		go ev.EmitSubscriptionRenewed(context.WithoutCancel(ctx), subscriptionEvent(org.Name, sub))
	}
	return &Recorded{Subscription: *viewSubscription(sub), Outcome: RecordExtended}, nil
}

// replacedExternal is the subject's externally collected plan on planID that a
// checkout purchase replaced after it lapsed (engine.Replaced), the latest one,
// or nil.
func replacedExternal(db *datastore.Datastore, subject, planID string, test bool) *subscription.Subscription {
	subs, err := userSubscriptions(db, subject, test)
	if err != nil {
		return nil
	}
	var last *subscription.Subscription
	for _, s := range subs {
		reason, _ := s.Metadata["endReason"].(string)
		if s.Type != subscription.External || s.Status != subscription.Canceled || reason != string(engine.Replaced) || planOf(s) != planID {
			continue
		}
		if last == nil || s.PeriodEnd.After(last.PeriodEnd) {
			last = s
		}
	}
	return last
}

// recordUnapplied records a payment for old, an external plan a checkout
// purchase (held) replaced before the payment arrived: the payment is checked
// against old's price as bought, kept once per period as a payment.unapplied
// billing event carrying its facts for a refund review, and moves no plan.
func recordUnapplied(ctx context.Context, org *organization.Organization, db *datastore.Datastore, old, held *subscription.Subscription, in RecordIn, processor string, reference map[string]string, start, end time.Time) (*Recorded, error) {
	seatMult := int64(1)
	if old.Plan.PerSeat && old.Quantity > 1 {
		seatMult = int64(old.Quantity)
	}
	if err := priceMatches(planOf(old), int64(old.Plan.Price)*seatMult, in.PriceCents); err != nil {
		return nil, err
	}
	key := "payment.unapplied:" + old.Id() + ":" + strconv.FormatInt(start.Unix(), 10) + ":" + strconv.FormatInt(end.Unix(), 10)
	data := types.Map{"subscriptionId": old.Id(), "replacedBy": held.Id(), "amountCents": in.PriceCents,
		"currency": string(old.Plan.Currency), "processor": processor, "reference": reference,
		"periodStart": start.Format(time.RFC3339), "periodEnd": end.Format(time.RFC3339)}
	if _, err := engine.EmitBillingEventOnce(db, key, "payment.unapplied", "subscription", old.Id(), old.UserId, data); err != nil {
		return nil, fmt.Errorf("record the payment for replaced subscription %s: %w", old.Id(), err)
	}
	log.Warn("billing: REFUND REVIEW %s paid %d cents through %s for subscription %s, which subscription %s replaced; the payment moves no plan",
		old.UserId, in.PriceCents, processor, old.Id(), held.Id())
	return &Recorded{Subscription: *viewSubscription(old), Outcome: RecordUnapplied}, nil
}

// lapsedExternal is the subject's externally collected plan whose recorded
// period ended past the grace window (engine.Lapsed), or nil: the plan a late
// payment extends.
func lapsedExternal(db *datastore.Datastore, subject string, test bool) *subscription.Subscription {
	subs, err := userSubscriptions(db, subject, test)
	if err != nil {
		return nil
	}
	now := time.Now()
	for _, s := range subs {
		if s.Type == subscription.External && !isBundleRow(s) && engine.Lapsed(s, now) {
			return s
		}
	}
	return nil
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

// How a recorded row's periods are collected: by the processor outside
// commerce, or by the engine from the subject's prepaid money.
const (
	collectionExternal = "external"
	collectionBalance  = processorBalance
)

// recordMetadata carries the collection on the row: how it is collected, who
// collected it, the latest payment's references, the terms, and every period
// recorded so far.
func recordMetadata(prev types.Map, collection, processor string, reference map[string]string, terms string, start, end time.Time) types.Map {
	m := types.Map{}
	for k, v := range prev {
		m[k] = v
	}
	ref := make(map[string]interface{}, len(reference))
	for k, v := range reference {
		ref[k] = v
	}
	m["collection"] = collection
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

// processorBalance is the processor that is commerce itself: the period is paid
// from the subject's prepaid money.
const processorBalance = "balance"

// monthEnd is the most a calendar month's end shortens a period against
// engine.Advance, which carries a day a month lacks into the next one: January 31
// ends on February 28 by the calendar and on March 3 by Advance.
const monthEnd = 3 * 24 * time.Hour

// recordFromBalance opens the plan on a period paid now from the subject's
// prepaid money, as a regular subscription the engine renews from the same
// money.
//
// Everything that can refuse refuses before money moves, and refusing writes
// nothing: the period must be one of the plan's and the one running now, the
// price must be the plan's, the subject must hold no plan — nor one gone past
// due — and its prepaid money must cover the period. The draw is the one a self-serve purchase makes —
// credits, then the balance on the one ledger — all or nothing.
func recordFromBalance(ctx context.Context, org *organization.Organization, db *datastore.Datastore, p *plan.Plan, in RecordIn, subject, planID string, reference map[string]string, start, end time.Time) (*Recorded, error) {
	if one := engine.Advance(start, p); end.After(one) || end.Before(one.Add(-monthEnd)) {
		return nil, saleRefusal{saleRefused, fmt.Sprintf(
			"a period paid from the balance is one %s of the plan: periodEnd must be %s, or up to three days earlier where a month is shorter",
			p.Interval, one.Format(time.RFC3339))}
	}
	if now := time.Now(); start.After(now) || !end.After(now) {
		return nil, saleRefusal{saleRefused, "a period paid from the balance is the one running now: it must have started and not yet ended"}
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

	// One record, once. A retry of a record whose answer was lost replays it; a
	// concurrent one waits. The guard sits before the one-plan check so the retry
	// is answered with its plan rather than told the subject already holds one.
	rec, replay, err := idempotencykey.Begin(db, "billing-record:"+subject, processorBalance+":"+planID+":"+strconv.FormatInt(start.Unix(), 10))
	switch {
	case err != nil:
		return nil, fmt.Errorf("record subscription: cannot tell a retry from a first attempt: %w", err)
	case replay && rec.Status == idempotencykey.StatusCompleted:
		sub, err := loadSubscription(ctx, org, rec.Response)
		if err != nil {
			return nil, err
		}
		return &Recorded{Subscription: *viewSubscription(sub), Outcome: RecordUnchanged}, nil
	case replay:
		return nil, saleRefusal{saleHeld, "this plan is already being recorded"}
	}
	abandon := func() { _ = rec.Delete() }

	if held := billingSubscription(db, subject, org.TestMode()); held != nil {
		abandon()
		heldSlug := held.Plan.Slug
		if heldSlug == "" {
			heldSlug = held.PlanId
		}
		return nil, saleRefusal{saleHeld, fmt.Sprintf(
			"this account already holds the %q plan (subscription %s); change that subscription instead of opening a second one",
			heldSlug, held.Id())}
	}

	cur := p.Currency
	if cur == "" {
		cur = currency.USD
	}
	// The draw refuses a balance that cannot cover the period, having moved
	// nothing. Its ref is this record's, so a retry after the money moved finds
	// the draw it already made instead of being measured against what is left.
	drawn, err := prepaidFor(ctx, org).Draw(ctx, subject, cur, in.PriceCents, "record:"+subject+":"+planID+":"+strconv.FormatInt(start.Unix(), 10))
	if err != nil {
		abandon()
		if errors.Is(err, errShort) {
			return nil, saleRefusal{saleDeclined, err.Error()}
		}
		return nil, fmt.Errorf("record subscription: %w", err)
	}

	sub, inv, err := openPaid(db, p, &createSubscriptionRequest{
		UserId:               subject,
		PlanId:               planID,
		DefaultPaymentMethod: "credits",
		Quantity:             qty,
		Test:                 org.TestMode(),
		Collected:            &collected{processor: "credit", start: start, end: end},
	}, 0, "", drawn)
	if err != nil {
		// The money moved and no subscription holds it. The guard stays started,
		// so a retry replays the draw rather than paying twice.
		uncredited(ctx, in.Events, org.Name, subject, drawn.Ref,
			"a period was drawn from the balance and no subscription was opened: "+err.Error(), in.PriceCents, false)
		return nil, err
	}
	retireLapsed(ctx, org, db, sub, in.Events)

	// The draw and the invoice it paid are the reference.
	paid := make(map[string]string, len(reference)+2)
	for k, v := range reference {
		paid[k] = v
	}
	if drawn.Ref != "" {
		paid["ledger"] = drawn.Ref
	}
	if inv != nil {
		paid["invoice"] = inv.Id()
	}
	sub.Metadata = recordMetadata(sub.Metadata, collectionBalance, processorBalance, paid, in.Terms, start, end)
	if err := sub.Update(); err != nil {
		log.Error("RECONCILE: subscription %s (subject=%s) was paid from the balance and its reference was not saved: %v", sub.Id(), subject, err)
	}
	_ = idempotencykey.Complete(rec, sub.Id())
	emitSale(ctx, in.Events, org.Name, sub, inv)
	return &Recorded{Subscription: *viewSubscription(sub), Outcome: RecordCreated}, nil
}
