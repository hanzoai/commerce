// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package billing

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hanzoai/commerce/auth"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/util/bit"
	"github.com/hanzoai/commerce/util/permission"
	"github.com/hanzoai/commerce/util/test/ae"
	"github.com/zap-proto/zip"
)

// A team bought with one seat type mixes in the other after the sale: every
// team bought before premium seats existed, and every team a checkout sold
// with one seat type, adds a premium seat by changing its seats, and the
// renewal that opens the next period charges it.

func TestAStandardTeamAddsAPremiumSeatThatItsNextRenewalCharges(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("team-mix")
	m := squareMock("cust_tm", "ccof_tm", "sqpay_tm")
	withFakeSquare(t, m)
	sold, err := SubscribeCard(ctx, org, SubscribeIn{SourceID: "cnon:ok", PlanID: "team", Subject: org.Name, Quantity: 2})
	if err != nil {
		t.Fatalf("subscribe team x2: %v", err)
	}
	id := sold.Sale.SubscriptionID

	out := patched(t, invokeSubPatch(org, ctx, c1MintPrincipal, id, `{"lines":[{"planId":"team-premium","quantity":1}]}`))
	lines, _ := out["lines"].([]any)
	if len(lines) != 1 {
		t.Fatalf("PATCH answer's lines %v, want team-premium added", out["lines"])
	}
	if l := lines[0].(map[string]any); l["plan"] != "team-premium" || l["pendingQuantity"] != float64(1) || l["quantity"] != nil && l["quantity"] != float64(0) {
		t.Fatalf("the added line is %v, want team-premium with no seats now and 1 pending", l)
	}
	// The period in hand keeps what it was paid for: two standard seats.
	v := viewOf(t, ctx, org, org.Name)
	paidAs(t, v, "card", 2, 5000)
	if l := v.Lines[0]; l.Price != 10000 || l.Seats != 0 {
		t.Fatalf("before renewal the line is %+v, want $100 a seat and none paid", l)
	}

	db := datastore.New(org.Namespaced(ctx))
	row := subscription.New(db)
	if err := row.GetById(id); err != nil {
		t.Fatalf("load: %v", err)
	}
	row.PeriodStart, row.PeriodEnd = time.Now().AddDate(0, -2, -10), time.Now().AddDate(0, -1, -10)
	if err := row.Update(); err != nil {
		t.Fatalf("rewind: %v", err)
	}
	m.chargeRef = "sqpay_tm_renew"
	if r := invokeRenew(org, ctx, id); r.StatusCode != http.StatusOK {
		t.Fatalf("renew: %d %s", r.StatusCode, bodyOf(r))
	}
	if m.lastChargeAmount != 2*2500+10000 {
		t.Fatalf("renewal charged %d, want %d (2 standard, 1 premium)", m.lastChargeAmount, 2*2500+10000)
	}
	v = viewOf(t, ctx, org, org.Name)
	paidAs(t, v, "card", 3, 15000)
	if l := v.Lines[0]; l.Quantity != 1 || l.PendingQuantity != 0 || l.Seats != 1 {
		t.Fatalf("after renewal the line is %+v, want 1 held and paid", l)
	}
}

func TestChangeSeatsMovesATeamsSeatsAndNothingElse(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("team-resize")
	withFakeSquare(t, squareMock("cust_tz", "ccof_tz", "sqpay_tz"))
	sold, err := SubscribeCard(ctx, org, SubscribeIn{SourceID: "cnon:ok", PlanID: "team", Subject: org.Name, Quantity: 2})
	if err != nil {
		t.Fatalf("subscribe team x2: %v", err)
	}
	id := sold.Sale.SubscriptionID

	v, err := ChangeSeats(ctx, org, AnyHolder, id, 1, premiumLine)
	if err != nil {
		t.Fatalf("1 standard and 1 premium: %v", err)
	}
	if v.PendingQuantity != 1 || len(v.Lines) != 1 || v.Lines[0].Plan != "team-premium" || v.Lines[0].PendingQuantity != 1 {
		t.Fatalf("the change reads %+v (lines %+v), want 1 standard and 1 premium pending", v, v.Lines)
	}
	// Asking again by the seat type's own name moves the line it holds.
	if v, err = ChangeSeats(ctx, org, AnyHolder, id, 0, []LineIn{{PlanID: "team-premium", Quantity: 3}}); err != nil || len(v.Lines) != 1 || v.Lines[0].PendingQuantity != 3 {
		t.Fatalf("3 premium: %+v, %v", v, err)
	}

	for name, tc := range map[string]struct {
		quantity int
		lines    []LineIn
	}{
		"a plan outside the team":     {0, []LineIn{{PlanID: "max-5x", Quantity: 1}}},
		"the team's own plan as line": {0, []LineIn{{PlanID: "team", Quantity: 1}}},
		"no seats of a seat type":     {0, []LineIn{{PlanID: "team-premium", Quantity: 0}}},
		"one seat in all":             {1, []LineIn{{PlanID: "team-premium", Quantity: 0}}},
		"over a hundred":              {98, []LineIn{{PlanID: "team-premium", Quantity: 3}}},
	} {
		if _, err := ChangeSeats(ctx, org, AnyHolder, id, tc.quantity, tc.lines); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if _, err := ChangeSeats(ctx, org, "team-resize/alice", id, 0, premiumLine); !IsSubscriptionNotFound(err) {
		t.Errorf("a member's own wallet changing the team's seats: %v, want not found", err)
	}

	solo := moneyOrg("solo-resize")
	one, err := SubscribeCard(ctx, solo, SubscribeIn{SourceID: "cnon:ok", PlanID: "dev", Subject: solo.Name})
	if err != nil {
		t.Fatalf("subscribe dev: %v", err)
	}
	if _, err := ChangeSeats(ctx, solo, AnyHolder, one.Sale.SubscriptionID, 0, premiumLine); !IsSubscriptionRefused(err) {
		t.Errorf("a premium seat beside Pro: %v, want refused", err)
	}
}

func TestASeatIsAssignedOnlyToAPersonOfTheOrg(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("team-who")
	withFakeSquare(t, squareMock("cust_tw", "ccof_tw", "sqpay_tw"))
	sold, err := SubscribeCard(ctx, org, SubscribeIn{SourceID: "cnon:ok", PlanID: "team", Subject: org.Name, Quantity: 1, Lines: premiumLine})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	id := sold.Sale.SubscriptionID
	for _, m := range []string{"evil/eve", "not-an-iam-id", "team-who/", "team-who/a/b", "/eve"} {
		if _, err := AssignSeats(ctx, org, AnyHolder, id, []subscription.Seat{{Member: m, Plan: "team-premium"}}); !IsSubscriptionRefused(err) {
			t.Errorf("a premium seat for %q: %v, want refused", m, err)
		}
	}
	v, err := AssignSeats(ctx, org, AnyHolder, id, []subscription.Seat{{Member: " Team-Who/Alice ", Plan: "team-premium"}})
	if err != nil || len(v.Assigned) != 1 || v.Assigned[0].Member != "team-who/alice" {
		t.Fatalf("a person of the org: %+v, %v; want team-who/alice", v, err)
	}
	if _, err := AssignSeats(ctx, org, AnyHolder, id, []subscription.Seat{
		{Member: "team-who/alice", Plan: "team"}, {Member: "TEAM-WHO/ALICE", Plan: "team-premium"},
	}); !IsSubscriptionRefused(err) {
		t.Errorf("one person on two seats by two spellings: %v, want refused", err)
	}
}

// A member who pays from the org's pool holds the team's subscription as its
// wallet does, and assigns none of its seats: that is an admin's.
func TestOnlyAnAdminAssignsSeatsOverHTTP(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("team-http")
	withFakeSquare(t, squareMock("cust_th", "ccof_th", "sqpay_th"))
	sold, err := SubscribeCard(ctx, org, SubscribeIn{SourceID: "cnon:ok", PlanID: "team", Subject: org.Name, Quantity: 1, Lines: premiumLine})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	put := func(identity func(*zip.Ctx)) *http.Response {
		req := httptest.NewRequest(http.MethodPut, "/v1/billing/subscriptions/"+sold.Sale.SubscriptionID+"/seats",
			bytes.NewBufferString(`{"assigned":[{"member":"team-http/bob","plan":"team-premium"}]}`))
		req.Header.Set("Content-Type", "application/json")
		return driveSeeded(func(c *zip.Ctx) {
			c.Locals("organization", org)
			c.SetContext(ctx)
			identity(c)
		}, "/v1/billing/subscriptions/:id/seats", req, AssignBillingSeats)
	}
	member := func(c *zip.Ctx) {
		c.Locals("iam_authenticated", true)
		c.Locals("permissions", bit.Field(permission.None))
		c.Locals("iam_claims", &auth.IAMClaims{Owner: org.Name})
	}
	if r := put(member); r.StatusCode != http.StatusForbidden {
		t.Fatalf("a member assigning seats: %d %s, want 403", r.StatusCode, bodyOf(r))
	}
	if got := viewOf(t, ctx, org, org.Name).Assigned; len(got) != 0 {
		t.Fatalf("a refused assignment wrote %+v", got)
	}
	if r := put(c1MintPrincipal); r.StatusCode != http.StatusOK {
		t.Fatalf("the platform assigning seats: %d %s, want 200", r.StatusCode, bodyOf(r))
	}
}

// A payment recorded for a team of both seat types is checked against every
// seat, opens the row with both, and a later one extends it only at the price
// of every seat it holds.
func TestARecordedTeamPaysForEverySeatType(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("team-rec")
	withFakeSquare(t, squareMock("cust_trc", "ccof_trc", "sqpay_trc"))
	in := RecordIn{
		Subject: org.Name, PlanID: "team", Quantity: 1, Lines: premiumLine,
		PeriodStart: recStart, PeriodEnd: recEnd,
		Processor: "square", Reference: map[string]string{"invoice": "inv:team-1"},
	}
	for _, short := range []int64{5000, 2500, 10000} {
		in.PriceCents = short
		if _, err := RecordSubscription(ctx, org, in); err == nil {
			t.Fatalf("a team of 1 standard and 1 premium recorded at %d cents", short)
		}
	}
	in.PriceCents = 12500
	got, err := RecordSubscription(ctx, org, in)
	if err != nil {
		t.Fatalf("record 1+1 at $125: %v", err)
	}
	if s := got.Subscription; got.Outcome != RecordCreated || s.Quantity != 1 || len(s.Lines) != 1 || s.Lines[0].Quantity != 1 || s.Lines[0].Price != 10000 {
		t.Fatalf("the recorded team is %+v (lines %+v)", s, s.Lines)
	}

	next := in
	next.PeriodStart, next.PeriodEnd = recEnd, recEnd.AddDate(0, 1, 0)
	next.Lines, next.Reference = nil, map[string]string{"invoice": "inv:team-2"}
	next.PriceCents = 5000
	if _, err := RecordSubscription(ctx, org, next); err == nil {
		t.Fatal("the next period of a 1+1 team extended at $50")
	}
	next.Lines = []LineIn{{PlanID: "team-premium", Quantity: 2}}
	next.PriceCents = 22500
	if _, err := RecordSubscription(ctx, org, next); err == nil {
		t.Fatal("a payment naming 2 premium seats extended a team that holds 1")
	}
	next.Lines, next.PriceCents = premiumLine, 12500
	if got, err = RecordSubscription(ctx, org, next); err != nil || got.Outcome != RecordExtended {
		t.Fatalf("the next period at $125: %+v, %v", got, err)
	}
}
