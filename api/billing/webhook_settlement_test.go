package billing

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/models/types/currency"
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
// rawSig is supplied (the forged/unsigned cases).
func deliverWebhook(ctx ae.Context, orgName, secret string, body []byte, rawSig string) *http.Response {
	sig := rawSig
	if sig == "" {
		sig = squareSign(secret, testWebhookURL, body)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/billing/webhooks/square", strings.NewReader(string(body)))
	req.Header.Set(squareSigHeader, sig)
	req.Header.Set("X-Org-Id", orgName)
	return driveSeeded(func(c *zip.Ctx) {
		c.SetContext(ctx)
	}, "/v1/billing/webhooks/:provider", req, HandleProviderWebhook)
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

// The same payment, once actually captured, does credit — so refusing APPROVED
// delays the credit to capture rather than dropping a real payment.
func TestWebhookSettlement_CompletedCredits(t *testing.T) {
	const secret = "whsec_completed"
	registerSquare(t, secret)
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("wh-completed")

	body := settlementEvent("evt_done", "pay_done", "COMPLETED", "wh-completed", 5000)
	if r := deliverWebhook(ctx, "wh-completed", secret, body, ""); r.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, want 200", r.StatusCode)
	}
	if got := balanceOf(t, ctx, org, "wh-completed"); got != 5000 {
		t.Fatalf("balance=%d after a COMPLETED capture, want 5000 — a real settlement was dropped", got)
	}
}

// Square retries a delivery for up to 72h until it gets a 2xx, so the same
// settled payment arrives repeatedly. It must credit exactly once.
func TestWebhookSettlement_Replay_CreditsOnce(t *testing.T) {
	const secret = "whsec_replay_credit"
	registerSquare(t, secret)
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("wh-replay")

	body := settlementEvent("evt_rep", "pay_rep", "COMPLETED", "wh-replay", 2500)
	for i := 0; i < 3; i++ {
		if r := deliverWebhook(ctx, "wh-replay", secret, body, ""); r.StatusCode != http.StatusOK {
			t.Fatalf("delivery %d status=%d, want 200", i+1, r.StatusCode)
		}
	}
	if got := balanceOf(t, ctx, org, "wh-replay"); got != 2500 {
		t.Fatalf("balance=%d after 3 identical deliveries, want 2500 — provider retries double-credited", got)
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
