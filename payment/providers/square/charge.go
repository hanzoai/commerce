package square

import (
	"context"

	"github.com/hanzoai/money"

	"github.com/hanzoai/commerce/payment/processor"
)

// Charge takes a tokenized payment and captures it at once. req.Token is a Square
// Web Payments SDK source id (card nonce, Apple Pay or Google Pay token) or a
// card-on-file id. TransactionID in the result is the Square payment id.
func (p *Provider) Charge(ctx context.Context, req processor.PaymentRequest) (*processor.PaymentResult, error) {
	if err := p.checkAvailable(); err != nil {
		return nil, err
	}
	return p.inner.Charge(ctx, req)
}

// Authorize takes a tokenized payment without capturing it; Capture or
// CancelAuthorization settles it.
func (p *Provider) Authorize(ctx context.Context, req processor.PaymentRequest) (*processor.PaymentResult, error) {
	if err := p.checkAvailable(); err != nil {
		return nil, err
	}
	return p.inner.Authorize(ctx, req)
}

// Capture completes an authorized payment by its Square payment id.
func (p *Provider) Capture(ctx context.Context, transactionID string, amount money.Amount) (*processor.PaymentResult, error) {
	if err := p.checkAvailable(); err != nil {
		return nil, err
	}
	return p.inner.Capture(ctx, transactionID, amount)
}

// GetTransaction reads a payment by its Square payment id.
func (p *Provider) GetTransaction(ctx context.Context, txID string) (*processor.Transaction, error) {
	if err := p.checkAvailable(); err != nil {
		return nil, err
	}
	return p.inner.GetTransaction(ctx, txID)
}

// CancelAuthorization voids an uncaptured authorization.
func (p *Provider) CancelAuthorization(ctx context.Context, paymentID string) error {
	if err := p.checkAvailable(); err != nil {
		return err
	}
	return p.inner.CancelAuthorization(ctx, paymentID)
}

// checkAvailable refuses an API call on a provider with no API client.
func (p *Provider) checkAvailable() error {
	if p.inner == nil || p.config.AccessToken == "" || p.config.LocationID == "" {
		return processor.NewPaymentError(processor.Square, "NOT_CONFIGURED",
			"square processor not configured", processor.ErrProcessorNotAvailable)
	}
	return nil
}
