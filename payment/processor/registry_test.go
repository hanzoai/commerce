package processor_test

import (
	"context"
	"os"
	"testing"

	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
	square "github.com/hanzoai/commerce/thirdparty/square"
)

// These tests used to exist to prove one negative: that Stripe could never win a
// charge even with a live secret key hydrated. That proof is obsolete because the
// thing it guarded against is DELETED — no Stripe provider, no Stripe processor
// type, and no flag whose wrong value could bring either back. What remains worth
// pinning is what is now true: Square is the fiat rail, crypto routes to MPC, and
// the deny mechanism still works for whatever a deployment chooses to deny.

// withNoDenyPolicy guarantees COMMERCE_DISABLED_PROCESSORS is UNSET so
// DefaultConfig() applies the default (deny nothing). t.Setenv cannot unset, so
// save/unset/restore by hand.
func withNoDenyPolicy(t *testing.T) {
	t.Helper()
	if v, ok := os.LookupEnv("COMMERCE_DISABLED_PROCESSORS"); ok {
		if err := os.Unsetenv("COMMERCE_DISABLED_PROCESSORS"); err != nil {
			t.Fatalf("unset COMMERCE_DISABLED_PROCESSORS: %v", err)
		}
		t.Cleanup(func() { _ = os.Setenv("COMMERCE_DISABLED_PROCESSORS", v) })
	}
}

// configuredSquare returns a REAL Square processor with creds (IsAvailable()==true).
func configuredSquare() processor.PaymentProcessor {
	return square.NewProcessor(square.Config{
		AccessToken: "sq-access", LocationID: "sq-loc", Environment: "production",
	})
}

// fakeProcessor is a minimal PaymentProcessor for branches where a real provider
// is heavy (crypto/MPC).
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

// Square is the fiat rail for every currency it supports.
func TestSelectProcessor_FiatSelectsSquare(t *testing.T) {
	withNoDenyPolicy(t)
	reg := processor.NewRegistry(processor.DefaultConfig())
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

// A currency the fiat rail does not support yields NO processor rather than a
// silent fallback to something that cannot settle it.
func TestSelectProcessor_UnsupportedCurrencyYieldsNoProcessor(t *testing.T) {
	withNoDenyPolicy(t)
	reg := processor.NewRegistry(processor.DefaultConfig())
	reg.Register(configuredSquare()) // does NOT support CHF

	if _, err := reg.SelectProcessor(context.Background(), processor.PaymentRequest{
		Amount: 100, Currency: currency.CHF,
	}); err == nil {
		t.Fatal("a currency no registered processor supports must yield NO processor; got one")
	}
}

// Crypto routing is unchanged: a crypto currency selects the crypto processor (MPC).
func TestSelectProcessor_Crypto_SelectsMPC(t *testing.T) {
	withNoDenyPolicy(t)
	reg := processor.NewRegistry(processor.DefaultConfig())
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

// Nothing is denied by default — the deny set existed for exactly one entry, and
// that entry is gone from the codebase.
func TestDisabledByPolicy_DefaultDeniesNothing(t *testing.T) {
	withNoDenyPolicy(t)
	for _, p := range []processor.ProcessorType{processor.Square, processor.MPC, processor.PayPal, processor.Adyen} {
		if processor.DisabledByPolicy(p) {
			t.Fatalf("default policy must deny nothing; %q was denied", p)
		}
	}
}

// An empty or whitespace value (the k8s placeholder shape) reads as "unset", and
// the explicit sentinel "none" means the same thing — deny nothing.
func TestDisabledByPolicy_EmptyAndNoneDenyNothing(t *testing.T) {
	for _, v := range []string{"", "   ", "none", "NONE"} {
		t.Setenv("COMMERCE_DISABLED_PROCESSORS", v)
		if processor.DisabledByPolicy(processor.Square) {
			t.Fatalf("COMMERCE_DISABLED_PROCESSORS=%q must deny nothing", v)
		}
	}
}

// The mechanism still works: a deployment can deny a rail it does not want, and
// a denied processor is never selected from any branch.
func TestDisabledByPolicy_DenyListIsHonored(t *testing.T) {
	t.Setenv("COMMERCE_DISABLED_PROCESSORS", "square")
	if !processor.DisabledByPolicy(processor.Square) {
		t.Fatal("COMMERCE_DISABLED_PROCESSORS=square must deny Square")
	}
	if processor.DisabledByPolicy(processor.MPC) {
		t.Fatal("the deny list names Square only; MPC must stay enabled")
	}

	reg := processor.NewRegistry(processor.DefaultConfig())
	reg.Register(configuredSquare())
	if _, err := reg.SelectProcessor(context.Background(), processor.PaymentRequest{
		Amount: 100, Currency: currency.USD,
	}); err == nil {
		t.Fatal("a denied processor must not be selectable, even as the only registered one")
	}
}
