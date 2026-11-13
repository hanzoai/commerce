package billing

import (
	"net/http"
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billingevent"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/util/test/ae"
)

// The ladder end to end, on a real store: the @hanzo/plans 1.10.1 catalog is
// seeded into the SQLite database the test context opens, as a boot does; every
// rung is bought through POST /v1/billing/subscribe/card and charged its list
// price on the card rail; each buyer's default reload is switched on and the
// recharge sweep charges it once the balance is below $10; and a holder of a
// retired rung is moved onto the ladder by the next boot and renewed at the new
// price. The card rail is commerce's Square processor interface answered in
// process (withFakeSquare): Square's sandbox needs credentials this test does not
// carry.
func TestE2E_TheLadderIsSoldReloadedAndMigrated(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	if _, _, err := SeedPlans(ctx); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// The public catalog sells the ladder, in order, at its list prices, each
	// with the default reload terms; the archived rungs are not on it.
	plans, err := ReadPlans(ctx, "pay.hanzo.ai", categoryPersonal, nil)
	if err != nil {
		t.Fatalf("read plans: %v", err)
	}
	ladder := []struct {
		slug  string
		cents int64
	}{{"go", 800}, {"pro-3x", 1500}, {"dev-5x", 4500}, {"max-15x", 9500}, {"ultra", 19500}, {"ultimate", 49500}, {"unlimited", 99500}}
	listed := map[string]PlanView{}
	order := []string{}
	for _, p := range plans {
		listed[p.Slug] = p
		order = append(order, p.Slug)
	}
	for i, r := range ladder {
		p, ok := listed[r.slug]
		if !ok || p.Price != r.cents {
			t.Fatalf("the catalog lists %s at %d (listed %v), want %d: %v", r.slug, p.Price, ok, r.cents, order)
		}
		if p.Reload == nil || p.Reload.ThresholdCents != 1000 || p.Reload.AmountCents != 5000 {
			t.Fatalf("%s shows the reload %+v, want $50 below $10", r.slug, p.Reload)
		}
		if i > 0 && p.Price <= listed[ladder[i-1].slug].Price {
			t.Fatalf("the ladder does not climb at %s", r.slug)
		}
	}
	for _, slug := range []string{"dev", "max-5x", "max-20x", "max-50x"} {
		if _, ok := listed[slug]; ok {
			t.Fatalf("the archived %s is on sale", slug)
		}
	}

	// Every rung is bought by card at its list price, opens an active, paid
	// period, and switches the default reload on.
	for _, r := range ladder {
		org := payerOrg(t, ctx, "e2e-"+r.slug)
		m := squareMock("cust_"+r.slug, "ccof_"+r.slug, "sqpay_"+r.slug)
		withFakeSquare(t, m)
		resp := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:ok","planId":"`+r.slug+`"}`, nil)
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("%s by card: %d %s", r.slug, resp.StatusCode, bodyOf(resp))
		}
		out := jsonBody(t, resp)
		if m.chargeCalls != 1 || m.lastChargeAmount != r.cents || out["amountCents"] != float64(r.cents) {
			t.Fatalf("%s charged %d time(s), last %d, receipt %v; want once at %d", r.slug, m.chargeCalls, m.lastChargeAmount, out["amountCents"], r.cents)
		}
		if reloadState(out) != armed {
			t.Fatalf("%s receipt reload %v, want armed", r.slug, out["reload"])
		}
		db := datastore.New(org.Namespaced(ctx))
		sub := parentSub(t, db, org.Name, r.slug)
		if sub == nil || sub.Status != subscription.Active || !sub.TrialEnd.IsZero() {
			t.Fatalf("%s opened %+v, want an active subscription with no trial", r.slug, sub)
		}
		invs := invoicesForSub(t, db, sub.Id())
		if len(invs) != 1 || invs[0].Status != billinginvoice.Paid || invs[0].AmountPaid != r.cents {
			t.Fatalf("%s first invoice %+v, want one paid at %d", r.slug, invs, r.cents)
		}
		if c := storedRule(t, ctx, org); c == nil || !c.Enabled || c.Plan != r.slug || c.ThresholdCents != 1000 || c.AmountCents != 5000 {
			t.Fatalf("%s left the rule %+v, want it on at $50 below $10", r.slug, c)
		}
	}

	// The reload: every buyer's balance is $0, below the $10 threshold, so one
	// sweep charges each buyer's card $50 and credits it; a second sweep finds every
	// balance funded and charges nothing; spent below $10 again, the next sweep
	// reloads that buyer alone. One card rail answers every org from here on.
	goOrg := payerOrg(t, ctx, "e2e-reload")
	m := squareMock("cust_reload", "ccof_reload", "sqpay_reload")
	withFakeSquare(t, m)
	if r := invokeSubscribeCard(goOrg, ctx, `{"sourceId":"cnon:ok","planId":"go"}`, nil); r.StatusCode != http.StatusCreated {
		t.Fatalf("go for the reload: %d %s", r.StatusCode, bodyOf(r))
	}
	buyers := []string{goOrg.Name}
	for _, r := range ladder {
		buyers = append(buyers, "e2e-"+r.slug)
	}
	charges := m.chargeCalls
	if run := rechargeRun(t, ctx); run.Charged != len(buyers) || m.chargeCalls != charges+len(buyers) || m.lastChargeAmount != 5000 {
		t.Fatalf("the first sweep charged %d card(s) (%d calls, last %d), want each of the %d buyers once at $50",
			run.Charged, m.chargeCalls-charges, m.lastChargeAmount, len(buyers))
	}
	for _, name := range buyers {
		if b := walletOf(t, ctx, moneyOrg(name), name); b != 5000 {
			t.Fatalf("%s holds %d after the sweep, want the $50 reload credited", name, b)
		}
	}
	charges = m.chargeCalls
	if run := rechargeRun(t, ctx); run.Charged != 0 || m.chargeCalls != charges {
		t.Fatalf("funded balances were reloaded: %d charges", m.chargeCalls-charges)
	}
	spend(datastore.New(goOrg.Namespaced(ctx)), goOrg.Name, 4200, "usage")
	m.chargeRef = "sqpay_reload_2" // each charge Square answers names its own payment
	if run := rechargeRun(t, ctx); run.Charged != 1 || m.chargeCalls != charges+1 || walletOf(t, ctx, goOrg, goOrg.Name) != 5800 {
		t.Fatalf("spent to $8, the sweep charged %d card(s) and left %d; want one second $50 reload, $58",
			m.chargeCalls-charges, walletOf(t, ctx, goOrg, goOrg.Name))
	}

	// A holder of a retired rung — Pro at $20, archived — is moved by the next boot
	// onto the dearest rung listed at or below what it pays, pro-3x at $15, with an
	// audit record, and its renewal charges the new price.
	held := payerOrg(t, ctx, "e2e-retired")
	hm := squareMock("cust_ret", "ccof_ret", "sqpay_ret")
	withFakeSquare(t, hm)
	hdb := datastore.New(held.Namespaced(ctx))
	old := seedCardBackedSub(t, hdb, held.Name, "dev", "ccof_ret", "cust_ret")
	if _, _, err := SeedPlans(ctx); err != nil {
		t.Fatalf("the next boot: %v", err)
	}
	moved := reload(t, hdb, old.Id())
	if subscriptionSlug(moved) != "pro-3x" || moved.Plan.Price != 1500 {
		t.Fatalf("after the boot the holder is on %s at %d, want pro-3x at 1500", subscriptionSlug(moved), moved.Plan.Price)
	}
	var audit []*billingevent.BillingEvent
	if _, err := billingevent.Query(hdb).Filter("Type=", "subscription.migrated").GetAll(&audit); err != nil || len(audit) != 1 ||
		audit[0].Data["from"] != "dev" || audit[0].Data["to"] != "pro-3x" {
		t.Fatalf("audit %v (%v), want one record of dev -> pro-3x", audit, err)
	}
	if out := renewDue(ctx, held, ""); len(out) != 1 || !out[0].Success || out[0].AmountCents != 1500 || hm.lastChargeAmount != 1500 {
		t.Fatalf("the moved holder's renewal: %+v (card %d), want pro-3x's 1500", out, hm.lastChargeAmount)
	}
	if armedAt(t, ctx, held, held.Name) {
		t.Fatal("the migration switched a reload on for a holder who never saw its terms")
	}
}
