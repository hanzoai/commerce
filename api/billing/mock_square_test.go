// Copyright 2014-present Hanzo AI Inc. Licensed under MIT OR Apache-2.0.

package billing

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/money"
)

// SetProcessorsForOrg replaces the processor resolver, returning a cleanup function.
func SetProcessorsForOrg(fn func(*organization.Organization) *processor.Registry) func() {
	old := processorsForOrg
	processorsForOrg = fn
	return func() { processorsForOrg = old }
}

// MockSquareProcessor provides an in-memory Square processor implementing
// PaymentProcessor, squareCustomerProcessor, vaulter, and finder for tests and local simulation.
type MockSquareProcessor struct {
	processor.BaseProcessor

	authorizeErr error
	authorizeID  string
	cancelErr    error
	cancelCalled bool
	cancelledID  string

	createCustomerErr   error
	createdCustomerID   string
	createCustomerCalls int
	customers           map[string]string // reference → customer id, what FindCustomer finds
	findErr             error
	findCalls           int
	addCardErr          error
	addedCardID         string
	addCardCustomerID   string
	addCardNonce        string
	removeErr           error
	removeCalled        bool
	removeCustomerID    string
	removeCardID        string

	vaultCard processor.Card
	listCards []processor.Card
	listErr   error

	mu                 sync.Mutex
	chargeErr          error
	chargeRef          string
	chargeCalls        int
	lastChargeToken    string
	lastChargeCustomer string
	lastChargeAmount   int64
	chargedKeys        map[string]*processor.PaymentResult

	// A wallet's payment: authorized (naming walletCard, or walletBrand for a
	// wallet that names none), then captured or voided.
	walletCard          *processor.Card
	walletBrand         string
	authorizeCalls      int
	lastAuthorizeAmount int64
	authorizeKeys       []string
	captureErr          error
	captureStatus       string // "" captures (COMPLETED)
	captureCalls        int
	cancelCalls         int
	// authorizeLost is how many authorizations go unanswered before one is
	// answered; Square answers a repeated key with the first call's payment.
	authorizeLost int
	// captureLost is how many captures go unanswered before captureErr applies.
	captureLost int
	// readStatus is the status a payment reads back as; "" goes unanswered.
	readStatus string
	readCalls  int
}

// NewMockSquareProcessor creates a mock Square processor that vaults to (customerID, cardID)
// and charges to chargeRef.
func NewMockSquareProcessor(customerID, cardID, chargeRef string) *MockSquareProcessor {
	m := &MockSquareProcessor{
		BaseProcessor:     *processor.NewBaseProcessor(processor.Square, []currency.Type{currency.USD}),
		createdCustomerID: customerID,
		addedCardID:       cardID,
		chargeRef:         chargeRef,
	}
	m.SetConfigured(true)
	return m
}

func (m *MockSquareProcessor) Type() processor.ProcessorType { return processor.Square }

func (m *MockSquareProcessor) Charge(ctx context.Context, req processor.PaymentRequest) (*processor.PaymentResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.IdempotencyKey != "" {
		if prev, ok := m.chargedKeys[req.IdempotencyKey]; ok {
			return prev, prev.Error
		}
	}

	m.chargeCalls++
	m.lastChargeToken = req.Token
	m.lastChargeCustomer = req.CustomerID
	m.lastChargeAmount = int64(req.Amount)

	var res *processor.PaymentResult
	if m.chargeErr != nil {
		res = &processor.PaymentResult{Success: false, ErrorMessage: m.chargeErr.Error(), Error: m.chargeErr}
	} else {
		ref := m.chargeRef
		if ref == "" {
			ref = "sqpay_test"
		}
		res = &processor.PaymentResult{Success: true, TransactionID: ref, ProcessorRef: ref, Status: "COMPLETED"}
	}
	if req.IdempotencyKey != "" {
		if m.chargedKeys == nil {
			m.chargedKeys = map[string]*processor.PaymentResult{}
		}
		m.chargedKeys[req.IdempotencyKey] = res
	}
	return res, res.Error
}

func (m *MockSquareProcessor) Authorize(ctx context.Context, req processor.PaymentRequest) (*processor.PaymentResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.authorizeCalls++
	m.lastAuthorizeAmount = int64(req.Amount)
	m.authorizeKeys = append(m.authorizeKeys, req.IdempotencyKey)
	if m.authorizeLost > 0 {
		m.authorizeLost--
		return nil, errors.New("authorize: connection reset")
	}
	if m.authorizeErr != nil {
		return &processor.PaymentResult{Success: false, ErrorMessage: m.authorizeErr.Error(), Error: m.authorizeErr}, m.authorizeErr
	}
	ref := m.authorizeID
	if ref == "" {
		ref = fmt.Sprintf("sqauth_%d", m.authorizeCalls)
	}
	return &processor.PaymentResult{
		Success:       true,
		TransactionID: ref,
		ProcessorRef:  ref,
		Status:        "APPROVED",
		Card:          m.walletCard,
		Wallet:        m.walletBrand,
	}, nil
}

func (m *MockSquareProcessor) Capture(ctx context.Context, txID string, amount money.Amount) (*processor.PaymentResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.captureCalls++
	if m.captureLost > 0 {
		m.captureLost--
		return nil, errors.New("capture: connection reset")
	}
	if m.captureErr != nil {
		return nil, m.captureErr
	}
	status := m.captureStatus
	if status == "" {
		status = "COMPLETED"
	}
	return &processor.PaymentResult{
		Success:       processor.Settled(status),
		TransactionID: txID,
		ProcessorRef:  txID,
		Status:        status,
		Card:          m.walletCard,
		Wallet:        m.walletBrand,
	}, nil
}

func (m *MockSquareProcessor) Refund(ctx context.Context, req processor.RefundRequest) (*processor.RefundResult, error) {
	return nil, errors.New("not implemented")
}

func (m *MockSquareProcessor) GetTransaction(ctx context.Context, txID string) (*processor.Transaction, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.readCalls++
	if m.readStatus == "" {
		return nil, errors.New("read: connection reset")
	}
	return &processor.Transaction{ID: txID, ProcessorRef: txID, Status: m.readStatus}, nil
}

func (m *MockSquareProcessor) ValidateWebhook(ctx context.Context, payload []byte, sig string) (*processor.WebhookEvent, error) {
	return nil, errors.New("not implemented")
}

func (m *MockSquareProcessor) IsAvailable(ctx context.Context) bool { return true }

func (m *MockSquareProcessor) CancelAuthorization(ctx context.Context, paymentID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cancelCalls++
	m.cancelCalled = true
	m.cancelledID = paymentID
	return m.cancelErr
}

func (m *MockSquareProcessor) CreateCustomer(ctx context.Context, email, name string, metadata map[string]interface{}) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.createCustomerCalls++
	if m.createCustomerErr != nil {
		return "", m.createCustomerErr
	}
	if ref, _ := metadata["reference"].(string); ref != "" {
		if m.customers == nil {
			m.customers = map[string]string{}
		}
		m.customers[ref] = m.createdCustomerID
	}
	return m.createdCustomerID, nil
}

// FindCustomer answers with the customer CreateCustomer made under reference,
// as Square's search does once it has indexed it.
func (m *MockSquareProcessor) FindCustomer(ctx context.Context, reference string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.findCalls++
	if m.findErr != nil {
		return "", m.findErr
	}
	return m.customers[reference], nil
}

func (m *MockSquareProcessor) AddPaymentMethod(ctx context.Context, customerID, token string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.addCardCustomerID = customerID
	m.addCardNonce = token
	if m.addCardErr != nil {
		return "", m.addCardErr
	}
	return m.addedCardID, nil
}

func (m *MockSquareProcessor) RemovePaymentMethod(ctx context.Context, customerID, paymentMethodID string) error {
	m.removeCalled = true
	m.removeCustomerID = customerID
	m.removeCardID = paymentMethodID
	return m.removeErr
}

func (m *MockSquareProcessor) Vault(ctx context.Context, customerID, token string) (processor.Card, error) {
	id, err := m.AddPaymentMethod(ctx, customerID, token)
	if err != nil {
		return processor.Card{}, err
	}
	card := m.vaultCard
	if card.ID == "" {
		card.ID = id
	}
	if card.CustomerID == "" {
		card.CustomerID = customerID
	}
	return card, nil
}

func (m *MockSquareProcessor) Cards(ctx context.Context, customerID string) ([]processor.Card, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.listCards, nil
}
