package billing

import (
	"context"
	"testing"

	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/payment"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/payment/routing"
)

func TestDisabledByEnvHardDisablesStripeAcrossRuntimeRoutingAndConfig(t *testing.T) {
	ctx := context.Background()
	t.Setenv("COMMERCE_DISABLED_PROCESSORS", "stripe")

	if !processor.DisabledByEnv(processor.Stripe) {
		t.Fatal("expected DisabledByEnv(stripe) = true")
	}
	if !processor.DisabledByPolicy(processor.Stripe) {
		t.Fatal("expected DisabledByPolicy(stripe) = true")
	}
	if processor.DisabledByEnv(processor.Square) {
		t.Fatal("expected DisabledByEnv(square) = false")
	}

	// StripeKeys returns empty even when an org has its own live Stripe keys.
	org := &organization.Organization{}
	org.Name = "hanzo"
	org.Live = true
	org.Stripe.Live.AccessToken = "sk_live_org_key"
	org.Stripe.Live.PublishableKey = "pk_live_org_key"
	if got := payment.StripeKeys(org); got.SecretKey != "" || got.PublishableKey != "" {
		t.Fatalf("StripeKeys = %+v, want empty when stripe is hard-disabled", got)
	}

	// ReadPaymentConfig hides Stripe publishable key and sets Enabled=false.
	pc := ReadPaymentConfig(ctx, org)
	if pc.Stripe.Enabled || pc.Stripe.PublishableKey != "" {
		t.Fatalf("ReadPaymentConfig.Stripe = %+v, want zeroed", pc.Stripe)
	}
	for cur, proc := range pc.Cards {
		if proc == string(processor.Stripe) {
			t.Fatalf("ReadPaymentConfig.Cards[%q] = stripe, want no stripe routing", cur)
		}
	}

	// applyEnvDisabledProcessors clamps Stripe off even when a stored policy has it on,
	// and routing cleanly places US and non-US buyers on Square.
	pol := applyEnvDisabledProcessors(routing.Default())
	if pol.On(processor.Stripe) {
		t.Fatal("expected stripe On=false after applyEnvDisabledProcessors")
	}
	if pol.StripeCrypto() {
		t.Fatal("expected StripeCrypto()=false when stripe is hard-disabled")
	}
	for _, country := range []string{"US", "DE", "BR", "IN"} {
		d := pol.Decide(routing.Input{
			Org:      "hanzo",
			Key:      "buyer-" + country,
			Country:  country,
			Currency: "usd",
			Capable:  []routing.Capability{{Processor: processor.Square, OK: true}},
		})
		if d.Processor != processor.Square {
			t.Fatalf("country=%s routed to %q (rule=%s), want square", country, d.Processor, d.Rule)
		}
	}

	// chargeProblems reports zero alarms for a hard-disabled Stripe processor.
	polWithStripe := routing.Default()
	polWithStripe.Accounts = []routing.Account{
		{Processor: "square", ID: "live", On: true, Cards: true, Settings: []routing.Named{{Name: "locationId", Value: "LOC"}}},
		{Processor: "stripe", ID: "live", On: true, Cards: true, Secrets: []routing.Named{{Name: "secretKey", Value: "/commerce-secrets/STRIPE_SECRET_KEY"}}},
	}
	for _, prob := range chargeProblems(ctx, polWithStripe) {
		if len(prob) >= 6 && prob[:6] == "stripe" {
			t.Fatalf("unexpected stripe alarm when hard-disabled: %s", prob)
		}
	}
}
