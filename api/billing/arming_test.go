package billing

import (
	stdjson "encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/autorecharge"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/test/ae"
)

// saveCard saves a card for subject through the payment-method core.
func saveCardFor(t *testing.T, ctx ae.Context, org *organization.Organization, subject string) {
	t.Helper()
	if _, _, err := CreateMethod(ctx, org, "buyer@acme.test", CreateMethodIn{
		CustomerId: subject, Type: "card", ProviderRef: "cnon:ok",
	}); err != nil {
		t.Fatalf("save card: %v", err)
	}
}

// waitingRule writes the rule a sale that answered needs_card leaves for payer.
func waitingRule(t *testing.T, db *datastore.Datastore, payer, slug string) {
	t.Helper()
	cfg := autorecharge.New(db)
	cfg.UserId, cfg.Plan, cfg.Waiting, cfg.Currency = payer, slug, true, "usd"
	if err := cfg.Create(); err != nil {
		t.Fatal(err)
	}
}

// Saving a card switches on the reload a sale left waiting for one, and nothing
// else: not a plan bought before its row stated a reload, and not a trial.
func TestSavingACardArmsOnlyAWaitingReload(t *testing.T) {
	t.Run("waiting", func(t *testing.T) {
		ctx := ae.NewContext()
		defer ctx.Close()
		reloadPlan(t, ctx, "ultra", 19500, nil, ultraReload)
		org := moneyOrg("save-waiting")
		db := datastore.New(org.Namespaced(ctx))
		seedCredit(db, org.Name, 19500, "topup", time.Time{})
		withFakeSquare(t, squareMock("cust_sw", "ccof_sw", ""))

		out := subscribeAs(t, ctx, org, `{"sourceId":"balance","planId":"ultra"}`, nil)
		if got := reloadState(out); got != needsCard {
			t.Fatalf("receipt reload state %q, want %q", got, needsCard)
		}
		if c := loadAutoRecharge(db, org.Name); c == nil || !c.Waiting || c.Enabled || c.Plan != "ultra" {
			t.Fatalf("the sale left %+v, want the rule off and waiting for ultra", c)
		}
		saveCardFor(t, ctx, org, org.Name)
		if !armedAt(t, ctx, org, org.Name) {
			t.Fatal("saving a card did not switch on the waiting reload")
		}
	})
	t.Run("bought before the row stated a reload", func(t *testing.T) {
		ctx := ae.NewContext()
		defer ctx.Close()
		reloadPlan(t, ctx, "ultra", 19500, nil, ultraReload)
		org := moneyOrg("save-before")
		db := datastore.New(org.Namespaced(ctx))
		seedSub(t, db, org.Name, "ultra", subscription.Active, "square", "")
		withFakeSquare(t, squareMock("cust_sb", "ccof_sb", ""))

		saveCardFor(t, ctx, org, org.Name)
		if c := loadAutoRecharge(db, org.Name); c != nil {
			t.Fatalf("saving a card switched on a reload the buyer never saw: %+v", c)
		}
	})
	t.Run("trial", func(t *testing.T) {
		ctx := ae.NewContext()
		defer ctx.Close()
		reloadPlan(t, ctx, "ultra", 19500, nil, ultraReload)
		org := moneyOrg("save-trial")
		db := datastore.New(org.Namespaced(ctx))
		seedSub(t, db, org.Name, "ultra", subscription.Trialing, "square", "")
		waitingRule(t, db, org.Name, "ultra")
		withFakeSquare(t, squareMock("cust_st", "ccof_st", ""))

		saveCardFor(t, ctx, org, org.Name)
		if c := loadAutoRecharge(db, org.Name); c == nil || c.Enabled {
			t.Fatalf("saving a card switched on a trialing payer's reload: %+v", c)
		}
	})
}

// subscriptionOn is payer's live subscription to slug.
func subscriptionOn(t *testing.T, ctx ae.Context, org *organization.Organization, payer, slug string) *subscription.Subscription {
	t.Helper()
	s := parentSub(t, datastore.New(org.Namespaced(ctx)), payer, slug)
	if s == nil {
		t.Fatalf("%s holds no %s subscription", payer, slug)
	}
	return s
}

// Cancelling a plan — now or at its period's end — switches off the reload it
// switched on. A rule the payer set themselves is left as they set it.
func TestCancellingAPlanStopsItsReload(t *testing.T) {
	for name, end := range map[string]bool{"now": false, "at period end": true} {
		t.Run(name, func(t *testing.T) {
			ctx := ae.NewContext()
			defer ctx.Close()
			reloadPlan(t, ctx, "ultra", 19500, nil, ultraReload)
			org := moneyOrg("cancel-reload")
			withFakeSquare(t, squareMock("cust_cr", "ccof_cr", "sqpay_cr"))
			subscribeAs(t, ctx, org, `{"sourceId":"cnon:ok","planId":"ultra"}`, nil)

			if _, err := CancelSubscription(ctx, org, AnyHolder, subscriptionOn(t, ctx, org, org.Name, "ultra").Id(), end); err != nil {
				t.Fatalf("cancel: %v", err)
			}
			if c := storedRule(t, ctx, org); c == nil || c.Enabled || c.Plan != "" {
				t.Fatalf("after the cancel the rule is %+v, want it off", c)
			}
		})
	}
	t.Run("the payer's own rule", func(t *testing.T) {
		ctx := ae.NewContext()
		defer ctx.Close()
		reloadPlan(t, ctx, "ultra", 19500, nil, ultraReload)
		org := moneyOrg("cancel-own")
		db := datastore.New(org.Namespaced(ctx))
		cfg := autorecharge.New(db)
		cfg.UserId, cfg.Enabled, cfg.ThresholdCents, cfg.AmountCents, cfg.Currency = org.Name, true, 2000, 7000, "usd"
		if err := cfg.Create(); err != nil {
			t.Fatal(err)
		}
		withFakeSquare(t, squareMock("cust_co", "ccof_co", "sqpay_co"))
		out := subscribeAs(t, ctx, org, `{"sourceId":"cnon:ok","planId":"ultra"}`, nil)
		if got := reloadState(out); got != kept {
			t.Fatalf("receipt reload state %q, want %q", got, kept)
		}
		if _, err := CancelSubscription(ctx, org, AnyHolder, subscriptionOn(t, ctx, org, org.Name, "ultra").Id(), false); err != nil {
			t.Fatalf("cancel: %v", err)
		}
		if c := storedRule(t, ctx, org); !c.Enabled || c.AmountCents != 7000 {
			t.Fatalf("the payer's own rule became %+v, want it on at $70", c)
		}
	})
}

// Moving to a plan whose row states no reload — buying it, or moving the
// subscription onto it — switches off the reload the plan left switched on.
func TestMovingOffAPlanStopsItsReload(t *testing.T) {
	t.Run("a sale of the next plan", func(t *testing.T) {
		ctx := ae.NewContext()
		defer ctx.Close()
		reloadPlan(t, ctx, "ultra", 19500, nil, ultraReload)
		reloadPlan(t, ctx, "plain", 2000, nil, nil)
		org := moneyOrg("move-sale")
		withFakeSquare(t, squareMock("cust_ms", "ccof_ms", "sqpay_ms"))
		subscribeAs(t, ctx, org, `{"sourceId":"cnon:ok","planId":"ultra"}`, nil)
		if !armedAt(t, ctx, org, org.Name) {
			t.Fatal("ultra did not switch the reload on")
		}
		subscribeAs(t, ctx, org, `{"sourceId":"cnon:ok","planId":"plain"}`, nil)
		if c := storedRule(t, ctx, org); c.Enabled || c.Plan != "" {
			t.Fatalf("after the move the rule is %+v, want it off", c)
		}
	})
	t.Run("a move of the subscription", func(t *testing.T) {
		ctx := ae.NewContext()
		defer ctx.Close()
		reloadPlan(t, ctx, "ultra", 19500, nil, ultraReload)
		reloadPlan(t, ctx, "plain", 2000, nil, nil)
		org := moneyOrg("acme")
		withFakeSquare(t, squareMock("cust_mm", "ccof_mm", "sqpay_mm"))
		subscribeAs(t, ctx, org, `{"sourceId":"cnon:ok","planId":"ultra"}`, nil)
		id := subscriptionOn(t, ctx, org, org.Name, "ultra").Id()
		if r := invokeSubPatch(org, ctx, c1MintPrincipal, id, `{"planId":"plain"}`); r.StatusCode != http.StatusOK {
			t.Fatalf("move: %d %s", r.StatusCode, bodyOf(r))
		}
		if c := storedRule(t, ctx, org); c.Enabled || c.Plan != "" {
			t.Fatalf("after the move the rule is %+v, want it off", c)
		}
	})
}

// A reload charge records whose payment it is, so a refund of it takes the
// credit back out of the payer's wallet and switches the payer's reload off.
func TestARefundedReloadStopsTheReload(t *testing.T) {
	const secret = "whsec_reload_refund"
	registerSquare(t, secret)
	led := newFakeLedger()
	injectLedger(t, led)
	ctx := ae.NewContext()
	defer ctx.Close()
	org := payerOrg(t, ctx, "hanzo")
	payerRule(t, ctx, org, "hanzo/ivy", 2500, 1000)
	m := squareMock("", "", "sqpay_ivy")
	withFakeSquare(t, m)

	if run := rechargeRun(t, ctx); run.Charged != 1 {
		t.Fatalf("the reload did not charge: %+v", run)
	}
	deliver(t, ctx, secret, refundEvent("evt_rf_ivy", "refund.updated", "sqrf_ivy", "sqpay_ivy", "COMPLETED", 2500))

	if len(led.clawbacks) != 1 || led.clawbacks[0].Subject != "hanzo/ivy" || led.clawbacks[0].Payment != "sqpay_ivy" {
		t.Fatalf("clawbacks %+v, want the reload's 2500 taken back from hanzo/ivy", led.clawbacks)
	}
	if c := loadAutoRecharge(datastore.New(org.Namespaced(ctx)), "hanzo/ivy"); c == nil || c.Enabled {
		t.Fatalf("a refunded reload left the rule %+v, want it off", c)
	}
	if run := rechargeRun(t, ctx); run.Charged != 0 || m.chargeCalls != 1 {
		t.Fatalf("the card was charged again after the refund: %+v", run)
	}
}

// CheckReload is the top-up's own bound: no reload at all, or $5 to $5,000 below a
// threshold of $0 to $5,000.
func TestCheckReloadIsTheTopUpsBounds(t *testing.T) {
	for _, tc := range []struct {
		threshold, amount int64
		ok                bool
	}{
		{0, 0, true},
		{1000, 0, true},
		{1000, 5000, true},
		{0, 500, true},
		{500000, 500000, true},
		{0, 499, false},
		{0, 500001, false},
		{-1, 5000, false},
		{500001, 5000, false},
	} {
		err := CheckReload(tc.threshold, tc.amount)
		if (err == nil) != tc.ok {
			t.Errorf("CheckReload(%d, %d) = %v, want ok %v", tc.threshold, tc.amount, err, tc.ok)
		}
		if err != nil && !IsAutoRechargeRefused(err) {
			t.Errorf("CheckReload(%d, %d) refused with %v, not the customer's sentence", tc.threshold, tc.amount, err)
		}
	}
}

// A personal plan is sold by the month: its row's annual price is not served, and
// its reload terms are.
func TestAPersonalPlanServesItsReloadAndNoAnnualPrice(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	if _, _, err := SeedPlans(ctx); err != nil {
		t.Fatalf("seed: %v", err)
	}
	p := plan.New(plan.AuthorityDB(ctx))
	p.Slug, p.Name, p.Category, p.Currency = "ultra", "Ultra", "personal", currency.USD
	p.Price, p.PriceAnnual, p.AnnualTotal = 19500, 16250, 195000
	zero := 0
	p.Limits = &plan.Limits{ReloadsPerDay: &zero, Reload: ultraReload}
	p.AdminEdited, p.Managed = true, true
	if err := p.Create(); err != nil {
		t.Fatal(err)
	}
	plans, err := ReadPlans(ctx, "", "", nil)
	if err != nil {
		t.Fatalf("read plans: %v", err)
	}
	raw, _ := stdjson.Marshal(plans)
	var rows []map[string]any
	if err := stdjson.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if r["slug"] != "ultra" {
			continue
		}
		if r["priceAnnual"] != nil || r["annualTotal"] != nil {
			t.Errorf("ultra serves an annual price: priceAnnual=%v annualTotal=%v", r["priceAnnual"], r["annualTotal"])
		}
		want := map[string]any{"thresholdCents": float64(1000), "amountCents": float64(5000), "perDay": float64(0)}
		got, _ := r["reload"].(map[string]any)
		for k, v := range want {
			if got[k] != v {
				t.Errorf("ultra serves reload %v, want %v", got, want)
				break
			}
		}
		return
	}
	t.Fatal("the catalog does not list ultra")
}

// An org's admin buying a plan for a member switches no reload on: the buyer who
// saw the terms is not the payer a reload would charge.
func TestAnAdminBuyingForAMemberArmsNothing(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	reloadPlan(t, ctx, "ultra", 19500, nil, ultraReload)
	org := moneyOrg("acme")
	withFakeSquare(t, squareMock("cust_ab", "ccof_ab", "sqpay_ab"))

	out := subscribeAs(t, ctx, org, `{"sourceId":"cnon:ok","planId":"ultra","userId":"acme/bob"}`, nil)
	if _, ok := out["reload"]; ok {
		t.Fatalf("a sale for a member answered reload %v", out["reload"])
	}
	db := datastore.New(org.Namespaced(ctx))
	for _, payer := range []string{"acme/bob", "acme"} {
		if c := loadAutoRecharge(db, payer); c != nil {
			t.Errorf("a sale for a member wrote a rule for %s: %+v", payer, c)
		}
	}
}
