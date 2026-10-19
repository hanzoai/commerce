package billing

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/billing/creditledger"
	"github.com/hanzoai/commerce/checkout"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billingevent"
	"github.com/hanzoai/commerce/models/idempotencykey"
	"github.com/hanzoai/commerce/models/paymentorg"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
	squareprovider "github.com/hanzoai/commerce/payment/providers/square"
	"github.com/hanzoai/commerce/util/test/ae"

	. "github.com/hanzoai/commerce/types"
)

// Square nests the changed resource under data.object.payment. The processor
// passes data.object through as the event's Data map, so the settlement parser
// must unwrap the "payment" key before reading id/status/amount.
func TestSettlementParsing_SquareNestedPayment(t *testing.T) {
	// Shape of a real Square payment.updated event's data.object.
	data := Map{
		"payment": map[string]interface{}{
			"id":           "pay_ABC123",
			"status":       "COMPLETED",
			"reference_id": "user-42",
			"amount_money": map[string]interface{}{
				"amount":   float64(2500),
				"currency": "USD",
			},
		},
	}

	pay := unwrapObject(data, "payment")
	if got := stringField(pay, "id"); got != "pay_ABC123" {
		t.Fatalf("payment id = %q, want pay_ABC123 (unwrap failed)", got)
	}
	if got := stringField(pay, "status"); got != "COMPLETED" {
		t.Fatalf("status = %q, want COMPLETED", got)
	}
	if got := stringField(pay, "reference_id"); got != "user-42" {
		t.Fatalf("reference_id = %q, want user-42", got)
	}

	amount, cur := settlementAmount(pay)
	if amount != currency.Cents(2500) {
		t.Fatalf("amount = %d, want 2500", amount)
	}
	if cur != currency.USD {
		t.Fatalf("currency = %q, want usd", cur)
	}
}

// Processors that expose fields at the top level (no nesting) must still parse.
func TestSettlementParsing_FlatFallback(t *testing.T) {
	data := Map{
		"id":       "pay_FLAT",
		"status":   "completed",
		"currency": "eur",
		"amount":   float64(199),
	}
	pay := unwrapObject(data, "payment") // no "payment" key -> returns data itself
	if got := stringField(pay, "id"); got != "pay_FLAT" {
		t.Fatalf("flat id = %q, want pay_FLAT", got)
	}
	amount, cur := settlementAmount(pay)
	if amount != currency.Cents(199) || cur != currency.Type("eur") {
		t.Fatalf("flat amount/cur = %d/%q, want 199/eur", amount, cur)
	}
}

func TestIsSettlementEvent(t *testing.T) {
	settling := []string{"payment.completed", "payment.updated", "invoice.paid"}
	for _, ty := range settling {
		if !isSettlementEvent(ty) {
			t.Errorf("%s should be a settlement event", ty)
		}
	}
	for _, ty := range []string{"payment.created", "refund.created", "dispute.created", ""} {
		if isSettlementEvent(ty) {
			t.Errorf("%s should NOT be a settlement event", ty)
		}
	}
}

// The Square signature header must be picked up by the ingress.
func TestPickSignatureHeader_Square(t *testing.T) {
	h := map[string][]string{"X-Square-Hmacsha256-Signature": {"sig123"}}
	if got := pickSignatureHeader(h, "square"); got != "sig123" {
		t.Fatalf("square signature header not picked: %q", got)
	}
	// Also found without a hint (header-name fallback).
	if got := pickSignatureHeader(h, ""); got != "sig123" {
		t.Fatalf("square signature header not picked without hint: %q", got)
	}
}

// ─── HandleProviderWebhook, end to end ──────────────────────────────────────
//
// The tests above exercise pure parsing helpers. These drive the REAL webhook
// handler — signature verification, org resolution, event de-duplication and
// the ledger write — because that whole path had no coverage on the ONE branch
// in the codebase that mints balance from an inbound callback.
//
// What they pin:
//
//	APPROVED           → NO credit (authorized funds are not captured funds)
//	COMPLETED          → credit
//	same event twice   → ONE credit (provider retries are relentless)
//	unsigned / forged  → 401 AND no ledger write

// settlementEvent builds a signed Square payment.updated delivery crediting
// subject with amountCents at the given payment status.
func settlementEvent(eventID, paymentID, status, subject string, amountCents int64) []byte {
	return []byte(fmt.Sprintf(
		`{"merchant_id":"M1","type":"payment.updated","event_id":%q,"created_at":%q,`+
			`"data":{"type":"payment","id":%q,"object":{"payment":{"id":%q,"status":%q,`+
			`"reference_id":%q,"amount_money":{"amount":%d,"currency":"USD"}}}}}`,
		eventID, time.Now().UTC().Format(time.RFC3339), paymentID, paymentID, status,
		subject, amountCents,
	))
}

// deliverWebhook posts body to the real handler, signed with secret unless
// rawSig is supplied (the forged/unsigned cases), as a delivery from the Square
// account orgName owns ("" is the deployment's own). The org is a property of
// the account that verifies the delivery — what a provider sends carries none —
// so the verifier registered for the call names it.
func deliverWebhook(ctx ae.Context, orgName, secret string, body []byte, rawSig string) *http.Response {
	sig := rawSig
	if sig == "" {
		sig = squareSign(secret, testWebhookURL, body)
	}
	prev, _ := processor.Global().Registered(processor.Square)
	processor.Global().Register(squareprovider.NewProvider(squareprovider.Config{
		AccessToken:         "test-token",
		LocationID:          "L1",
		WebhookSignatureKey: secret,
		WebhookURL:          testWebhookURL,
		Environment:         "sandbox",
		Org:                 orgName,
	}))
	if prev != nil {
		defer processor.Global().Register(prev)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/commerce/webhooks/square", strings.NewReader(string(body)))
	req.Header.Set(squareSigHeader, sig)
	return driveSeeded(func(c *zip.Ctx) {
		c.SetContext(ctx)
	}, "/v1/commerce/webhooks/:provider", req, HandleProviderWebhook)
}

// THE hole: an APPROVED authorization is reserved, not captured. It can still be
// voided or left to expire, so crediting it mints balance against money that may
// never arrive — and the charge path (processor.Settled) already refused it, so
// the two planes disagreed about what counts as money.
func TestWebhookSettlement_ApprovedDoesNotCredit(t *testing.T) {
	const secret = "whsec_approved"
	registerSquare(t, secret)
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("wh-approved")

	body := settlementEvent("evt_appr", "pay_appr", "APPROVED", "wh-approved", 5000)
	resp := deliverWebhook(ctx, "wh-approved", secret, body, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, want 200 (the event is valid and recorded; it just must not credit)", resp.StatusCode)
	}
	if got := balanceOf(t, ctx, org, "wh-approved"); got != 0 {
		t.Fatalf("balance=%d after an APPROVED authorization, want 0 — balance was minted from UNCAPTURED funds", got)
	}
}

// A COMPLETED capture whose payer this process CAN name credits that payer.
//
// It is driven through a subscription because that is the only shape of settled
// payment a callback can attribute: the payment object itself names no wallet in our
// books. This test used to pass a subject in `reference_id` and assert it was
// credited, which is what let the handler treat an ORDER id — the only thing
// production ever puts there — as a spendable wallet.
func TestWebhookSettlement_CompletedCredits(t *testing.T) {
	const secret = "whsec_completed"
	registerSquare(t, secret)
	ctx := ae.NewContext()
	defer ctx.Close()
	org := webhookOrg(t, ctx, "wh-completed", true)
	seedProviderSubscription(t, org, ctx, "sub_done", "wh-completed/alice")

	body := renewalEvent("evt_done", "inv_done", "sub_done", 5000)
	if r := deliverWebhook(ctx, "wh-completed", secret, body, ""); r.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, want 200", r.StatusCode)
	}
	if got := balanceOf(t, ctx, org, "wh-completed/alice"); got != 5000 {
		t.Fatalf("balance=%d after a settled renewal, want 5000 — a real settlement was dropped", got)
	}
}

// Square retries a delivery for up to 72h until it gets a 2xx, so the same
// settled payment arrives repeatedly. It must credit exactly once.
//
// Each delivery carries a FRESH event id, because the event-id guard would otherwise
// answer first and prove nothing about the settlement path underneath it. What has to
// hold is that the SETTLEMENT is credited once, keyed on the payment, however many
// distinct deliveries announce it.
func TestWebhookSettlement_Replay_CreditsOnce(t *testing.T) {
	const secret = "whsec_replay_credit"
	registerSquare(t, secret)
	ctx := ae.NewContext()
	defer ctx.Close()
	org := webhookOrg(t, ctx, "wh-replay", true)
	seedProviderSubscription(t, org, ctx, "sub_rep", "wh-replay/alice")

	for i := 0; i < 3; i++ {
		body := renewalEvent(fmt.Sprintf("evt_rep_%d", i), "inv_rep", "sub_rep", 2500)
		if r := deliverWebhook(ctx, "wh-replay", secret, body, ""); r.StatusCode != http.StatusOK {
			t.Fatalf("delivery %d status=%d, want 200", i+1, r.StatusCode)
		}
	}
	if got := balanceOf(t, ctx, org, "wh-replay/alice"); got != 2500 {
		t.Fatalf("balance=%d after 3 deliveries of one settlement, want 2500 — provider retries double-credited", got)
	}
}

// A forged callback must be refused BEFORE anything is written — the signature
// is the only trust anchor on this route, which is unauthenticated by design.
func TestWebhookSettlement_ForgedSignature_NoCredit(t *testing.T) {
	const secret = "whsec_forged"
	registerSquare(t, secret)
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("wh-forged")

	// A body that WOULD credit $500, signed with the wrong key.
	body := settlementEvent("evt_forged", "pay_forged", "COMPLETED", "wh-forged", 50000)
	badSig := squareSign("whsec_attacker", testWebhookURL, body)

	resp := deliverWebhook(ctx, "wh-forged", secret, body, badSig)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status=%d, want 401 for a forged signature", resp.StatusCode)
	}
	if got := balanceOf(t, ctx, org, "wh-forged"); got != 0 {
		t.Fatalf("balance=%d from a FORGED callback, want 0 — anyone who can reach the endpoint can mint money", got)
	}
}

// A delivery Square retries hours after the event — because the first attempt met a
// deploy, an outage, anything but a 2xx — carries the event's ORIGINAL created_at,
// and it is the same money. It settles exactly as the first attempt would have. The
// receiver used to refuse anything older than five minutes, so every retry of every
// event first sent while it was down was refused until Square gave up.
func TestWebhookSettlement_LateRetryCredits(t *testing.T) {
	const secret = "whsec_late"
	registerSquare(t, secret)
	ctx := ae.NewContext()
	defer ctx.Close()
	org := webhookOrg(t, ctx, "wh-late", true)
	seedProviderSubscription(t, org, ctx, "sub_late", "wh-late/alice")

	body := []byte(fmt.Sprintf(
		`{"merchant_id":"M1","type":"invoice.paid","event_id":"evt_late","created_at":%q,`+
			`"data":{"type":"invoice","id":"inv_late","object":{"id":"inv_late","subscription_id":"sub_late",`+
			`"amount_money":{"amount":4200,"currency":"USD"}}}}`,
		time.Now().Add(-3*time.Hour).UTC().Format(time.RFC3339)))
	if r := deliverWebhook(ctx, "wh-late", secret, body, ""); r.StatusCode != http.StatusOK {
		t.Fatalf("status=%d for a genuine delivery retried 3h later, want 200", r.StatusCode)
	}
	if got := balanceOf(t, ctx, org, "wh-late/alice"); got != 4200 {
		t.Fatalf("balance=%d after a late retry of a settled renewal, want 4200", got)
	}
}

// A charge taken in session writes its receipt in the PAYER's org and indexes the
// payment to it. The delivery resolves the processor account's org, which is not
// that one, so the index is what tells it the payment is already credited: it is
// accounted for, credits nothing, and raises no RECONCILE alarm.
func TestWebhookSettlement_IndexedReceiptIsAccountedFor(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := webhookOrg(t, ctx, "wh-processor-org", true)
	if err := paymentorg.Put("pay_payer_org", "payer-org", "payer-org/bob", "", false); err != nil {
		t.Fatalf("index: %v", err)
	}
	event := &processor.WebhookEvent{ID: "evt_idx", Type: "payment.updated", Processor: processor.Square}
	pay := map[string]interface{}{"id": "pay_payer_org", "status": "COMPLETED"}
	db := datastore.New(org.Namespaced(ctx))

	who, why, err := settlementDestination(db, org, event, pay, chargeSourceKind(processor.Square), "pay_payer_org")
	if err != nil || who != nil || why != "" {
		t.Fatalf("an indexed payment resolved to (%+v, %q), want accounted for (nil, \"\")", who, why)
	}

	who, why, err = settlementDestination(db, org, event, map[string]interface{}{"id": "pay_nobody"}, chargeSourceKind(processor.Square), "pay_nobody")
	if err != nil || who != nil || why == "" {
		t.Fatalf("a payment no record names resolved to (%+v, %q), want refused with a reason", who, why)
	}
}

// flakyLedger fails its first n credits, like a host ledger that did not answer.
type flakyLedger struct {
	*fakeLedger
	failures int
}

func (f *flakyLedger) Credit(ctx context.Context, in creditledger.CreditInput) (string, int64, error) {
	if f.failures > 0 {
		f.failures--
		return "", 0, errors.New("ledger unavailable")
	}
	return f.fakeLedger.Credit(ctx, in)
}

// A settlement whose credit did not land is not acknowledged: the delivery
// answers 503 and stays open, so the provider's redelivery credits it — once.
// It used to answer 200 and record the event, and the redelivery Square never
// had a reason to send would have been a duplicate anyway.
func TestWebhookSettlement_FailedCreditIsRedelivered(t *testing.T) {
	const secret = "whsec_flaky"
	registerSquare(t, secret)
	fl := &flakyLedger{fakeLedger: newFakeLedger(), failures: 1}
	injectLedger(t, fl)
	ctx := ae.NewContext()
	defer ctx.Close()
	brand := checkout.DefaultBrandSlug()
	org := webhookOrg(t, ctx, brand, true)
	seedProviderSubscription(t, org, ctx, "sub_flaky", brand+"/bob")
	body := renewalEvent("evt_flaky", "inv_flaky", "sub_flaky", 5000)

	if r := deliverWebhook(ctx, "", secret, body, ""); r.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("a delivery whose credit failed answered %d, want 503 so the provider redelivers", r.StatusCode)
	}
	if n := len(fl.credits()); n != 0 {
		t.Fatalf("%d credits after a failed one", n)
	}
	// The open guard is re-claimed after StartedTTL; here the redelivery arrives
	// after it, which is what Square's backoff does.
	reopen(t, ctx, "square:evt_flaky")
	if r := deliverWebhook(ctx, "", secret, body, ""); r.StatusCode != http.StatusOK {
		t.Fatalf("the redelivery answered %d, want 200", r.StatusCode)
	}
	if r := deliverWebhook(ctx, "", secret, body, ""); r.StatusCode != http.StatusOK {
		t.Fatalf("a third delivery answered %d", r.StatusCode)
	}
	if got := fl.credits(); len(got) != 1 || got[0].AmountCents != 5000 {
		t.Fatalf("credits after redelivery = %+v, want exactly one of 5000", got)
	}
	var rows []*billingevent.BillingEvent
	if _, err := billingevent.Query(datastore.New(org.Namespaced(ctx))).Filter("ObjectId=", "evt_flaky").GetAll(&rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("%d audit rows for one event delivered three times, want 1", len(rows))
	}
}

// reopen ages a started delivery guard past StartedTTL, as the clock would.
func reopen(t *testing.T, ctx ae.Context, key string) {
	t.Helper()
	db := webhookGuardDB(ctx)
	g := idempotencykey.New(db)
	if err := g.Get(db.NewKey(g.Kind(), idempotencykey.DeterministicID("webhook", key), 0, nil)); err != nil {
		t.Fatalf("no guard for %s: %v", key, err)
	}
	if g.Status != idempotencykey.StatusStarted {
		t.Fatalf("guard for %s is %q, want started (left open)", key, g.Status)
	}
	if err := db.Delete(db.NewKey(g.Kind(), idempotencykey.DeterministicID("webhook", key), 0, nil)); err != nil {
		t.Fatal(err)
	}
}
