package billing

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/types/currency"
	types "github.com/hanzoai/commerce/types"
	"github.com/hanzoai/commerce/util/test/ae"
)

// authorityRow adds a plan row to the plan authority, as admin.hanzo.ai adds one.
func authorityRow(t *testing.T, db *datastore.Datastore, slug, category string, price currency.Cents, interval types.Interval) {
	t.Helper()
	p := plan.New(plan.AuthorityDB(db.Context))
	p.Slug, p.Name, p.Category, p.Price = slug, slug, category, price
	p.Currency, p.Interval, p.IntervalCount = currency.USD, interval, 1
	p.Status = plan.StatusActive
	if err := p.Create(); err != nil {
		t.Fatalf("add %s: %v", slug, err)
	}
}

// refusedMonthly asserts a 400 naming the monthly rule.
func refusedMonthly(t *testing.T, what string, r *http.Response) {
	t.Helper()
	body := bodyOf(r)
	if r.StatusCode != http.StatusBadRequest || !strings.Contains(body, errPersonalMonthly.Error()) {
		t.Fatalf("%s: status=%d body=%s, want 400 %q", what, r.StatusCode, body, errPersonalMonthly)
	}
}

// A personal plan is not sold by the year on any buy path, whether its row carries
// an annual price (dev) or none (an admin-added go): the refusal names the rule,
// lands before a card is touched, and leaves no subscription. By the month it sells.
func TestAYearOfAPersonalPlanIsRefused(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("monthly")
	db := datastore.New(org.Namespaced(ctx))
	authorityRow(t, db, "go", categoryPersonal, 800, types.Monthly)
	m := squareMock("cust_mo", "ccof_mo", "sqpay_mo")
	withFakeSquare(t, m)

	if lookupPlan("dev").AnnualTotal <= 0 {
		t.Fatal("dev states no annual price, so this test cannot show one is refused")
	}
	for what, body := range map[string]string{
		"dev by the year, a new card":   `{"sourceId":"cnon:ok","planId":"dev","interval":"year"}`,
		"go by the year, a new card":    `{"sourceId":"cnon:ok","planId":"go","interval":"year"}`,
		"dev by the year, a saved card": `{"paymentMethodId":"pm_saved","planId":"dev","interval":"year"}`,
		"dev by the year, a wallet":     `{"sourceId":"wnon:ok","planId":"dev","interval":"year","wallet":true}`,
	} {
		refusedMonthly(t, what, invokeSubscribeCard(org, ctx, body, nil))
	}
	if m.chargeCalls != 0 {
		t.Fatalf("a refused year charged the card %d time(s)", m.chargeCalls)
	}
	if subs := liveSubs(t, db, "monthly"); len(subs) != 0 {
		t.Fatalf("a refused year left %d subscription(s)", len(subs))
	}

	refusedMonthly(t, "dev by the year, POST /v1/billing/subscriptions",
		invokeSub(org, ctx, c1MintPrincipal, CreateBillingSubscription, `{"userId":"monthly","planId":"dev","interval":"year"}`))

	start := time.Now().Add(-time.Hour)
	for _, processor := range []string{"wire", "square"} {
		_, err := RecordSubscription(ctx, org, RecordIn{
			Subject: "monthly", PlanID: "dev", Interval: types.Yearly, PriceCents: int64(lookupPlan("dev").AnnualTotal),
			PeriodStart: start, PeriodEnd: start.AddDate(1, 0, 0), Processor: processor,
			Reference: map[string]string{"invoice": "inv_year"},
		})
		if !IsSaleRefused(err) || !strings.Contains(err.Error(), errPersonalMonthly.Error()) {
			t.Fatalf("a staff record of dev by the year, %s: %v, want %q", processor, err, errPersonalMonthly)
		}
	}

	r := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:ok","planId":"go"}`, nil)
	if r.StatusCode != http.StatusCreated {
		t.Fatalf("go by the month: status=%d body=%s, want 201", r.StatusCode, bodyOf(r))
	}
	if m.chargeCalls != 1 || m.lastChargeAmount != 800 {
		t.Fatalf("go by the month charged %d time(s), last %d; want once at 800", m.chargeCalls, m.lastChargeAmount)
	}
}

// The rule is about personal plans: a business plan is still priced by the year,
// and a personal row stated by the year is refused as a yearly purchase is.
func TestOnlyAPersonalYearIsRefused(t *testing.T) {
	team := planFromStatic(lookupPlan("team"))
	if at, err := planAt(team, 0, types.Yearly); err != nil || at.Interval != types.Yearly {
		t.Fatalf("team by the year: %+v, %v", at, err)
	}
	stated := planFromStatic(lookupPlan("dev"))
	stated.Interval = types.Yearly
	if _, err := planAt(stated, 0, ""); !errors.Is(err, errPersonalMonthly) {
		t.Fatalf("a personal row stated by the year, bought as published: %v, want %q", err, errPersonalMonthly)
	}
}

// A move onto a personal plan its row states by the year is refused, as a purchase
// of it is; the subscription keeps the plan it holds.
func TestAMoveOntoAYearlyPersonalPlanIsRefused(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("acme")
	db := datastore.New(org.Namespaced(ctx))
	authorityRow(t, db, "dev-year", categoryPersonal, 20000, types.Yearly)

	s := subscription.New(db)
	s.UserId = "acme/owner"
	s.Status = subscription.Active
	s.ProviderType = "square"
	s.Plan.Slug = "dev"
	s.PlanId = "dev"
	s.Quantity = 1
	if err := s.Create(); err != nil {
		t.Fatalf("seed sub: %v", err)
	}
	refusedMonthly(t, "a move onto dev-year", invokeSubPatch(org, ctx, c1MintPrincipal, s.Id(), `{"planId":"dev-year"}`))
	got := subscription.New(db)
	if err := got.GetById(s.Id()); err != nil || got.Plan.Slug != "dev" {
		t.Fatalf("after the refused move the subscription holds %q (%v), want dev", got.Plan.Slug, err)
	}
}
