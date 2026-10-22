package engine

import (
	"context"
	"testing"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/types"
	"github.com/hanzoai/commerce/util/test/ae"
)

// A plan coupon of N months: the sale grants the subscription the coupon from its
// first paid period (Grant), and each of the first N cycles invoiced from there
// takes the percent off the plan fee. These tests run one subscription through
// its sale and renewals,
// each renewal judged at the instant just after the period before it ended
// (renewAt), so every period is the one a real calendar would bill.

// pro is Hanzo Pro as the catalog sells it monthly: $20.
var pro = plan.Plan{Name: "Pro", Slug: "dev", Price: currency.Cents(2000), Currency: currency.USD, Interval: types.Monthly, IntervalCount: 1}

// beta is 50OFFBETA: 50% off Pro for three months.
var beta = Coupon{Code: "50OFFBETA", Percent: 50, Months: 3}

// sold opens a Pro subscription bought now with c over plans, as the card sale
// does: the coupon granted from the period it holds, then that period's paid
// invoice.
func sold(t *testing.T, db *datastore.Datastore, user string, c Coupon, plans ...string) (*subscription.Subscription, *billinginvoice.BillingInvoice) {
	t.Helper()
	sub := subscription.New(db)
	sub.UserId = user
	sub.PlanId = "dev"
	sub.Plan = pro
	sub.Status = subscription.Active
	sub.DefaultPaymentMethod = "pm_" + user
	sub.PeriodStart = time.Now().Truncate(time.Second)
	sub.PeriodEnd = Advance(sub.PeriodStart, &sub.Plan)
	Grant(sub, c, plans, sub.PeriodStart)
	if err := sub.Create(); err != nil {
		t.Fatalf("create sub: %v", err)
	}
	first, err := CreatePaidFirstInvoice(db, sub, "card", "sq_"+user)
	if err != nil {
		t.Fatalf("first invoice: %v", err)
	}
	return sub, first
}

// renewNext renews sub as of an hour after the period it holds ends, and stores
// the row as the billing cycle does.
func renewNext(t *testing.T, db *datastore.Datastore, sub *subscription.Subscription, pre Prepaid) *billinginvoice.BillingInvoice {
	t.Helper()
	inv, res, err := renewAt(context.Background(), db, sub, pre, nil, sub.PeriodEnd.Add(time.Hour))
	if err != nil || inv == nil || res == nil || !res.Success {
		t.Fatalf("renew: inv=%v res=%+v err=%v", inv, res, err)
	}
	if err := sub.Update(); err != nil {
		t.Fatalf("store the renewed row: %v", err)
	}
	return inv
}

// row is p as a plan row in db, which a change of plan needs.
func row(db *datastore.Datastore, p plan.Plan) *plan.Plan {
	r := plan.New(db)
	r.Name, r.Slug, r.Price, r.Currency, r.Interval, r.IntervalCount = p.Name, p.Slug, p.Price, p.Currency, p.Interval, p.IntervalCount
	return r
}

// planFee is what an invoice's plan lines bill before any discount.
func planFee(inv *billinginvoice.BillingInvoice) int64 {
	n := int64(0)
	for _, li := range inv.LineItems {
		if li.Type == billinginvoice.LineSubscription {
			n += li.Amount
		}
	}
	return n
}

// TestPlanCoupon_ThreeMonthsThenFull is the 50OFFBETA contract: the sale and two
// renewals bill $10 each, every renewal after them $20, and each discount reads
// as its month of three, lands on the invoice the way the first month's does,
// and is what the ledger is drawn for.
func TestPlanCoupon_ThreeMonthsThenFull(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := datastore.New(c)
	db.SetNamespace("coupon-months")

	sub, first := sold(t, db, "coupon-months/alice", beta, "dev")
	if sub.CouponMonths != 3 || !sub.CouponFrom.Equal(sub.PeriodStart) {
		t.Fatalf("granted %d months from %v, want three from %v", sub.CouponMonths, sub.CouponFrom, sub.PeriodStart)
	}
	pre := &purse{balance: 1000000}
	invs := []*billinginvoice.BillingInvoice{first}
	for range 4 {
		invs = append(invs, renewNext(t, db, sub, pre))
	}

	wantDue := []int64{1000, 1000, 1000, 2000, 2000}
	wantName := []string{
		"50OFFBETA — 50% off, month 1 of 3",
		"50OFFBETA — 50% off, month 2 of 3",
		"50OFFBETA — 50% off, month 3 of 3",
		"", "",
	}
	for i, inv := range invs {
		t.Logf("invoice %d: period %s  plan fee %d  discount %d  due %d  %q",
			i+1, inv.PeriodStart.Format("2006-01-02"), planFee(inv), inv.Discount, inv.AmountDue, inv.DiscountName)
		if planFee(inv) != 2000 || inv.AmountDue != wantDue[i] || inv.Discount != 2000-wantDue[i] || inv.DiscountName != wantName[i] {
			t.Fatalf("invoice %d: plan fee %d, due %d, discount %d, %q; want 2000 less %d to %d, %q",
				i+1, planFee(inv), inv.AmountDue, inv.Discount, inv.DiscountName, 2000-wantDue[i], wantDue[i], wantName[i])
		}
		if i < 3 {
			// The month's discount is recorded on its invoice exactly as the first
			// month's is: the code, the percent and the cents it took off.
			if inv.Metadata["coupon"] != "50OFFBETA" || inv.Metadata["couponPercent"] != 50 ||
				inv.Metadata["couponCents"] != int64(1000) || inv.Metadata["couponMonth"] != i+1 || inv.Metadata["couponMonths"] != 3 {
				t.Fatalf("invoice %d metadata %v, want 50OFFBETA at 50%% for 1000 cents, month %d of 3", i+1, inv.Metadata, i+1)
			}
		} else if inv.Metadata["coupon"] != nil {
			t.Fatalf("invoice %d carries %v after the three months", i+1, inv.Metadata["coupon"])
		}
		if i > 0 && !inv.PeriodStart.Equal(invs[i-1].PeriodEnd) {
			t.Fatalf("invoice %d starts %v, not where %d ended (%v)", i+1, inv.PeriodStart, i, invs[i-1].PeriodEnd)
		}
	}
	// The ledger: each renewal draws its invoice's AmountDue, so what the books
	// record as paid for months 2 and 3 is the discounted $10.
	if got := pre.draws; len(got) != 4 || got[0] != 1000 || got[1] != 1000 || got[2] != 2000 || got[3] != 2000 {
		t.Fatalf("ledger draws %v, want [1000 1000 2000 2000]", got)
	}
	// Nothing on the row was spent: the window is what it was at the sale.
	if sub.CouponCode != "50OFFBETA" || sub.CouponMonths != 3 || sub.CouponPercent != 50 {
		t.Fatalf("the row's coupon moved: %q %d%% %d months", sub.CouponCode, sub.CouponPercent, sub.CouponMonths)
	}
}

// TestPlanCoupon_RenewingAgainSpendsNoMonth: a renewal whose row was not saved
// after its invoice, renewed again, finds that invoice rather than discounting
// another period, and a renewal asked for before the period is due bills
// nothing. Three periods are discounted however often the renewals run.
func TestPlanCoupon_RenewingAgainSpendsNoMonth(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := datastore.New(c)
	db.SetNamespace("coupon-rerun")

	sub, first := sold(t, db, "coupon-rerun/bob", beta, "dev")
	pre := &purse{balance: 1000000}
	at := sub.PeriodEnd.Add(time.Hour)
	second, _, err := renewAt(context.Background(), db, sub, pre, nil, at)
	if err != nil || second == nil {
		t.Fatalf("renew: %v", err)
	}
	// The row as stored never heard of that renewal.
	stale := subscription.New(db)
	if err := stale.GetById(sub.Id()); err != nil {
		t.Fatalf("read: %v", err)
	}
	if !stale.PeriodEnd.Before(at) {
		t.Fatal("the stored row already moved; the test needs an unsaved renewal")
	}
	again, _, err := renewAt(context.Background(), db, stale, pre, nil, at)
	if err != nil || again == nil || again.Id() != second.Id() {
		t.Fatalf("renewing the unsaved row: %v, %v; want invoice %s again", again, err, second.Id())
	}
	// Asked again at once, the period is not due and nothing is billed.
	if inv, res, err := renewAt(context.Background(), db, stale, pre, nil, at); err != nil || inv != nil || res == nil || res.Success {
		t.Fatalf("an early renewal billed %v (%+v, %v)", inv, res, err)
	}
	if err := stale.Update(); err != nil {
		t.Fatalf("store: %v", err)
	}
	third := renewNext(t, db, stale, pre)
	fourth := renewNext(t, db, stale, pre)

	discounted := 0
	all := make([]*billinginvoice.BillingInvoice, 0)
	if _, err := billinginvoice.Query(db).Filter("SubscriptionId=", sub.Id()).GetAll(&all); err != nil {
		t.Fatalf("invoices: %v", err)
	}
	for _, inv := range all {
		if inv.Metadata["coupon"] == "50OFFBETA" {
			discounted++
		}
	}
	if len(all) != 4 || discounted != 3 || first.AmountDue != 1000 || second.AmountDue != 1000 ||
		third.AmountDue != 1000 || fourth.AmountDue != 2000 {
		t.Fatalf("%d invoices, %d discounted, dues %d %d %d %d; want 4 with 3 discounted, 1000 1000 1000 2000",
			len(all), discounted, first.AmountDue, second.AmountDue, third.AmountDue, fourth.AmountDue)
	}
	if got := pre.draws; len(got) != 3 || got[0] != 1000 || got[1] != 1000 || got[2] != 2000 {
		t.Fatalf("ledger draws %v, want one each: [1000 1000 2000]", got)
	}
}

// TestPlanCoupon_AYearIsNeverDiscounted: a subscription moved to yearly billing
// inside its window pays the year in full; the coupon is a percent off monthly
// periods, as the sale only sells it monthly.
func TestPlanCoupon_AYearIsNeverDiscounted(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := datastore.New(c)
	db.SetNamespace("coupon-annual")

	sub, _ := sold(t, db, "coupon-annual/carol", beta, "dev")
	pre := &purse{balance: 1000000}
	if second := renewNext(t, db, sub, pre); second.AmountDue != 1000 {
		t.Fatalf("month 2 due %d, want 1000", second.AmountDue)
	}
	year := row(db, pro)
	year.Price, year.Interval = currency.Cents(20000), types.Yearly
	if _, err := ChangePlan(sub, year, false); err != nil {
		t.Fatalf("change to a year: %v", err)
	}
	inv := renewNext(t, db, sub, pre)
	if inv.AmountDue != 20000 || inv.Discount != 0 || inv.Metadata["coupon"] != nil {
		t.Fatalf("the year inside the window: due %d discount %d meta %v, want 20000 in full", inv.AmountDue, inv.Discount, inv.Metadata)
	}
	if !inv.PeriodEnd.Equal(inv.PeriodStart.AddDate(1, 0, 0)) {
		t.Fatalf("the year bills [%v, %v)", inv.PeriodStart, inv.PeriodEnd)
	}
}

// TestPlanCoupon_OnlyThePlansItNames: moved from Pro to Max inside the window, a
// coupon naming Pro alone stops; one naming Max too goes on, off Max's price,
// for the months that are left.
func TestPlanCoupon_OnlyThePlansItNames(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := datastore.New(c)
	db.SetNamespace("coupon-plans")

	max5 := row(db, plan.Plan{Name: "Max", Slug: "max-5x", Price: currency.Cents(10000), Currency: currency.USD, Interval: types.Monthly, IntervalCount: 1})
	pre := &purse{balance: 1000000}

	proOnly, _ := sold(t, db, "coupon-plans/dan", beta, "dev")
	if _, err := ChangePlan(proOnly, max5, false); err != nil {
		t.Fatalf("move: %v", err)
	}
	if inv := renewNext(t, db, proOnly, pre); inv.AmountDue != 10000 || inv.Metadata["coupon"] != nil {
		t.Fatalf("Max under a Pro-only coupon: due %d meta %v, want 10000", inv.AmountDue, inv.Metadata)
	}

	both, _ := sold(t, db, "coupon-plans/erin", beta, "dev", "max-5x")
	if _, err := ChangePlan(both, max5, false); err != nil {
		t.Fatalf("move: %v", err)
	}
	inv := renewNext(t, db, both, pre)
	if inv.AmountDue != 5000 || inv.DiscountName != "50OFFBETA — 50% off, month 2 of 3" {
		t.Fatalf("Max under a coupon naming it: due %d %q, want 5000, month 2 of 3", inv.AmountDue, inv.DiscountName)
	}
}

// TestPlanCoupon_ATrialComesFirst: bought into a trial, the coupon's first month
// is the first paid period, the one the trial's end bills.
func TestPlanCoupon_ATrialComesFirst(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := datastore.New(c)
	db.SetNamespace("coupon-trial")

	trial := row(db, pro)
	trial.TrialPeriodDays = 14
	sub := subscription.New(db)
	sub.UserId = "coupon-trial/fay"
	sub.DefaultPaymentMethod = "pm_fay"
	StartSubscription(sub, trial)
	Grant(sub, beta, []string{"dev"}, sub.PeriodStart)
	if err := sub.Create(); err != nil {
		t.Fatalf("create: %v", err)
	}
	if sub.Status != subscription.Trialing || !sub.CouponFrom.Equal(sub.TrialEnd) {
		t.Fatalf("status %s, window from %v; want a trial and the window from its end %v", sub.Status, sub.CouponFrom, sub.TrialEnd)
	}
	pre := &purse{balance: 1000000}
	inv, res, err := renewAt(context.Background(), db, sub, pre, nil, sub.TrialEnd.Add(time.Hour))
	if err != nil || !res.Success {
		t.Fatalf("the trial's end: %+v %v", res, err)
	}
	if err := sub.Update(); err != nil {
		t.Fatalf("store: %v", err)
	}
	if !inv.PeriodStart.Equal(sub.TrialEnd) || inv.AmountDue != 1000 || inv.DiscountName != "50OFFBETA — 50% off, month 1 of 3" {
		t.Fatalf("first paid period %v due %d %q, want the trial's end at 1000, month 1 of 3", inv.PeriodStart, inv.AmountDue, inv.DiscountName)
	}
	dues := []int64{inv.AmountDue}
	for range 3 {
		dues = append(dues, renewNext(t, db, sub, pre).AmountDue)
	}
	if dues[1] != 1000 || dues[2] != 1000 || dues[3] != 2000 {
		t.Fatalf("after the trial: %v, want [1000 1000 1000 2000]", dues)
	}
}

// TestPlanCoupon_StacksWithTheRenewalCoupon: a win-back staff put on a row inside
// its sale's months comes off what the plan coupon left, both named and recorded
// apart; the renewal after it is the plan coupon's month alone.
func TestPlanCoupon_StacksWithTheRenewalCoupon(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := datastore.New(c)
	db.SetNamespace("coupon-stack")

	sub, _ := sold(t, db, "coupon-stack/gus", beta, "dev")
	sub.NextCouponCode, sub.NextCouponPercent = "WINBACK", 50
	pre := &purse{balance: 1000000}
	inv := renewNext(t, db, sub, pre)
	if inv.AmountDue != Discounted(2000, 50, 50) || inv.Discount != 1500 ||
		inv.DiscountName != "50OFFBETA — 50% off, month 2 of 3; WINBACK — 50% off this renewal" ||
		inv.Metadata["couponCents"] != int64(1000) || inv.Metadata["renewalCoupon"] != "WINBACK" || inv.Metadata["renewalCouponCents"] != int64(500) {
		t.Fatalf("both: due %d discount %d %q meta %v", inv.AmountDue, inv.Discount, inv.DiscountName, inv.Metadata)
	}
	if sub.NextCouponCode != "" {
		t.Fatalf("the renewal coupon stayed on the row: %q", sub.NextCouponCode)
	}
	if next := renewNext(t, db, sub, pre); next.AmountDue != 1000 || next.Metadata["renewalCoupon"] != nil {
		t.Fatalf("month 3: due %d meta %v, want 1000 with the plan coupon alone", next.AmountDue, next.Metadata)
	}
}

// TestPlanCoupon_AGiftSpendsNoMonth: days given to a row inside its months move
// its next period later and spend none of them; the three discounted cycles are
// three invoices however long the periods between them ran.
func TestPlanCoupon_AGiftSpendsNoMonth(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := datastore.New(c)
	db.SetNamespace("coupon-gift")

	sub, first := sold(t, db, "coupon-gift/ida", beta, "dev")
	// Thirty days given in the first month, as api/billing.Gift gives them.
	sub.PeriodEnd = sub.PeriodEnd.AddDate(0, 0, 30)
	if err := sub.Update(); err != nil {
		t.Fatalf("give: %v", err)
	}
	pre := &purse{balance: 1000000}
	dues := []int64{first.AmountDue}
	names := []string{first.DiscountName}
	for range 3 {
		inv := renewNext(t, db, sub, pre)
		dues, names = append(dues, inv.AmountDue), append(names, inv.DiscountName)
	}
	if dues[0] != 1000 || dues[1] != 1000 || dues[2] != 1000 || dues[3] != 2000 ||
		names[1] != "50OFFBETA — 50% off, month 2 of 3" || names[2] != "50OFFBETA — 50% off, month 3 of 3" {
		t.Fatalf("after a 30-day gift: %v %q, want [1000 1000 1000 2000], months 2 and 3 of 3", dues, names)
	}
}

// TestPlanCoupon_AVoidedPeriodSpendsNoMonth: a renewal never paid lapses, its
// invoice is voided, and the period the row is next billed for is the month it
// did not use.
func TestPlanCoupon_AVoidedPeriodSpendsNoMonth(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := datastore.New(c)
	db.SetNamespace("coupon-lapse")

	sub, _ := sold(t, db, "coupon-lapse/jon", beta, "dev")
	broke := &purse{}
	unpaid, res, err := renewAt(context.Background(), db, sub, broke, nil, sub.PeriodEnd.Add(time.Hour))
	if err != nil || res.Success || unpaid.AmountDue != 1000 || sub.Status != subscription.PastDue {
		t.Fatalf("month 2 unpaid: %v %+v %v status %s", unpaid, res, err, sub.Status)
	}
	if err := sub.Update(); err != nil {
		t.Fatalf("store: %v", err)
	}
	// Two months on, the lapsed period is void and the one running now is owed.
	pre := &purse{balance: 1000000}
	inv, res, err := renewAt(context.Background(), db, sub, pre, nil, unpaid.PeriodEnd.AddDate(0, 1, 0).Add(-time.Hour))
	if err != nil || !res.Success || inv.Id() == unpaid.Id() {
		t.Fatalf("after the lapse: %v %+v %v", inv, res, err)
	}
	if inv.AmountDue != 1000 || inv.DiscountName != "50OFFBETA — 50% off, month 2 of 3" {
		t.Fatalf("the period after a voided one: due %d %q, want 1000, month 2 of 3", inv.AmountDue, inv.DiscountName)
	}
}

// TestPlanCoupon_ACanceledRowIsNotRenewed: a subscription canceled inside its
// window ends with the period it paid for; its remaining months go with it.
func TestPlanCoupon_ACanceledRowIsNotRenewed(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := datastore.New(c)
	db.SetNamespace("coupon-cancel")

	sub, _ := sold(t, db, "coupon-cancel/hal", beta, "dev")
	if err := CancelSubscription(sub, true); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	inv, res, err := renewAt(context.Background(), db, sub, &purse{balance: 1000000}, nil, sub.PeriodEnd.Add(time.Hour))
	if err != nil || inv != nil || res == nil || res.Success || sub.Status != subscription.Canceled {
		t.Fatalf("a canceled row renewed: inv %v res %+v err %v status %s", inv, res, err, sub.Status)
	}
}
