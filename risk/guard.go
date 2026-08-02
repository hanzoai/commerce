// Copyright © 2026 Hanzo AI. MIT License.

package risk

import (
	"context"

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
