package stripe

import (
	"context"
	"net/http"
	"time"

	"github.com/stripe/stripe-go/v84"
	"github.com/stripe/stripe-go/v84/client"
	"github.com/stripe/stripe-go/v84/webhook"

	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
)

// StripeProcessor implements the processor.PaymentProcessor interface
type StripeProcessor struct {
	*processor.BaseProcessor
	accessToken   string
	webhookSecret string
	client        *client.API
}

// NewProcessor creates a new Stripe processor
func NewProcessor(accessToken, webhookSecret string) *StripeProcessor {
	sp := &StripeProcessor{
		BaseProcessor: processor.NewBaseProcessor(processor.Stripe, StripeSupportedCurrencies()),
		accessToken:   accessToken,
		webhookSecret: webhookSecret,
	}

	if accessToken != "" {
		sp.initClient()
		sp.SetConfigured(true)
	}

	return sp
}

// initClient initializes the Stripe client
func (sp *StripeProcessor) initClient() {
	httpClient := &http.Client{Timeout: 55 * time.Second}
	stripe.SetBackend(stripe.APIBackend, nil)
	stripe.SetHTTPClient(httpClient)

	sp.client = &client.API{}
	sp.client.Init(sp.accessToken, nil)
}

// StripeSupportedCurrencies returns all currencies Stripe supports
func StripeSupportedCurrencies() []currency.Type {
	return []currency.Type{
		currency.USD, currency.EUR, currency.GBP, currency.CAD, currency.AUD,
		currency.JPY, currency.CHF, currency.HKD, currency.SGD, currency.SEK,
		currency.DKK, currency.NOK, currency.NZD, currency.MXN, currency.BRL,
		currency.PLN, currency.CZK, currency.HUF, currency.RON, currency.BGN,
		currency.INR, currency.MYR, currency.THB, currency.PHP, currency.TWD,
		currency.KRW, currency.CNY, currency.AED, currency.ZAR,
	}
}

// Type returns the processor type
func (sp *StripeProcessor) Type() processor.ProcessorType {
	return processor.Stripe
}

// Charge processes a payment
func (sp *StripeProcessor) Charge(ctx context.Context, req processor.PaymentRequest) (*processor.PaymentResult, error) {
	if err := processor.ValidateRequest(req); err != nil {
		return nil, err
	}

	params := &stripe.ChargeParams{
		Amount:   stripe.Int64(int64(req.Amount)),
		Currency: stripe.String(string(req.Currency)),
		Capture:  stripe.Bool(true),
	}

	if req.Description != "" {
		params.Description = stripe.String(req.Description)
	}

	// Set payment source
	if req.Token != "" {
		params.SetSource(req.Token)
	} else if req.CustomerID != "" {
		params.Customer = stripe.String(req.CustomerID)
	}

	// Add metadata
	for k, v := range req.Metadata {
		if s, ok := v.(string); ok {
			params.AddMetadata(k, s)
		}
	}

	if req.OrderID != "" {
		params.AddMetadata("order", req.OrderID)
	}

	params.AddExpand("balance_transaction")

	ch, err := sp.client.Charges.New(params)
	if err != nil {
		return &processor.PaymentResult{
			Success:      false,
			Error:        err,
			ErrorMessage: err.Error(),
		}, err
	}

	fee := currency.Cents(0)
	if ch.BalanceTransaction != nil {
		fee = currency.Cents(ch.BalanceTransaction.Fee)
	}

	return &processor.PaymentResult{
		Success:       true,
		TransactionID: ch.ID,
		ProcessorRef:  ch.ID,
		Fee:           fee,
		Status:        string(ch.Status),
		Metadata: map[string]interface{}{
			"receipt_url": ch.ReceiptURL,
			"captured":    ch.Captured,
		},
	}, nil
}

// Authorize authorizes a payment without capturing
func (sp *StripeProcessor) Authorize(ctx context.Context, req processor.PaymentRequest) (*processor.PaymentResult, error) {
	if err := processor.ValidateRequest(req); err != nil {
		return nil, err
	}

	params := &stripe.ChargeParams{
		Amount:   stripe.Int64(int64(req.Amount)),
		Currency: stripe.String(string(req.Currency)),
		Capture:  stripe.Bool(false), // Authorize only
	}

	if req.Description != "" {
		params.Description = stripe.String(req.Description)
	}

	if req.Token != "" {
		params.SetSource(req.Token)
	} else if req.CustomerID != "" {
		params.Customer = stripe.String(req.CustomerID)
	}

	for k, v := range req.Metadata {
		if s, ok := v.(string); ok {
			params.AddMetadata(k, s)
		}
	}

	params.AddExpand("balance_transaction")

	ch, err := sp.client.Charges.New(params)
	if err != nil {
		return &processor.PaymentResult{
			Success:      false,
			Error:        err,
			ErrorMessage: err.Error(),
		}, err
	}

	return &processor.PaymentResult{
		Success:       true,
		TransactionID: ch.ID,
		ProcessorRef:  ch.ID,
		Status:        "authorized",
		Metadata: map[string]interface{}{
			"captured": ch.Captured,
		},
	}, nil
}

// Capture captures a previously authorized payment
func (sp *StripeProcessor) Capture(ctx context.Context, transactionID string, amount currency.Cents) (*processor.PaymentResult, error) {
	params := &stripe.ChargeCaptureParams{}
	if amount > 0 {
		params.Amount = stripe.Int64(int64(amount))
	}

	ch, err := sp.client.Charges.Capture(transactionID, params)
	if err != nil {
		return &processor.PaymentResult{
			Success:      false,
			Error:        err,
			ErrorMessage: err.Error(),
		}, err
	}

	fee := currency.Cents(0)
	if ch.BalanceTransaction != nil {
		fee = currency.Cents(ch.BalanceTransaction.Fee)
	}

	return &processor.PaymentResult{
		Success:       true,
		TransactionID: ch.ID,
		ProcessorRef:  ch.ID,
		Fee:           fee,
		Status:        "captured",
	}, nil
}

// Refund processes a refund
func (sp *StripeProcessor) Refund(ctx context.Context, req processor.RefundRequest) (*processor.RefundResult, error) {
	params := &stripe.RefundParams{
		Charge: stripe.String(req.TransactionID),
	}

	if req.Amount > 0 {
		params.Amount = stripe.Int64(int64(req.Amount))
	}

	if req.Reason != "" {
		params.AddMetadata("reason", req.Reason)
	}

	refund, err := sp.client.Refunds.New(params)
	if err != nil {
		return &processor.RefundResult{
			Success:      false,
			Error:        err,
			ErrorMessage: err.Error(),
		}, err
	}

	return &processor.RefundResult{
		Success:      true,
		RefundID:     refund.ID,
		ProcessorRef: refund.ID,
	}, nil
}

// GetTransaction retrieves transaction details
func (sp *StripeProcessor) GetTransaction(ctx context.Context, txID string) (*processor.Transaction, error) {
	params := &stripe.ChargeParams{
		Expand: []*string{stripe.String("balance_transaction")},
	}

	ch, err := sp.client.Charges.Get(txID, params)
	if err != nil {
		return nil, err
	}

	fee := currency.Cents(0)
	if ch.BalanceTransaction != nil {
		fee = currency.Cents(ch.BalanceTransaction.Fee)
	}

	return &processor.Transaction{
		ID:           ch.ID,
		ProcessorRef: ch.ID,
		Type:         "charge",
		Amount:       currency.Cents(ch.Amount),
		Currency:     currency.Type(ch.Currency),
		Status:       string(ch.Status),
		Fee:          fee,
		CustomerID:   ch.Customer.ID,
		CreatedAt:    ch.Created,
		UpdatedAt:    ch.Created,
	}, nil
}

// ValidateWebhook validates an incoming webhook
func (sp *StripeProcessor) ValidateWebhook(ctx context.Context, payload []byte, signature string) (*processor.WebhookEvent, error) {
	event, err := webhook.ConstructEvent(payload, signature, sp.webhookSecret)
	if err != nil {
		return nil, processor.ErrWebhookValidationFailed
	}

	return &processor.WebhookEvent{
		ID:        event.ID,
		Type:      string(event.Type),
		Processor: processor.Stripe,
		Data:      event.Data.Object,
		Timestamp: event.Created,
	}, nil
}

// IsAvailable checks if the processor is configured and available
func (sp *StripeProcessor) IsAvailable(ctx context.Context) bool {
	return sp.accessToken != "" && sp.client != nil
}

// Ensure StripeProcessor implements PaymentProcessor
var _ processor.PaymentProcessor = (*StripeProcessor)(nil)

// StripeSubscriptionProcessor extends StripeProcessor with subscription support
type StripeSubscriptionProcessor struct {
	*StripeProcessor
}

// NewSubscriptionProcessor creates a processor with subscription support
func NewSubscriptionProcessor(accessToken, webhookSecret string) *StripeSubscriptionProcessor {
	return &StripeSubscriptionProcessor{
		StripeProcessor: NewProcessor(accessToken, webhookSecret),
	}
}

// CreateSubscription creates a recurring subscription
func (sp *StripeSubscriptionProcessor) CreateSubscription(ctx context.Context, req processor.SubscriptionRequest) (*processor.Subscription, error) {
	params := &stripe.SubscriptionParams{
		Customer: stripe.String(req.CustomerID),
		Items: []*stripe.SubscriptionItemsParams{
			{
				Price:    stripe.String(req.PlanID),
				Quantity: stripe.Int64(int64(req.Quantity)),
			},
		},
	}

	if req.TrialDays > 0 {
		params.TrialPeriodDays = stripe.Int64(int64(req.TrialDays))
	}

	for k, v := range req.Metadata {
		if s, ok := v.(string); ok {
			params.AddMetadata(k, s)
		}
	}

	sub, err := sp.client.Subscriptions.New(params)
	if err != nil {
		return nil, err
	}

	result := &processor.Subscription{
		ID:                sub.ID,
		CustomerID:        sub.Customer.ID,
		PlanID:            req.PlanID,
		Status:            string(sub.Status),
		CancelAtPeriodEnd: sub.CancelAtPeriodEnd,
	}
	if len(sub.Items.Data) > 0 {
		result.CurrentPeriodStart = sub.Items.Data[0].CurrentPeriodStart
		result.CurrentPeriodEnd = sub.Items.Data[0].CurrentPeriodEnd
	}
	return result, nil
}

// GetSubscription retrieves subscription details
func (sp *StripeSubscriptionProcessor) GetSubscription(ctx context.Context, subscriptionID string) (*processor.Subscription, error) {
	sub, err := sp.client.Subscriptions.Get(subscriptionID, nil)
	if err != nil {
		return nil, err
	}

	planID := ""
	if len(sub.Items.Data) > 0 && sub.Items.Data[0].Price != nil {
		planID = sub.Items.Data[0].Price.ID
	}

	result := &processor.Subscription{
		ID:                sub.ID,
		CustomerID:        sub.Customer.ID,
		PlanID:            planID,
		Status:            string(sub.Status),
		CancelAtPeriodEnd: sub.CancelAtPeriodEnd,
	}
	if len(sub.Items.Data) > 0 {
		result.CurrentPeriodStart = sub.Items.Data[0].CurrentPeriodStart
		result.CurrentPeriodEnd = sub.Items.Data[0].CurrentPeriodEnd
	}
	return result, nil
}

// CancelSubscription cancels a subscription
func (sp *StripeSubscriptionProcessor) CancelSubscription(ctx context.Context, subscriptionID string, immediately bool) error {
	if immediately {
		_, err := sp.client.Subscriptions.Cancel(subscriptionID, nil)
		return err
	}

	// Cancel at period end
	params := &stripe.SubscriptionParams{
		CancelAtPeriodEnd: stripe.Bool(true),
	}
	_, err := sp.client.Subscriptions.Update(subscriptionID, params)
	return err
}

// UpdateSubscription modifies a subscription
func (sp *StripeSubscriptionProcessor) UpdateSubscription(ctx context.Context, subscriptionID string, req processor.SubscriptionUpdate) (*processor.Subscription, error) {
	params := &stripe.SubscriptionParams{}

	if req.PlanID != "" {
		params.Items = []*stripe.SubscriptionItemsParams{
			{Price: stripe.String(req.PlanID)},
		}
	}

	if req.CancelAtPeriodEnd != nil {
		params.CancelAtPeriodEnd = req.CancelAtPeriodEnd
	}

	sub, err := sp.client.Subscriptions.Update(subscriptionID, params)
	if err != nil {
		return nil, err
	}

	planID := ""
	if len(sub.Items.Data) > 0 && sub.Items.Data[0].Price != nil {
		planID = sub.Items.Data[0].Price.ID
	}

	result := &processor.Subscription{
		ID:                sub.ID,
		CustomerID:        sub.Customer.ID,
		PlanID:            planID,
		Status:            string(sub.Status),
		CancelAtPeriodEnd: sub.CancelAtPeriodEnd,
	}
	if len(sub.Items.Data) > 0 {
		result.CurrentPeriodStart = sub.Items.Data[0].CurrentPeriodStart
		result.CurrentPeriodEnd = sub.Items.Data[0].CurrentPeriodEnd
	}
	return result, nil
}

// ListSubscriptions lists subscriptions for a customer
func (sp *StripeSubscriptionProcessor) ListSubscriptions(ctx context.Context, customerID string) ([]*processor.Subscription, error) {
	params := &stripe.SubscriptionListParams{
		Customer: stripe.String(customerID),
	}

	iter := sp.client.Subscriptions.List(params)
	var subs []*processor.Subscription

	for iter.Next() {
		s := iter.Subscription()
		planID := ""
		if len(s.Items.Data) > 0 && s.Items.Data[0].Price != nil {
			planID = s.Items.Data[0].Price.ID
		}

		sub := &processor.Subscription{
			ID:                s.ID,
			CustomerID:        s.Customer.ID,
			PlanID:            planID,
			Status:            string(s.Status),
			CancelAtPeriodEnd: s.CancelAtPeriodEnd,
		}
		if len(s.Items.Data) > 0 {
			sub.CurrentPeriodStart = s.Items.Data[0].CurrentPeriodStart
			sub.CurrentPeriodEnd = s.Items.Data[0].CurrentPeriodEnd
		}
		subs = append(subs, sub)
	}

	if err := iter.Err(); err != nil {
		return nil, err
	}

	return subs, nil
}

// Ensure StripeSubscriptionProcessor implements SubscriptionProcessor
var _ processor.SubscriptionProcessor = (*StripeSubscriptionProcessor)(nil)
