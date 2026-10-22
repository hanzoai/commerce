// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package billing

// coupon_extend.go — a plan coupon's months, extended to the subscriptions
// already bought with it.
//
// A sale grants its subscription the months its coupon has at that moment
// (engine.Grant). A SuperAdmin who gives a coupon more months has changed what
// the code is sold as, and the buyers already holding it were sold under the
// same name: ExtendCoupon grants each of them the months the coupon has now,
// counted from their own first paid period. It never takes a month or a plan
// away, and it moves no money: a cycle among the granted months already
// invoiced without the coupon is reported (Missed) for a person to settle.

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/hanzoai/commerce/api/promo"
	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/subscription"

	. "github.com/hanzoai/commerce/types"
)

// CouponExtension is what ExtendCoupon found and did for one code.
type CouponExtension struct {
	// Code is the coupon, as stored.
	Code string `json:"code"`
	// Months is how many monthly periods the coupon discounts now, the window
	// every buyer is extended to.
	Months int `json:"months"`
	// Applied is false for a dry run, which reads and writes nothing.
	Applied bool `json:"applied"`
	// Orgs is every org holding a use of the code.
	Orgs []string `json:"orgs"`
	// Subscriptions is every subscription of those orgs bought with the code.
	Subscriptions []Extended `json:"subscriptions"`
}

// Extended is one subscription bought with the coupon, and its grant.
type Extended struct {
	Org            string `json:"org"`
	SubscriptionID string `json:"subscriptionId,omitempty"`
	UserID         string `json:"userId,omitempty"`
	Status         string `json:"status,omitempty"`
	Test           bool   `json:"test,omitempty"`
	// Percent, Months and From are the grant the row holds after this: Percent
	// off each of its first Months billing cycles, counted from From.
	Percent int        `json:"percent,omitempty"`
	Months  int        `json:"months,omitempty"`
	From    *time.Time `json:"from,omitempty"`
	// Billed is how many cycles have been invoiced since From, discounted or not.
	Billed int `json:"billed"`
	// Missed counts the cycles among the first Months invoiced without the
	// coupon, paid or still owed: renewals that ran before it was extended.
	Missed int `json:"missed,omitempty"`
	// Changed is whether the row's grant moved (or, on a dry run, would).
	Changed bool `json:"changed"`
	// Error is why the row was left as it was.
	Error string `json:"error,omitempty"`
}

// ExtendCoupon grants the plan coupon code, as it now stands, to every live
// subscription already bought with it: each org that redeemed it, each of its
// subscriptions the sale noted the code on. The window runs from the row's first
// paid period — the one the sale granted, else the first invoice the coupon
// discounted — for the coupon's months or the row's, whichever is more, at the
// percent the row was sold, over the plans it was granted and the coupon's.
//
// With apply false it changes nothing and answers what it would do. With apply
// true each row it moves is written and recorded, with by, as a
// subscription.coupon_extended billing event in the org's namespace — the audit
// record beside the caller's own. A row whose record cannot be written is
// reported as failed, as SetCoupon fails a coupon stored without its record.
//
// A canceled subscription renews nothing and is reported, not changed.
func ExtendCoupon(ctx context.Context, raw string, apply bool, by string) (*CouponExtension, error) {
	cpn, err := promo.ReadCoupon(ctx, raw)
	if err != nil {
		return nil, err
	}
	orgs, err := promo.Redeemers(ctx, cpn.Code)
	if err != nil {
		return nil, fmt.Errorf("the uses of coupon %s could not be read: %w", cpn.Code, err)
	}
	out := &CouponExtension{Code: cpn.Code, Months: cpn.Months, Applied: apply, Orgs: orgs, Subscriptions: []Extended{}}
	for _, name := range orgs {
		org, err := orgNamed(ctx, name)
		if err != nil {
			out.Subscriptions = append(out.Subscriptions, Extended{Org: name, Error: err.Error()})
			continue
		}
		subs, err := ListSubscriptions(ctx, org, "", "")
		if err != nil {
			out.Subscriptions = append(out.Subscriptions, Extended{Org: name, Error: "subscriptions could not be read: " + err.Error()})
			continue
		}
		for _, s := range subs {
			if code, _ := s.Metadata["coupon"].(string); code != cpn.Code {
				continue
			}
			out.Subscriptions = append(out.Subscriptions, extendOne(ctx, org, s.Id(), cpn, apply, by))
		}
	}
	return out, nil
}

// extendOne extends one subscription's grant to cpn's months. It writes only
// while the row is the one it read (Swap) and reads again when a renewal wrote
// first, so it never puts back a period a renewal moved.
func extendOne(ctx context.Context, org *organization.Organization, id string, cpn *promo.CouponView, apply bool, by string) Extended {
	for range mixin.Swaps {
		e, sub, before, after := grantOne(ctx, org, id, cpn)
		if e.Error != "" || !apply || !e.Changed {
			return e
		}
		ok, err := sub.Swap()
		if err != nil {
			e.Error = "the grant was not stored: " + err.Error()
			return e
		}
		if !ok {
			continue
		}
		db := datastore.New(org.Namespaced(ctx))
		if _, err := engine.EmitBillingEvent(db, "subscription.coupon_extended", "subscription", sub.Id(), sub.UserId,
			Map{"coupon": after, "by": by}, Map{"coupon": before}); err != nil {
			e.Error = "the grant was stored and its audit record was not: " + err.Error()
		}
		return e
	}
	return Extended{Org: org.Name, SubscriptionID: id, Error: fmt.Sprintf("the subscription changed under %d reads in a row", mixin.Swaps)}
}

// grantOne reads one subscription and puts cpn's months on it, in memory: what
// the row would hold, and the grant before and after.
func grantOne(ctx context.Context, org *organization.Organization, id string, cpn *promo.CouponView) (Extended, *subscription.Subscription, Map, Map) {
	e := Extended{Org: org.Name, SubscriptionID: id}
	sub, err := loadSubscription(ctx, org, id)
	if err != nil {
		e.Error = err.Error()
		return e, nil, nil, nil
	}
	db := datastore.New(org.Namespaced(ctx))
	e.UserID, e.Status, e.Test = sub.UserId, string(sub.Status), sub.Test
	switch sub.Status {
	case subscription.Active, subscription.PastDue, subscription.Trialing:
	default:
		e.Error = "the subscription is " + string(sub.Status) + ", so no renewal bills it"
		return e, nil, nil, nil
	}

	invs := make([]*billinginvoice.BillingInvoice, 0)
	if _, err := billinginvoice.Query(db).Filter("SubscriptionId=", sub.Id()).Filter("UserId=", sub.UserId).GetAll(&invs); err != nil {
		e.Error = "invoices could not be read: " + err.Error()
		return e, nil, nil, nil
	}
	slices.SortFunc(invs, func(a, b *billinginvoice.BillingInvoice) int { return a.PeriodStart.Compare(b.PeriodStart) })

	// Where the months are counted from and at what percent: the grant the sale
	// made, else the first invoice the coupon discounted, which is the first paid
	// period.
	from, percent := sub.CouponFrom, sub.CouponPercent
	if sub.CouponCode != cpn.Code || from.IsZero() {
		from, percent = time.Time{}, 0
		for _, inv := range invs {
			if code, _ := inv.Metadata["coupon"].(string); code == cpn.Code && inv.Status != billinginvoice.Void {
				from, percent = inv.PeriodStart, number(inv.Metadata["couponPercent"])
				break
			}
		}
	}
	if from.IsZero() {
		e.Error = "no invoice of the subscription carries the coupon, so its first paid period is unknown"
		return e, nil, nil, nil
	}
	if percent < 1 {
		percent = cpn.Percent
	}
	months := cpn.Months
	plans := slices.Clone(cpn.Plans)
	if sub.CouponCode == cpn.Code {
		months = max(months, sub.CouponMonths)
		for _, p := range sub.CouponPlans {
			if !slices.Contains(plans, p) {
				plans = append(plans, p)
			}
		}
	}

	before := grantOf(sub)
	engine.Grant(sub, engine.Coupon{Code: cpn.Code, Percent: percent, Months: months}, plans, from)
	after := grantOf(sub)
	e.Percent, e.Months = sub.CouponPercent, sub.CouponMonths
	f := sub.CouponFrom
	e.From = &f
	e.Changed = !sameGrant(before, after)

	// The cycles since the first paid period, in order, counted as engine.Billed
	// counts them: the first Months are the coupon's, and one billed without it
	// is missed.
	seen := map[int64]bool{}
	for _, inv := range invs {
		at := inv.PeriodStart.Unix()
		if inv.Status == billinginvoice.Void || at < from.Unix() || seen[at] || !engine.BillsPlan(inv) {
			continue
		}
		seen[at] = true
		e.Billed++
		if code, _ := inv.Metadata["coupon"].(string); e.Billed <= months && code != cpn.Code {
			e.Missed++
		}
	}
	return e, sub, before, after
}

// grantOf is a row's plan coupon grant as the audit record states it.
func grantOf(sub *subscription.Subscription) Map {
	if sub.CouponCode == "" {
		return nil
	}
	return Map{
		"code": sub.CouponCode, "percent": sub.CouponPercent, "months": sub.CouponMonths,
		"plans": slices.Clone(sub.CouponPlans), "from": sub.CouponFrom.UTC(),
	}
}

func sameGrant(a, b Map) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a["code"] == b["code"] && a["percent"] == b["percent"] && a["months"] == b["months"] &&
		slices.Equal(a["plans"].([]string), b["plans"].([]string)) &&
		a["from"].(time.Time).Equal(b["from"].(time.Time))
}

// number reads a whole number a stored invoice's metadata holds: JSON hands it
// back as a float64, a row not yet stored as the int it was written as.
func number(v interface{}) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	}
	return 0
}

// orgNamed reads the organization named name, never creating one.
func orgNamed(ctx context.Context, name string) (*organization.Organization, error) {
	orgs := make([]*organization.Organization, 0, 1)
	if _, err := organization.Query(datastore.New(ctx)).Filter("Name=", name).Limit(1).GetAll(&orgs); err != nil {
		return nil, fmt.Errorf("organization %s could not be read: %w", name, err)
	}
	if len(orgs) == 0 {
		return nil, fmt.Errorf("organization %s is not in the store", name)
	}
	return orgs[0], nil
}
