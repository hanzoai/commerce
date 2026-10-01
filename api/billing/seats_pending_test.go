package billing

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/util/test/ae"
)

// A change of seats on a live plan waits for the next period, whose invoice charges
// it; the period in hand keeps the seats it was paid for.

func patched(t *testing.T, r *http.Response) map[string]any {
	t.Helper()
	if r.StatusCode != http.StatusOK {
		t.Fatalf("PATCH: %d %s", r.StatusCode, bodyOf(r))
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(bodyOf(r)), &out); err != nil {
		t.Fatalf("decode PATCH answer: %v", err)
	}
	return out
}

func seatsOf(t *testing.T, out map[string]any, quantity, pending int) {
	t.Helper()
	q, _ := out["quantity"].(float64)
	p, _ := out["pendingQuantity"].(float64)
	if int(q) != quantity || int(p) != pending {
		t.Fatalf("answer holds %v seat(s), %v pending; want %d, %d", out["quantity"], out["pendingQuantity"], quantity, pending)
	}
}

func TestUpdateSubscription_ASeatIncreaseWaitsForTheRenewalThatChargesIt(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("seats-pending")
	m := squareMock("cust_sp", "ccof_sp", "sqpay_sp")
	withFakeSquare(t, m)
	sub := subscribed(t, ctx, org, "seats-pending", "team", 2, nil)
	db := datastore.New(org.Namespaced(ctx))

	// Members beyond the seats paid for now are refused, whatever the next period holds.
	if r := invokeSubPatch(org, ctx, c1MintPrincipal, sub.Id(),
		`{"quantity":5,"members":["seats-pending/a","seats-pending/b","seats-pending/c"]}`); r.StatusCode != http.StatusBadRequest {
		t.Fatalf("members beyond the paid seats: %d %s; want 400", r.StatusCode, bodyOf(r))
	}

	seatsOf(t, patched(t, invokeSubPatch(org, ctx, c1MintPrincipal, sub.Id(), `{"quantity":5}`)), 2, 5)
	v := viewOf(t, ctx, org, "seats-pending")
	paidAs(t, v, "card", 2, 5000)
	if v.Quantity != 2 || v.PendingQuantity != 5 {
		t.Fatalf("row holds %d seat(s), %d pending; want 2, 5", v.Quantity, v.PendingQuantity)
	}

	// The period ends; the renewal charges the seats asked for, and the row holds them.
	row := subscription.New(db)
	if err := row.GetById(sub.Id()); err != nil {
		t.Fatalf("load: %v", err)
	}
	row.PeriodStart, row.PeriodEnd = time.Now().AddDate(0, -2, -10), time.Now().AddDate(0, -1, -10)
	if err := row.Update(); err != nil {
		t.Fatalf("rewind: %v", err)
	}
	m.chargeRef = "sqpay_sp_renew"
	r := invokeRenew(org, ctx, sub.Id())
	if r.StatusCode != http.StatusOK {
		t.Fatalf("renew: %d %s", r.StatusCode, bodyOf(r))
	}
	if m.lastChargeAmount != 12500 {
		t.Fatalf("renewal charged %d, want 12500 (5 seats)", m.lastChargeAmount)
	}
	var renewed *billinginvoice.BillingInvoice
	for _, inv := range invoicesForSub(t, db, sub.Id()) {
		if inv.PaymentRef == "sqpay_sp_renew" {
			renewed = inv
		}
	}
	if renewed == nil || renewed.LineItems[0].Quantity != 5 {
		t.Fatalf("renewal invoice %+v; want one billing 5 seats", renewed)
	}
	v = viewOf(t, ctx, org, "seats-pending")
	paidAs(t, v, "card", 5, 12500)
	if v.Quantity != 5 || v.PendingQuantity != 0 {
		t.Fatalf("row holds %d seat(s), %d pending; want 5, 0", v.Quantity, v.PendingQuantity)
	}

	// A decrease waits the same way.
	seatsOf(t, patched(t, invokeSubPatch(org, ctx, c1MintPrincipal, sub.Id(), `{"quantity":3}`)), 5, 3)
	paidAs(t, viewOf(t, ctx, org, "seats-pending"), "card", 5, 12500)
}

// A plan paid outside commerce takes its new seats when the payment for the next
// period is recorded at them.
func TestUpdateSubscription_AnExternalPlanTakesItsSeatsWithTheNextRecord(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("seats-ext")
	db := datastore.New(org.Namespaced(ctx))
	in := recordDev(t, db)
	in.Subject, in.PlanID, in.Quantity, in.PriceCents = "seats-ext", "team", 2, 5000
	got, err := RecordSubscription(ctx, org, in)
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	seatsOf(t, patched(t, invokeSubPatch(org, ctx, c1MintPrincipal, got.Subscription.ID, `{"quantity":3}`)), 2, 3)
	paidAs(t, viewOf(t, ctx, org, "seats-ext"), "external:square", 2, 5000)

	next := in
	next.PeriodStart, next.PeriodEnd = recEnd, recEnd.AddDate(0, 1, 0)
	next.Quantity, next.PriceCents = 3, 7500
	if out, err := RecordSubscription(ctx, org, next); err != nil || out.Outcome != RecordExtended {
		t.Fatalf("record the next period at 3 seats: %+v, %v", out, err)
	}
	v := viewOf(t, ctx, org, "seats-ext")
	paidAs(t, v, "external:square", 3, 7500)
	if v.Quantity != 3 || v.PendingQuantity != 0 {
		t.Fatalf("row holds %d seat(s), %d pending; want 3, 0", v.Quantity, v.PendingQuantity)
	}
}
