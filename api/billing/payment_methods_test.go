package billing

import (
	"context"
	"errors"
	"testing"

	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
)

// mockSquareProcessor implements processor.PaymentProcessor + preAuthVerifier
type mockSquareProcessor = MockSquareProcessor

func newMockSquare(authorizeErr error, authorizeID string, cancelErr error) *mockSquareProcessor {
	m := &MockSquareProcessor{
		BaseProcessor: *processor.NewBaseProcessor(processor.Square, []currency.Type{currency.USD}),
		authorizeErr:  authorizeErr,
		authorizeID:   authorizeID,
		cancelErr:     cancelErr,
	}
	m.SetConfigured(true)
	return m
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
