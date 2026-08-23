package billing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/api/promo"
	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/events"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/idempotencykey"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/paymentmethod"
	storemodel "github.com/hanzoai/commerce/models/store"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/thirdparty/kms"
	"github.com/hanzoai/commerce/util/json/http"
)

// processorsForOrg resolves the per-org payment processor registry — the ONE seam
// through which saved-card subscription code reaches the payment provider. It is a
// package var so tests can inject a registry with a fake Square processor;
// production is payment.ProcessorsForOrg (built from the org's KMS-hydrated
// credentials). Callers MUST hydrate the org's creds first (hydratePaymentCreds).
var processorsForOrg = payment.ProcessorsForOrg

// hydratePaymentCreds loads the org's payment-provider credentials from KMS (via
// the request-scoped cached client in c.Locals) so processorsForOrg sees real
// Square credentials. Best-effort + non-fatal — mirrors TopupWithToken /
// CreatePaymentMethod. A no-op when KMS is not wired (dev/tests / env-var creds).
func hydratePaymentCreds(c *zip.Ctx, org *organization.Organization) {
	if v := c.Locals("kms"); v != nil {
		if kmsClient, ok := v.(*kms.CachedClient); ok {
			if err := kms.Hydrate(kmsClient, org); err != nil {
				log.Error("KMS hydration failed for org %q: %v", org.Name, err, c)
			}
		}
	}
}

// inOrgSubject narrows a requested billing subject to the caller's OWN org: it
// returns the requested subject only when it is the org slug or a <org>/<user>
// child of it, else the org slug — fail-secure, so a subject (and the money keyed
// to it) can never be steered outside the caller's org. The in-org bound used by
// subscribeSubject. (The top-up endpoints no longer take a requested subject at all —
// they credit the caller's own payer identity, userBillingKey.)
func inOrgSubject(org, requested string) string {
	s := strings.ToLower(strings.TrimSpace(requested))
	if s == org || strings.HasPrefix(s, org+"/") {
		return s
	}
	return org
}

// subscribeSubject resolves the billing subject a card subscription belongs to:
// the org slug, or a finer in-org <org>/<user> subject when the requested userId
// is provably within the caller's own org (the subject arrives in the request
// BODY). Returns "" when no org resolves (caller 401s).
//
// NOTE: unlike the top-up endpoints (which credit the caller's own payer identity via
// userBillingKey), a subscription still honors a body userId within the org bound.
// A subscription is a durable object an org admin may create ON BEHALF OF a member,
// so its owner is not necessarily the caller. If that ever needs to become the
// signed-identity rule too, it is a deliberate change to subscription ownership,
// not a drive-by — hence it is called out here rather than silently aligned.
func subscribeSubject(c *zip.Ctx, requested string) string {
	org := orgBillingKey(c)
	if org == "" {
		return ""
	}
	return inOrgSubject(org, requested)
}

type subscribeCardRequest struct {
	// Exactly one of SourceID | MethodID names the card. SourceID is a fresh
	// Square Web Payments SDK nonce (single-use, vaulted before charging);
	// MethodID (wire: paymentMethodId) is a payment method the subject already
	// saved — its vaulted card is charged directly, nothing is re-entered and
	// no new row is created.
	SourceID string `json:"sourceId,omitempty"`
	MethodID string `json:"paymentMethodId,omitempty"`
	PlanID   string `json:"planId"`
	UserID   string `json:"userId,omitempty"`
	StoreID  string `json:"storeId,omitempty"`
	Quantity int    `json:"quantity,omitempty"`
	Currency string `json:"currency,omitempty"`
	// Level picks which of the plan's published prices to buy at — an INDEX into
	// the catalog's `prices`, never an amount. 0 (the default) is the plan's base
	// price, so every client that predates levels keeps buying exactly what it
	// bought before. plan.LevelPrice says why the wire carries a choice: there is
	// no field here an amount could be written in, so underpaying is not a check
	// that can be forgotten — it is a request that cannot be expressed.
	Level int `json:"level,omitempty"`
}

// SubscribeIn is the whole input of a card subscription. Every field is a VALUE
// the caller resolved — nothing is read from a request, a header or the
// environment inside the core.
type SubscribeIn struct {
	// Exactly one of SourceID | MethodID names the card. SourceID is a fresh
	// Square Web Payments SDK nonce (single-use, vaulted before charging);
	// MethodID is a card the subject already saved, charged directly.
	SourceID string
	MethodID string
	// PlanID is the plan to buy. Its CATALOG price is what gets charged; there is
	// no amount on this input, so underpaying is not a check that can be
	// forgotten — it is a request that cannot be expressed.
	PlanID string
	// Subject is the billing subject this subscription belongs to, ALREADY bounded
	// to the caller's own org by the endpoint that resolved it (subscribeSubject).
	Subject string
	// StoreID scopes the sale to one store, when the seller has several.
	StoreID string
	// Quantity is the billable seat count for a per-seat plan, floored at the
	// catalog's minSeats. A flat plan always bills once.
	Quantity int
	// Level picks which of the plan's published prices to buy at — an INDEX into
	// the catalog's prices, never an amount.
	Level int
	// Currency is honored only when it matches the plan's own; a mismatch is
	// refused rather than charged in a weaker unit.
	Currency string
	// Email is stamped on the Square customer a fresh vault creates.
	Email string
	// IdempotencyKey is the caller's explicit retry key. Empty falls back to the
	// windowed derivation over the stable facts (store, plan, level).
	IdempotencyKey string
	// Promo is the platform's active offer, resolved by the caller because
	// reading it needs a request. It can only ever REDUCE the charge, and a
	// charge that falls to nothing is refused rather than sold — so the price
	// this core charges is still the catalog's, discounted by an offer, never a
	// number the caller chose.
	Promo *promo.Promo
	// KMS is the request-scoped cached client used to hydrate the org's payment
	// credentials. Nil is legitimate and non-fatal (dev/tests, env-var creds).
	KMS *kms.CachedClient
	// Events is the analytics collector the money alarm fires on when a settled
	// charge leaves no subscription behind. Nil is a no-op.
	Events *events.Client
}

// Sale is the receipt for a card subscription: what was opened, what was
// charged, and what paid for it. The same fields the browser has always
// received, so the endpoint renders it directly and a typed caller reads the same
// answer.
type Sale struct {
	// SubscriptionID is the subscription this sale opened.
	SubscriptionID string `json:"subscriptionId"`
	// InvoiceID is the PAID first-period invoice. Empty when the charge settled
	// and the invoice record did not — the entitlement stands either way.
	InvoiceID string `json:"invoiceId"`
	// PlanID is the plan bought.
	PlanID string `json:"planId"`
	// Level is which of the plan's published prices it was bought at.
	Level int `json:"level"`
	// PaymentMethodID is the vaulted card that paid, and that renewals charge.
	PaymentMethodID string `json:"paymentMethodId"`
	// AmountCents is what the card was actually charged for the first period —
	// price x seats, less any promotion.
	AmountCents int64 `json:"amountCents"`
	// Currency is the ISO code, the plan's own.
	Currency string `json:"currency"`
	// Status is "ok" on a settled sale. A failure is an error, never a status.
	Status string `json:"status"`
}

// The kinds of no a sale can end in. They are separate because each is a
// different thing for the buyer to do — fix the request, name something that
// exists, wait for the sale already in flight, use another card, try a different
// card — and the endpoint turns each into the status it has always answered. The
// reason lives here; the number lives at the endpoint.
type saleKind int

const (
	saleRefused      saleKind = iota // the request asks for something that cannot be sold
	saleMissing                      // it named a plan or a store that is not there
	saleHeld                         // an identical sale is in flight, or this account already pays
	saleUnchargeable                 // no card rail can take this org's money
	saleDeclined                     // the card said no
)

// saleRefusal is a card subscription that did not happen, carrying which kind of
// no it was and the sentence the buyer should read.
type saleRefusal struct {
	kind saleKind
	msg  string
}

func (e saleRefusal) Error() string { return e.msg }

func isSale(err error, k saleKind) bool {
	var e saleRefusal
	return errors.As(err, &e) && e.kind == k
}

// IsSaleRefused reports whether the sale cannot be made as asked: no card named
// or two, no plan, a level the plan does not publish, seats below its floor, a
// currency it is not priced in, or a price of nothing.
func IsSaleRefused(err error) bool { return isSale(err, saleRefused) }

// IsSaleNotFound reports whether the request named a plan or store that is not
// on sale here. A retired tier answers the same as one that never existed, so
// archiving a plan cannot be detected by trying to buy it.
func IsSaleNotFound(err error) bool { return isSale(err, saleMissing) }

// IsSaleConflict reports whether something already holds this sale: an identical
// attempt is in flight, or the account already pays for a plan. Neither is
// retryable — the answer is to wait, or to change the subscription that exists.
func IsSaleConflict(err error) bool { return isSale(err, saleHeld) }

// IsSaleUnchargeable reports whether this organization has no card-on-file rail
// able to take the money — a configuration state, not a decline.
func IsSaleUnchargeable(err error) bool { return isSale(err, saleUnchargeable) }

// IsSaleDeclined reports whether the processor refused the card. The message is
// already the customer-facing sentence parseCardDeclineReason produced, which is
// why the endpoint sends it verbatim rather than writing a second one.
func IsSaleDeclined(err error) bool { return isSale(err, saleDeclined) }

// saleOutcome is what the subscribe worker learned, carried by VALUE.
//
// It exists because a sale has two shapes of answer — a fresh one and a replay
// of a sealed one — and the endpoint needs to tell them apart to answer 201 or 200.
// It carries the ROWS as well as the receipt so the endpoint can fire its analytics
// off the models, exactly as it always has.
type saleOutcome struct {
	// Sale is the receipt for a fresh sale. Nil when Replayed is set.
	Sale *Sale
	// Sub is the subscription that was opened.
	Sub *subscription.Subscription
	// Inv is the paid first-period invoice, or nil when recording it failed.
	Inv *billinginvoice.BillingInvoice
	// Replayed is the sealed body of an earlier identical sale, VERBATIM. Raw
	// JSON rather than a map: it is the bytes the first answer was, so a retry
	// gets that answer back byte for byte instead of one built again.
	Replayed json.RawMessage `json:"replayed,omitempty"`
}

// Sold is what a sale answered with, and it keeps the two shapes APART.
//
// A sale has two: a fresh receipt, and the sealed body of an identical earlier
// one replayed verbatim. A caller has to tell them apart — the endpoint answers 201
// for the first and 200 for the second, and a retry that got 201 would read as a
// second subscription having been opened. Exactly one field is ever set.
type Sold struct {
	// Sale is the receipt for a fresh sale. Nil on a replay.
	Sale *Sale `json:"sale,omitempty"`
	// Replayed is the sealed body of the earlier sale, VERBATIM — the bytes the
	// first answer was, not a second rendering of them.
	Replayed json.RawMessage `json:"replayed,omitempty"`
}

// SubscribeCard is Subscribe with the two answers kept apart, and with the
// sale's own analytics fired from the context rather than from a request.
//
// It exists because Subscribe flattens a replay into a receipt, which is right
// for a caller that only wants to know what was bought and wrong for an ENDPOINT,
// which owes the status and the events. A peer serving this address needs both,
// so both are answered here rather than re-derived on the other side.
func SubscribeCard(ctx context.Context, org *organization.Organization, in SubscribeIn) (*Sold, error) {
	out, err := subscribe(ctx, org, in)
	if err != nil {
		return nil, err
	}
	if len(out.Replayed) > 0 {
		return &Sold{Replayed: out.Replayed}, nil
	}
	emitSale(ctx, in.Events, org.Name, out.Sub, out.Inv)
	return &Sold{Sale: out.Sale}, nil
}

// Subscribe vaults or reuses a card, charges it for the plan's FIRST period at
// the SERVER-AUTHORITATIVE catalog price, and opens the subscription — one act,
// all of it server-side.
//
// The settled charge IS the mint authority here, which is why this path creates
// a paid-tier subscription without the C1-a gate CreateBillingSubscription
// applies: that gate exists to stop a paid tier being minted for FREE, and money
// that arrived is the thing it was asking for.
//
// It takes values rather than a request so the sale is reachable by a peer that
// holds no ledger. What it will NOT do is resolve who is buying: Subject arrives
// already bounded to the caller's own org, because a core that resolved identity
// from its own input would let any endpoint hand it a subject nobody proved.
func Subscribe(ctx context.Context, org *organization.Organization, in SubscribeIn) (*Sale, error) {
	out, err := subscribe(ctx, org, in)
	if err != nil {
		return nil, err
	}
	if len(out.Replayed) > 0 {
		var prev Sale
		if err := json.Unmarshal(out.Replayed, &prev); err != nil {
			return nil, fmt.Errorf("could not read the completed sale: %w", err)
		}
		return &prev, nil
	}
	return out.Sale, nil
}

// subscribe is the worker: it returns the ROWS alongside the receipt, so the
// HTTP endpoint emits from the models and answers 201 or 200 as it always has, while
// the typed caller reads Sale. Two projections of one result, never two
// implementations.
//
// The order is the money order and is not incidental: everything that can refuse
// refuses BEFORE the card is touched, the idempotency guard is taken before the
// charge, the one-paid-subscription check sits AFTER the guard so a lost-response
// retry replays its receipt instead of being told it already pays, and the
// subscription is created only once the charge has settled.
func subscribe(ctx context.Context, org *organization.Organization, in SubscribeIn) (*saleOutcome, error) {
	if org == nil {
		return nil, errors.New("subscribe: no organization")
	}
	if in.Subject == "" {
		return nil, errors.New("subscribe: no subject")
	}

	// Hydrate the org's payment credentials so the per-org Square processor used
	// to vault + charge the card carries real credentials. Best-effort: a missing
	// KMS client is dev/test with env-var creds, not a failure.
	if in.KMS != nil {
		if err := kms.Hydrate(in.KMS, org); err != nil {
			log.Error("KMS hydration failed for org %q: %v", org.Name, err)
		}
	}
	db := datastore.New(org.Namespaced(ctx))

	sourceID := strings.TrimSpace(in.SourceID)
	methodID := strings.TrimSpace(in.MethodID)
	planID := strings.TrimSpace(in.PlanID)
	if (sourceID == "") == (methodID == "") {
		return nil, saleRefusal{saleRefused, "send exactly one of sourceId (a new card) or paymentMethodId (a saved card)"}
	}
	if planID == "" {
		return nil, saleRefusal{saleRefused, "planId is required"}
	}
	if in.StoreID != "" {
		s := storemodel.New(db)
		if err := s.GetById(in.StoreID); err != nil {
			return nil, saleRefusal{saleMissing, "store not found"}
		}
	}

	// Server-authoritative plan + price. A client amount is NEVER consulted — the
	// charge is the plan's catalog price × billable seats, so a scripted request
	// cannot underpay.
	p, err := resolveSubscriptionPlan(db, planID)
	if err != nil {
		return nil, saleRefusal{saleMissing, "plan not found"}
	}
	if !p.Listed() {
		// Refused BEFORE the card is charged — a retired tier must never take money.
		return nil, saleRefusal{saleMissing, "plan not found"}
	}
	// The price this card is charged, chosen from what the catalog publishes. A
	// level the plan does not publish is refused here — before the card is
	// touched, before the idempotency guard is taken, and without ever consulting
	// an amount the client sent, because the client sends none.
	p, err = planAtLevel(p, in.Level)
	if err != nil {
		return nil, saleRefusal{saleRefused, err.Error()}
	}
	if int64(p.Price) <= 0 {
		// A $0 plan needs no card — the free tier is self-serve via POST
		// /v1/billing/subscriptions. This endpoint's contract is a PAID card sub.
		return nil, saleRefusal{saleRefused,
			fmt.Sprintf("plan %q is free — no card charge required; use POST /v1/billing/subscriptions", planID)}
	}
	// Billable seats: a per-seat plan charges Price × quantity (floored at the
	// catalog's minSeats); a flat plan is always ×1. This MUST match
	// buildPeriodInvoice's seat math so the first charge equals the invoice's
	// AmountDue — never under-collect a per-seat plan yet record it paid in full.
	qty := in.Quantity
	if qty < 1 {
		qty = 1
	}
	seatMult := int64(1)
	if perSeat(planID) {
		if min := minSeats(planID); qty < min {
			return nil, saleRefusal{saleRefused,
				fmt.Sprintf("plan %q requires at least %d seats (got %d)", planID, min, qty)}
		}
		seatMult = int64(qty)
	}
	chargeCents := int64(p.Price) * seatMult
	// The advertised promo, actually applied. GET /v1/billing/plans annotates each
	// plan with promoPercent off the SAME gate (active + inside its window +
	// AppliesTo), and until now nothing subtracted it here: the page could say
	// "50% off" while this line charged full price and the receipt agreed with the
	// charge, not the offer. Resolved once, carried on the subscription, so every
	// renewal prices identically — and computed through engine.DiscountCents, the
	// same function that discounts the invoice, so charge == AmountDue by
	// construction rather than by two implementations agreeing.
	promoPercent, promoName := 0, ""
	if pr := in.Promo; pr != nil && pr.AppliesTo(planID) {
		promoPercent, promoName = pr.PercentOff, pr.Name()
		chargeCents -= engine.DiscountCents(chargeCents, promoPercent)
	}
	if chargeCents <= 0 {
		// A 100%-off promo leaves nothing to charge, and a card sale of $0 is not a
		// sale — Square rejects a zero charge, so this would surface as a decline.
		// Refuse BEFORE the card is touched and name the real reason.
		return nil, saleRefusal{saleRefused,
			fmt.Sprintf("plan %q costs nothing after the current promotion — no card charge required; use POST /v1/billing/subscriptions", planID)}
	}
	// Currency is SERVER-AUTHORITATIVE: the plan's own currency (default USD). A
	// client currency is honored ONLY when it matches the plan's; a mismatched
	// currency (e.g. "jpy" for a USD-priced plan) is rejected — otherwise the
	// price's minor units would be charged in a weaker unit (2000 JPY ~ $13 for a
	// $20 plan), an underpayment.
	cur := p.Currency
	if cur == "" {
		cur = currency.USD
	}
	if reqCur := currency.Type(strings.ToLower(strings.TrimSpace(in.Currency))); reqCur != "" && reqCur != cur {
		return nil, saleRefusal{saleRefused,
			fmt.Sprintf("plan priced in %s; currency %s not accepted", cur, reqCur)}
	}

	// Idempotency (money-critical). The client SHOULD send a STABLE X-Idempotency-Key
	// per checkout attempt (the SPA does), so a lost-response retry — even with a
	// FRESH single-use nonce — replays instead of vaulting + charging again. Absent a
	// key, fall back to the STABLE (store, plan, level): the single-use nonce is NOT a
	// stable key (a re-tokenized retry mints a new nonce), which is exactly the
	// double-charge to avoid. Scoped to the subject so keys never collide across tenants.
	//
	// The LEVEL is part of those facts, for the same reason the seat count is: a
	// different level is a different purchase. Keyed on the plan alone, a customer
	// who came back and moved Max from $99 to $299 inside the window would reuse
	// the first attempt's key, and the server would REPLAY the $99 charge and hand
	// back its receipt — selling the wrong thing quietly. Scoping by level keeps
	// the retry guard doing its job without letting it mask a genuine change.
	guard := in.IdempotencyKey
	if guard == "" {
		guard = windowKey("store:" + in.StoreID + ":plan:" + planID + ":level:" + strconv.Itoa(in.Level))
	}
	// The Square idempotency key is derived from the SAME stable guard key (never the
	// single-use nonce), so Square itself de-dups the money move even if the local
	// guard store is unavailable OR two submits race — the definitive backstop.
	squareKey := gatewayKey("subscribe", in.Subject, guard)

	rec, replay, gerr := idempotencykey.Begin(db, "billing-subscribe:"+in.Subject+":"+in.StoreID, guard)
	if gerr != nil {
		// Guard store unavailable. Proceed WITHOUT the local guard: the stable Square
		// idempotency key above still makes the CHARGE exactly-once at the processor,
		// so a retry can't double-charge (at worst a duplicate sub row, far less
		// severe than a double charge) rather than block a legitimate first subscribe.
		log.Error("subscribe idempotency Begin failed (subject=%s): %v", in.Subject, gerr)
		rec = nil
	} else if replay {
		if rec.Status == idempotencykey.StatusCompleted && rec.Response != "" {
			return &saleOutcome{Replayed: json.RawMessage(rec.Response)}, nil
		}
		return nil, saleRefusal{saleHeld, "subscription already in progress"}
	}
	// abandon releases the guard on pre-charge failures where no money moved, so a
	// later attempt is not wedged.
	abandon := func() {
		if rec != nil {
			_ = rec.Delete()
		}
	}

	// ONE PAID SUBSCRIPTION PER SUBJECT — refused BEFORE the card is touched.
	//
	// This endpoint only ever STARTED a subscription: it charged, then called
	// createSubscription unconditionally, and nothing looked for one the subject
	// already had. The idempotency guard did not help — it keys on (subject, store,
	// plan), so a DIFFERENT plan is a different key and replays nothing. A paying
	// Pro customer clicking Upgrade on Max was therefore charged $99 in full, got a
	// SECOND active subscription, and kept renewing the first one. billingSubscription
	// explains why a second row also shadows the first's allotment.
	//
	// Refusing is the honest answer rather than the complete one: MOVING the
	// subscription is what the customer asked for, and doing that correctly means
	// pricing a mid-period change (what is owed today versus at renewal), which is a
	// policy decision this act cannot invent. PATCH /v1/billing/subscriptions/:id is
	// where a move belongs, and it currently refuses an allotment increase without
	// mint credentials precisely because engine.ChangePlan swaps the plan for free.
	// Until that endpoint can charge, a customer who wants a different tier is told so by
	// name — which is strictly better than being billed twice for it.
	//
	// It sits AFTER the idempotency guard on purpose. A retry carrying the SAME key
	// is a lost-response retry of the attempt that created the subscription, and it
	// must replay that receipt — asking first would answer a conflict to the customer
	// whose payment actually went through.
	if held := billingSubscription(db, in.Subject); held != nil {
		abandon()
		heldSlug := held.Plan.Slug
		if heldSlug == "" {
			heldSlug = held.PlanId
		}
		return nil, saleRefusal{saleHeld, fmt.Sprintf(
			"this account already pays for the %q plan (subscription %s); change that subscription instead of buying a second one",
			heldSlug, held.Id())}
	}

	// Resolve the card to charge. A fresh nonce is vaulted through saveCard (the
	// ONE constructor — validates by vaulting, stamps brand/last4, and returns
	// the existing row instead of stacking a duplicate); a paymentMethodId names
	// a card the subject already saved, charged directly with nothing re-entered.
	reg := processorsForOrg(org)
	cp, ok := squareCustomerProcessorFrom(reg)
	if !ok {
		abandon()
		return nil, saleRefusal{saleUnchargeable, "no card-on-file payment processor available for this organization"}
	}
	var pm *paymentmethod.PaymentMethod
	fresh := false // whether THIS request vaulted a new card (decline cleanup)
	if methodID != "" {
		pm = paymentmethod.New(db)
		if err := pm.GetById(methodID); err != nil {
			abandon()
			return nil, fmt.Errorf("%w: %v", errNoMethod, err)
		}
		// The subject is the pinned caller; another subject's method inside the org
		// is unreachable — the same answer as one that does not exist, so nothing
		// here is an existence oracle.
		if pm.CustomerId != in.Subject && pm.UserId != in.Subject {
			abandon()
			return nil, errNoMethod
		}
		if strings.TrimSpace(pm.ProviderRef) == "" || squareCustomerIDOf(pm) == "" {
			abandon()
			return nil, errMethodUnchargeable
		}
	} else {
		var err error
		pm, fresh, err = saveCard(ctx, db, cp, in.Subject, strings.TrimSpace(in.Email), sourceID)
		if err != nil {
			abandon()
			return nil, saleRefusal{saleDeclined, parseCardDeclineReason(&processor.PaymentResult{ErrorMessage: err.Error()}, err)}
		}
	}

	// Charge the SAVED card (card-on-file id + Square customer id) for the first
	// period at the server-authoritative price × seats, with the stable Square
	// idempotency key. All-or-nothing: on a decline no subscription is created.
	res, err := chargeSavedCard(ctx, reg, pm.ProviderRef, squareCustomerIDOf(pm), chargeCents, cur, squareKey,
		fmt.Sprintf("Subscription %s — first period", p.Name))
	if err != nil || res == nil || !res.Success {
		// A card vaulted BY THIS REQUEST is removed again so a declined attempt
		// leaves no orphan — but a card the customer already had on file (saved
		// earlier, or the dedupe match) is theirs and stays.
		if fresh {
			_ = cp.RemovePaymentMethod(ctx, squareCustomerIDOf(pm), pm.ProviderRef)
			_ = pm.Delete()
		}
		abandon()
		log.Error("saved-card charge failed for subscribe (subject=%s): %v", in.Subject, err)
		return nil, saleRefusal{saleDeclined, parseCardDeclineReason(res, err)}
	}

	// The charged card is the subscription's default from here on.
	if !pm.IsDefault {
		pm.IsDefault = true
		if err := pm.Update(); err != nil {
			log.Error("subscribe: failed to mark method %s default (subject=%s): %v", pm.Id(), in.Subject, err)
		}
	}

	// Create the subscription, authorized because the payment settled (mint
	// authority = the settled charge — the CreateBillingSubscription C1-a gate is
	// deliberately not on this path). No trial: the customer is paying now, so the
	// first period is Active immediately.
	p.TrialPeriodDays = 0
	sub, err := createSubscription(db, p, &createSubscriptionRequest{
		UserId:               in.Subject,
		PlanId:               planID,
		StoreId:              in.StoreID,
		DefaultPaymentMethod: pm.Id(),
		Quantity:             qty,
		Metadata:             map[string]interface{}{"source": "subscribe/card"},
	})
	if err != nil {
		// Money moved + card saved; surface the failure. Leave the guard STARTED so a
		// retry replays rather than re-charging.
		uncredited(ctx, in.Events, org.Name, in.Subject, res.ProcessorRef,
			"the first-period charge settled and no subscription was created: "+err.Error(),
			chargeCents, false)
		return nil, err
	}

	// Mark the FIRST period PAID by the card charge — a paid BillingInvoice
	// referencing the processor ref — and advance to the next period. This also makes
	// the subscription payment-backed (Square provider + a linked invoice), so its
	// INCLUDED monthly allotment flows (subscriptionPaymentBacked).
	sub.ProviderType = string(processor.Square)
	// Stamp the promo BEFORE the invoice is built — buildPeriodInvoice prices the
	// discount off the subscription, so setting it after would invoice the first
	// period at full price against a discounted charge.
	sub.DiscountPercent, sub.DiscountName = promoPercent, promoName
	inv, err := engine.CreatePaidFirstInvoice(db, sub, "card", res.ProcessorRef)
	if err != nil {
		// The subscription exists and the money is collected; a first-invoice record
		// failure is non-fatal to the entitlement. Log for reconciliation.
		log.Error("subscribe: failed to record paid first invoice (subject=%s, ref=%s): %v", in.Subject, res.ProcessorRef, err)
	}
	if err := sub.Update(); err != nil {
		log.Error("subscribe: failed to update subscription after first invoice (subject=%s): %v", in.Subject, err)
	}

	invoiceID := ""
	if inv != nil {
		invoiceID = inv.Id()
	}
	sale := &Sale{
		SubscriptionID:  sub.Id(),
		InvoiceID:       invoiceID,
		PlanID:          planID,
		Level:           in.Level,
		PaymentMethodID: pm.Id(),
		AmountCents:     chargeCents,
		Currency:        string(cur),
		Status:          "ok",
	}
	// Seal the idempotency guard with the exact success body so a retry replays it
	// verbatim (no second vault/charge/subscribe, identical response).
	if rec != nil {
		if body, mErr := json.Marshal(sale); mErr == nil {
			_ = idempotencykey.Complete(rec, string(body))
		}
	}
	return &saleOutcome{Sale: sale, Sub: sub, Inv: inv}, nil
}

// SubscribeWithCard vaults a Square card nonce as a reusable card-on-file, charges
// it for the plan's FIRST period at the SERVER-AUTHORITATIVE catalog price, and
// creates the subscription — all server-side, in one transaction of intent.
//
//	POST /v1/billing/subscribe/card
//
// Body: { sourceId | paymentMethodId, planId, userId?, quantity?, level?, currency? }
// — NO client amount: the price is the plan's catalog price, always.
// Header (optional): X-Idempotency-Key — a retry/double-submit with the same key
// (or, absent a key, the same store/plan/level inside a window) never
// double-charges: it replays the first result verbatim.
// Returns: { subscriptionId, invoiceId, planId, level, paymentMethodId, amountCents, currency, status }
//
// Everything below the bind is reading values off the request and handing them to
// Subscribe: the buyer bounded to the caller's own org, the retry key from the
// header, the buyer's email and the KMS and analytics clients from the request
// locals, and the platform promo, which needs a request to read and so is
// resolved here rather than inside the act.
func SubscribeWithCard(c *zip.Ctx) error {
	// The OK form, for the reason orgBillingKey states: GetOrganization
	// MustGet-PANICS on a missing org, and this handler is reachable without one.
	// IAMTokenRequired deliberately FALLS THROUGH without setting the local when
	// the gateway named no principal (`ownerID == "" || userID == ""`), so legacy
	// auth still gets its turn — every other billing read tolerates that; this
	// endpoint dereferenced it and took the whole request down at line one.
	//
	// Measured in production 2026-08-06: the panic recovered as a bare 500 with no
	// body, so the console could not say what went wrong, and `zero subscriptions
	// exist platform-wide` was the visible symptom. It also starves the serving
	// plane — ai's funding gate requires a confirmed paying subscriber for any
	// prepaid SKU and fails closed, so with no subscription anywhere every prepaid
	// model is refused for everyone.
	org, ok := middleware.GetOrganizationOK(c)
	if !ok || org == nil {
		return http.Fail(c, 401, "sign in to subscribe", nil)
	}

	var req subscribeCardRequest
	if err := c.Bind(&req); err != nil {
		return http.Fail(c, 400, "invalid request body", err)
	}

	// Billing subject: the org slug, or an in-org <org>/<user> ONLY when provably
	// within the caller's own org (userId is honored only inside that bound).
	subject := subscribeSubject(c, req.UserID)
	if subject == "" {
		return http.Fail(c, 401, "missing identity headers", nil)
	}

	iamEmail, _ := c.Locals("iam_email").(string)
	out, err := subscribe(c.Context(), org, SubscribeIn{
		SourceID:       req.SourceID,
		MethodID:       req.MethodID,
		PlanID:         req.PlanID,
		Subject:        subject,
		StoreID:        req.StoreID,
		Quantity:       req.Quantity,
		Level:          req.Level,
		Currency:       req.Currency,
		Email:          iamEmail,
		IdempotencyKey: strings.TrimSpace(c.Header("X-Idempotency-Key")),
		Promo:          promo.Active(c),
		KMS:            kmsOf(c),
		Events:         eventsOf(c),
	})
	if err != nil {
		switch {
		case IsSaleRefused(err):
			return http.Fail(c, 400, err.Error(), nil)
		case IsSaleNotFound(err):
			return http.Fail(c, 404, err.Error(), nil)
		case IsSaleConflict(err):
			return http.Fail(c, 409, err.Error(), nil)
		case IsSaleUnchargeable(err):
			return http.Fail(c, 422, err.Error(), nil)
		case IsSaleDeclined(err):
			return http.Fail(c, 402, err.Error(), nil)
		case IsMethodNotFound(err):
			return http.Fail(c, 404, "payment method not found", err)
		case IsMethodUnchargeable(err):
			return http.Fail(c, 422, "saved payment method has no chargeable card — add the card again", nil)
		}
		// What is left is the subscription create itself failing after the money
		// settled: a seat refusal is the caller's 400, anything else a 500.
		return subscriptionCreateError(c, err)
	}

	if len(out.Replayed) > 0 {
		// The sealed body IS the answer, verbatim — the same bytes the first
		// attempt sent, not a second rendering of them.
		c.SetHeader("Content-Type", "application/json")
		return c.Bytes(200, out.Replayed)
	}

	emitSubscriptionCreated(c, org.Name, out.Sub)
	if out.Inv != nil {
		emitInvoicePaid(c, org.Name, out.Inv)
	}
	return c.JSON(201, out.Sale)
}

// chargeSavedCard charges an already-vaulted card-on-file (cardID owned by the
// Square customerID) for amountCents via the org's processor registry. It is the
// ONE saved-card charge path — shared by SubscribeWithCard's first charge and the
// renewal ProviderCharger. reg MUST be a per-org registry (processorsForOrg(org))
// whose org has hydrated Square credentials.
func chargeSavedCard(ctx context.Context, reg *processor.Registry, cardID, customerID string, amountCents int64, cur currency.Type, idempotencyKey, desc string) (*processor.PaymentResult, error) {
	req := processor.PaymentRequest{
		Token:          cardID,     // Square card-on-file id as the SourceID
		CustomerID:     customerID, // the Square customer that owns the card (required for card-on-file)
		Amount:         currency.Cents(amountCents),
		Currency:       cur,
		IdempotencyKey: idempotencyKey, // Square de-dups the money move on this key
		Description:    desc,
	}
	proc, err := reg.SelectProcessor(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("no payment processor for saved-card charge: %w", err)
	}
	return proc.Charge(ctx, req)
}

// chargeProviderForOrg returns an engine.ProviderCharger bound to org: given an
// open invoice, it resolves the invoice's subscription → DefaultPaymentMethod →
// the vaulted card (paymentmethod.ProviderRef = card-on-file id, Metadata
// squareCustomerId) and charges the remaining amount on it via the org's Square
// processor, returning the processor ref. org MUST already be KMS-hydrated by the
// caller (as TopupWithToken / auto-recharge do) so processorsForOrg sees real
// credentials. A resolution/charge failure returns an error → CollectInvoice
// leaves the invoice OPEN (no double-charge on retry).
func chargeProviderForOrg(org *organization.Organization) engine.ProviderCharger {
	return func(ctx context.Context, db *datastore.Datastore, inv *billinginvoice.BillingInvoice, amountCents int64) (string, error) {
		if amountCents <= 0 {
			return "", fmt.Errorf("nothing to charge")
		}
		if inv.SubscriptionId == "" {
			return "", fmt.Errorf("invoice %s has no subscription; no card on file to charge", inv.Id())
		}
		sub := subscription.New(db)
		if err := sub.GetById(inv.SubscriptionId); err != nil {
			return "", fmt.Errorf("subscription %s not found: %w", inv.SubscriptionId, err)
		}
		if strings.TrimSpace(sub.DefaultPaymentMethod) == "" {
			return "", fmt.Errorf("subscription %s has no default payment method", sub.Id())
		}
		pm := paymentmethod.New(db)
		if err := pm.GetById(sub.DefaultPaymentMethod); err != nil {
			return "", fmt.Errorf("payment method %s not found: %w", sub.DefaultPaymentMethod, err)
		}
		cardID := strings.TrimSpace(pm.ProviderRef)
		if cardID == "" {
			return "", fmt.Errorf("payment method %s has no card-on-file token", pm.Id())
		}
		cur := inv.Currency
		if cur == "" {
			cur = currency.USD
		}
		// Square idempotency key scoped to (subscription, period, attempt). The PERIOD
		// (not the invoice id) is the shared axis: concurrent renewals each build their
		// OWN invoice row for the same period, so an invoice-id key would NOT de-dup them
		// — the period does. AttemptCount makes each dunning RETRY a FRESH key so the
		// processor lets the re-collection through (a period-only key would make the
		// gateway replay the first decline forever and wedge dunning).
		squareKey := "collect:sub:" + inv.SubscriptionId +
			":period:" + strconv.FormatInt(inv.PeriodStart.Unix(), 10) +
			":attempt:" + strconv.Itoa(inv.AttemptCount)
		reg := processorsForOrg(org)
		res, err := chargeSavedCard(ctx, reg, cardID, squareCustomerIDOf(pm), amountCents, cur, squareKey,
			fmt.Sprintf("Subscription renewal invoice %s", inv.NumberStr))
		if err != nil || res == nil || !res.Success {
			return "", fmt.Errorf("%s", parseCardDeclineReason(res, err))
		}
		return res.ProcessorRef, nil
	}
}

// squareCustomerIDOf reads the Square customer id stored on a vaulted payment
// method (the CreatePaymentMethod / subscribe-card convention:
// Metadata["squareCustomerId"]). A card-on-file is only chargeable in its Square
// customer's context, so this is required to charge a saved card.
func squareCustomerIDOf(pm *paymentmethod.PaymentMethod) string {
	if pm.Metadata != nil {
		if v, ok := pm.Metadata["squareCustomerId"].(string); ok {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
