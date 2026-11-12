package stripe_test

import (
	"context"
	"strings"
	"testing"

	"github.com/hanzoai/money"

	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/payment/providers/stripe"
	"github.com/hanzoai/commerce/payment/providers/stripe/stripetest"
)

// These drive the real Stripe provider over HTTP against Stripe's contract
// (stripetest): every parameter it sends must be one Stripe's spec declares,
// and every answer is Stripe's own fixture shape.

func contractProvider(t *testing.T) (*stripe.Provider, *stripetest.Server) {
	t.Helper()
	srv := stripetest.New(t, "sk_test_contract")
	srv.Route(t)
	return stripe.NewProvider(stripe.Config{SecretKey: "sk_test_contract", PublishableKey: "pk_test_contract"}), srv
}

// A card that pays: the charge settles, Stripe de-dups it on the caller's key,
// and a retry under the same key replays instead of charging twice.
func TestContract_ChargeSettlesOnceUnderItsKey(t *testing.T) {
	p, srv := contractProvider(t)
	req := processor.PaymentRequest{
		Amount: 2500, Currency: currency.CHF, Token: "pm_card_visa",
		IdempotencyKey: "topup:hanzo/alice:k1", Description: "Top-up",
	}
	res, err := p.Charge(context.Background(), req)
	if err != nil || !res.Success || res.Status != "succeeded" {
		t.Fatalf("Charge = %+v, %v; want a settled charge", res, err)
	}
	if in := srv.Intent(res.TransactionID); in == nil || in.Amount != 2500 || in.Currency != "chf" {
		t.Fatalf("Stripe holds %+v; want 2500 chf", in)
	}
	again, err := p.Charge(context.Background(), req)
	if err != nil || again.TransactionID != res.TransactionID {
		t.Fatalf("retry = %+v, %v; want the replay of %s", again, err, res.TransactionID)
	}
	if srv.Intents() != 1 {
		t.Fatalf("Stripe holds %d PaymentIntents after a retry; want 1", srv.Intents())
	}
	if len(srv.Keys) != 2 || srv.Keys[0] != "topup:hanzo/alice:k1" {
		t.Fatalf("Idempotency-Keys sent = %q", srv.Keys)
	}
}

// A card that needs 3-D Secure moves no money and must not read as paid.
func TestContract_AuthenticationRequiredIsNotPaid(t *testing.T) {
	p, srv := contractProvider(t)
	res, err := p.Charge(context.Background(), processor.PaymentRequest{
		Amount: 2500, Currency: currency.EUR, Token: "pm_card_authenticationRequired", IdempotencyKey: "k-3ds",
	})
	if err == nil || res == nil || res.Success {
		t.Fatalf("Charge = %+v, %v; want a failed charge", res, err)
	}
	if in := srv.Intent(res.TransactionID); in == nil || in.Status != "requires_action" {
		t.Fatalf("Stripe holds %+v; want requires_action", in)
	}
}

// A declined card is a failed charge carrying Stripe's reason.
func TestContract_DeclineCarriesStripesReason(t *testing.T) {
	p, _ := contractProvider(t)
	res, err := p.Charge(context.Background(), processor.PaymentRequest{
		Amount: 2500, Currency: currency.USD, Token: "pm_card_chargeDeclined", IdempotencyKey: "k-decline",
	})
	if err == nil || res.Success || !strings.Contains(err.Error(), "declined") {
		t.Fatalf("Charge = %+v, %v; want Stripe's decline", res, err)
	}
}

// Authorize holds, Capture takes, Refund returns — each under the spec's
// parameters, the refund under its own key.
func TestContract_AuthorizeCaptureRefund(t *testing.T) {
	p, srv := contractProvider(t)
	auth, err := p.Authorize(context.Background(), processor.PaymentRequest{
		Amount: 1500, Currency: currency.USD, Token: "pm_card_visa", IdempotencyKey: "sale-1",
	})
	if err != nil || !auth.Success {
		t.Fatalf("Authorize = %+v, %v", auth, err)
	}
	if in := srv.Intent(auth.TransactionID); in.Status != "requires_capture" {
		t.Fatalf("after Authorize Stripe holds %s; want requires_capture", in.Status)
	}
	cap, err := p.Capture(context.Background(), auth.TransactionID, money.FromMinor(1500, currency.USD.Money()))
	if err != nil || !cap.Success {
		t.Fatalf("Capture = %+v, %v", cap, err)
	}
	ref, err := p.Refund(context.Background(), processor.RefundRequest{
		TransactionID: auth.TransactionID, Amount: money.FromMinor(500, currency.USD.Money()), IdempotencyKey: "refund-1",
	})
	if err != nil || !ref.Success {
		t.Fatalf("Refund = %+v, %v", ref, err)
	}
	if srv.Keys[len(srv.Keys)-1] != "refund-1" {
		t.Fatalf("refund sent key %q; want refund-1", srv.Keys[len(srv.Keys)-1])
	}
}

// A key longer than Stripe's limit is fitted, deterministically, so the charge
// is taken rather than refused and a retry still replays.
func TestContract_LongKeyIsFitted(t *testing.T) {
	p, srv := contractProvider(t)
	long := strings.Repeat("subscribe-wallet:hanzo/guest-0123456789ab:", 8)
	req := processor.PaymentRequest{Amount: 900, Currency: currency.USD, Token: "pm_card_visa", IdempotencyKey: long}
	a, err := p.Charge(context.Background(), req)
	if err != nil || !a.Success {
		t.Fatalf("Charge under a %d-character key: %+v, %v", len(long), a, err)
	}
	b, err := p.Charge(context.Background(), req)
	if err != nil || b.TransactionID != a.TransactionID {
		t.Fatalf("retry under the same long key opened %s; want %s", b.TransactionID, a.TransactionID)
	}
	if k := srv.Keys[0]; len(k) > 255 || k == long {
		t.Fatalf("sent Idempotency-Key of %d characters", len(k))
	}
}

// Stripe's card form takes cards and card wallets from anywhere, and nothing
// it would have to redirect for.
func TestReaches(t *testing.T) {
	p := stripe.NewProvider(stripe.Config{SecretKey: "sk_test_contract"})
	for _, m := range []string{"", "card", "apple_pay", "google_pay"} {
		if !p.Reaches("DE", m) {
			t.Errorf("Reaches(DE, %q) = false", m)
		}
	}
	for _, m := range []string{"ach", "ideal", "cash_app"} {
		if p.Reaches("US", m) {
			t.Errorf("Reaches(US, %q) = true", m)
		}
	}
}
