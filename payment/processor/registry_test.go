package processor_test

import (
	"context"
	"testing"

	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/payment/providers/square"
	"github.com/hanzoai/commerce/payment/providers/stripe"
)

// withDefaultDisabledPolicy lets every processor take a new charge for the test
// (COMMERCE_CHARGE_PROCESSORS=*), so the tests of the selection rule see it
// across processors. The deployment default is Stripe alone: see
// TestChargePolicy_DefaultIsStripeOnly.
func withDefaultDisabledPolicy(t *testing.T) {
	t.Helper()
	t.Setenv(processor.ChargeProcessors, "*")
}

// configuredStripe returns a REAL Stripe provider whose IsAvailable() is true —
// a Stripe secret key IS hydrated. Selection never calls Stripe, so the key is
// never sent anywhere.
func configuredStripe() processor.PaymentProcessor {
	return stripe.NewProvider(stripe.Config{SecretKey: "sk_live_DECISIVE_FAKE"})
}

// configuredSquare returns a REAL Square processor with creds (IsAvailable()==true).
func configuredSquare() processor.PaymentProcessor {
	return square.NewProvider(square.Config{
		AccessToken: "sq-access", LocationID: "sq-loc", Environment: "production",
	})
}

// fakeProcessor is a minimal PaymentProcessor for branches where a real provider
// is heavy (crypto/MPC). BaseProcessor supplies Type/SupportedCurrencies/
// IsAvailable/Authorize/Capture; we add the rest.
type fakeProcessor struct{ *processor.BaseProcessor }

func newFakeProcessor(t processor.ProcessorType, available bool, cur []currency.Type) *fakeProcessor {
	bp := processor.NewBaseProcessor(t, cur)
	bp.SetConfigured(available)
	return &fakeProcessor{BaseProcessor: bp}
}

func (f *fakeProcessor) Charge(context.Context, processor.PaymentRequest) (*processor.PaymentResult, error) {
	return &processor.PaymentResult{Success: true, TransactionID: "fake", Status: "succeeded"}, nil
}
func (f *fakeProcessor) Refund(context.Context, processor.RefundRequest) (*processor.RefundResult, error) {
	return &processor.RefundResult{Success: true}, nil
}
func (f *fakeProcessor) GetTransaction(context.Context, string) (*processor.Transaction, error) {
	return &processor.Transaction{ID: "fake"}, nil
}
func (f *fakeProcessor) ValidateWebhook(context.Context, []byte, string) (*processor.WebhookEvent, error) {
	return &processor.WebhookEvent{}, nil
}

// Square stays the default wherever it works: a USD charge selects Square even
// when a Stripe secret key IS hydrated (Stripe.IsAvailable()==true).
func TestSelectProcessor_USD_SquareEvenWhenStripeConfigured(t *testing.T) {
	withDefaultDisabledPolicy(t)
	reg := processor.NewRegistry(processor.DefaultConfig())
	reg.Register(configuredStripe())
	reg.Register(configuredSquare())

	proc, err := reg.SelectProcessor(context.Background(), processor.PaymentRequest{
		Amount: 500, Currency: currency.USD,
	})
	if err != nil {
		t.Fatalf("SelectProcessor(USD): %v", err)
	}
	if proc.Type() != processor.Square {
		t.Fatalf("USD selected %q; want square — Square takes every payment it can", proc.Type())
	}
}

// Every currency Square takes goes to Square, never Stripe.
func TestSelectProcessor_SquareCurrenciesStayOnSquare(t *testing.T) {
	withDefaultDisabledPolicy(t)
	reg := processor.NewRegistry(processor.DefaultConfig())
	reg.Register(configuredStripe())
	reg.Register(configuredSquare())

	for _, cur := range []currency.Type{currency.USD, currency.EUR, currency.GBP, currency.CAD, currency.AUD, currency.JPY} {
		proc, err := reg.SelectProcessor(context.Background(), processor.PaymentRequest{Amount: 200, Currency: cur})
		if err != nil {
			t.Fatalf("SelectProcessor(%s): %v", cur, err)
		}
		if proc.Type() != processor.Square {
			t.Fatalf("currency %s selected %q; want square", cur, proc.Type())
		}
	}
}

// A currency Square does not take falls back to Stripe.
func TestSelectProcessor_StripeTakesWhatSquareCannot(t *testing.T) {
	withDefaultDisabledPolicy(t)
	reg := processor.NewRegistry(processor.DefaultConfig())
	reg.Register(configuredStripe()) // takes CHF
	reg.Register(configuredSquare()) // does NOT take CHF

	proc, err := reg.SelectProcessor(context.Background(), processor.PaymentRequest{Amount: 100, Currency: currency.CHF})
	if err != nil {
		t.Fatalf("SelectProcessor(CHF): %v", err)
	}
	if proc.Type() != processor.Stripe {
		t.Fatalf("CHF selected %q; want stripe — Square does not take CHF", proc.Type())
	}
}

// An org with no usable Square account falls back to Stripe, in a currency
// Square would otherwise take.
func TestSelectProcessor_NoSquareAccount_Stripe(t *testing.T) {
	withDefaultDisabledPolicy(t)
	reg := processor.NewRegistry(processor.DefaultConfig())
	reg.Register(configuredStripe())
	reg.Register(square.NewProvider(square.Config{})) // registered, no credentials: unavailable

	proc, err := reg.SelectProcessor(context.Background(), processor.PaymentRequest{Amount: 100, Currency: currency.USD})
	if err != nil {
		t.Fatalf("SelectProcessor(USD): %v", err)
	}
	if proc.Type() != processor.Stripe {
		t.Fatalf("USD with Square unconfigured selected %q; want stripe", proc.Type())
	}
}

// A caller that names Stripe gets Stripe, even where Square would take the payment.
func TestSelectProcessor_ExplicitStripe(t *testing.T) {
	withDefaultDisabledPolicy(t)
	reg := processor.NewRegistry(processor.DefaultConfig())
	reg.Register(configuredStripe())
	reg.Register(configuredSquare())

	for _, pref := range []interface{}{"stripe", processor.Stripe} {
		proc, err := reg.SelectProcessor(context.Background(), processor.PaymentRequest{
			Amount: 100, Currency: currency.USD,
			Options: map[string]interface{}{"processor": pref},
		})
		if err != nil {
			t.Fatalf("SelectProcessor(explicit %v): %v", pref, err)
		}
		if proc.Type() != processor.Stripe {
			t.Fatalf("explicit %v selected %q; want stripe", pref, proc.Type())
		}
	}
}

// A named processor that cannot take the payment yields NO processor — never a
// substitute. Its token is that processor's, so any other one would fail the
// charge (or, worse, take it against the wrong account).
func TestSelectProcessor_NamedButUnavailable_NoSubstitute(t *testing.T) {
	withDefaultDisabledPolicy(t)
	reg := processor.NewRegistry(processor.DefaultConfig())
	reg.Register(configuredSquare()) // Stripe not registered

	if p, err := reg.SelectProcessor(context.Background(), processor.PaymentRequest{
		Amount: 100, Currency: currency.USD,
		Options: map[string]interface{}{"processor": "stripe"},
	}); err == nil {
		t.Fatalf("explicit stripe with no Stripe account selected %q; want an error", p.Type())
	}

	// Named Square, in a currency Square does not take: no processor, not Stripe.
	reg.Register(configuredStripe())
	if p, err := reg.SelectProcessor(context.Background(), processor.PaymentRequest{
		Amount: 100, Currency: currency.CHF,
		Options: map[string]interface{}{"processor": processor.Square},
	}); err == nil {
		t.Fatalf("explicit square for CHF selected %q; want an error", p.Type())
	}
}

// The deny switch binds every branch: with Stripe denied, a Stripe-only currency
// has no processor and a caller naming Stripe is refused.
func TestSelectProcessor_DeniedStripe(t *testing.T) {
	t.Setenv(processor.ChargeProcessors, "square")
	reg := processor.NewRegistry(processor.DefaultConfig())
	reg.Register(configuredStripe())
	reg.Register(configuredSquare())

	if p, err := reg.SelectProcessor(context.Background(), processor.PaymentRequest{Amount: 100, Currency: currency.CHF}); err == nil {
		t.Fatalf("CHF with Stripe denied selected %q; want none", p.Type())
	}
	if p, err := reg.SelectProcessor(context.Background(), processor.PaymentRequest{
		Amount: 100, Currency: currency.USD, Options: map[string]interface{}{"processor": "stripe"},
	}); err == nil {
		t.Fatalf("explicit stripe with Stripe denied selected %q; want none", p.Type())
	}
	if _, err := reg.Get(processor.Stripe); err == nil {
		t.Fatal("Get(Stripe) must error when Stripe is denied")
	}
	if p, err := reg.Fiat(context.Background(), processor.Need{Currency: currency.USD, Country: "DE", Method: "card"}); err == nil {
		t.Fatalf("a DE card with Stripe denied selected %q; want none", p.Type())
	}
}

// Crypto routing is unchanged: a crypto currency selects the crypto processor (MPC).
func TestSelectProcessor_Crypto_SelectsMPC(t *testing.T) {
	withDefaultDisabledPolicy(t)
	reg := processor.NewRegistry(processor.DefaultConfig())
	reg.Register(configuredStripe())
	reg.Register(configuredSquare())
	reg.Register(newFakeProcessor(processor.MPC, true, []currency.Type{currency.BTC}))

	proc, err := reg.SelectProcessor(context.Background(), processor.PaymentRequest{Amount: 1000, Currency: currency.BTC})
	if err != nil {
		t.Fatalf("SelectProcessor(BTC): %v", err)
	}
	if proc.Type() != processor.MPC {
		t.Fatalf("crypto selected %q; want mpc", proc.Type())
	}
}

// Fiat with a buyer's country and method: Square in Square's countries and
// methods, Stripe for a card outside them, nobody for what neither takes.
func TestFiat_CountryAndMethod(t *testing.T) {
	withDefaultDisabledPolicy(t)
	reg := processor.NewRegistry(processor.DefaultConfig())
	reg.Register(configuredStripe())
	reg.Register(configuredSquare())
	// A processor with no Reach is never chosen for a stated country or method:
	// nothing says where it takes payments.
	reg.Register(newFakeProcessor(processor.Adyen, true, []currency.Type{currency.USD}))

	cases := []struct {
		country, method string
		want            processor.ProcessorType // "" = no processor
	}{
		{"US", "card", processor.Square},
		{"GB", "apple_pay", processor.Square},
		{"UK", "card", processor.Square},
		{"US", "cash_app", processor.Square},
		{"", "card", processor.Square},
		{"DE", "card", processor.Stripe},
		{"BR", "google_pay", processor.Stripe},
		{"DE", "cash_app", ""}, // Cash App Pay is Square's, and Square does not take DE
		{"US", "ach", ""},      // Square declines ACH; Stripe's form here takes no ACH
	}
	for _, c := range cases {
		p, err := reg.Fiat(context.Background(), processor.Need{Currency: currency.USD, Country: c.country, Method: c.method})
		switch {
		case c.want == "" && err == nil:
			t.Errorf("%s/%s: selected %q; want none", c.country, c.method, p.Type())
		case c.want != "" && err != nil:
			t.Errorf("%s/%s: %v; want %s", c.country, c.method, err, c.want)
		case c.want != "" && p.Type() != c.want:
			t.Errorf("%s/%s: selected %q; want %s", c.country, c.method, p.Type(), c.want)
		}
	}
}

// Cards is Fiat's answer for a card, per currency: what a checkout mounts.
func TestCards_PerCountry(t *testing.T) {
	withDefaultDisabledPolicy(t)
	reg := processor.NewRegistry(processor.DefaultConfig())
	reg.Register(configuredStripe())
	reg.Register(configuredSquare())

	us := reg.Cards(context.Background(), "US")
	if us[currency.USD] != processor.Square || us[currency.EUR] != processor.Square {
		t.Fatalf("US cards = %v; want usd and eur on square", us)
	}
	if us[currency.CHF] != processor.Stripe {
		t.Fatalf("US cards = %v; want chf on stripe (Square does not take CHF)", us)
	}
	de := reg.Cards(context.Background(), "DE")
	for _, cur := range []currency.Type{currency.USD, currency.EUR, currency.CHF} {
		if de[cur] != processor.Stripe {
			t.Fatalf("DE cards = %v; want %s on stripe (Square does not take DE)", de, cur)
		}
	}

	squareOnly := processor.NewRegistry(processor.DefaultConfig())
	squareOnly.Register(configuredSquare())
	if de := squareOnly.Cards(context.Background(), "DE"); len(de) != 0 {
		t.Fatalf("DE cards with Square only = %v; want none", de)
	}
	if us := squareOnly.Cards(context.Background(), "US"); us[currency.USD] != processor.Square || us[currency.CHF] != "" {
		t.Fatalf("US cards with Square only = %v; want usd on square and no chf", us)
	}
}

// The deployment default is Stripe alone: Square, PayPal and the rest take no new
// charge, however configured, and Stripe takes every one.
func TestChargePolicy_DefaultIsStripeOnly(t *testing.T) {
	t.Setenv(processor.ChargeProcessors, "")
	if processor.DisabledByPolicy(processor.Stripe) {
		t.Fatal("the default policy denies Stripe")
	}
	for _, p := range []processor.ProcessorType{processor.Square, processor.PayPal, processor.MPC, processor.Wire, processor.Kraken} {
		if !processor.DisabledByPolicy(p) {
			t.Fatalf("the default policy lets %s take a new charge", p)
		}
	}
	reg := processor.NewRegistry(processor.DefaultConfig())
	reg.Register(configuredStripe())
	reg.Register(configuredSquare())
	for _, cur := range []currency.Type{currency.USD, currency.EUR, currency.CHF} {
		p, err := reg.SelectProcessor(context.Background(), processor.PaymentRequest{Amount: 100, Currency: cur})
		if err != nil || p.Type() != processor.Stripe {
			t.Fatalf("%s: %v %v; want stripe", cur, p, err)
		}
	}
	if _, err := reg.Get(processor.Square); err == nil {
		t.Fatal("Get(Square) succeeded under the default policy")
	}
	// A customer already on Square is still reachable, for renewals.
	if p, err := reg.Existing(context.Background(), processor.Square); err != nil || p.Type() != processor.Square {
		t.Fatalf("Existing(Square) = %v, %v", p, err)
	}
	// A processor comes back by config.
	t.Setenv(processor.ChargeProcessors, "stripe, square")
	if processor.DisabledByPolicy(processor.Square) || !processor.DisabledByPolicy(processor.PayPal) {
		t.Fatal("COMMERCE_CHARGE_PROCESSORS=stripe,square must allow exactly those")
	}
	t.Setenv(processor.ChargeProcessors, "*")
	if processor.DisabledByPolicy(processor.PayPal) || processor.DisabledByPolicy(processor.MPC) {
		t.Fatal("COMMERCE_CHARGE_PROCESSORS=* must allow every processor")
	}
}
