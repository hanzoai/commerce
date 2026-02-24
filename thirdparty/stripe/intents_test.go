package stripe

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	sgo "github.com/stripe/stripe-go/v84"
	"github.com/stripe/stripe-go/v84/client"

	"github.com/hanzoai/commerce/models/payment"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
)

// makeTestProcessor creates a StripeProcessor backed by a mock Stripe API server.
// Returns the processor and the server (caller must defer server.Close()).
func makeTestProcessor(t *testing.T, handler http.Handler) (*StripeProcessor, *httptest.Server) {
	t.Helper()

	server := httptest.NewServer(handler)

	// Create a Stripe backend pointing at our test server
	backend := sgo.GetBackendWithConfig(sgo.APIBackend, &sgo.BackendConfig{
		URL: sgo.String(server.URL),
	})

	api := &client.API{}
	api.Init("sk_test_fake", &sgo.Backends{
		API:     backend,
		Uploads: backend,
	})

	sp := &StripeProcessor{
		BaseProcessor: processor.NewBaseProcessor(processor.Stripe, StripeSupportedCurrencies()),
		accessToken:   "sk_test_fake",
		webhookSecret: "whsec_test",
		client:        api,
	}
	sp.SetConfigured(true)

	return sp, server
}

// stripePaymentIntentResponse builds a mock Stripe PaymentIntent JSON response.
func stripePaymentIntentResponse(id string, status string, amount int64) []byte {
	resp := map[string]interface{}{
		"id":                  id,
		"object":              "payment_intent",
		"amount":              amount,
		"currency":            "usd",
		"status":              status,
		"client_secret":       id + "_secret_test",
		"capture_method":      "automatic",
		"confirmation_method": "automatic",
		"amount_capturable":   0,
		"amount_received":     amount,
	}
	b, _ := json.Marshal(resp)
	return b
}

// ---------------------------------------------------------------------------
// NewProcessor tests
// ---------------------------------------------------------------------------

func TestNewProcessor_WithToken(t *testing.T) {
	sp := NewProcessor("sk_test_123", "whsec_abc")
	if sp == nil {
		t.Fatal("NewProcessor returned nil")
	}
	if sp.accessToken != "sk_test_123" {
		t.Fatalf("accessToken = %q, want sk_test_123", sp.accessToken)
	}
	if sp.webhookSecret != "whsec_abc" {
		t.Fatalf("webhookSecret = %q, want whsec_abc", sp.webhookSecret)
	}
	if sp.client == nil {
		t.Fatal("client should be initialized when token is set")
	}
	if !sp.IsAvailable(context.Background()) {
		t.Fatal("should be available when token is set")
	}
	if sp.Type() != processor.Stripe {
		t.Fatalf("Type = %q, want stripe", sp.Type())
	}
}

func TestNewProcessor_EmptyToken(t *testing.T) {
	sp := NewProcessor("", "whsec_abc")
	if sp == nil {
		t.Fatal("NewProcessor returned nil")
	}
	if sp.client != nil {
		t.Fatal("client should be nil when token is empty")
	}
	if sp.IsAvailable(context.Background()) {
		t.Fatal("should not be available when token is empty")
	}
}

func TestNewSubscriptionProcessor(t *testing.T) {
	sp := NewSubscriptionProcessor("sk_test_sub", "whsec_sub")
	if sp == nil {
		t.Fatal("NewSubscriptionProcessor returned nil")
	}
	if sp.accessToken != "sk_test_sub" {
		t.Fatalf("accessToken mismatch")
	}
}

// ---------------------------------------------------------------------------
// Charge validation
// ---------------------------------------------------------------------------

func TestStripeProcessor_Charge_InvalidRequest_ZeroAmount(t *testing.T) {
	sp := NewProcessor("sk_test_fake", "")
	_, err := sp.Charge(context.Background(), processor.PaymentRequest{
		Amount:   0,
		Currency: "usd",
	})
	if err == nil {
		t.Fatal("expected error for zero amount")
	}
}

func TestStripeProcessor_Charge_InvalidRequest_NoCurrency(t *testing.T) {
	sp := NewProcessor("sk_test_fake", "")
	_, err := sp.Charge(context.Background(), processor.PaymentRequest{
		Amount:   1000,
		Currency: "",
	})
	if err == nil {
		t.Fatal("expected error for empty currency")
	}
}

func TestStripeProcessor_Authorize_InvalidRequest(t *testing.T) {
	sp := NewProcessor("sk_test_fake", "")
	_, err := sp.Authorize(context.Background(), processor.PaymentRequest{
		Amount:   0,
		Currency: "usd",
	})
	if err == nil {
		t.Fatal("expected error for zero amount")
	}
}

// ---------------------------------------------------------------------------
// ChargeViaIntent validation
// ---------------------------------------------------------------------------

func TestChargeViaIntent_InvalidRequest_ZeroAmount(t *testing.T) {
	sp := NewProcessor("sk_test_fake", "")
	_, err := sp.ChargeViaIntent(context.Background(), processor.PaymentRequest{
		Amount:   0,
		Currency: "usd",
	})
	if err == nil {
		t.Fatal("expected error for zero amount")
	}
}

func TestChargeViaIntent_InvalidRequest_NoCurrency(t *testing.T) {
	sp := NewProcessor("sk_test_fake", "")
	_, err := sp.ChargeViaIntent(context.Background(), processor.PaymentRequest{
		Amount:   1000,
		Currency: "",
	})
	if err == nil {
		t.Fatal("expected error for empty currency")
	}
}

// ---------------------------------------------------------------------------
// AuthorizeViaIntent validation
// ---------------------------------------------------------------------------

func TestAuthorizeViaIntent_InvalidRequest_ZeroAmount(t *testing.T) {
	sp := NewProcessor("sk_test_fake", "")
	_, err := sp.AuthorizeViaIntent(context.Background(), processor.PaymentRequest{
		Amount:   0,
		Currency: "usd",
	})
	if err == nil {
		t.Fatal("expected error for zero amount")
	}
}

func TestAuthorizeViaIntent_InvalidRequest_NoCurrency(t *testing.T) {
	sp := NewProcessor("sk_test_fake", "")
	_, err := sp.AuthorizeViaIntent(context.Background(), processor.PaymentRequest{
		Amount:   500,
		Currency: "",
	})
	if err == nil {
		t.Fatal("expected error for empty currency")
	}
}

// ---------------------------------------------------------------------------
// ChargeViaIntent with mock server
// ---------------------------------------------------------------------------

func TestChargeViaIntent_Success(t *testing.T) {
	sp, server := makeTestProcessor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"id":              "pi_charge_test",
			"object":          "payment_intent",
			"amount":          5000,
			"currency":        "usd",
			"status":          "succeeded",
			"client_secret":   "pi_charge_test_secret",
			"amount_received": 5000,
			"latest_charge": map[string]interface{}{
				"id":     "ch_123",
				"object": "charge",
				"balance_transaction": map[string]interface{}{
					"id":     "txn_123",
					"object": "balance_transaction",
					"fee":    150,
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	result, err := sp.ChargeViaIntent(context.Background(), processor.PaymentRequest{
		Amount:      5000,
		Currency:    "usd",
		Token:       "pm_test",
		CustomerID:  "cus_test",
		Description: "Test charge",
		OrderID:     "ord_123",
		Metadata: map[string]interface{}{
			"key1": "val1",
			"key2": 42, // non-string should be skipped
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatal("expected success")
	}
	if result.TransactionID != "pi_charge_test" {
		t.Fatalf("TransactionID = %q, want pi_charge_test", result.TransactionID)
	}
	if result.ProcessorRef != "pi_charge_test" {
		t.Fatalf("ProcessorRef = %q, want pi_charge_test", result.ProcessorRef)
	}
	if result.Fee != currency.Cents(150) {
		t.Fatalf("Fee = %d, want 150", result.Fee)
	}
	if result.Status != "succeeded" {
		t.Fatalf("Status = %q, want succeeded", result.Status)
	}
	if result.Metadata == nil {
		t.Fatal("Metadata should not be nil")
	}
	if result.Metadata["client_secret"] != "pi_charge_test_secret" {
		t.Fatalf("Metadata[client_secret] = %v", result.Metadata["client_secret"])
	}
}

func TestChargeViaIntent_StripeError(t *testing.T) {
	sp, server := makeTestProcessor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(402)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"type":    "card_error",
				"message": "Your card was declined.",
				"code":    "card_declined",
			},
		})
	}))
	defer server.Close()

	result, err := sp.ChargeViaIntent(context.Background(), processor.PaymentRequest{
		Amount:   1000,
		Currency: "usd",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if result == nil {
		t.Fatal("result should not be nil even on error")
	}
	if result.Success {
		t.Fatal("should not be successful")
	}
	if result.ErrorMessage == "" {
		t.Fatal("ErrorMessage should be set")
	}
}

func TestChargeViaIntent_NoFee(t *testing.T) {
	sp, server := makeTestProcessor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":              "pi_nofee",
			"object":          "payment_intent",
			"amount":          1000,
			"currency":        "usd",
			"status":          "succeeded",
			"client_secret":   "secret",
			"amount_received": 1000,
		})
	}))
	defer server.Close()

	result, err := sp.ChargeViaIntent(context.Background(), processor.PaymentRequest{
		Amount:   1000,
		Currency: "usd",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Fee != 0 {
		t.Fatalf("Fee = %d, want 0", result.Fee)
	}
}

// ---------------------------------------------------------------------------
// AuthorizeViaIntent with mock server
// ---------------------------------------------------------------------------

func TestAuthorizeViaIntent_Success(t *testing.T) {
	sp, server := makeTestProcessor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":                "pi_auth_test",
			"object":            "payment_intent",
			"amount":            3000,
			"currency":          "usd",
			"status":            "requires_capture",
			"client_secret":     "pi_auth_test_secret",
			"amount_capturable": 3000,
		})
	}))
	defer server.Close()

	result, err := sp.AuthorizeViaIntent(context.Background(), processor.PaymentRequest{
		Amount:      3000,
		Currency:    "usd",
		Token:       "pm_auth",
		CustomerID:  "cus_auth",
		Description: "Auth test",
		OrderID:     "ord_auth",
		Metadata: map[string]interface{}{
			"key": "val",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatal("expected success")
	}
	if result.TransactionID != "pi_auth_test" {
		t.Fatalf("TransactionID = %q", result.TransactionID)
	}
	if result.Status != "requires_capture" {
		t.Fatalf("Status = %q, want requires_capture", result.Status)
	}
}

func TestAuthorizeViaIntent_StripeError(t *testing.T) {
	sp, server := makeTestProcessor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(402)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"type":    "card_error",
				"message": "Insufficient funds",
			},
		})
	}))
	defer server.Close()

	result, err := sp.AuthorizeViaIntent(context.Background(), processor.PaymentRequest{
		Amount:   5000,
		Currency: "usd",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if result == nil {
		t.Fatal("result should not be nil")
	}
	if result.Success {
		t.Fatal("should not succeed")
	}
}

// ---------------------------------------------------------------------------
// CaptureIntent with mock server
// ---------------------------------------------------------------------------

func TestCaptureIntent_Success(t *testing.T) {
	sp, server := makeTestProcessor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":              "pi_captured",
			"object":          "payment_intent",
			"amount":          2000,
			"currency":        "usd",
			"status":          "succeeded",
			"amount_received": 2000,
			"latest_charge": map[string]interface{}{
				"id":     "ch_cap",
				"object": "charge",
				"balance_transaction": map[string]interface{}{
					"id":     "txn_cap",
					"object": "balance_transaction",
					"fee":    58,
				},
			},
		})
	}))
	defer server.Close()

	result, err := sp.CaptureIntent(context.Background(), "pi_toCapture", currency.Cents(2000))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatal("expected success")
	}
	if result.Status != "captured" {
		t.Fatalf("Status = %q, want captured", result.Status)
	}
	if result.Fee != currency.Cents(58) {
		t.Fatalf("Fee = %d, want 58", result.Fee)
	}
}

func TestCaptureIntent_ZeroAmount(t *testing.T) {
	sp, server := makeTestProcessor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     "pi_cap_zero",
			"object": "payment_intent",
			"status": "succeeded",
		})
	}))
	defer server.Close()

	result, err := sp.CaptureIntent(context.Background(), "pi_x", currency.Cents(0))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatal("expected success")
	}
}

func TestCaptureIntent_StripeError(t *testing.T) {
	sp, server := makeTestProcessor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"type":    "invalid_request_error",
				"message": "This PaymentIntent could not be captured",
			},
		})
	}))
	defer server.Close()

	result, err := sp.CaptureIntent(context.Background(), "pi_bad", currency.Cents(1000))
	if err == nil {
		t.Fatal("expected error")
	}
	if result == nil {
		t.Fatal("result should not be nil")
	}
	if result.Success {
		t.Fatal("should not succeed")
	}
}

// ---------------------------------------------------------------------------
// CreateSetupIntent with mock server
// ---------------------------------------------------------------------------

func TestCreateSetupIntent_Success(t *testing.T) {
	sp, server := makeTestProcessor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":            "seti_test",
			"object":        "setup_intent",
			"client_secret": "seti_test_secret",
			"status":        "requires_payment_method",
		})
	}))
	defer server.Close()

	id, secret, err := sp.CreateSetupIntent(context.Background(), "cus_setup", "off_session")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "seti_test" {
		t.Fatalf("id = %q, want seti_test", id)
	}
	if secret != "seti_test_secret" {
		t.Fatalf("secret = %q", secret)
	}
}

func TestCreateSetupIntent_EmptyParams(t *testing.T) {
	sp, server := makeTestProcessor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":            "seti_empty",
			"object":        "setup_intent",
			"client_secret": "seti_empty_secret",
		})
	}))
	defer server.Close()

	id, _, err := sp.CreateSetupIntent(context.Background(), "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "seti_empty" {
		t.Fatalf("id = %q", id)
	}
}

func TestCreateSetupIntent_StripeError(t *testing.T) {
	sp, server := makeTestProcessor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"type":    "invalid_request_error",
				"message": "Invalid customer",
			},
		})
	}))
	defer server.Close()

	_, _, err := sp.CreateSetupIntent(context.Background(), "bad_cus", "")
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---------------------------------------------------------------------------
// ConfirmSetupIntent with mock server
// ---------------------------------------------------------------------------

func TestConfirmSetupIntent_Success(t *testing.T) {
	sp, server := makeTestProcessor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     "seti_confirmed",
			"object": "setup_intent",
			"status": "succeeded",
		})
	}))
	defer server.Close()

	err := sp.ConfirmSetupIntent(context.Background(), "seti_123", "pm_card_visa")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConfirmSetupIntent_StripeError(t *testing.T) {
	sp, server := makeTestProcessor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"type":    "invalid_request_error",
				"message": "Setup intent not found",
			},
		})
	}))
	defer server.Close()

	err := sp.ConfirmSetupIntent(context.Background(), "seti_bad", "pm_x")
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---------------------------------------------------------------------------
// AttachPaymentMethod with mock server
// ---------------------------------------------------------------------------

func TestAttachPaymentMethod_Success(t *testing.T) {
	sp, server := makeTestProcessor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     "pm_attached",
			"object": "payment_method",
			"type":   "card",
		})
	}))
	defer server.Close()

	err := sp.AttachPaymentMethod(context.Background(), "pm_123", "cus_456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAttachPaymentMethod_StripeError(t *testing.T) {
	sp, server := makeTestProcessor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{"message": "already attached"},
		})
	}))
	defer server.Close()

	err := sp.AttachPaymentMethod(context.Background(), "pm_x", "cus_x")
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---------------------------------------------------------------------------
// DetachPaymentMethod with mock server
// ---------------------------------------------------------------------------

func TestDetachPaymentMethod_Success(t *testing.T) {
	sp, server := makeTestProcessor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     "pm_detached",
			"object": "payment_method",
		})
	}))
	defer server.Close()

	err := sp.DetachPaymentMethod(context.Background(), "pm_456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDetachPaymentMethod_StripeError(t *testing.T) {
	sp, server := makeTestProcessor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{"message": "not found"},
		})
	}))
	defer server.Close()

	err := sp.DetachPaymentMethod(context.Background(), "pm_bad")
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---------------------------------------------------------------------------
// CreateCustomer with mock server
// ---------------------------------------------------------------------------

func TestCreateCustomer_Success(t *testing.T) {
	sp, server := makeTestProcessor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     "cus_new",
			"object": "customer",
			"email":  "test@example.com",
		})
	}))
	defer server.Close()

	id, err := sp.CreateCustomer(context.Background(), "test@example.com", "Test User", map[string]interface{}{
		"org": "hanzo",
		"num": 42, // non-string skipped
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "cus_new" {
		t.Fatalf("id = %q, want cus_new", id)
	}
}

func TestCreateCustomer_EmptyParams(t *testing.T) {
	sp, server := makeTestProcessor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     "cus_empty",
			"object": "customer",
		})
	}))
	defer server.Close()

	id, err := sp.CreateCustomer(context.Background(), "", "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "cus_empty" {
		t.Fatalf("id = %q", id)
	}
}

func TestCreateCustomer_StripeError(t *testing.T) {
	sp, server := makeTestProcessor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{"message": "invalid email"},
		})
	}))
	defer server.Close()

	_, err := sp.CreateCustomer(context.Background(), "bad", "", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---------------------------------------------------------------------------
// UpdateCustomer with mock server
// ---------------------------------------------------------------------------

func TestUpdateCustomer_Success(t *testing.T) {
	sp, server := makeTestProcessor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     "cus_upd",
			"object": "customer",
		})
	}))
	defer server.Close()

	err := sp.UpdateCustomer(context.Background(), "cus_upd", map[string]interface{}{
		"email":       "new@example.com",
		"description": "Updated desc",
		"name":        "New Name",
		"custom_key":  "custom_val",
		"non_string":  123, // skipped
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdateCustomer_StripeError(t *testing.T) {
	sp, server := makeTestProcessor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{"message": "not found"},
		})
	}))
	defer server.Close()

	err := sp.UpdateCustomer(context.Background(), "cus_bad", map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---------------------------------------------------------------------------
// DeleteCustomer with mock server
// ---------------------------------------------------------------------------

func TestDeleteCustomer_Success(t *testing.T) {
	sp, server := makeTestProcessor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":      "cus_del",
			"object":  "customer",
			"deleted": true,
		})
	}))
	defer server.Close()

	err := sp.DeleteCustomer(context.Background(), "cus_del")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteCustomer_StripeError(t *testing.T) {
	sp, server := makeTestProcessor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{"message": "No such customer"},
		})
	}))
	defer server.Close()

	err := sp.DeleteCustomer(context.Background(), "cus_nonexist")
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---------------------------------------------------------------------------
// Processor.Charge with mock server
// ---------------------------------------------------------------------------

func TestStripeProcessor_Charge_Success(t *testing.T) {
	sp, server := makeTestProcessor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":       "ch_test",
			"object":   "charge",
			"amount":   2000,
			"currency": "usd",
			"status":   "succeeded",
			"captured": true,
			"balance_transaction": map[string]interface{}{
				"id":     "txn_test",
				"object": "balance_transaction",
				"fee":    58,
			},
			"receipt_url": "https://receipt.stripe.com/test",
		})
	}))
	defer server.Close()

	result, err := sp.Charge(context.Background(), processor.PaymentRequest{
		Amount:      2000,
		Currency:    "usd",
		Token:       "tok_visa",
		CustomerID:  "cus_test",
		Description: "Test",
		OrderID:     "ord_123",
		Metadata: map[string]interface{}{
			"key": "val",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatal("expected success")
	}
	if result.TransactionID != "ch_test" {
		t.Fatalf("TransactionID = %q", result.TransactionID)
	}
	if result.Fee != currency.Cents(58) {
		t.Fatalf("Fee = %d, want 58", result.Fee)
	}
}

func TestStripeProcessor_Charge_StripeError(t *testing.T) {
	sp, server := makeTestProcessor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(402)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{"message": "Card declined"},
		})
	}))
	defer server.Close()

	result, err := sp.Charge(context.Background(), processor.PaymentRequest{
		Amount:   1000,
		Currency: "usd",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if result.Success {
		t.Fatal("should not succeed")
	}
}

func TestStripeProcessor_Charge_WithCustomerOnly(t *testing.T) {
	sp, server := makeTestProcessor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     "ch_cust",
			"object": "charge",
			"status": "succeeded",
		})
	}))
	defer server.Close()

	result, err := sp.Charge(context.Background(), processor.PaymentRequest{
		Amount:     1000,
		Currency:   "usd",
		CustomerID: "cus_default_card",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatal("expected success")
	}
}

// ---------------------------------------------------------------------------
// Processor.Authorize with mock server
// ---------------------------------------------------------------------------

func TestStripeProcessor_Authorize_Success(t *testing.T) {
	sp, server := makeTestProcessor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":       "ch_auth",
			"object":   "charge",
			"amount":   3000,
			"currency": "usd",
			"status":   "pending",
			"captured": false,
		})
	}))
	defer server.Close()

	result, err := sp.Authorize(context.Background(), processor.PaymentRequest{
		Amount:      3000,
		Currency:    "usd",
		Token:       "tok_auth",
		CustomerID:  "cus_auth",
		Description: "Auth",
		Metadata:    map[string]interface{}{"k": "v"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatal("expected success")
	}
	if result.Status != "authorized" {
		t.Fatalf("Status = %q, want authorized", result.Status)
	}
}

// ---------------------------------------------------------------------------
// Processor.Capture with mock server
// ---------------------------------------------------------------------------

func TestStripeProcessor_Capture_Success(t *testing.T) {
	sp, server := makeTestProcessor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":       "ch_captured",
			"object":   "charge",
			"status":   "succeeded",
			"captured": true,
			"balance_transaction": map[string]interface{}{
				"id":     "txn_cap",
				"object": "balance_transaction",
				"fee":    100,
			},
		})
	}))
	defer server.Close()

	result, err := sp.Capture(context.Background(), "ch_toCapture", currency.Cents(5000))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatal("expected success")
	}
	if result.Fee != currency.Cents(100) {
		t.Fatalf("Fee = %d, want 100", result.Fee)
	}
}

func TestStripeProcessor_Capture_ZeroAmount(t *testing.T) {
	sp, server := makeTestProcessor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     "ch_cap_zero",
			"object": "charge",
			"status": "succeeded",
		})
	}))
	defer server.Close()

	result, err := sp.Capture(context.Background(), "ch_x", currency.Cents(0))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatal("expected success")
	}
}

// ---------------------------------------------------------------------------
// Processor.Refund with mock server
// ---------------------------------------------------------------------------

func TestStripeProcessor_Refund_Success(t *testing.T) {
	sp, server := makeTestProcessor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":       "re_test",
			"object":   "refund",
			"amount":   500,
			"currency": "usd",
			"status":   "succeeded",
		})
	}))
	defer server.Close()

	result, err := sp.Refund(context.Background(), processor.RefundRequest{
		TransactionID: "ch_orig",
		Amount:        500,
		Reason:        "duplicate",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatal("expected success")
	}
	if result.RefundID != "re_test" {
		t.Fatalf("RefundID = %q", result.RefundID)
	}
}

func TestStripeProcessor_Refund_FullAmount(t *testing.T) {
	sp, server := makeTestProcessor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     "re_full",
			"object": "refund",
			"status": "succeeded",
		})
	}))
	defer server.Close()

	result, err := sp.Refund(context.Background(), processor.RefundRequest{
		TransactionID: "ch_full",
		Amount:        0, // 0 = full refund
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatal("expected success")
	}
}

// ---------------------------------------------------------------------------
// IsAvailable
// ---------------------------------------------------------------------------

func TestStripeProcessor_IsAvailable_True(t *testing.T) {
	sp, server := makeTestProcessor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	if !sp.IsAvailable(context.Background()) {
		t.Fatal("should be available")
	}
}

func TestStripeProcessor_IsAvailable_NoClient(t *testing.T) {
	sp := &StripeProcessor{
		BaseProcessor: processor.NewBaseProcessor(processor.Stripe, nil),
		accessToken:   "sk_test",
		client:        nil,
	}
	if sp.IsAvailable(context.Background()) {
		t.Fatal("should not be available without client")
	}
}

func TestStripeProcessor_IsAvailable_NoToken(t *testing.T) {
	sp := &StripeProcessor{
		BaseProcessor: processor.NewBaseProcessor(processor.Stripe, nil),
		accessToken:   "",
	}
	if sp.IsAvailable(context.Background()) {
		t.Fatal("should not be available without token")
	}
}

// ---------------------------------------------------------------------------
// Type
// ---------------------------------------------------------------------------

func TestStripeProcessor_Type(t *testing.T) {
	sp := NewProcessor("sk_test", "")
	if sp.Type() != processor.Stripe {
		t.Fatalf("Type = %q, want stripe", sp.Type())
	}
}

// ---------------------------------------------------------------------------
// Processor.GetTransaction with mock server
// ---------------------------------------------------------------------------

func TestStripeProcessor_GetTransaction_Success(t *testing.T) {
	sp, server := makeTestProcessor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":       "ch_get",
			"object":   "charge",
			"amount":   4200,
			"currency": "usd",
			"status":   "succeeded",
			"customer": map[string]interface{}{"id": "cus_get"},
			"created":  1700000000,
			"balance_transaction": map[string]interface{}{
				"id":     "txn_get",
				"object": "balance_transaction",
				"fee":    120,
			},
		})
	}))
	defer server.Close()

	tx, err := sp.GetTransaction(context.Background(), "ch_get")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tx.ID != "ch_get" {
		t.Fatalf("ID = %q, want ch_get", tx.ID)
	}
	if tx.Amount != currency.Cents(4200) {
		t.Fatalf("Amount = %d, want 4200", tx.Amount)
	}
	if tx.Currency != currency.Type("usd") {
		t.Fatalf("Currency = %q, want usd", tx.Currency)
	}
	if tx.Fee != currency.Cents(120) {
		t.Fatalf("Fee = %d, want 120", tx.Fee)
	}
	if tx.CustomerID != "cus_get" {
		t.Fatalf("CustomerID = %q, want cus_get", tx.CustomerID)
	}
	if tx.Type != "charge" {
		t.Fatalf("Type = %q, want charge", tx.Type)
	}
	if tx.Status != "succeeded" {
		t.Fatalf("Status = %q, want succeeded", tx.Status)
	}
}

func TestStripeProcessor_GetTransaction_NoFee(t *testing.T) {
	sp, server := makeTestProcessor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":       "ch_nofee",
			"object":   "charge",
			"amount":   1000,
			"currency": "eur",
			"status":   "pending",
			"customer": map[string]interface{}{"id": "cus_nofee"},
			"created":  1700000000,
		})
	}))
	defer server.Close()

	tx, err := sp.GetTransaction(context.Background(), "ch_nofee")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tx.Fee != 0 {
		t.Fatalf("Fee = %d, want 0", tx.Fee)
	}
}

func TestStripeProcessor_GetTransaction_Error(t *testing.T) {
	sp, server := makeTestProcessor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{"message": "No such charge"},
		})
	}))
	defer server.Close()

	_, err := sp.GetTransaction(context.Background(), "ch_missing")
	if err == nil {
		t.Fatal("expected error for missing charge")
	}
}

// ---------------------------------------------------------------------------
// Processor.ValidateWebhook
// ---------------------------------------------------------------------------

func TestStripeProcessor_ValidateWebhook_InvalidSignature(t *testing.T) {
	sp := NewProcessor("sk_test", "whsec_secret")
	_, err := sp.ValidateWebhook(context.Background(), []byte(`{"id":"evt_1"}`), "bad_sig")
	if err == nil {
		t.Fatal("expected error for invalid signature")
	}
}

func TestStripeProcessor_ValidateWebhook_EmptyPayload(t *testing.T) {
	sp := NewProcessor("sk_test", "whsec_secret")
	_, err := sp.ValidateWebhook(context.Background(), []byte{}, "")
	if err == nil {
		t.Fatal("expected error for empty payload")
	}
}

// ---------------------------------------------------------------------------
// Processor.Authorize with description and metadata
// ---------------------------------------------------------------------------

func TestStripeProcessor_Authorize_WithCustomerOnly(t *testing.T) {
	sp, server := makeTestProcessor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":       "ch_auth_cust",
			"object":   "charge",
			"status":   "pending",
			"captured": false,
		})
	}))
	defer server.Close()

	result, err := sp.Authorize(context.Background(), processor.PaymentRequest{
		Amount:     2000,
		Currency:   "usd",
		CustomerID: "cus_only",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatal("expected success")
	}
}

func TestStripeProcessor_Authorize_Error(t *testing.T) {
	sp, server := makeTestProcessor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(402)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{"message": "Card declined"},
		})
	}))
	defer server.Close()

	result, err := sp.Authorize(context.Background(), processor.PaymentRequest{
		Amount:   1000,
		Currency: "usd",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if result.Success {
		t.Fatal("should not succeed")
	}
	if result.ErrorMessage == "" {
		t.Fatal("ErrorMessage should be set")
	}
}

// ---------------------------------------------------------------------------
// Processor.Capture with error
// ---------------------------------------------------------------------------

func TestStripeProcessor_Capture_Error(t *testing.T) {
	sp, server := makeTestProcessor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{"message": "already captured"},
		})
	}))
	defer server.Close()

	result, err := sp.Capture(context.Background(), "ch_bad", currency.Cents(1000))
	if err == nil {
		t.Fatal("expected error")
	}
	if result.Success {
		t.Fatal("should not succeed")
	}
}

// ---------------------------------------------------------------------------
// Processor.Refund with error
// ---------------------------------------------------------------------------

func TestStripeProcessor_Refund_Error(t *testing.T) {
	sp, server := makeTestProcessor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{"message": "already refunded"},
		})
	}))
	defer server.Close()

	result, err := sp.Refund(context.Background(), processor.RefundRequest{
		TransactionID: "ch_refunded",
		Amount:        500,
		Reason:        "duplicate",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if result.Success {
		t.Fatal("should not succeed")
	}
	if result.ErrorMessage == "" {
		t.Fatal("ErrorMessage should be set")
	}
}

// ---------------------------------------------------------------------------
// Processor.Charge with no balance transaction (nil)
// ---------------------------------------------------------------------------

func TestStripeProcessor_Charge_NoBalanceTransaction(t *testing.T) {
	sp, server := makeTestProcessor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     "ch_nobal",
			"object": "charge",
			"status": "succeeded",
		})
	}))
	defer server.Close()

	result, err := sp.Charge(context.Background(), processor.PaymentRequest{
		Amount:   1000,
		Currency: "usd",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Fee != 0 {
		t.Fatalf("Fee = %d, want 0 when no balance_transaction", result.Fee)
	}
}

// ---------------------------------------------------------------------------
// Processor.Charge with metadata (non-string values skipped)
// ---------------------------------------------------------------------------

func TestStripeProcessor_Charge_MetadataFiltering(t *testing.T) {
	sp, server := makeTestProcessor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     "ch_meta",
			"object": "charge",
			"status": "succeeded",
		})
	}))
	defer server.Close()

	result, err := sp.Charge(context.Background(), processor.PaymentRequest{
		Amount:   1000,
		Currency: "usd",
		Metadata: map[string]interface{}{
			"str_key": "str_val",
			"int_key": 42,
			"bool_key": true,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatal("expected success")
	}
}

// ---------------------------------------------------------------------------
// SubscriptionProcessor — CreateSubscription
// ---------------------------------------------------------------------------

func makeTestSubscriptionProcessor(t *testing.T, handler http.Handler) (*StripeSubscriptionProcessor, *httptest.Server) {
	t.Helper()
	sp, server := makeTestProcessor(t, handler)
	return &StripeSubscriptionProcessor{StripeProcessor: sp}, server
}

func TestSubscriptionProcessor_CreateSubscription_Success(t *testing.T) {
	sp, server := makeTestSubscriptionProcessor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     "sub_new",
			"object": "subscription",
			"status": "active",
			"customer": map[string]interface{}{"id": "cus_sub"},
			"cancel_at_period_end": false,
			"items": map[string]interface{}{
				"data": []map[string]interface{}{
					{
						"id":                   "si_1",
						"current_period_start": 1700000000,
						"current_period_end":   1702592000,
					},
				},
			},
		})
	}))
	defer server.Close()

	sub, err := sp.CreateSubscription(context.Background(), processor.SubscriptionRequest{
		CustomerID: "cus_sub",
		PlanID:     "price_test",
		Quantity:   1,
		TrialDays:  7,
		Metadata:   map[string]interface{}{"key": "val", "num": 42},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sub.ID != "sub_new" {
		t.Fatalf("ID = %q, want sub_new", sub.ID)
	}
	if sub.Status != "active" {
		t.Fatalf("Status = %q, want active", sub.Status)
	}
	if sub.CustomerID != "cus_sub" {
		t.Fatalf("CustomerID = %q, want cus_sub", sub.CustomerID)
	}
}

func TestSubscriptionProcessor_CreateSubscription_NoItems(t *testing.T) {
	sp, server := makeTestSubscriptionProcessor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":       "sub_noitems",
			"object":   "subscription",
			"status":   "active",
			"customer": map[string]interface{}{"id": "cus_2"},
			"items":    map[string]interface{}{"data": []interface{}{}},
		})
	}))
	defer server.Close()

	sub, err := sp.CreateSubscription(context.Background(), processor.SubscriptionRequest{
		CustomerID: "cus_2",
		PlanID:     "price_2",
		Quantity:   1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sub.CurrentPeriodStart != 0 {
		t.Fatalf("expected 0 period start for no items, got %d", sub.CurrentPeriodStart)
	}
}

func TestSubscriptionProcessor_CreateSubscription_Error(t *testing.T) {
	sp, server := makeTestSubscriptionProcessor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{"message": "No such customer"},
		})
	}))
	defer server.Close()

	_, err := sp.CreateSubscription(context.Background(), processor.SubscriptionRequest{
		CustomerID: "cus_bad",
		PlanID:     "price_x",
		Quantity:   1,
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---------------------------------------------------------------------------
// SubscriptionProcessor — GetSubscription
// ---------------------------------------------------------------------------

func TestSubscriptionProcessor_GetSubscription_Success(t *testing.T) {
	sp, server := makeTestSubscriptionProcessor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":       "sub_get",
			"object":   "subscription",
			"status":   "active",
			"customer": map[string]interface{}{"id": "cus_get"},
			"cancel_at_period_end": true,
			"items": map[string]interface{}{
				"data": []map[string]interface{}{
					{
						"id":                   "si_get",
						"current_period_start": 1700000000,
						"current_period_end":   1702592000,
						"price":                map[string]interface{}{"id": "price_get"},
					},
				},
			},
		})
	}))
	defer server.Close()

	sub, err := sp.GetSubscription(context.Background(), "sub_get")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sub.ID != "sub_get" {
		t.Fatalf("ID = %q, want sub_get", sub.ID)
	}
	if sub.PlanID != "price_get" {
		t.Fatalf("PlanID = %q, want price_get", sub.PlanID)
	}
	if !sub.CancelAtPeriodEnd {
		t.Fatal("CancelAtPeriodEnd should be true")
	}
}

func TestSubscriptionProcessor_GetSubscription_NoPrice(t *testing.T) {
	sp, server := makeTestSubscriptionProcessor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":       "sub_noprice",
			"object":   "subscription",
			"status":   "canceled",
			"customer": map[string]interface{}{"id": "cus_np"},
			"items":    map[string]interface{}{"data": []map[string]interface{}{{"id": "si_np"}}},
		})
	}))
	defer server.Close()

	sub, err := sp.GetSubscription(context.Background(), "sub_noprice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sub.PlanID != "" {
		t.Fatalf("PlanID should be empty when no price, got %q", sub.PlanID)
	}
}

func TestSubscriptionProcessor_GetSubscription_Error(t *testing.T) {
	sp, server := makeTestSubscriptionProcessor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{"message": "No such subscription"},
		})
	}))
	defer server.Close()

	_, err := sp.GetSubscription(context.Background(), "sub_missing")
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---------------------------------------------------------------------------
// SubscriptionProcessor — CancelSubscription
// ---------------------------------------------------------------------------

func TestSubscriptionProcessor_CancelSubscription_Immediately(t *testing.T) {
	sp, server := makeTestSubscriptionProcessor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":       "sub_cancel",
			"object":   "subscription",
			"status":   "canceled",
			"customer": map[string]interface{}{"id": "cus_c"},
		})
	}))
	defer server.Close()

	err := sp.CancelSubscription(context.Background(), "sub_cancel", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSubscriptionProcessor_CancelSubscription_AtPeriodEnd(t *testing.T) {
	sp, server := makeTestSubscriptionProcessor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":       "sub_defer",
			"object":   "subscription",
			"status":   "active",
			"customer": map[string]interface{}{"id": "cus_d"},
			"cancel_at_period_end": true,
		})
	}))
	defer server.Close()

	err := sp.CancelSubscription(context.Background(), "sub_defer", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSubscriptionProcessor_CancelSubscription_Error(t *testing.T) {
	sp, server := makeTestSubscriptionProcessor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{"message": "Not found"},
		})
	}))
	defer server.Close()

	err := sp.CancelSubscription(context.Background(), "sub_missing", true)
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---------------------------------------------------------------------------
// SubscriptionProcessor — UpdateSubscription
// ---------------------------------------------------------------------------

func TestSubscriptionProcessor_UpdateSubscription_Success(t *testing.T) {
	cancelEnd := true
	sp, server := makeTestSubscriptionProcessor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":       "sub_upd",
			"object":   "subscription",
			"status":   "active",
			"customer": map[string]interface{}{"id": "cus_upd"},
			"cancel_at_period_end": true,
			"items": map[string]interface{}{
				"data": []map[string]interface{}{
					{
						"id":                   "si_upd",
						"current_period_start": 1700000000,
						"current_period_end":   1702592000,
						"price":                map[string]interface{}{"id": "price_new"},
					},
				},
			},
		})
	}))
	defer server.Close()

	sub, err := sp.UpdateSubscription(context.Background(), "sub_upd", processor.SubscriptionUpdate{
		PlanID:            "price_new",
		CancelAtPeriodEnd: &cancelEnd,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sub.PlanID != "price_new" {
		t.Fatalf("PlanID = %q, want price_new", sub.PlanID)
	}
	if !sub.CancelAtPeriodEnd {
		t.Fatal("CancelAtPeriodEnd should be true")
	}
}

func TestSubscriptionProcessor_UpdateSubscription_Error(t *testing.T) {
	sp, server := makeTestSubscriptionProcessor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{"message": "Invalid plan"},
		})
	}))
	defer server.Close()

	_, err := sp.UpdateSubscription(context.Background(), "sub_bad", processor.SubscriptionUpdate{
		PlanID: "price_invalid",
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---------------------------------------------------------------------------
// SubscriptionProcessor — ListSubscriptions
// ---------------------------------------------------------------------------

func TestSubscriptionProcessor_ListSubscriptions_Success(t *testing.T) {
	sp, server := makeTestSubscriptionProcessor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"object":   "list",
			"has_more": false,
			"data": []map[string]interface{}{
				{
					"id":       "sub_1",
					"object":   "subscription",
					"status":   "active",
					"customer": map[string]interface{}{"id": "cus_list"},
					"items": map[string]interface{}{
						"data": []map[string]interface{}{
							{
								"id":    "si_1",
								"price": map[string]interface{}{"id": "price_a"},
							},
						},
					},
				},
				{
					"id":       "sub_2",
					"object":   "subscription",
					"status":   "canceled",
					"customer": map[string]interface{}{"id": "cus_list"},
					"items":    map[string]interface{}{"data": []interface{}{}},
				},
			},
		})
	}))
	defer server.Close()

	subs, err := sp.ListSubscriptions(context.Background(), "cus_list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(subs) != 2 {
		t.Fatalf("expected 2 subscriptions, got %d", len(subs))
	}
	if subs[0].ID != "sub_1" {
		t.Fatalf("first sub ID = %q, want sub_1", subs[0].ID)
	}
	if subs[0].PlanID != "price_a" {
		t.Fatalf("first sub PlanID = %q, want price_a", subs[0].PlanID)
	}
	if subs[1].PlanID != "" {
		t.Fatalf("second sub PlanID should be empty, got %q", subs[1].PlanID)
	}
}

func TestSubscriptionProcessor_ListSubscriptions_Empty(t *testing.T) {
	sp, server := makeTestSubscriptionProcessor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"object":   "list",
			"has_more": false,
			"data":     []interface{}{},
		})
	}))
	defer server.Close()

	subs, err := sp.ListSubscriptions(context.Background(), "cus_empty")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(subs) != 0 {
		t.Fatalf("expected 0 subscriptions, got %d", len(subs))
	}
}

// ---------------------------------------------------------------------------
// PaymentToCard
// ---------------------------------------------------------------------------

func TestPaymentToCard(t *testing.T) {
	pay := &payment.Payment{}
	pay.Account.Number = "4242424242424242"
	pay.Account.CVC = "123"
	pay.Account.Month = 12
	pay.Account.Year = 2027
	pay.Buyer.FirstName = "John"
	pay.Buyer.LastName = "Doe"
	pay.Buyer.BillingAddress.Line1 = "123 Main St"
	pay.Buyer.BillingAddress.Line2 = "Apt 4"
	pay.Buyer.BillingAddress.City = "New York"
	pay.Buyer.BillingAddress.State = "NY"
	pay.Buyer.BillingAddress.PostalCode = "10001"
	pay.Buyer.BillingAddress.Country = "US"

	card := PaymentToCard(pay)
	if card == nil {
		t.Fatal("card should not be nil")
	}
	if *card.Number != "4242424242424242" {
		t.Fatalf("Number = %q, want 4242...", *card.Number)
	}
	if *card.CVC != "123" {
		t.Fatalf("CVC = %q, want 123", *card.CVC)
	}
	if *card.ExpMonth != "12" {
		t.Fatalf("ExpMonth = %q, want 12", *card.ExpMonth)
	}
	if *card.ExpYear != "2027" {
		t.Fatalf("ExpYear = %q, want 2027", *card.ExpYear)
	}
	if *card.AddressLine1 != "123 Main St" {
		t.Fatalf("AddressLine1 = %q", *card.AddressLine1)
	}
	if *card.AddressCity != "New York" {
		t.Fatalf("AddressCity = %q", *card.AddressCity)
	}
	if *card.AddressState != "NY" {
		t.Fatalf("AddressState = %q", *card.AddressState)
	}
	if *card.AddressZip != "10001" {
		t.Fatalf("AddressZip = %q", *card.AddressZip)
	}
	if *card.AddressCountry != "US" {
		t.Fatalf("AddressCountry = %q", *card.AddressCountry)
	}
}

func TestPaymentToCard_EmptyFields(t *testing.T) {
	pay := &payment.Payment{}
	card := PaymentToCard(pay)
	if card == nil {
		t.Fatal("card should not be nil even with empty payment")
	}
	if *card.Number != "" {
		t.Fatalf("Number should be empty, got %q", *card.Number)
	}
}

// ---------------------------------------------------------------------------
// SubscriptionToCard
// ---------------------------------------------------------------------------

func TestSubscriptionToCard(t *testing.T) {
	sub := &subscription.Subscription{}
	sub.Account.Number = "5555555555554444"
	sub.Account.CVC = "456"
	sub.Account.Month = 6
	sub.Account.Year = 2028
	sub.Buyer.FirstName = "Jane"
	sub.Buyer.LastName = "Smith"
	sub.Buyer.BillingAddress.Line1 = "456 Oak Ave"

	card := SubscriptionToCard(sub)
	if card == nil {
		t.Fatal("card should not be nil")
	}
	if *card.Number != "5555555555554444" {
		t.Fatalf("Number = %q", *card.Number)
	}
	if *card.ExpMonth != "6" {
		t.Fatalf("ExpMonth = %q, want 6", *card.ExpMonth)
	}
	if *card.AddressLine1 != "456 Oak Ave" {
		t.Fatalf("AddressLine1 = %q", *card.AddressLine1)
	}
}

// ---------------------------------------------------------------------------
// toPtr generic helper
// ---------------------------------------------------------------------------

func TestToPtr(t *testing.T) {
	s := "hello"
	p := toPtr(s)
	if *p != "hello" {
		t.Fatalf("toPtr string = %q", *p)
	}

	n := 42
	np := toPtr(n)
	if *np != 42 {
		t.Fatalf("toPtr int = %d", *np)
	}
}

// ---------------------------------------------------------------------------
// Client.New
// ---------------------------------------------------------------------------

func TestNewClient(t *testing.T) {
	ctx := context.Background()
	c := New(ctx, "sk_test_new")
	if c == nil {
		t.Fatal("client should not be nil")
	}
	if c.API == nil {
		t.Fatal("client.API should not be nil")
	}
}

// ---------------------------------------------------------------------------
// RefundPayment validation paths
// ---------------------------------------------------------------------------

func TestRefundPayment_GreaterThanPayment(t *testing.T) {
	ctx := context.Background()
	c := New(ctx, "sk_test_refund")
	pay := &payment.Payment{}
	pay.Amount = 1000
	pay.AmountRefunded = 0
	pay.Status = payment.Paid

	_, err := c.RefundPayment(pay, 1500)
	if err == nil {
		t.Fatal("expected error when refund > payment")
	}
}

func TestRefundPayment_ExceedsWithPrior(t *testing.T) {
	ctx := context.Background()
	c := New(ctx, "sk_test_refund2")
	pay := &payment.Payment{}
	pay.Amount = 1000
	pay.AmountRefunded = 600
	pay.Status = payment.Paid

	_, err := c.RefundPayment(pay, 500) // 600 + 500 = 1100 > 1000
	if err == nil {
		t.Fatal("expected error when cumulative refund > payment")
	}
}

func TestRefundPayment_UnpaidTransaction(t *testing.T) {
	ctx := context.Background()
	c := New(ctx, "sk_test_refund3")
	pay := &payment.Payment{}
	pay.Amount = 1000
	pay.Status = payment.Unpaid

	_, err := c.RefundPayment(pay, 500)
	if err == nil {
		t.Fatal("expected error for unpaid transaction")
	}
}
