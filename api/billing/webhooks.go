package billing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	nethttp "net/http"
	"strings"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/billing/creditledger"
	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/checkout"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/mintauth"
	"github.com/hanzoai/commerce/models/billingevent"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/dispute"
	"github.com/hanzoai/commerce/models/idempotencykey"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/paymentorg"
	"github.com/hanzoai/commerce/models/pendingintent"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/transaction"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/util/nscontext"

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
//
//	POST /v1/commerce/webhooks/:provider
//
// The path names the processor, and only that processor verifies the delivery: a
// name no processor is registered under is refused, never tried against the rest.
// The verified event is recorded once — globally, by (processor, event id) — then
// written to the billing events of the org the processor account belongs to, and
// its settlement, subscription and clawback effects are applied.
//
// Nothing about WHERE an event lands comes from the request. The org is the one
// the verifying processor names (processor.WebhookEvent.Org), else the
// deployment's own brand: a provider sends no identity, and a header or session
// on a replayed delivery is the replayer's.
func HandleProviderWebhook(c *zip.Ctx) error {
	providerHint := strings.ToLower(strings.TrimSpace(c.Param("provider")))
	payload := c.Body()

	// Every provider puts its signature in a different header.
	reqHeaders := http.Header{}
	for k, vals := range c.Fiber().GetReqHeaders() {
		for _, v := range vals {
			reqHeaders.Add(k, v)
		}
	}
	signature := pickSignatureHeader(reqHeaders, providerHint)
	if signature == "" {
		return jsonhttp.Fail(c, nethttp.StatusBadRequest, "missing webhook signature header", nil)
	}

	ctx, cancel := context.WithTimeout(c.Context(), 15*time.Second)
	defer cancel()

	event, err := tryValidateWebhook(ctx, providerHint, payload, signature)
	if err != nil || event == nil {
		log.Warn("webhook signature validation failed (provider hint=%s): %v", providerHint, err)
		return jsonhttp.Fail(c, nethttp.StatusUnauthorized, "invalid webhook signature", err)
	}

	// ONCE, AND FOR THE WHOLE DEPLOYMENT. A provider retries a delivery until it
	// sees a 2xx, and resends the same body each time, so the event id is what
	// makes the second one a duplicate. The guard is keyed on the processor that
	// verified it and the event id, in the system namespace, and taken with an
	// insert-if-absent swap — not on the path segment and not inside one org's
	// books, either of which a replayer chooses.
	guard, seen, err := idempotencykey.Begin(webhookGuardDB(ctx), "webhook", string(event.Processor)+":"+event.ID)
	switch {
	case err != nil:
		log.Error("webhook %s %s: the delivery guard could not be read: %v", event.Processor, event.ID, err)
		return jsonhttp.Fail(c, nethttp.StatusServiceUnavailable, "webhook guard unavailable", err)
	case seen && guard.Status == idempotencykey.StatusCompleted:
		return c.JSON(nethttp.StatusOK, map[string]any{
			"received":  true,
			"type":      event.Type,
			"id":        event.ID,
			"duplicate": true,
		})
	case seen:
		// Another delivery of this event is being applied right now. Refuse this one
		// so the provider retries it, rather than acknowledging work that may not land.
		return jsonhttp.Fail(c, nethttp.StatusConflict, "webhook delivery already in progress", nil)
	}

	org := bookedOrg(c.Context(), event)
	if org == nil {
		return jsonhttp.Fail(c, nethttp.StatusServiceUnavailable, "organization context unavailable", nil)
	}
	db := datastore.New(org.Namespaced(c.Context()))

	// The effects. Each one is safe to apply again — a credit is keyed on the
	// payment id, a clawback on the refund or dispute id, a dispute report on its
	// version — so one that could not land (a store or ledger that did not answer)
	// is reported here and the delivery is left OPEN: no Complete, a 503, and the
	// provider redelivers. The guard is re-claimed once StartedTTL has passed.
	var effects []error
	if strings.HasPrefix(event.Type, "subscription.") || strings.HasPrefix(event.Type, "invoice.") {
		effects = append(effects, applySubscriptionEvent(db, event))
	}
	// Settle funds for completed payments. This is the path Square's
	// payment.updated(status=COMPLETED) callback exercises once a card
	// authorization captures — the moment funds are guaranteed.
	if isSettlementEvent(event.Type) {
		effects = append(effects, applySettlementEvent(ctx, db, org, event))
	}
	// A cancelled payment credits nothing and closes its waiting record, so the
	// reconciler never looks at it again. (payment.failed leaves the record open: the
	// buyer may try the same intent with another method.)
	if event.Type == "payment.canceled" && event.Processor == processor.Stripe {
		if pi := stringField(event.Data, "payment_intent"); pi != "" {
			settleWallet(pi)
		}
	}
	if isClawbackEvent(event.Type) {
		effects = append(effects, applyClawbackEvent(ctx, db, org, event))
	}
	// charge.refunded names the charge; the refunds on it are read from Stripe and
	// each taken back once, by its own id.
	if event.Processor == processor.Stripe && event.Type == "refund.succeeded" {
		test, _ := event.Data["stripe_test"].(bool)
		refunds, rerr := stripeRefunds(ctx, org, stringField(event.Data, "charge"), test)
		effects = append(effects, rerr)
		for _, r := range refunds {
			r.ID = event.ID
			effects = append(effects, applyClawbackEvent(ctx, db, org, r))
		}
	}
	if err := errors.Join(effects...); err != nil {
		log.Error("webhook %s %s: an effect did not land; the delivery stays open for the provider to redeliver: %v", event.Processor, event.ID, err)
		return jsonhttp.Fail(c, nethttp.StatusServiceUnavailable, "webhook effect did not land; redeliver", err)
	}

	// The org's own record of the event, an audit trail independent of the
	// processor's retention. Written once, by the delivery that applied it.
	evt := billingevent.New(db)
	evt.Type = event.Type
	evt.ObjectType = string(event.Processor)
	evt.ObjectId = event.ID
	evt.Livemode = !org.TestMode() // one authority for the books and the record of them: the org
	if event.Data != nil {
		evt.Data = event.Data
	}
	if err := evt.Create(); err != nil {
		log.Warn("failed to persist billing event %s: %v", event.ID, err)
	}

	if err := idempotencykey.Complete(guard, ""); err != nil {
		log.Warn("webhook %s %s: applied, and the delivery guard was not marked complete: %v", event.Processor, event.ID, err)
	}
	return c.JSON(nethttp.StatusOK, map[string]any{
		"received": true,
		"type":     event.Type,
		"id":       event.ID,
	})
}

// webhookGuardDB is the deployment-wide namespace the delivery guard lives in, so
// one event is one record whichever org it is booked to.
func webhookGuardDB(ctx context.Context) *datastore.Datastore {
	return datastore.New(nscontext.WithNamespace(ctx, "system"))
}

// bookedOrg is the org a verified delivery is booked to: the org the verifying
// processor's account belongs to, or the deployment's own brand when the
// processor names none (the deployment's Square account, seeded from its
// environment). The request contributes nothing — no header, no session.
//
// THE ORG RECORD SAYS WHICH BOOKS. It is loaded and believed, never forced live:
// an org that does not say Live transacts in sandbox, so a sandbox callback can
// never credit the spendable books.
func bookedOrg(ctx context.Context, event *processor.WebhookEvent) *organization.Organization {
	name := strings.TrimSpace(event.Org)
	if name == "" {
		name = checkout.DefaultBrandSlug()
	}
	org := organization.New(datastore.New(ctx))
	org.Name = name
	if err := org.GetOrCreate("Name=", name); err != nil {
		log.Warn("webhook org resolve for '%s' failed: %v", name, err)
		return nil
	}
	if event != nil && event.Data != nil {
		sb, _ := event.Data["paypal_sandbox"].(bool)
		if st, _ := event.Data["stripe_test"].(bool); st {
			sb = true
		}
		if sb {
			cp := org.In(organization.Test)
			return &cp
		}
	}
	return org
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
// that decides whether a request is served never changed. The in-session endpoints close
// that half themselves (the host's settle.go, at the point their handler returns);
// this is the same half for the settlements that have no endpoint.
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
// record rather than from whichever endpoint happened to be looking.
//
// A "payment.updated" event only settles when the payment object reports a
// terminal completed/captured status; pending or failed updates are ignored.
func applySettlementEvent(ctx context.Context, db *datastore.Datastore, org *organization.Organization, event *processor.WebhookEvent) error {
	if event.Data == nil {
		return nil
	}

	// Square nests the changed resource under data.object.<kind> (e.g.
	// data.object.payment for payment.* events). The processor passes
	// data.object through as event.Data, so unwrap the payment object here.
	// Fall back to event.Data itself for processors that put fields at the
	// top level.
	pay := unwrapObject(event.Data, "payment")

	paymentID := stringField(pay, "id")
	if paymentID == "" {
		return nil
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
		return nil
	}

	kind := chargeSourceKind(event.Processor)
	if kind == "" {
		reconcileSettlement(org, event, paymentID, "the delivery names no processor, so this payment has no reference this ledger can key on")
		return nil
	}

	amount, cur := settlementAmount(pay)
	if amount <= 0 {
		log.Warn("settlement %s: non-positive amount, recorded event only", paymentID)
		return nil
	}

	// WHOSE MONEY THIS IS, or nobody's. Everything below this line addresses the
	// wallet this answers with; nothing below it can invent one.
	who, why, err := settlementDestination(db, org, event, pay, kind, paymentID)
	if err != nil {
		return fmt.Errorf("settlement %s: %w", paymentID, err)
	}
	if who == nil {
		if why == "" {
			// Already accounted for by an endpoint that owns the credit: a registered
			// intent's row is closed with it.
			if pi := stringField(pay, "payment_intent"); event.Processor == processor.Stripe && pi != "" {
				settleWallet(pi)
			}
			return nil
		}
		reconcileSettlement(org, event, paymentID, why)
		return nil
	}

	// What the processor took, kept for the soundness check; what is CREDITED is the
	// quote the buyer agreed to when it differs (a local method), never the
	// processor's figure in another currency.
	charged, chargedCur := amount, cur
	if who.credit > 0 {
		amount, cur = who.credit, who.creditCur
	}

	// A Stripe intent is credited by ONE claimant across the webhook, the
	// reconciler, the browser's top-up and a sale (the claim `billing-intent`). One
	// that finds it taken has nothing to do; one that fails to credit gives it back
	// so the redelivery can.
	var claim *idempotencykey.IdempotencyKey
	if pi := stringField(pay, "payment_intent"); event.Processor == processor.Stripe && pi != "" {
		g, taken, cerr := idempotencykey.Begin(webhookGuardDB(ctx), "billing-intent", pi)
		if cerr != nil {
			return fmt.Errorf("settlement %s: the intent's claim could not be taken: %w", paymentID, cerr)
		}
		if taken {
			// Another holder is crediting it, or has. One still at it may yet give
			// the claim back, and then nothing else would credit: answer a retry.
			if g.Status != idempotencykey.StatusCompleted {
				return fmt.Errorf("settlement %s: another path holds the intent's claim; redeliver", paymentID)
			}
			settleWallet(pi)
			return nil
		}
		claim = g
		// Money in hand only: a charge already refunded or disputed is not credited.
		// The check is never skipped: a processor that cannot be asked is a retry, and
		// it is asked with the keys of the org the intent was recorded in, in the
		// mode it was recorded in.
		keysOrg := org
		if pr, found, _ := pendingintent.Get(pi); found {
			o, oerr := intentOrg(ctx, pr.Org, pr.Test)
			if oerr != nil {
				_ = g.Delete()
				return fmt.Errorf("settlement %s: %w", paymentID, oerr)
			}
			keysOrg = o
		}
		sp, serr := processorsForOrg(keysOrg).Existing(ctx, processor.Stripe)
		cs, ok := sp.(interface {
			ChargeSound(context.Context, string, int64) (bool, error)
		})
		if serr != nil || !ok {
			_ = g.Delete()
			return fmt.Errorf("settlement %s: Stripe cannot be asked about the charge: %v", paymentID, serr)
		}
		sound, cerr := cs.ChargeSound(ctx, paymentID, int64(charged))
		if cerr != nil {
			_ = g.Delete()
			return fmt.Errorf("settlement %s: the charge could not be read: %w", paymentID, cerr)
		}
		if !sound {
			// Taken money that is refunded in part, disputed or short is not credited,
			// and is said loudly: someone must place what is left of it.
			_ = idempotencykey.Complete(g, "unsound")
			settleWallet(pi)
			log.Error("RECONCILE: charge %s (intent %s, %d %s) was taken and is refunded, disputed or short, so nothing was credited; place what remains of it by hand", paymentID, pi, amount, cur)
			return nil
		}
	}
	release := func() {
		if claim != nil {
			_ = claim.Delete()
		}
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
			Cash:           true, // a provider-verified settlement of a captured payment
		}); err != nil {
			// The provider retries for up to 72h, so the honest answer is to leave no
			// marker and let the next delivery try again. Loud, because until one of
			// them succeeds this is a payment that settled and a balance that did not.
			log.Error("RECONCILE: settlement %s (%s %d %s) settled and the spendable balance was NOT credited: org=%s subject=%s test=%v: %v — retryable: the provider redelivers and the credit is keyed on the payment id",
				paymentID, event.Type, amount, cur, who.org, who.subject, who.test, err)
			release()
			return fmt.Errorf("settlement %s: credit: %w", paymentID, err)
		}
	}

	// The receipt is written in the books of the org the money was credited in.
	if who.org != strings.ToLower(strings.TrimSpace(org.Name)) {
		db = datastore.New(nscontext.WithNamespace(ctx, who.org))
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
	if err := paymentorg.Put(paymentID, who.org, who.subject, "", 0, who.test); err != nil {
		log.Warn("settlement %s: the org index was not written: %v", paymentID, err)
	} else if who.credit > 0 {
		// Paid in the buyer's currency against a quote: remember both figures, so a
		// refund or a dispute in that currency is taken back in the dollars credited.
		if _, _, err := paymentorg.Change(paymentID, func(r *paymentorg.PaymentOrg) bool {
			r.PaidCents, r.PaidCurrency, r.CreditCents = int64(charged), strings.ToLower(string(chargedCur)), int64(amount)
			return true
		}); err != nil {
			log.Error("RECONCILE: payment %s was credited %d against a quote of %d %s and the quote was not recorded, so a refund of it cannot be converted: %v", paymentID, amount, charged, chargedCur, err)
		}
	}
	if claim != nil {
		_ = idempotencykey.Complete(claim, paymentID)
	}
	if pi := stringField(pay, "payment_intent"); pi != "" {
		settleWallet(pi)
	}
	return nil
}

// settleWallet closes a pending intent that credited a wallet. It NEVER closes a
// plan's or an invoice's: those are finished by their own sale or payment, or
// refunded by the reconciler, and a webhook that closed one would leave a charge
// with no plan, no refund and no alarm.
func settleWallet(pi string) {
	p, found, err := pendingintent.Get(pi)
	if err != nil || !found || p.Purpose == pendingintent.Sale || p.Purpose == pendingintent.Invoice {
		return
	}
	if _, err := pendingintent.Settle(pi); err != nil {
		log.Warn("pending intent %s was not marked settled: %v", pi, err)
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
	// credit is what the payment funds when that is not what the processor took: a
	// local method's quoted USD. Zero means the settled amount itself.
	credit    currency.Cents
	creditCur currency.Type
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
// An error means a record could not be READ, so none of the three is known yet:
// the delivery stays open and the provider redelivers it.
//
// The lookups are all keyed on the provider's payment id, and each one is a
// record commerce wrote itself:
//
//	THE PAYMENT INDEX names the org and wallet of every receipt a charge endpoint
//	wrote, wherever it wrote it. Indexed means credited, in whichever books.
//
//	A STAMPED LEDGER ROW is an in-session charge. Both card endpoints stamp the deposit
//	they write with (SourceKind, SourceId) at charge time, so this row means either
//	that an endpoint took this payment — in which case the host credited the spendable
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
//	the endpoints resolve and the gate debits.
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
func settlementDestination(db *datastore.Datastore, org *organization.Organization, event *processor.WebhookEvent, pay map[string]interface{}, kind, paymentID string) (*destination, string, error) {
	// THE PAYMENT INDEX FIRST, because it is the one record that is not in this
	// org's books. A charge taken in session writes its receipt in the PAYER's org
	// and then indexes the payment id to it (models/paymentorg); this delivery
	// resolved the org the processor account belongs to, which is a different
	// namespace for every payer but one. Without this read, every in-session charge
	// by a payer outside that org came back "no charge this process wrote", a
	// RECONCILE alarm for money that was already credited.
	if _, found, err := paymentorg.Get(paymentID); err != nil {
		return nil, "", fmt.Errorf("the payment index could not be read: %w", err)
	} else if found {
		return nil, "", nil
	}

	stamped := transaction.New(db)
	if found, err := stamped.Query().
		Filter("SourceKind=", kind).
		Filter("SourceId=", paymentID).
		Get(); err != nil {
		return nil, "", fmt.Errorf("the ledger could not be read: %w", err)
	} else if found {
		return nil, "", nil
	}

	inv := billinginvoice.New(db)
	if found, err := inv.Query().Filter("PaymentRef=", paymentID).Get(); err != nil {
		return nil, "", fmt.Errorf("the invoices could not be read: %w", err)
	} else if found {
		return nil, "", nil
	}

	// A PaymentIntent pre-registered for its buyer before their bank's step
	// (pendingintent) says whose it is and what for, whether or not the buyer's
	// browser ever came back. A sale's intent is finished by its sale, and a
	// reconciler refunds one that never is; a wallet's is credited here.
	if event.Processor == processor.Stripe {
		if pi := stringField(pay, "payment_intent"); pi != "" {
			p, found, err := pendingintent.Get(pi)
			if err != nil {
				return nil, "", fmt.Errorf("the pending intents could not be read: %w", err)
			}
			if found {
				amount, cur := settlementAmount(pay)
				wantCents, wantCur := p.Charged()
				switch {
				case p.Purpose == pendingintent.Sale || p.Purpose == pendingintent.Invoice:
					return nil, "", nil
				case int64(amount) != wantCents || !strings.EqualFold(string(cur), wantCur):
					return nil, fmt.Sprintf("intent %s was registered for %d %s and settled %d %s", pi, wantCents, wantCur, amount, cur), nil
				}
				// The receipt may already be in the PAYER's org, whose books are not the
				// event's org's: asked of both, or a payment the browser credited is
				// credited again.
				payer := transaction.New(datastore.New(nscontext.WithNamespace(db.Context, strings.ToLower(strings.TrimSpace(p.Org)))))
				if dup, derr := payer.Query().Filter("SourceKind=", kind).Filter("SourceId=", paymentID).Get(); derr != nil {
					return nil, "", fmt.Errorf("the payer's ledger could not be read: %w", derr)
				} else if dup {
					return nil, "", nil
				}
				d := &destination{org: strings.ToLower(strings.TrimSpace(p.Org)), subject: strings.ToLower(strings.TrimSpace(p.Subject)), test: p.Test}
				if p.PaidCurrency != "" {
					// The buyer paid the quoted local amount; the wallet gets the USD they agreed to buy.
					d.credit, d.creditCur = currency.Cents(p.Cents), currency.Type(p.Currency)
				}
				return d, "", nil
			}
		}
	}

	// Only a subscription lifecycle delivery carries a subscription to resolve. A
	// payment.* event that reached here named no subscription and matched no record
	// commerce wrote, which is exactly the unattributable case.
	if !strings.HasPrefix(event.Type, "subscription.") && !strings.HasPrefix(event.Type, "invoice.") {
		return nil, "no charge this process wrote carries this payment reference, and the payload names no subscription — the payer is unknown", nil
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
		return nil, "the settlement names no subscription, so there is nothing to resolve a payer from", nil
	}
	sub := subscription.New(db)
	found, err := sub.Query().Filter("ProviderId=", ref).Get()
	if err != nil {
		return nil, "", fmt.Errorf("the subscriptions could not be read: %w", err)
	}
	if !found {
		return nil, "the subscription this settlement names is not in these books, so the payer is unknown", nil
	}
	subject := strings.ToLower(strings.TrimSpace(sub.UserId))
	if subject == "" {
		return nil, "the subscription this settlement names states no owner, so the payer is unknown", nil
	}
	return &destination{
		org:     strings.ToLower(strings.TrimSpace(org.Name)),
		subject: subject,
		// Which books, from the one authority the charge itself used: a sandbox
		// deployment's settlement funds the sandbox ledger and can never buy live
		// inference.
		test: org.TestMode(),
	}, "", nil
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
// the card endpoints stamp a settled charge with it, and this callback looks that charge
// up by it. Two literals drift, the lookup silently finds nothing, and "nothing" here
// reads exactly like "no endpoint ever took this payment" — which credits it twice.
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
// books it was refused in, and the endpoint that settles it, because a settled payment
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
func applySubscriptionEvent(db *datastore.Datastore, event *processor.WebhookEvent) error {
	// The provider's SUBSCRIPTION id, which for an invoice delivery is not the
	// object's own id — see [subscriptionRef]. Reading "id" alone meant every
	// invoice.* event looked a subscription up by an INVOICE id, found none, and
	// silently left the local row stale.
	id := subscriptionRef(event.Type, event.Data)
	if id == "" {
		return nil
	}

	sub := subscription.New(db)
	found, err := sub.Query().Filter("ProviderId=", id).Get()
	if err != nil {
		return fmt.Errorf("subscription %s: %w", id, err)
	}
	if !found {
		// Unknown subscription — likely created outside commerce.
		return nil
	}

	if status, ok := event.Data["status"].(string); ok && status != "" {
		sub.Status = subscription.Status(status)
	}
	if event.Type == "subscription.canceled" {
		sub.Canceled = true
		sub.CanceledAt = time.Now().UTC()
	}
	if err := sub.Update(); err != nil {
		return fmt.Errorf("subscription %s: %w", sub.Id(), err)
	}
	return nil
}

// tryValidateWebhook asks the processor the path names, and only it, to verify a
// delivery. A name no processor is registered under, and a processor that holds
// no webhook secret, are refused.
//
// Inbound validation ignores the disabled-processor deny policy (which governs
// OUTBOUND charge selection only): a processor disabled for new charges — e.g.
// Stripe — must still validate its existing customers' webhook deliveries. Hence
// Registered, not Get.
func tryValidateWebhook(ctx context.Context, providerHint string, payload []byte, signature string) (*processor.WebhookEvent, error) {
	p, err := processor.Global().Registered(processor.ProcessorType(providerHint))
	if err != nil {
		return nil, fmt.Errorf("%w: no processor named %q", processor.ErrWebhookValidationFailed, providerHint)
	}
	// Only a processor that holds the secret its deliveries are signed with may
	// verify one. Every provider package is registered, configured or not, and
	// some verifiers need no secret at all (a public-chain lookup, a certificate
	// fetched from a URL the payload names); asking one of those would let a
	// caller author an event.
	if rx, ok := p.(processor.WebhookReceiver); !ok || !rx.ReceivesWebhooks() {
		return nil, fmt.Errorf("%w: %q holds no webhook secret", processor.ErrWebhookValidationFailed, providerHint)
	}
	evt, err := p.ValidateWebhook(ctx, payload, signature)
	if err != nil {
		return nil, err
	}
	if evt != nil && evt.ID == "" {
		// The event id is the delivery guard's key; an event without one cannot
		// be recorded once, so it is not applied at all.
		return nil, fmt.Errorf("%w: %s delivery carries no event id", processor.ErrWebhookValidationFailed, providerHint)
	}
	if evt != nil && evt.Processor == processor.PayPal {
		normalizePayPalWebhookEvent(evt)
	}
	if evt != nil && evt.Processor == processor.Stripe {
		normalizeStripeWebhookEvent(evt)
	}
	return evt, nil
}

// encodePayPalWebhookHeaders packs PayPal's 5 transmission signature headers
// into the JSON object paypal.Provider.ValidateWebhook forwards to
// POST /v1/notifications/verify-webhook-signature.
func encodePayPalWebhookHeaders(h http.Header) string {
	sig := strings.TrimSpace(h.Get("Paypal-Transmission-Sig"))
	if sig == "" {
		return ""
	}
	b, err := json.Marshal(map[string]string{
		"transmission_id":   strings.TrimSpace(h.Get("Paypal-Transmission-Id")),
		"transmission_time": strings.TrimSpace(h.Get("Paypal-Transmission-Time")),
		"cert_url":          strings.TrimSpace(h.Get("Paypal-Cert-Url")),
		"auth_algo":         strings.TrimSpace(h.Get("Paypal-Auth-Algo")),
		"transmission_sig":  sig,
	})
	if err != nil {
		return ""
	}
	return string(b)
}

// normalizePayPalWebhookEvent maps PayPal REST API v2 event types and resource
// payloads into the canonical settlement, refund, dispute, and subscription
// event shapes HandleProviderWebhook applies.
func normalizePayPalWebhookEvent(evt *processor.WebhookEvent) {
	if evt == nil {
		return
	}
	if evt.Data == nil {
		evt.Data = map[string]interface{}{}
	}
	rawType := strings.ToUpper(strings.TrimSpace(evt.Type))
	evt.Data["paypal_event_type"] = evt.Type
	res, _ := evt.Data["resource"].(map[string]interface{})

	switch rawType {
	case "PAYMENT.CAPTURE.COMPLETED":
		evt.Type = "payment.completed"
		populatePayPalPaymentData(evt.Data, res, "COMPLETED")
	case "PAYMENT.CAPTURE.PENDING", "PAYMENT.CAPTURE.DENIED", "PAYMENT.CAPTURE.DECLINED":
		evt.Type = "payment.updated"
		populatePayPalPaymentData(evt.Data, res, "")
	case "PAYMENT.CAPTURE.REFUNDED":
		evt.Type = "refund.updated"
		populatePayPalRefundData(evt.Data, res, false)
	case "PAYMENT.CAPTURE.REVERSED":
		evt.Type = "refund.updated"
		populatePayPalRefundData(evt.Data, res, true)
	case "CUSTOMER.DISPUTE.CREATED", "CUSTOMER.DISPUTE.UPDATED", "CUSTOMER.DISPUTE.RESOLVED":
		evt.Type = "dispute.state.updated"
		populatePayPalDisputeData(evt.Data, res)
	case "BILLING.SUBSCRIPTION.CANCELLED", "BILLING.SUBSCRIPTION.SUSPENDED", "BILLING.SUBSCRIPTION.EXPIRED":
		evt.Type = "subscription.canceled"
		if id := stringField(res, "id"); id != "" {
			evt.Data["id"] = id
		}
		evt.Data["status"] = "canceled"
	case "BILLING.SUBSCRIPTION.ACTIVATED", "BILLING.SUBSCRIPTION.UPDATED", "BILLING.SUBSCRIPTION.RE-ACTIVATED":
		evt.Type = "subscription.updated"
		if id := stringField(res, "id"); id != "" {
			evt.Data["id"] = id
		}
		if st := strings.ToLower(stringField(res, "status")); st != "" {
			evt.Data["status"] = st
		}
	}
}

func parsePayPalMoneyMap(raw interface{}) map[string]interface{} {
	m, ok := raw.(map[string]interface{})
	if !ok {
		return nil
	}
	valStr := stringField(m, "value")
	curCode := stringField(m, "currency_code")
	if curCode == "" {
		curCode = stringField(m, "currency")
	}
	if valStr == "" || curCode == "" {
		return nil
	}
	cur := currency.Type(strings.ToLower(curCode))
	cents, err := cur.Parse(valStr)
	if err != nil {
		return nil
	}
	return map[string]interface{}{
		"amount":   int64(cents),
		"currency": strings.ToUpper(curCode),
	}
}

func populatePayPalPaymentData(dst map[string]interface{}, res map[string]interface{}, defaultStatus string) {
	if res == nil {
		return
	}
	pay := map[string]interface{}{}
	if id := stringField(res, "id"); id != "" {
		pay["id"] = id
	}
	st := stringField(res, "status")
	if st == "" {
		st = defaultStatus
	}
	if st != "" {
		pay["status"] = st
	}
	if am := parsePayPalMoneyMap(res["amount"]); am != nil {
		pay["amount_money"] = am
	}
	dst["payment"] = pay
}

func populatePayPalRefundData(dst map[string]interface{}, res map[string]interface{}, forceCompleted bool) {
	if res == nil {
		return
	}
	rf := map[string]interface{}{}
	id := stringField(res, "id")
	if id != "" {
		rf["id"] = id
	}
	captureID := firstNonEmpty(
		stringField(res, "capture_id"),
		stringField(res, "payment_id"),
		extractPayPalCaptureIDFromLinks(res["links"]),
	)
	if captureID == "" && forceCompleted {
		captureID = id
	}
	if captureID != "" {
		rf["payment_id"] = captureID
	}
	st := stringField(res, "status")
	if forceCompleted || st == "" {
		st = "COMPLETED"
	}
	rf["status"] = st
	if am := parsePayPalMoneyMap(res["amount"]); am != nil {
		rf["amount_money"] = am
	}
	dst["refund"] = rf
}

func extractPayPalCaptureIDFromLinks(raw interface{}) string {
	links, ok := raw.([]interface{})
	if !ok {
		return ""
	}
	const marker = "/v2/payments/captures/"
	for _, item := range links {
		lm, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		rel := strings.ToLower(stringField(lm, "rel"))
		href := stringField(lm, "href")
		if rel != "up" && !strings.Contains(href, marker) {
			continue
		}
		if idx := strings.Index(href, marker); idx >= 0 {
			rest := href[idx+len(marker):]
			if slash := strings.IndexAny(rest, "/?#"); slash >= 0 {
				rest = rest[:slash]
			}
			if rest = strings.TrimSpace(rest); rest != "" {
				return rest
			}
		}
	}
	return ""
}

func populatePayPalDisputeData(dst map[string]interface{}, res map[string]interface{}) {
	if res == nil {
		return
	}
	dp := map[string]interface{}{}
	id := firstNonEmpty(stringField(res, "dispute_id"), stringField(res, "id"))
	if id != "" {
		dp["id"] = id
	}
	paymentID := firstNonEmpty(stringField(res, "payment_id"), extractPayPalDisputedCaptureID(res["disputed_transactions"]))
	if paymentID != "" {
		dp["payment_id"] = paymentID
	}
	dp["state"] = mapPayPalDisputeState(res)
	if ut := firstNonEmpty(stringField(res, "update_time"), stringField(res, "create_time")); ut != "" {
		dp["updated_at"] = ut
	}
	am := parsePayPalMoneyMap(res["dispute_amount"])
	if am == nil {
		am = parsePayPalMoneyMap(res["amount"])
	}
	if am != nil {
		dp["amount_money"] = am
	}
	dst["dispute"] = dp
}

func extractPayPalDisputedCaptureID(raw interface{}) string {
	txs, ok := raw.([]interface{})
	if !ok {
		return ""
	}
	for _, item := range txs {
		tm, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if id := stringField(tm, "seller_transaction_id"); id != "" {
			return id
		}
	}
	return ""
}

func mapPayPalDisputeState(res map[string]interface{}) string {
	stage := strings.ToUpper(stringField(res, "dispute_life_cycle_stage"))
	status := strings.ToUpper(stringField(res, "status"))
	outcome := ""
	if om, ok := res["dispute_outcome"].(map[string]interface{}); ok {
		outcome = strings.ToUpper(stringField(om, "outcome_code"))
	}
	if stage == "INQUIRY" {
		switch status {
		case "WAITING_FOR_SELLER_RESPONSE":
			return "INQUIRY_EVIDENCE_REQUIRED"
		case "RESOLVED":
			return "INQUIRY_CLOSED"
		default:
			return "INQUIRY_PROCESSING"
		}
	}
	if status == "RESOLVED" {
		switch outcome {
		case "RESOLVED_SELLER_FAVOUR", "CANCELED_BY_BUYER":
			return dispute.StateWon
		case "ACCEPTED":
			return "ACCEPTED"
		default:
			return "LOST"
		}
	}
	if status == "WAITING_FOR_SELLER_RESPONSE" {
		return "EVIDENCE_REQUIRED"
	}
	return "PROCESSING"
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
			if v := encodePayPalWebhookHeaders(h); v != "" {
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
			if name == "Paypal-Transmission-Sig" {
				return encodePayPalWebhookHeaders(h)
			}
			return v
		}
	}
	return ""
}

// isClawbackEvent reports whether an event returns a settled payment's money to the
// payer, or reports a dispute of it: a refund, or a dispute's creation or change of
// state (Square's dispute.state.updated, once named dispute.state.changed).
func isClawbackEvent(eventType string) bool {
	switch eventType {
	case "refund.created", "refund.updated", "dispute.created", "dispute.state.updated", "dispute.state.changed", "dispute.updated":
		return true
	default:
		return false
	}
}

// applyClawbackEvent takes a refunded or disputed payment's money back out of the
// wallet it credited, and gives a dispute's back when the merchant wins it. A payment
// that paid an invoice credited no wallet: the invoice records the refund, or every
// state the dispute reports, instead, and the period it paid for reads as its
// payment now stands.
//
// The org and the wallet are the ones the payment's own receipt names — found through
// the payment index (models/paymentorg), since the callback names only the payment —
// never the request's org. A refund acts once it has COMPLETED. A dispute moves money
// only while the processor withholds the funds (dispute.Withholds — a chargeback, or
// one lost or accepted; an inquiry holds none) and gives it back when it is WON. Every
// dispute report is ordered by the dispute's version, else its updated_at, and one
// older than the report already recorded changes nothing. The ledger posts each once
// per refund or dispute id, and never takes back more than the payment brought in.
func applyClawbackEvent(ctx context.Context, booked *datastore.Datastore, org *organization.Organization, event *processor.WebhookEvent) error {
	if event.Data == nil {
		return nil
	}
	if excess, _ := event.Data[excessRefund].(bool); excess {
		return nil // the invoice is paid in full: this only returned an overpayment
	}
	obj := unwrapObject(event.Data, "refund")
	paymentID := stringField(obj, "payment_id")
	isDispute := strings.HasPrefix(event.Type, "dispute.")
	state, version, at := "", int64(0), time.Time{}
	if isDispute {
		obj = unwrapObject(event.Data, "dispute")
		paymentID = stringField(obj, "payment_id")
		if paymentID == "" {
			if dp, ok := obj["disputed_payment"].(map[string]interface{}); ok {
				paymentID = stringField(dp, "payment_id")
			}
		}
		state = strings.ToUpper(stringField(obj, "state"))
		version, _ = numberField(obj, "version")
		at, _ = time.Parse(time.RFC3339, stringField(obj, "updated_at"))
	} else if st := stringField(obj, "status"); !strings.EqualFold(st, "COMPLETED") {
		return nil
	}
	id := stringField(obj, "id")
	amount, cur := settlementAmount(obj)
	// What moves wallet money: a refund, funds a dispute withholds, a dispute won.
	won := state == dispute.StateWon
	moves := won || (amount > 0 && (!isDispute || dispute.Withholds(state)))
	// What is recorded: a refund, and any state a dispute reports.
	records := (!isDispute && amount > 0) || state != ""
	if paymentID == "" || id == "" || !records {
		return nil
	}
	where, found, err := paymentorg.Get(paymentID)
	if err == nil && !found {
		// The index is the way back to the payment's books. Without it, the books
		// of the org whose account delivered this are searched for what the payment
		// wrote, and the index is written again from them (traced).
		where, found, err = traced(booked, org, paymentID)
	}
	if err != nil {
		return fmt.Errorf("%s %s: the payment index could not be read: %w", event.Type, id, err)
	}
	if !found {
		if moves {
			log.Error("RECONCILE: %s %s concerns payment %s, which no receipt recorded — settle it by hand", event.Type, id, paymentID)
		}
		return nil
	}
	if event.Org != "" && event.Org != where.Org {
		// An account that belongs to one org reports on its own payments only.
		log.Error("RECONCILE: %s %s from %s's account names payment %s, whose receipt is %s's — refused", event.Type, id, event.Org, paymentID, where.Org)
		return nil
	}
	db := datastore.New(nscontext.WithNamespace(ctx, where.Org))
	kind := "refund"
	if isDispute {
		kind = "dispute"
	}
	ref := kind + ":" + id

	// ONE CHANGE TO A PAYMENT AT A TIME. What a refund takes is decided from what
	// earlier ones took, and the credit of an order's starting balance from what
	// they all took, so each is read and written under the payment's lease: two
	// refunds delivered together are applied one after the other, and one that
	// finds it held past paymentWait answers an error, which the processor
	// redelivers.
	lease, held, err := holdPayment(db, paymentID)
	switch {
	case err != nil:
		return fmt.Errorf("%s %s on payment %s: the payment could not be held: %w", event.Type, id, paymentID, err)
	case !held:
		return fmt.Errorf("%s %s on payment %s: another change to the payment is being recorded", event.Type, id, paymentID)
	}
	defer lease.Release()
	if where, found, err = paymentorg.Get(paymentID); err != nil || !found {
		return fmt.Errorf("%s %s: payment %s could not be read again: %v", event.Type, id, paymentID, err)
	}
	if where.Sale && where.Invoice == "" {
		// A sale's payment paid an invoice, never a wallet: the record names it once
		// the sale wrote it, and until then it is read from the sale's books.
		if where.Invoice, err = paidInvoice(db, paymentID); err != nil {
			return fmt.Errorf("%s %s on payment %s: %w", event.Type, id, paymentID, err)
		}
	}
	if where.Balance > 0 {
		return takeOrder(ctx, db, where, event.Type, ref, id, paymentID, amount, cur, state, version, at)
	}
	if where.Sale && where.Invoice == "" {
		log.Error("RECONCILE: %s %s concerns payment %s, whose sale has not recorded its invoice yet; the delivery stays open until it does", event.Type, id, paymentID)
		return fmt.Errorf("%s %s on payment %s: the sale has not recorded its invoice yet", event.Type, id, paymentID)
	}
	if where.Invoice != "" {
		// A payment that paid an invoice credited no wallet: the invoice records
		// the refund, or every state a dispute reports.
		if isDispute {
			err = engine.RecordDispute(db, where.Invoice, state, version, at)
		} else {
			err = engine.RecordRefund(db, where.Invoice, id, int64(amount))
		}
		if err := invoiced(err, event.Type, id, paymentID, where); err != nil {
			return err
		}
		// A sale's payment that is refunded, or that a dispute withholds, stops the
		// automatic reload the sale switched on, as a reload's own payment does.
		if moves && !won {
			stopReload(ctx, where.Org, where.Subject, event.Type+" "+id)
		}
		return nil
	}
	if isDispute {
		latest, err := engine.RecordPaymentDispute(paymentID, state, version, at)
		if err != nil {
			return fmt.Errorf("%s %s on payment %s: the dispute's state was not recorded: %w", event.Type, id, paymentID, err)
		}
		if !latest {
			return nil // an earlier report than the one recorded moves nothing
		}
	}
	// A payment to a wallet that is refunded, or that a dispute withholds, stops
	// that wallet's automatic reload: its card is not charged off-session again
	// until the payer switches the rule back on.
	if moves && !won {
		stopReload(ctx, where.Org, where.Subject, event.Type+" "+id)
	}
	if !moves {
		return nil
	}
	// A payment paid in the buyer's currency against a quote was credited in dollars:
	// what is refunded or withheld in the buyer's currency is taken back as its
	// proportional share of that credit, never the foreign figure read as dollars, and
	// never more than the credit less what earlier ones took.
	take := int64(amount)
	if where.PaidCurrency != "" && where.PaidCents > 0 && strings.EqualFold(string(cur), where.PaidCurrency) {
		total, own, _, cerr := claimed(db, where.Subject, paymentID, ref)
		if cerr != nil {
			return fmt.Errorf("%s %s on payment %s: what earlier ones took could not be read: %w", event.Type, id, paymentID, cerr)
		}
		take = quotedShare(where.CreditCents, int64(amount), where.PaidCents)
		if own > 0 {
			take = own // this one was applied before: the redelivery posts what was recorded
		} else {
			take = min(take, where.CreditCents-total)
		}
		cur = currency.USD
		if take <= 0 && !won {
			return nil
		}
	}
	return clawback(ctx, db, creditledger.ClawbackInput{
		Org: where.Org, Subject: where.Subject, Payment: paymentID,
		Ref: ref, AmountCents: take, Reason: event.Type + " " + id, Test: where.Test,
	}, cur, won)
}

// quotedShare is the part of a credit that a refund of `refunded` takes back, when the
// payment was `paid` in the buyer's currency and credited `credit` in the wallet's:
// credit * refunded / paid, to the nearest cent, never more than the credit. A full
// refund is the whole credit exactly.
func quotedShare(credit, refunded, paid int64) int64 {
	if credit <= 0 || refunded <= 0 || paid <= 0 {
		return 0
	}
	if refunded >= paid {
		return credit
	}
	return min((credit*refunded+paid/2)/paid, credit)
}

// takeOrder applies a refund or a dispute to an organization's order: one payment
// that paid its seats' invoice and bought its starting balance. It is taken back
// from the starting balance FIRST, because that part is spendable, up to what the
// balance still holds (paymentorg.PaymentOrg.Left); what a refund returns beyond it
// is the invoice's, and a dispute is the whole payment's, so the invoice records
// every state it reports.
//
// What a refund or a dispute takes is recorded on the payment's record once, by its
// ref, whether or not the balance is credited yet, and in the same swap that reads
// whether the credit is decided (endow). Taken before, the credit nets against it
// and the wallet is not touched; taken after, it is taken out of the wallet, after
// landing the credit it takes from if that had not landed yet, so a take is never
// posted against money the ledger does not hold. A dispute won gives back what its
// loss took, the same way. The caller holds the payment's lease.
func takeOrder(ctx context.Context, db *datastore.Datastore, where *paymentorg.PaymentOrg, what, ref, id, paymentID string, amount currency.Cents, cur currency.Type, state string, version int64, at time.Time) error {
	isDispute := state != ""
	won := state == dispute.StateWon
	withholds := amount > 0 && (!isDispute || dispute.Withholds(state))
	if isDispute {
		if where.Invoice != "" {
			if err := invoiced(engine.RecordDispute(db, where.Invoice, state, version, at), what, id, paymentID, where); err != nil {
				return err
			}
		}
		latest, err := engine.RecordPaymentDispute(paymentID, state, version, at)
		if err != nil {
			return fmt.Errorf("%s %s on payment %s: the dispute's state was not recorded: %w", what, id, paymentID, err)
		}
		if !latest {
			return nil // an earlier report than the one recorded moves nothing
		}
	}
	var t paymentorg.Take
	po, _, err := paymentorg.Change(paymentID, func(r *paymentorg.PaymentOrg) bool {
		got := r.Took(ref)
		switch {
		case got != nil:
			t = *got
			if !won || got.Restored {
				return false
			}
			got.Restored = true
			return true
		case won || !withholds:
			t = paymentorg.Take{}
			return false
		}
		t = paymentorg.Take{Ref: ref, Cents: min(int64(amount), r.Left()), Moved: r.Credited}
		r.Takes = append(r.Takes, t)
		return true
	})
	if err != nil {
		return fmt.Errorf("%s %s on payment %s: what it takes of the starting balance was not recorded: %w", what, id, paymentID, err)
	}
	if !isDispute && int64(amount) > t.Cents {
		rest := int64(amount) - t.Cents
		if where.Invoice == "" {
			// The take is recorded; the rest waits for the sale to write its invoice,
			// and the processor delivers this again.
			return fmt.Errorf("%s %s returns %d of payment %s beyond its starting balance, and the sale has not recorded its invoice yet", what, id, rest, paymentID)
		}
		if err := invoiced(engine.RecordRefund(db, where.Invoice, id, rest), what, id, paymentID, where); err != nil {
			return err
		}
	}
	// A sale's payment that is refunded, or that a dispute withholds, stops the
	// automatic reload the sale switched on, as a reload's own payment does.
	if withholds && !won {
		stopReload(ctx, where.Org, where.Subject, what+" "+id)
	}
	if t.Cents <= 0 || !t.Moved {
		// Nothing taken, or taken while the credit was undecided: a refund is netted
		// from the credit and a dispute's take is moved when the credit is decided
		// (Decide), so the wallet holds none of it yet.
		return nil
	}
	if won {
		// What the loss took from the wallet is given back, once.
		return clawback(ctx, db, creditledger.ClawbackInput{
			Org: where.Org, Subject: where.Subject, Payment: paymentID,
			Ref: ref, AmountCents: t.Cents, Reason: what + " " + id, Test: where.Test,
		}, cur, true)
	}
	// Taken after the credit was decided: the credit lands first if it had not,
	// so the take has the money it takes back in both books, then every take
	// moved to the wallet is taken from it, this one included, once each.
	var payer *organization.Organization
	if po.Credit > 0 && po.Deposit == "" {
		if payer, err = orgNamed(ctx, where.Org); err != nil {
			return fmt.Errorf("%s %s on payment %s: %w", what, id, paymentID, err)
		}
	}
	if _, err := land(ctx, nil, payer, db, cur, po, paymentID); err != nil {
		return fmt.Errorf("%s %s on payment %s: what it takes was not taken from the wallet: %w", what, id, paymentID, err)
	}
	return nil
}

// paidInvoice is the invoice a sale's payment paid, read from the sale's books and
// written onto the payment's record; "" while the sale has not recorded it.
func paidInvoice(db *datastore.Datastore, paymentID string) (string, error) {
	inv := billinginvoice.New(db)
	found, err := inv.Query().Filter("PaymentRef=", paymentID).Get()
	if err != nil || !found {
		return "", err
	}
	if _, _, err := paymentorg.Change(paymentID, func(r *paymentorg.PaymentOrg) bool {
		if r.Invoice != "" {
			return false
		}
		r.Invoice = inv.Id()
		return true
	}); err != nil {
		return "", err
	}
	return inv.Id(), nil
}

// invoiced is what recording a refund or a dispute on the invoice a payment paid
// answered: nothing to do on success; a RECONCILE for an invoice these books do not
// hold, which redelivering cannot make appear; an error for anything else.
func invoiced(err error, what, id, paymentID string, where *paymentorg.PaymentOrg) error {
	switch {
	case errors.Is(err, datastore.ErrNoSuchEntity):
		log.Error("RECONCILE: %s %s on payment %s names invoice %s in %s, which does not exist — record it by hand", what, id, paymentID, where.Invoice, where.Org)
	case err != nil:
		return fmt.Errorf("%s %s on payment %s did not reach invoice %s in %s: %w", what, id, paymentID, where.Invoice, where.Org, err)
	}
	return nil
}

// clawback takes in.AmountCents back from the wallet a payment credited, or, for a
// dispute won, gives back what its loss took: commerce's own books first (clawLocal)
// — without a host ledger they are the balance, and with one they are commerce's
// record of the same money, as the settlement's local receipt is — then the host's.
func clawback(ctx context.Context, db *datastore.Datastore, in creditledger.ClawbackInput, cur currency.Type, won bool) error {
	if err := clawLocal(db, in, cur, won); err != nil {
		return fmt.Errorf("%s on payment %s for %s/%s was not recorded: %w", in.Reason, in.Payment, in.Org, in.Subject, err)
	}
	if led := creditledger.Get(); led != nil {
		var err error
		if won {
			_, err = led.Restore(ctx, in)
		} else {
			_, err = led.Clawback(ctx, in)
		}
		if err != nil {
			return fmt.Errorf("%s on payment %s for %s/%s did not post: %w", in.Reason, in.Payment, in.Org, in.Subject, err)
		}
	}
	return nil
}

// claimed is what refunds and disputes of payment have taken back from subject in
// commerce's own books: in all, and by ref alone, and whether ref's take was given
// back (a dispute won).
func claimed(db *datastore.Datastore, subject, payment, ref string) (total, own int64, restored bool, err error) {
	rows := make([]*transaction.Transaction, 0)
	if _, err := transaction.Query(db).Filter("Tags=", "clawback:"+payment).GetAll(&rows); err != nil {
		return 0, 0, false, err
	}
	for _, r := range rows {
		switch {
		case r.Type == transaction.Withdraw && r.SourceId == subject:
			total += int64(r.Amount)
			if r.Notes == ref {
				own = int64(r.Amount)
			}
		case r.Type == transaction.Deposit && r.DestinationId == subject:
			total -= int64(r.Amount)
			restored = restored || r.Notes == "restore "+ref
		}
	}
	return total, own, restored, nil
}

// clawLocal records in commerce's own books what a refund or a dispute (in.Ref)
// takes back from the wallet a payment credited: a withdrawal of in.AmountCents,
// never more than the payment's deposits there less what earlier ones took, once
// per ref; or, for a dispute won, the deposit of what its loss took, once. Without
// a host ledger these books are the balance, so a refunded payment no longer stays
// spendable in them.
func clawLocal(db *datastore.Datastore, in creditledger.ClawbackInput, cur currency.Type, won bool) error {
	total, own, restored, err := claimed(db, in.Subject, in.Payment, in.Ref)
	if err != nil {
		return err
	}
	t := transaction.New(db)
	t.Currency, t.Tags, t.Test = cur, "clawback:"+in.Payment, in.Test
	if won {
		if own == 0 || restored {
			return nil
		}
		t.Type, t.DestinationId, t.DestinationKind = transaction.Deposit, in.Subject, transaction.IAMUserKind
		t.Amount, t.Notes = currency.Cents(own), "restore "+in.Ref
		// Giving back what these books took is not new money.
		t.SetContext(mintauth.WithAuthorized(t.Context()))
		return t.Create()
	}
	if own > 0 {
		return nil // this refund or dispute already took its part
	}
	deps := make([]*transaction.Transaction, 0)
	if _, err := transaction.Query(db).Filter("SourceId=", in.Payment).Filter("DestinationId=", in.Subject).GetAll(&deps); err != nil {
		return err
	}
	paid := int64(0)
	for _, d := range deps {
		if d.Type == transaction.Deposit {
			paid += int64(d.Amount)
		}
	}
	amount := min(in.AmountCents, paid-total)
	if amount <= 0 {
		return nil
	}
	t.Type, t.SourceId, t.SourceKind = transaction.Withdraw, in.Subject, transaction.IAMUserKind
	t.Amount, t.Notes = currency.Cents(amount), in.Ref
	return t.Create()
}

// traced finds a payment the index does not name in the books of the org whose
// account delivered its event — the invoice it paid, the deposit it made — and
// indexes it again from what they say: an order's payment did both, and the
// deposit is its credited starting balance. Nothing found answers found=false.
func traced(db *datastore.Datastore, org *organization.Organization, paymentID string) (*paymentorg.PaymentOrg, bool, error) {
	if db == nil || org == nil {
		return nil, false, nil
	}
	inv := billinginvoice.New(db)
	paid, err := inv.Query().Filter("PaymentRef=", paymentID).Get()
	if err != nil {
		return nil, false, err
	}
	dep := transaction.New(db)
	held, err := dep.Query().Filter("SourceId=", paymentID).Filter("Type=", transaction.Deposit).Get()
	if err != nil {
		return nil, false, err
	}
	switch {
	case paid && held && dep.DestinationId == inv.UserId:
		if err := paymentorg.Open(paymentID, org.Name, inv.UserId, "", strings.TrimSuffix(dep.SourceKind, "-payment"), int64(dep.Amount), org.TestMode()); err != nil {
			return nil, false, err
		}
		if _, _, err := paymentorg.Change(paymentID, func(r *paymentorg.PaymentOrg) bool {
			r.Invoice, r.Credited, r.Credit, r.Deposit = inv.Id(), true, int64(dep.Amount), dep.Id()
			return true
		}); err != nil {
			return nil, false, err
		}
	case paid:
		err = paymentorg.Put(paymentID, org.Name, inv.UserId, inv.Id(), 0, org.TestMode())
	case held:
		err = paymentorg.Put(paymentID, org.Name, dep.DestinationId, "", 0, org.TestMode())
	default:
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	log.Warn("payment %s: the index was missing and was written again from %s's books", paymentID, org.Name)
	return paymentorg.Get(paymentID)
}
