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
	"github.com/hanzoai/commerce/util/test/ae"
)

// TestCycleEndpoints_RefuseAnUnclearDryRun: a cycle endpoint charges cards only
// from a request that says dryRun=false. Saying nothing is a dry run, and a
// misspelled parameter or a dryRun that is not true or false is refused.
func TestCycleEndpoints_RefuseAnUnclearDryRun(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("cyc-dryparse")
	m := squareMock("", "", "sqpay_dp")
	withFakeSquare(t, m)
	db := datastore.New(org.Namespaced(ctx))
	cardSubThrough(t, db, "cyc-dryparse", time.Now().Add(-time.Minute))

	run := func(query string) int {
		req := httptest.NewRequest(http.MethodPost, "/v1/billing/cycle/run"+query, nil)
		return driveSeeded(func(c *zip.Ctx) { c.Locals("organization", org); c.SetContext(ctx) }, "/v1/billing/cycle/run", req, RunBillingCycle).StatusCode
	}
	for _, q := range []string{"?dryRun=yes", "?dryrun=true", "?dry_run=1", "?dryRun=true&force=1"} {
		if code := run(q); code != http.StatusBadRequest {
			t.Fatalf("%s: %d, want 400", q, code)
		}
	}
	if code := run("?dryRun=1"); code != http.StatusOK || m.chargeCalls != 0 {
		t.Fatalf("?dryRun=1: %d with %d charges, want a dry run", code, m.chargeCalls)
	}
	if m.chargeCalls != 0 {
		t.Fatalf("a refused request charged %d times", m.chargeCalls)
	}
	if code := run(""); code != http.StatusOK || m.chargeCalls != 0 {
		t.Fatalf("no dryRun: %d with %d charges, want a dry run", code, m.chargeCalls)
	}
	if code := run("?dryRun=false"); code != http.StatusOK || m.chargeCalls != 1 {
		t.Fatalf("?dryRun=false: %d with %d charges, want the real run", code, m.chargeCalls)
	}
}

// TestCycle_UnsettledPaymentIsLookedUpNotChargedAgain: Square answers a charge
// with a payment it took and did not settle. That is not a decline: the payment
// id is kept, and the next run looks it up; once it has settled, the renewal is
// paid by it and the card is not charged again.
func TestCycle_UnsettledPaymentIsLookedUpNotChargedAgain(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("cyc-unsettled")
	m := squareMock("", "", "sqpay_held")
	m.chargeStatus = "APPROVED"
	withFakeSquare(t, m)
	db := datastore.New(org.Namespaced(ctx))
	d := time.Now().Truncate(time.Second)
	sub := cardSubThrough(t, db, "cyc-unsettled", d)

	only(t, cycleAt(t, ctx, org, d, false), engine.Skipped)
	inv := invoicesForSub(t, db, sub.Id())[0]
	if inv.PendingRef != "sqpay_held" || inv.AttemptCount != 0 {
		t.Fatalf("pending ref %q attempts %d, want the payment id kept and nothing counted", inv.PendingRef, inv.AttemptCount)
	}

	m.txStatus = map[string]string{"sqpay_held": "COMPLETED"}
	only(t, cycleAt(t, ctx, org, d.Add(time.Hour), false), engine.Retried)
	if m.chargeCalls != 1 {
		t.Fatalf("card charged %d times, want the one payment", m.chargeCalls)
	}
	if paid, _ := invoiceRow(db, inv.Id()); paid.Status != billinginvoice.Paid || paid.PaymentRef != "sqpay_held" {
		t.Fatalf("invoice %s ref %q, want paid by the looked-up payment", paid.Status, paid.PaymentRef)
	}
}

// TestLineItems_KeepTheInvoiceListed: editing a draft invoice writes it back,
// and it stays in the invoice list.
func TestLineItems_KeepTheInvoiceListed(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("inv-edit")
	view, f := RaiseInvoice(ctx, org, InvoiceIn{UserID: "inv-edit/alice", Lines: []InvoiceLine{{Description: "setup", Amount: 500}}})
	if f != nil {
		t.Fatalf("raise: %+v", f)
	}
	body := `{"type":"one_off","description":"extra","amount":300}`
	req := httptest.NewRequest(http.MethodPost, "/v1/billing/invoices/"+view.ID+"/line-items", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := driveSeeded(func(c *zip.Ctx) { c.Locals("organization", org); c.SetContext(ctx) },
		"/v1/billing/invoices/:id/line-items", req, AddInvoiceLineItem)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		t.Fatalf("add line item: %d", resp.StatusCode)
	}
	listed, err := ListInvoices(ctx, org, "inv-edit/alice", "", "", 0, 0)
	if err != nil || len(listed) != 1 || listed[0].ID != view.ID {
		t.Fatalf("listed %+v (err %v), want the edited draft", listed, err)
	}
}
