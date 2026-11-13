package billing

import (
	"testing"

	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/paymentmethod"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/util/test/ae"
)

// A customer who bought a plan with a Square card keeps renewing on that card
// after the deployment's policy turns to Stripe alone: the policy governs where
// NEW cards are saved, not whether a subscription lapses. The sale itself runs
// under the config that allows Square (it is how the customer got there); the
// renewal runs under the default.
func TestExistingSquareCardKeepsRenewingUnderTheStripeOnlyPolicy(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("legacy-renewal")
	m := trialSquare(t, "fp_legacy")
	trialPlan(t, ctx, "uni-legacy", trialCents, 7)
	db := datastore.New(org.Namespaced(ctx))

	t.Setenv(processor.ChargeProcessors, "*")
	sale := buyTrial(t, ctx, org, "legacy-renewal", "uni-legacy")
	pm := paymentmethod.New(db)
	if err := pm.GetById(sale.PaymentMethodID); err != nil {
		t.Fatal(err)
	}
	if providerOf(pm) != processor.Square {
		t.Fatalf("the card was saved with %s; want square", providerOf(pm))
	}

	t.Setenv(processor.ChargeProcessors, "")
	t.Setenv("STRIPE_TEST_SECRET_KEY", "sk_test_present") // the default is Stripe alone where Stripe is keyed
	if _, ok := cardVaultFrom(processorsForOrg(org)); ok {
		t.Fatal("a NEW card could be saved with Square under the Stripe-only policy")
	}
	endTrial(t, db, sale.SubscriptionID)
	out := renewDue(ctx, org, "")
	if len(out) != 1 || !out[0].Success || m.chargeCalls != 1 {
		t.Fatalf("renewal under the Stripe-only policy = %+v (square charges %d); want one success on the customer's own card", out, m.chargeCalls)
	}
	if got := reload(t, db, sale.SubscriptionID); got.ProviderType != string(processor.Square) || engine.Over(got, got.PeriodStart) {
		t.Fatalf("subscription = %s over=%v", got.ProviderType, engine.Over(got, got.PeriodStart))
	}
}

// A Square holder's renewals move to a Stripe card when one is saved or made the
// default — and only then; and a card removed hands its renewals to another.
func TestRenewalsMoveToAStripeCardAndOffARemovedOne(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("repoint")
	trialSquare(t, "fp_repoint")
	trialPlan(t, ctx, "uni-repoint", trialCents, 7)
	db := datastore.New(org.Namespaced(ctx))
	t.Setenv(processor.ChargeProcessors, "*")
	sale := buyTrial(t, ctx, org, "repoint", "uni-repoint")
	square := paymentmethod.New(db)
	if err := square.GetById(sale.PaymentMethodID); err != nil {
		t.Fatal(err)
	}

	stripeCard := func(ref string, def bool) *paymentmethod.PaymentMethod {
		pm := paymentmethod.New(db)
		pm.CustomerId, pm.UserId, pm.Type = "repoint", "repoint", "card"
		pm.ProviderRef, pm.ProviderType, pm.IsDefault = ref, string(processor.Stripe), def
		pm.Metadata = map[string]interface{}{"customerId": "cus_x"}
		if err := pm.Create(); err != nil {
			t.Fatal(err)
		}
		return pm
	}
	held := func() string { return reload(t, db, sale.SubscriptionID).DefaultPaymentMethod }

	if held() != square.Id() {
		t.Fatalf("the sale's card %s is not the subscription's default %s", square.Id(), held())
	}
	// Making a card the default clears the others (SetDefaultMethod).
	square.IsDefault = false
	if err := square.Update(); err != nil {
		t.Fatal(err)
	}
	first := stripeCard("pm_first", true)
	repointRenewals(ctx, org, "repoint", first)
	if held() != first.Id() {
		t.Fatalf("after a Stripe card was saved the subscription renews on %s; want %s", held(), first.Id())
	}
	// Already on Stripe: a second Stripe card does not take it over.
	second := stripeCard("pm_second", false)
	repointRenewals(ctx, org, "repoint", second)
	if held() != first.Id() {
		t.Fatalf("a second Stripe card took the renewals: %s", held())
	}
	// No subject names no subscriptions: nothing moves, and no stranger's card is an heir.
	moveRenewalsOff(ctx, org, "", first)
	if held() != first.Id() {
		t.Fatal("an empty subject moved renewals")
	}
	if heirOf(db, "", first.Id()) != nil {
		t.Fatal("an empty subject found an heir")
	}
	// The card is removed: its renewals go to the other.
	moveRenewalsOff(ctx, org, "repoint", first)
	if held() != second.Id() {
		t.Fatalf("after the card was removed the subscription renews on %s; want %s", held(), second.Id())
	}
}
