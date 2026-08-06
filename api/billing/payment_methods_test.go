package billing

import (
	"context"
	"errors"
	"github.com/hanzoai/money"
	"sync"
	"testing"

	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
)

// mockSquareProcessor implements processor.PaymentProcessor + preAuthVerifier
// for unit testing the pre-auth flow without hitting Square.
type mockSquareProcessor struct {
	processor.BaseProcessor

	// Controls whether Authorize succeeds
	authorizeErr error
	authorizeID  string

	// Controls whether CancelAuthorization succeeds
	cancelErr    error
	cancelCalled bool
	cancelledID  string

	// Card-on-file (CustomerProcessor) behavior + call capture.
	createCustomerErr   error
	createdCustomerID   string
	createCustomerCalls int
	addCardErr          error
	addedCardID         string
	addCardCustomerID   string
	addCardNonce        string
	removeErr           error
	removeCalled        bool
	removeCustomerID    string
	removeCardID        string

	// Richer vault/list faces. vaultCard, when set, is the card facts Vault
	// reports (its ID defaults to addedCardID); listCards is what Cards returns.
	vaultCard processor.Card
	listCards []processor.Card
	listErr   error

	// Charge (saved-card) behavior + call capture — used by the subscribe/card
	// + renewal tests. Unset chargeErr => success with chargeRef (or a default).
	// mu guards the counters so the concurrency tests can drive Charge in parallel;
	// chargedKeys makes Charge idempotent on req.IdempotencyKey (mirrors real Square:
	// a repeat key returns the first SUCCESS with no new charge), which is exactly
	// the double-charge backstop the stable-key fix relies on.
	mu                 sync.Mutex
	chargeErr          error
	chargeRef          string
	chargeCalls        int
	lastChargeToken    string
	lastChargeCustomer string
	lastChargeAmount   int64
	chargedKeys        map[string]*processor.PaymentResult
}

func newMockSquare(authorizeErr error, authorizeID string, cancelErr error) *mockSquareProcessor {
	m := &mockSquareProcessor{
		BaseProcessor: *processor.NewBaseProcessor(processor.Square, []currency.Type{currency.USD}),
		authorizeErr:  authorizeErr,
		authorizeID:   authorizeID,
		cancelErr:     cancelErr,
	}
	m.SetConfigured(true)
	return m
}

func (m *mockSquareProcessor) Type() processor.ProcessorType { return processor.Square }

func (m *mockSquareProcessor) Charge(ctx context.Context, req processor.PaymentRequest) (*processor.PaymentResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Square de-dups on the idempotency key: a repeat key returns the FIRST result
	// (success OR decline) with no new charge. So a dunning retry that must actually
	// re-charge REQUIRES a fresh key — which the (invoice, attempt) scoping provides.
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

func (m *mockSquareProcessor) Authorize(ctx context.Context, req processor.PaymentRequest) (*processor.PaymentResult, error) {
	if m.authorizeErr != nil {
		return &processor.PaymentResult{Success: false, ErrorMessage: m.authorizeErr.Error(), Error: m.authorizeErr}, m.authorizeErr
	}
	return &processor.PaymentResult{
		Success:       true,
		TransactionID: m.authorizeID,
		ProcessorRef:  m.authorizeID,
		Status:        "authorized",
	}, nil
}

func (m *mockSquareProcessor) Capture(ctx context.Context, txID string, amount money.Amount) (*processor.PaymentResult, error) {
	return nil, errors.New("not implemented")
}

func (m *mockSquareProcessor) Refund(ctx context.Context, req processor.RefundRequest) (*processor.RefundResult, error) {
	return nil, errors.New("not implemented")
}

func (m *mockSquareProcessor) GetTransaction(ctx context.Context, txID string) (*processor.Transaction, error) {
	return nil, errors.New("not implemented")
}

func (m *mockSquareProcessor) ValidateWebhook(ctx context.Context, payload []byte, sig string) (*processor.WebhookEvent, error) {
	return nil, errors.New("not implemented")
}

func (m *mockSquareProcessor) IsAvailable(ctx context.Context) bool { return true }

func (m *mockSquareProcessor) CancelAuthorization(ctx context.Context, paymentID string) error {
	m.cancelCalled = true
	m.cancelledID = paymentID
	return m.cancelErr
}

// --- squareCustomerProcessor implementation ---------------------------------

func (m *mockSquareProcessor) CreateCustomer(ctx context.Context, email, name string, metadata map[string]interface{}) (string, error) {
	m.createCustomerCalls++
	if m.createCustomerErr != nil {
		return "", m.createCustomerErr
	}
	return m.createdCustomerID, nil
}

func (m *mockSquareProcessor) AddPaymentMethod(ctx context.Context, customerID, token string) (string, error) {
	m.addCardCustomerID = customerID
	m.addCardNonce = token
	if m.addCardErr != nil {
		return "", m.addCardErr
	}
	return m.addedCardID, nil
}

func (m *mockSquareProcessor) RemovePaymentMethod(ctx context.Context, customerID, paymentMethodID string) error {
	m.removeCalled = true
	m.removeCustomerID = customerID
	m.removeCardID = paymentMethodID
	return m.removeErr
}

// Vault is the richer vault face: the same attach, reporting card facts.
// Facts default to just the id (a processor that reports nothing), so tests
// that never set vaultCard behave exactly as with AddPaymentMethod.
func (m *mockSquareProcessor) Vault(ctx context.Context, customerID, token string) (processor.Card, error) {
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

// Cards lists the vaulted cards for a customer (the heal face).
func (m *mockSquareProcessor) Cards(ctx context.Context, customerID string) ([]processor.Card, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.listCards, nil
}

// registerMockSquare replaces the Square processor in the global registry for the duration of a test.
func registerMockSquare(t *testing.T, mock *mockSquareProcessor) func() {
	t.Helper()
	old, _ := processor.Get(processor.Square)
	processor.Register(mock)
	return func() {
		if old != nil {
			processor.Register(old)
		}
	}
}

// ---- Tests ------------------------------------------------------------------

func TestVerifyCardWithPreAuth_Success(t *testing.T) {
	mock := newMockSquare(nil, "pay_abc123", nil)
	cleanup := registerMockSquare(t, mock)
	defer cleanup()

	err := verifyCardWithPreAuth(context.Background(), processor.Global(), "cnon:card-nonce-ok", "user-1")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Authorization should have been cancelled immediately.
	if !mock.cancelCalled {
		t.Error("expected CancelAuthorization to be called after successful pre-auth")
	}
	if mock.cancelledID != "pay_abc123" {
		t.Errorf("CancelAuthorization called with wrong ID: %q, want %q", mock.cancelledID, "pay_abc123")
	}
}

func TestVerifyCardWithPreAuth_CardDeclined(t *testing.T) {
	mock := newMockSquare(errors.New("CARD_DECLINED"), "", nil)
	cleanup := registerMockSquare(t, mock)
	defer cleanup()

	err := verifyCardWithPreAuth(context.Background(), processor.Global(), "cnon:card-nonce-declined", "user-2")
	if err == nil {
		t.Fatal("expected an error for declined card, got nil")
	}
	if mock.cancelCalled {
		t.Error("CancelAuthorization must not be called when pre-auth fails")
	}
}

func TestVerifyCardWithPreAuth_CancelFailureIsNonFatal(t *testing.T) {
	// Cancel fails but the overall verification should still succeed.
	mock := newMockSquare(nil, "pay_xyz789", errors.New("network timeout"))
	cleanup := registerMockSquare(t, mock)
	defer cleanup()

	err := verifyCardWithPreAuth(context.Background(), processor.Global(), "cnon:card-nonce-ok", "user-3")
	if err != nil {
		t.Fatalf("cancel failure must be non-fatal, but got: %v", err)
	}
	if !mock.cancelCalled {
		t.Error("expected CancelAuthorization to be attempted")
	}
}

func TestVerifyCardWithPreAuth_NoSquareProcessor(t *testing.T) {
	// Unregister Square to simulate it not being configured. Capture the
	// prior registration and restore it on cleanup so this test does not
	// pollute the shared global registry for tests that run after it.
	old, _ := processor.Get(processor.Square)
	processor.Global().Unregister(processor.Square)
	defer func() {
		if old != nil {
			processor.Register(old)
		}
	}()

	err := verifyCardWithPreAuth(context.Background(), processor.Global(), "cnon:card-nonce", "user-4")
	if err != nil {
		t.Fatalf("should skip pre-auth gracefully when Square is not registered, got: %v", err)
	}
}

// ---- Card-on-file -----------------------------------------------------------

func TestAttachSquareCardOnFile_CreatesCustomerAndCard(t *testing.T) {
	m := newMockSquare(nil, "", nil)
	m.createdCustomerID = "cust_new"
	m.addedCardID = "ccof_new"

	cof, err := attachSquareCardOnFile(context.Background(), m, "", "owner@acme.test", "acme", "cnon:fresh-nonce")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if m.createCustomerCalls != 1 {
		t.Errorf("expected CreateCustomer to be called once, got %d", m.createCustomerCalls)
	}
	if cof.CustomerID != "cust_new" || cof.CardID != "ccof_new" {
		t.Errorf("unexpected card-on-file: %+v", cof)
	}
	if m.addCardCustomerID != "cust_new" {
		t.Errorf("AddPaymentMethod got customer %q, want %q", m.addCardCustomerID, "cust_new")
	}
	if m.addCardNonce != "cnon:fresh-nonce" {
		t.Errorf("AddPaymentMethod got nonce %q, want the single-use nonce", m.addCardNonce)
	}
}

func TestAttachSquareCardOnFile_ReusesExistingCustomer(t *testing.T) {
	m := newMockSquare(nil, "", nil)
	m.addedCardID = "ccof_second"

	cof, err := attachSquareCardOnFile(context.Background(), m, "cust_existing", "", "acme", "cnon:second-card")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if m.createCustomerCalls != 0 {
		t.Errorf("expected CreateCustomer NOT to be called when reusing a customer, got %d calls", m.createCustomerCalls)
	}
	if cof.CustomerID != "cust_existing" {
		t.Errorf("expected reused customer %q, got %q", "cust_existing", cof.CustomerID)
	}
	if m.addCardCustomerID != "cust_existing" {
		t.Errorf("AddPaymentMethod got customer %q, want %q", m.addCardCustomerID, "cust_existing")
	}
}

func TestAttachSquareCardOnFile_DeclinedCard(t *testing.T) {
	m := newMockSquare(nil, "", nil)
	m.createdCustomerID = "cust_decline"
	m.addCardErr = errors.New("CARD_DECLINED")

	if _, err := attachSquareCardOnFile(context.Background(), m, "", "", "acme", "cnon:bad"); err == nil {
		t.Fatal("expected an error when the card is declined, got nil")
	}
}
