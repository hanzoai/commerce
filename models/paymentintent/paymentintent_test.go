package paymentintent

import (
	"testing"
)

// --- Confirm ---

func TestConfirm_FromRequiresConfirmation(t *testing.T) {
	pi := &PaymentIntent{
		Status:          RequiresConfirmation,
		PaymentMethodId: "pm_123",
	}
	if err := pi.Confirm(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pi.Status != Processing {
		t.Errorf("expected %s, got %s", Processing, pi.Status)
	}
}

func TestConfirm_FromRequiresPaymentMethod(t *testing.T) {
	pi := &PaymentIntent{
		Status:          RequiresPaymentMethod,
		PaymentMethodId: "pm_456",
	}
	if err := pi.Confirm(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pi.Status != Processing {
		t.Errorf("expected %s, got %s", Processing, pi.Status)
	}
}

func TestConfirm_MissingPaymentMethod(t *testing.T) {
	pi := &PaymentIntent{
		Status: RequiresConfirmation,
	}
	err := pi.Confirm()
	if err == nil {
		t.Fatal("expected error for missing payment method")
	}
}

func TestConfirm_InvalidStatus_Processing(t *testing.T) {
	pi := &PaymentIntent{
		Status:          Processing,
		PaymentMethodId: "pm_123",
	}
	err := pi.Confirm()
	if err == nil {
		t.Fatal("expected error confirming from Processing")
	}
}

func TestConfirm_InvalidStatus_Succeeded(t *testing.T) {
	pi := &PaymentIntent{
		Status:          Succeeded,
		PaymentMethodId: "pm_123",
	}
	err := pi.Confirm()
	if err == nil {
		t.Fatal("expected error confirming from Succeeded")
	}
}

func TestConfirm_InvalidStatus_Canceled(t *testing.T) {
	pi := &PaymentIntent{
		Status:          Canceled,
		PaymentMethodId: "pm_123",
	}
	err := pi.Confirm()
	if err == nil {
		t.Fatal("expected error confirming from Canceled")
	}
}

func TestConfirm_InvalidStatus_RequiresCapture(t *testing.T) {
	pi := &PaymentIntent{
		Status:          RequiresCapture,
		PaymentMethodId: "pm_123",
	}
	err := pi.Confirm()
	if err == nil {
		t.Fatal("expected error confirming from RequiresCapture")
	}
}

// --- MarkSucceeded ---

func TestMarkSucceeded(t *testing.T) {
	pi := &PaymentIntent{
		Status:           Processing,
		Amount:           5000,
		AmountCapturable: 5000,
	}
	pi.MarkSucceeded("ch_abc", 5000)
	if pi.Status != Succeeded {
		t.Errorf("expected %s, got %s", Succeeded, pi.Status)
	}
	if pi.ProviderRef != "ch_abc" {
		t.Errorf("expected providerRef ch_abc, got %s", pi.ProviderRef)
	}
	if pi.AmountReceived != 5000 {
		t.Errorf("expected amountReceived 5000, got %d", pi.AmountReceived)
	}
	if pi.AmountCapturable != 0 {
		t.Errorf("expected amountCapturable 0, got %d", pi.AmountCapturable)
	}
}

func TestMarkSucceeded_PartialAmount(t *testing.T) {
	pi := &PaymentIntent{Status: Processing, Amount: 10000}
	pi.MarkSucceeded("ch_partial", 7500)
	if pi.AmountReceived != 7500 {
		t.Errorf("expected 7500, got %d", pi.AmountReceived)
	}
}

// --- MarkRequiresCapture ---

func TestMarkRequiresCapture(t *testing.T) {
	pi := &PaymentIntent{
		Status: Processing,
		Amount: 8000,
	}
	pi.MarkRequiresCapture("ch_auth")
	if pi.Status != RequiresCapture {
		t.Errorf("expected %s, got %s", RequiresCapture, pi.Status)
	}
	if pi.ProviderRef != "ch_auth" {
		t.Errorf("expected providerRef ch_auth, got %s", pi.ProviderRef)
	}
	if pi.AmountCapturable != 8000 {
		t.Errorf("expected amountCapturable 8000, got %d", pi.AmountCapturable)
	}
}

// --- Capture ---

func TestCapture_FullAmount(t *testing.T) {
	pi := &PaymentIntent{
		Status:           RequiresCapture,
		Amount:           5000,
		AmountCapturable: 5000,
	}
	if err := pi.Capture(5000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pi.Status != Succeeded {
		t.Errorf("expected %s, got %s", Succeeded, pi.Status)
	}
	if pi.AmountReceived != 5000 {
		t.Errorf("expected amountReceived 5000, got %d", pi.AmountReceived)
	}
	if pi.AmountCapturable != 0 {
		t.Errorf("expected amountCapturable 0, got %d", pi.AmountCapturable)
	}
}

func TestCapture_PartialAmount(t *testing.T) {
	pi := &PaymentIntent{
		Status:           RequiresCapture,
		Amount:           10000,
		AmountCapturable: 10000,
	}
	if err := pi.Capture(6000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pi.AmountReceived != 6000 {
		t.Errorf("expected 6000, got %d", pi.AmountReceived)
	}
	if pi.Status != Succeeded {
		t.Errorf("expected %s, got %s", Succeeded, pi.Status)
	}
}

func TestCapture_ExceedsCapturable(t *testing.T) {
	pi := &PaymentIntent{
		Status:           RequiresCapture,
		AmountCapturable: 3000,
	}
	err := pi.Capture(5000)
	if err == nil {
		t.Fatal("expected error for amount exceeding capturable")
	}
}

func TestCapture_InvalidStatus_Processing(t *testing.T) {
	pi := &PaymentIntent{Status: Processing}
	err := pi.Capture(1000)
	if err == nil {
		t.Fatal("expected error capturing from Processing")
	}
}

func TestCapture_InvalidStatus_Succeeded(t *testing.T) {
	pi := &PaymentIntent{Status: Succeeded}
	err := pi.Capture(1000)
	if err == nil {
		t.Fatal("expected error capturing from Succeeded")
	}
}

func TestCapture_ZeroAmount(t *testing.T) {
	pi := &PaymentIntent{
		Status:           RequiresCapture,
		AmountCapturable: 5000,
	}
	if err := pi.Capture(0); err != nil {
		t.Fatalf("unexpected error capturing zero: %v", err)
	}
	if pi.AmountReceived != 0 {
		t.Errorf("expected 0, got %d", pi.AmountReceived)
	}
}

// --- Cancel ---

func TestCancel_FromProcessing(t *testing.T) {
	pi := &PaymentIntent{Status: Processing}
	if err := pi.Cancel("duplicate"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pi.Status != Canceled {
		t.Errorf("expected %s, got %s", Canceled, pi.Status)
	}
	if pi.CancellationReason != "duplicate" {
		t.Errorf("expected reason 'duplicate', got %q", pi.CancellationReason)
	}
	if pi.CanceledAt.IsZero() {
		t.Error("expected CanceledAt to be set")
	}
}

func TestCancel_FromRequiresPaymentMethod(t *testing.T) {
	pi := &PaymentIntent{Status: RequiresPaymentMethod}
	if err := pi.Cancel("abandoned"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pi.Status != Canceled {
		t.Errorf("expected %s, got %s", Canceled, pi.Status)
	}
}

func TestCancel_FromRequiresConfirmation(t *testing.T) {
	pi := &PaymentIntent{Status: RequiresConfirmation}
	if err := pi.Cancel("requested_by_customer"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pi.Status != Canceled {
		t.Errorf("expected %s, got %s", Canceled, pi.Status)
	}
}

func TestCancel_FromRequiresAction(t *testing.T) {
	pi := &PaymentIntent{Status: RequiresAction}
	if err := pi.Cancel("expired"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pi.Status != Canceled {
		t.Errorf("expected %s, got %s", Canceled, pi.Status)
	}
}

func TestCancel_FromRequiresCapture(t *testing.T) {
	pi := &PaymentIntent{Status: RequiresCapture}
	if err := pi.Cancel("voided"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pi.Status != Canceled {
		t.Errorf("expected %s, got %s", Canceled, pi.Status)
	}
}

func TestCancel_InvalidStatus_Succeeded(t *testing.T) {
	pi := &PaymentIntent{Status: Succeeded}
	err := pi.Cancel("too_late")
	if err == nil {
		t.Fatal("expected error canceling from Succeeded")
	}
}

func TestCancel_InvalidStatus_Canceled(t *testing.T) {
	pi := &PaymentIntent{Status: Canceled}
	err := pi.Cancel("again")
	if err == nil {
		t.Fatal("expected error canceling already-canceled intent")
	}
}

// --- Status constants ---

func TestStatusConstants(t *testing.T) {
	cases := []struct {
		status Status
		want   string
	}{
		{RequiresPaymentMethod, "requires_payment_method"},
		{RequiresConfirmation, "requires_confirmation"},
		{RequiresAction, "requires_action"},
		{Processing, "processing"},
		{RequiresCapture, "requires_capture"},
		{Succeeded, "succeeded"},
		{Canceled, "canceled"},
	}
	for _, tc := range cases {
		if string(tc.status) != tc.want {
			t.Errorf("status %q != %q", tc.status, tc.want)
		}
	}
}

// --- Full lifecycle ---

func TestFullLifecycle_ConfirmThenSucceed(t *testing.T) {
	pi := &PaymentIntent{
		Status:          RequiresConfirmation,
		PaymentMethodId: "pm_test",
		Amount:          2500,
	}
	if err := pi.Confirm(); err != nil {
		t.Fatalf("Confirm: %v", err)
	}
	pi.MarkSucceeded("ch_final", 2500)
	if pi.Status != Succeeded {
		t.Errorf("expected Succeeded, got %s", pi.Status)
	}
}

func TestFullLifecycle_ConfirmAuthCapture(t *testing.T) {
	pi := &PaymentIntent{
		Status:          RequiresConfirmation,
		PaymentMethodId: "pm_auth",
		Amount:          9900,
	}
	if err := pi.Confirm(); err != nil {
		t.Fatalf("Confirm: %v", err)
	}
	pi.MarkRequiresCapture("ch_hold")
	if err := pi.Capture(9900); err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if pi.Status != Succeeded {
		t.Errorf("expected Succeeded, got %s", pi.Status)
	}
}

func TestFullLifecycle_ConfirmThenCancel(t *testing.T) {
	pi := &PaymentIntent{
		Status:          RequiresConfirmation,
		PaymentMethodId: "pm_cancel",
		Amount:          1000,
	}
	if err := pi.Confirm(); err != nil {
		t.Fatalf("Confirm: %v", err)
	}
	if err := pi.Cancel("changed_mind"); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if pi.Status != Canceled {
		t.Errorf("expected Canceled, got %s", pi.Status)
	}
}
