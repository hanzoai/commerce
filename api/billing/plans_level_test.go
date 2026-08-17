package billing

import (
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/test/ae"
)

// A plan sold at a chosen level: the catalog publishes every price the tier is
// sold at, the buyer names WHICH one by index, and the server holds the prices.
//
// These pin the two facts that make it money-safe:
//
//	a level the catalog does not publish is REFUSED, before any card is touched
//	a subscription bought at a level RENEWS at that level, not at the base price
//
// The second is the one that would be silent. A first charge is watched by the
// customer and by a receipt; the second month's is watched by nobody.

// maxLevels reads the ladder the catalog publishes, so these tests assert the
// mechanism rather than restating the prices — a reprice must not fail a test
// about how levels work. (The prices themselves are canaried in
// plans_drift_test.go, which is where a deliberate change is meant to show up.)
func maxLevels(t *testing.T) []int64 {
	t.Helper()
	p := lookupPlan("max")
	if p == nil {
		t.Fatal("plan \"max\" missing from the catalog")
	}
	if len(p.Prices) < 2 {
		t.Fatalf("plan \"max\" publishes %d prices; these tests need a ladder", len(p.Prices))
	}
	return p.Prices
}

// TestLevelPrice is the rule itself, away from HTTP: level 0 is the plan's own
// price, a published level is its own price, and everything else is refused.
func TestLevelPrice(t *testing.T) {
	p := &plan.Plan{
		Slug:   "max",
		Price:  currency.Cents(9900),
		Prices: []currency.Cents{9900, 19900, 29900},
	}

	for level, want := range map[int]currency.Cents{0: 9900, 1: 19900, 2: 29900} {
		got, err := p.LevelPrice(level)
		if err != nil {
			t.Fatalf("level %d: %v", level, err)
		}
		if got != want {
			t.Errorf("level %d = %d cents, want %d", level, got, want)
		}
	}

	// Past the end, negative, and far past the end are all the same refusal.
	for _, level := range []int{3, 10, -1, 1 << 30} {
		if _, err := p.LevelPrice(level); err == nil {
			t.Errorf("level %d was accepted; a level the plan does not publish must be refused", level)
		}
	}

	// A plan with NO ladder is sold at exactly one price. This is the shape every
	// other tier in the catalog has, so it is the case that must not break: level
	// 0 works, and every level above it is refused rather than falling back to the
	// base price and quietly selling the wrong thing.
	flat := &plan.Plan{Slug: "pro", Price: currency.Cents(4900)}
	if got, err := flat.LevelPrice(0); err != nil || got != 4900 {
		t.Fatalf("flat plan level 0 = %d, %v; want 4900, nil", got, err)
	}
	if _, err := flat.LevelPrice(1); err == nil {
		t.Error("a plan that publishes no ladder accepted level 1")
	}
}

// TestSubscribeWithCardRefusesUnpublishedLevel is the money property: a purchase
// naming a level the catalog does not publish takes NO money and leaves NOTHING
// behind — no charge, no vaulted card, no subscription.
//
// It is the test that stands in for "a request naming $1 for Max must be
// refused". There is no amount on this wire to name, so the way to ask for a
// price the catalog never published is to name a level it never published, and
// that is what this refuses.
func TestSubscribeWithCardRefusesUnpublishedLevel(t *testing.T) {
	ladder := maxLevels(t)

	// One past the end of the ladder, negative, absurd, and a level on a plan that
	// publishes no ladder at all.
	cases := map[string]string{
		"one past the end":   `{"sourceId":"cnon:ok","planId":"max","level":` + itoa(len(ladder)) + `}`,
		"far past the end":   `{"sourceId":"cnon:ok","planId":"max","level":999}`,
		"negative":           `{"sourceId":"cnon:ok","planId":"max","level":-1}`,
		"plan has no ladder": `{"sourceId":"cnon:ok","planId":"pro","level":1}`,
	}

	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			ctx := ae.NewContext()
			defer ctx.Close()
			org := moneyOrg("lvl-refuse")
			m := squareMock("cust_x", "ccof_x", "sqpay_x")
			withFakeSquare(t, m)

			resp := invokeSubscribeCard(org, ctx, body, nil)
			if resp.StatusCode != http.StatusBadRequest {
				raw, _ := io.ReadAll(resp.Body)
				t.Fatalf("status=%d body=%s, want 400", resp.StatusCode, string(raw))
			}

			// Refused BEFORE the card: nothing was vaulted and nothing was charged.
			if m.chargeCalls != 0 {
				t.Errorf("charge calls=%d, want 0 — an unpublished level must never take money", m.chargeCalls)
			}
			if m.createCustomerCalls != 0 || m.addCardNonce != "" {
				t.Errorf("card was vaulted (customers=%d nonce=%q); the refusal must land before the card is touched",
					m.createCustomerCalls, m.addCardNonce)
			}

			db := datastore.New(org.Namespaced(ctx))
			if s := parentSub(t, db, "lvl-refuse", "max"); s != nil {
				t.Errorf("a subscription was created (%s) for a refused level", s.Id())
			}
			if s := parentSub(t, db, "lvl-refuse", "pro"); s != nil {
				t.Errorf("a subscription was created (%s) for a refused level", s.Id())
			}
			if pms := pmsFor(t, db, "lvl-refuse"); len(pms) != 0 {
				t.Errorf("payment methods=%d, want 0 — a refused purchase leaves no card behind", len(pms))
			}
		})
	}
}

// TestSubscribeWithCardChargesTheChosenLevel: the first charge is the level's
// price, and the subscription carries it.
func TestSubscribeWithCardChargesTheChosenLevel(t *testing.T) {
	ladder := maxLevels(t)
	const level = 3
	want := ladder[level]

	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("lvl-buy")
	m := squareMock("cust_l", "ccof_l", "sqpay_l")
	withFakeSquare(t, m)

	resp := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:ok","planId":"max","level":3}`, nil)
	if resp.StatusCode != http.StatusCreated {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d body=%s, want 201", resp.StatusCode, string(raw))
	}
	out := jsonBody(t, resp)
	if int64(out["amountCents"].(float64)) != want {
		t.Fatalf("response amountCents=%v, want %d (level %d)", out["amountCents"], want, level)
	}
	if int64(m.lastChargeAmount) != want {
		t.Fatalf("charged %d, want %d — the card must be charged the chosen level", m.lastChargeAmount, want)
	}

	// The subscription carries the chosen price, which is what every later invoice
	// is built from.
	db := datastore.New(org.Namespaced(ctx))
	sub := parentSub(t, db, "lvl-buy", "max")
	if sub == nil {
		t.Fatal("no max subscription created")
	}
	if int64(sub.Plan.Price) != want {
		t.Fatalf("subscription plan price=%d, want %d", sub.Plan.Price, want)
	}
	if sub.Plan.Slug != "max" {
		t.Fatalf("subscription plan slug=%q, want max — a level is a price, never a different tier", sub.Plan.Slug)
	}

	// The plan the subscription points AT is the same row a base-price purchase
	// would point at. A level is priced on a copy of the authority row, and the
	// copy has to keep the row's identity: PlanId is what the renewal charger,
	// the invoice line and every entitlement read resolve through, so a copy that
	// lost it would leave the subscription pointing at nothing.
	base, err := resolveSubscriptionPlan(db, "max")
	if err != nil {
		t.Fatalf("resolve max: %v", err)
	}
	if sub.PlanId != base.Id() || sub.PlanId == "" {
		t.Fatalf("subscription PlanId=%q, want the max plan id %q", sub.PlanId, base.Id())
	}
	if int64(base.Price) != ladder[0] {
		t.Fatalf("the authority row was repriced to %d by a level purchase; it must still be %d",
			base.Price, ladder[0])
	}
}

// TestRenewalChargesTheChosenLevel is the one that matters: a customer who buys
// Max at a level is charged that level AGAIN at renewal, not the base price.
//
// It renews through the HTTP door, which re-reads the subscription by id, so the
// price under test is the one that survived storage — not one held in memory
// from the purchase. A subscription that silently dropped to $99 in month two
// would pass every test that never re-read it.
func TestRenewalChargesTheChosenLevel(t *testing.T) {
	ladder := maxLevels(t)
	const level = 5
	want := ladder[level]
	base := ladder[0]

	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("lvl-renew")
	m := squareMock("cust_r", "ccof_r", "sqpay_r")
	withFakeSquare(t, m)

	resp := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:ok","planId":"max","level":5}`, nil)
	if resp.StatusCode != http.StatusCreated {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("subscribe status=%d body=%s, want 201", resp.StatusCode, string(raw))
	}
	if int64(m.lastChargeAmount) != want {
		t.Fatalf("first charge=%d, want %d", m.lastChargeAmount, want)
	}

	db := datastore.New(org.Namespaced(ctx))
	sub := parentSub(t, db, "lvl-renew", "max")
	if sub == nil {
		t.Fatal("no max subscription created")
	}

	// Age the subscription so its current period has elapsed — a renewal only
	// bills a period that is actually due.
	fresh := subscription.New(db)
	if err := fresh.GetById(sub.Id()); err != nil {
		t.Fatalf("re-read subscription: %v", err)
	}
	fresh.PeriodStart = time.Now().AddDate(0, -2, 0)
	fresh.PeriodEnd = time.Now().AddDate(0, -1, 0)
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
	if int64(m.lastChargeAmount) == base {
		t.Fatalf("renewal charged the BASE price %d — the chosen level %d (%d) was lost between the purchase and the renewal",
			base, level, want)
	}
	if int64(m.lastChargeAmount) != want {
		t.Fatalf("renewal charged %d, want %d (level %d)", m.lastChargeAmount, want, level)
	}
	if m.lastChargeToken != "ccof_r" {
		t.Fatalf("renewal charged token=%q, want the vaulted card ccof_r", m.lastChargeToken)
	}

	// The renewal invoice is the level's price too — the receipt and the charge
	// agree, and a per-seat multiplier never crept in on a flat plan.
	invs := invoicesForSub(t, db, sub.Id())
	var renewal *billinginvoice.BillingInvoice
	for _, inv := range invs {
		if inv.PeriodStart.Equal(fresh.PeriodStart) {
			renewal = inv
		}
	}
	if renewal == nil {
		t.Fatalf("no invoice for the renewed period (%d invoices for the subscription)", len(invs))
	}
	if int64(renewal.AmountDue) != want {
		t.Fatalf("renewal invoice amountDue=%d, want %d", renewal.AmountDue, want)
	}
	if renewal.Status != billinginvoice.Paid {
		t.Fatalf("renewal invoice status=%s, want paid", renewal.Status)
	}
}

// TestAbsentLevelBuysTheBasePrice: every client that predates levels keeps
// buying exactly what it bought before, and naming level 0 explicitly is the
// same purchase as naming nothing.
func TestAbsentLevelBuysTheBasePrice(t *testing.T) {
	base := maxLevels(t)[0]

	for name, body := range map[string]string{
		"absent":  `{"sourceId":"cnon:ok","planId":"max"}`,
		"level 0": `{"sourceId":"cnon:ok","planId":"max","level":0}`,
	} {
		t.Run(name, func(t *testing.T) {
			ctx := ae.NewContext()
			defer ctx.Close()
			org := moneyOrg("lvl-base")
			m := squareMock("cust_b", "ccof_b", "sqpay_b")
			withFakeSquare(t, m)

			resp := invokeSubscribeCard(org, ctx, body, nil)
			if resp.StatusCode != http.StatusCreated {
				raw, _ := io.ReadAll(resp.Body)
				t.Fatalf("status=%d body=%s, want 201", resp.StatusCode, string(raw))
			}
			if int64(m.lastChargeAmount) != base {
				t.Fatalf("charged %d, want the base price %d", m.lastChargeAmount, base)
			}
		})
	}
}

// TestCatalogPublishesTheLadder: the prices a client may choose between reach the
// wire. A ladder the server holds and never publishes is one no page can render,
// and a client would have to hardcode the prices to draw the control — which is
// exactly the two-places-for-one-price the catalog exists to prevent.
func TestCatalogPublishesTheLadder(t *testing.T) {
	max := lookupPlan("max")
	if max == nil {
		t.Fatal("plan \"max\" missing from the catalog")
	}
	if len(max.Prices) == 0 {
		t.Fatal("the catalog serves no ladder for max; nothing can render the control")
	}
	if max.Prices[0] != max.Price {
		t.Fatalf("ladder starts at %d but the plan's price is %d; the first position on the control would quote one number and charge another",
			max.Prices[0], max.Price)
	}

	// The authority row carries it too, so an admin-edited catalog keeps serving a
	// ladder rather than dropping to a single price the moment a row is edited.
	row := planFromStatic(max)
	if len(row.Prices) != len(max.Prices) {
		t.Fatalf("seed row carries %d prices, catalog publishes %d", len(row.Prices), len(max.Prices))
	}
	back := staticPlanFromModel(row)
	for i := range max.Prices {
		if back.Prices[i] != max.Prices[i] {
			t.Fatalf("ladder position %d survived the row round-trip as %d, want %d", i, back.Prices[i], max.Prices[i])
		}
	}
}

// itoa keeps the table above readable without pulling strconv into a test file
// that otherwise has no arithmetic in it.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		return "-" + string(b)
	}
	return string(b)
}
