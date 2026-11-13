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
// collection charges the customer's card for it. It grants no credit either: the
// payment bought the plan, and whatever the plan includes is the plan's to
// confer, not this act's to mint. A later period arrives the same way the first
// did, by recording the payment that covers it; with none, the plan lapses at its
// end (engine.Over). The customer is reminded before then (RemindDue), and a card
// plan bought meanwhile starts at that end (scheduleSale).
//
// The PRICE is still the catalog's. The caller states what the customer paid and
// the act checks it against what the plan costs, so recording a payment against
// the wrong plan is refused rather than booked. Nothing on the wire sets a price.
//
// It is the staff path, so it may open a PRIVATE plan — one made for a single
// customer, which no self-serve path offers (plan.Assignable).
//
// NO PROCESSOR IS PREPAID MONEY. A plan is never paid from the balance or from
// credit: those pay for usage. "balance", "credit", "credits" and "prepaid" are
// refused, and a plan is either recorded as collected by whoever collected it,
// given as a comp, or bought by card.
//
// ONE PROCESSOR IS NOBODY: "comp". A plan an owner gives away — our own orgs,
// a partner — is recorded as what it is, a plan nobody paid for: priceCents 0,
// whatever the plan costs (a plan with no price, such as enterprise, included),
// the approval that gave it in reference.order and why in terms. Its row is
// external like any record, so it never renews and nothing ever charges it, and
// its plan is held at price 0, so it adds nothing to MRR. A comp is extended only
// by a comp; a paid plan takes one over by naming it in Replaces, and a comp
// takes over a paid plan the same way.
//
// A record may TAKE OVER the plan the subject holds, when the caller names it in
// Replaces. The new plan is recorded first; the held plan ends only after, so a
// refusal at any step leaves it as it was.

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/events"
	"github.com/hanzoai/commerce/log"
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
	// Lines are a team's other seat types the payment bought beside PlanID's seats
	// (LineIn): a team of one standard and one premium seat is PlanID "team",
	// Quantity 1 and one line of "team-premium". PriceCents covers every seat.
	Lines []LineIn
	// PriceCents is what the customer paid for one period, all seats. It is a
	// CHECK and never a price: it must equal the plan's price for that period.
	PriceCents int64
	// PeriodStart and PeriodEnd bound the period the payment covers.
	PeriodStart time.Time
	PeriodEnd   time.Time
	// Processor is who collected the money: "square", "wire", … It becomes the
	// row's provider, and nothing here ever calls it. "comp" is a plan nobody paid
	// for: PriceCents 0, the approval in Reference "order", and Terms saying why.
	// Prepaid money is no processor (prepaidSource): a plan is never paid from it.
	Processor string
	// Reference is the processor's own ids for the payment (invoice, order,
	// payment, customer, receipt), kept on the row for reconciliation.
	Reference map[string]string
	// Terms is anything agreed alongside the plan, kept verbatim on the row.
	Terms string
	// Replaces is the id of the plan the subject holds that this one takes over.
	// The held plan ends at once, and only after the new one is recorded. Empty, a
	// subject holding another plan is refused.
	Replaces string
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
	// Replaced is the plan this record took over, as it now stands; nil when it
	// took over none.
	Replaced *Subscription
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
// is harmless; a later period on the same plan extends that row. A different
// plan takes over the held one only when Replaces names it. Anything else is
// refused with the reason.
func RecordSubscription(ctx context.Context, org *organization.Organization, in RecordIn) (*Recorded, error) {
	if org == nil {
		return nil, errors.New("record subscription: no organization")
	}
	subject := strings.TrimSpace(in.Subject)
	planID := strings.TrimSpace(in.PlanID)
	in.Replaces = strings.TrimSpace(in.Replaces)
	processor := strings.ToLower(strings.TrimSpace(in.Processor))
	reference := cleanReference(in.Reference)
	start, end := in.PeriodStart.UTC(), in.PeriodEnd.UTC()
	comp := processor == processorComp
	switch {
	case subject == "":
		return nil, saleRefusal{saleRefused, "subject is required"}
	case planID == "":
		return nil, saleRefusal{saleRefused, "planId is required"}
	case processor == "":
		return nil, saleRefusal{saleRefused, "processor is required: name who collected the payment"}
	case prepaidSource(processor):
		return nil, coded(saleRefused, "plan_needs_card", fmt.Sprintf(
			"processor %q is refused: a plan is never paid from the prepaid balance or credit, which pay for usage only; record who collected the payment (%s), give a comp, or have the customer subscribe by card",
			processor, strings.Join(RecordProcessors[:len(RecordProcessors)-1], ", ")))
	case !slices.Contains(RecordProcessors, processor):
		return nil, coded(saleRefused, "processor_unknown", fmt.Sprintf(
			"processor %q is not one a payment is recorded from; name one of %s", processor, strings.Join(RecordProcessors, ", ")))
	case comp && (reference["order"] == "" || strings.TrimSpace(in.Terms) == ""):
		return nil, saleRefusal{saleRefused, "a comp names the approval that gave it in reference.order and why in terms"}
	case comp && in.PriceCents != 0:
		return nil, saleRefusal{saleRefused, "a comp is a plan nobody paid for: priceCents is 0"}
	case len(reference) == 0:
		return nil, saleRefusal{saleRefused, "reference is required: the processor's own ids for the payment"}
	case start.IsZero() || !end.After(start):
		return nil, saleRefusal{saleRefused, "periodEnd must be after periodStart"}
	case !comp && in.PriceCents <= 0:
		return nil, saleRefusal{saleRefused, "priceCents is required: what the customer paid for the period"}
	}

	db := datastore.New(org.Namespaced(ctx))
	// One sale, completion or record for a subject at a time (holdSale): a record
	// reads the plan the subject holds and opens or moves one, as a sale does, so
	// it never runs beside a sale of the same subject.
	lease, err := holdSale(db, subject)
	if err != nil {
		return nil, err
	}
	defer lease.Release()
	p, err := resolveSubscriptionPlan(db, planID)
	if err != nil {
		return nil, saleRefusal{saleMissing, fmt.Sprintf("plan %q not found", planID)}
	}
	if p, err = planAtInterval(p, in.Interval); err != nil {
		return nil, saleRefusal{saleRefused, err.Error()}
	}
	// A card plan waiting to start at the held plan's end is the subject's next
	// period already; recording another over it would pay for the same time twice.
	if waiting := scheduledFor(db, subject, org.TestMode()); waiting != nil {
		return nil, saleRefusal{saleHeld, fmt.Sprintf(
			"this account's %q plan starts by card on %s (subscription %s); cancel it before recording another plan",
			subscriptionSlug(waiting), waiting.PeriodStart.UTC().Format(time.DateOnly), waiting.Id())}
	}

	held := billingSubscription(db, subject, org.TestMode())
	sameKind := held != nil && (held.ProviderType == processorComp) == comp
	if held != nil && (in.Replaces == "" || (sameRung(held, planID, p.Interval) && sameKind)) {
		// The next period of a plan the subject already holds: a renewal, which
		// keeps the period the subscription was bought over. It extends only the
		// plan held, priced from the row's own snapshot, so a plan retired while
		// the subject pays for it by wire still takes its next payment.
		return extendRecorded(ctx, org, held, in, planID, p.Interval, reference, start, end)
	}
	// Opening a plan, or moving onto one, is for a plan staff may put a customer
	// on: listed or private, never draft or archived.
	if !p.Assignable() {
		return nil, saleRefusal{saleMissing, fmt.Sprintf("plan %q not found", planID)}
	}
	if err := monthlyOnly(p, in.Interval); err != nil {
		return nil, saleRefusal{saleRefused, err.Error()}
	}
	if err := takesOver(held, in.Replaces); err != nil {
		return nil, err
	}

	// Every rule that can refuse the plan is asked before anything moves.
	lines, qty, seatMult, err := recordSeats(db, p, in)
	if err != nil {
		return nil, err
	}
	// A price staff agreed with this customer is what a payment for the plan's own
	// seats is checked against, and the row carries it.
	customer, err := agreedPrice(db, subject, planID, p)
	if err != nil {
		return nil, saleRefusal{saleUnavailable, "the price agreed for this account could not be read; nothing was recorded"}
	}
	list := int64(p.Price)
	if comp {
		// Held at price 0: what the row's plan costs is what MRR and every
		// reader of the row sum, and nobody pays for a comp. The price it listed
		// at is kept beside it, which is what makes it a paid plan given away.
		p.Price = 0
		for i := range lines {
			lines[i].Price = 0
		}
	} else if err := priceMatches(planID, seatPrice(list, customer)*seatMult+lineCents(lines), in.PriceCents); err != nil {
		return nil, err
	}

	meta := recordMetadata(nil, collectionExternal, processor, reference, in.Terms, start, end, seatMult+int64(lineSeats(lines)), in.PriceCents, lines)
	if comp {
		meta["listCents"] = list
	}
	sub, err := createSubscription(db, p, &createSubscriptionRequest{
		UserId:        subject,
		PlanId:        planID,
		Quantity:      qty,
		Metadata:      meta,
		Test:          org.TestMode(),
		Collected:     &collected{processor: processor, start: start, end: end, external: true},
		priced:        lines,
		customerCents: customer,
	})
	if err != nil {
		return nil, err
	}
	var replaced *Subscription
	if held != nil {
		old, err := retire(ctx, org, held.Id(), in.Events)
		if err != nil {
			// Nothing moved, so the new row is withdrawn and the held plan stands.
			withdraw(db, sub)
			return nil, fmt.Errorf("record subscription: subscription %s could not be ended, so the new plan was not recorded: %w", held.Id(), err)
		}
		replaced = viewSubscription(old)
	}
	emitSale(ctx, in.Events, org.Name, sub, nil)
	return &Recorded{Subscription: *viewSubscription(sub), Outcome: RecordCreated, Replaced: replaced}, nil
}

// sameRung reports whether held is already the plan, at the interval, a record
// names. A record of the plan the subject holds extends it; nothing is replaced.
func sameRung(held *subscription.Subscription, planID string, interval types.Interval) bool {
	return heldSlug(held) == planID && held.Plan.Interval == interval
}

// heldSlug is the slug of the plan a row holds, its plan id where it names none.
func heldSlug(held *subscription.Subscription) string {
	if held.Plan.Slug != "" {
		return held.Plan.Slug
	}
	return held.PlanId
}

// takesOver refuses a Replaces that does not name the plan the subject holds:
// a caller must name the row it is ending, and a stale name ends nothing.
func takesOver(held *subscription.Subscription, replaces string) error {
	switch {
	case replaces == "":
		return nil
	case held == nil:
		return saleRefusal{saleRefused, fmt.Sprintf("subscription %s is not a plan this account holds, so there is nothing to replace", replaces)}
	case held.Id() != replaces:
		return saleRefusal{saleHeld, fmt.Sprintf(
			"this account holds the %q plan as subscription %s, not %s; name the plan it holds", heldSlug(held), held.Id(), replaces)}
	}
	return nil
}

// retire ends the plan a record took over, at once, through the one cancel. The
// row ends before its open invoices are voided; a void that fails after the row
// ended is logged for reconciliation, since nothing renews an ended row.
func retire(ctx context.Context, org *organization.Organization, id string, ev *events.Client) (*subscription.Subscription, error) {
	sub, err := cancelSubscription(ctx, org, AnyHolder, id, false)
	if err != nil {
		row, lerr := loadSubscription(ctx, org, id)
		if lerr != nil || row.Status != subscription.Canceled {
			return nil, err
		}
		log.Error("RECONCILE: subscription %s had ended when a record replaced it, and ending it answered: %v", id, err)
		sub = row
	}
	if ev != nil {
		go ev.EmitSubscriptionCanceled(context.WithoutCancel(ctx), subscriptionEvent(org.Name, sub))
	}
	return sub, nil
}

// withdraw removes a row this record opened, and the bundle rows opened with it,
// when the plan it was to replace could not be ended. No money moved for it.
func withdraw(db *datastore.Datastore, sub *subscription.Subscription) {
	subs, _ := userSubscriptions(db, sub.UserId, sub.Test)
	for _, s := range subs {
		if parent, _ := s.Metadata["bundleParent"].(string); parent == sub.Id() {
			if err := s.Delete(); err != nil {
				log.Error("RECONCILE: bundle subscription %s of withdrawn subscription %s was not removed: %v", s.Id(), sub.Id(), err)
			}
		}
	}
	if err := sub.Delete(); err != nil {
		log.Error("RECONCILE: subscription %s (subject=%s) was opened to replace a plan that could not be ended, and was not removed: %v", sub.Id(), sub.UserId, err)
	}
}

// extendRecorded answers a payment for a subject that already holds a paid plan.
func extendRecorded(ctx context.Context, org *organization.Organization, held *subscription.Subscription, in RecordIn, planID string, interval types.Interval, reference map[string]string, start, end time.Time) (*Recorded, error) {
	slug := heldSlug(held)
	if held.Type != subscription.External {
		return nil, saleRefusal{saleHeld, fmt.Sprintf(
			"this account already pays for the %q plan through checkout (subscription %s); name it in replaces to take it over, or change that subscription instead",
			slug, held.Id())}
	}
	if slug != planID || held.Plan.Interval != interval {
		return nil, saleRefusal{saleHeld, fmt.Sprintf(
			"this account already holds the %q plan by the %s (subscription %s); a payment for another plan cannot extend it, but may take it over by naming it in replaces",
			slug, held.Plan.Interval, held.Id())}
	}
	comp := strings.EqualFold(strings.TrimSpace(in.Processor), processorComp)
	if heldComp := held.ProviderType == processorComp; comp != heldComp {
		return nil, saleRefusal{saleHeld, fmt.Sprintf(
			"subscription %s is %s; a comp extends only a comp and a payment only a paid plan, so name it in replaces to take it over",
			held.Id(), map[bool]string{true: "a comp", false: "a paid plan"}[heldComp])}
	}
	// The period recorded again holds the seats it was paid for; a later period
	// opens at the seats a change asked for, when one waits for it.
	same := start.Equal(held.PeriodStart.UTC()) && end.Equal(held.PeriodEnd.UTC())
	quantity := held.Quantity
	if !same && held.PendingQuantity > 0 {
		quantity = held.PendingQuantity
	}
	if in.Quantity > 0 && in.Quantity != quantity {
		return nil, saleRefusal{saleRefused, fmt.Sprintf(
			"subscription %s holds %d seat(s) for this period; a payment for %d cannot extend it", held.Id(), quantity, in.Quantity)}
	}
	seatMult := int64(1)
	if held.Plan.PerSeat && quantity > 1 {
		seatMult = int64(quantity)
	}
	// A team's other seat types hold their seats for the period the same way,
	// and a payment that names its lines names the ones the row holds.
	lines := slices.Clone(held.Lines)
	for i := range lines {
		if l := &lines[i]; !same && l.PendingQuantity > 0 {
			l.Quantity, l.PendingQuantity = l.PendingQuantity, 0
		}
	}
	if why := sameLines(lines, in.Lines); why != "" {
		return nil, saleRefusal{saleRefused, fmt.Sprintf("subscription %s %s for this period; a payment for other seats cannot extend it", held.Id(), why)}
	}
	// The row keeps the prices it was opened at — and the customer's own, when one
	// was agreed — so a later payment is checked against them and not against
	// whatever the catalog sells today. A comp is held at 0 and was checked at the
	// door.
	if !comp {
		if err := priceMatches(slug, seatPrice(int64(held.Plan.Price), held.CustomerCents)*seatMult+lineCents(lines), in.PriceCents); err != nil {
			return nil, err
		}
	}
	if same {
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
	sub.Quantity, sub.PendingQuantity, sub.Lines = quantity, 0, lines
	sub.Metadata = recordMetadata(sub.Metadata, collectionExternal, sub.ProviderType, reference, in.Terms, start, end, seatMult+int64(lineSeats(lines)), in.PriceCents, lines)
	if err := sub.Update(); err != nil {
		return nil, err
	}
	if ev := in.Events; ev != nil {
		go ev.EmitSubscriptionRenewed(context.WithoutCancel(ctx), subscriptionEvent(org.Name, sub))
	}
	return &Recorded{Subscription: *viewSubscription(sub), Outcome: RecordExtended}, nil
}

// recordSeats is what a record opens a plan with, refused before anything moves:
// a team's other seat types priced at the period paid (linesAt), the seats of the
// plan's own, and the seats its price is multiplied by — one for a flat plan.
func recordSeats(db *datastore.Datastore, p *plan.Plan, in RecordIn) ([]subscription.Line, int, int64, error) {
	cur := p.Currency
	if cur == "" {
		cur = currency.USD
	}
	lines, err := linesAt(db, p, in.Lines, p.Interval, cur)
	switch {
	case errors.Is(err, errLinePlan):
		return nil, 0, 0, saleRefusal{saleMissing, "plan not found"}
	case errors.Is(err, errPlanUnread):
		return nil, 0, 0, saleRefusal{saleUnavailable, err.Error()}
	case err != nil:
		return nil, 0, 0, saleRefusal{saleRefused, err.Error()}
	}
	qty, err := seats(p, in.Quantity, lines, 0)
	if err != nil {
		return nil, 0, 0, saleRefusal{saleRefused, err.Error()}
	}
	if p.PerSeat {
		return lines, qty, int64(qty), nil
	}
	return lines, qty, 1, nil
}

// sameLines is why a payment naming lines does not name the seat types a row
// holds for the period, "" when it does or names none.
func sameLines(held []subscription.Line, in []LineIn) string {
	if len(in) == 0 {
		return ""
	}
	want := make(map[string]int, len(held))
	for _, l := range held {
		if l.Quantity > 0 {
			want[l.Plan] = l.Quantity
		}
	}
	got := make(map[string]int, len(in))
	for _, l := range in {
		got[strings.TrimSpace(l.PlanID)] += l.Quantity
	}
	if !maps.Equal(want, got) {
		return fmt.Sprintf("holds seat types %v", want)
	}
	return ""
}

// seatPrice is what one seat of a plan listed at list costs a customer whose own
// price is customer (0 for none): the lower of the two.
func seatPrice(list, customer int64) int64 {
	if customer > 0 && customer < list {
		return customer
	}
	return list
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

// collectionExternal is how a recorded row's periods are collected: by the
// processor, outside commerce.
const collectionExternal = "external"

// recordMetadata carries the collection on the row: how it is collected, who
// collected it, the latest payment's references, the terms, and every period
// recorded so far with the seats and the price its payment was checked against.
//
// A team's other seat types are recorded beside the seats (lines: each seat
// type's seats that period), so what the period paid for reads back by seat type
// (periodPaid), as a charged invoice's plan lines do.
func recordMetadata(prev types.Map, collection, processor string, reference map[string]string, terms string, start, end time.Time, seats, priceCents int64, lines []subscription.Line) types.Map {
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
	period := map[string]interface{}{
		"start":      start.Format(time.RFC3339),
		"end":        end.Format(time.RFC3339),
		"reference":  ref,
		"quantity":   seats,
		"priceCents": priceCents,
	}
	if by := map[string]interface{}{}; len(lines) > 0 {
		for _, l := range lines {
			if l.Quantity > 0 {
				by[l.Plan] = int64(l.Quantity)
			}
		}
		period["lines"] = by
	}
	periods, _ := m["periods"].([]interface{})
	m["periods"] = append(periods, period)
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

// processorComp is no processor at all: a plan given away, recorded at price 0.
const processorComp = "comp"

// RecordProcessors is every processor a payment collected outside checkout is
// recorded from, and comp, a plan given away, last. A record naming another is
// refused, so a person recording a payment picks one of these rather than
// spelling a name a reconciliation would not find.
var RecordProcessors = []string{"square", "paypal", "wire", "ach", "check", "crypto", processorComp}
