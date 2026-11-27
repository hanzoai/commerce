package checkout

import (
	"testing"

	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/payment/processor"
)

func TestOrgResolverHidesStripeWhenDisabledByPolicyOrEnv(t *testing.T) {
	org := &organization.Organization{}
	org.Name = "hanzo"
	org.Live = true
	org.Stripe.Live.AccessToken = "sk_live_secret"
	org.Stripe.Live.PublishableKey = "pk_live_public"
	org.Square.Production.AccessToken = "sq_live_token"
	org.Square.Production.LocationId = "LOC123"
	org.Square.Production.ApplicationId = "sq0idp-app"

	restoreHard := processor.SetHardDisabledPolicy(func() map[processor.ProcessorType]bool {
		return processor.ChargeNames("stripe")
	})
	defer restoreHard()
	restore := processor.SetChargePolicy(func() map[processor.ProcessorType]bool {
		return processor.ChargeNames("square,paypal,kraken,mpc")
	})
	defer restore()

	r := NewOrgResolver(func(slug string) (*organization.Organization, bool) {
		return org, true
	})
	got, err := r.Resolve("pay.hanzo.ai")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Stripe.PublishableKey != "" {
		t.Fatalf("Stripe.PublishableKey = %q, want empty when stripe is disabled", got.Stripe.PublishableKey)
	}
	for _, p := range got.Providers {
		if p.Name == "stripe" {
			t.Fatalf("stripe surfaced in Providers: %+v", got.Providers)
		}
	}
	if _, err := got.reg.Registered(processor.Stripe); err == nil {
		t.Fatal("stripe remained registered on org resolver registry")
	}
}
