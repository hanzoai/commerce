package billing

import (
	"context"
	"testing"
	"time"

	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/paymentmethod"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
	types "github.com/hanzoai/commerce/types"
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
	m := legacySquare(t, "fp_legacy")
	legacyPlan(t, ctx, "uni-legacy")
	db := datastore.New(org.Namespaced(ctx))

	t.Cleanup(processor.SetChargePolicy(func() map[processor.ProcessorType]bool { return processor.ChargeNames("*") }))
	sale := legacySale(t, ctx, org, "legacy-renewal", "uni-legacy")
	pm := paymentmethod.New(db)
	if err := pm.GetById(sale.PaymentMethodID); err != nil {
		t.Fatal(err)
	}
	if providerOf(pm) != processor.Square {
		t.Fatalf("the card was saved with %s; want square", providerOf(pm))
	}

	t.Cleanup(processor.SetChargePolicy(func() map[processor.ProcessorType]bool { return processor.ChargeNames("stripe") }))
	t.Setenv("STRIPE_TEST_SECRET_KEY", "sk_test_present") // the default is Stripe alone where Stripe is keyed
	if _, ok := cardVaultFrom(processorsForOrg(org), ""); ok {
		t.Fatal("a NEW card could be saved with Square under the Stripe-only policy")
	}
	endPeriod(t, db, sale.SubscriptionID)
	before := m.chargeCalls // the sale charged the first period
	out := renewDue(ctx, org, "")
	if len(out) != 1 || !out[0].Success || m.chargeCalls != before+1 {
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
	legacySquare(t, "fp_repoint")
	legacyPlan(t, ctx, "uni-repoint")
	db := datastore.New(org.Namespaced(ctx))
	t.Cleanup(processor.SetChargePolicy(func() map[processor.ProcessorType]bool { return processor.ChargeNames("*") }))
	sale := legacySale(t, ctx, org, "repoint", "uni-repoint")
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

// legacySquare is a Square whose vault reports a card with fingerprint fp: the
// account an existing customer's card was saved with.
func legacySquare(t *testing.T, fp string) *mockSquareProcessor {
	t.Helper()
	m := squareMock("cust_legacy", "ccof_legacy", "sqpay_legacy")
	m.vaultCard = processor.Card{Fingerprint: fp, Brand: "VISA", Last4: "4242", ExpMonth: 12, ExpYear: 2030}
	withFakeSquare(t, m)
	return m
}

// legacyPlan is a monthly plan priced at $15.
func legacyPlan(t *testing.T, ctx context.Context, slug string) {
	t.Helper()
	pl := plan.New(plan.AuthorityDB(ctx))
	pl.Slug, pl.Name = slug, slug
	pl.Price, pl.Currency = currency.Cents(1500), currency.USD
	pl.Interval, pl.IntervalCount = types.Monthly, 1
	pl.Category, pl.Status, pl.Managed = "personal", plan.StatusActive, true
	pl.MustCreate()
}

// legacySale buys slug with a fresh card as subject.
func legacySale(t *testing.T, ctx context.Context, org *organization.Organization, subject, slug string) *Sale {
	t.Helper()
	sale, err := Subscribe(ctx, org, SubscribeIn{SourceID: "cnon:ok", PlanID: slug, Subject: subject, IdempotencyKey: subject + ":" + slug})
	if err != nil {
		t.Fatalf("sale of %s to %s: %v", slug, subject, err)
	}
	return sale
}

// endPeriod moves a subscription's period into the past, so its renewal is due.
func endPeriod(t *testing.T, db *datastore.Datastore, id string) {
	t.Helper()
	s := reload(t, db, id)
	shift := time.Until(s.PeriodEnd) + time.Hour
	s.PeriodStart, s.PeriodEnd = s.PeriodStart.Add(-shift), s.PeriodEnd.Add(-shift)
	if err := s.Update(); err != nil {
		t.Fatalf("end period: %v", err)
	}
}
