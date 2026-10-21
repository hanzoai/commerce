package billing

import (
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/util/test/ae"
)

// A coupon for the next renewal lands on the subject's live subscription, upper
// case, and percent 0 takes it off; the store holds what the answer says.
func TestSetNextCoupon_SetsAndClearsTheLiveRow(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("next-coupon-set")
	withFakeSquare(t, squareMock("cust_nc", "ccof_nc", "sqpay_nc"))
	sub := subscribed(t, ctx, org, "next-coupon-set/ada", "dev", 1, nil)

	got, err := SetNextCoupon(ctx, org, "next-coupon-set/ada", " winback ", 30)
	if err != nil {
		t.Fatalf("set: %v", err)
	}
	if got.ID != sub.Id() || got.NextCouponCode != "WINBACK" || got.NextCouponPercent != 30 {
		t.Fatalf("answered %s %q at %d, want %s WINBACK at 30", got.ID, got.NextCouponCode, got.NextCouponPercent, sub.Id())
	}
	row := subscription.New(datastore.New(org.Namespaced(ctx)))
	if err := row.GetById(sub.Id()); err != nil || row.NextCouponCode != "WINBACK" || row.NextCouponPercent != 30 {
		t.Fatalf("stored %q at %d (%v)", row.NextCouponCode, row.NextCouponPercent, err)
	}

	got, err = SetNextCoupon(ctx, org, "next-coupon-set/ada", "ignored", 0)
	if err != nil || got.NextCouponCode != "" || got.NextCouponPercent != 0 {
		t.Fatalf("clear: %+v, %v", got, err)
	}
	if err := row.GetById(sub.Id()); err != nil || row.NextCouponCode != "" || row.NextCouponPercent != 0 {
		t.Fatalf("stored after clearing %q at %d (%v)", row.NextCouponCode, row.NextCouponPercent, err)
	}
}

// What cannot be a coupon for a renewal is refused, and a subject with no live
// subscription is not found; neither writes anything.
func TestSetNextCoupon_Refusals(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("next-coupon-refused")
	withFakeSquare(t, squareMock("cust_nr", "ccof_nr", "sqpay_nr"))
	subscribed(t, ctx, org, "next-coupon-refused/ada", "dev", 1, nil)

	for name, tc := range map[string]struct {
		subject, code string
		percent       int
	}{
		"no subject":       {"", "WINBACK", 30},
		"below 0":          {"next-coupon-refused/ada", "WINBACK", -1},
		"above 100":        {"next-coupon-refused/ada", "WINBACK", 101},
		"no code":          {"next-coupon-refused/ada", "", 30},
		"a malformed code": {"next-coupon-refused/ada", "win back!", 30},
	} {
		if _, err := SetNextCoupon(ctx, org, tc.subject, tc.code, tc.percent); !IsSubscriptionRefused(err) {
			t.Errorf("%s: %v, want a refusal (400)", name, err)
		}
	}
	if _, err := SetNextCoupon(ctx, org, "next-coupon-refused/nobody", "WINBACK", 30); !IsSubscriptionNotFound(err) {
		t.Errorf("a subject with no subscription: %v, want not found (404)", err)
	}
	if got := viewOf(t, ctx, org, "next-coupon-refused/ada"); got.NextCouponCode != "" || got.NextCouponPercent != 0 {
		t.Fatalf("a refusal wrote %q at %d", got.NextCouponCode, got.NextCouponPercent)
	}

	// A plan collected outside commerce is never renewed here, so nothing would
	// take the coupon off an invoice; one waiting can still be taken off.
	db := datastore.New(org.Namespaced(ctx))
	if _, err := RecordSubscription(ctx, org, recordDev(t, db)); err != nil {
		t.Fatalf("record: %v", err)
	}
	if _, err := SetNextCoupon(ctx, org, "acme", "WINBACK", 30); !IsSubscriptionRefused(err) {
		t.Errorf("an external plan: %v, want a refusal (400)", err)
	}
	if _, err := SetNextCoupon(ctx, org, "acme", "", 0); err != nil {
		t.Errorf("clearing an external plan: %v", err)
	}
}
