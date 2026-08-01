// Copyright © 2026 Hanzo AI. MIT License.

package risk

import (
	"context"
	"strings"

	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
)

// Guard screens an authorization before it reaches the gateway.
//
// It IS a processor.PaymentProcessor, so it composes with whatever the org
// already uses — one gateway, the multi-processor router, a crypto processor —
// and the screening is therefore processor-agnostic by construction rather than
// by a per-processor integration. A merchant who processes payments elsewhere
// gets the same judgement through POST /v1/billing/risk/screen, which is the
// same [Screener] with no processor behind it.
//
// It intercepts exactly the two calls that AUTHORIZE money: Charge and
// Authorize. Capture, Refund and the reads pass straight through, because
// stopping a capture after an authorization already succeeded strands the
// customer's money without protecting anyone.
type Guard struct {
	Inner    processor.PaymentProcessor
	Screener *Screener
}

// NewGuard wraps inner so every authorization through it is screened.
func NewGuard(inner processor.PaymentProcessor, s *Screener) *Guard {
	return &Guard{Inner: inner, Screener: s}
}

var _ processor.PaymentProcessor = (*Guard)(nil)

func (g *Guard) Type() processor.ProcessorType { return g.Inner.Type() }

func (g *Guard) Charge(ctx context.Context, req processor.PaymentRequest) (*processor.PaymentResult, error) {
	if res, err := g.screen(ctx, req); res != nil || err != nil {
		return res, err
	}
	return g.Inner.Charge(ctx, req)
}

func (g *Guard) Authorize(ctx context.Context, req processor.PaymentRequest) (*processor.PaymentResult, error) {
	if res, err := g.screen(ctx, req); res != nil || err != nil {
		return res, err
	}
	return g.Inner.Authorize(ctx, req)
}

func (g *Guard) Capture(ctx context.Context, txID string, amount currency.Cents) (*processor.PaymentResult, error) {
	return g.Inner.Capture(ctx, txID, amount)
}

func (g *Guard) Refund(ctx context.Context, req processor.RefundRequest) (*processor.RefundResult, error) {
	return g.Inner.Refund(ctx, req)
}

func (g *Guard) GetTransaction(ctx context.Context, txID string) (*processor.Transaction, error) {
	return g.Inner.GetTransaction(ctx, txID)
}

func (g *Guard) ValidateWebhook(ctx context.Context, payload []byte, sig string) (*processor.WebhookEvent, error) {
	return g.Inner.ValidateWebhook(ctx, payload, sig)
}

func (g *Guard) SupportedCurrencies() []currency.Type { return g.Inner.SupportedCurrencies() }

func (g *Guard) IsAvailable(ctx context.Context) bool { return g.Inner.IsAvailable(ctx) }

// screen returns a refusal result when the move must not happen, and nil when
// the gateway should be called. An error from the screener itself is a fault,
// not a judgement, and is returned as one — the caller decides, once, in the
// place that knows whether a fault should stop a payment.
func (g *Guard) screen(ctx context.Context, req processor.PaymentRequest) (*processor.PaymentResult, error) {
	rec, err := g.Screener.Screen(ctx, Move{
		Stage:     Payment,
		Subject:   Subject{Kind: KindCustomer, ID: req.CustomerID},
		Amount:    req.Amount,
		Currency:  req.Currency,
		Signals:   Signals(req.Metadata),
		Reference: req.OrderID,
		Processor: string(g.Inner.Type()),
		Idem:      req.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	if !Refused(rec) {
		return nil, nil
	}
	return &processor.PaymentResult{
		Success:      false,
		Status:       rec.Action,
		ErrorMessage: refusalMessage(rec.Reason),
		Error:        ErrRefused,
		Metadata:     map[string]any{"screen": rec.Id(), "decision": rec.Decision},
	}, ErrRefused
}

func refusalMessage(reason string) string {
	if reason == "" {
		return "payment refused by risk"
	}
	return "payment refused by risk: " + reason
}

// signalKeys is the CLOSED set of facts that may travel from a payment request
// to the scoring plane. It is an allowlist and not a denylist because a payment
// request's metadata is caller-shaped: a merchant who puts a card number in a
// metadata field must not thereby send it to a scoring plane, and no list of
// forbidden key names can be relied on to catch that.
var signalKeys = map[string]bool{
	"ip":          true,
	"asn":         true,
	"country":     true,
	"email":       true,
	"phone":       true,
	"device":      true,
	"fingerprint": true,
	"ua":          true,
	"bin":         true,
	"funding":     true,
	"brand":       true,
	"last4":       true,
	"channel":     true,
	"agent":       true,
	"session":     true,
}

// Signals projects a payment request's metadata onto the facts the scoring
// plane may see: allowlisted keys with string values, lowercased for one
// spelling per fact.
func Signals(meta map[string]any) map[string]string {
	if len(meta) == 0 {
		return nil
	}
	out := map[string]string{}
	for k, v := range meta {
		key := strings.ToLower(strings.TrimSpace(k))
		if !signalKeys[key] {
			continue
		}
		if s, ok := v.(string); ok && s != "" {
			out[key] = s
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
