// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package billing

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/hanzoai/commerce/api/promo"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/subscription"
	types "github.com/hanzoai/commerce/types"
	"github.com/hanzoai/commerce/util/test/ae"
)

// A team is sold in two seat types that one team can mix — team, the standard
// seat at $25 ($240 a year), and team-premium at $100 ($960) — on ONE
// subscription, from 2 members in all.

var premiumLine = []LineIn{{PlanID: "team-premium", Quantity: 1}}

// planLines are the invoice's plan lines, by plan: their seats and their amount.
func planLines(inv *billinginvoice.BillingInvoice) map[string][2]int64 {
	out := map[string][2]int64{}
	for _, li := range inv.LineItems {
		if li.Type == billinginvoice.LineSubscription {
			out[li.PlanId] = [2]int64{li.Quantity, li.Amount}
		}
	}
	return out
}

func TestATeamOfOneStandardAndOnePremiumSeatIsChargedBoth(t *testing.T) {
	for _, tc := range []struct {
		name      string
		interval  types.Interval
		charge    int64
		std, prem int64 // each seat type's price for the period
		mrr       int64
	}{
		{"monthly", "", 2500 + 10000, 2500, 10000, 12500},
		{"yearly", types.Yearly, 24000 + 96000, 24000, 96000, 2000 + 8000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := ae.NewContext()
			defer ctx.Close()
			org := moneyOrg("mixed-" + tc.name)
			m := squareMock("cust_mx", "ccof_mx", "sqpay_mx")
			withFakeSquare(t, m)

			sold, err := SubscribeCard(ctx, org, SubscribeIn{SourceID: "cnon:ok", PlanID: "team", Subject: org.Name,
				Quantity: 1, Lines: premiumLine, Interval: tc.interval})
			if err != nil || sold.Sale == nil {
				t.Fatalf("a team of one standard and one premium seat: %+v, %v", sold, err)
			}
			if m.lastChargeAmount != tc.charge || sold.Sale.AmountCents != tc.charge {
				t.Fatalf("charged %d (receipt %d), want %d", m.lastChargeAmount, sold.Sale.AmountCents, tc.charge)
			}

			sub, err := loadSubscription(ctx, org, sold.Sale.SubscriptionID)
			if err != nil {
				t.Fatalf("load: %v", err)
			}
			if sub.Plan.Slug != "team" || sub.Quantity != 1 || int64(sub.Plan.Price) != tc.std {
				t.Fatalf("row is %s x%d at %d, want team x1 at %d", sub.Plan.Slug, sub.Quantity, sub.Plan.Price, tc.std)
			}
			if len(sub.Lines) != 1 || sub.Lines[0].Plan != "team-premium" || sub.Lines[0].Quantity != 1 || int64(sub.Lines[0].Price) != tc.prem {
				t.Fatalf("row's lines %+v, want one team-premium seat at %d", sub.Lines, tc.prem)
			}
			if got := SubscriptionMRRCents(sub); got != tc.mrr {
				t.Fatalf("MRR %d, want %d", got, tc.mrr)
			}

			db := datastore.New(org.Namespaced(ctx))
			invs := invoicesForSub(t, db, sub.Id())
			if len(invs) != 1 {
				t.Fatalf("%d invoices, want the one paid first period", len(invs))
			}
			lines := planLines(invs[0])
			// The plan's own line is keyed by the row's plan id, each other seat type's by its slug.
			if lines[sub.PlanId] != [2]int64{1, tc.std} || lines["team-premium"] != [2]int64{1, tc.prem} || invs[0].AmountPaid != tc.charge {
				t.Fatalf("first invoice bills %v and was paid %d; want team 1 x %d, team-premium 1 x %d, paid %d",
					lines, invs[0].AmountPaid, tc.std, tc.prem, tc.charge)
			}

			// The quantities and seat types read back, and the period was paid for both.
			v := viewOf(t, ctx, org, org.Name)
			paidAs(t, v, "card", 2, tc.charge)
			if v.Plan.Slug != "team" || v.Quantity != 1 || len(v.Lines) != 1 {
				t.Fatalf("view %+v", v)
			}
			if l := v.Lines[0]; l.Plan != "team-premium" || l.Quantity != 1 || l.Seats != 1 || l.Price != tc.prem {
				t.Fatalf("view's line %+v, want team-premium x1, paid for 1, at %d", l, tc.prem)
			}
		})
	}
}

func TestATeamIsTwoMembersInAllAndAtMostAHundred(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("team-floor")
	withFakeSquare(t, squareMock("cust_tf", "ccof_tf", "sqpay_tf"))

	for _, tc := range []struct {
		name  string
		plan  string
		qty   int
		lines []LineIn
	}{
		{"one standard seat alone", "team", 1, nil},
		{"one premium seat alone", "team-premium", 1, nil},
		{"101 members in all", "team", 60, []LineIn{{PlanID: "team-premium", Quantity: 41}}},
		{"a line of no seats", "team", 2, []LineIn{{PlanID: "team-premium", Quantity: 0}}},
		{"the plan again as a line", "team", 1, []LineIn{{PlanID: "team", Quantity: 1}}},
		{"one seat type twice", "team", 1, []LineIn{{PlanID: "team-premium", Quantity: 1}, {PlanID: "team-premium", Quantity: 1}}},
		{"a plan of no team", "team", 1, []LineIn{{PlanID: "dev", Quantity: 1}}},
		{"lines on a plan sold alone", "dev", 1, premiumLine},
	} {
		_, err := SubscribeCard(ctx, org, SubscribeIn{SourceID: "cnon:ok", PlanID: tc.plan, Subject: org.Name, Quantity: tc.qty, Lines: tc.lines})
		if !IsSaleRefused(err) {
			t.Errorf("%s: %v, want the sale refused before any charge", tc.name, err)
		}
	}
	if _, err := SubscribeCard(ctx, org, SubscribeIn{SourceID: "cnon:ok", PlanID: "team", Subject: org.Name, Quantity: 1,
		Lines: []LineIn{{PlanID: "team-nonesuch", Quantity: 1}}}); !IsSaleNotFound(err) {
		t.Errorf("a line of a plan not on sale: %v, want not found", err)
	}

	// Two of either is a team, and so is the most the team holds.
	for _, tc := range []struct {
		plan   string
		qty    int
		lines  []LineIn
		charge int64
	}{
		{"team-premium", 2, nil, 20000},
		{"team", 60, []LineIn{{PlanID: "team-premium", Quantity: 40}}, 60*2500 + 40*10000},
	} {
		o := moneyOrg("team-ok-" + tc.plan)
		sold, err := SubscribeCard(ctx, o, SubscribeIn{SourceID: "cnon:ok", PlanID: tc.plan, Subject: o.Name, Quantity: tc.qty, Lines: tc.lines})
		if err != nil || sold.Sale.AmountCents != tc.charge {
			t.Errorf("%s x%d %v: %+v, %v; want %d charged", tc.plan, tc.qty, tc.lines, sold, err, tc.charge)
		}
	}
}

// Each seat type renews at its own seats, and a change of either waits for the
// renewal that charges it.
func TestATeamRenewsEverySeatTypeAndChangesItsSeatsAtRenewal(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("team-renew")
	m := squareMock("cust_tr", "ccof_tr", "sqpay_tr")
	withFakeSquare(t, m)
	sold, err := SubscribeCard(ctx, org, SubscribeIn{SourceID: "cnon:ok", PlanID: "team", Subject: org.Name, Quantity: 1, Lines: premiumLine})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	id := sold.Sale.SubscriptionID
	db := datastore.New(org.Namespaced(ctx))

	// The old per-plan floor would refuse this team's single standard seat.
	if r := invokeSubPatch(org, ctx, c1MintPrincipal, id, `{}`); r.StatusCode != http.StatusOK {
		t.Fatalf("a PATCH of a 1+1 team: %d %s; want 200", r.StatusCode, bodyOf(r))
	}
	for _, body := range []string{
		`{"lines":[{"planId":"team-premium","quantity":0}],"quantity":1}`,
		`{"lines":[{"planId":"max-5x","quantity":1}]}`,
		`{"planId":"team-premium"}`,
	} {
		if r := invokeSubPatch(org, ctx, c1MintPrincipal, id, body); r.StatusCode != http.StatusBadRequest {
			t.Errorf("PATCH %s: %d %s; want 400", body, r.StatusCode, bodyOf(r))
		}
	}
	out := patched(t, invokeSubPatch(org, ctx, c1MintPrincipal, id, `{"lines":[{"planId":"team-premium","quantity":3}]}`))
	if lines, _ := out["lines"].([]any); len(lines) != 1 || lines[0].(map[string]any)["pendingQuantity"] != float64(3) {
		t.Fatalf("PATCH answer's lines %v, want team-premium pending 3", out["lines"])
	}
	paidAs(t, viewOf(t, ctx, org, org.Name), "card", 2, 12500)

	row := subscription.New(db)
	if err := row.GetById(id); err != nil {
		t.Fatalf("load: %v", err)
	}
	row.PeriodStart, row.PeriodEnd = time.Now().AddDate(0, -2, -10), time.Now().AddDate(0, -1, -10)
	if err := row.Update(); err != nil {
		t.Fatalf("rewind: %v", err)
	}
	m.chargeRef = "sqpay_tr_renew"
	if r := invokeRenew(org, ctx, id); r.StatusCode != http.StatusOK {
		t.Fatalf("renew: %d %s", r.StatusCode, bodyOf(r))
	}
	if m.lastChargeAmount != 2500+3*10000 {
		t.Fatalf("renewal charged %d, want %d (1 standard, 3 premium)", m.lastChargeAmount, 2500+3*10000)
	}
	v := viewOf(t, ctx, org, org.Name)
	paidAs(t, v, "card", 4, 32500)
	if l := v.Lines[0]; l.Quantity != 3 || l.PendingQuantity != 0 || l.Seats != 3 {
		t.Fatalf("after renewal the line is %+v, want 3 held and paid, none pending", l)
	}
}

func TestAnAdminAssignsEachMemberASeatTypeWithinTheSeatsBought(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("team-assign")
	withFakeSquare(t, squareMock("cust_ta", "ccof_ta", "sqpay_ta"))
	sold, err := SubscribeCard(ctx, org, SubscribeIn{SourceID: "cnon:ok", PlanID: "team", Subject: org.Name, Quantity: 1, Lines: premiumLine})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	id := sold.Sale.SubscriptionID
	alice := subscription.Seat{Member: "team-assign/alice", Plan: "team"}
	bob := subscription.Seat{Member: "team-assign/bob", Plan: "team-premium"}

	v, err := AssignSeats(ctx, org, AnyHolder, id, []subscription.Seat{alice, bob})
	if err != nil || len(v.Assigned) != 2 || v.Assigned[0] != alice || v.Assigned[1] != bob {
		t.Fatalf("assign alice standard and bob premium: %+v, %v", v, err)
	}
	if got := viewOf(t, ctx, org, org.Name).Assigned; len(got) != 2 || got[1] != bob {
		t.Fatalf("the seats read back as %+v", got)
	}

	carol := subscription.Seat{Member: "team-assign/carol", Plan: "team-premium"}
	for name, seats := range map[string][]subscription.Seat{
		"a second premium member on one premium seat": {alice, bob, carol},
		"a member on two seats":                       {alice, {Member: alice.Member, Plan: "team-premium"}},
		"a seat type the team does not hold":          {{Member: "team-assign/dan", Plan: "max-5x"}},
		"a seat with no member":                       {{Plan: "team"}},
	} {
		if _, err := AssignSeats(ctx, org, AnyHolder, id, seats); !IsSubscriptionRefused(err) {
			t.Errorf("%s: %v, want refused", name, err)
		}
	}
	if _, err := AssignSeats(ctx, org, "team-assign/alice", id, []subscription.Seat{carol}); !IsSubscriptionNotFound(err) {
		t.Errorf("a member's own wallet assigning the team's seats: %v, want not found", err)
	}
	// A refused assignment changes nothing; an empty one unassigns every seat.
	if got := viewOf(t, ctx, org, org.Name).Assigned; len(got) != 2 {
		t.Fatalf("a refused assignment left %+v", got)
	}
	if v, err := AssignSeats(ctx, org, AnyHolder, id, nil); err != nil || len(v.Assigned) != 0 {
		t.Fatalf("unassign: %+v, %v", v, err)
	}

	// A plan sold alone has no seats to assign.
	solo := moneyOrg("solo-assign")
	one, err := SubscribeCard(ctx, solo, SubscribeIn{SourceID: "cnon:ok", PlanID: "dev", Subject: solo.Name})
	if err != nil {
		t.Fatalf("subscribe dev: %v", err)
	}
	if _, err := AssignSeats(ctx, solo, AnyHolder, one.Sale.SubscriptionID, []subscription.Seat{{Member: "solo-assign/a", Plan: "dev"}}); !IsSubscriptionRefused(err) {
		t.Errorf("assigning a Pro plan's seat: %v, want refused", err)
	}
}

// An offer takes its percent off a team's charge only when it covers every seat
// type bought: the charge and the invoice discount them together, so an offer for
// the standard seat alone would otherwise discount the premium one too.
func TestAnOfferCoversATeamOnlyWhenItCoversEverySeatType(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	warmCouponNS(t, ctx)
	withFakeSquare(t, squareMock("cust_of", "ccof_of", "sqpay_of"))

	for _, tc := range []struct {
		name   string
		plans  []string
		charge int64
	}{
		{"a promotion of the standard seat alone", []string{"team"}, 12500},
		{"a promotion of both seat types", []string{"team", "team-premium"}, 6250},
	} {
		org := moneyOrg("offer-" + strings.ReplaceAll(tc.name, " ", "-"))
		sold, err := SubscribeCard(ctx, org, SubscribeIn{SourceID: "cnon:ok", PlanID: "team", Subject: org.Name, Quantity: 1,
			Lines: premiumLine, Promo: &promo.Promo{PercentOff: 50, Active: true, Plans: tc.plans}})
		if err != nil || sold.Sale.AmountCents != tc.charge {
			t.Errorf("%s: %+v, %v; want %d charged", tc.name, sold, err, tc.charge)
		}
	}

	now := time.Now().UTC()
	start, end := now.Add(-time.Minute), now.Add(time.Hour)
	planCoupon(t, ctx, "TEAMHALF", promo.CouponSpec{Percent: 50, Start: &start, End: &end, Limit: 5, Plans: []string{"team"}, Enabled: true})
	org := moneyOrg("offer-coupon")
	if _, err := SubscribeCard(ctx, org, SubscribeIn{SourceID: "cnon:ok", PlanID: "team", Subject: org.Name, Quantity: 1,
		Lines: premiumLine, Coupon: "TEAMHALF"}); !IsSaleRefused(err) {
		t.Errorf("a coupon of the standard seat alone on a mixed team: %v, want refused before any charge", err)
	}
}

// The catalog serves both seat types with the group that sells them together.
func TestTheCatalogNamesTheTeamsSeatTypes(t *testing.T) {
	for _, slug := range []string{"team", "team-premium"} {
		p := lookupPlan(slug)
		if p == nil || p.Group != "team" || !p.PerSeat || minSeats(slug) != 2 || maxSeats(slug) != 100 {
			t.Errorf("%s: %+v, want a per-seat seat type of team, 2 to 100", slug, p)
		}
		if v := staticPlanFromModel(planFromStatic(p)); v.Group != "team" {
			t.Errorf("%s served from the plan authority has group %q", slug, v.Group)
		}
	}
	if group("dev") != "" {
		t.Error("Pro is sold alone")
	}
}
