package billing

import (
	"context"
	stdjson "encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/autorecharge"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/types"
	"github.com/hanzoai/commerce/util/test/ae"
)

// reloadPlan writes a plan authority row for slug, sold monthly at cents, whose
// limits state reloads (nil states none) and the reload buying it switches on.
func reloadPlan(t *testing.T, ctx context.Context, slug string, cents int64, reloads *int, reload *plan.Reload) {
	t.Helper()
	p := plan.New(plan.AuthorityDB(ctx))
	p.Slug, p.Name, p.Category = slug, slug, "personal"
	p.Price, p.Currency, p.Interval, p.IntervalCount = currency.Cents(cents), currency.USD, types.Monthly, 1
	p.Limits = &plan.Limits{ReloadsPerDay: reloads, Reload: reload}
	p.AdminEdited, p.Managed = true, true
	if err := p.Create(); err != nil {
		t.Fatalf("create plan %s: %v", slug, err)
	}
}

// onPlan seeds a recharge org whose own account is served from slug.
func onPlan(t *testing.T, ctx ae.Context, name, slug string) *organization.Organization {
	t.Helper()
	org := seedRechargeOrg(t, ctx, name, 500, 1000000)
	seedSub(t, datastore.New(org.Namespaced(ctx)), name, slug, subscription.Active, "square", "")
	return org
}

// reloads drives n sweeps of org's own account, spending a cent before each so
// each is a new trigger, and answers the card's charges.
func reloads(t *testing.T, ctx ae.Context, org *organization.Organization, m *mockSquareProcessor, n int) int {
	t.Helper()
	db := datastore.New(org.Namespaced(ctx))
	for i := 1; i <= n; i++ {
		spend(db, org.Name, 1, "usage")
		rechargeRun(t, ctx)
	}
	return m.chargeCalls
}

// A plan whose row bounds nothing is reloaded whenever it is due, past the bound
// every other plan keeps.
func TestAnUnboundedPlanReloadsPastTheDailyBound(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	zero := 0
	reloadPlan(t, ctx, "ultra", 19500, &zero, nil)
	org := onPlan(t, ctx, "reload-unbounded", "ultra")
	m := squareMock("", "", "sqpay_unbounded")
	withFakeSquare(t, m)

	if got, want := reloads(t, ctx, org, m, ReloadsPerDay+2), ReloadsPerDay+2; got != want {
		t.Fatalf("an unbounded plan was charged %d times in %d due sweeps, want %d", got, want, want)
	}
	v, err := ReadAutoRecharge(ctx, org, "")
	if err != nil {
		t.Fatal(err)
	}
	if v.PerDay != 0 {
		t.Fatalf("the rule reads %d reloads a day, want 0 (unbounded)", v.PerDay)
	}
}

// A plan's own bound is the one the sweep keeps, and the refusal names it.
func TestAPlansBoundIsTheSweepsBound(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	one := 1
	reloadPlan(t, ctx, "capped", 2000, &one, nil)
	org := onPlan(t, ctx, "reload-one", "capped")
	m := squareMock("", "", "sqpay_one")
	withFakeSquare(t, m)

	if got := reloads(t, ctx, org, m, 1); got != 1 {
		t.Fatalf("first reload charged %d times, want 1", got)
	}
	spend(datastore.New(org.Namespaced(ctx)), org.Name, 1, "usage")
	run := rechargeRun(t, ctx)
	if m.chargeCalls != 1 || run.Charged != 0 {
		t.Fatalf("a plan bounded at one reload a day was charged %d times", m.chargeCalls)
	}
	if len(run.Results) != 1 || !strings.Contains(run.Results[0].Error, "1 times today") {
		t.Fatalf("the skipped reload said %+v, want the plan's bound named", run.Results)
	}
}

// A plan that states no bound keeps the package's, and so does an org on no plan.
func TestAPlanStatingNoBoundKeepsTheDefault(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	reloadPlan(t, ctx, "quiet", 2000, nil, nil)
	for _, org := range []*organization.Organization{
		onPlan(t, ctx, "reload-quiet", "quiet"),
		seedRechargeOrg(t, ctx, "reload-noplan", 500, 1000000),
	} {
		v, err := ReadAutoRecharge(ctx, org, "")
		if err != nil {
			t.Fatal(err)
		}
		if v.PerDay != ReloadsPerDay {
			t.Errorf("%s reads %d reloads a day, want %d", org.Name, v.PerDay, ReloadsPerDay)
		}
	}
}

// The public catalog serves a plan's reload as the terms a checkout shows the
// buyer — threshold, amount and the daily bound, 0 for none — and never the
// row's raw limits.
func TestThePublicCatalogServesTheReloadTerms(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	if _, _, err := SeedPlans(ctx); err != nil {
		t.Fatalf("seed: %v", err)
	}
	zero := 0
	reloadPlan(t, ctx, "ultra", 19500, &zero, &plan.Reload{ThresholdCents: 1000, AmountCents: 5000})
	plans, err := ReadPlans(ctx, "", "", nil)
	if err != nil {
		t.Fatalf("read plans: %v", err)
	}
	raw, err := stdjson.Marshal(plans)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"slug":"ultra"`) {
		t.Fatalf("the catalog does not list the row under test: %s", raw)
	}
	if !strings.Contains(string(raw), `"reload":{"thresholdCents":1000,"amountCents":5000,"perDay":0}`) {
		t.Errorf("the public catalog does not serve ultra's reload terms: %s", raw)
	}
	if strings.Contains(string(raw), `"reloadsPerDay"`) {
		t.Error("the public catalog serves the row's raw reloadsPerDay")
	}
	if n := strings.Count(string(raw), `"reload":`); n != 1 {
		t.Errorf("%d plans carry reload terms, want only ultra", n)
	}
}

// subscribeTo buys slug with a new card as the org's own account.
func subscribeTo(t *testing.T, ctx ae.Context, org *organization.Organization, slug string) {
	t.Helper()
	resp := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:ok","planId":"`+slug+`"}`, nil)
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("subscribe %s: status=%d body=%s, want 201", slug, resp.StatusCode, b)
	}
}

// Buying a plan whose row states a reload switches the org's auto-recharge on at
// its figures, and the rule reads the plan's bound.
func TestBuyingAPlanWithAReloadSwitchesItOn(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	zero := 0
	reloadPlan(t, ctx, "ultra", 19500, &zero, &plan.Reload{ThresholdCents: 1000, AmountCents: 5000})
	org := moneyOrg("reload-arm")
	withFakeSquare(t, squareMock("cust_arm", "ccof_arm", "sqpay_arm"))

	subscribeTo(t, ctx, org, "ultra")

	cfg := storedRule(t, ctx, org)
	if cfg == nil || !cfg.Enabled || cfg.ThresholdCents != 1000 || cfg.AmountCents != 5000 || cfg.Currency != "usd" {
		t.Fatalf("the rule after buying ultra is %+v, want on at $50 below $10 in usd", cfg)
	}
	v, err := ReadAutoRecharge(ctx, org, "")
	if err != nil {
		t.Fatal(err)
	}
	if v.PerDay != 0 {
		t.Fatalf("the rule reads %d reloads a day, want 0 (unbounded)", v.PerDay)
	}
}

// A plan that states no reload leaves the org with no rule.
func TestBuyingAPlanWithoutAReloadSwitchesNothingOn(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	reloadPlan(t, ctx, "plain", 2000, nil, nil)
	org := moneyOrg("reload-plain")
	withFakeSquare(t, squareMock("cust_plain", "ccof_plain", "sqpay_plain"))

	subscribeTo(t, ctx, org, "plain")

	if cfg := storedRule(t, ctx, org); cfg != nil {
		t.Fatalf("buying a plan with no reload stored the rule %+v", cfg)
	}
}

// A rule the org switched on is its own: buying a plan with a reload leaves it as
// it stands. One the org switched off is switched on at the plan's figures.
func TestBuyingAPlanKeepsARuleTheOrgSwitchedOn(t *testing.T) {
	for _, on := range []bool{true, false} {
		ctx := ae.NewContext()
		reloadPlan(t, ctx, "ultra", 19500, nil, &plan.Reload{ThresholdCents: 1000, AmountCents: 5000})
		name := "reload-off"
		if on {
			name = "reload-own"
		}
		org := moneyOrg(name)
		cfg := autorecharge.New(datastore.New(org.Namespaced(ctx)))
		cfg.UserId, cfg.Enabled, cfg.ThresholdCents, cfg.AmountCents, cfg.Currency = name, on, 2000, 7000, "usd"
		if err := cfg.Create(); err != nil {
			t.Fatal(err)
		}
		withFakeSquare(t, squareMock("cust_"+name, "ccof_"+name, "sqpay_"+name))

		out := subscribeAs(t, ctx, org, `{"sourceId":"cnon:ok","planId":"ultra"}`, nil)
		if want := map[bool]string{true: kept, false: armed}[on]; reloadState(out) != want {
			t.Errorf("rule on %v: receipt reload state %q, want %q", on, reloadState(out), want)
		}

		got := storedRule(t, ctx, org)
		switch {
		case on && (!got.Enabled || got.ThresholdCents != 2000 || got.AmountCents != 7000):
			t.Errorf("the org's own rule became %+v, want it on at $70 below $20", got)
		case !on && (!got.Enabled || got.ThresholdCents != 1000 || got.AmountCents != 5000):
			t.Errorf("a rule switched off became %+v, want it on at the plan's $50 below $10", got)
		}
		ctx.Close()
	}
}
