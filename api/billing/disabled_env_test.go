package billing

import (
	"context"
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/paymentmethod"
	"github.com/hanzoai/commerce/payment"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/payment/routing"
	"github.com/hanzoai/commerce/util/test/ae"
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
	// disables LocalOn, rewrites Fallback to square, promotes Square weight when zero,
	// strips Stripe cryptoRails, and routes US and non-US buyers to Square.
	stored := routing.Default()
	stored.LocalOn = true
	stored.Fallback = []string{"stripe"}
	stored.Processors = []routing.ProcessorRule{
		{Processor: "square", On: true, Weight: 0},
		{Processor: "stripe", On: true, Weight: 100},
	}
	pol := applyEnvDisabledProcessors(stored)
	if pol.On(processor.Stripe) {
		t.Fatal("expected stripe On=false after applyEnvDisabledProcessors")
	}
	if pol.LocalOn {
		t.Fatal("expected LocalOn=false when stripe is hard-disabled")
	}
	if pol.FXSource != routing.FXReference {
		t.Fatalf("FXSource = %q, want %q", pol.FXSource, routing.FXReference)
	}
	if len(pol.Fallback) != 1 || pol.Fallback[0] != "square" {
		t.Fatalf("Fallback = %v, want [square]", pol.Fallback)
	}
	if pol.StripeCrypto() {
		t.Fatal("expected StripeCrypto()=false when stripe is hard-disabled")
	}
	for _, r := range pol.CryptoRails {
		if r.Processor == "stripe" {
			t.Fatalf("CryptoRails retained disabled stripe route: %+v", r)
		}
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

func TestDisabledStripeReturningCustomerVaultsSameCardOnSquareAndHidesStripeFromList(t *testing.T) {
	t.Setenv("COMMERCE_DISABLED_PROCESSORS", "stripe")
	ctx := ae.NewContext()
	defer ctx.Close()

	org := moneyOrg("retbuyer")
	db := datastore.New(org.Namespaced(ctx))

	// Existing customer previously vaulted Visa *4242 (12/2029) as default on Stripe.
	oldStripe := paymentmethod.New(db)
	oldStripe.SetKey(db.NewKey("paymentmethod", "", 0, db.NewKey("synckey", "", 1, nil)))
	oldStripe.CustomerId = "retbuyer"
	oldStripe.UserId = "retbuyer"
	oldStripe.Type = "card"
	oldStripe.ProviderRef = "pm_frozen_stripe"
	oldStripe.ProviderType = "stripe"
	oldStripe.IsDefault = true
	oldStripe.Card = &paymentmethod.CardDetails{Brand: "Visa", Last4: "4242", ExpMonth: 12, ExpYear: 2029}
	oldStripe.Metadata = map[string]interface{}{"customerId": "cus_frozen", "cardId": "pm_frozen_stripe", "verified": true}
	if err := oldStripe.Create(); err != nil {
		t.Fatalf("create old stripe card: %v", err)
	}

	// ListMethods hides the unchargeable Stripe card while Stripe is disabled.
	listed, err := ListMethods(ctx, org, "retbuyer", "")
	if err != nil {
		t.Fatalf("ListMethods: %v", err)
	}
	if len(listed) != 0 {
		t.Fatalf("ListMethods returned %d methods (%+v), want 0 while stripe is disabled", len(listed), listed)
	}

	// Re-entering the exact same physical Visa *4242 card on Square vaults cleanly
	// (never matching the disabled Stripe row) and promotes the Square card to default.
	m := squareMock("sq_cust_1", "ccof_new_sq", "sqpay_1")
	m.vaultCard = processor.Card{ID: "ccof_new_sq", Brand: "Visa", Last4: "4242", ExpMonth: 12, ExpYear: 2029, Fingerprint: "fp_sq"}
	withFakeSquare(t, m)

	saved, created, err := saveCard(ctx, db, m, "retbuyer", processor.Payer{}, "cnon:card-nonce-ok")
	if err != nil {
		t.Fatalf("saveCard on Square: %v", err)
	}
	if !created || saved.ProviderType != "square" || saved.ProviderRef != "ccof_new_sq" {
		t.Fatalf("saveCard returned created=%v type=%q ref=%q, want true/square/ccof_new_sq", created, saved.ProviderType, saved.ProviderRef)
	}
	if !saved.IsDefault {
		t.Fatal("expected new Square card to take over IsDefault from disabled Stripe card")
	}

	listedAfter, err := ListMethods(ctx, org, "retbuyer", "")
	if err != nil {
		t.Fatalf("ListMethods after Square save: %v", err)
	}
	if len(listedAfter) != 1 || listedAfter[0].ProviderType != "square" || listedAfter[0].ProviderRef != "ccof_new_sq" {
		t.Fatalf("ListMethods after Square save = %+v, want single square card", listedAfter)
	}
}
