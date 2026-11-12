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
