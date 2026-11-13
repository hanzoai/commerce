package billing

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/billing/creditledger"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/payment/providers/paypal"
	types "github.com/hanzoai/commerce/types"
	"github.com/hanzoai/commerce/util/test/ae"
)

type rewritePayPalTransport struct {
	base      http.RoundTripper
	targetURL string
}

func (rt *rewritePayPalTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.URL.Scheme = "http"
	clone.URL.Host = strings.TrimPrefix(rt.targetURL, "http://")
	return rt.base.RoundTrip(clone)
}

func newMockPayPalServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/oauth2/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "mock-paypal-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	})
	mux.HandleFunc("/v2/checkout/orders", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		intent, _ := body["intent"].(string)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":     "PP-ORD-CREATED-1",
			"intent": intent,
			"status": "CREATED",
		})
	})
	mux.HandleFunc("/v2/checkout/orders/PP-ORD-TOPUP-1/capture", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":     "PP-ORD-TOPUP-1",
			"status": "COMPLETED",
			"payment_source": map[string]any{
				"paypal": map[string]any{"account_id": "PAYERACCT123"},
			},
			"purchase_units": []map[string]any{{
				"payments": map[string]any{
					"captures": []map[string]any{{
						"id":     "PP-CAP-TOPUP-1",
						"status": "COMPLETED",
						"amount": map[string]string{"currency_code": "USD", "value": "25.00"},
					}},
				},
			}},
		})
	})
	mux.HandleFunc("/v2/checkout/orders/PP-ORD-SUB-1/authorize", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":     "PP-ORD-SUB-1",
			"status": "COMPLETED",
			"payment_source": map[string]any{
				"card": map[string]any{"brand": "VISA", "last_digits": "1881", "expiry": "2029-05"},
			},
			"purchase_units": []map[string]any{{
				"payments": map[string]any{
					"authorizations": []map[string]any{{
						"id":     "PP-AUTH-SUB-1",
						"status": "CREATED",
						"amount": map[string]string{"currency_code": "USD", "value": "20.00"},
					}},
				},
			}},
		})
	})
	mux.HandleFunc("/v2/payments/authorizations/PP-AUTH-SUB-1/capture", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":     "PP-CAP-SUB-1",
			"status": "COMPLETED",
			"amount": map[string]string{"currency_code": "USD", "value": "20.00"},
		})
	})
	mux.HandleFunc("/v1/notifications/verify-webhook-signature", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "application/json")
		if body["webhook_id"] == "WH-LIVE-TEST" && body["transmission_sig"] == "valid-paypal-sig" {
			_ = json.NewEncoder(w).Encode(map[string]string{"verification_status": "SUCCESS"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"verification_status": "FAILURE"})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func injectMockPayPal(t *testing.T, srv *httptest.Server) *paypal.Provider {
	t.Helper()
	origTransport := http.DefaultTransport
	http.DefaultTransport = &rewritePayPalTransport{base: origTransport, targetURL: srv.URL}
	t.Cleanup(func() { http.DefaultTransport = origTransport })

	pp := paypal.NewProvider(paypal.Config{
		ClientID:     "sb-client-id",
		ClientSecret: "sb-client-secret",
		WebhookID:    "WH-LIVE-TEST",
		Sandbox:      true,
	})

	prevProcessors := processorsForOrg
	processorsForOrg = func(org *organization.Organization) *processor.Registry {
		reg := processor.NewRegistry(processor.DefaultConfig())
		reg.Register(pp)
		return reg
	}
	t.Cleanup(func() { processorsForOrg = prevProcessors })

	prevGlobal, _ := processor.Global().Registered(processor.PayPal)
	processor.Global().Register(pp)
	t.Cleanup(func() {
		if prevGlobal != nil {
			processor.Global().Register(prevGlobal)
		}
	})
	return pp
}

func TestPayPal_CreateOrderTopupSubscribeAndWebhooks(t *testing.T) {
	srv := newMockPayPalServer(t)
	injectMockPayPal(t, srv)

	fake := newFakeLedger()
	injectLedger(t, fake)

	ctx := ae.NewContext()
	defer ctx.Close()

	db := datastore.New(ctx)
	org := organization.New(db)
	org.Name = "hanzo"
	org.MustCreate()

	pl := plan.New(plan.AuthorityDB(ctx))
	pl.Slug = "dev-paypal"
	pl.Name = "Developer"
	pl.Price = currency.Cents(2000)
	pl.Currency = currency.USD
	pl.Interval = types.Monthly
	pl.IntervalCount = 1
	pl.Category = "personal"
	pl.Status = plan.StatusActive
	pl.Managed = true
	pl.MustCreate()

	// 1. CreatePayPalOrder for top-up (CAPTURE, bounded).
	if _, f := CreatePayPalOrder(ctx, org, PayPalOrderIn{AmountCents: 100, Subject: "hanzo/alice"}); f == nil || f.Status != 400 {
		t.Fatalf("expected 400 for sub-minimum topup, got %+v", f)
	}
	topOrder, f := CreatePayPalOrder(ctx, org, PayPalOrderIn{
		AmountCents:    2500,
		Currency:       "usd",
		Subject:        "hanzo/alice",
		IdempotencyKey: "pp-order-top-1",
	})
	if f != nil {
		t.Fatalf("CreatePayPalOrder topup: %v", f)
	}
	if topOrder.OrderID != "PP-ORD-CREATED-1" || topOrder.Intent != "CAPTURE" || topOrder.AmountCents != 2500 {
		t.Fatalf("unexpected topup order: %+v", topOrder)
	}

	// 2. TakePayment with Provider="paypal" captures order and credits balance.
	paid, pf := TakePayment(ctx, org, TakePaymentIn{
		SourceID:       "PP-ORD-TOPUP-1",
		AmountCents:    2500,
		Currency:       "usd",
		Subject:        "hanzo/alice",
		IdempotencyKey: "pp-take-1",
		Provider:       "paypal",
	})
	if pf != nil {
		t.Fatalf("TakePayment paypal: %v", pf)
	}
	// Mirror the host's settle.go post-handler credit into creditledger under ProcessorRef.
	if paid.ProcessorRef != "PP-CAP-TOPUP-1" {
		t.Fatalf("unexpected TakePaymentOut: %+v", paid)
	}
	if _, _, err := fake.Credit(ctx, creditledger.CreditInput{
		Org:            org.Name,
		Subject:        "hanzo/alice",
		AmountCents:    2500,
		Currency:       "usd",
		IdempotencyKey: paid.ProcessorRef,
		Test:           paid.Test,
	}); err != nil {
		t.Fatalf("host settle credit: %v", err)
	}

	// 3. CreatePayPalOrder + Subscribe with Provider="paypal" (AUTHORIZE -> Capture -> Subscription).
	subOrder, sf := CreatePayPalOrder(ctx, org, PayPalOrderIn{
		PlanID:         "dev-paypal",
		Subject:        "hanzo/alice",
		IdempotencyKey: "pp-order-sub-1",
	})
	if sf != nil {
		t.Fatalf("CreatePayPalOrder sub: %v", sf)
	}
	if subOrder.Intent != "AUTHORIZE" || subOrder.AmountCents != 2000 {
		t.Fatalf("unexpected sub order: %+v", subOrder)
	}

	sale, serr := Subscribe(ctx, org, SubscribeIn{
		SourceID:       "PP-ORD-SUB-1",
		PlanID:         "dev-paypal",
		Subject:        "hanzo/alice",
		Provider:       "paypal",
		Wallet:         true,
		IdempotencyKey: "pp-sub-1",
	})
	if serr != nil {
		t.Fatalf("Subscribe paypal: %v", serr)
	}
	if sale.SubscriptionID == "" || sale.AmountCents != 2000 || sale.PaymentMethodID != "credits" {
		t.Fatalf("unexpected PayPal subscription sale: %+v", sale)
	}

	// 4. Deliver PayPal webhooks through HandleProviderWebhook:
	//    - PAYMENT.CAPTURE.COMPLETED for PP-CAP-TOPUP-1 (already indexed -> no duplicate credit)
	//    - PAYMENT.CAPTURE.REFUNDED via links[rel=up] -> claws back $10.00 (1000 cents)
	//    - CUSTOMER.DISPUTE.CREATED (CHARGEBACK) -> claws back remaining $15.00 (1500 cents)
	//    - CUSTOMER.DISPUTE.RESOLVED (RESOLVED_SELLER_FAVOUR) -> restores $15.00 (1500 cents)
	deliverPP := func(body string, sig string) int {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/v1/commerce/webhooks/paypal", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if sig != "" {
			req.Header.Set("Paypal-Transmission-Id", "tx-1")
			req.Header.Set("Paypal-Transmission-Time", "2026-11-02T12:00:00Z")
			req.Header.Set("Paypal-Cert-Url", "https://api.paypal.com/v1/notifications/certs/CERT-1")
			req.Header.Set("Paypal-Auth-Algo", "SHA256withRSA")
			req.Header.Set("Paypal-Transmission-Sig", sig)
		}
		resp := driveSeeded(func(c *zip.Ctx) { c.SetContext(ctx) }, "/v1/commerce/webhooks/:provider", req, HandleProviderWebhook)
		return resp.StatusCode
	}

	// Unsigned -> 400; invalid sig -> 401.
	if st := deliverPP(`{"id":"WH-BAD-0","event_type":"PAYMENT.CAPTURE.COMPLETED"}`, ""); st != http.StatusBadRequest {
		t.Fatalf("unsigned status = %d, want 400", st)
	}
	if st := deliverPP(`{"id":"WH-BAD-1","event_type":"PAYMENT.CAPTURE.COMPLETED"}`, "forged-sig"); st != http.StatusUnauthorized {
		t.Fatalf("forged status = %d, want 401", st)
	}

	// Settlement of already-credited topup capture -> 200, 0 extra credits.
	creditsBefore := len(fake.credits())
	capEvt := `{"id":"WH-PP-CAP-1","event_type":"PAYMENT.CAPTURE.COMPLETED","resource":{"id":"PP-CAP-TOPUP-1","status":"COMPLETED","amount":{"currency_code":"USD","value":"25.00"}}}`
	if st := deliverPP(capEvt, "valid-paypal-sig"); st != http.StatusOK {
		t.Fatalf("PAYMENT.CAPTURE.COMPLETED status = %d, want 200", st)
	}
	if got := len(fake.credits()); got != creditsBefore {
		t.Fatalf("expected 0 duplicate webhook credits for in-session capture, before=%d after=%d", creditsBefore, got)
	}

	// Partial refund ($10.00) referencing capture via links[rel=up].
	refEvt := fmt.Sprintf(`{"id":"WH-PP-REF-1","event_type":"PAYMENT.CAPTURE.REFUNDED","resource":{
		"id":"PP-REF-1","status":"COMPLETED","amount":{"currency_code":"USD","value":"10.00"},
		"links":[{"href":"https://api.paypal.com/v2/payments/captures/PP-CAP-TOPUP-1","rel":"up","method":"GET"}]
	}}`)
	if st := deliverPP(refEvt, "valid-paypal-sig"); st != http.StatusOK {
		t.Fatalf("PAYMENT.CAPTURE.REFUNDED status = %d, want 200", st)
	}
	fake.mu.Lock()
	if len(fake.clawbacks) != 1 || fake.clawbacks[0].AmountCents != 1000 {
		t.Fatalf("unexpected clawbacks after refund: %+v", fake.clawbacks)
	}
	fake.mu.Unlock()

	// Chargeback dispute ($25.00 disputed, capped to remaining $15.00 in local books; clawback posted).
	dispEvt := `{"id":"WH-PP-DISP-1","event_type":"CUSTOMER.DISPUTE.CREATED","resource":{
		"dispute_id":"PP-DISP-1","dispute_life_cycle_stage":"CHARGEBACK","status":"WAITING_FOR_SELLER_RESPONSE",
		"update_time":"2026-11-02T13:00:00Z",
		"dispute_amount":{"currency_code":"USD","value":"15.00"},
		"disputed_transactions":[{"seller_transaction_id":"PP-CAP-TOPUP-1"}]
	}}`
	if st := deliverPP(dispEvt, "valid-paypal-sig"); st != http.StatusOK {
		t.Fatalf("CUSTOMER.DISPUTE.CREATED status = %d, want 200", st)
	}
	fake.mu.Lock()
	if len(fake.clawbacks) != 2 || fake.clawbacks[1].AmountCents != 1500 {
		t.Fatalf("unexpected clawbacks after dispute: %+v", fake.clawbacks)
	}
	fake.mu.Unlock()

	// Dispute won -> restores withheld $15.00.
	wonEvt := `{"id":"WH-PP-DISP-2","event_type":"CUSTOMER.DISPUTE.RESOLVED","resource":{
		"dispute_id":"PP-DISP-1","dispute_life_cycle_stage":"CHARGEBACK","status":"RESOLVED",
		"dispute_outcome":{"outcome_code":"RESOLVED_SELLER_FAVOUR"},
		"update_time":"2026-11-02T14:00:00Z",
		"dispute_amount":{"currency_code":"USD","value":"15.00"},
		"disputed_transactions":[{"seller_transaction_id":"PP-CAP-TOPUP-1"}]
	}}`
	if st := deliverPP(wonEvt, "valid-paypal-sig"); st != http.StatusOK {
		t.Fatalf("CUSTOMER.DISPUTE.RESOLVED status = %d, want 200", st)
	}
	fake.mu.Lock()
	if len(fake.restores) != 1 || fake.restores[0].AmountCents != 1500 {
		t.Fatalf("unexpected restores after dispute won: %+v", fake.restores)
	}
	fake.mu.Unlock()
}
