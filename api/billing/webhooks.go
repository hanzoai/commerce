package billing

import (
	"context"
	"math"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/billing/creditledger"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/mintauth"
	"github.com/hanzoai/commerce/models/billingevent"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/transaction"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/secret"

	// Blank-import the provider barrel so every provider's init() registers
	// with processor.Global() before HandleProviderWebhook runs. Without this
	// the global registry is empty and tryValidateWebhook can never reach any
	// provider's ValidateWebhook (the per-org payment.ProcessorsForOrg path is
	// separate and unaffected). This is the single owning import for the
	// generic webhook dispatcher; do not scatter barrel imports elsewhere.
	_ "github.com/hanzoai/commerce/payment/providers"
	jsonhttp "github.com/hanzoai/commerce/util/json/http"

	. "github.com/hanzoai/commerce/types"
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
func HandleProviderWebhook(c *zip.Ctx) error {
	providerHint := strings.ToLower(strings.TrimSpace(c.Param("provider")))
	payload := c.Body()

	// Every provider puts its signature in a different header; let the router
	// try each processor with the one most likely to match.
	reqHeaders := http.Header{}
	for k, vals := range c.Fiber().GetReqHeaders() {
		for _, v := range vals {
			reqHeaders.Add(k, v)
		}
	}
	signature := pickSignatureHeader(reqHeaders, providerHint)
	if signature == "" {
		return jsonhttp.Fail(c, http.StatusBadRequest, "missing webhook signature header", nil)
	}

	ctx, cancel := context.WithTimeout(c.Context(), 15*time.Second)
	defer cancel()

	event, err := tryValidateWebhook(ctx, providerHint, payload, signature)
	if err != nil || event == nil {
		log.Warn("webhook signature validation failed (provider hint=%s): %v", providerHint, err)
		return jsonhttp.Fail(c, http.StatusUnauthorized, "invalid webhook signature", err)
	}

	// Persist the raw event so the app has an audit trail independent of
	// processor-side retention. Webhooks arrive with no session, so resolve
	// the owning org the same way service-token calls do.
	org := resolveWebhookOrg(c)
	if org == nil {
		return jsonhttp.Fail(c, http.StatusServiceUnavailable, "organization context unavailable", nil)
	}
	db := datastore.New(org.Namespaced(c.Context()))

	// Idempotency: providers retry aggressively (Square retries for up to
	// 72h until it gets a 2xx). If we already recorded this event ID, ack
	// without re-applying side effects.
	if event.ID != "" && eventAlreadyProcessed(db, providerHint, event.ID) {
		return c.JSON(http.StatusOK, map[string]any{
			"received":  true,
			"type":      event.Type,
			"id":        event.ID,
			"duplicate": true,
		})
	}

	evt := billingevent.New(db)
	evt.Type = event.Type
	evt.ObjectType = providerHint
	evt.ObjectId = event.ID
	evt.Livemode = !org.TestMode() // one authority for the books and the record of them: the org
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

	// Settle funds for completed payments. This is the path Square's
	// payment.updated(status=COMPLETED) callback exercises once a card
	// authorization captures — the moment funds are guaranteed.
	if isSettlementEvent(event.Type) {
		applySettlementEvent(ctx, db, org, event)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"received": true,
		"type":     event.Type,
		"id":       event.ID,
	})
}

// resolveWebhookOrg determines which org a sessionless provider webhook
// belongs to. Order: X-Org-Id header, COMMERCE_SERVICE_ORG env, then the
// default "hanzo" org. Mirrors the service-token resolution in
// middleware/accesstoken.go so webhooks and service calls agree on scope.
func resolveWebhookOrg(c *zip.Ctx) *organization.Organization {
	// GetOrganizationOK, not GetOrganization: webhook ingress runs OUTSIDE the
	// auth-token group, so no middleware ever set "organization" — the MustGet
	// variant panics (500) on every sessionless delivery, exactly the case this
	// resolver exists to handle.
	if org, ok := middleware.GetOrganizationOK(c); ok && org != nil {
		return org
	}
	db := datastore.New(c.Context())
	orgName := strings.TrimSpace(c.Header("X-Org-Id"))
	if orgName == "" {
		orgName = strings.TrimSpace(os.Getenv("COMMERCE_SERVICE_ORG"))
	}
	if orgName == "" {
		orgName = "hanzo"
	}
	// A webhook caller must not be able to provision an org named after a raw
	// API key: reject a bearer-shaped selector before GetOrCreate (incident
	// 2026-07-02).
	if secret.Like(orgName) {
		log.Warn("webhook org resolve: bearer-shaped org selector; refusing to provision")
		return nil
	}
	org := organization.New(db)
	org.Name = orgName
	if err := org.GetOrCreate("Name=", orgName); err != nil {
		log.Warn("webhook org resolve for '%s' failed: %v", orgName, err)
		return nil
	}
	// THE ORG RECORD SAYS WHICH BOOKS, and this used to overwrite it.
	//
	// The line here was `org.Live = true`, justified by a deployment-wide
	// SQUARE_ENVIRONMENT that overrode the flag — so forcing it looked free. That
	// override is GONE: organization.TestMode is now `!o.Live` and nothing else, and
	// its own note says the authority is the org record "and nothing else". So the
	// forcing stopped being a no-op and became the whole answer: every settlement,
	// including a sandbox merchant's, was stamped live and credited the spendable
	// books. A Square SANDBOX callback — signed with the sandbox key, funding nothing
	// — minted balance that buys real inference.
	//
	// Loading the org and believing it is also what fails CLOSED, per that same note:
	// an org that does not say Live transacts in sandbox, so a new or half-configured
	// tenant credits the test books rather than the spendable ones. The wrong
	// direction here is money a customer has to ask for; the other direction is money
	// nobody paid for.
	return org
}

// eventAlreadyProcessed reports whether a billing event with this provider
// object type and ID was already recorded — the idempotency guard against
// provider retries.
func eventAlreadyProcessed(db *datastore.Datastore, objectType, eventID string) bool {
	existing := billingevent.New(db)
	found, err := existing.Query().
		Filter("ObjectType=", objectType).
		Filter("ObjectId=", eventID).
		Get()
	return err == nil && found
}

// isSettlementEvent reports whether an event represents settled (captured)
// funds that should credit a balance.
func isSettlementEvent(eventType string) bool {
	switch eventType {
	case "payment.completed", "payment.updated", "invoice.paid":
		return true
	default:
		return false
	}
}

// applySettlementEvent credits the balance a settled payment funds — in the ONE
// ledger the spend gate reads, at the address that gate will debit.
//
// # The money that arrived and never landed
//
// This callback used to write a row into commerce's OWN transaction store and stop
// there. Nothing in the embedded binary spends from that store: the prepaid balance
// every gate consults is the host's finance ledger, reached through [creditledger].
// So an asynchronously-settled payment moved real money, wrote a row, and the balance
// that decides whether a request is served never changed. The in-session doors close
// that half themselves (the host's settle.go, at the point their handler returns);
// this is the same half for the settlements that have no door.
//
// # WHO PAID IS READ FROM WHAT WE WROTE, NEVER FROM THE CALLBACK
//
// Nothing on a provider's payment object names a wallet in our books, and the code
// that believed otherwise is what this replaces. `reference_id` is OUR field, but the
// only thing that ever sets it is checkout, which sets it to an ORDER id
// (api/checkout/square/authorize.go); `customer_id` is the PROVIDER's own customer,
// which is not an identity in this system at all. Both were being lowercased into
// DestinationId under DestinationKind "iam-user" — the gateway-spendable wallet — so
// a settled payment minted balance at an address that was an order id, a Square
// customer id, or nobody, and did it behind a 200.
//
// So the payer is resolved by looking the PAYMENT ID up in the records commerce
// itself wrote when it took the money ([settlementDestination]). Everything that cannot be
// resolved that way is REFUSED and reconciled by hand — see [reconcileSettlement].
// There is no fallback address, because every candidate for one is somebody else's
// wallet.
//
// # Idempotent, and idempotent about the SAME thing
//
// The key is the provider's payment id on both halves: the stamped transaction this
// writes is found by the next delivery, and the seam credit carries it as the
// idempotency key. The host dedups a credit on that key WITHIN one ledger, so the
// key is only exactly-once if every path that can credit one payment resolves the
// same (org, subject, test) — which is why the address comes from the one persisted
// record rather than from whichever door happened to be looking.
//
// A "payment.updated" event only settles when the payment object reports a
// terminal completed/captured status; pending or failed updates are ignored.
func applySettlementEvent(ctx context.Context, db *datastore.Datastore, org *organization.Organization, event *processor.WebhookEvent) {
	if event.Data == nil {
		return
	}

	// Square nests the changed resource under data.object.<kind> (e.g.
	// data.object.payment for payment.* events). The processor passes
	// data.object through as event.Data, so unwrap the payment object here.
	// Fall back to event.Data itself for processors that put fields at the
	// top level.
	pay := unwrapObject(event.Data, "payment")

	paymentID := stringField(pay, "id")
	if paymentID == "" {
		return
	}

	// Only act on CAPTURED funds. Whenever the payment object reports a status it
	// MUST mean settled — processor.Settled, the SAME definition the charge path
	// uses, so a callback can never credit something a charge would have refused.
	// One rule for every payment.* event, not a per-event-type special case.
	//
	// This is where an APPROVED authorization used to be credited: funds reserved
	// but never taken, so a void or an expiry left the balance standing against
	// money that never arrived (a card-verification pre-auth is authorized and
	// then deliberately voided). An authorization that is genuinely captured
	// emits a later status change to COMPLETED, which credits then.
	//
	// An event whose object carries no status at all (invoice.paid) is settled by
	// its event type, which isSettlementEvent already gated.
	if st := stringField(pay, "status"); st != "" && !processor.Settled(st) {
		return
	}

	kind := chargeSourceKind(event.Processor)
	if kind == "" {
		reconcileSettlement(org, event, paymentID, "the delivery names no processor, so this payment has no reference this ledger can key on")
		return
	}

	amount, cur := settlementAmount(pay)
	if amount <= 0 {
		log.Warn("settlement %s: non-positive amount, recorded event only", paymentID)
		return
	}

	// WHOSE MONEY THIS IS, or nobody's. Everything below this line addresses the
	// wallet this answers with; nothing below it can invent one.
	who, why := settlementDestination(db, org, event, pay, kind, paymentID)
	if who == nil {
		if why == "" {
			return // already accounted for by a door that owns the credit
		}
		reconcileSettlement(org, event, paymentID, why)
		return
	}

	// THE SPENDABLE LEDGER FIRST, the local receipt second, and that order is the
	// recovery. This write is what makes the money spendable; the row below is a
	// record of it AND the marker that stops the next delivery. Writing the marker
	// first would mean a failed credit is never retried — the redelivery would find
	// the marker and skip a payment that funded nothing. This way a lost credit is
	// simply re-attempted on the next delivery, and a repeated one is the host's
	// idempotent replay of the same key at the same address.
	if led := creditledger.Get(); led != nil {
		if _, _, err := led.Credit(ctx, creditledger.CreditInput{
			Org:            who.org,
			Subject:        who.subject,
			Currency:       string(cur),
			Reason:         "settlement " + paymentID,
			Tag:            "topup",
			IdempotencyKey: paymentID,
			AmountCents:    int64(amount),
			Test:           who.test,
		}); err != nil {
			// The provider retries for up to 72h, so the honest answer is to leave no
			// marker and let the next delivery try again. Loud, because until one of
			// them succeeds this is a payment that settled and a balance that did not.
			log.Error("RECONCILE: settlement %s (%s %d %s) settled and the spendable balance was NOT credited: org=%s subject=%s test=%v: %v — retryable: the provider redelivers and the credit is keyed on the payment id",
				paymentID, event.Type, amount, cur, who.org, who.subject, who.test, err)
			return
		}
	}

	// The local receipt: commerce's own record of the same payment, at the SAME
	// address the seam credit went to, stamped with the payment id so the next
	// delivery recognises it ([settlementDestination]). Standalone — no host ledger — this
	// row IS the balance, which is why it is written either way.
	trans := transaction.New(db)
	trans.Type = transaction.Deposit
	trans.DestinationId = who.subject
	trans.DestinationKind = transaction.IAMUserKind
	trans.Currency = cur
	trans.Amount = amount
	trans.SourceId = paymentID
	trans.SourceKind = kind
	trans.Event = event.ID
	trans.Tags = "topup"
	trans.Notes = string(event.Processor) + " settlement " + paymentID
	trans.Metadata = Map{"provider": string(event.Processor), "eventType": event.Type}
	// The SAME bit the seam credit above carried, off the same value — one resolution
	// of which books, spent twice, so the two records of one payment can never
	// disagree about whether the money is spendable.
	trans.Test = who.test
	// A provider-signature-verified settlement for a real captured payment IS the
	// mint authority (the trust anchor is the per-provider signature checked in
	// HandleProviderWebhook), so authorize this write at the ledger sink.
	trans.SetContext(mintauth.WithAuthorized(trans.Context()))
	if err := trans.Create(); err != nil {
		log.Warn("settlement %s: failed to write the local receipt: %v", paymentID, err)
	}
}

// destination is WHERE a settled payment's money belongs: the ledger, the account
// inside it, and which books. It is the same three-part address
// [creditledger.CreditInput] names — and the same one the transaction model calls a
// Destination — because it is resolved once here and spent on both writes below.
type destination struct {
	org     string
	subject string
	test    bool
}

// settlementDestination answers who a settled payment funds, from what commerce
// ALREADY WROTE about that payment id — never from the callback's own fields.
//
// It returns one of three things, and the difference between the last two is the
// whole point:
//
//	an address — this payment funds prepaid balance and nothing has credited it yet.
//	(nil, "") — ACCOUNTED FOR. Some other path owns this payment's credit, so
//	crediting again would be the double-credit. Silent, because it is not a defect.
//	(nil, why) — UNATTRIBUTABLE. The money moved and this process cannot say whose
//	it is. Refused and reconciled by hand; there is no safe address to guess.
//
// The three lookups are all keyed on the provider's payment id, and each one is a
// record commerce wrote itself:
//
//	A STAMPED LEDGER ROW is an in-session charge. Both card doors stamp the deposit
//	they write with (SourceKind, SourceId) at charge time, so this row means either
//	that a door took this payment — in which case the host credited the spendable
//	ledger at its own resolved payer as the handler returned — or that an earlier
//	delivery of this very callback already credited it. Both are "already done".
//
//	A PAID INVOICE is a subscription charge. The money bought a PLAN: the collection
//	waterfall charges the card for the invoice REMAINDER and marks the invoice paid
//	with the processor's reference. Crediting a prepaid wallet for it would hand the
//	customer their subscription payment back as spendable balance.
//
//	A SUBSCRIPTION KEYED BY THE PROVIDER'S ID is a subscription billed OUTSIDE
//	commerce, reached only after the two above missed — so there is no local invoice
//	and no local deposit, and this callback is the only record of the money there
//	will be. Its UserId is the billing subject, the same <org> or <org>/<user> key
//	the doors resolve and the gate debits.
//
// EVERYTHING ELSE IS REFUSED: a payment link, a charge raised in the provider's
// dashboard, a checkout order's capture. Those really are unattributable here —
// commerce wrote nothing keyed on the payment id — and the previous code's habit of
// reaching for reference_id or customer_id is what put money in wallets that were an
// order id or a stranger's.
// pay is the SETTLED OBJECT, already unwrapped by the caller — the same map paymentID
// was read from. It is passed rather than re-derived because reading the subscription
// link off the ENVELOPE while the payment id came from the object inside it is how the
// two stopped describing one payment: a delivery nesting its object one level down
// answered a real id for the payment and nothing at all for the subscription, which is
// precisely the blank key the guard below now refuses.
func settlementDestination(db *datastore.Datastore, org *organization.Organization, event *processor.WebhookEvent, pay map[string]interface{}, kind, paymentID string) (*destination, string) {
	stamped := transaction.New(db)
	if found, err := stamped.Query().
		Filter("SourceKind=", kind).
		Filter("SourceId=", paymentID).
		Get(); err != nil {
		return nil, "the ledger could not be read to tell whether this payment was already credited"
	} else if found {
		return nil, ""
	}

	inv := billinginvoice.New(db)
	if found, err := inv.Query().Filter("PaymentRef=", paymentID).Get(); err != nil {
		return nil, "the invoices could not be read to tell whether this payment settled one"
	} else if found {
		return nil, ""
	}

	// Only a subscription lifecycle delivery carries a subscription to resolve. A
	// payment.* event that reached here named no subscription and matched no record
	// commerce wrote, which is exactly the unattributable case.
	if !strings.HasPrefix(event.Type, "subscription.") && !strings.HasPrefix(event.Type, "invoice.") {
		return nil, "no charge this process wrote carries this payment reference, and the payload names no subscription — the payer is unknown"
	}
	// A BLANK KEY IS NOT A LOOKUP, IT IS A WILDCARD, and this is the guard that says so.
	//
	// No production call site sets ProviderId — every subscription commerce creates
	// itself leaves it "" — so `Filter("ProviderId=", "")` does not miss. It MATCHES,
	// and .Get() answers with the first internal subscription in the org. A delivery
	// naming no subscription therefore credited a stranger's wallet for whatever
	// amount the payload asked for, which is the wildcard a signature forger would
	// aim at. The sibling reconciler has always refused a blank id up front
	// ([applySubscriptionEvent]); this asks the same question before the same query.
	//
	// It is stated here rather than left to the not-found branch because a blank key
	// never REACHES not-found: the emptier the payload, the more it matches.
	ref := subscriptionRef(event.Type, pay)
	if ref == "" {
		return nil, "the settlement names no subscription, so there is nothing to resolve a payer from"
	}
	sub := subscription.New(db)
	found, err := sub.Query().Filter("ProviderId=", ref).Get()
	if err != nil || !found {
		return nil, "the subscription this settlement names is not in these books, so the payer is unknown"
	}
	subject := strings.ToLower(strings.TrimSpace(sub.UserId))
	if subject == "" {
		return nil, "the subscription this settlement names states no owner, so the payer is unknown"
	}
	return &destination{
		org:     strings.ToLower(strings.TrimSpace(org.Name)),
		subject: subject,
		// Which books, from the one authority the charge itself used: a sandbox
		// deployment's settlement funds the sandbox ledger and can never buy live
		// inference.
		test: org.TestMode(),
	}, ""
}

// subscriptionRef is the provider's SUBSCRIPTION id in a lifecycle delivery, or "".
//
// An explicit link is the only thing read for an INVOICE delivery. The object's own
// "id" used to be a general fallback, and it is a different thing on every event type
// this is asked about: on an invoice.paid it is the INVOICE's id, so the fallback
// quietly offered an invoice id as a subscription key. That is not merely a lookup
// that misses — paired with the blank-key wildcard it was the shape of a payload that
// resolves to somebody. A subscription is named or it is not.
//
// The one place the object's own id IS the subscription is a subscription.* delivery,
// where the object simply IS the subscription. That is a fact about the event type,
// so the event type is what decides it rather than a guess ordered by luck.
func subscriptionRef(eventType string, data map[string]interface{}) string {
	if s := firstNonEmpty(
		stringField(data, "subscription_id"),
		stringField(data, "subscriptionId"),
		stringField(data, "subscription"),
	); s != "" {
		return s
	}
	// THIS BRANCH IS SAFE ONLY BECAUSE [isSettlementEvent] EXCLUDES subscription.*.
	//
	// Read together, the bare id here and [settlementAmount]'s read of the same
	// attacker-supplied object are a mint: name any subscription's ProviderId, state
	// any amount, and its owner is credited. Nothing in THIS function prevents that.
	// What prevents it is that a subscription.* delivery never reaches
	// [applySettlementEvent] at all — only payment.completed, payment.updated and
	// invoice.paid settle, and this branch cannot fire for any of them.
	//
	// So the two guards are not equals: the event-type gate is the load-bearing one.
	// Adding ANY subscription.* type to isSettlementEvent turns this line into a live
	// mint, and it would look like a one-word change to whoever makes it. If a
	// subscription.* type ever has to settle, resolve its payer from a record commerce
	// wrote — the way the other branches do — before that type is added.
	if strings.HasPrefix(eventType, "subscription.") {
		return stringField(data, "id")
	}
	return ""
}

// chargeSourceKind is the ledger SourceKind under which a processor's own charge
// reference is filed. It is ONE function because two sides must spell it identically:
// the card doors stamp a settled charge with it, and this callback looks that charge
// up by it. Two literals drift, the lookup silently finds nothing, and "nothing" here
// reads exactly like "no door ever took this payment" — which credits it twice.
//
// It carries the processor's name because the reference inside it is that processor's,
// and two processors' id spaces are not one space. For Square it is the same
// "square-payment" every row already written by this callback carries.
func chargeSourceKind(p processor.ProcessorType) string {
	name := strings.ToLower(strings.TrimSpace(string(p)))
	if name == "" {
		return ""
	}
	return name + "-payment"
}

// reconcileSettlement is the ONE answer to "money arrived and this process cannot say
// whose it is".
//
// It is loud and it credits nothing, and those are the same decision. Every address
// available to guess with belongs to somebody else — the service org whose token
// resolved the namespace, the provider's own customer id, an order id sitting in
// reference_id — so a guess is not a slightly-wrong credit, it is a customer's money
// in a stranger's wallet, reported as success. The refusal names the payment, the
// books it was refused in, and the door that settles it, because a settled payment
// nobody is told about is the defect this whole path exists to end.
func reconcileSettlement(org *organization.Organization, event *processor.WebhookEvent, paymentID, why string) {
	log.Error("RECONCILE: a settled payment could not be attributed and NO balance was credited — "+
		"payment=%s provider=%s eventType=%s books=%s test=%v why=%s recovery=identify the payer and issue the balance with POST /v1/admin/grants, or refund the charge with the processor",
		paymentID, event.Processor, event.Type, org.Name, org.TestMode(), why)
}

// settlementAmount extracts the captured amount (cents) and currency from a
// provider payment object. Square nests it under amount_money{amount,currency}.
func settlementAmount(data Map) (currency.Cents, currency.Type) {
	cur := currency.USD
	if m, ok := data["amount_money"].(map[string]interface{}); ok {
		amt, ok := numberField(m, "amount")
		if !ok {
			return 0, cur
		}
		if c := stringField(m, "currency"); c != "" {
			cur = currency.Type(strings.ToLower(c))
		}
		return currency.Cents(amt), cur
	}
	// Flat fallbacks used by other processors.
	if c := stringField(data, "currency"); c != "" {
		cur = currency.Type(strings.ToLower(c))
	}
	amt, ok := numberField(data, "amount")
	if !ok {
		return 0, cur
	}
	return currency.Cents(amt), cur
}

// unwrapObject returns data[key] as a map when present (Square nests the
// changed resource one level deep, e.g. data.object.payment), otherwise the
// data map itself (processors that expose fields at the top level).
func unwrapObject(data Map, key string) map[string]interface{} {
	if inner, ok := data[key].(map[string]interface{}); ok {
		return inner
	}
	return data
}

func stringField(m map[string]interface{}, key string) string {
	v, _ := m[key].(string)
	return strings.TrimSpace(v)
}

// numberField reads a MINOR-UNIT amount out of a decoded webhook body, and
// reports whether it could be read exactly.
//
// Every processor that reaches here states the amount in minor units as a whole
// number — Square's amount_money.amount and Stripe's amount are both integer
// cents — but the body arrives as map[string]interface{}, so encoding/json has
// already turned each one into a float64. A whole number survives that exactly.
//
// A FRACTIONAL value therefore does not mean "cents with a fraction", it means
// the payload is not the shape this code assumes — most likely major units, so
// 19.99 is a $19.99 payment and not 19 cents. int64() truncated it, and because
// this feeds a Deposit credited to a customer, it under-credited them by two
// orders of magnitude and did it silently.
//
// There is no safe guess between the two readings, so this refuses. The caller
// already treats a non-positive amount as "record the event, write no ledger
// row" and warns, which is exactly the right outcome for an amount we cannot
// read.
func numberField(m map[string]interface{}, key string) (int64, bool) {
	switch n := m[key].(type) {
	case float64:
		if n != math.Trunc(n) {
			return 0, false
		}
		return int64(n), true
	case int64:
		return n, true
	case int:
		return int64(n), true
	}
	return 0, false
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// applySubscriptionEvent reconciles the local subscription row with a
// lifecycle event from the payment provider.
func applySubscriptionEvent(db *datastore.Datastore, event *processor.WebhookEvent) {
	// The provider's SUBSCRIPTION id, which for an invoice delivery is not the
	// object's own id — see [subscriptionRef]. Reading "id" alone meant every
	// invoice.* event looked a subscription up by an INVOICE id, found none, and
	// silently left the local row stale.
	id := subscriptionRef(event.Type, event.Data)
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

	// Inbound validation ignores the disabled-processor deny policy (which
	// governs OUTBOUND charge selection only): a processor disabled for new
	// charges — e.g. Stripe — must still validate its existing customers' webhook
	// deliveries. Hence Registered / RegisteredProcessors, not Get / Available.

	// Fast path: provider hint specified.
	if providerHint != "" {
		if p, err := registry.Registered(processor.ProcessorType(providerHint)); err == nil {
			return p.ValidateWebhook(ctx, payload, signature)
		}
	}

	// Fallback: try every registered, configured processor until one succeeds.
	var lastErr error
	for _, p := range registry.RegisteredProcessors() {
		if !p.IsAvailable(ctx) {
			continue
		}
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
		"X-Webhook-Signature",    // Hanzo MPC (hex HMAC-SHA256 over the raw body)
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
		case "mpc":
			if v := h.Get("X-Webhook-Signature"); v != "" {
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
