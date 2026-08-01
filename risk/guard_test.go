// Copyright © 2026 Hanzo AI. MIT License.

package risk

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hanzoai/commerce/models/control"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/util/test/ae"
)

// gateway is a processor a test drives: it records whether it was reached.
type gateway struct {
	charged    int
	authorized int
	captured   int
	refunded   int
}

func (g *gateway) Type() processor.ProcessorType { return "square" }

func (g *gateway) Charge(context.Context, processor.PaymentRequest) (*processor.PaymentResult, error) {
	g.charged++
	return &processor.PaymentResult{Success: true, TransactionID: "tx_1"}, nil
}

func (g *gateway) Authorize(context.Context, processor.PaymentRequest) (*processor.PaymentResult, error) {
	g.authorized++
	return &processor.PaymentResult{Success: true, TransactionID: "tx_1"}, nil
}

func (g *gateway) Capture(context.Context, string, currency.Cents) (*processor.PaymentResult, error) {
	g.captured++
	return &processor.PaymentResult{Success: true}, nil
}

func (g *gateway) Refund(context.Context, processor.RefundRequest) (*processor.RefundResult, error) {
	g.refunded++
	return &processor.RefundResult{Success: true}, nil
}

func (g *gateway) GetTransaction(context.Context, string) (*processor.Transaction, error) {
	return &processor.Transaction{}, nil
}

func (g *gateway) ValidateWebhook(context.Context, []byte, string) (*processor.WebhookEvent, error) {
	return &processor.WebhookEvent{}, nil
}

func (g *gateway) SupportedCurrencies() []currency.Type { return []currency.Type{currency.USD} }
func (g *gateway) IsAvailable(context.Context) bool     { return true }

// TestGuard_RefusalNeverReachesTheGateway — the whole point of screening at
// authorization time is that the refused charge is never presented.
func TestGuard_RefusalNeverReachesTheGateway(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	s := tenant("guardblock", ctx, &oracle{answer: &Decision{ID: "d1", Action: Block}})
	inner := &gateway{}
	g := NewGuard(inner, s)

	res, err := g.Authorize(context.Background(), processor.PaymentRequest{
		CustomerID: "c1", Amount: 2500, Currency: currency.USD,
	})
	if !errors.Is(err, ErrRefused) {
		t.Fatalf("err=%v want ErrRefused", err)
	}
	if res == nil || res.Success {
		t.Fatalf("result=%+v want an unsuccessful refusal carrying the reason", res)
	}
	if inner.authorized != 0 || inner.charged != 0 {
		t.Fatalf("the gateway was reached: authorized=%d charged=%d", inner.authorized, inner.charged)
	}
	if res.Metadata["screen"] == "" {
		t.Fatal("the refusal does not name the screen that produced it")
	}
}

// TestGuard_AllowReachesTheGatewayUnchanged.
func TestGuard_AllowReachesTheGatewayUnchanged(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	s := tenant("guardallow", ctx, &oracle{answer: &Decision{ID: "d1", Action: Allow}})
	inner := &gateway{}
	g := NewGuard(inner, s)

	res, err := g.Charge(context.Background(), processor.PaymentRequest{
		CustomerID: "c1", Amount: 2500, Currency: currency.USD,
	})
	if err != nil || res == nil || !res.Success {
		t.Fatalf("charge: res=%+v err=%v", res, err)
	}
	if inner.charged != 1 {
		t.Fatalf("the gateway was charged %d times, want 1", inner.charged)
	}
}

// TestGuard_APayoutHoldDoesNotStopACharge — a hold bears on money leaving, and
// a charge is money arriving.
func TestGuard_APayoutHoldDoesNotStopACharge(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	s := tenant("guardhold", ctx, &oracle{answer: &Decision{Action: Allow}})
	if _, err := Place(s, customer("c1"), control.Hold, 0, time.Time{}, "payouts held"); err != nil {
		t.Fatalf("place: %v", err)
	}
	inner := &gateway{}
	if _, err := NewGuard(inner, s).Authorize(context.Background(), processor.PaymentRequest{
		CustomerID: "c1", Amount: 100, Currency: currency.USD,
	}); err != nil {
		t.Fatalf("authorize: %v", err)
	}
	if inner.authorized != 1 {
		t.Fatal("a payout hold stopped an inbound charge")
	}
}

// TestGuard_CaptureAndRefundPassStraightThrough — stopping a capture after an
// authorization already succeeded strands the customer's money without
// protecting anyone.
func TestGuard_CaptureAndRefundPassStraightThrough(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	s := tenant("guardpass", ctx, &oracle{answer: &Decision{Action: Block}})
	inner := &gateway{}
	g := NewGuard(inner, s)

	if _, err := g.Capture(context.Background(), "tx_1", 100); err != nil {
		t.Fatalf("capture: %v", err)
	}
	if _, err := g.Refund(context.Background(), processor.RefundRequest{TransactionID: "tx_1"}); err != nil {
		t.Fatalf("refund: %v", err)
	}
	if inner.captured != 1 || inner.refunded != 1 {
		t.Fatalf("capture=%d refund=%d, want both to pass through", inner.captured, inner.refunded)
	}
}

// TestSignals_OnlyTheAllowlistTravels — a merchant who puts a card number in a
// metadata field must not thereby send it to a scoring plane, and no list of
// forbidden key names can be relied on to catch that.
func TestSignals_OnlyTheAllowlistTravels(t *testing.T) {
	got := Signals(map[string]any{
		"IP":            "203.0.113.7",
		"Bin":           "424242",
		"card_number":   "4242424242424242",
		"pan":           "4242424242424242",
		"cvc":           "123",
		"token":         "tok_live_abc",
		"secret":        "sk_live_abc",
		"anything_else": "leak me",
		"count":         42, // not a string
		"ip_extra":      "no",
	})
	want := map[string]string{"ip": "203.0.113.7", "bin": "424242"}
	if len(got) != len(want) {
		t.Fatalf("signals=%v want exactly %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("signals[%q]=%q want %q", k, got[k], v)
		}
	}
	for _, forbidden := range []string{"card_number", "pan", "cvc", "token", "secret", "anything_else", "count", "ip_extra"} {
		if _, ok := got[forbidden]; ok {
			t.Fatalf("%q reached the scoring plane", forbidden)
		}
	}
}

// TestGuard_SignalsAreTheOnlyThingSent — proved end to end through the guard,
// so the allowlist is not merely a function that happens to be correct.
func TestGuard_SignalsAreTheOnlyThingSent(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	p := &oracle{answer: &Decision{Action: Allow}}
	g := NewGuard(&gateway{}, tenant("guardsignals", ctx, p))

	if _, err := g.Authorize(context.Background(), processor.PaymentRequest{
		CustomerID: "c1", Amount: 100, Currency: currency.USD, Token: "tok_live_abc",
		Metadata: map[string]any{"ip": "198.51.100.9", "card_number": "4242424242424242"},
	}); err != nil {
		t.Fatalf("authorize: %v", err)
	}
	if len(p.asks) != 1 {
		t.Fatalf("the plane was asked %d times", len(p.asks))
	}
	sent := p.asks[0].Signals
	if sent["ip"] != "198.51.100.9" {
		t.Fatalf("ip did not travel: %v", sent)
	}
	for k, v := range sent {
		if v == "4242424242424242" || v == "tok_live_abc" {
			t.Fatalf("a payment credential travelled as signal %q", k)
		}
	}
}
