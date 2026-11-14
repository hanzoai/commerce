package processor_test

import (
	"context"
	"testing"

	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/payment/providers/square"
	"github.com/hanzoai/commerce/payment/providers/square/squaretest"
	"github.com/hanzoai/commerce/payment/providers/stripe"
)

// withDefaultDisabledPolicy lets every processor take a new charge for the test
// (COMMERCE_CHARGE_PROCESSORS=*), so the tests of the selection rule see it
// across processors. The deployment default is Stripe alone: see
// TestChargePolicy_DefaultIsStripeOnly.
func withDefaultDisabledPolicy(t *testing.T) {
	t.Helper()
	t.Cleanup(processor.SetChargePolicy(func() map[processor.ProcessorType]bool { return processor.ChargeNames("*") }))
}

// configuredStripe returns a REAL Stripe provider whose IsAvailable() is true —
// a Stripe secret key IS hydrated. Selection never calls Stripe, so the key is
// never sent anywhere.
func configuredStripe() processor.PaymentProcessor {
	return stripe.NewProvider(stripe.Config{SecretKey: "sk_live_DECISIVE_FAKE"})
}

// configuredSquare returns a REAL Square processor with creds (IsAvailable()==true).
func configuredSquare(t *testing.T) processor.PaymentProcessor {
	t.Helper()
	return configuredSquareAt(t, "US", "usd")
}

// configuredSquareAt is Square with a merchant location in country, in currency.
func configuredSquareAt(t *testing.T, country, cur string) processor.PaymentProcessor {
	t.Helper()
	return square.NewProvider(square.Config{
		AccessToken: "sq-access", LocationID: "sq-loc", Environment: "production",
		Endpoint: squaretest.Cards(t, country, cur),
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
	reg.Register(configuredSquare(t))

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

// Square takes the currency of the merchant's location, and only that: a US/USD
// location takes dollars, and every other currency is Stripe's.
func TestSelectProcessor_SquareTakesOnlyItsLocationsCurrency(t *testing.T) {
	withDefaultDisabledPolicy(t)
	reg := processor.NewRegistry(processor.DefaultConfig())
	reg.Register(configuredStripe())
	reg.Register(configuredSquare(t))

	for cur, want := range map[currency.Type]processor.ProcessorType{
		currency.USD: processor.Square,
		currency.EUR: processor.Stripe, currency.GBP: processor.Stripe, currency.CAD: processor.Stripe,
		currency.AUD: processor.Stripe, currency.JPY: processor.Stripe,
	} {
		proc, err := reg.SelectProcessor(context.Background(), processor.PaymentRequest{Amount: 200, Currency: cur})
		if err != nil {
			t.Fatalf("SelectProcessor(%s): %v", cur, err)
		}
		if proc.Type() != want {
			t.Fatalf("currency %s selected %q; want %s", cur, proc.Type(), want)
		}
	}
}

// A location that holds no card processing is not Square's to offer: nothing is.
func TestSelectProcessor_NoCardLocationNoSquare(t *testing.T) {
	withDefaultDisabledPolicy(t)
	reg := processor.NewRegistry(processor.DefaultConfig())
	reg.Register(configuredStripe())
	reg.Register(square.NewProvider(square.Config{
		AccessToken: "sq-access", LocationID: "sq-loc", Environment: "production",
		Endpoint: squaretest.Location(t, "US", "usd", "AUTOMATIC_TRANSFERS"),
	}))
	proc, err := reg.SelectProcessor(context.Background(), processor.PaymentRequest{Amount: 200, Currency: currency.USD})
	if err != nil || proc.Type() != processor.Stripe {
		t.Fatalf("a location without card processing offered Square: %v %v", proc, err)
	}
}

// A currency Square does not take falls back to Stripe.
func TestSelectProcessor_StripeTakesWhatSquareCannot(t *testing.T) {
	withDefaultDisabledPolicy(t)
	reg := processor.NewRegistry(processor.DefaultConfig())
	reg.Register(configuredStripe())  // takes CHF
	reg.Register(configuredSquare(t)) // does NOT take CHF

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
	reg.Register(configuredSquare(t))

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
	reg.Register(configuredSquare(t)) // Stripe not registered

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
	t.Cleanup(processor.SetChargePolicy(func() map[processor.ProcessorType]bool { return processor.ChargeNames("square") }))
	reg := processor.NewRegistry(processor.DefaultConfig())
	reg.Register(configuredStripe())
	reg.Register(configuredSquare(t))

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
	// Stripe denied, the merchant's USD location still takes a dollar card from a DE buyer.
	if p, err := reg.Fiat(context.Background(), processor.Need{Currency: currency.USD, Country: "DE", Method: "card"}); err != nil || p.Type() != processor.Square {
		t.Fatalf("a DE dollar card with Stripe denied selected %v (%v); want Square", p, err)
	}
	if p, err := reg.Fiat(context.Background(), processor.Need{Currency: currency.EUR, Country: "DE", Method: "card"}); err == nil {
		t.Fatalf("a EUR card with Stripe denied and a USD location selected %q; want none", p.Type())
	}
}

// Crypto routing is unchanged: a crypto currency selects the crypto processor (MPC).
func TestSelectProcessor_Crypto_SelectsMPC(t *testing.T) {
	withDefaultDisabledPolicy(t)
	reg := processor.NewRegistry(processor.DefaultConfig())
	reg.Register(configuredStripe())
	reg.Register(configuredSquare(t))
	reg.Register(newFakeProcessor(processor.MPC, true, []currency.Type{currency.BTC}))

	proc, err := reg.SelectProcessor(context.Background(), processor.PaymentRequest{Amount: 1000, Currency: currency.BTC})
	if err != nil {
		t.Fatalf("SelectProcessor(BTC): %v", err)
	}
	if proc.Type() != processor.MPC {
		t.Fatalf("crypto selected %q; want mpc", proc.Type())
	}
}

// Fiat by method: the buyer's country decides nothing (a German buyer paying dollars
// to a US location is Square's to take); the method does. Square takes a card, the
// wallets and Cash App Pay; neither takes ACH here.
func TestFiat_MethodNotCountry(t *testing.T) {
	withDefaultDisabledPolicy(t)
	reg := processor.NewRegistry(processor.DefaultConfig())
	reg.Register(configuredStripe())
	reg.Register(configuredSquare(t))
	// A processor with no Reach is never chosen for a stated country or method:
	// nothing says where it takes payments.
	reg.Register(newFakeProcessor(processor.Adyen, true, []currency.Type{currency.USD}))

	cases := []struct {
		country, method string
		want            processor.ProcessorType // "" = no processor
	}{
		{"US", "card", processor.Square},
		{"GB", "apple_pay", processor.Square},
		{"DE", "card", processor.Square},
		{"BR", "google_pay", processor.Square},
		{"US", "cash_app", processor.Square},
		{"", "card", processor.Square},
		{"US", "ach", ""}, // Square declines ACH; Stripe's form here takes no ACH
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

// Cards is Fiat's answer for a card, per currency: what a checkout mounts. Only the
// currency of the merchant's Square location is Square's; the buyer's country does
// not change the answer.
func TestCards_OnlyEligibleCurrenciesAreSquares(t *testing.T) {
	withDefaultDisabledPolicy(t)
	reg := processor.NewRegistry(processor.DefaultConfig())
	reg.Register(configuredStripe())
	reg.Register(configuredSquare(t))

	for _, country := range []string{"US", "DE", ""} {
		cards := reg.Cards(context.Background(), country)
		if cards[currency.USD] != processor.Square {
			t.Fatalf("%q cards = %v; want usd on square", country, cards)
		}
		for _, cur := range []currency.Type{currency.EUR, currency.GBP, currency.CAD, currency.JPY, currency.CHF} {
			if cards[cur] != processor.Stripe {
				t.Fatalf("%q cards = %v; want %s on stripe (the location is USD)", country, cards, cur)
			}
		}
	}

	squareOnly := processor.NewRegistry(processor.DefaultConfig())
	squareOnly.Register(configuredSquare(t))
	if cards := squareOnly.Cards(context.Background(), "DE"); cards[currency.USD] != processor.Square || len(cards) != 1 {
		t.Fatalf("cards with Square only = %v; want usd on square and nothing else", cards)
	}
}

// The charge policy is the routing policy's processor list, read when a charge is
// attempted: a registry built once refuses a processor the moment the policy switches it
// off, and takes it again the moment it is back on, with no restart and no new registry.
func TestChargePolicy_IsReadWhenTheChargeIsAttempted(t *testing.T) {
	allowed := "stripe"
	t.Cleanup(processor.SetChargePolicy(func() map[processor.ProcessorType]bool { return processor.ChargeNames(allowed) }))
	reg := processor.NewRegistry(processor.DefaultConfig())
	reg.Register(configuredStripe())
	reg.Register(configuredSquare(t))
	ctx := context.Background()

	for _, cur := range []currency.Type{currency.USD, currency.EUR, currency.CHF} {
		p, err := reg.SelectProcessor(ctx, processor.PaymentRequest{Amount: 100, Currency: cur})
		if err != nil || p.Type() != processor.Stripe {
			t.Fatalf("%s: %v %v; want stripe", cur, p, err)
		}
	}
	for _, p := range []processor.ProcessorType{processor.Square, processor.PayPal, processor.MPC, processor.Wire, processor.Kraken} {
		if !processor.DisabledByPolicy(p) {
			t.Fatalf("a policy naming only stripe lets %s take a new charge", p)
		}
	}
	if _, err := reg.Get(processor.Square); err == nil {
		t.Fatal("Get(Square) succeeded while the policy has it off")
	}
	// A customer already on Square is still reachable, for renewals.
	if p, err := reg.Existing(ctx, processor.Square); err != nil || p.Type() != processor.Square {
		t.Fatalf("Existing(Square) = %v, %v", p, err)
	}

	// The admin switches Square on: the next attempt takes it, on the same registry.
	allowed = "stripe, square"
	if _, err := reg.Get(processor.Square); err != nil {
		t.Fatalf("Square switched on and still refused: %v", err)
	}
	if processor.DisabledByPolicy(processor.Square) || !processor.DisabledByPolicy(processor.PayPal) {
		t.Fatal("a policy of stripe and square must allow exactly those")
	}
	// And off again: refused at once.
	allowed = "stripe"
	if _, err := reg.Get(processor.Square); err == nil {
		t.Fatal("Square switched off and still taken")
	}
	allowed = "*"
	if processor.DisabledByPolicy(processor.PayPal) || processor.DisabledByPolicy(processor.MPC) {
		t.Fatal("a policy of * must allow every processor")
	}
}
