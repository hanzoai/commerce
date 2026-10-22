package billing

import (
	"net/http"
	"testing"
	"time"

	"github.com/hanzoai/commerce/api/promo"
	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billingevent"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/util/test/ae"
)

// A coupon sold as one month and then made three: ExtendCoupon grants the buyers
// who already hold it the three months, from their own first paid period — a row
// sold before windows existed (no window, the first invoice the only record) as
// much as one sold with a window — reports a canceled row without touching it,
// writes nothing on a dry run, records each row it moves, and changes nothing the
// second time.
func TestExtendCoupon(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	warmCouponNS(t, ctx)
	now := time.Now().UTC()
	start, end := now.Add(-time.Minute), now.Add(30*24*time.Hour)
	spec := promo.CouponSpec{Percent: 50, Start: &start, End: &end, Limit: 100, Plans: []string{"dev"}, Enabled: true}
	planCoupon(t, ctx, "50OFFBETA", spec)
	m := squareMock("cust_ex", "ccof_ex", "sqpay_ex")
	withFakeSquare(t, m)

	buy := func(name, fp string) (*subscription.Subscription, *datastore.Datastore) {
		t.Helper()
		org := webhookOrg(t, ctx, name, true)
		m.vaultCard = processor.Card{Brand: "VISA", Last4: "4242", ExpMonth: 12, ExpYear: 2030, Fingerprint: fp}
		if r := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:`+name+`","planId":"dev","coupon":"50OFFBETA"}`, nil); r.StatusCode != http.StatusCreated {
			t.Fatalf("%s: status=%d %s", name, r.StatusCode, bodyOf(r))
		}
		db := datastore.New(org.Namespaced(ctx))
		return parentSub(t, db, name, "dev"), db
	}
	old, oldDB := buy("cp-ext-old", "fp_old")
	fresh, freshDB := buy("cp-ext-new", "fp_new")
	gone, goneDB := buy("cp-ext-gone", "fp_gone")

	// The old buyer was sold before a subscription carried its coupon: no window,
	// only the coupon noted on the row and on its first invoice.
	o := reload(t, oldDB, old.Id())
	o.CouponCode, o.CouponPercent, o.CouponMonths, o.CouponPlans = "", 0, 0, nil
	o.CouponFrom = time.Time{}
	if err := o.Update(); err != nil {
		t.Fatalf("strip: %v", err)
	}
	g := reload(t, goneDB, gone.Id())
	if err := engine.CancelSubscription(g, false); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if err := g.Update(); err != nil {
		t.Fatalf("store cancel: %v", err)
	}

	spec.Months = 3
	planCoupon(t, ctx, "50OFFBETA", spec)

	dry, err := ExtendCoupon(ctx, "50offbeta", false, "z@hanzo.ai")
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if dry.Applied || dry.Months != 3 || len(dry.Orgs) != 3 || len(dry.Subscriptions) != 3 {
		t.Fatalf("dry run %+v, want 3 orgs and 3 subscriptions at 3 months, not applied", dry)
	}
	if r := reload(t, freshDB, fresh.Id()); r.CouponMonths != 1 {
		t.Fatalf("a dry run moved the window to %d months", r.CouponMonths)
	}

	run, err := ExtendCoupon(ctx, "50OFFBETA", true, "z@hanzo.ai")
	if err != nil {
		t.Fatalf("extend: %v", err)
	}
	by := map[string]Extended{}
	for _, e := range run.Subscriptions {
		by[e.Org] = e
	}
	for _, c := range []struct {
		name string
		db   *datastore.Datastore
		id   string
	}{{"cp-ext-old", oldDB, old.Id()}, {"cp-ext-new", freshDB, fresh.Id()}} {
		e := by[c.name]
		r := reload(t, c.db, c.id)
		first := invoicesForSub(t, c.db, c.id)[0]
		if e.Error != "" || !e.Changed || e.Months != 3 || e.Billed != 1 || e.Missed != 0 || r.CouponCode != "50OFFBETA" || r.CouponMonths != 3 || r.CouponPercent != 50 ||
			r.CouponFrom.Unix() != first.PeriodStart.Unix() || len(r.CouponPlans) != 1 {
			t.Fatalf("%s: %+v; row %q %d%% %d months from %v, want 50OFFBETA 50%% 3 months from %v, one cycle billed",
				c.name, e, r.CouponCode, r.CouponPercent, r.CouponMonths, r.CouponFrom, first.PeriodStart)
		}
		events := make([]*billingevent.BillingEvent, 0)
		if _, err := billingevent.Query(c.db).Filter("ObjectId=", c.id).Filter("Type=", "subscription.coupon_extended").GetAll(&events); err != nil || len(events) != 1 {
			t.Fatalf("%s: %d audit records (%v), want 1", c.name, len(events), err)
		}
	}
	if e := by["cp-ext-gone"]; e.Error == "" || e.Changed {
		t.Fatalf("canceled row %+v, want reported and unchanged", e)
	}
	if r := reload(t, goneDB, gone.Id()); r.CouponMonths != 1 {
		t.Fatalf("the canceled row was extended to %d months", r.CouponMonths)
	}

	again, err := ExtendCoupon(ctx, "50OFFBETA", true, "z@hanzo.ai")
	if err != nil {
		t.Fatalf("again: %v", err)
	}
	for _, e := range again.Subscriptions {
		if e.Changed {
			t.Fatalf("a second run changed %+v", e)
		}
	}

	// The old buyer's renewal a month on is month 2 of 3.
	s := soldAgo(t, oldDB, old.Id(), 35)
	inv, _, err := engine.RenewSubscription(ctx, oldDB, s, prepaidFor(ctx, webhookOrg(t, ctx, "cp-ext-old", true)), chargeProviderForOrg(webhookOrg(t, ctx, "cp-ext-old", true)))
	if err != nil || inv == nil || inv.AmountDue != 1000 || inv.DiscountName != "50OFFBETA — 50% off, month 2 of 3" {
		t.Fatalf("month 2 after the extension: %v %v", inv, err)
	}
}
