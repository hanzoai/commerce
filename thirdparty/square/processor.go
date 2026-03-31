package square

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	square "github.com/square/square-go-sdk/v3"
	"github.com/square/square-go-sdk/v3/core"
	"github.com/square/square-go-sdk/v3/option"
	"github.com/square/square-go-sdk/v3/customers/client"
	"github.com/square/square-go-sdk/v3/payments"
	"github.com/square/square-go-sdk/v3/refunds"

	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
)

// SquareProcessor implements the processor.PaymentProcessor interface
type SquareProcessor struct {
	*processor.BaseProcessor
	accessToken    string
	locationID     string
	webhookSecret  string
	environment    string // "sandbox" or "production"
	paymentsClient  *payments.Client
	refundsClient   *refunds.Client
	customersClient *client.Client
}

// Config holds Square processor configuration
type Config struct {
	AccessToken   string
	LocationID    string
	WebhookSecret string
	Environment   string // "sandbox" or "production"
}

// NewProcessor creates a new Square processor
func NewProcessor(cfg Config) *SquareProcessor {
	sp := &SquareProcessor{
		BaseProcessor: processor.NewBaseProcessor(processor.Square, SquareSupportedCurrencies()),
		accessToken:   cfg.AccessToken,
		locationID:    cfg.LocationID,
		webhookSecret: cfg.WebhookSecret,
		environment:   cfg.Environment,
	}

	if cfg.AccessToken != "" {
		sp.initClient()
		sp.SetConfigured(true)
	}

	return sp
}

// initClient initializes the Square client
func (sp *SquareProcessor) initClient() {
	opts := []option.RequestOption{
		option.WithToken(sp.accessToken),
	}

	if sp.environment == "sandbox" {
		opts = append(opts, option.WithBaseURL("https://connect.squareupsandbox.com"))
	}

	reqOpts := core.NewRequestOptions(opts...)
	sp.paymentsClient = payments.NewClient(reqOpts)
	sp.refundsClient = refunds.NewClient(reqOpts)
	sp.customersClient = client.NewClient(reqOpts)
}

// SquareSupportedCurrencies returns currencies Square supports
func SquareSupportedCurrencies() []currency.Type {
	return []currency.Type{
		currency.USD, currency.CAD, currency.GBP, currency.EUR,
		currency.AUD, currency.JPY,
	}
}

// Type returns the processor type
func (sp *SquareProcessor) Type() processor.ProcessorType {
	return processor.Square
}

// Charge processes a payment
func (sp *SquareProcessor) Charge(ctx context.Context, req processor.PaymentRequest) (*processor.PaymentResult, error) {
	if err := processor.ValidateRequest(req); err != nil {
		return nil, err
	}

	idempotencyKey := uuid.New().String()

	paymentReq := &square.CreatePaymentRequest{
		SourceID:       req.Token,
		IdempotencyKey: idempotencyKey,
		AmountMoney: &square.Money{
			Amount:   square.Int64(int64(req.Amount)),
			Currency: squareCurrency(req.Currency),
		},
		LocationID:   square.String(sp.locationID),
		Autocomplete: square.Bool(true), // Capture immediately
	}

	if req.CustomerID != "" {
		paymentReq.CustomerID = square.String(req.CustomerID)
	}

	if req.Description != "" {
		paymentReq.Note = square.String(req.Description)
	}

	if req.OrderID != "" {
		paymentReq.ReferenceID = square.String(req.OrderID)
	}

	resp, err := sp.paymentsClient.Create(ctx, paymentReq)
	if err != nil {
		return &processor.PaymentResult{
			Success:      false,
			Error:        err,
			ErrorMessage: err.Error(),
		}, err
	}

	payment := resp.Payment
	fee := currency.Cents(0)
	if len(payment.ProcessingFee) > 0 && payment.ProcessingFee[0].AmountMoney != nil {
		fee = currency.Cents(*payment.ProcessingFee[0].AmountMoney.Amount)
	}

	paymentStatus := ""
	if payment.Status != nil {
		paymentStatus = *payment.Status
	}

	return &processor.PaymentResult{
		Success:       true,
		TransactionID: *payment.ID,
		ProcessorRef:  *payment.ID,
		Fee:           fee,
		Status:        paymentStatus,
		Metadata: map[string]interface{}{
			"receipt_url": payment.ReceiptURL,
			"order_id":    payment.OrderID,
		},
	}, nil
}

// Authorize authorizes a payment without capturing
func (sp *SquareProcessor) Authorize(ctx context.Context, req processor.PaymentRequest) (*processor.PaymentResult, error) {
	if err := processor.ValidateRequest(req); err != nil {
		return nil, err
	}

	idempotencyKey := uuid.New().String()

	paymentReq := &square.CreatePaymentRequest{
		SourceID:       req.Token,
		IdempotencyKey: idempotencyKey,
		AmountMoney: &square.Money{
			Amount:   square.Int64(int64(req.Amount)),
			Currency: squareCurrency(req.Currency),
		},
		LocationID:   square.String(sp.locationID),
		Autocomplete: square.Bool(false), // Authorize only
	}

	if req.CustomerID != "" {
		paymentReq.CustomerID = square.String(req.CustomerID)
	}

	if req.Description != "" {
		paymentReq.Note = square.String(req.Description)
	}

	resp, err := sp.paymentsClient.Create(ctx, paymentReq)
	if err != nil {
		return &processor.PaymentResult{
			Success:      false,
			Error:        err,
			ErrorMessage: err.Error(),
		}, err
	}

	return &processor.PaymentResult{
		Success:       true,
		TransactionID: *resp.Payment.ID,
		ProcessorRef:  *resp.Payment.ID,
		Status:        "authorized",
		Metadata: map[string]interface{}{
			"order_id": resp.Payment.OrderID,
		},
	}, nil
}

// Capture captures a previously authorized payment
func (sp *SquareProcessor) Capture(ctx context.Context, transactionID string, amount currency.Cents) (*processor.PaymentResult, error) {
	resp, err := sp.paymentsClient.Complete(ctx, &square.CompletePaymentRequest{
		PaymentID: transactionID,
	})
	if err != nil {
		return &processor.PaymentResult{
			Success:      false,
			Error:        err,
			ErrorMessage: err.Error(),
		}, err
	}

	payment := resp.Payment
	fee := currency.Cents(0)
	if len(payment.ProcessingFee) > 0 && payment.ProcessingFee[0].AmountMoney != nil {
		fee = currency.Cents(*payment.ProcessingFee[0].AmountMoney.Amount)
	}

	return &processor.PaymentResult{
		Success:       true,
		TransactionID: *payment.ID,
		ProcessorRef:  *payment.ID,
		Fee:           fee,
		Status:        "captured",
	}, nil
}

// Refund processes a refund
func (sp *SquareProcessor) Refund(ctx context.Context, req processor.RefundRequest) (*processor.RefundResult, error) {
	idempotencyKey := uuid.New().String()

	refundReq := &square.RefundPaymentRequest{
		IdempotencyKey: idempotencyKey,
		PaymentID:      square.String(req.TransactionID),
		AmountMoney: &square.Money{
			Amount:   square.Int64(int64(req.Amount)),
			Currency: squareCurrency(currency.USD), // Would get from transaction
		},
	}

	if req.Reason != "" {
		refundReq.Reason = square.String(req.Reason)
	}

	resp, err := sp.refundsClient.RefundPayment(ctx, refundReq)
	if err != nil {
		return &processor.RefundResult{
			Success:      false,
			Error:        err,
			ErrorMessage: err.Error(),
		}, err
	}

	return &processor.RefundResult{
		Success:      true,
		RefundID:     resp.Refund.ID,
		ProcessorRef: resp.Refund.ID,
	}, nil
}

// GetTransaction retrieves transaction details
func (sp *SquareProcessor) GetTransaction(ctx context.Context, txID string) (*processor.Transaction, error) {
	resp, err := sp.paymentsClient.Get(ctx, &square.GetPaymentsRequest{
		PaymentID: txID,
	})
	if err != nil {
		return nil, err
	}

	payment := resp.Payment
	fee := currency.Cents(0)
	if len(payment.ProcessingFee) > 0 && payment.ProcessingFee[0].AmountMoney != nil {
		fee = currency.Cents(*payment.ProcessingFee[0].AmountMoney.Amount)
	}

	createdAt := int64(0)
	if payment.CreatedAt != nil {
		if t, err := time.Parse(time.RFC3339, *payment.CreatedAt); err == nil {
			createdAt = t.Unix()
		}
	}

	customerID := ""
	if payment.CustomerID != nil {
		customerID = *payment.CustomerID
	}

	paymentStatus := ""
	if payment.Status != nil {
		paymentStatus = *payment.Status
	}

	return &processor.Transaction{
		ID:           *payment.ID,
		ProcessorRef: *payment.ID,
		Type:         "charge",
		Amount:       currency.Cents(*payment.AmountMoney.Amount),
		Currency:     currency.Type(*payment.AmountMoney.Currency),
		Status:       paymentStatus,
		Fee:          fee,
		CustomerID:   customerID,
		CreatedAt:    createdAt,
		UpdatedAt:    createdAt,
	}, nil
}

// ValidateWebhook validates an incoming webhook and parses the event.
func (sp *SquareProcessor) ValidateWebhook(ctx context.Context, payload []byte, signature string) (*processor.WebhookEvent, error) {
	if sp.webhookSecret == "" {
		return nil, processor.ErrWebhookValidationFailed
	}

	// Square uses HMAC-SHA256 for webhook signatures
	mac := hmac.New(sha256.New, []byte(sp.webhookSecret))
	mac.Write(payload)
	expectedSig := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(signature), []byte(expectedSig)) {
		return nil, processor.ErrWebhookValidationFailed
	}

	// Parse the webhook event JSON
	var evt struct {
		MerchantID string `json:"merchant_id"`
		Type       string `json:"type"`
		EventID    string `json:"event_id"`
		CreatedAt  string `json:"created_at"`
		Data       struct {
			Type   string                 `json:"type"`
			ID     string                 `json:"id"`
			Object map[string]interface{} `json:"object"`
		} `json:"data"`
	}

	if err := json.Unmarshal(payload, &evt); err != nil {
		return nil, fmt.Errorf("failed to parse Square webhook: %w", err)
	}

	ts := time.Now().Unix()
	if evt.CreatedAt != "" {
		if t, err := time.Parse(time.RFC3339, evt.CreatedAt); err == nil {
			ts = t.Unix()
		}
	}

	return &processor.WebhookEvent{
		ID:        evt.EventID,
		Type:      mapSquareEventType(evt.Type),
		Processor: processor.Square,
		Data:      evt.Data.Object,
		Timestamp: ts,
	}, nil
}

// mapSquareEventType converts Square event types to normalized types.
func mapSquareEventType(sqType string) string {
	switch sqType {
	case "payment.completed":
		return "payment.completed"
	case "payment.created":
		return "payment.created"
	case "payment.updated":
		return "payment.updated"
	case "refund.created":
		return "refund.created"
	case "refund.updated":
		return "refund.updated"
	case "invoice.payment_made":
		return "invoice.paid"
	case "subscription.created":
		return "subscription.created"
	case "subscription.updated":
		return "subscription.updated"
	case "dispute.created":
		return "dispute.created"
	case "dispute.state.changed":
		return "dispute.updated"
	case "customer.created":
		return "customer.created"
	case "customer.updated":
		return "customer.updated"
	case "customer.deleted":
		return "customer.deleted"
	default:
		return sqType
	}
}

// CancelAuthorization voids a previously authorized (uncaptured) payment.
// Call this immediately after a successful pre-auth to release the hold.
func (sp *SquareProcessor) CancelAuthorization(ctx context.Context, paymentID string) error {
	_, err := sp.paymentsClient.Cancel(ctx, &square.CancelPaymentsRequest{
		PaymentID: paymentID,
	})
	if err != nil {
		return fmt.Errorf("cancel authorization %s: %w", paymentID, err)
	}
	return nil
}

// IsAvailable checks if the processor is configured and available
func (sp *SquareProcessor) IsAvailable(ctx context.Context) bool {
	return sp.accessToken != "" && sp.locationID != "" && sp.paymentsClient != nil
}

// squareCurrency converts currency.Type to Square currency pointer.
// Square requires uppercase ISO 4217 codes (e.g. "USD" not "usd").
func squareCurrency(c currency.Type) *square.Currency {
	curr := square.Currency(strings.ToUpper(string(c)))
	return &curr
}

// Ensure SquareProcessor implements PaymentProcessor
var _ processor.PaymentProcessor = (*SquareProcessor)(nil)
