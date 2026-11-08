package billing

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/api/promo"
	"github.com/hanzoai/commerce/billing/creditledger"
	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/events"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/idempotencykey"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/paymentmethod"
	"github.com/hanzoai/commerce/models/paymentorg"
	"github.com/hanzoai/commerce/models/plan"
	storemodel "github.com/hanzoai/commerce/models/store"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment"
	"github.com/hanzoai/commerce/payment/processor"
	types "github.com/hanzoai/commerce/types"
	"github.com/hanzoai/commerce/util/json/http"
)

// processorsForOrg resolves the per-org payment processor registry — the ONE seam
// through which saved-card subscription code reaches the payment provider. It is a
// package var so tests can inject a registry with a fake Square processor;
// production is payment.ProcessorsForOrg.
//
// Billing is a customer paying the brand, so the paying org's own payment
// credentials are never loaded here (payment.Hydrate is for the org that receives
// the money): ProcessorsForOrg falls back to the deployment's own Square account.
var processorsForOrg = payment.ProcessorsForOrg

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
	// Lines are the team's other seat types, bought with it (see LineIn).
	Lines    []LineIn `json:"lines,omitempty"`
	Currency string   `json:"currency,omitempty"`
	// Level picks which of the plan's published prices to buy at — an INDEX into
	// the catalog's `prices`, never an amount. 0 (the default) is the plan's base
	// price, so every client that predates levels keeps buying exactly what it
	// bought before. plan.LevelPrice says why the wire carries a choice: there is
	// no field here an amount could be written in, so underpaying is not a check
	// that can be forgotten — it is a request that cannot be expressed.
	Level int `json:"level,omitempty"`
	// Interval picks which of the plan's published billing periods to buy at.
	// Empty (the default) and "month" both buy the recurring price, so every
	// client that predates annual billing keeps buying exactly what it bought
	// before; "year" buys the catalog's annual price, charged for the whole year
	// at once. planAtInterval says what that amounts to.
	Interval types.Interval `json:"interval,omitempty"`
	// Coupon is a plan coupon CODE (case-insensitive) for a percent off the first
	// month — a code, never an amount: the percent is the stored coupon's.
	Coupon string `json:"coupon,omitempty"`
	// Wallet says SourceID is a wallet's single-use token (Apple Pay, Google Pay,
	// Cash App Pay, PayPal): it pays the first period itself and is kept nowhere,
	// so the subscription renews from the prepaid balance.
	Wallet bool `json:"wallet,omitempty"`
	// Provider names the explicit wallet/card rail ("square" or "paypal") when
	// SourceID belongs to a specific provider such as PayPal Orders v2.
	Provider string `json:"provider,omitempty"`
	// BalanceCents is the starting usage balance an organization's order adds to
	// its prepaid balance in the same payment (see SubscribeIn.BalanceCents).
	BalanceCents int64 `json:"balanceCents,omitempty"`
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
	// Lines are a team's other seat types, bought on the same subscription: plans
	// of PlanID's catalog group, each with its seats. The team's floor and ceiling
	// count every seat, and the charge is every seat type's price times its seats.
	Lines []LineIn
	// Level picks which of the plan's published prices to buy at — an INDEX into
	// the catalog's prices, never an amount.
	Level int
	// Interval picks which of the plan's published billing periods to buy at.
	// Empty means the plan's recurring price; types.Yearly buys the year at once.
	Interval types.Interval
	// Currency is honored only when it matches the plan's own; a mismatch is
	// refused rather than charged in a weaker unit.
	Currency string
	// Email is stamped on the Square customer a fresh vault creates, and named
	// in the operator's sale notice.
	Email string
	// Coupon is a plan coupon code the buyer typed. Empty buys at the price the
	// catalog and promo set. A code that does not stand, does not cover the plan,
	// or was redeemed by this org or card REFUSES the sale before the card is
	// charged — it never silently sells at full price.
	Coupon string
	// Wallet says SourceID is a wallet's single-use token: the first period is
	// authorized on it and captured once any coupon's use is reserved against the
	// card behind it. A wallet keeps no card, so its plan renews only once a card
	// plan is bought to follow it.
	Wallet bool
	// Provider names the explicit wallet processor ("square" or "paypal"); empty
	// defaults to Square.
	Provider string
	// IdempotencyKey is the caller's explicit retry key. Empty falls back to the
	// windowed derivation over the stable facts (store, plan, level).
	IdempotencyKey string
	// Promo is the platform's active offer, resolved by the caller because
	// reading it needs a request. It can only ever REDUCE the charge, and a
	// charge that falls to nothing is refused rather than sold — so the price
	// this core charges is still the catalog's, discounted by an offer, never a
	// number the caller chose.
	Promo *promo.Promo
	// Events is the analytics collector the money alarm fires on when a settled
	// charge leaves no subscription behind. Nil is a no-op.
	Events *events.Client
	// Delegated says the caller buys for a Subject that is not its own payer — an
	// org's admin buying for a member. The buyer who sees the plan's reload terms
	// is not the payer whose card a reload would charge, so a delegated sale
	// switches no reload on (armSale).
	Delegated bool
	// BalanceCents is the starting usage balance an organization's order adds to
	// its prepaid balance, charged to the card in the same payment as the plan's
	// first period and credited as a top-up is. It is the one amount a sale
	// carries, and the plan's row bounds it: whole dollars, no more than the row's
	// limits.balanceCeiling, and none on a plan whose row states no ceiling.
	// Zero buys the plan alone.
	BalanceCents int64
}

// armSale is what a paid sale of slug does to the buying payer's auto-recharge
// (armForPlan): nothing for a delegated sale.
func armSale(ctx context.Context, org *organization.Organization, in SubscribeIn, slug string, cur currency.Type) *Arming {
	if in.Delegated {
		return nil
	}
	return armForPlan(ctx, org, in.Subject, slug, cur)
}

// Sale is the receipt for a card subscription: what was opened, what was
// charged, and what paid for it. The same fields the browser has always
// received, so the endpoint renders it directly and a typed caller reads the same
// answer.
type Sale struct {
	// SubscriptionID is the subscription this sale opened.
	SubscriptionID string `json:"subscriptionId"`
	// InvoiceID is the PAID first-period invoice. Empty when the charge settled
	// and the invoice record did not — the entitlement stands either way — and
	// for a trial, which bills nothing today.
	InvoiceID string `json:"invoiceId"`
	// PlanID is the plan bought.
	PlanID string `json:"planId"`
	// Level is which of the plan's published prices it was bought at.
	Level int `json:"level"`
	// Interval is the period bought, and the period this subscription renews on:
	// "month" for the recurring price, "year" when AmountCents is the whole year.
	Interval types.Interval `json:"interval"`
	// PaymentMethodID is the vaulted card that paid, and that renewals charge.
	PaymentMethodID string `json:"paymentMethodId"`
	// AmountCents is what the card was actually charged: the first period — price
	// x seats, less any promotion and coupon — and the starting balance
	// (BalanceCents) when the order bought one. Zero for a trial.
	AmountCents int64 `json:"amountCents"`
	// BalanceCents is the part of AmountCents credited to the buyer's prepaid
	// balance, and BalanceID the receipt of that credit — the deposit row, or the
	// chain mint. Both are absent when the sale bought no starting balance.
	BalanceCents int64  `json:"balanceCents,omitempty"`
	BalanceID    string `json:"balanceId,omitempty"`
	// Seats is how many seats the sale bought, every seat type counted; 1 on a
	// flat plan.
	Seats int64 `json:"seats,omitempty"`
	// Category is the ladder the plan sits on — personal, team, enterprise — as
	// its row states it.
	Category string `json:"category,omitempty"`
	// Replaced is the paid subscription the account held that this sale replaced
	// and canceled once its own payment settled; absent when it held none.
	Replaced string `json:"replaced,omitempty"`
	// TrialEnd is when the free trial this sale opened ends and the first paid
	// period is charged. Absent when the sale charged today.
	TrialEnd *time.Time `json:"trialEnd,omitempty"`
	// TrialEndCents is what the first paid period costs at TrialEnd — price x
	// seats, less any promotion — collected as every renewal is. Absent when the
	// sale charged today.
	TrialEndCents int64 `json:"trialEndCents,omitempty"`
	// StartsAt is when the plan this sale bought starts and its card is first
	// charged: the end of the plan the account held, paid outside checkout or from
	// the balance (Follows). Absent when the sale charged today.
	StartsAt *time.Time `json:"startsAt,omitempty"`
	// StartsCents is what the first period costs at StartsAt — price x seats, less
	// any promotion — collected as every renewal is. Absent when the sale charged
	// today.
	StartsCents int64 `json:"startsCents,omitempty"`
	// Follows is the subscription the account held that this sale's plan follows:
	// it ends at StartsAt, as it was going to. Absent when the sale charged today.
	Follows string `json:"follows,omitempty"`
	// Currency is the ISO code, the plan's own.
	Currency string `json:"currency"`
	// Status is "ok" on a settled sale. A failure is an error, never a status.
	Status string `json:"status"`
	// Coupon, CouponPercent and CouponMonths name the coupon the subscription was
	// bought with, absent when none was: CouponPercent off each of its first
	// CouponMonths monthly periods, this one the first. Renewals after them bill
	// without it.
	Coupon        string `json:"coupon,omitempty"`
	CouponPercent int    `json:"couponPercent,omitempty"`
	CouponMonths  int    `json:"couponMonths,omitempty"`
	// Reload is what the sale did to the buyer's auto-recharge, present only for a
	// plan whose row states a reload (Arming). Absent on a trial, which switches
	// it on when its first paid period is charged.
	Reload *Arming `json:"reload,omitempty"`
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
	saleUnavailable                  // nothing was charged and the sale could not go on now; try again
	saleUnsettled                    // the processor did not confirm a payment it may have taken
)

// saleRefusal is a card subscription that did not happen, carrying which kind of
// no it was and the sentence the buyer should read.
type saleRefusal struct {
	kind saleKind
	msg  string
}

func (e saleRefusal) Error() string { return e.msg }

// Refusal is a sale refusal that also names which one it is, as a Code a page
// acts on instead of the sentence: "seats_out_of_range", "balance_out_of_range",
// "org_required", "not_sold_by_invoice", "plan_held", "invoice_open",
// "order_placing", "key_reused", "wire_not_configured", "plan_needs_card" (a plan
// asked of prepaid money), "processor_unknown" (a record naming no processor of
// RecordProcessors). Open is the invoice an
// "invoice_open" refusal names, with a new link to it. Every IsSale check reads
// the refusal it carries.
type Refusal struct {
	saleRefusal
	Code string
	Open *InvoiceOrder
}

// Unwrap is the refusal Refusal carries.
func (e *Refusal) Unwrap() error { return e.saleRefusal }

// coded is a refusal of kind with its code.
func coded(kind saleKind, code, msg string) *Refusal {
	return &Refusal{saleRefusal: saleRefusal{kind, msg}, Code: code}
}

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

// IsSaleUnavailable reports that nothing was charged and the sale could not be
// made just now: the attempt could not be recorded, or the processor could not
// be reached. Trying again is the remedy.
func IsSaleUnavailable(err error) bool { return isSale(err, saleUnavailable) }

// IsSaleUnsettled reports that the processor may have taken a payment it did not
// confirm. The attempt is sealed, so its retry is refused rather than charged
// again, and the payment is reconciled.
func IsSaleUnsettled(err error) bool { return isSale(err, saleUnsettled) }

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
// all of it server-side. A plan with a free trial opens in its trial instead,
// charging nothing until the trial ends (subscribe says when).
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
// subscription is created only once the charge has settled — or, for a trial,
// once the card is vaulted, with nothing charged.
func subscribe(ctx context.Context, org *organization.Organization, in SubscribeIn) (*saleOutcome, error) {
	if org == nil {
		return nil, errors.New("subscribe: no organization")
	}
	if in.Subject == "" {
		return nil, errors.New("subscribe: no subject")
	}
	if strings.EqualFold(strings.TrimSpace(in.Provider), string(processor.PayPal)) {
		in.Wallet = true
	}
	db := datastore.New(org.Namespaced(ctx))

	sourceID := strings.TrimSpace(in.SourceID)
	methodID := strings.TrimSpace(in.MethodID)
	planID := strings.TrimSpace(in.PlanID)
	if prepaidSource(sourceID) {
		// A plan is never bought from prepaid money: credits and the balance pay for
		// usage, and a plan is billed to a card every period.
		return nil, coded(saleRefused, "plan_needs_card", "a plan is bought with a card: prepaid credit and the balance pay for usage only")
	}
	if (sourceID == "") == (methodID == "") {
		return nil, saleRefusal{saleRefused, "send exactly one of sourceId (a new card) or paymentMethodId (a saved card)"}
	}
	if in.Wallet && sourceID == "" {
		return nil, saleRefusal{saleRefused, "a wallet pays with the token it minted: send it as sourceId"}
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
	// The order this request sends, whole: one key sent with another order is
	// refused, and an order whose starting balance is still owed is finished by
	// sending it again, under any key and whatever the catalog says now.
	facts := factsOf(in)
	if out, ok, err := owedSale(ctx, db, org, in, facts); ok {
		return out, err
	}

	q, err := price(db, org, in, planID, sourceID)
	if err != nil {
		return nil, err
	}
	p, planID, cur, lines, qty := q.plan, q.slug, q.cur, q.lines, q.qty
	bal, seatMult, seats, chargeCents := q.balance, q.seatMult, q.seats, q.charge
	promoPercent, promoName := q.promoPercent, q.promoName
	customer := q.customer

	// A coupon's request shape is checked here, before anything is held. Whether
	// the coupon stands, covers this plan and is still this org's to redeem is
	// state, and is asked after the idempotency guard, for the reason the
	// one-paid-subscription check is: a lost-response retry of a sale that used
	// the last redemption must replay its receipt, not be told the coupon is
	// exhausted.
	couponCode := ""
	if raw := strings.TrimSpace(in.Coupon); raw != "" {
		couponCode = promo.CouponCode(raw)
		switch {
		case couponCode == "":
			return nil, saleRefusal{saleRefused, "coupon is not valid"}
		case !engine.BillsMonthly(p):
			return nil, saleRefusal{saleRefused,
				fmt.Sprintf("coupon %s takes a percent off monthly periods, so it applies to monthly billing only", couponCode)}
		}
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
	//
	// The PERIOD is one of those facts too, and the same size of one: a buyer who
	// switched Dev from monthly to annual inside the window would replay the $19
	// charge and be handed a receipt for a year. It is the RESOLVED period that
	// scopes the key, not the field as sent, because an absent interval and "month"
	// are the same purchase and must share a key rather than charge twice.
	//
	// The COUPON is one of those facts when one is sent: a buyer who retried with a
	// different code, or with none, is making a different purchase. It is absent
	// from the facts when no coupon is sent, so a coupon-less sale keys exactly as
	// it always has.
	guard := in.IdempotencyKey
	if guard == "" {
		facts := "store:" + in.StoreID + ":plan:" + planID +
			":level:" + strconv.Itoa(in.Level) + ":interval:" + string(p.Interval)
		if couponCode != "" {
			facts += ":coupon:" + couponCode
		}
		// A team's other seat types are facts too: another mix is another purchase.
		for _, l := range lines {
			facts += ":line:" + l.Plan + ":" + strconv.Itoa(l.Quantity)
		}
		// So is a starting balance: another balance is another payment.
		if bal > 0 {
			facts += ":balance:" + strconv.FormatInt(bal, 10)
		}
		guard = windowKey(facts)
	}
	// The Square idempotency key is derived from the SAME stable guard key (never the
	// single-use nonce), so Square itself de-dups the money move even if the local
	// guard store is unavailable OR two submits race — the definitive backstop.
	squareKey := gatewayKey("subscribe", in.Subject, guard)

	rec, replay, gerr := idempotencykey.Begin(db, "billing-subscribe:"+in.Subject+":"+in.StoreID, guard)
	if gerr == nil && rec.Facts != "" && rec.Facts != facts {
		// A key names one order. The same key sent with another order — more
		// seats, another mix of them, another balance, plan, term or coupon — is
		// refused rather than answered with the first order's receipt.
		return nil, saleRefusal{saleHeld, "this X-Idempotency-Key was used for another order: send a new key for a new order"}
	}
	if gerr != nil {
		// Guard store unavailable. Proceed WITHOUT the local guard: the stable Square
		// idempotency key above still makes the CHARGE exactly-once at the processor,
		// so a retry can't double-charge (at worst a duplicate sub row, far less
		// severe than a double charge) rather than block a legitimate first subscribe.
		log.Error("subscribe idempotency Begin failed (subject=%s): %v", in.Subject, gerr)
		rec = nil
	} else if replay {
		if rec.Status != idempotencykey.StatusCompleted || rec.Response == "" {
			return nil, saleRefusal{saleHeld, "subscription already in progress"}
		}
		return &saleOutcome{Replayed: json.RawMessage(rec.Response)}, nil
	} else {
		rec.Facts = facts
	}
	if rec == nil && bal > 0 {
		// An order's starting balance is owed between its charge and its credit,
		// and the guard is what finishes it: without one, nothing is charged.
		return nil, saleRefusal{saleUnavailable, "billing could not be reached; nothing was charged, try again"}
	}
	// abandon releases the guard on pre-charge failures where no money moved, so a
	// later attempt is not wedged. A guard a stopped sale marked after its charge
	// (draftOf) holds that payment's sale, and is never released.
	abandon := func() {
		if rec != nil && rec.RecoveryPoint == "" {
			_ = rec.Delete()
		}
	}

	// ONE SALE PER SUBJECT IN FLIGHT, whatever key it came under. The guard above
	// is one key's; an order sent twice under two keys (two tabs, a key minted per
	// click) is two keys, and the one-paid-subscription check below reads the
	// subscriptions before either has written its own, so both would pass it and
	// both be charged. This lock is taken atomically and held until the sale
	// returns, so the second finds the first in flight, and once the first has
	// written its subscription the check below refuses the second by name. A store
	// that cannot be asked refuses the sale, with nothing charged.
	lease, lerr := holdSale(db, in.Subject)
	if lerr != nil {
		abandon()
		return nil, lerr
	}
	defer lease.Release()

	// A sale under this key stopped after its charge settled: it is finished from
	// what it recorded then, never charged or refused again.
	if d := draftOf(rec); d != nil {
		sub, err := opened(db, in.Subject, org.TestMode(), d.Payment)
		if err != nil {
			return nil, unfinished
		}
		p.TrialPeriodDays = 0
		return sell(ctx, db, org, in, rec, p, sub, qty, lines, d)
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
	held := billingSubscription(db, in.Subject, org.TestMode())
	// A card plan already waiting to start is this subject's next plan, so a second
	// one is refused as a second paid plan is.
	if waiting := scheduledFor(db, in.Subject, org.TestMode()); waiting != nil {
		abandon()
		return nil, saleRefusal{saleHeld, fmt.Sprintf(
			"this account's %q plan already starts by card on %s (subscription %s); cancel it to buy another",
			subscriptionSlug(waiting), waiting.PeriodStart.UTC().Format(time.DateOnly), waiting.Id())}
	}
	// A plan paid outside checkout, or one the balance used to renew, does not renew
	// by card. Bought by card while it runs, the card plan starts when it ends, so
	// nothing is paid twice: the card is kept now and charged at that start.
	starts := time.Time{}
	if held != nil && followed(held) {
		starts = held.PeriodEnd
	}
	switch {
	case starts.IsZero():
	case in.Wallet:
		abandon()
		return nil, saleRefusal{saleRefused, fmt.Sprintf(
			"this account's plan runs to %s, and a plan that starts then is charged to a card on file: a wallet keeps none, so pay with a card",
			starts.UTC().Format(time.DateOnly))}
	case bal > 0:
		abandon()
		return nil, coded(saleRefused, "balance_out_of_range", fmt.Sprintf(
			"this account's plan runs to %s and the card plan starts then, charging nothing today, so it takes no starting balance",
			starts.UTC().Format(time.DateOnly)))
	case couponCode != "":
		abandon()
		return nil, saleRefusal{saleRefused, fmt.Sprintf(
			"this account's plan runs to %s and the card plan starts then, charging nothing today; a coupon is redeemed on a charge made today",
			starts.UTC().Format(time.DateOnly))}
	}
	if held != nil && starts.IsZero() {
		heldSlug := held.Plan.Slug
		if heldSlug == "" {
			heldSlug = held.PlanId
		}
		isYear := in.Interval == types.Yearly
		heldYear := held.Plan.Interval == types.Yearly || string(held.Plan.Interval) == "year"
		samePlan := (heldSlug == planID || held.PlanId == planID) && held.Plan.Price == p.Price
		sameInterval := (isYear == heldYear)
		sameQty := in.Quantity <= 1 || held.Quantity == in.Quantity
		if held.Status == subscription.Trialing || (samePlan && sameInterval && sameQty) {
			abandon()
			return nil, saleRefusal{saleHeld, fmt.Sprintf(
				"this account already pays for the %q plan (subscription %s); change that subscription instead of buying a second one",
				heldSlug, held.Id())}
		}
	}

	slug := p.Slug
	if slug == "" {
		slug = planID
	}

	// The coupon, resolved now that the guard is held: it must stand, cover this
	// plan, and not have been redeemed by this org. Its percent comes off what
	// the promo left, through the function the invoice prices with, so the charge
	// equals the first invoice's AmountDue. The subscription is granted it for its
	// first months (engine.Grant), so the renewals inside them bill it too and
	// every renewal after them bills the catalog price.
	cpn := engine.Coupon{}
	var couponPlans []string
	if couponCode != "" {
		offer, reason := promo.FindCoupon(ctx, couponCode, time.Now())
		if reason == "" && !everyLine(slug, lines, offer.AppliesTo) {
			reason = promo.CouponNotApplicable
		}
		if reason != "" {
			abandon()
			return nil, saleRefusal{saleRefused, couponRefusal(couponCode, slug, reason)}
		}
		if promo.Redeemed(ctx, couponCode, org.Name) {
			abandon()
			return nil, saleRefusal{saleHeld, fmt.Sprintf("%v (%s)", promo.ErrCouponRedeemedByOrg, couponCode)}
		}
		cpn, couponPlans = offer.Coupon(), offer.Plans
		chargeCents = engine.Discounted(chargeCents, cpn.Percent)
		if chargeCents <= 0 {
			abandon()
			return nil, saleRefusal{saleRefused,
				fmt.Sprintf("coupon %s leaves nothing to charge for plan %q, and a card sale must charge something", couponCode, planID)}
		}
	}
	listCents := int64(p.Price)*seatMult + lineCents(lines)
	// What the receipt says of the sale beside its money: the paid plan it
	// replaces, if any.
	replaced := ""
	if held != nil {
		replaced = held.Id()
	}

	if in.Wallet {
		return walletSale(ctx, db, org, in, walletTerms{
			plan: p, planID: planID, slug: slug, qty: qty, seats: seatMult, lines: lines,
			charge: chargeCents, list: listCents, cur: cur, coupon: cpn, couponPlans: couponPlans,
			promoPercent: promoPercent, promoName: promoName, customer: customer,
			sourceID: sourceID, guard: guard, rec: rec, abandon: abandon,
		})
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
	// unvault removes a card vaulted BY THIS REQUEST when the sale ends without
	// taking money, so the attempt leaves no orphan — but a card the customer
	// already had on file (saved earlier, or the dedupe match) is theirs and stays.
	unvault := func() {
		if fresh {
			_ = cp.RemovePaymentMethod(ctx, squareCustomerIDOf(pm), pm.ProviderRef)
			_ = pm.Delete()
		}
	}

	if !starts.IsZero() {
		return scheduleSale(ctx, db, org, in, rec, p, pm, held, starts, saleTerms{
			planID: planID, slug: slug, qty: qty, lines: lines, seats: seats, charge: chargeCents,
			list: listCents, cur: cur, promoPercent: promoPercent, promoName: promoName, customer: customer,
		}, unvault, abandon)
	}

	// A plan sold with a free trial opens in it when this is the plan's first
	// trial for the buyer (heldBefore) and for the card, in any org
	// (promo.TakeTrial). Nothing is charged: the card was validated when it was
	// vaulted, and the renewal sweep charges it for the first paid period when the
	// trial ends (engine.IsDue). A buyer or a card that has had the trial buys
	// exactly as a plan without one does, charged now: the trial is an offer to new
	// buyers, and refusing the sale instead would leave a returning buyer no way to
	// buy the plan. A store that cannot answer ends the sale with nothing charged:
	// neither a trial nor a charge the buyer did not expect.
	if p.TrialPeriodDays > 0 {
		held, err := heldBefore(db, slug, in.Subject, org.TestMode())
		if err != nil {
			unvault()
			abandon()
			return nil, fmt.Errorf("plan %q has a free trial and the subscriptions of %s could not be read, so nothing was charged: %w", slug, in.Subject, err)
		}
		var release func()
		if !held {
			release, err = promo.TakeTrial(ctx, slug, instrumentOf(pm), org.TestMode())
			if err != nil && !errors.Is(err, promo.ErrTrialTaken) {
				unvault()
				abandon()
				return nil, fmt.Errorf("plan %q has a free trial and the card's use of it was not recorded, so nothing was charged: %w", slug, err)
			}
		}
		if release != nil {
			// A coupon waits out the trial: it is granted from the first paid period,
			// so the charge the trial's end makes is the one it discounts first. Its
			// use is reserved now, against the card, as a sale charged today reserves
			// it: an org and a card each redeem a code once, trial or not.
			var hold *promo.Hold
			if cpn.Code != "" {
				h, herr := promo.Reserve(ctx, cpn.Code, org.Name, instrumentOf(pm), time.Now())
				if herr != nil {
					release()
					unvault()
					abandon()
					return nil, holdRefusal(cpn.Code, herr)
				}
				hold = h
			}
			if !pm.IsDefault {
				pm.IsDefault = true
				if err := pm.Update(); err != nil {
					log.Error("subscribe: failed to mark method %s default (subject=%s): %v", pm.Id(), in.Subject, err)
				}
			}
			meta := map[string]interface{}{"source": "subscribe/card"}
			if cpn.Code != "" {
				meta["coupon"] = cpn.Code
			}
			sub, err := createSubscription(db, p, &createSubscriptionRequest{
				UserId:               in.Subject,
				PlanId:               planID,
				StoreId:              in.StoreID,
				DefaultPaymentMethod: pm.Id(),
				Quantity:             qty,
				Metadata:             meta,
				Test:                 org.TestMode(),
				priced:               lines,
			})
			if err != nil {
				hold.Release()
				release()
				unvault()
				abandon()
				return nil, err
			}
			// Square-backed like a paid sale, so it is the subject's subscription
			// (billingSubscription) while it trials, and priced under the promo and
			// the coupon it was bought with when the trial ends.
			sub.ProviderType = string(processor.Square)
			sub.DiscountPercent, sub.DiscountName = promoPercent, promoName
			sub.CustomerCents = customer
			engine.Grant(sub, cpn, couponPlans, sub.PeriodStart)
			if err := sub.Update(); err != nil {
				log.Error("subscribe: failed to update trial subscription %s (subject=%s): %v", sub.Id(), in.Subject, err)
			}
			end := sub.TrialEnd
			sale := &Sale{
				SubscriptionID:  sub.Id(),
				PlanID:          planID,
				Level:           in.Level,
				Interval:        p.Interval,
				PaymentMethodID: pm.Id(),
				Seats:           seats,
				Category:        p.Category,
				Currency:        string(cur),
				Status:          "ok",
				TrialEnd:        &end,
				TrialEndCents:   chargeCents,
				Coupon:          cpn.Code,
				CouponPercent:   cpn.Percent,
				CouponMonths:    cpn.Months,
			}
			if rec != nil {
				if body, mErr := json.Marshal(sale); mErr == nil {
					_ = idempotencykey.Complete(rec, string(body))
				}
			}
			noticeSale(ctx, saleNote{
				Org: org.Name, Email: in.Email, Plan: p, Seats: seats, Sale: sale,
				ListCents: listCents, PromoName: promoName, Coupon: cpn, Test: org.TestMode(),
			})
			return &saleOutcome{Sale: sale, Sub: sub}, nil
		}
	}

	// The coupon's use is RESERVED before the card is charged, under the coupon's
	// lock: the redemption rows for this org and this card are written and the use
	// counted against the limit, so two buyers racing for the last use cannot both
	// be charged for it. A card that already redeemed the code is refused here,
	// once the card is known and before any money moves. A decline gives the use
	// back; once the charge settles it is spent, whatever happens after.
	var hold *promo.Hold
	if cpn.Code != "" {
		h, herr := promo.Reserve(ctx, cpn.Code, org.Name, instrumentOf(pm), time.Now())
		if herr != nil {
			unvault()
			abandon()
			return nil, holdRefusal(cpn.Code, herr)
		}
		hold = h
	}

	// Charge the SAVED card (card-on-file id + Square customer id) for the first
	// period at the server-authoritative price × seats, and the order's starting
	// balance with it — one payment — with the stable Square idempotency key.
	// All-or-nothing: on a decline no subscription is created and nothing is
	// credited.
	desc := fmt.Sprintf("Subscription %s — first period", p.Name)
	if bal > 0 {
		desc = fmt.Sprintf("Subscription %s — first period and starting usage balance", p.Name)
	}
	res, err := chargeSavedCard(ctx, reg, pm.ProviderRef, squareCustomerIDOf(pm), chargeCents+bal, cur, squareKey, desc)
	if err != nil || res == nil || !res.Success {
		hold.Release()
		unvault()
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

	// The payment is recorded before anything else is written: its order, with the
	// starting balance it owes, beside the receipt the sale will answer. From here
	// every step is keyed on the payment, so a sale that stops part-way is
	// finished by the same order sent again (draftOf), and a refund or a dispute
	// that arrives meanwhile is netted against the credit rather than lost.
	d := &draft{
		Payment: res.ProcessorRef,
		Sale: Sale{
			PlanID:          planID,
			Level:           in.Level,
			Interval:        p.Interval,
			PaymentMethodID: pm.Id(),
			AmountCents:     chargeCents + bal,
			BalanceCents:    bal,
			Seats:           seats,
			Category:        p.Category,
			Replaced:        replaced,
			Currency:        string(cur),
			Status:          "ok",
			Coupon:          cpn.Code,
			CouponPercent:   cpn.Percent,
			CouponMonths:    cpn.Months,
		},
		List:        listCents,
		Promo:       promoPercent,
		PromoName:   promoName,
		Customer:    customer,
		CouponPlans: couponPlans,
	}
	if err := enter(org, in, rec, d); err != nil {
		uncredited(ctx, in.Events, org.Name, in.Subject, d.Payment, "the charge settled and the sale could not record it: "+err.Error(), chargeCents+bal, false)
		return nil, unfinished
	}
	// No trial: the customer is paying now, so the first period is Active
	// immediately.
	p.TrialPeriodDays = 0
	return sell(ctx, db, org, in, rec, p, nil, qty, lines, d)
}

// priced is an order priced from the catalog: the plan at the level and period
// asked for, its currency, the team's other seat types, the seats, the starting
// balance and the first period's charge under the current promotion. The card
// sale and the invoice sale price an order through it, so both charge the same.
type priced struct {
	plan                     *plan.Plan
	slug                     string
	cur                      currency.Type
	lines                    []subscription.Line
	qty                      int
	balance, seatMult, seats int64
	charge                   int64
	promoPercent             int
	promoName                string
	// customer is the price staff agreed this subject pays for one seat of the
	// plan each period (customerprice); 0 when none was.
	customer int64
}

// price prices in's order, refusing everything the catalog does not sell, before
// anything is held or charged.
func price(db *datastore.Datastore, org *organization.Organization, in SubscribeIn, planID, sourceID string) (*priced, error) {
	// Server-authoritative plan + price. A client amount is NEVER consulted — the
	// charge is the plan's catalog price × billable seats, so a scripted request
	// cannot underpay.
	p, err := resolveSubscriptionPlan(db, planID)
	switch {
	case errors.Is(err, errPlanUnread):
		return nil, saleRefusal{saleUnavailable, err.Error()}
	case err != nil:
		return nil, saleRefusal{saleMissing, "plan not found"}
	}
	// Every rule below reads the catalog by slug. A plan named by its row id is
	// the same plan, so it is priced, seated and promoted as its slug is.
	if p.Slug != "" {
		planID = p.Slug
	}
	if !p.Listed() && p.Status != plan.StatusPrivate {
		// Refused BEFORE the card is charged — a retired tier must never take money.
		return nil, saleRefusal{saleMissing, "plan not found"}
	}
	if p.ContactSales {
		// A plan sold by our sales team is never bought by card, whatever price its
		// row states: its row may carry the seat price its card shows while it states
		// no seat terms, and a card sale of it would then charge one seat for the
		// whole plan.
		return nil, saleRefusal{saleRefused, fmt.Sprintf("plan %q is sold by our sales team, not by card", planID)}
	}
	// The price this card is charged, chosen from what the catalog publishes. A
	// level or a period the plan does not publish is refused here — before the card
	// is touched, before the idempotency guard is taken, and without ever
	// consulting an amount the client sent, because the client sends none.
	p, err = planAt(p, in.Level, in.Interval)
	if err != nil {
		return nil, saleRefusal{saleRefused, err.Error()}
	}
	if int64(p.Price) <= 0 {
		// A $0 plan needs no card — the free tier is self-serve via POST
		// /v1/billing/subscriptions. This endpoint's contract is a PAID card sub.
		return nil, saleRefusal{saleRefused,
			fmt.Sprintf("plan %q is free — no card charge required; use POST /v1/billing/subscriptions", planID)}
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
	// A team's other seat types, priced at the period bought, as the plan is. A
	// line naming a plan not on sale is the same miss as the plan itself.
	lines, err := linesAt(db, p, in.Lines, p.Interval, cur)
	switch {
	case errors.Is(err, errLinePlan):
		return nil, saleRefusal{saleMissing, "plan not found"}
	case errors.Is(err, errPlanUnread):
		return nil, saleRefusal{saleUnavailable, err.Error()}
	case err != nil:
		return nil, saleRefusal{saleRefused, err.Error()}
	}
	// Billable seats: a per-seat plan charges Price × quantity, and a team each of
	// its other seat types' price × its seats, the team floored at the catalog's
	// minSeats over every seat; a flat plan is always ×1. This MUST match
	// buildPeriodInvoice's seat math so the first charge equals the invoice's
	// AmountDue — never under-collect a per-seat plan yet record it paid in full.
	qty := in.Quantity
	if qty < 1 {
		qty = 1
	}
	// The seat terms are the row's in hand, read once: the row that prices the
	// sale is the row that seats it, and createSubscription stamps the same row on
	// the subscription every invoice bills from.
	terms := seatOf(p)
	if why := terms.refusal(planID, qty, lines); why != "" {
		return nil, coded(saleRefused, "seats_out_of_range", why)
	}
	if code, why := orderRefusal(org, p, planID, in, sourceID); why != "" {
		return nil, coded(saleRefused, code, why)
	}
	bal := in.BalanceCents
	seatMult := int64(1)
	if terms.each {
		seatMult = int64(qty)
	}
	// The seats the receipt states, every seat type counted.
	seats := seatMult + int64(lineSeats(lines))
	chargeCents := int64(p.Price)*seatMult + lineCents(lines)
	// A price staff agreed for this customer and plan replaces the list price of
	// the plan's own seats, today and at every renewal; no promotion applies
	// beside it, since it is the price this customer pays.
	customer, err := agreedPrice(db, in.Subject, planID, p)
	if err != nil {
		return nil, saleRefusal{saleUnavailable, "the price agreed for this account could not be read; nothing was charged, try again"}
	}
	// A private plan is made for one customer: it is sold by card only to a
	// subject staff agreed a price of it with, and is no plan to anyone else.
	if !p.Listed() && customer == 0 {
		return nil, saleRefusal{saleMissing, "plan not found"}
	}
	if customer > 0 {
		chargeCents = customer*seatMult + lineCents(lines)
	}
	// The advertised promo, actually applied. GET /v1/billing/plans annotates each
	// plan with promoPercent off the SAME gate (active + inside its window +
	// AppliesTo), and until now nothing subtracted it here: the page could say
	// "50% off" while this line charged full price and the receipt agreed with the
	// charge, not the offer. Resolved once, carried on the subscription, so every
	// renewal prices identically — and computed through engine.Discounted, the
	// same function that discounts the invoice, so charge == AmountDue by
	// construction rather than by two implementations agreeing.
	promoPercent, promoName := 0, ""
	if pr := in.Promo; customer == 0 && pr != nil && everyLine(planID, lines, pr.AppliesTo) {
		promoPercent, promoName = pr.PercentOff, pr.Name()
		chargeCents = engine.Discounted(chargeCents, promoPercent)
	}
	if chargeCents <= 0 {
		// A 100%-off promo leaves nothing to charge, and a card sale of $0 is not a
		// sale — Square rejects a zero charge, so this would surface as a decline.
		// Refuse BEFORE the card is touched and name the real reason.
		return nil, saleRefusal{saleRefused,
			fmt.Sprintf("plan %q costs nothing after the current promotion — no card charge required; use POST /v1/billing/subscriptions", planID)}
	}
	return &priced{
		plan: p, slug: planID, cur: cur, lines: lines, qty: qty,
		balance: bal, seatMult: seatMult, seats: seats, charge: chargeCents,
		promoPercent: promoPercent, promoName: promoName, customer: customer,
	}, nil
}

// draft is what a card sale has decided by the time its charge settles: the
// payment, and the receipt it answers with less the rows not written yet. It is
// marked on the sale's guard then (enter), so a retry that re-claims the guard of a
// sale that stopped part-way finishes that sale from here rather than charging
// again or refusing the plan just paid for.
type draft struct {
	Payment     string   `json:"payment"`
	Sale        Sale     `json:"sale"`
	List        int64    `json:"list"`
	Promo       int      `json:"promo,omitempty"`
	PromoName   string   `json:"promoName,omitempty"`
	Customer    int64    `json:"customer,omitempty"`
	CouponPlans []string `json:"couponPlans,omitempty"`
}

// draftOf is the draft a stopped sale marked on its guard, or nil.
func draftOf(rec *idempotencykey.IdempotencyKey) *draft {
	if rec == nil || rec.RecoveryPoint == "" {
		return nil
	}
	d := new(draft)
	if json.Unmarshal([]byte(rec.RecoveryPoint), d) != nil || d.Payment == "" {
		return nil
	}
	return d
}

// enter records a settled charge before the sale writes anything else: the
// payment's record (models/paymentorg) with the order's starting balance, owed,
// and the draft on the sale's guard.
func enter(org *organization.Organization, in SubscribeIn, rec *idempotencykey.IdempotencyKey, d *draft) error {
	order := ""
	if rec != nil {
		order = idempotencykey.DeterministicID(rec.Scope, rec.IdemKey)
	}
	if err := paymentorg.Open(d.Payment, org.Name, in.Subject, order, string(processor.Square), d.Sale.BalanceCents, org.TestMode()); err != nil {
		return err
	}
	if rec == nil {
		return nil
	}
	body, err := json.Marshal(d)
	if err != nil {
		return err
	}
	return idempotencykey.Mark(rec, string(body))
}

// unfinished is a card sale whose charge settled and whose rows were not all
// written: the same order sent again finishes it, charging nothing again.
var unfinished = saleRefusal{saleUnavailable, "the payment settled and the order is not finished yet; send the same order again in a few minutes to finish it — nothing is charged again"}

// owedBalance is an order whose starting balance its payment still owes the
// wallet: the same order sent again credits it, charging nothing again.
var owedBalance = saleRefusal{saleUnavailable, "the payment settled and its starting balance is not credited yet; send the same order again to credit it — nothing is charged again"}

// opened is the subscription the payment opened, or nil when it opened none. A
// read that fails is an error, never "none": answering none would open a second.
func opened(db *datastore.Datastore, subject string, test bool, payment string) (*subscription.Subscription, error) {
	subs, err := userSubscriptions(db, subject, test)
	if err != nil {
		return nil, err
	}
	for _, s := range subs {
		if s.Metadata["payment"] == payment && s.Metadata["bundleParent"] == nil &&
			!strings.EqualFold(strings.TrimSpace(s.ProviderType), "bundle") {
			// A row read by query is not bound to the store, so the sale's writes go
			// through a fresh read of the same id.
			row := subscription.New(db)
			if err := row.GetById(s.Id()); err != nil {
				return nil, err
			}
			return row, nil
		}
	}
	return nil, nil
}

// sell writes what a settled card charge bought, each step keyed on the payment so
// a sale run again after it stopped finishes rather than repeats: the subscription
// (sub, or a new one), the period it paid, the payment's record of both, the
// receipt, and then the starting balance (conclude). Every write that fails leaves
// the guard started, holding the draft, for the retry to finish.
func sell(ctx context.Context, db *datastore.Datastore, org *organization.Organization, in SubscribeIn, rec *idempotencykey.IdempotencyKey, p *plan.Plan, sub *subscription.Subscription, qty int, lines []subscription.Line, d *draft) (*saleOutcome, error) {
	sale := d.Sale
	cpn := engine.Coupon{Code: sale.Coupon, Percent: sale.CouponPercent, Months: sale.CouponMonths}
	if sub == nil {
		// Created because the payment settled (mint authority = the settled
		// charge — the CreateBillingSubscription C1-a gate is deliberately not on
		// this path).
		meta := map[string]interface{}{"source": "subscribe/card", "payment": d.Payment}
		if cpn.Code != "" {
			meta["coupon"] = cpn.Code
		}
		var err error
		sub, err = createSubscription(db, p, &createSubscriptionRequest{
			UserId:               in.Subject,
			PlanId:               sale.PlanID,
			StoreId:              in.StoreID,
			DefaultPaymentMethod: sale.PaymentMethodID,
			Quantity:             qty,
			Metadata:             meta,
			Test:                 org.TestMode(),
			priced:               lines,
		})
		if err != nil {
			uncredited(ctx, in.Events, org.Name, in.Subject, d.Payment,
				"the first-period charge settled and no subscription was created: "+err.Error(), sale.AmountCents, false)
			return nil, err
		}
	}

	// Mark the FIRST period PAID by the card charge — a paid BillingInvoice
	// referencing the payment — and keep the row on that period. This also makes
	// the subscription payment-backed (Square provider + a linked invoice), so its
	// INCLUDED monthly allotment flows (subscriptionPaymentBacked). The promo and
	// the coupon are stamped BEFORE the invoice is built — buildPeriodInvoice
	// prices both off the subscription, so setting them after would invoice the
	// first period at full price against a discounted charge.
	sub.ProviderType = string(processor.Square)
	sub.DiscountPercent, sub.DiscountName = d.Promo, d.PromoName
	sub.CustomerCents = d.Customer
	engine.Grant(sub, cpn, d.CouponPlans, sub.PeriodStart)
	inv, err := paidBy(db, sub, d.Payment)
	if err == nil {
		_, _, err = paymentorg.Change(d.Payment, func(r *paymentorg.PaymentOrg) bool {
			if r.Invoice == inv.Id() {
				return false
			}
			r.Invoice = inv.Id()
			return true
		})
	}
	if err != nil {
		uncredited(ctx, in.Events, org.Name, in.Subject, d.Payment,
			"the first-period charge settled and its paid invoice was not recorded: "+err.Error(), sale.AmountCents, false)
		return nil, unfinished
	}
	if err := sub.Update(); err != nil {
		log.Error("subscribe: failed to update subscription after first invoice (subject=%s): %v", in.Subject, err)
	}

	sale.SubscriptionID, sale.InvoiceID = sub.Id(), inv.Id()
	if sale.BalanceCents == 0 {
		// Nothing is owed: a plan whose row states a reload switches the buyer's
		// auto-recharge on now the first period is paid, before the receipt is
		// sealed so a retry replays it.
		sale.Reload = armSale(ctx, org, in, sale.PlanID, currency.Type(sale.Currency))
	}
	// Seal the guard with the exact success body so a retry replays it verbatim (no
	// second vault/charge/subscribe, identical response). An order's starting
	// balance is credited LAST, after this seal names it owed (no balanceId): a
	// credit that fails leaves the receipt owing it, and the same order sent again
	// completes it (complete), once.
	sealSale(rec, &sale)
	noticeSale(ctx, saleNote{
		Org: org.Name, Email: in.Email, Plan: p, Seats: sale.Seats, Sale: &sale,
		ListCents: d.List, PromoName: d.PromoName, Coupon: cpn, Test: org.TestMode(),
	})
	if sale.Replaced != "" && sale.Replaced != sub.Id() {
		was := subscription.New(db)
		if err := was.GetById(sale.Replaced); err == nil {
			_ = engine.CancelSubscription(was, false)
			_ = was.Update()
			if slug := subscriptionSlug(was); slug != sale.PlanID {
				disarm(ctx, org, in.Subject, slug)
			}
		}
	}
	if sale.BalanceCents == 0 {
		return &saleOutcome{Sale: &sale, Sub: sub, Inv: inv}, nil
	}
	return conclude(ctx, db, org, in, rec, &sale, d.Payment, sub, inv)
}

// paidBy is the paid first-period invoice payment paid for sub: the one already
// recorded, or a new one.
func paidBy(db *datastore.Datastore, sub *subscription.Subscription, payment string) (*billinginvoice.BillingInvoice, error) {
	inv := billinginvoice.New(db)
	found, err := inv.Query().Filter("PaymentRef=", payment).Filter("SubscriptionId=", sub.Id()).Get()
	switch {
	case err != nil:
		return nil, err
	case found:
		return inv, nil
	}
	return engine.CreatePaidFirstInvoice(db, sub, "card", payment)
}

// saleScope and saleKey name the one lock a subject's sale holds while it is in
// flight (subscribe): one key per subject, whatever plan or retry key it is for.
const (
	saleScope = "billing-sale:"
	saleKey   = "sale"
)

// sealSale seals the guard with a sale's receipt, so a retry under its key
// replays it.
func sealSale(rec *idempotencykey.IdempotencyKey, sale *Sale) {
	if rec == nil {
		return
	}
	if body, err := json.Marshal(sale); err == nil {
		_ = idempotencykey.Complete(rec, string(body))
	}
}

// owing reports whether a sealed receipt still owes its starting balance.
func (s *Sale) owing() bool {
	return s.BalanceCents > 0 && s.BalanceID == ""
}

// factsOf is the digest of the order a request sends — every field that says what
// is bought, as sent: the store, the plan, its level and term, the seats of every
// seat type, the starting balance, the coupon and how it is paid. Never the card
// token, which a retry mints afresh.
func factsOf(in SubscribeIn) string {
	h := sha256.New()
	fmt.Fprintf(h, "store=%s\nplan=%s\nlevel=%d\ninterval=%s\nquantity=%d\nbalance=%d\ncoupon=%s\nwallet=%t\n",
		strings.TrimSpace(in.StoreID), strings.TrimSpace(in.PlanID), in.Level, in.Interval, max(in.Quantity, 1),
		in.BalanceCents, promo.CouponCode(strings.TrimSpace(in.Coupon)), in.Wallet)
	for _, l := range in.Lines {
		fmt.Fprintf(h, "line=%s:%d\n", strings.TrimSpace(l.PlanID), l.Quantity)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// owedSale finishes an order whose starting balance is still owed when this
// request is that order: the same facts, under any key. It is asked before the
// catalog is read, so neither a new key nor an edit of the plan since strands the
// money the order paid. Only an order buys a balance, so a request that buys none
// asks nothing.
func owedSale(ctx context.Context, db *datastore.Datastore, org *organization.Organization, in SubscribeIn, facts string) (*saleOutcome, bool, error) {
	if in.BalanceCents <= 0 {
		return nil, false, nil
	}
	orders, err := paymentorg.Orders(org.Name, in.Subject)
	if err != nil {
		return nil, true, saleRefusal{saleUnavailable, "billing could not be reached; nothing was charged, try again"}
	}
	for _, payment := range orders {
		po, found, err := paymentorg.Get(payment)
		if err != nil || !found {
			continue
		}
		rec := idempotencykey.New(db)
		if rec.Get(db.NewKey(rec.Kind(), po.Order, 0, nil)) != nil || rec.Facts != facts ||
			rec.Status != idempotencykey.StatusCompleted {
			continue
		}
		var sale Sale
		if json.Unmarshal([]byte(rec.Response), &sale) != nil || !sale.owing() {
			continue
		}
		out, err := complete(ctx, db, org, in, rec, &sale, payment)
		return out, true, err
	}
	return nil, false, nil
}

// complete finishes a sale whose starting balance was owed when its receipt was
// sealed: it credits what the payment's record says is owed now (endow), switches
// the plan's reload on, seals the receipt with the credit, and answers it as the
// sale — the first answer was the failure. It holds the subject's sale lease, so it
// never runs beside the sale it completes.
func complete(ctx context.Context, db *datastore.Datastore, org *organization.Organization, in SubscribeIn, rec *idempotencykey.IdempotencyKey, sale *Sale, payment string) (*saleOutcome, error) {
	lease, err := holdSale(db, in.Subject)
	if err != nil {
		return nil, err
	}
	defer lease.Release()
	var sub *subscription.Subscription
	if s := subscription.New(db); s.GetById(sale.SubscriptionID) == nil {
		sub = s
	}
	var inv *billinginvoice.BillingInvoice
	if i := billinginvoice.New(db); i.GetById(sale.InvoiceID) == nil {
		inv = i
	}
	return conclude(ctx, db, org, in, rec, sale, payment, sub, inv)
}

// conclude credits an order's starting balance as its payment's record now stands
// (endow), then switches the plan's reload on — only once the balance is credited,
// and only for a payment no refund or dispute took back — and seals the receipt
// with the credit: the amount that reached the wallet and its receipt. A balance
// refunds and disputes took in full is credited nothing, and the receipt then
// owes nothing.
func conclude(ctx context.Context, db *datastore.Datastore, org *organization.Organization, in SubscribeIn, rec *idempotencykey.IdempotencyKey, sale *Sale, payment string, sub *subscription.Subscription, inv *billinginvoice.BillingInvoice) (*saleOutcome, error) {
	cur := currency.Type(sale.Currency)
	po, err := endow(ctx, in.Events, org, db, cur, payment)
	if err != nil {
		return nil, err
	}
	sale.BalanceCents, sale.BalanceID = po.Credit, po.Deposit
	if po.Taken() == 0 {
		sale.Reload = armSale(ctx, org, in, sale.PlanID, cur)
	}
	sealSale(rec, sale)
	return &saleOutcome{Sale: sale, Sub: sub, Inv: inv}, nil
}

// paymentScope names the lease a payment is changed under: the credit of an
// order's starting balance, and every refund and dispute of the payment, one at a
// time.
const paymentScope = "billing-payment:"

// paymentWait is how long a refund or a dispute of a payment waits for another
// change of it to finish: each holds the lease for one read and a few writes, so
// deliveries that arrive together are applied one after the other, not refused. A
// sale crediting the payment does not wait: its buyer is answered at once, and
// sends the order again.
const paymentWait = 3 * time.Second

// holdPayment takes payment's lease, waiting up to paymentWait for its holder;
// held=false means it was still held then.
func holdPayment(db *datastore.Datastore, payment string) (*idempotencykey.Lease, bool, error) {
	return hold(db, paymentScope, payment)
}

// hold takes the lease on (scope, key), waiting up to paymentWait for its holder.
func hold(db *datastore.Datastore, scope, key string) (*idempotencykey.Lease, bool, error) {
	for wait := time.Duration(0); ; wait += 25 * time.Millisecond {
		lease, held, err := idempotencykey.Hold(db, scope, key)
		if err != nil || held || wait >= paymentWait {
			return lease, held, err
		}
		time.Sleep(25 * time.Millisecond)
	}
}

// endow credits an order's starting balance to its wallet as the payment's record
// stands, under the payment's lease so no refund or dispute of it is applied
// beside. The credit is decided once (paymentorg.PaymentOrg.Decide): the balance
// less what refunds took — final, so netted — with every dispute's standing take
// moved to the wallet, to be taken from it once the credit lands, so a dispute won
// later is given back through the ledger that took it, once. A refund is either
// netted in the credit or taken after it (takeOrder), never neither. The decided
// credit then lands (land). A credit already decided is landed again, which
// credits nothing twice, and one refunds took in full credits nothing.
func endow(ctx context.Context, ev *events.Client, org *organization.Organization, db *datastore.Datastore, cur currency.Type, payment string) (*paymentorg.PaymentOrg, error) {
	lease, held, err := idempotencykey.Hold(db, paymentScope, payment)
	switch {
	case err != nil:
		return nil, owedBalance
	case !held:
		return nil, saleRefusal{saleHeld, "a refund or a dispute of this payment is being recorded; send the order again in a moment"}
	}
	defer lease.Release()
	po, found, err := paymentorg.Change(payment, func(r *paymentorg.PaymentOrg) bool {
		return r.Decide()
	})
	switch {
	case err != nil:
		return nil, owedBalance
	case !found:
		uncredited(ctx, ev, org.Name, "", payment, "the order's payment is not on record, so its starting balance was not credited", 0, true)
		return nil, saleRefusal{saleUnsettled, "the order's payment is not on record, so its starting balance was not credited"}
	}
	return land(ctx, ev, org, db, cur, po, payment)
}

// land puts an order's decided credit into both books, once: the deposit in
// commerce's own (fund) and the ledger the spend gate reads (bank), both keyed on
// the payment, then records the credit's receipt on the payment's record. Then
// every take moved to the wallet and not given back is taken from it, once each
// (clawback). org is the payer's, and only read to credit. The caller holds the
// payment's lease.
func land(ctx context.Context, ev *events.Client, org *organization.Organization, db *datastore.Datastore, cur currency.Type, po *paymentorg.PaymentOrg, payment string) (*paymentorg.PaymentOrg, error) {
	if po.Credit > 0 && po.Deposit == "" {
		id, _, err := fund(ctx, ev, org, db, po.Subject, po.Credit, cur, processor.ProcessorType(cmp.Or(po.Via, string(processor.Square))), payment)
		if err != nil {
			return nil, owedBalance
		}
		if err := bank(ctx, org, po.Subject, cur, receipt{ProcessorRef: payment, AmountCents: po.Credit}, "starting balance"); err != nil {
			uncredited(ctx, ev, org.Name, po.Subject, payment, "the order's starting balance was not credited to the spendable balance: "+err.Error(), po.Credit, false)
			return nil, owedBalance
		}
		if po, _, err = paymentorg.Change(payment, func(r *paymentorg.PaymentOrg) bool {
			r.Deposit = id
			return true
		}); err != nil {
			return nil, owedBalance
		}
	}
	for _, t := range po.Takes {
		if !t.Moved || t.Restored || t.Cents <= 0 {
			continue
		}
		in := creditledger.ClawbackInput{
			Org: po.Org, Subject: po.Subject, Payment: payment,
			Ref: t.Ref, AmountCents: t.Cents, Reason: t.Ref, Test: po.Test,
		}
		if err := clawback(ctx, db, in, cur, false); err != nil {
			log.Error("RECONCILE: payment %s: %v", payment, err)
			return nil, owedBalance
		}
	}
	return po, nil
}

// couponRefusal is the sentence a buyer reads when the coupon they sent cannot
// be used for this plan, by the reason the plans quote names.
func couponRefusal(code, slug, reason string) string {
	switch reason {
	case promo.CouponExpired:
		return fmt.Sprintf("coupon %s has expired", code)
	case promo.CouponExhausted:
		return fmt.Sprintf("coupon %s has been fully redeemed", code)
	case promo.CouponNotApplicable:
		return fmt.Sprintf("coupon %s does not apply to plan %q", code, slug)
	}
	return fmt.Sprintf("coupon %s is not valid", code)
}

// orderRefusal is why a sale breaks the order terms the plan's row states, and its
// code ("org_required" or "balance_out_of_range"), or "" when it keeps them.
//
// A row whose limits state how an organization orders it (plan.Limits Ordered) is
// an organization's plan, so it is bought for the organization's own account and
// never for a member's wallet. The endpoint resolved Subject from the caller's own
// credential, and only an admin of the organization (or the platform) resolves to
// its account, so this is where an order from anyone else is refused, before
// anything is held or charged.
//
// A starting balance is whole dollars, no more than the row's balanceCeiling, and
// bought with a card in the payment that charges today: not from prepaid money,
// not by a wallet token (kept nowhere, so there is no card to stand behind the
// credit), and not on a sale that opens in a free trial and charges nothing. A row
// that states no ceiling takes none.
func orderRefusal(org *organization.Organization, p *plan.Plan, slug string, in SubscribeIn, source string) (code, why string) {
	l := p.Limits
	if l.Ordered() && !strings.EqualFold(in.Subject, org.Name) {
		return "org_required", fmt.Sprintf("plan %q is ordered by an organization: an admin of %s buys it for the organization", slug, org.Name)
	}
	const balance = "balance_out_of_range"
	bal := in.BalanceCents
	switch {
	case bal == 0:
		return "", ""
	case bal < 0 || bal%100 != 0:
		return balance, "balanceCents is a whole number of dollars, 0 or more"
	case l == nil || l.BalanceCeiling == nil || *l.BalanceCeiling <= 0:
		return balance, fmt.Sprintf("plan %q takes no starting balance", slug)
	case bal > int64(*l.BalanceCeiling)*100:
		return balance, fmt.Sprintf("plan %q takes a starting balance of at most $%d (got $%d)", slug, *l.BalanceCeiling, bal/100)
	case in.Wallet:
		return balance, "a starting balance is bought with a card: send sourceId or paymentMethodId"
	case p.TrialPeriodDays > 0:
		return balance, fmt.Sprintf("plan %q opens in a free trial that charges nothing today, so it takes no starting balance", slug)
	}
	return "", ""
}

// holdRefusal is a coupon reservation's refusal as a sale refusal: the coupon
// no longer standing is the request's 400, an org or card that already redeemed
// it, or another code of its campaign, is a 409, and a store that could not
// record the use is the sale failing.
func holdRefusal(code string, err error) error {
	var cr promo.CouponRefusal
	switch {
	case errors.As(err, &cr):
		return saleRefusal{saleRefused, couponRefusal(code, "", cr.Reason)}
	case errors.Is(err, promo.ErrCouponRedeemedByOrg), errors.Is(err, promo.ErrCouponRedeemedByCard), errors.Is(err, promo.ErrCouponCampaignRedeemed):
		return saleRefusal{saleHeld, fmt.Sprintf("%v (%s)", err, code)}
	}
	return fmt.Errorf("coupon %s: the use was not recorded, so the card was not charged: %w", code, err)
}

// instrumentOf is a saved card's identity across orgs and vaults, for the
// one-use-per-card rule: the processor's fingerprint of the card number, else
// its brand, last four and expiry, else the vaulted card itself.
func instrumentOf(pm *paymentmethod.PaymentMethod) string {
	if fp := metaStr(pm, "fingerprint"); fp != "" {
		return "fp:" + fp
	}
	if c := pm.Card; c != nil && c.Brand != "" && c.Last4 != "" {
		return fmt.Sprintf("card:%s:%s:%d/%d", strings.ToLower(c.Brand), c.Last4, c.ExpMonth, c.ExpYear)
	}
	return "ref:" + strings.TrimSpace(pm.ProviderRef)
}

// heldBefore reports whether subject has held plan slug before, in any status,
// among the rows that speak in this mode (speaks): a subject that has, has had
// the plan's trial or paid for the plan, and is not offered the trial again.
func heldBefore(db *datastore.Datastore, slug, subject string, test bool) (bool, error) {
	subs := make([]*subscription.Subscription, 0)
	if _, err := subscription.Query(db).Filter("UserId=", subject).GetAll(&subs); err != nil {
		return false, err
	}
	for _, s := range subs {
		if speaks(s, test) && subscriptionSlug(s) == slug {
			return true, nil
		}
	}
	return false, nil
}

// SubscribeWithCard vaults a Square card nonce as a reusable card-on-file, charges
// it for the plan's FIRST period at the SERVER-AUTHORITATIVE catalog price, and
// creates the subscription — all server-side, in one transaction of intent.
//
//	POST /v1/billing/subscribe/card
//
// Body: { sourceId | paymentMethodId, planId, userId?, quantity?, lines?, level?, interval?, currency?, coupon?, balanceCents? }
// — lines are a team's other seat types, [{planId, quantity}]: a team of one
// standard and one premium seat is planId "team", quantity 1, lines
// [{"planId":"team-premium","quantity":1}], charged both seats' prices
// — NO client price: the price is the plan's catalog price, always. coupon is a
// plan coupon code for a percent off the first months it names, monthly only.
// balanceCents is an organization's starting usage balance, charged in the same
// payment and credited to its prepaid balance: whole dollars, at most the row's
// limits.balanceCeiling (orderRefusal).
// Header (optional): X-Idempotency-Key — a retry/double-submit with the same key
// (or, absent a key, the same store/plan/level inside a window) never
// double-charges: it replays the first result verbatim.
// Returns: { subscriptionId, invoiceId, planId, level, interval, paymentMethodId, amountCents, balanceCents?, balanceId?, seats, category, replaced?, currency, status, coupon?, couponPercent?, couponMonths?, trialEnd?, trialEndCents? }
//
// A plan with a free trial (trialPeriodDays) bought by a buyer and a card that
// have not had it charges nothing today: amountCents is 0, invoiceId is empty,
// and trialEnd and trialEndCents say when the card is charged and for how much.
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

	if !sold(c) {
		return http.Fail(c, 404, "plan not found", nil)
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
	// A member subscribes, and pays from, their own account whatever userId names.
	// The org's own account, and another member's, are the org's to subscribe: its
	// admins, a platform SuperAdmin or a service.
	if !mayActForOrg(c) {
		own := userBillingKey(c)
		if own == "" || own == orgBillingKey(c) {
			return http.Fail(c, 403, "only an admin of this organization may subscribe it", nil)
		}
		subject = own
	}

	iamEmail, _ := c.Locals("iam_email").(string)
	out, err := subscribe(c.Context(), org, SubscribeIn{
		SourceID:       req.SourceID,
		MethodID:       req.MethodID,
		PlanID:         req.PlanID,
		Subject:        subject,
		StoreID:        req.StoreID,
		Quantity:       req.Quantity,
		Lines:          req.Lines,
		Level:          req.Level,
		Interval:       req.Interval,
		Currency:       req.Currency,
		Email:          iamEmail,
		Coupon:         req.Coupon,
		Wallet:         req.Wallet,
		Provider:       req.Provider,
		IdempotencyKey: strings.TrimSpace(c.Header("X-Idempotency-Key")),
		Promo:          promo.Active(c),
		Events:         eventsOf(c),
		Delegated:      subject != userBillingKey(c),
		BalanceCents:   req.BalanceCents,
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
		case IsSaleUnavailable(err):
			return http.Fail(c, 503, err.Error(), nil)
		case IsSaleUnsettled(err):
			return http.Fail(c, 502, err.Error(), nil)
		case IsMethodNotFound(err):
			return http.Fail(c, 404, "payment method not found", err)
		case IsMethodUnchargeable(err):
			return http.Fail(c, 422, "saved payment method has no chargeable card — add the card again", nil)
		case IsTopupUncredited(err):
			return http.Fail(c, 500, "the payment settled and its starting balance was not credited; contact support", err)
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
// processor (processorsForOrg), returning the processor ref. A resolution/charge
// failure returns an error → CollectInvoice
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
		if err := paymentorg.Put(res.ProcessorRef, org.Name, inv.UserId, inv.Id(), 0, org.TestMode()); err != nil {
			log.Warn("renewal: payment %s: the index to invoice %s was not written, so its refund or dispute will not find the period it paid: %v", res.ProcessorRef, inv.Id(), err)
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

// holdSale takes subject's sale lease: one sale, completion or staff record for a
// subject at a time, whatever key it came under (saleScope).
func holdSale(db *datastore.Datastore, subject string) (*idempotencykey.Lease, error) {
	lease, held, err := idempotencykey.Hold(db, saleScope+subject, saleKey)
	switch {
	case err != nil:
		log.Error("subscribe: the sale lease could not be taken (subject=%s): %v", subject, err)
		return nil, saleRefusal{saleUnavailable, "billing could not be reached; nothing was charged, try again"}
	case !held:
		return nil, coded(saleHeld, "order_placing", "a sale for this account is already in progress")
	}
	return lease, nil
}
