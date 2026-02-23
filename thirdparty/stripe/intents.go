package stripe

import (
	"context"
	"fmt"

	sgo "github.com/stripe/stripe-go/v84"

	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
)

// ChargeViaIntent creates a PaymentIntent, confirms it immediately with
// automatic capture, and returns the result.
func (sp *StripeProcessor) ChargeViaIntent(ctx context.Context, req processor.PaymentRequest) (*processor.PaymentResult, error) {
	if err := processor.ValidateRequest(req); err != nil {
		return nil, err
	}

	params := &sgo.PaymentIntentParams{
		Amount:        sgo.Int64(int64(req.Amount)),
		Currency:      sgo.String(string(req.Currency)),
		Confirm:       sgo.Bool(true),
		CaptureMethod: sgo.String("automatic"),
	}

	if req.Token != "" {
		params.PaymentMethod = sgo.String(req.Token)
	}
	if req.CustomerID != "" {
		params.Customer = sgo.String(req.CustomerID)
	}
	if req.Description != "" {
		params.Description = sgo.String(req.Description)
	}

	params.Metadata = make(map[string]string)
	for k, v := range req.Metadata {
		if s, ok := v.(string); ok {
			params.Metadata[k] = s
		}
	}
	if req.OrderID != "" {
		params.Metadata["order"] = req.OrderID
	}

	params.AddExpand("latest_charge.balance_transaction")

	pi, err := sp.client.PaymentIntents.New(params)
	if err != nil {
		return &processor.PaymentResult{
			Success:      false,
			Error:        err,
			ErrorMessage: err.Error(),
		}, err
	}

	fee := currency.Cents(0)
	if pi.LatestCharge != nil && pi.LatestCharge.BalanceTransaction != nil {
		fee = currency.Cents(pi.LatestCharge.BalanceTransaction.Fee)
	}

	return &processor.PaymentResult{
		Success:       true,
		TransactionID: pi.ID,
		ProcessorRef:  pi.ID,
		Fee:           fee,
		Status:        string(pi.Status),
		Metadata: map[string]interface{}{
			"client_secret": pi.ClientSecret,
			"captured":      pi.Status == sgo.PaymentIntentStatusSucceeded,
		},
	}, nil
}

// AuthorizeViaIntent creates a PaymentIntent with manual capture so funds
// are held but not collected until CaptureIntent is called.
func (sp *StripeProcessor) AuthorizeViaIntent(ctx context.Context, req processor.PaymentRequest) (*processor.PaymentResult, error) {
	if err := processor.ValidateRequest(req); err != nil {
		return nil, err
	}

	params := &sgo.PaymentIntentParams{
		Amount:        sgo.Int64(int64(req.Amount)),
		Currency:      sgo.String(string(req.Currency)),
		Confirm:       sgo.Bool(true),
		CaptureMethod: sgo.String("manual"),
	}

	if req.Token != "" {
		params.PaymentMethod = sgo.String(req.Token)
	}
	if req.CustomerID != "" {
		params.Customer = sgo.String(req.CustomerID)
	}
	if req.Description != "" {
		params.Description = sgo.String(req.Description)
	}

	params.Metadata = make(map[string]string)
	for k, v := range req.Metadata {
		if s, ok := v.(string); ok {
			params.Metadata[k] = s
		}
	}
	if req.OrderID != "" {
		params.Metadata["order"] = req.OrderID
	}

	pi, err := sp.client.PaymentIntents.New(params)
	if err != nil {
		return &processor.PaymentResult{
			Success:      false,
			Error:        err,
			ErrorMessage: err.Error(),
		}, err
	}

	return &processor.PaymentResult{
		Success:       true,
		TransactionID: pi.ID,
		ProcessorRef:  pi.ID,
		Status:        string(pi.Status),
		Metadata: map[string]interface{}{
			"client_secret":    pi.ClientSecret,
			"amount_capturable": pi.AmountCapturable,
		},
	}, nil
}

// CaptureIntent captures a previously authorized PaymentIntent.
func (sp *StripeProcessor) CaptureIntent(ctx context.Context, intentID string, amount currency.Cents) (*processor.PaymentResult, error) {
	params := &sgo.PaymentIntentCaptureParams{}
	if amount > 0 {
		params.AmountToCapture = sgo.Int64(int64(amount))
	}
	params.AddExpand("latest_charge.balance_transaction")

	pi, err := sp.client.PaymentIntents.Capture(intentID, params)
	if err != nil {
		return &processor.PaymentResult{
			Success:      false,
			Error:        err,
			ErrorMessage: err.Error(),
		}, err
	}

	fee := currency.Cents(0)
	if pi.LatestCharge != nil && pi.LatestCharge.BalanceTransaction != nil {
		fee = currency.Cents(pi.LatestCharge.BalanceTransaction.Fee)
	}

	return &processor.PaymentResult{
		Success:       true,
		TransactionID: pi.ID,
		ProcessorRef:  pi.ID,
		Fee:           fee,
		Status:        "captured",
	}, nil
}

// CreateSetupIntent creates a SetupIntent for saving a payment method
// without charging the customer. Returns (setupIntentID, clientSecret, error).
func (sp *StripeProcessor) CreateSetupIntent(ctx context.Context, customerID string, usage string) (string, string, error) {
	params := &sgo.SetupIntentParams{}
	if customerID != "" {
		params.Customer = sgo.String(customerID)
	}
	if usage != "" {
		params.Usage = sgo.String(usage)
	}

	si, err := sp.client.SetupIntents.New(params)
	if err != nil {
		return "", "", fmt.Errorf("create setup intent: %w", err)
	}

	return si.ID, si.ClientSecret, nil
}

// ConfirmSetupIntent confirms a SetupIntent with a payment method.
func (sp *StripeProcessor) ConfirmSetupIntent(ctx context.Context, setupIntentID, paymentMethodID string) error {
	params := &sgo.SetupIntentConfirmParams{
		PaymentMethod: sgo.String(paymentMethodID),
	}

	_, err := sp.client.SetupIntents.Confirm(setupIntentID, params)
	if err != nil {
		return fmt.Errorf("confirm setup intent: %w", err)
	}

	return nil
}

// AttachPaymentMethod attaches a payment method to a customer.
func (sp *StripeProcessor) AttachPaymentMethod(ctx context.Context, paymentMethodID, customerID string) error {
	params := &sgo.PaymentMethodAttachParams{
		Customer: sgo.String(customerID),
	}

	_, err := sp.client.PaymentMethods.Attach(paymentMethodID, params)
	if err != nil {
		return fmt.Errorf("attach payment method: %w", err)
	}

	return nil
}

// DetachPaymentMethod detaches a payment method from its customer.
func (sp *StripeProcessor) DetachPaymentMethod(ctx context.Context, paymentMethodID string) error {
	params := &sgo.PaymentMethodDetachParams{}

	_, err := sp.client.PaymentMethods.Detach(paymentMethodID, params)
	if err != nil {
		return fmt.Errorf("detach payment method: %w", err)
	}

	return nil
}

// CreateCustomer creates a Stripe customer and returns the customer ID.
func (sp *StripeProcessor) CreateCustomer(ctx context.Context, email, name string, metadata map[string]interface{}) (string, error) {
	params := &sgo.CustomerParams{}
	if email != "" {
		params.Email = sgo.String(email)
	}
	if name != "" {
		params.Description = sgo.String(name)
	}

	for k, v := range metadata {
		if s, ok := v.(string); ok {
			params.AddMetadata(k, s)
		}
	}

	cust, err := sp.client.Customers.New(params)
	if err != nil {
		return "", fmt.Errorf("create customer: %w", err)
	}

	return cust.ID, nil
}

// UpdateCustomer updates a Stripe customer's details.
func (sp *StripeProcessor) UpdateCustomer(ctx context.Context, customerID string, updates map[string]interface{}) error {
	params := &sgo.CustomerParams{}

	if email, ok := updates["email"].(string); ok {
		params.Email = sgo.String(email)
	}
	if desc, ok := updates["description"].(string); ok {
		params.Description = sgo.String(desc)
	}
	if name, ok := updates["name"].(string); ok {
		params.Description = sgo.String(name)
	}

	// Pass remaining string keys as metadata.
	for k, v := range updates {
		switch k {
		case "email", "description", "name":
			continue
		default:
			if s, ok := v.(string); ok {
				params.AddMetadata(k, s)
			}
		}
	}

	_, err := sp.client.Customers.Update(customerID, params)
	if err != nil {
		return fmt.Errorf("update customer: %w", err)
	}

	return nil
}

// DeleteCustomer permanently deletes a Stripe customer.
func (sp *StripeProcessor) DeleteCustomer(ctx context.Context, customerID string) error {
	_, err := sp.client.Customers.Del(customerID, nil)
	if err != nil {
		return fmt.Errorf("delete customer: %w", err)
	}

	return nil
}
