package setupintent

import (
	"testing"
)

// --- Confirm ---

func TestConfirm_FromRequiresConfirmation(t *testing.T) {
	si := &SetupIntent{
		Status:          RequiresConfirmation,
		PaymentMethodId: "pm_123",
	}
	if err := si.Confirm(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if si.Status != Processing {
		t.Errorf("expected %s, got %s", Processing, si.Status)
	}
}

func TestConfirm_FromRequiresPaymentMethod(t *testing.T) {
	si := &SetupIntent{
		Status:          RequiresPaymentMethod,
		PaymentMethodId: "pm_456",
	}
	if err := si.Confirm(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if si.Status != Processing {
		t.Errorf("expected %s, got %s", Processing, si.Status)
	}
}

func TestConfirm_MissingPaymentMethod(t *testing.T) {
	si := &SetupIntent{
		Status: RequiresConfirmation,
	}
	err := si.Confirm()
	if err == nil {
		t.Fatal("expected error for missing payment method")
	}
}

func TestConfirm_InvalidStatus_Processing(t *testing.T) {
	si := &SetupIntent{
		Status:          Processing,
		PaymentMethodId: "pm_123",
	}
	err := si.Confirm()
	if err == nil {
		t.Fatal("expected error confirming from Processing")
	}
}

func TestConfirm_InvalidStatus_Succeeded(t *testing.T) {
	si := &SetupIntent{
		Status:          Succeeded,
		PaymentMethodId: "pm_123",
	}
	err := si.Confirm()
	if err == nil {
		t.Fatal("expected error confirming from Succeeded")
	}
}

func TestConfirm_InvalidStatus_Canceled(t *testing.T) {
	si := &SetupIntent{
		Status:          Canceled,
		PaymentMethodId: "pm_123",
	}
	err := si.Confirm()
	if err == nil {
		t.Fatal("expected error confirming from Canceled")
	}
}

func TestConfirm_InvalidStatus_RequiresAction(t *testing.T) {
	si := &SetupIntent{
		Status:          RequiresAction,
		PaymentMethodId: "pm_123",
	}
	err := si.Confirm()
	if err == nil {
		t.Fatal("expected error confirming from RequiresAction")
	}
}

// --- MarkSucceeded ---

func TestMarkSucceeded(t *testing.T) {
	si := &SetupIntent{Status: Processing}
	si.MarkSucceeded("seti_ref_abc")
	if si.Status != Succeeded {
		t.Errorf("expected %s, got %s", Succeeded, si.Status)
	}
	if si.ProviderRef != "seti_ref_abc" {
		t.Errorf("expected providerRef seti_ref_abc, got %s", si.ProviderRef)
	}
}

func TestMarkSucceeded_OverwritesProviderRef(t *testing.T) {
	si := &SetupIntent{
		Status:      Processing,
		ProviderRef: "old_ref",
	}
	si.MarkSucceeded("new_ref")
	if si.ProviderRef != "new_ref" {
		t.Errorf("expected new_ref, got %s", si.ProviderRef)
	}
}

// --- Cancel ---

func TestCancel_FromRequiresPaymentMethod(t *testing.T) {
	si := &SetupIntent{Status: RequiresPaymentMethod}
	if err := si.Cancel("abandoned"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if si.Status != Canceled {
		t.Errorf("expected %s, got %s", Canceled, si.Status)
	}
	if si.CancellationReason != "abandoned" {
		t.Errorf("expected reason 'abandoned', got %q", si.CancellationReason)
	}
	if si.CanceledAt.IsZero() {
		t.Error("expected CanceledAt to be set")
	}
}

func TestCancel_FromRequiresConfirmation(t *testing.T) {
	si := &SetupIntent{Status: RequiresConfirmation}
	if err := si.Cancel("duplicate"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if si.Status != Canceled {
		t.Errorf("expected %s, got %s", Canceled, si.Status)
	}
}

func TestCancel_FromRequiresAction(t *testing.T) {
	si := &SetupIntent{Status: RequiresAction}
	if err := si.Cancel("expired"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if si.Status != Canceled {
		t.Errorf("expected %s, got %s", Canceled, si.Status)
	}
}

func TestCancel_FromProcessing(t *testing.T) {
	si := &SetupIntent{Status: Processing}
	if err := si.Cancel("timeout"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if si.Status != Canceled {
		t.Errorf("expected %s, got %s", Canceled, si.Status)
	}
}

func TestCancel_InvalidStatus_Succeeded(t *testing.T) {
	si := &SetupIntent{Status: Succeeded}
	err := si.Cancel("too_late")
	if err == nil {
		t.Fatal("expected error canceling from Succeeded")
	}
}

func TestCancel_InvalidStatus_Canceled(t *testing.T) {
	si := &SetupIntent{Status: Canceled}
	err := si.Cancel("again")
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
	si := &SetupIntent{
		Status:          RequiresConfirmation,
		PaymentMethodId: "pm_test",
	}
	if err := si.Confirm(); err != nil {
		t.Fatalf("Confirm: %v", err)
	}
	si.MarkSucceeded("seti_done")
	if si.Status != Succeeded {
		t.Errorf("expected Succeeded, got %s", si.Status)
	}
}

func TestFullLifecycle_ConfirmThenCancel(t *testing.T) {
	si := &SetupIntent{
		Status:          RequiresPaymentMethod,
		PaymentMethodId: "pm_cancel",
	}
	if err := si.Confirm(); err != nil {
		t.Fatalf("Confirm: %v", err)
	}
	if err := si.Cancel("changed_mind"); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if si.Status != Canceled {
		t.Errorf("expected Canceled, got %s", si.Status)
	}
}
