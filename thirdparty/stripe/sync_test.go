package stripe

import (
	"testing"
	"time"

	sgo "github.com/stripe/stripe-go/v84"

	"github.com/hanzoai/commerce/models/dispute"
	"github.com/hanzoai/commerce/models/paymentintent"
	"github.com/hanzoai/commerce/models/paymentmethod"
	"github.com/hanzoai/commerce/models/refund"
	"github.com/hanzoai/commerce/models/setupintent"
	"github.com/hanzoai/commerce/models/types/currency"
)

// --- MapWebhookEventType ---

func TestMapWebhookEventType(t *testing.T) {
	cases := []struct {
		stripe string
		want   string
	}{
		// PaymentIntent events
		{"payment_intent.succeeded", "payment_intent.succeeded"},
		{"payment_intent.payment_failed", "payment_intent.failed"},
		{"payment_intent.canceled", "payment_intent.canceled"},
		{"payment_intent.created", "payment_intent.created"},
		{"payment_intent.requires_action", "payment_intent.requires_action"},
		// SetupIntent events
		{"setup_intent.succeeded", "setup_intent.succeeded"},
		{"setup_intent.setup_failed", "setup_intent.failed"},
		// Charge/refund/dispute events
		{"charge.refunded", "refund.created"},
		{"charge.dispute.created", "dispute.created"},
		{"charge.dispute.updated", "dispute.updated"},
		{"charge.dispute.closed", "dispute.closed"},
		// Subscription events
		{"customer.subscription.created", "subscription.created"},
		{"customer.subscription.updated", "subscription.updated"},
		{"customer.subscription.deleted", "subscription.canceled"},
		// Invoice events
		{"invoice.paid", "invoice.paid"},
		{"invoice.payment_failed", "invoice.payment_failed"},
		{"invoice.finalized", "invoice.finalized"},
		{"invoice.voided", "invoice.voided"},
		// Payment method events
		{"payment_method.attached", "payment_method.attached"},
		{"payment_method.detached", "payment_method.detached"},
		// Unknown events pass through
		{"some.unknown.event", "some.unknown.event"},
		{"", ""},
	}

	for _, tc := range cases {
		t.Run(tc.stripe, func(t *testing.T) {
			got := MapWebhookEventType(tc.stripe)
			if got != tc.want {
				t.Errorf("MapWebhookEventType(%q) = %q, want %q", tc.stripe, got, tc.want)
			}
		})
	}
}

// --- SyncPaymentIntent ---

func TestSyncPaymentIntent_FullFields(t *testing.T) {
	src := &sgo.PaymentIntent{
		ID:       "pi_123",
		Amount:   5000,
		Currency: "usd",
		Status:   sgo.PaymentIntentStatusSucceeded,
		Customer: &sgo.Customer{ID: "cus_abc"},
		PaymentMethod: &sgo.PaymentMethod{
			ID: "pm_xyz",
		},
		CaptureMethod:      "automatic",
		ConfirmationMethod: "automatic",
		AmountCapturable:   0,
		AmountReceived:     5000,
		Description:        "Test charge",
		ReceiptEmail:       "user@example.com",
		CanceledAt:         1700000000,
		CancellationReason: "requested_by_customer",
		LastPaymentError:   &sgo.Error{Msg: "card_declined"},
		ClientSecret:       "pi_123_secret_abc",
	}

	dst := &paymentintent.PaymentIntent{}
	SyncPaymentIntent(dst, src)

	if dst.ProviderRef != "pi_123" {
		t.Errorf("ProviderRef = %q, want %q", dst.ProviderRef, "pi_123")
	}
	if dst.ProviderType != "stripe" {
		t.Errorf("ProviderType = %q, want %q", dst.ProviderType, "stripe")
	}
	if dst.Amount != 5000 {
		t.Errorf("Amount = %d, want %d", dst.Amount, 5000)
	}
	if dst.Currency != currency.Type("usd") {
		t.Errorf("Currency = %q, want %q", dst.Currency, "usd")
	}
	if dst.Status != paymentintent.Succeeded {
		t.Errorf("Status = %q, want %q", dst.Status, paymentintent.Succeeded)
	}
	if dst.CustomerId != "cus_abc" {
		t.Errorf("CustomerId = %q, want %q", dst.CustomerId, "cus_abc")
	}
	if dst.PaymentMethodId != "pm_xyz" {
		t.Errorf("PaymentMethodId = %q, want %q", dst.PaymentMethodId, "pm_xyz")
	}
	if dst.CaptureMethod != "automatic" {
		t.Errorf("CaptureMethod = %q, want %q", dst.CaptureMethod, "automatic")
	}
	if dst.ConfirmationMethod != "automatic" {
		t.Errorf("ConfirmationMethod = %q, want %q", dst.ConfirmationMethod, "automatic")
	}
	if dst.AmountCapturable != 0 {
		t.Errorf("AmountCapturable = %d, want %d", dst.AmountCapturable, 0)
	}
	if dst.AmountReceived != 5000 {
		t.Errorf("AmountReceived = %d, want %d", dst.AmountReceived, 5000)
	}
	if dst.Description != "Test charge" {
		t.Errorf("Description = %q, want %q", dst.Description, "Test charge")
	}
	if dst.ReceiptEmail != "user@example.com" {
		t.Errorf("ReceiptEmail = %q, want %q", dst.ReceiptEmail, "user@example.com")
	}
	if dst.ClientSecret != "pi_123_secret_abc" {
		t.Errorf("ClientSecret = %q, want %q", dst.ClientSecret, "pi_123_secret_abc")
	}
	if dst.CancellationReason != "requested_by_customer" {
		t.Errorf("CancellationReason = %q, want %q", dst.CancellationReason, "requested_by_customer")
	}
	if dst.LastError != "card_declined" {
		t.Errorf("LastError = %q, want %q", dst.LastError, "card_declined")
	}
	wantCanceled := time.Unix(1700000000, 0)
	if !dst.CanceledAt.Equal(wantCanceled) {
		t.Errorf("CanceledAt = %v, want %v", dst.CanceledAt, wantCanceled)
	}
}

func TestSyncPaymentIntent_NilOptionalFields(t *testing.T) {
	src := &sgo.PaymentIntent{
		ID:       "pi_456",
		Amount:   1000,
		Currency: "eur",
		Status:   sgo.PaymentIntentStatusRequiresPaymentMethod,
		// Customer, PaymentMethod, LastPaymentError all nil
		// CanceledAt = 0
	}

	dst := &paymentintent.PaymentIntent{}
	SyncPaymentIntent(dst, src)

	if dst.CustomerId != "" {
		t.Errorf("CustomerId = %q, want empty", dst.CustomerId)
	}
	if dst.PaymentMethodId != "" {
		t.Errorf("PaymentMethodId = %q, want empty", dst.PaymentMethodId)
	}
	if dst.LastError != "" {
		t.Errorf("LastError = %q, want empty", dst.LastError)
	}
	if !dst.CanceledAt.IsZero() {
		t.Errorf("CanceledAt = %v, want zero", dst.CanceledAt)
	}
	if dst.Status != paymentintent.RequiresPaymentMethod {
		t.Errorf("Status = %q, want %q", dst.Status, paymentintent.RequiresPaymentMethod)
	}
}

// Test all PaymentIntent status mappings via SyncPaymentIntent.
func TestSyncPaymentIntent_StatusMapping(t *testing.T) {
	cases := []struct {
		stripe sgo.PaymentIntentStatus
		want   paymentintent.Status
	}{
		{sgo.PaymentIntentStatusRequiresPaymentMethod, paymentintent.RequiresPaymentMethod},
		{sgo.PaymentIntentStatusRequiresConfirmation, paymentintent.RequiresConfirmation},
		{sgo.PaymentIntentStatusRequiresAction, paymentintent.RequiresAction},
		{sgo.PaymentIntentStatusProcessing, paymentintent.Processing},
		{sgo.PaymentIntentStatusRequiresCapture, paymentintent.RequiresCapture},
		{sgo.PaymentIntentStatusSucceeded, paymentintent.Succeeded},
		{sgo.PaymentIntentStatusCanceled, paymentintent.Canceled},
		// Unknown status falls through as-is.
		{sgo.PaymentIntentStatus("exotic_status"), paymentintent.Status("exotic_status")},
	}

	for _, tc := range cases {
		t.Run(string(tc.stripe), func(t *testing.T) {
			src := &sgo.PaymentIntent{Status: tc.stripe}
			dst := &paymentintent.PaymentIntent{}
			SyncPaymentIntent(dst, src)
			if dst.Status != tc.want {
				t.Errorf("status %q -> %q, want %q", tc.stripe, dst.Status, tc.want)
			}
		})
	}
}

// --- SyncSetupIntent ---

func TestSyncSetupIntent_FullFields(t *testing.T) {
	src := &sgo.SetupIntent{
		ID:                 "seti_789",
		Customer:           &sgo.Customer{ID: "cus_def"},
		PaymentMethod:      &sgo.PaymentMethod{ID: "pm_ghi"},
		Status:             sgo.SetupIntentStatusSucceeded,
		Usage:              "off_session",
		CancellationReason: "abandoned",
		LastSetupError:     &sgo.Error{Msg: "authentication_required"},
		ClientSecret:       "seti_789_secret_xyz",
	}

	dst := &setupintent.SetupIntent{}
	SyncSetupIntent(dst, src)

	if dst.ProviderRef != "seti_789" {
		t.Errorf("ProviderRef = %q, want %q", dst.ProviderRef, "seti_789")
	}
	if dst.ProviderType != "stripe" {
		t.Errorf("ProviderType = %q, want %q", dst.ProviderType, "stripe")
	}
	if dst.CustomerId != "cus_def" {
		t.Errorf("CustomerId = %q, want %q", dst.CustomerId, "cus_def")
	}
	if dst.PaymentMethodId != "pm_ghi" {
		t.Errorf("PaymentMethodId = %q, want %q", dst.PaymentMethodId, "pm_ghi")
	}
	if dst.Status != setupintent.Succeeded {
		t.Errorf("Status = %q, want %q", dst.Status, setupintent.Succeeded)
	}
	if dst.Usage != "off_session" {
		t.Errorf("Usage = %q, want %q", dst.Usage, "off_session")
	}
	if dst.CancellationReason != "abandoned" {
		t.Errorf("CancellationReason = %q, want %q", dst.CancellationReason, "abandoned")
	}
	if dst.LastError != "authentication_required" {
		t.Errorf("LastError = %q, want %q", dst.LastError, "authentication_required")
	}
	if dst.ClientSecret != "seti_789_secret_xyz" {
		t.Errorf("ClientSecret = %q, want %q", dst.ClientSecret, "seti_789_secret_xyz")
	}
}

func TestSyncSetupIntent_NilOptionalFields(t *testing.T) {
	src := &sgo.SetupIntent{
		ID:     "seti_empty",
		Status: sgo.SetupIntentStatusRequiresPaymentMethod,
	}

	dst := &setupintent.SetupIntent{}
	SyncSetupIntent(dst, src)

	if dst.CustomerId != "" {
		t.Errorf("CustomerId = %q, want empty", dst.CustomerId)
	}
	if dst.PaymentMethodId != "" {
		t.Errorf("PaymentMethodId = %q, want empty", dst.PaymentMethodId)
	}
	if dst.LastError != "" {
		t.Errorf("LastError = %q, want empty", dst.LastError)
	}
}

// Test all SetupIntent status mappings via SyncSetupIntent.
func TestSyncSetupIntent_StatusMapping(t *testing.T) {
	cases := []struct {
		stripe sgo.SetupIntentStatus
		want   setupintent.Status
	}{
		{sgo.SetupIntentStatusRequiresPaymentMethod, setupintent.RequiresPaymentMethod},
		{sgo.SetupIntentStatusRequiresConfirmation, setupintent.RequiresConfirmation},
		{sgo.SetupIntentStatusRequiresAction, setupintent.RequiresAction},
		{sgo.SetupIntentStatusProcessing, setupintent.Processing},
		{sgo.SetupIntentStatusSucceeded, setupintent.Succeeded},
		{sgo.SetupIntentStatusCanceled, setupintent.Canceled},
		// Unknown status falls through as-is.
		{sgo.SetupIntentStatus("new_status"), setupintent.Status("new_status")},
	}

	for _, tc := range cases {
		t.Run(string(tc.stripe), func(t *testing.T) {
			src := &sgo.SetupIntent{Status: tc.stripe}
			dst := &setupintent.SetupIntent{}
			SyncSetupIntent(dst, src)
			if dst.Status != tc.want {
				t.Errorf("status %q -> %q, want %q", tc.stripe, dst.Status, tc.want)
			}
		})
	}
}

// --- SyncPaymentMethod ---

func TestSyncPaymentMethod_WithCard(t *testing.T) {
	src := &sgo.PaymentMethod{
		ID:       "pm_card_123",
		Type:     "card",
		Customer: &sgo.Customer{ID: "cus_card"},
		Card: &sgo.PaymentMethodCard{
			Brand:    "visa",
			Last4:    "4242",
			ExpMonth: 12,
			ExpYear:  2027,
			Funding:  "credit",
			Country:  "US",
		},
	}

	dst := &paymentmethod.PaymentMethod{}
	SyncPaymentMethod(dst, src)

	if dst.ProviderRef != "pm_card_123" {
		t.Errorf("ProviderRef = %q, want %q", dst.ProviderRef, "pm_card_123")
	}
	if dst.ProviderType != "stripe" {
		t.Errorf("ProviderType = %q, want %q", dst.ProviderType, "stripe")
	}
	if dst.Type != "card" {
		t.Errorf("Type = %q, want %q", dst.Type, "card")
	}
	if dst.CustomerId != "cus_card" {
		t.Errorf("CustomerId = %q, want %q", dst.CustomerId, "cus_card")
	}
	if dst.Card == nil {
		t.Fatal("Card is nil")
	}
	if dst.Card.Brand != "visa" {
		t.Errorf("Card.Brand = %q, want %q", dst.Card.Brand, "visa")
	}
	if dst.Card.Last4 != "4242" {
		t.Errorf("Card.Last4 = %q, want %q", dst.Card.Last4, "4242")
	}
	if dst.Card.ExpMonth != 12 {
		t.Errorf("Card.ExpMonth = %d, want %d", dst.Card.ExpMonth, 12)
	}
	if dst.Card.ExpYear != 2027 {
		t.Errorf("Card.ExpYear = %d, want %d", dst.Card.ExpYear, 2027)
	}
	if dst.Card.Funding != "credit" {
		t.Errorf("Card.Funding = %q, want %q", dst.Card.Funding, "credit")
	}
	if dst.Card.Country != "US" {
		t.Errorf("Card.Country = %q, want %q", dst.Card.Country, "US")
	}
	if dst.BankAccount != nil {
		t.Errorf("BankAccount should be nil, got %+v", dst.BankAccount)
	}
}

func TestSyncPaymentMethod_WithBankAccount(t *testing.T) {
	src := &sgo.PaymentMethod{
		ID:   "pm_bank_456",
		Type: "us_bank_account",
		USBankAccount: &sgo.PaymentMethodUSBankAccount{
			BankName:          "Chase",
			Last4:             "6789",
			RoutingNumber:     "021000021",
			AccountHolderType: sgo.PaymentMethodUSBankAccountAccountHolderTypeIndividual,
		},
	}

	dst := &paymentmethod.PaymentMethod{}
	SyncPaymentMethod(dst, src)

	if dst.Card != nil {
		t.Errorf("Card should be nil, got %+v", dst.Card)
	}
	if dst.BankAccount == nil {
		t.Fatal("BankAccount is nil")
	}
	if dst.BankAccount.BankName != "Chase" {
		t.Errorf("BankAccount.BankName = %q, want %q", dst.BankAccount.BankName, "Chase")
	}
	if dst.BankAccount.Last4 != "6789" {
		t.Errorf("BankAccount.Last4 = %q, want %q", dst.BankAccount.Last4, "6789")
	}
	if dst.BankAccount.RoutingNumber != "021000021" {
		t.Errorf("BankAccount.RoutingNumber = %q, want %q", dst.BankAccount.RoutingNumber, "021000021")
	}
	if dst.BankAccount.AccountType != "individual" {
		t.Errorf("BankAccount.AccountType = %q, want %q", dst.BankAccount.AccountType, "individual")
	}
}

func TestSyncPaymentMethod_NoCardOrBank(t *testing.T) {
	src := &sgo.PaymentMethod{
		ID:   "pm_other",
		Type: "sepa_debit",
	}

	dst := &paymentmethod.PaymentMethod{}
	SyncPaymentMethod(dst, src)

	if dst.ProviderRef != "pm_other" {
		t.Errorf("ProviderRef = %q, want %q", dst.ProviderRef, "pm_other")
	}
	if dst.Type != "sepa_debit" {
		t.Errorf("Type = %q, want %q", dst.Type, "sepa_debit")
	}
	if dst.Card != nil {
		t.Errorf("Card should be nil")
	}
	if dst.BankAccount != nil {
		t.Errorf("BankAccount should be nil")
	}
	if dst.CustomerId != "" {
		t.Errorf("CustomerId = %q, want empty (no customer)", dst.CustomerId)
	}
}

// --- SyncRefund ---

func TestSyncRefund_WithPaymentIntent(t *testing.T) {
	src := &sgo.Refund{
		ID:            "re_abc",
		Amount:        2500,
		Currency:      "usd",
		Status:        sgo.RefundStatusSucceeded,
		Reason:        sgo.RefundReasonRequestedByCustomer,
		ReceiptNumber: "1234-5678",
		FailureReason: "",
		PaymentIntent: &sgo.PaymentIntent{ID: "pi_parent"},
	}

	dst := &refund.Refund{}
	SyncRefund(dst, src)

	if dst.ProviderRef != "re_abc" {
		t.Errorf("ProviderRef = %q, want %q", dst.ProviderRef, "re_abc")
	}
	if dst.Amount != 2500 {
		t.Errorf("Amount = %d, want %d", dst.Amount, 2500)
	}
	if dst.Currency != currency.Type("usd") {
		t.Errorf("Currency = %q, want %q", dst.Currency, "usd")
	}
	if dst.Status != refund.Succeeded {
		t.Errorf("Status = %q, want %q", dst.Status, refund.Succeeded)
	}
	if dst.Reason != "requested_by_customer" {
		t.Errorf("Reason = %q, want %q", dst.Reason, "requested_by_customer")
	}
	if dst.ReceiptNumber != "1234-5678" {
		t.Errorf("ReceiptNumber = %q, want %q", dst.ReceiptNumber, "1234-5678")
	}
	if dst.PaymentIntentId != "pi_parent" {
		t.Errorf("PaymentIntentId = %q, want %q", dst.PaymentIntentId, "pi_parent")
	}
}

func TestSyncRefund_WithoutPaymentIntent(t *testing.T) {
	src := &sgo.Refund{
		ID:            "re_nopi",
		Amount:        100,
		Currency:      "gbp",
		Status:        sgo.RefundStatusPending,
		Reason:        sgo.RefundReasonDuplicate,
		FailureReason: "expired_or_canceled_card",
	}

	dst := &refund.Refund{}
	SyncRefund(dst, src)

	if dst.PaymentIntentId != "" {
		t.Errorf("PaymentIntentId = %q, want empty", dst.PaymentIntentId)
	}
	if dst.Status != refund.Pending {
		t.Errorf("Status = %q, want %q", dst.Status, refund.Pending)
	}
	if dst.FailureReason != "expired_or_canceled_card" {
		t.Errorf("FailureReason = %q, want %q", dst.FailureReason, "expired_or_canceled_card")
	}
}

// Test all Refund status mappings via SyncRefund.
func TestSyncRefund_StatusMapping(t *testing.T) {
	cases := []struct {
		stripe sgo.RefundStatus
		want   refund.Status
	}{
		{sgo.RefundStatusPending, refund.Pending},
		{sgo.RefundStatusSucceeded, refund.Succeeded},
		{sgo.RefundStatusFailed, refund.Failed},
		{sgo.RefundStatusCanceled, refund.Canceled},
		// Unknown status falls through as-is.
		{sgo.RefundStatus("requires_action"), refund.Status("requires_action")},
	}

	for _, tc := range cases {
		t.Run(string(tc.stripe), func(t *testing.T) {
			src := &sgo.Refund{Status: tc.stripe}
			dst := &refund.Refund{}
			SyncRefund(dst, src)
			if dst.Status != tc.want {
				t.Errorf("status %q -> %q, want %q", tc.stripe, dst.Status, tc.want)
			}
		})
	}
}

// --- SyncDispute ---

func TestSyncDispute_WithEvidenceAndPaymentIntent(t *testing.T) {
	src := &sgo.Dispute{
		ID:       "dp_abc",
		Amount:   7500,
		Currency: "usd",
		Status:   sgo.DisputeStatusNeedsResponse,
		Reason:   sgo.DisputeReasonFraudulent,
		EvidenceDetails: &sgo.DisputeEvidenceDetails{
			DueBy: 1700100000,
		},
		PaymentIntent: &sgo.PaymentIntent{ID: "pi_disputed"},
	}

	dst := &dispute.Dispute{}
	SyncDispute(dst, src)

	if dst.ProviderRef != "dp_abc" {
		t.Errorf("ProviderRef = %q, want %q", dst.ProviderRef, "dp_abc")
	}
	if dst.Amount != 7500 {
		t.Errorf("Amount = %d, want %d", dst.Amount, 7500)
	}
	if dst.Currency != currency.Type("usd") {
		t.Errorf("Currency = %q, want %q", dst.Currency, "usd")
	}
	if dst.Status != dispute.NeedsResponse {
		t.Errorf("Status = %q, want %q", dst.Status, dispute.NeedsResponse)
	}
	if dst.Reason != "fraudulent" {
		t.Errorf("Reason = %q, want %q", dst.Reason, "fraudulent")
	}
	if dst.PaymentIntentId != "pi_disputed" {
		t.Errorf("PaymentIntentId = %q, want %q", dst.PaymentIntentId, "pi_disputed")
	}
	wantDue := time.Unix(1700100000, 0)
	if !dst.EvidenceDueBy.Equal(wantDue) {
		t.Errorf("EvidenceDueBy = %v, want %v", dst.EvidenceDueBy, wantDue)
	}
}

func TestSyncDispute_NilEvidenceDetailsAndPaymentIntent(t *testing.T) {
	src := &sgo.Dispute{
		ID:       "dp_nopi",
		Amount:   300,
		Currency: "eur",
		Status:   sgo.DisputeStatusWon,
		Reason:   sgo.DisputeReasonGeneral,
	}

	dst := &dispute.Dispute{}
	SyncDispute(dst, src)

	if dst.PaymentIntentId != "" {
		t.Errorf("PaymentIntentId = %q, want empty", dst.PaymentIntentId)
	}
	if !dst.EvidenceDueBy.IsZero() {
		t.Errorf("EvidenceDueBy = %v, want zero", dst.EvidenceDueBy)
	}
	if dst.Status != dispute.Won {
		t.Errorf("Status = %q, want %q", dst.Status, dispute.Won)
	}
}

func TestSyncDispute_EvidenceDetailsZeroDueBy(t *testing.T) {
	src := &sgo.Dispute{
		ID:     "dp_zerodue",
		Status: sgo.DisputeStatusUnderReview,
		EvidenceDetails: &sgo.DisputeEvidenceDetails{
			DueBy: 0, // zero means no deadline
		},
	}

	dst := &dispute.Dispute{}
	SyncDispute(dst, src)

	if !dst.EvidenceDueBy.IsZero() {
		t.Errorf("EvidenceDueBy = %v, want zero (DueBy=0 should skip)", dst.EvidenceDueBy)
	}
}

// Test all Dispute status mappings via SyncDispute.
func TestSyncDispute_StatusMapping(t *testing.T) {
	cases := []struct {
		stripe sgo.DisputeStatus
		want   dispute.Status
	}{
		{sgo.DisputeStatusWarningNeedsResponse, dispute.WarningNeedsResponse},
		{sgo.DisputeStatusNeedsResponse, dispute.NeedsResponse},
		{sgo.DisputeStatusUnderReview, dispute.UnderReview},
		{sgo.DisputeStatusWon, dispute.Won},
		{sgo.DisputeStatusLost, dispute.Lost},
		{sgo.DisputeStatusWarningUnderReview, dispute.WarningUnderReview},
		// Unknown status falls through as-is.
		{sgo.DisputeStatus("prevented"), dispute.Status("prevented")},
	}

	for _, tc := range cases {
		t.Run(string(tc.stripe), func(t *testing.T) {
			src := &sgo.Dispute{Status: tc.stripe}
			dst := &dispute.Dispute{}
			SyncDispute(dst, src)
			if dst.Status != tc.want {
				t.Errorf("status %q -> %q, want %q", tc.stripe, dst.Status, tc.want)
			}
		})
	}
}

// --- Additional edge-case tests ---

// SyncPaymentIntent: zero values and empty strings.
func TestSyncPaymentIntent_ZeroValues(t *testing.T) {
	src := &sgo.PaymentIntent{
		ID:       "",
		Amount:   0,
		Currency: "",
		Status:   "",
		// All pointer fields nil, all scalars zero.
	}

	dst := &paymentintent.PaymentIntent{}
	SyncPaymentIntent(dst, src)

	if dst.ProviderRef != "" {
		t.Errorf("ProviderRef = %q, want empty", dst.ProviderRef)
	}
	if dst.Amount != 0 {
		t.Errorf("Amount = %d, want 0", dst.Amount)
	}
	if dst.Currency != currency.Type("") {
		t.Errorf("Currency = %q, want empty", dst.Currency)
	}
	if dst.Status != paymentintent.Status("") {
		t.Errorf("Status = %q, want empty", dst.Status)
	}
	if dst.CaptureMethod != "" {
		t.Errorf("CaptureMethod = %q, want empty", dst.CaptureMethod)
	}
	if dst.ConfirmationMethod != "" {
		t.Errorf("ConfirmationMethod = %q, want empty", dst.ConfirmationMethod)
	}
	if dst.Description != "" {
		t.Errorf("Description = %q, want empty", dst.Description)
	}
	if dst.ReceiptEmail != "" {
		t.Errorf("ReceiptEmail = %q, want empty", dst.ReceiptEmail)
	}
	if dst.ClientSecret != "" {
		t.Errorf("ClientSecret = %q, want empty", dst.ClientSecret)
	}
	if dst.CancellationReason != "" {
		t.Errorf("CancellationReason = %q, want empty", dst.CancellationReason)
	}
	if dst.LastError != "" {
		t.Errorf("LastError = %q, want empty", dst.LastError)
	}
	if !dst.CanceledAt.IsZero() {
		t.Errorf("CanceledAt = %v, want zero", dst.CanceledAt)
	}
	if dst.CustomerId != "" {
		t.Errorf("CustomerId = %q, want empty", dst.CustomerId)
	}
	if dst.PaymentMethodId != "" {
		t.Errorf("PaymentMethodId = %q, want empty", dst.PaymentMethodId)
	}
	if dst.ProviderType != "stripe" {
		t.Errorf("ProviderType = %q, want %q", dst.ProviderType, "stripe")
	}
}

// SyncPaymentIntent: LastPaymentError with empty Msg.
func TestSyncPaymentIntent_LastErrorEmptyMsg(t *testing.T) {
	src := &sgo.PaymentIntent{
		ID:               "pi_errempty",
		Status:           sgo.PaymentIntentStatusRequiresAction,
		LastPaymentError: &sgo.Error{Msg: ""},
	}

	dst := &paymentintent.PaymentIntent{}
	SyncPaymentIntent(dst, src)

	if dst.LastError != "" {
		t.Errorf("LastError = %q, want empty (error Msg was empty)", dst.LastError)
	}
}

// SyncPaymentIntent: overwrite pre-existing dst fields.
func TestSyncPaymentIntent_OverwriteExisting(t *testing.T) {
	dst := &paymentintent.PaymentIntent{
		ProviderRef:   "old_ref",
		Amount:        9999,
		CustomerId:    "old_cus",
		Description:   "old desc",
		ClientSecret:  "old_secret",
	}

	src := &sgo.PaymentIntent{
		ID:          "pi_new",
		Amount:      42,
		Currency:    "jpy",
		Status:      sgo.PaymentIntentStatusProcessing,
		Description: "new desc",
	}

	SyncPaymentIntent(dst, src)

	if dst.ProviderRef != "pi_new" {
		t.Errorf("ProviderRef = %q, want %q", dst.ProviderRef, "pi_new")
	}
	if dst.Amount != 42 {
		t.Errorf("Amount = %d, want 42", dst.Amount)
	}
	// nil Customer does NOT clear dst.CustomerId -- it retains old value.
	if dst.CustomerId != "old_cus" {
		t.Errorf("CustomerId = %q, want %q (nil Customer retains old)", dst.CustomerId, "old_cus")
	}
	if dst.Description != "new desc" {
		t.Errorf("Description = %q, want %q", dst.Description, "new desc")
	}
}

// SyncSetupIntent: zero values.
func TestSyncSetupIntent_ZeroValues(t *testing.T) {
	src := &sgo.SetupIntent{
		ID:     "",
		Status: "",
	}

	dst := &setupintent.SetupIntent{}
	SyncSetupIntent(dst, src)

	if dst.ProviderRef != "" {
		t.Errorf("ProviderRef = %q, want empty", dst.ProviderRef)
	}
	if dst.ProviderType != "stripe" {
		t.Errorf("ProviderType = %q, want %q", dst.ProviderType, "stripe")
	}
	if dst.Status != setupintent.Status("") {
		t.Errorf("Status = %q, want empty", dst.Status)
	}
	if dst.Usage != "" {
		t.Errorf("Usage = %q, want empty", dst.Usage)
	}
	if dst.ClientSecret != "" {
		t.Errorf("ClientSecret = %q, want empty", dst.ClientSecret)
	}
	if dst.CancellationReason != "" {
		t.Errorf("CancellationReason = %q, want empty", dst.CancellationReason)
	}
}

// SyncSetupIntent: LastSetupError with empty Msg.
func TestSyncSetupIntent_LastErrorEmptyMsg(t *testing.T) {
	src := &sgo.SetupIntent{
		ID:             "seti_errempty",
		Status:         sgo.SetupIntentStatusRequiresAction,
		LastSetupError: &sgo.Error{Msg: ""},
	}

	dst := &setupintent.SetupIntent{}
	SyncSetupIntent(dst, src)

	if dst.LastError != "" {
		t.Errorf("LastError = %q, want empty (error Msg was empty)", dst.LastError)
	}
}

// SyncPaymentMethod: nil Card AND nil USBankAccount AND nil Customer.
func TestSyncPaymentMethod_AllNil(t *testing.T) {
	src := &sgo.PaymentMethod{
		ID:   "pm_nil",
		Type: "unknown_type",
		// Customer, Card, USBankAccount all nil
	}

	dst := &paymentmethod.PaymentMethod{}
	SyncPaymentMethod(dst, src)

	if dst.ProviderRef != "pm_nil" {
		t.Errorf("ProviderRef = %q, want %q", dst.ProviderRef, "pm_nil")
	}
	if dst.Type != "unknown_type" {
		t.Errorf("Type = %q, want %q", dst.Type, "unknown_type")
	}
	if dst.CustomerId != "" {
		t.Errorf("CustomerId = %q, want empty", dst.CustomerId)
	}
	if dst.Card != nil {
		t.Errorf("Card = %+v, want nil", dst.Card)
	}
	if dst.BankAccount != nil {
		t.Errorf("BankAccount = %+v, want nil", dst.BankAccount)
	}
	if dst.ProviderType != "stripe" {
		t.Errorf("ProviderType = %q, want %q", dst.ProviderType, "stripe")
	}
}

// SyncPaymentMethod: empty strings and zero values in Card.
func TestSyncPaymentMethod_CardZeroValues(t *testing.T) {
	src := &sgo.PaymentMethod{
		ID:   "pm_zerocard",
		Type: "card",
		Card: &sgo.PaymentMethodCard{
			Brand:    "",
			Last4:    "",
			ExpMonth: 0,
			ExpYear:  0,
			Funding:  "",
			Country:  "",
		},
	}

	dst := &paymentmethod.PaymentMethod{}
	SyncPaymentMethod(dst, src)

	if dst.Card == nil {
		t.Fatal("Card is nil, expected non-nil with zero values")
	}
	if dst.Card.Brand != "" {
		t.Errorf("Card.Brand = %q, want empty", dst.Card.Brand)
	}
	if dst.Card.Last4 != "" {
		t.Errorf("Card.Last4 = %q, want empty", dst.Card.Last4)
	}
	if dst.Card.ExpMonth != 0 {
		t.Errorf("Card.ExpMonth = %d, want 0", dst.Card.ExpMonth)
	}
	if dst.Card.ExpYear != 0 {
		t.Errorf("Card.ExpYear = %d, want 0", dst.Card.ExpYear)
	}
	if dst.Card.Funding != "" {
		t.Errorf("Card.Funding = %q, want empty", dst.Card.Funding)
	}
	if dst.Card.Country != "" {
		t.Errorf("Card.Country = %q, want empty", dst.Card.Country)
	}
}

// SyncPaymentMethod: both Card and USBankAccount set (edge case).
func TestSyncPaymentMethod_BothCardAndBank(t *testing.T) {
	src := &sgo.PaymentMethod{
		ID:   "pm_both",
		Type: "card",
		Card: &sgo.PaymentMethodCard{
			Brand: "mastercard",
			Last4: "1234",
		},
		USBankAccount: &sgo.PaymentMethodUSBankAccount{
			BankName: "Wells Fargo",
			Last4:    "5678",
		},
	}

	dst := &paymentmethod.PaymentMethod{}
	SyncPaymentMethod(dst, src)

	// Both should be populated -- sync copies whatever Stripe sends.
	if dst.Card == nil {
		t.Fatal("Card is nil")
	}
	if dst.Card.Brand != "mastercard" {
		t.Errorf("Card.Brand = %q, want %q", dst.Card.Brand, "mastercard")
	}
	if dst.BankAccount == nil {
		t.Fatal("BankAccount is nil")
	}
	if dst.BankAccount.BankName != "Wells Fargo" {
		t.Errorf("BankAccount.BankName = %q, want %q", dst.BankAccount.BankName, "Wells Fargo")
	}
}

// SyncRefund: zero amount, empty currency, no reason.
func TestSyncRefund_ZeroValues(t *testing.T) {
	src := &sgo.Refund{
		ID:       "re_zero",
		Amount:   0,
		Currency: "",
		Status:   sgo.RefundStatusPending,
		Reason:   "",
	}

	dst := &refund.Refund{}
	SyncRefund(dst, src)

	if dst.ProviderRef != "re_zero" {
		t.Errorf("ProviderRef = %q, want %q", dst.ProviderRef, "re_zero")
	}
	if dst.Amount != 0 {
		t.Errorf("Amount = %d, want 0", dst.Amount)
	}
	if dst.Currency != currency.Type("") {
		t.Errorf("Currency = %q, want empty", dst.Currency)
	}
	if dst.Reason != "" {
		t.Errorf("Reason = %q, want empty", dst.Reason)
	}
	if dst.ReceiptNumber != "" {
		t.Errorf("ReceiptNumber = %q, want empty", dst.ReceiptNumber)
	}
	if dst.FailureReason != "" {
		t.Errorf("FailureReason = %q, want empty", dst.FailureReason)
	}
	if dst.PaymentIntentId != "" {
		t.Errorf("PaymentIntentId = %q, want empty", dst.PaymentIntentId)
	}
}

// SyncRefund: FailureReason set alongside successful status (edge case from Stripe).
func TestSyncRefund_SucceededWithFailureReason(t *testing.T) {
	src := &sgo.Refund{
		ID:            "re_weirdcombo",
		Amount:        100,
		Currency:      "usd",
		Status:        sgo.RefundStatusSucceeded,
		FailureReason: "stale_data",
	}

	dst := &refund.Refund{}
	SyncRefund(dst, src)

	if dst.Status != refund.Succeeded {
		t.Errorf("Status = %q, want %q", dst.Status, refund.Succeeded)
	}
	if dst.FailureReason != "stale_data" {
		t.Errorf("FailureReason = %q, want %q", dst.FailureReason, "stale_data")
	}
}

// SyncDispute: zero amount, empty currency, empty reason.
func TestSyncDispute_ZeroValues(t *testing.T) {
	src := &sgo.Dispute{
		ID:       "dp_zero",
		Amount:   0,
		Currency: "",
		Status:   sgo.DisputeStatusNeedsResponse,
		Reason:   "",
	}

	dst := &dispute.Dispute{}
	SyncDispute(dst, src)

	if dst.ProviderRef != "dp_zero" {
		t.Errorf("ProviderRef = %q, want %q", dst.ProviderRef, "dp_zero")
	}
	if dst.Amount != 0 {
		t.Errorf("Amount = %d, want 0", dst.Amount)
	}
	if dst.Currency != currency.Type("") {
		t.Errorf("Currency = %q, want empty", dst.Currency)
	}
	if dst.Reason != "" {
		t.Errorf("Reason = %q, want empty", dst.Reason)
	}
	if dst.PaymentIntentId != "" {
		t.Errorf("PaymentIntentId = %q, want empty", dst.PaymentIntentId)
	}
	if !dst.EvidenceDueBy.IsZero() {
		t.Errorf("EvidenceDueBy = %v, want zero", dst.EvidenceDueBy)
	}
}

// SyncDispute: EvidenceDetails non-nil but DueBy is zero.
// (already tested above but with explicit sub-test name for clarity)
func TestSyncDispute_EvidenceDetailsPresent_DueByZero(t *testing.T) {
	src := &sgo.Dispute{
		ID:     "dp_evidence_zero",
		Status: sgo.DisputeStatusWarningNeedsResponse,
		EvidenceDetails: &sgo.DisputeEvidenceDetails{
			DueBy: 0,
		},
	}

	dst := &dispute.Dispute{}
	SyncDispute(dst, src)

	if !dst.EvidenceDueBy.IsZero() {
		t.Errorf("EvidenceDueBy = %v, want zero (DueBy=0 should skip)", dst.EvidenceDueBy)
	}
	if dst.Status != dispute.WarningNeedsResponse {
		t.Errorf("Status = %q, want %q", dst.Status, dispute.WarningNeedsResponse)
	}
}

// SyncDispute: WarningClosed (unknown to switch) falls through.
func TestSyncDispute_UnknownStatusFallthrough(t *testing.T) {
	src := &sgo.Dispute{
		ID:     "dp_unknown",
		Status: sgo.DisputeStatus("charge_refunded"),
		Reason: sgo.DisputeReasonProductNotReceived,
	}

	dst := &dispute.Dispute{}
	SyncDispute(dst, src)

	if dst.Status != dispute.Status("charge_refunded") {
		t.Errorf("Status = %q, want %q", dst.Status, "charge_refunded")
	}
	if dst.Reason != "product_not_received" {
		t.Errorf("Reason = %q, want %q", dst.Reason, "product_not_received")
	}
}

// SyncDispute: all dispute reasons exercise the Reason field mapping.
func TestSyncDispute_Reasons(t *testing.T) {
	reasons := []sgo.DisputeReason{
		sgo.DisputeReasonFraudulent,
		sgo.DisputeReasonGeneral,
		sgo.DisputeReasonProductNotReceived,
		sgo.DisputeReasonDuplicate,
		sgo.DisputeReasonSubscriptionCanceled,
		sgo.DisputeReasonUnrecognized,
	}

	for _, r := range reasons {
		t.Run(string(r), func(t *testing.T) {
			src := &sgo.Dispute{
				ID:     "dp_reason",
				Status: sgo.DisputeStatusNeedsResponse,
				Reason: r,
			}
			dst := &dispute.Dispute{}
			SyncDispute(dst, src)
			if dst.Reason != string(r) {
				t.Errorf("Reason = %q, want %q", dst.Reason, string(r))
			}
		})
	}
}

// MapWebhookEventType: additional pass-through cases.
func TestMapWebhookEventType_MorePassthrough(t *testing.T) {
	passthroughCases := []string{
		"checkout.session.completed",
		"customer.created",
		"customer.deleted",
		"transfer.created",
		"payout.paid",
		"account.updated",
	}

	for _, ev := range passthroughCases {
		t.Run(ev, func(t *testing.T) {
			got := MapWebhookEventType(ev)
			if got != ev {
				t.Errorf("MapWebhookEventType(%q) = %q, want pass-through %q", ev, got, ev)
			}
		})
	}
}
