package square

import (
	"context"

	"github.com/hanzoai/commerce/payment/processor"
)

// The customer and card-on-file calls the billing handlers make on the registered
// Square processor (api/billing/square_cardonfile.go asserts them), each refused on
// a provider with no API client.

// CreateCustomer creates a Square customer and returns its id.
func (p *Provider) CreateCustomer(ctx context.Context, email, name string, metadata map[string]interface{}) (string, error) {
	if err := p.checkAvailable(); err != nil {
		return "", err
	}
	return p.inner.CreateCustomer(ctx, email, name, metadata)
}

// FindCustomer returns the oldest customer whose reference_id is reference, or "".
func (p *Provider) FindCustomer(ctx context.Context, reference string) (string, error) {
	if err := p.checkAvailable(); err != nil {
		return "", err
	}
	return p.inner.FindCustomer(ctx, reference)
}

// GetCustomer reads a customer.
func (p *Provider) GetCustomer(ctx context.Context, customerID string) (map[string]interface{}, error) {
	if err := p.checkAvailable(); err != nil {
		return nil, err
	}
	return p.inner.GetCustomer(ctx, customerID)
}

// UpdateCustomer changes a customer.
func (p *Provider) UpdateCustomer(ctx context.Context, customerID string, updates map[string]interface{}) error {
	if err := p.checkAvailable(); err != nil {
		return err
	}
	return p.inner.UpdateCustomer(ctx, customerID, updates)
}

// DeleteCustomer removes a customer.
func (p *Provider) DeleteCustomer(ctx context.Context, customerID string) error {
	if err := p.checkAvailable(); err != nil {
		return err
	}
	return p.inner.DeleteCustomer(ctx, customerID)
}

// AddPaymentMethod vaults a card nonce on a customer and returns the card id.
func (p *Provider) AddPaymentMethod(ctx context.Context, customerID, token string) (string, error) {
	if err := p.checkAvailable(); err != nil {
		return "", err
	}
	return p.inner.AddPaymentMethod(ctx, customerID, token)
}

// Vault vaults a card nonce on a customer and returns the card as Square reports it.
func (p *Provider) Vault(ctx context.Context, customerID, token string) (processor.Card, error) {
	if err := p.checkAvailable(); err != nil {
		return processor.Card{}, err
	}
	return p.inner.Vault(ctx, customerID, token)
}

// Cards lists the cards vaulted on a customer.
func (p *Provider) Cards(ctx context.Context, customerID string) ([]processor.Card, error) {
	if err := p.checkAvailable(); err != nil {
		return nil, err
	}
	return p.inner.Cards(ctx, customerID)
}

// RemovePaymentMethod disables a vaulted card.
func (p *Provider) RemovePaymentMethod(ctx context.Context, customerID, paymentMethodID string) error {
	if err := p.checkAvailable(); err != nil {
		return err
	}
	return p.inner.RemovePaymentMethod(ctx, customerID, paymentMethodID)
}
