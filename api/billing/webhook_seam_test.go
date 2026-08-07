package billing

// webhook_seam_test.go is the money-in half of the provider callback: WHICH WALLET a
// settlement funds in the ONE ledger the spend gate reads, and — more often — which
// ones it must refuse to fund.
//
// The callback used to write only into commerce's own transaction store, which
// nothing in the embedded binary spends from, so an asynchronously-settled payment
// moved real money and the balance that decides whether a request is served never
// changed. It also resolved the payer from the provider's payload, where no field
// names a wallet in our books: `reference_id` is an ORDER id wherever production sets
// it, `customer_id` is the provider's own customer. Both were being written as an
// "iam-user" destination — the gateway-spendable wallet.
//
// So these pin the two halves that have to hold together:
//
//	CREDIT ONCE, AT THE RIGHT ADDRESS — a settlement whose payer is resolvable from
//	what commerce WROTE reaches the seam at that payer's (org, subject, test), and a
//	redelivery does not credit it again.
//
//	REFUSE EVERYTHING ELSE — an in-session charge (its own door already credited the
//	spendable ledger), an invoice already settled, and above all a payment nobody in
//	these books can be tied to. A wrong credit is not a small error here: it is one
//	customer's money in another's wallet, reported as success.

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/transaction"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/util/test/ae"
)

// renewalEvent builds a signed invoice.paid delivery for an externally-billed
// subscription: the invoice is the settled object and it names the subscription that
// owns it, which is the only path from a callback to a wallet in our books.
func renewalEvent(eventID, invoiceID, subID string, amountCents int64) []byte {
	return []byte(fmt.Sprintf(
		`{"merchant_id":"M1","type":"invoice.paid","event_id":%q,"created_at":%q,`+
			`"data":{"type":"invoice","id":%q,"object":{"id":%q,"subscription_id":%q,`+
			`"amount_money":{"amount":%d,"currency":"USD"}}}}`,
		eventID, time.Now().UTC().Format(time.RFC3339), invoiceID, invoiceID, subID, amountCents,
	))
}

// seedProviderSubscription writes the subscription an external provider bills — the
// one record that ties a provider's subscription id to a billing subject in our
// books. subject is the wallet key the spend gate debits: the org slug, or the finer
// "<org>/<user>" of a member who spends from their own.
func seedProviderSubscription(t *testing.T, org *organization.Organization, ctx context.Context, providerID, subject string) {
	t.Helper()
	db := datastore.New(org.Namespaced(ctx))
	sub := subscription.New(db)
	sub.ProviderId = providerID
	sub.ProviderType = "square"
	sub.UserId = subject
	sub.Status = subscription.Active
	sub.MustCreate()
}

// seedInternalSubscription writes a subscription as commerce's OWN doors create one:
// ProviderType internal/bundle and ProviderId UNSET. Every production subscription is
// this shape — no call site anywhere sets ProviderId.
func seedInternalSubscription(t *testing.T, org *organization.Organization, ctx context.Context, subject string) {
	t.Helper()
	db := datastore.New(org.Namespaced(ctx))
	sub := subscription.New(db)
	sub.ProviderType = "internal"
	sub.UserId = subject
	sub.Status = subscription.Active
	sub.MustCreate()
}

// seedInSessionCharge writes the ledger row a card door writes when it takes a
// payment — stamped with the processor's own reference for that charge, which is
// exactly what the door does at charge time (payment_core.go, topup.go).
func seedInSessionCharge(t *testing.T, org *organization.Organization, ctx context.Context, subject, paymentID string, cents int64) {
	t.Helper()
	db := datastore.New(org.Namespaced(ctx))
	tr := transaction.New(db)
	tr.Type = transaction.Deposit
	tr.DestinationId = subject
	tr.DestinationKind = transaction.IAMUserKind
	tr.Currency = currency.USD
	tr.Amount = currency.Cents(cents)
	tr.Tags = "topup"
	tr.SourceKind = "square-payment"
	tr.SourceId = paymentID
	tr.Test = org.TestMode()
	tr.MustCreate()
}

// seedPaidInvoice writes the invoice a collection settles: marked paid, carrying the
// processor's reference for the charge that paid it. That is what CollectInvoice and
// the first-invoice-on-subscribe path both write (engine MarkPaid).
func seedPaidInvoice(t *testing.T, org *organization.Organization, ctx context.Context, subject, paymentID string, cents int64) {
	t.Helper()
	db := datastore.New(org.Namespaced(ctx))
	inv := billinginvoice.New(db)
	inv.UserId = subject
	inv.Currency = currency.USD
	inv.AmountDue = cents
	inv.Status = billinginvoice.Open
	if err := inv.MarkPaid("card", paymentID); err != nil {
		t.Fatalf("mark invoice paid: %v", err)
	}
	if err := inv.Create(); err != nil {
		t.Fatalf("seed paid invoice: %v", err)
	}
}

// webhookOrg PERSISTS the org a sessionless callback will resolve, and returns the
// same record the handler will read.
//
// A callback carries no session, so its org comes from the STORE — and the store is
// now also where its test-ness comes from, since the resolver stopped overwriting the
// record with Live. An in-memory org built by the test is therefore not the org the
// handler sees: GetOrCreate would mint a fresh one, defaulting to sandbox, and the
// test would assert against a different record than the one that decided the books.
func webhookOrg(t *testing.T, ctx context.Context, name string, live bool) *organization.Organization {
	t.Helper()
	o := organization.New(datastore.New(ctx))
	o.Name = name
	if err := o.GetOrCreate("Name=", name); err != nil {
		t.Fatalf("seed org %q: %v", name, err)
	}
	o.Live = live
	o.MustUpdate()
	return o
}

// (i) A pure-async settlement credits the SEAM — the ledger the spend gate reads —
// at the subscription's real (org, subject), in the books the org transacts in, and
// exactly once however many times the provider redelivers it.
//
// This is the whole money-in gap in one test: before, this payment credited commerce's
// own store at a wallet named after a Square customer, and the gate's balance never
// moved.
func TestWebhookSeam_AsyncSettlementCreditsTheSpendableLedger(t *testing.T) {
	const secret = "whsec_seam_async"
	registerSquare(t, secret)
	fake := newFakeLedger()
	injectLedger(t, fake)

	ctx := ae.NewContext()
	defer ctx.Close()
	org := webhookOrg(t, ctx, "seam-async", true) // Live ⇒ the live books
	seedProviderSubscription(t, org, ctx, "sub_async", "seam-async/alice")

	for i := 0; i < 3; i++ {
		body := renewalEvent(fmt.Sprintf("evt_async_%d", i), "inv_async", "sub_async", 7500)
		if r := deliverWebhook(ctx, "seam-async", secret, body, ""); r.StatusCode != http.StatusOK {
			t.Fatalf("delivery %d: status=%d", i+1, r.StatusCode)
		}
	}

	posted := fake.credits()
	if len(posted) != 1 {
		t.Fatalf("the seam took %d credits for ONE settlement, want 1 — a redelivered payment was credited again: %+v", len(posted), posted)
	}
	got := posted[0]
	if got.Org != "seam-async" {
		t.Errorf("CreditInput.Org=%q, want seam-async (the ledger holding the payer's wallet)", got.Org)
	}
	if got.Subject != "seam-async/alice" {
		t.Errorf("CreditInput.Subject=%q, want seam-async/alice — the subscription's own owner, which is the key the spend gate debits", got.Subject)
	}
	if got.Test {
		t.Errorf("CreditInput.Test=true for a LIVE org — a real payment was parked in books no gate reads")
	}
	if got.AmountCents != 7500 {
		t.Errorf("CreditInput.AmountCents=%d, want 7500", got.AmountCents)
	}
	if got.IdempotencyKey != "inv_async" {
		t.Errorf("CreditInput.IdempotencyKey=%q, want inv_async — the settlement's own id, which is what makes every path credit it once", got.IdempotencyKey)
	}
}

// (ii) A settlement this callback ALREADY credited is not credited again — rule 1,
// exercised where it can actually change the answer.
//
// It drives invoice.paid, because that is the only event type that reaches a credit
// at all: a payment.* delivery is refused by the event-type gate whatever rules 1 and
// 2 say, so asserting "no credit" on one proves nothing about them. Here a resolvable
// subscription IS present, so the ONLY thing standing between this delivery and a
// second credit is the stamped receipt the first delivery wrote.
func TestWebhookSeam_AnAlreadyCreditedSettlementIsNotCreditedTwice(t *testing.T) {
	const secret = "whsec_seam_insession"
	registerSquare(t, secret)
	fake := newFakeLedger()
	injectLedger(t, fake)

	ctx := ae.NewContext()
	defer ctx.Close()
	org := webhookOrg(t, ctx, "seam-door", true)
	seedProviderSubscription(t, org, ctx, "sub_door", "seam-door/alice")
	// The receipt a previous delivery (or a card door) wrote for this settlement.
	seedInSessionCharge(t, org, ctx, "seam-door/alice", "inv_door", 4200)

	body := renewalEvent("evt_door", "inv_door", "sub_door", 4200)
	if r := deliverWebhook(ctx, "seam-door", secret, body, ""); r.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, want 200 (the event is valid and recorded; it just must not credit)", r.StatusCode)
	}

	if n := len(fake.credits()); n != 0 {
		t.Fatalf("the seam took %d credits for a settlement already carrying a receipt, want 0 — the customer was credited twice for one payment: %+v", n, fake.credits())
	}
	if got := balanceOf(t, ctx, org, "seam-door/alice"); got != 4200 {
		t.Fatalf("local balance=%d, want 4200 (the single existing receipt)", got)
	}
}

// (iii) A settlement whose payer cannot be named REFUSES, and refuses ALL THE WAY:
// no seam credit, no local row, and above all nothing credited to the service org
// whose token happened to resolve the namespace.
//
// This is the shape production actually delivers — a payment link, a charge raised in
// the provider's dashboard, a checkout capture — and it is the one the old code was
// most wrong about. `reference_id` here is an order id and `customer_id` is a Square
// customer; both used to become an "iam-user" wallet holding real money.
func TestWebhookSeam_UnresolvablePayerRefuses(t *testing.T) {
	const secret = "whsec_seam_unknown"
	registerSquare(t, secret)
	fake := newFakeLedger()
	injectLedger(t, fake)

	ctx := ae.NewContext()
	defer ctx.Close()
	org := webhookOrg(t, ctx, "seam-unknown", true)

	// reference_id carries what checkout puts there: an ORDER id, not a wallet.
	body := settlementEvent("evt_unknown", "pay_unknown", "COMPLETED", "ord_9f3c", 90000)
	if r := deliverWebhook(ctx, "seam-unknown", secret, body, ""); r.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, want 200 — the delivery is valid and recorded, it just credits nothing", r.StatusCode)
	}

	if n := len(fake.credits()); n != 0 {
		t.Fatalf("the seam took %d credits for a payment with no resolvable payer, want 0 — money landed in a guessed wallet: %+v", n, fake.credits())
	}
	for _, guess := range []string{"ord_9f3c", "seam-unknown", "cust_seam-unknown"} {
		if got := balanceOf(t, ctx, org, guess); got != 0 {
			t.Errorf("balance of %q = %d, want 0 — an unattributable settlement minted balance at a guessed address", guess, got)
		}
	}
}

// (iv) A SANDBOX org's settlement funds the SANDBOX books, and the live ones stay at
// zero.
//
// Two things had to be true at once for this to hold, and neither was. The seam could
// not carry which books to write, so every credit through it landed live; and the
// callback's own org resolver overwrote the org record with Live — on the strength of
// a deployment-wide override that no longer exists — so a sandbox merchant's
// settlement was stamped live before the seam ever saw it. Together they made a Square
// SANDBOX callback mint balance that buys real inference.
func TestWebhookSeam_SandboxSettlementFundsSandboxBooks(t *testing.T) {
	const secret = "whsec_seam_sandbox"
	registerSquare(t, secret)
	fake := newFakeLedger()
	injectLedger(t, fake)

	ctx := ae.NewContext()
	defer ctx.Close()
	org := webhookOrg(t, ctx, "seam-sandbox", false)
	seedProviderSubscription(t, org, ctx, "sub_sandbox", "seam-sandbox/alice")

	body := renewalEvent("evt_sandbox", "inv_sandbox", "sub_sandbox", 3300)
	if r := deliverWebhook(ctx, "seam-sandbox", secret, body, ""); r.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, want 200", r.StatusCode)
	}

	posted := fake.credits()
	if len(posted) != 1 {
		t.Fatalf("seam credits=%d, want 1: %+v", len(posted), posted)
	}
	if !posted[0].Test {
		t.Fatalf("CreditInput.Test=false for a SANDBOX org — sandbox money was minted into the books the spend gate buys real inference from")
	}

	// The sandbox books hold it and the live books are empty — the read repeats the
	// whole address, so a wrong bucket cannot hide behind a right total.
	if bal, _ := fake.Balance(context.Background(), "seam-sandbox", "seam-sandbox/alice", "usd", true); bal != 3300 {
		t.Errorf("sandbox balance=%d, want 3300", bal)
	}
	if bal, _ := fake.Balance(context.Background(), "seam-sandbox", "seam-sandbox/alice", "usd", false); bal != 0 {
		t.Errorf("LIVE balance=%d, want 0 — a sandbox charge funded spendable money", bal)
	}
}

// ─── settlementDestination, asked directly ─────────────────────────────────
//
// The resolver answers three different things and the handler collapses two of them
// into "no credit", so driving HTTP cannot tell them apart. The difference is the
// whole operational value of rules 1 and 2: an ACCOUNTED-FOR payment is silent, and
// an UNATTRIBUTABLE one raises a RECONCILE alarm. An alarm that also fires on every
// successful subscription renewal is an alarm somebody mutes, and then the real one
// is missed too.

// verdict names which of the three answers the resolver gave.
type verdict string

const (
	credited      verdict = "credit"          // an address: this payment funds a wallet
	accountedFor  verdict = "silent-skip"     // (nil, ""): another path owns the credit
	unattributabl verdict = "reconcile-alarm" // (nil, why): refused and reported
)

func classify(d *destination, why string) verdict {
	switch {
	case d != nil:
		return credited
	case why == "":
		return accountedFor
	default:
		return unattributabl
	}
}

// resolve asks the resolver the way the handler does, with the object already
// unwrapped.
func resolve(t *testing.T, org *organization.Organization, ctx context.Context, eventType string, object map[string]interface{}) (*destination, string) {
	t.Helper()
	db := datastore.New(org.Namespaced(ctx))
	pay := unwrapObject(object, "payment")
	return settlementDestination(db, org,
		&processor.WebhookEvent{Type: eventType, Processor: processor.Square, Data: object},
		pay, "square-payment", stringField(pay, "id"))
}

// RULE 1, alone: a stamped receipt makes a settlement ACCOUNTED FOR — silent, not an
// alarm. This is what the in-session card doors buy: their charge already credited
// the spendable ledger through the host, so the callback must stand down without
// reporting a reconciliation nobody needs to do.
func TestSettlementDestination_AStampedChargeIsAccountedForAndSilent(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := webhookOrg(t, ctx, "res-stamped", true)
	seedInSessionCharge(t, org, ctx, "res-stamped/alice", "pay_door", 4200)

	d, why := resolve(t, org, ctx, "payment.updated", map[string]interface{}{
		"id": "pay_door", "status": "COMPLETED",
	})
	if got := classify(d, why); got != accountedFor {
		t.Fatalf("verdict=%s (why=%q), want %s — an in-session charge either got credited again or raised a false alarm", got, why, accountedFor)
	}
}

// RULE 2, alone: a charge that PAID AN INVOICE is accounted for too. The money bought
// a plan; the collection waterfall already recorded it against the invoice.
//
// This is the renewal callback, which is the case rule 2 exists for: without it every
// successful subscription renewal raises a RECONCILE alarm.
func TestSettlementDestination_AnInvoiceCollectionIsAccountedForAndSilent(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := webhookOrg(t, ctx, "res-invoice", true)
	seedPaidInvoice(t, org, ctx, "res-invoice/alice", "pay_collected", 1500)

	d, why := resolve(t, org, ctx, "payment.updated", map[string]interface{}{
		"id": "pay_collected", "status": "COMPLETED",
	})
	if got := classify(d, why); got != accountedFor {
		t.Fatalf("verdict=%s (why=%q), want %s — a settled invoice collection was credited as prepaid balance, or raised an alarm on a renewal that worked", got, why, accountedFor)
	}
}

// RULE 3, alone: an externally-billed subscription resolves to ITS OWN owner.
func TestSettlementDestination_ASubscriptionResolvesItsOwner(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := webhookOrg(t, ctx, "res-sub", true)
	seedProviderSubscription(t, org, ctx, "sub_ext", "res-sub/alice")

	d, why := resolve(t, org, ctx, "invoice.paid", map[string]interface{}{
		"id": "inv_1", "subscription_id": "sub_ext",
	})
	if got := classify(d, why); got != credited {
		t.Fatalf("verdict=%s (why=%q), want %s", got, why, credited)
	}
	if d.subject != "res-sub/alice" || d.org != "res-sub" {
		t.Fatalf("resolved (%q,%q), want (res-sub, res-sub/alice)", d.org, d.subject)
	}
}

// THE WILDCARD. A lifecycle delivery that names NO subscription must REFUSE.
//
// `Filter("ProviderId=", "")` is not a lookup that misses — it MATCHES, because no
// production call site ever sets ProviderId, so every subscription commerce created
// carries "". The first one found would be credited for whatever the payload asked.
// The emptier the payload, the more it matches.
//
// The delivery below is the reachable shape: the settled object nested one level
// down, so the payment id reads fine while nothing at the level the subscription link
// was read from says anything at all.
func TestSettlementDestination_ADeliveryNamingNoSubscriptionRefuses(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := webhookOrg(t, ctx, "res-wild", true)
	// An INTERNAL subscription: exactly what production has, ProviderId unset.
	seedInternalSubscription(t, org, ctx, "res-wild/victim-treasury")

	d, why := resolve(t, org, ctx, "invoice.paid", map[string]interface{}{
		"payment": map[string]interface{}{"id": "pay_wild", "status": "COMPLETED"},
	})
	if got := classify(d, why); got != unattributabl {
		t.Fatalf("verdict=%s, want %s — a blank subscription key matched an unrelated internal subscription and credited %q", got, unattributabl, func() string {
			if d != nil {
				return d.subject
			}
			return ""
		}())
	}
}

// AN INVOICE ID IS NOT A SUBSCRIPTION ID, even when one happens to equal the other.
//
// The object's own "id" used to be a general fallback for the subscription key, and on
// an invoice.paid that id is the INVOICE's. Normally it simply misses. It stops missing
// the moment some subscription's ProviderId collides with it — and then a delivery that
// names no subscription at all resolves to one, and credits its owner. The key is read
// from an explicit link, or the delivery is refused.
func TestSettlementDestination_AnInvoiceIdIsNotASubscriptionKey(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := webhookOrg(t, ctx, "res-collide", true)
	// A subscription whose provider key collides with the invoice id below.
	seedProviderSubscription(t, org, ctx, "inv_collide", "res-collide/victim")

	d, why := resolve(t, org, ctx, "invoice.paid", map[string]interface{}{
		"id": "inv_collide", // the INVOICE's id, and no subscription link anywhere
	})
	if got := classify(d, why); got != unattributabl {
		t.Fatalf("verdict=%s, want %s — an invoice id was accepted as a subscription key and credited %q", got, unattributabl, func() string {
			if d != nil {
				return d.subject
			}
			return ""
		}())
	}
}

// THE SPLICE: a subscription link sitting BESIDE the settled object, not inside it.
//
// This is why the resolver is handed the unwrapped `pay` map rather than re-deriving
// the key from the envelope. Read the subscription link off the ENVELOPE and the two
// values stop describing one payment: the amount and the payment id come from the
// object nested inside, while the payer comes from a sibling key an attacker put
// there. Splice a victim's subscription id next to your own payment object and the
// victim's wallet is credited for your amount.
//
// The envelope here is `data.object` — the processor passes exactly that through as
// event.Data (thirdparty/square/processor.go, Data: evt.Data.Object) — so the splice
// goes INSIDE object, beside "payment". A key at the `data` level never survives.
//
// One map, read once, is the whole defence, and this test is what keeps it that way.
func TestSettlementDestination_ASubscriptionSplicedBesideTheObjectRefuses(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := webhookOrg(t, ctx, "res-splice", true)
	seedProviderSubscription(t, org, ctx, "sub_victim", "res-splice/victim")

	d, why := resolve(t, org, ctx, "invoice.paid", map[string]interface{}{
		"subscription_id": "sub_victim", // spliced beside the object, not into it
		"payment": map[string]interface{}{
			"id": "p2", "status": "COMPLETED",
			"amount_money": map[string]interface{}{"amount": float64(999999), "currency": "USD"},
		},
	})
	if got := classify(d, why); got != unattributabl {
		t.Fatalf("verdict=%s, want %s — a subscription id spliced beside the settled object resolved to %q: the payer and the payment came from different maps", got, unattributabl, func() string {
			if d != nil {
				return d.subject
			}
			return ""
		}())
	}
}

// And the same splice through the whole handler, with the money on the wire.
func TestWebhookSeam_ASplicedSubscriptionMintsNothing(t *testing.T) {
	const secret = "whsec_seam_splice"
	registerSquare(t, secret)
	fake := newFakeLedger()
	injectLedger(t, fake)

	ctx := ae.NewContext()
	defer ctx.Close()
	org := webhookOrg(t, ctx, "seam-splice", true)
	seedProviderSubscription(t, org, ctx, "sub_victim", "seam-splice/victim")

	body := []byte(fmt.Sprintf(
		`{"merchant_id":"M1","type":"invoice.paid","event_id":"evt_splice","created_at":%q,`+
			`"data":{"type":"invoice","id":"inv_splice","object":{"subscription_id":"sub_victim",`+
			`"payment":{"id":"p2","status":"COMPLETED",`+
			`"amount_money":{"amount":999999,"currency":"USD"}}}}}`,
		time.Now().UTC().Format(time.RFC3339)))

	if r := deliverWebhook(ctx, "seam-splice", secret, body, ""); r.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, want 200", r.StatusCode)
	}
	if n := len(fake.credits()); n != 0 {
		t.Fatalf("a spliced subscription link minted %d credits: %+v", n, fake.credits())
	}
	if got := balanceOf(t, ctx, org, "seam-splice/victim"); got != 0 {
		t.Fatalf("victim balance=%d, want 0 — a spliced envelope key credited an unrelated wallet", got)
	}
}

// And the same hole through the whole handler, with money on the line: the amount is
// the attacker's, the wallet is a stranger's, and nothing may move.
func TestWebhookSeam_ADeliveryNamingNoSubscriptionMintsNothing(t *testing.T) {
	const secret = "whsec_seam_wild"
	registerSquare(t, secret)
	fake := newFakeLedger()
	injectLedger(t, fake)

	ctx := ae.NewContext()
	defer ctx.Close()
	org := webhookOrg(t, ctx, "seam-wild", true)
	seedInternalSubscription(t, org, ctx, "seam-wild/victim-treasury")

	body := []byte(fmt.Sprintf(
		`{"merchant_id":"M1","type":"invoice.paid","event_id":"evt_wild","created_at":%q,`+
			`"data":{"type":"invoice","id":"inv_wild","object":{"payment":{"id":"pay_wild",`+
			`"status":"COMPLETED","amount_money":{"amount":999999,"currency":"USD"}}}}}`,
		time.Now().UTC().Format(time.RFC3339)))

	if r := deliverWebhook(ctx, "seam-wild", secret, body, ""); r.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, want 200", r.StatusCode)
	}
	if n := len(fake.credits()); n != 0 {
		t.Fatalf("a delivery naming no subscription minted %d credits: %+v", n, fake.credits())
	}
	if got := balanceOf(t, ctx, org, "seam-wild/victim-treasury"); got != 0 {
		t.Fatalf("victim balance=%d, want 0 — an unrelated wallet was credited from a blank key", got)
	}
}
