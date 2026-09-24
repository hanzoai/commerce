package billing

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/thirdparty/kms"
	"github.com/hanzoai/commerce/util/test/ae"
)

// TestSettle_CancelAfterTheRowWasReadChargesNothing: the customer cancels at
// period end after the cycle read the row. The charge is asked for only against
// the row as it stands, so nothing is charged and the cancel stands.
func TestSettle_CancelAfterTheRowWasReadChargesNothing(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("cyc-stale-cancel")
	m := squareMock("", "", "sqpay_sc")
	withFakeSquare(t, m)
	db := datastore.New(org.Namespaced(ctx))
	now := time.Now()
	sub := cardSubThrough(t, db, "cyc-stale-cancel", now.Add(-time.Hour))
	stale := reloadSub(t, db, sub.Id())
	if _, err := cancelSubscription(ctx, org, sub.Id(), true); err != nil {
		t.Fatalf("cancel: %v", err)
	}

	res := settleOne(ctx, org, db, stale, engine.Run{Now: now}, nil, chargeProviderForOrg(org))
	if res.Action != engine.CanceledAtPeriodEnd || m.chargeCalls != 0 {
		t.Fatalf("result %+v with %d charges, want canceled_at_period_end and none", res, m.chargeCalls)
	}
	if got := reloadSub(t, db, sub.Id()); !got.EndCancel || got.Status != subscription.Canceled {
		t.Fatalf("row %s EndCancel=%v, want the cancel kept and the row ended", got.Status, got.EndCancel)
	}
}

// TestVoid_ReturnsWhatThePartPaymentTook: an invoice something was paid toward
// gives it back when it is voided — the credit it applied as a credit grant, the
// rest to the subject's balance on the one ledger — once, however often it is
// voided or written off.
func TestVoid_ReturnsWhatThePartPaymentTook(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("void-return")
	withFakeSquare(t, squareMock("", "", "sqpay_vr"))
	db := datastore.New(org.Namespaced(ctx))
	sub := seedCardBackedSub(t, db, "void-return", "dev", "ccof_vr", "cust_vr")
	inv := seedOpenInvoice(t, db, sub, 2000)
	part, err := invoiceRow(db, inv.Id())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	part.AmountPaid, part.CreditApplied = 800, 500 // 500 of credit, 300 of balance
	if err := part.Update(); err != nil {
		t.Fatalf("update: %v", err)
	}

	if _, f := voidInvoice(ctx, org, inv.Id(), nil, nil); f != nil {
		t.Fatalf("void: %+v", f)
	}
	if left, _ := BurnCreditsPreview(db, "void-return", 500); left != 0 {
		t.Fatalf("%d of the 500 cents of credit not returned", left)
	}
	if bal := walletOf(t, ctx, org, "void-return"); bal != 300 {
		t.Fatalf("balance %d, want the 300 cents returned", bal)
	}
	got, _ := invoiceRow(db, inv.Id())
	if got.Status != billinginvoice.Void || got.Metadata["returnedCents"] == nil {
		t.Fatalf("invoice %s metadata %v, want void with the return recorded", got.Status, got.Metadata)
	}

	// Returned again, even with the record of the return lost, it returns nothing.
	delete(got.Metadata, "returnedCents")
	if err := engine.ReturnPaid(ctx, db, got, prepaidFor(ctx, org), time.Now()); err != nil {
		t.Fatalf("second return: %v", err)
	}
	if left, _ := BurnCreditsPreview(db, "void-return", 1000); left != 500 {
		t.Fatalf("credit returned twice: %d of 1000 uncovered, want 500", left)
	}
	if bal := walletOf(t, ctx, org, "void-return"); bal != 300 {
		t.Fatalf("balance %d after a second return, want 300", bal)
	}
}

// TestCalculateTax_RefusesAnIssuedInvoice: tax is set on a draft; an issued
// invoice's AmountDue is fixed and payments are measured against it.
func TestCalculateTax_RefusesAnIssuedInvoice(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("tax-open")
	withFakeSquare(t, squareMock("", "", "sqpay_to"))
	db := datastore.New(org.Namespaced(ctx))
	sub := seedCardBackedSub(t, db, "tax-open", "dev", "ccof_to", "cust_to")
	inv := seedOpenInvoice(t, db, sub, 2000)

	req := httptest.NewRequest(http.MethodPost, "/v1/billing/invoices/"+inv.Id()+"/calculate-tax?country=US", nil)
	resp := driveSeeded(func(c *zip.Ctx) { c.Locals("organization", org); c.SetContext(ctx) },
		"/v1/billing/invoices/:id/calculate-tax", req, CalculateInvoiceTax)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("tax on an open invoice: %d, want 400", resp.StatusCode)
	}
	if got, _ := invoiceRow(db, inv.Id()); got.AmountDue != 2000 {
		t.Fatalf("AmountDue %d, want 2000 unchanged", got.AmountDue)
	}
}

// TestCycle_KMSFailureChargesNoCard: when the org's payment credentials cannot
// be read from KMS, no card is charged — the processor would fall back to the
// deployment's own credentials — and the attempt is not counted against the
// customer.
func TestCycle_KMSFailureChargesNoCard(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	broken := kms.NewCachedClient(kms.NewClient(&kms.Config{Enabled: true, URL: srv.URL, ClientID: "id", ClientSecret: "secret"}))

	org := moneyOrg("cyc-kms")
	m := squareMock("", "", "sqpay_kms")
	withFakeSquare(t, m)
	db := datastore.New(org.Namespaced(ctx))
	now := time.Now()
	sub := cardSubThrough(t, db, "cyc-kms", now.Add(-time.Hour))

	report := newCycleReport(now, false)
	cycleOrg(ctx, org, db, now, false, nil, cardFor(broken, org), "", report)
	res := only(t, report, engine.Skipped)
	if m.chargeCalls != 0 || !strings.Contains(res.Reason, "KMS") {
		t.Fatalf("%d charges, reason %q; want none, and the KMS failure named", m.chargeCalls, res.Reason)
	}
	if inv := invoicesForSub(t, db, sub.Id())[0]; inv.AttemptCount != 0 {
		t.Fatalf("attempts counted: %d, want 0", inv.AttemptCount)
	}
}
