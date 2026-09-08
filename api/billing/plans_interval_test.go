package billing

import (
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/types/currency"
	types "github.com/hanzoai/commerce/types"
	"github.com/hanzoai/commerce/util/test/ae"
)

// A plan bought for the year: the catalog publishes an annual price beside the
// recurring one, the buyer names WHICH period, and the server holds both prices.
//
// The arithmetic is the whole risk here, because it runs in two directions that
// nothing forces to agree. PriceAnnual is published PER MONTH — Dev's $15.58
// against a $19 list — while Price is collected in FULL once per period, and a
// yearly period is a year. So the charge is PriceAnnual x 12 and the revenue
// report is Price / 12, and if either side picked a different twelve the other
// would keep working: a year of Dev sold for $15.58 still renews, still invoices,
// still reports. These pin both directions and the identity between them.

// annual reads a plan's two published prices, so these tests assert the mechanism
// rather than restating the catalog — a reprice must not fail a test about how
// periods work. (The prices themselves are canaried in plans_drift_test.go.)
func annual(t *testing.T, slug string) (monthly, perMonthAnnual currency.Cents) {
	t.Helper()
	sp := lookupPlan(slug)
	if sp == nil {
		t.Fatalf("plan %q missing from the catalog", slug)
	}
	if sp.PriceAnnual <= 0 {
		t.Fatalf("plan %q publishes no annual price; these tests need one", slug)
	}
	return currency.Cents(sp.Price), currency.Cents(sp.PriceAnnual)
}

// TestPlanAtInterval is the rule itself, away from HTTP: a year costs twelve of
// the advertised months, an absent choice changes nothing, and a period the plan
// does not publish is refused rather than approximated.
func TestPlanAtInterval(t *testing.T) {
	p := &plan.Plan{
		Slug:          "dev",
		Price:         currency.Cents(1900),
		PriceAnnual:   currency.Cents(1558),
		Interval:      types.Monthly,
		IntervalCount: 1,
	}

	year, err := planAtInterval(p, types.Yearly)
	if err != nil {
		t.Fatalf("yearly: %v", err)
	}
	if year.Price != 1558*12 {
		t.Errorf("a year costs %d, want %d (the advertised %d x 12)", year.Price, 1558*12, 1558)
	}
	if year.Interval != types.Yearly || year.IntervalCount != 1 {
		t.Errorf("yearly plan bills every %d x %q, want 1 x %q", year.IntervalCount, year.Interval, types.Yearly)
	}

	// The authority row is untouched. A period is one customer's choice, and a
	// period written onto the shared row would be one persist away from moving
	// every other subscriber onto it.
	if p.Price != 1900 || p.Interval != types.Monthly {
		t.Fatalf("the authority row was repriced to %d/%q by an annual purchase", p.Price, p.Interval)
	}

	// Absent and "month" are the same purchase: the plan exactly as published.
	for _, chosen := range []types.Interval{"", types.Monthly} {
		at, err := planAtInterval(p, chosen)
		if err != nil {
			t.Fatalf("interval %q: %v", chosen, err)
		}
		if at.Price != p.Price || at.Interval != p.Interval || at.IntervalCount != p.IntervalCount {
			t.Errorf("interval %q gave %d/%q x %d, want the published %d/%q x %d",
				chosen, at.Price, at.Interval, at.IntervalCount, p.Price, p.Interval, p.IntervalCount)
		}
	}

	// A plan with no annual price is not sold annually. Twelve times nothing is a
	// $0 charge, which a card rail answers as a decline — blaming the buyer's card
	// for a tier that was never on sale by the year.
	for name, none := range map[string]*plan.Plan{
		"free tier":     {Slug: "free", Price: 0, PriceAnnual: 0},
		"contact sales": {Slug: "enterprise", ContactSales: true},
		"paid, no annual price": {
			Slug: "legacy", Price: currency.Cents(4900), Interval: types.Monthly,
		},
	} {
		if at, err := planAtInterval(none, types.Yearly); err == nil {
			t.Errorf("%s was sold annually at %d cents; it publishes no annual price", name, at.Price)
		}
	}

	// A period the catalog does not publish at all is refused, never rounded down
	// to the monthly one — a buyer asking for something we do not sell must be
	// told so, not charged for something else.
	for _, unknown := range []types.Interval{"week", "day", "annual", "yearly", "Year", "quarter"} {
		if _, err := planAtInterval(p, unknown); err == nil {
			t.Errorf("interval %q was accepted; only %q and %q are sold", unknown, types.Monthly, types.Yearly)
		}
	}
}

// TestAnnualNormalizesToTheAdvertisedPrice is the identity that holds the two
// halves together, run over the whole catalog: what the card is charged for a
// year, divided back down by the revenue reporter, is exactly the price the
// pricing page advertises. Neither side knows the other exists — one multiplies
// by twelve at the point of sale, the other divides by twelve at the point of
// report — so this is the only place they are made to agree.
func TestAnnualNormalizesToTheAdvertisedPrice(t *testing.T) {
	sold := 0
	for _, sp := range catalog {
		if sp.PriceAnnual <= 0 {
			continue
		}
		row := planFromStatic(&sp)
		year, err := planAtInterval(row, types.Yearly)
		if err != nil {
			t.Errorf("%s publishes an annual price of %d but is not sold annually: %v", sp.Slug, sp.PriceAnnual, err)
			continue
		}
		sold++

		if got := MonthlyNormalizedCents(int64(year.Price), string(year.Interval), year.IntervalCount); got != sp.PriceAnnual {
			t.Errorf("%s: a year at %d cents reports %d/mo of recurring revenue, want the advertised %d",
				sp.Slug, year.Price, got, sp.PriceAnnual)
		}

		// Through the subscription, which is what actually reaches the revenue
		// board: seats multiply the normalized month, never the year. The plan is
		// snapshotted onto the subscription exactly as StartSubscription does it.
		sub := &subscription.Subscription{Quantity: 3}
		sub.Plan = *year
		if got, want := SubscriptionMRRCents(sub), sp.PriceAnnual*3; got != want {
			t.Errorf("%s: three annual seats report %d/mo, want %d", sp.Slug, got, want)
		}
	}
	if sold == 0 {
		t.Fatal("no catalog plan publishes an annual price; this test asserted nothing")
	}
}

// TestAnnualPeriodIsAYear: the period a yearly subscription gets is the period it
// paid for. The charge is twelve months of money, so anything shorter bills it
// again early.
func TestAnnualPeriodIsAYear(t *testing.T) {
	_, perMonth := annual(t, "dev")

	ctx := ae.NewContext()
	defer ctx.Close()
	// The stored authority row, because StartSubscription records the plan's id on
	// the subscription — the period under test is the one a real purchase gets.
	row, err := resolveSubscriptionPlan(datastore.New(ctx), "dev")
	if err != nil {
		t.Fatalf("resolve dev: %v", err)
	}
	year, err := planAtInterval(row, types.Yearly)
	if err != nil {
		t.Fatalf("dev annually: %v", err)
	}

	sub := &subscription.Subscription{}
	engine.StartSubscription(sub, year)
	if want := sub.PeriodStart.AddDate(1, 0, 0); !sub.PeriodEnd.Equal(want) {
		t.Fatalf("a year of dev bills %s..%s, want it to end %s", sub.PeriodStart, sub.PeriodEnd, want)
	}
	if sub.Plan.Price != perMonth*12 {
		t.Fatalf("the subscription carries %d, want %d — the snapshot is what every renewal invoices",
			sub.Plan.Price, perMonth*12)
	}
}

// TestAnnualAtALevelIsRefused: the catalog prices a ladder of recurring prices and
// ONE annual figure standing against the base rung, so Max at level 3 has a
// published monthly price and no published annual price whatsoever. Deriving one
// from the discount the other rungs happen to carry would invent a price and then
// charge a year of it up front.
func TestAnnualAtALevelIsRefused(t *testing.T) {
	row := planFromStatic(lookupPlan("max"))
	if len(row.Prices) < 2 {
		t.Fatal("plan \"max\" publishes no ladder; this test needs one")
	}

	// The base rung is sold annually — a ladder must not cost a plan its annual price.
	base, err := planAt(row, 0, types.Yearly)
	if err != nil {
		t.Fatalf("max at level 0, annually: %v", err)
	}
	if base.Price != row.PriceAnnual*12 {
		t.Fatalf("max annual = %d, want %d", base.Price, row.PriceAnnual*12)
	}

	for level := 1; level < len(row.Prices); level++ {
		if at, err := planAt(row, level, types.Yearly); err == nil {
			t.Errorf("max sold annually at level %d for %d cents; that rung publishes no annual price", level, at.Price)
		}
	}

	// The two choices stay independent where the catalog prices both: every rung
	// is still sold by the month.
	for level := range row.Prices {
		at, err := planAt(row, level, types.Monthly)
		if err != nil {
			t.Fatalf("max at level %d, monthly: %v", level, err)
		}
		if at.Price != row.Prices[level] {
			t.Errorf("max level %d monthly = %d, want %d", level, at.Price, row.Prices[level])
		}
	}
}

// TestSubscribeWithCardChargesTheWholeYear is the money property: the card is
// charged twelve times the advertised monthly-equivalent, once, and the
// subscription carries that amount for every renewal after it.
func TestSubscribeWithCardChargesTheWholeYear(t *testing.T) {
	monthly, perMonth := annual(t, "dev")
	want := int64(perMonth) * 12

	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("yr-buy")
	m := squareMock("cust_y", "ccof_y", "sqpay_y")
	withFakeSquare(t, m)

	resp := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:ok","planId":"dev","interval":"year"}`, nil)
	if resp.StatusCode != http.StatusCreated {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d body=%s, want 201", resp.StatusCode, string(raw))
	}

	out := jsonBody(t, resp)
	if int64(out["amountCents"].(float64)) != want {
		t.Fatalf("receipt amountCents=%v, want %d (%d x 12)", out["amountCents"], want, perMonth)
	}
	if out["interval"] != string(types.Yearly) {
		t.Errorf("receipt interval=%v, want %q — the buyer must be told which period they bought",
			out["interval"], types.Yearly)
	}
	if int64(m.lastChargeAmount) != want {
		t.Fatalf("charged %d, want %d", m.lastChargeAmount, want)
	}
	if int64(m.lastChargeAmount) == int64(monthly) {
		t.Fatalf("charged the monthly price %d for a year", monthly)
	}

	db := datastore.New(org.Namespaced(ctx))
	sub := parentSub(t, db, "yr-buy", "dev")
	if sub == nil {
		t.Fatal("no dev subscription created")
	}
	if int64(sub.Plan.Price) != want {
		t.Fatalf("subscription carries %d, want %d", sub.Plan.Price, want)
	}
	if sub.Plan.Interval != types.Yearly {
		t.Fatalf("subscription interval=%q, want %q", sub.Plan.Interval, types.Yearly)
	}
	if want := sub.PeriodStart.AddDate(1, 0, 0); !sub.PeriodEnd.Equal(want) {
		t.Fatalf("period ends %s, want %s — a year was paid for", sub.PeriodEnd, want)
	}

	// A year of revenue reports as the advertised month, not as twelve of them.
	if got := SubscriptionMRRCents(sub); got != int64(perMonth) {
		t.Fatalf("MRR=%d, want the advertised %d", got, perMonth)
	}

	// The authority row still sells by the month to everyone else.
	base, err := resolveSubscriptionPlan(db, "dev")
	if err != nil {
		t.Fatalf("resolve dev: %v", err)
	}
	if base.Interval != types.Monthly || base.Price != monthly {
		t.Fatalf("the authority row became %d/%q after an annual purchase; it must still be %d/%q",
			base.Price, base.Interval, monthly, types.Monthly)
	}
	if sub.PlanId != base.Id() || sub.PlanId == "" {
		t.Fatalf("subscription PlanId=%q, want the dev plan id %q", sub.PlanId, base.Id())
	}
}

// TestSubscribeWithCardRefusesAnnualWithoutAnAnnualPrice: a plan the catalog does
// not sell by the year takes NO money and leaves NOTHING behind — no charge, no
// vaulted card, no subscription. The refusal lands before the card is touched, so
// the buyer reads why rather than reading a decline.
func TestSubscribeWithCardRefusesAnnualWithoutAnAnnualPrice(t *testing.T) {
	cases := map[string]string{
		"free tier has no annual price": `{"sourceId":"cnon:ok","planId":"free","interval":"year"}`,
		"contact sales has no price":    `{"sourceId":"cnon:ok","planId":"enterprise","interval":"year"}`,
		"a period we do not sell":       `{"sourceId":"cnon:ok","planId":"dev","interval":"week"}`,
		"annual at a level":             `{"sourceId":"cnon:ok","planId":"max","level":3,"interval":"year"}`,
	}

	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			ctx := ae.NewContext()
			defer ctx.Close()
			org := moneyOrg("yr-refuse")
			m := squareMock("cust_n", "ccof_n", "sqpay_n")
			withFakeSquare(t, m)

			resp := invokeSubscribeCard(org, ctx, body, nil)
			if resp.StatusCode != http.StatusBadRequest {
				raw, _ := io.ReadAll(resp.Body)
				t.Fatalf("status=%d body=%s, want 400", resp.StatusCode, string(raw))
			}
			if m.chargeCalls != 0 {
				t.Errorf("charge calls=%d, want 0 — a period we do not sell must never take money", m.chargeCalls)
			}
			if m.createCustomerCalls != 0 || m.addCardNonce != "" {
				t.Errorf("card was vaulted (customers=%d nonce=%q); the refusal must land before the card is touched",
					m.createCustomerCalls, m.addCardNonce)
			}

			db := datastore.New(org.Namespaced(ctx))
			for _, slug := range []string{"free", "enterprise", "dev", "max"} {
				if s := parentSub(t, db, "yr-refuse", slug); s != nil {
					t.Errorf("a %s subscription was created (%s) for a refused purchase", slug, s.Id())
				}
			}
			if pms := pmsFor(t, db, "yr-refuse"); len(pms) != 0 {
				t.Errorf("payment methods=%d, want 0 — a refused purchase leaves no card behind", len(pms))
			}
		})
	}
}

// TestAbsentIntervalBuysTheMonthlyPrice: every client that predates annual billing
// keeps buying exactly what it bought before, and naming "month" explicitly is the
// same purchase as naming nothing.
func TestAbsentIntervalBuysTheMonthlyPrice(t *testing.T) {
	monthly, _ := annual(t, "dev")

	for name, body := range map[string]string{
		"absent": `{"sourceId":"cnon:ok","planId":"dev"}`,
		"month":  `{"sourceId":"cnon:ok","planId":"dev","interval":"month"}`,
	} {
		t.Run(name, func(t *testing.T) {
			ctx := ae.NewContext()
			defer ctx.Close()
			org := moneyOrg("yr-base")
			m := squareMock("cust_m", "ccof_m", "sqpay_m")
			withFakeSquare(t, m)

			resp := invokeSubscribeCard(org, ctx, body, nil)
			if resp.StatusCode != http.StatusCreated {
				raw, _ := io.ReadAll(resp.Body)
				t.Fatalf("status=%d body=%s, want 201", resp.StatusCode, string(raw))
			}
			if int64(m.lastChargeAmount) != int64(monthly) {
				t.Fatalf("charged %d, want the monthly price %d", m.lastChargeAmount, monthly)
			}

			db := datastore.New(org.Namespaced(ctx))
			sub := parentSub(t, db, "yr-base", "dev")
			if sub == nil {
				t.Fatal("no dev subscription created")
			}
			if want := sub.PeriodStart.AddDate(0, 1, 0); !sub.PeriodEnd.Equal(want) {
				t.Fatalf("period ends %s, want a month at %s", sub.PeriodEnd, want)
			}
		})
	}
}

// TestAnnualRenewalChargesTheYearAgain is the one that would be silent. A first
// charge is watched by the customer and by a receipt; the second year's is watched
// by nobody. It renews through the HTTP endpoint, which re-reads the subscription
// by id, so the price under test is the one that survived storage rather than one
// held in memory from the purchase.
func TestAnnualRenewalChargesTheYearAgain(t *testing.T) {
	monthly, perMonth := annual(t, "dev")
	want := int64(perMonth) * 12

	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("yr-renew")
	m := squareMock("cust_ry", "ccof_ry", "sqpay_ry")
	withFakeSquare(t, m)

	resp := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:ok","planId":"dev","interval":"year"}`, nil)
	if resp.StatusCode != http.StatusCreated {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("subscribe status=%d body=%s, want 201", resp.StatusCode, string(raw))
	}
	if int64(m.lastChargeAmount) != want {
		t.Fatalf("first charge=%d, want %d", m.lastChargeAmount, want)
	}

	db := datastore.New(org.Namespaced(ctx))
	sub := parentSub(t, db, "yr-renew", "dev")
	if sub == nil {
		t.Fatal("no dev subscription created")
	}

	// Age it past the year it paid for — a renewal only bills an elapsed period.
	fresh := subscription.New(db)
	if err := fresh.GetById(sub.Id()); err != nil {
		t.Fatalf("re-read subscription: %v", err)
	}
	fresh.PeriodStart = time.Now().AddDate(-2, 0, 0)
	fresh.PeriodEnd = time.Now().AddDate(-1, 0, 0)
	if err := fresh.Update(); err != nil {
		t.Fatalf("age subscription: %v", err)
	}

	renew := invokeRenew(org, ctx, sub.Id())
	if renew.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(renew.Body)
		t.Fatalf("renew status=%d body=%s, want 200", renew.StatusCode, string(raw))
	}
	if m.chargeCalls != 2 {
		t.Fatalf("charge calls=%d, want 2 (the purchase and the renewal)", m.chargeCalls)
	}
	if int64(m.lastChargeAmount) == int64(monthly) {
		t.Fatalf("the renewal charged the MONTHLY price %d — the year was lost between the purchase and the renewal", monthly)
	}
	if int64(m.lastChargeAmount) != want {
		t.Fatalf("renewal charged %d, want %d", m.lastChargeAmount, want)
	}

	// The renewed period is another year, not another month: a yearly subscriber
	// billed monthly from the second period on is charged twelve times over.
	after := subscription.New(db)
	if err := after.GetById(sub.Id()); err != nil {
		t.Fatalf("re-read after renewal: %v", err)
	}
	if want := fresh.PeriodEnd.AddDate(1, 0, 0); !after.PeriodEnd.Equal(want) {
		t.Fatalf("the renewed period ends %s, want %s — a year was paid for again", after.PeriodEnd, want)
	}

	// The receipt agrees with the charge.
	var renewal *billinginvoice.BillingInvoice
	for _, inv := range invoicesForSub(t, db, sub.Id()) {
		if inv.PeriodStart.Equal(fresh.PeriodStart) {
			renewal = inv
		}
	}
	if renewal == nil {
		t.Fatal("no invoice for the renewed period")
	}
	if int64(renewal.AmountDue) != want {
		t.Fatalf("renewal invoice amountDue=%d, want %d", renewal.AmountDue, want)
	}
	if renewal.Status != billinginvoice.Paid {
		t.Fatalf("renewal invoice status=%s, want paid", renewal.Status)
	}
}
