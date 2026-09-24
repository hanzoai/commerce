package billing

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	squarecore "github.com/square/square-go-sdk/v3/core"
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/auth"
	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billingevent"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/util/test/ae"
)

func squareErr(status int, category, code string) error {
	return squarecore.NewAPIError(status, nil, errors.New(`{"errors":[{"category":"`+category+`","code":"`+code+`"}]}`))
}

// TestCardOutcome_OnlyTheCardsRefusalIsADecline: a charge is a decline only when
// Square refuses the card (PAYMENT_METHOD_ERROR). A reused key, our own
// credentials or location, a rate limit, a 5xx, a transport failure and a
// payment taken without settling are not known outcomes, whatever the status.
func TestCardOutcome_OnlyTheCardsRefusalIsADecline(t *testing.T) {
	for _, tc := range []struct {
		name  string
		res   *processor.PaymentResult
		err   error
		known bool
		ref   string
	}{
		{"settled", &processor.PaymentResult{Success: true, ProcessorRef: "p1", Status: "COMPLETED"}, nil, true, "p1"},
		{"card declined", nil, squareErr(402, "PAYMENT_METHOD_ERROR", "GENERIC_DECLINE"), true, ""},
		{"insufficient funds", nil, squareErr(402, "PAYMENT_METHOD_ERROR", "INSUFFICIENT_FUNDS"), true, ""},
		{"key reused", nil, squareErr(400, "INVALID_REQUEST_ERROR", "IDEMPOTENCY_KEY_REUSED"), false, ""},
		{"our credentials", nil, squareErr(401, "AUTHENTICATION_ERROR", "UNAUTHORIZED"), false, ""},
		{"our location", nil, squareErr(404, "INVALID_REQUEST_ERROR", "NOT_FOUND"), false, ""},
		{"rate limited", nil, squareErr(429, "RATE_LIMIT_ERROR", "RATE_LIMITED"), false, ""},
		{"square down", nil, squarecore.NewAPIError(502, nil, errors.New("bad gateway")), false, ""},
		{"transport", nil, errors.New("read tcp: i/o timeout"), false, ""},
		{"taken, not settled", &processor.PaymentResult{Success: false, ProcessorRef: "p2", Status: "APPROVED"}, nil, false, "p2"},
	} {
		ref, known, err := cardOutcome(tc.res, tc.err)
		if known != tc.known || ref != tc.ref || (tc.name != "settled") != (err != nil) {
			t.Errorf("%s: ref %q known %v err %v; want ref %q known %v", tc.name, ref, known, err, tc.ref, tc.known)
		}
	}
}

// TestVoid_RefusesAnInvoiceWhosePaymentIsUnresolved: an invoice whose last
// payment attempt has no known outcome cannot be voided — that money may have
// moved — and an invoice a payment holds cannot be voided or re-taxed at all.
func TestVoid_RefusesAnInvoiceWhosePaymentIsUnresolved(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("void-pending")
	withFakeSquare(t, squareMock("", "", "sqpay_vp"))
	db := datastore.New(org.Namespaced(ctx))
	sub := seedCardBackedSub(t, db, "void-pending", "dev", "ccof_vp", "cust_vp")
	inv := seedOpenInvoice(t, db, sub, 2000)

	release, err := engine.LockInvoice(db, inv.Id())
	if err != nil {
		t.Fatalf("lock: %v", err)
	}
	if _, f := voidInvoice(ctx, org, inv.Id(), nil, nil); f == nil || f.Status != http.StatusConflict {
		t.Fatalf("void of a held invoice: %+v, want 409", f)
	}
	if resp := invokePay(org, ctx, inv.Id()); resp.StatusCode != http.StatusConflict {
		t.Fatalf("pay of a held invoice: %d, want 409", resp.StatusCode)
	}
	release()

	pending, err := invoiceRow(db, inv.Id())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	pending.PendingMethod, pending.PendingAmount, pending.PendingKey = "card", 2000, "collect:test"
	if err := pending.Update(); err != nil {
		t.Fatalf("update: %v", err)
	}
	if _, f := voidInvoice(ctx, org, inv.Id(), nil, nil); f == nil || f.Status != http.StatusConflict {
		t.Fatalf("void with an unresolved attempt: %+v, want 409", f)
	}
}

// invokeVoidUnresolved drives the operator void as the platform operator
// admin/z.
func invokeVoidUnresolved(org *organization.Organization, ctx context.Context, invID, body string) *http.Response {
	req := httptest.NewRequest(http.MethodPost, "/v1/billing/invoices/"+invID+"/void-unresolved", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	return driveSeeded(func(c *zip.Ctx) {
		c.Locals("organization", org)
		c.Locals("iam_claims", &auth.IAMClaims{Owner: "admin", HomeOrg: "admin", Name: "z"})
		c.SetContext(ctx)
	}, "/v1/billing/invoices/:id/void-unresolved", req, VoidUnresolvedInvoice)
}

// TestVoidUnresolved_EndsAnEscalatedDeletedCard: a renewal charged to a card
// Square no longer has (NOT_FOUND) has no known outcome, is repeated under its
// key through the retry schedule, and is escalated. The customer's void refuses
// it; a platform operator's void, with a reason, voids it, records who and why
// with the abandoned attempt in the billing event ledger, and the next cycle
// ends the subscription on its voided invoice without charging again.
func TestVoidUnresolved_EndsAnEscalatedDeletedCard(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("void-unresolved")
	m := squareMock("", "", "")
	m.chargeErr = squareErr(404, "INVALID_REQUEST_ERROR", "NOT_FOUND")
	withFakeSquare(t, m)
	db := datastore.New(org.Namespaced(ctx))
	d := time.Now().Truncate(time.Second)
	sub := cardSubThrough(t, db, "void-unresolved", d)
	for _, at := range []time.Duration{0, 24 * time.Hour, 72 * time.Hour} {
		cycleAt(t, ctx, org, d.Add(at), false)
	}
	only(t, cycleAt(t, ctx, org, d.Add(169*time.Hour), false), engine.Escalated)
	inv := invoicesForSub(t, db, sub.Id())[0]
	key := inv.PendingKey

	if _, f := voidInvoice(ctx, org, inv.Id(), nil, nil); f == nil || f.Status != http.StatusConflict {
		t.Fatalf("customer void of an escalated invoice: %+v, want 409", f)
	}
	if resp := invokeVoidUnresolved(org, ctx, inv.Id(), `{"reason":"  "}`); resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("operator void with no reason: %d, want 400", resp.StatusCode)
	}
	if resp := invokeVoidUnresolved(org, ctx, inv.Id(), `{"reason":"card deleted at Square; no payment under the key"}`); resp.StatusCode != http.StatusOK {
		t.Fatalf("operator void: %d, want 200", resp.StatusCode)
	}
	got, err := invoiceRow(db, inv.Id())
	if err != nil || got.Status != billinginvoice.Void || got.PendingKey != "" {
		t.Fatalf("invoice %s pending %q (err %v), want void with the attempt taken over", got.Status, got.PendingKey, err)
	}
	evs := make([]*billingevent.BillingEvent, 0)
	if _, err := billingevent.Query(db).Filter("ObjectId=", inv.Id()).GetAll(&evs); err != nil {
		t.Fatalf("read events: %v", err)
	}
	audited := false
	for _, e := range evs {
		attempt, _ := e.Data["attempt"].(map[string]any)
		if e.Type == "invoice.operator_void" && e.Data["actor"] == "admin/z" && attempt["key"] == key &&
			e.Data["reason"] == "card deleted at Square; no payment under the key" {
			audited = true
		}
	}
	if !audited {
		t.Fatalf("events %+v, want invoice.operator_void naming admin/z, the reason and the attempt %s", evs, key)
	}

	calls := m.chargeCalls
	only(t, cycleAt(t, ctx, org, d.Add(200*time.Hour), false), engine.EndedInvoiceVoided)
	if row := reloadSub(t, db, sub.Id()); row.Status != subscription.Canceled || m.chargeCalls != calls {
		t.Fatalf("row %s after %d more charges, want canceled with none", row.Status, m.chargeCalls-calls)
	}
}

// TestVoidUnresolved_RefusedWhileTheCycleRepeats: an attempt the retry schedule
// is still repeating under its key belongs to the cycle; the operator void
// refuses it until it is escalated.
func TestVoidUnresolved_RefusedWhileTheCycleRepeats(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("void-unresolved-early")
	m := squareMock("", "", "")
	m.chargeErr = errors.New("read tcp: i/o timeout")
	withFakeSquare(t, m)
	db := datastore.New(org.Namespaced(ctx))
	d := time.Now().Truncate(time.Second)
	sub := cardSubThrough(t, db, "void-unresolved-early", d)
	only(t, cycleAt(t, ctx, org, d, false), engine.Skipped)
	inv := invoicesForSub(t, db, sub.Id())[0]
	if resp := invokeVoidUnresolved(org, ctx, inv.Id(), `{"reason":"early"}`); resp.StatusCode != http.StatusConflict {
		t.Fatalf("operator void of a past-due row's attempt: %d, want 409", resp.StatusCode)
	}
	if got, _ := invoiceRow(db, inv.Id()); got.Status != billinginvoice.Open || got.PendingKey == "" {
		t.Fatalf("invoice %s pending %q, want open with its attempt", got.Status, got.PendingKey)
	}
}
