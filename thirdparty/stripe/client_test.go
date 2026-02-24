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
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/transfer"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/models/user"
)

// makeTestClient creates a legacy Client backed by a mock Stripe API server.
func makeTestClient(t *testing.T, handler http.Handler) (Client, *httptest.Server) {
	t.Helper()

	server := httptest.NewServer(handler)

	backend := sgo.GetBackendWithConfig(sgo.APIBackend, &sgo.BackendConfig{
		URL: sgo.String(server.URL),
	})

	api := &client.API{}
	api.Init("sk_test_client", &sgo.Backends{
		API:     backend,
		Uploads: backend,
	})

	c := Client{
		API: api,
		ctx: context.Background(),
	}

	return c, server
}

// ---------------------------------------------------------------------------
// Client.GetCharge
// ---------------------------------------------------------------------------

func TestClient_GetCharge_Success(t *testing.T) {
	c, server := makeTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":       "ch_client",
			"object":   "charge",
			"amount":   5000,
			"currency": "usd",
			"status":   "succeeded",
		})
	}))
	defer server.Close()

	ch, err := c.GetCharge("ch_client")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ch == nil {
		t.Fatal("charge should not be nil")
	}
}

func TestClient_GetCharge_Error(t *testing.T) {
	c, server := makeTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{"message": "No such charge"},
		})
	}))
	defer server.Close()

	_, err := c.GetCharge("ch_missing")
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---------------------------------------------------------------------------
// Client.Capture
// ---------------------------------------------------------------------------

func TestClient_Capture_Success(t *testing.T) {
	c, server := makeTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":       "ch_cap",
			"object":   "charge",
			"status":   "succeeded",
			"captured": true,
		})
	}))
	defer server.Close()

	ch, err := c.Capture("ch_cap")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ch == nil {
		t.Fatal("charge should not be nil")
	}
}

func TestClient_Capture_Error(t *testing.T) {
	c, server := makeTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{"message": "already captured"},
		})
	}))
	defer server.Close()

	_, err := c.Capture("ch_bad")
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---------------------------------------------------------------------------
// Client.GetCard
// ---------------------------------------------------------------------------

func TestClient_GetCard_Success(t *testing.T) {
	c, server := makeTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     "card_123",
			"object": "card",
			"last4":  "4242",
			"brand":  "Visa",
		})
	}))
	defer server.Close()

	card, err := c.GetCard("card_123", "cus_123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if card == nil {
		t.Fatal("card should not be nil")
	}
}

func TestClient_GetCard_Error(t *testing.T) {
	c, server := makeTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{"message": "No such card"},
		})
	}))
	defer server.Close()

	_, err := c.GetCard("card_bad", "cus_bad")
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---------------------------------------------------------------------------
// Client.GetCustomer
// ---------------------------------------------------------------------------

func TestClient_GetCustomer_Success(t *testing.T) {
	c, server := makeTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     "cus_get",
			"object": "customer",
			"email":  "test@example.com",
		})
	}))
	defer server.Close()

	usr := &user.User{}
	usr.Accounts.Stripe.CustomerId = "cus_get"

	cust, err := c.GetCustomer("acct_connect", usr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cust == nil {
		t.Fatal("customer should not be nil")
	}
}

func TestClient_GetCustomer_Error(t *testing.T) {
	c, server := makeTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{"message": "No such customer"},
		})
	}))
	defer server.Close()

	usr := &user.User{}
	usr.Accounts.Stripe.CustomerId = "cus_missing"

	_, err := c.GetCustomer("token", usr)
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---------------------------------------------------------------------------
// Client.NewCustomer
// ---------------------------------------------------------------------------

// NOTE: Client.NewCustomer requires user.Id() which panics with "unimplemented"
// in the User model. Cannot test without a proper User initialization (needs datastore).
func TestClient_NewCustomer_RequiresDatastore(t *testing.T) {
	t.Skip("requires datastore: User.Id() panics without Entity init")
}

// ---------------------------------------------------------------------------
// Client.UpdateCustomer
// ---------------------------------------------------------------------------

// NOTE: Client.UpdateCustomer calls usr.Id() which panics.
func TestClient_UpdateCustomer_RequiresDatastore(t *testing.T) {
	t.Skip("requires datastore: User.Id() panics without Entity init")
}

// ---------------------------------------------------------------------------
// Client.NewCard
// ---------------------------------------------------------------------------

func TestClient_NewCard_Success(t *testing.T) {
	c, server := makeTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     "card_new",
			"object": "card",
			"last4":  "1234",
		})
	}))
	defer server.Close()

	usr := &user.User{}
	usr.Accounts.Stripe.CustomerId = "cus_card"

	card, err := c.NewCard("tok_newcard", usr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if card == nil {
		t.Fatal("card should not be nil")
	}
}

func TestClient_NewCard_Error(t *testing.T) {
	c, server := makeTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{"message": "Invalid token"},
		})
	}))
	defer server.Close()

	usr := &user.User{}
	usr.Accounts.Stripe.CustomerId = "cus_bad"

	_, err := c.NewCard("tok_bad", usr)
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---------------------------------------------------------------------------
// Client.UpdateCard
// ---------------------------------------------------------------------------

func TestClient_UpdateCard_Success(t *testing.T) {
	c, server := makeTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     "card_upd",
			"object": "card",
		})
	}))
	defer server.Close()

	usr := &user.User{}
	usr.Accounts.Stripe.CustomerId = "cus_card_upd"
	usr.Accounts.Stripe.CardId = "card_upd"

	card, err := c.UpdateCard("tok_upd", usr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if card == nil {
		t.Fatal("card should not be nil")
	}
}

// ---------------------------------------------------------------------------
// Client.DeleteCard
// ---------------------------------------------------------------------------

func TestClient_DeleteCard_Success(t *testing.T) {
	c, server := makeTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":      "card_del",
			"object":  "card",
			"deleted": true,
		})
	}))
	defer server.Close()

	usr := &user.User{}
	usr.Accounts.Stripe.CustomerId = "cus_del"

	card, err := c.DeleteCard("card_del", usr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if card == nil {
		t.Fatal("card should not be nil")
	}
}

func TestClient_DeleteCard_Error(t *testing.T) {
	c, server := makeTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{"message": "No such card"},
		})
	}))
	defer server.Close()

	usr := &user.User{}
	usr.Accounts.Stripe.CustomerId = "cus_del_bad"

	_, err := c.DeleteCard("card_missing", usr)
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---------------------------------------------------------------------------
// Client.Authorize
// ---------------------------------------------------------------------------

func TestClient_Authorize_Success(t *testing.T) {
	c, server := makeTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     "tok_auth",
			"object": "token",
			"type":   "card",
			"card": map[string]interface{}{
				"id":    "card_auth",
				"last4": "4242",
			},
		})
	}))
	defer server.Close()

	pay := &payment.Payment{}
	pay.Account.Number = "4242424242424242"
	pay.Account.CVC = "123"
	pay.Account.Month = 12
	pay.Account.Year = 2027

	tok, err := c.Authorize(pay)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok == nil {
		t.Fatal("token should not be nil")
	}
}

func TestClient_Authorize_Error(t *testing.T) {
	c, server := makeTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(402)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{"message": "Card error"},
		})
	}))
	defer server.Close()

	pay := &payment.Payment{}

	_, err := c.Authorize(pay)
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---------------------------------------------------------------------------
// Client.AuthorizeSubscription
// ---------------------------------------------------------------------------

func TestClient_AuthorizeSubscription_Success(t *testing.T) {
	c, server := makeTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     "tok_sub_auth",
			"object": "token",
			"type":   "card",
		})
	}))
	defer server.Close()

	sub := &subscription.Subscription{}
	sub.Account.Number = "5555555555554444"
	sub.Account.CVC = "456"
	sub.Account.Month = 6
	sub.Account.Year = 2028

	tok, err := c.AuthorizeSubscription(sub)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok == nil {
		t.Fatal("token should not be nil")
	}
}

// ---------------------------------------------------------------------------
// Client.NewPlan
// ---------------------------------------------------------------------------

func TestClient_NewPlan_Success(t *testing.T) {
	c, server := makeTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":             "plan_new",
			"object":         "plan",
			"nickname":       "Pro",
			"currency":       "usd",
			"interval":       "month",
			"interval_count": 1,
		})
	}))
	defer server.Close()

	p := &plan.Plan{
		Name:            "Pro",
		Currency:        "usd",
		IntervalCount:   1,
		TrialPeriodDays: 7,
	}
	p.Id_ = "plan_new"
	p.Interval = "month"

	result, err := c.NewPlan(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("plan should not be nil")
	}
}

func TestClient_NewPlan_Error(t *testing.T) {
	c, server := makeTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{"message": "Invalid plan"},
		})
	}))
	defer server.Close()

	p := &plan.Plan{}
	p.Id_ = "plan_err"
	p.Interval = "month"

	_, err := c.NewPlan(p)
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---------------------------------------------------------------------------
// Client.UpdatePlan
// ---------------------------------------------------------------------------

func TestClient_UpdatePlan_Success(t *testing.T) {
	c, server := makeTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":       "plan_upd",
			"object":   "plan",
			"nickname": "Pro Plus",
		})
	}))
	defer server.Close()

	p := &plan.Plan{
		Name:     "Pro Plus",
		Currency: "usd",
	}
	p.Id_ = "plan_upd"
	p.Interval = "month"

	result, err := c.UpdatePlan(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("plan should not be nil")
	}
}

// ---------------------------------------------------------------------------
// Client.NewCharge
// ---------------------------------------------------------------------------

func TestClient_NewCharge_SourceString(t *testing.T) {
	c, server := makeTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":       "ch_new",
			"object":   "charge",
			"amount":   3000,
			"currency": "usd",
			"status":   "pending",
		})
	}))
	defer server.Close()

	pay := &payment.Payment{
		Amount:   3000,
		Currency: "usd",
	}
	pay.Id_ = "pay_new"
	pay.Account.CustomerId = "cus_charge"
	pay.Description = "Test charge"
	pay.OrderId = "ord_new"
	pay.Metadata = map[string]interface{}{"key": "val"}

	ch, err := c.NewCharge("tok_123", pay)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ch == nil {
		t.Fatal("charge should not be nil")
	}
	if pay.Account.ChargeId == "" {
		t.Fatal("ChargeId should be updated")
	}
}

// NOTE: Client.NewCharge with User source calls usr.Id() which panics.
func TestClient_NewCharge_SourceUser_RequiresDatastore(t *testing.T) {
	t.Skip("requires datastore: User.Id() panics without Entity init")
}

func TestClient_NewCharge_Error(t *testing.T) {
	c, server := makeTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(402)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{"message": "Card declined"},
		})
	}))
	defer server.Close()

	pay := &payment.Payment{Amount: 1000, Currency: "usd"}
	pay.Id_ = "pay_err"

	_, err := c.NewCharge("tok_bad", pay)
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---------------------------------------------------------------------------
// Client.UpdateCharge
// ---------------------------------------------------------------------------

func TestClient_UpdateCharge_Success(t *testing.T) {
	c, server := makeTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     "ch_upd",
			"object": "charge",
			"status": "succeeded",
		})
	}))
	defer server.Close()

	pay := &payment.Payment{
		Description: "Updated desc",
	}
	pay.Id_ = "pay_upd"
	pay.OrderId = "ord_upd"
	pay.Account.ChargeId = "ch_upd"
	pay.Metadata = map[string]interface{}{"str_key": "val", "int_key": 42}
	pay.Buyer.UserId = "usr_buyer"

	ch, err := c.UpdateCharge(pay)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ch == nil {
		t.Fatal("charge should not be nil")
	}
}

func TestClient_UpdateCharge_NilMetadata(t *testing.T) {
	c, server := makeTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     "ch_nil_meta",
			"object": "charge",
			"status": "succeeded",
		})
	}))
	defer server.Close()

	pay := &payment.Payment{}
	pay.Id_ = "pay_nil"
	pay.Account.ChargeId = "ch_nil_meta"
	pay.Metadata = nil // will be initialized

	ch, err := c.UpdateCharge(pay)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ch == nil {
		t.Fatal("charge should not be nil")
	}
	if pay.Metadata == nil {
		t.Fatal("Metadata should be initialized")
	}
}

// ---------------------------------------------------------------------------
// Client.NewSubscription
// ---------------------------------------------------------------------------

func TestClient_NewSubscription_WithCustomer(t *testing.T) {
	c, server := makeTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":       "sub_client",
			"object":   "subscription",
			"status":   "active",
			"customer": map[string]interface{}{"id": "cus_sub"},
			"application_fee_percent": 5.0,
			"cancel_at_period_end":    false,
			"items": map[string]interface{}{
				"data": []map[string]interface{}{
					{
						"id":                   "si_1",
						"quantity":             2,
						"current_period_start": 1700000000,
						"current_period_end":   1702592000,
					},
				},
			},
		})
	}))
	defer server.Close()

	cust := &Customer{}
	cust.ID = "cus_sub"

	sub := &subscription.Subscription{}
	sub.Plan.Id_ = "plan_test"

	_, err := c.NewSubscription(cust, sub)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sub.Account.CustomerId != "cus_sub" {
		t.Fatalf("CustomerId = %q, want cus_sub", sub.Account.CustomerId)
	}
	if sub.Quantity != 2 {
		t.Fatalf("Quantity = %d, want 2", sub.Quantity)
	}
}

// NOTE: Client.NewSubscription with User calls usr.Id() which panics.
func TestClient_NewSubscription_WithUser_RequiresDatastore(t *testing.T) {
	t.Skip("requires datastore: User.Id() panics without Entity init")
}

func TestClient_NewSubscription_Error(t *testing.T) {
	c, server := makeTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{"message": "No such plan"},
		})
	}))
	defer server.Close()

	sub := &subscription.Subscription{}
	sub.Plan.Id_ = "plan_bad"

	_, err := c.NewSubscription(nil, sub)
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---------------------------------------------------------------------------
// Client.UpdateSubscription
// ---------------------------------------------------------------------------

func TestClient_UpdateSubscription_Success(t *testing.T) {
	c, server := makeTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":       "sub_upd_client",
			"object":   "subscription",
			"status":   "active",
			"customer": map[string]interface{}{"id": "cus_upd_sub"},
			"items": map[string]interface{}{
				"data": []map[string]interface{}{
					{
						"id":                   "si_upd",
						"quantity":             3,
						"current_period_start": 1700000000,
						"current_period_end":   1702592000,
					},
				},
			},
		})
	}))
	defer server.Close()

	sub := &subscription.Subscription{}
	sub.Ref.Stripe.Id = "sub_upd_client"
	sub.Account.CustomerId = "cus_upd_sub"
	sub.Plan.Id_ = "plan_new"
	sub.Quantity = 3

	_, err := c.UpdateSubscription(sub)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Client.CancelSubscription
// ---------------------------------------------------------------------------

func TestClient_CancelSubscription_Success(t *testing.T) {
	c, server := makeTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":          "sub_cancel_client",
			"object":      "subscription",
			"status":      "canceled",
			"customer":    map[string]interface{}{"id": "cus_cancel"},
			"canceled_at": 1700001000,
			"items":       map[string]interface{}{"data": []interface{}{}},
		})
	}))
	defer server.Close()

	sub := &subscription.Subscription{}
	sub.Ref.Stripe.Id = "sub_cancel_client"

	_, err := c.CancelSubscription(sub)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Client.Transfer
// ---------------------------------------------------------------------------

func TestClient_Transfer_Success(t *testing.T) {
	c, server := makeTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":              "tr_new",
			"object":          "transfer",
			"amount":          2000,
			"amount_reversed": 0,
			"currency":        "usd",
			"livemode":        true,
			"reversed":        false,
			"destination":     map[string]interface{}{"id": "acct_dest", "type": "custom"},
			"destination_payment": map[string]interface{}{
				"application_fee_amount": 100,
				"status":                "paid",
			},
			"source_transaction": map[string]interface{}{"id": "txn_src"},
			"source_type":        "card",
		})
	}))
	defer server.Close()

	tr := &transfer.Transfer{
		Amount:      currency.Cents(2000),
		Currency:    "usd",
		AffiliateId: "aff_1",
		PartnerId:   "part_1",
		FeeId:       "fee_1",
	}
	tr.Destination = "acct_dest"
	tr.Description = "Test transfer"
	tr.Id_ = "tr_new"

	_, err := c.Transfer(tr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_Transfer_Error(t *testing.T) {
	c, server := makeTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{"message": "Invalid destination"},
		})
	}))
	defer server.Close()

	tr := &transfer.Transfer{
		Amount:   currency.Cents(1000),
		Currency: "usd",
	}
	tr.Destination = "acct_bad"
	tr.Id_ = "tr_err"

	_, err := c.Transfer(tr)
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---------------------------------------------------------------------------
// Client.Payout
// ---------------------------------------------------------------------------

func TestClient_Payout_Success(t *testing.T) {
	c, server := makeTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":                   "po_new",
			"object":               "payout",
			"amount":               5000,
			"currency":             "usd",
			"livemode":             true,
			"arrival_date":         1700000000,
			"created":              1699999000,
			"statement_descriptor": "Payout test",
			"destination":          map[string]interface{}{"id": "ba_dest", "type": "bank_account"},
			"source_type":          "card",
			"type":                 "card",
			"status":               "paid",
		})
	}))
	defer server.Close()

	tr := &transfer.Transfer{
		Amount:      currency.Cents(5000),
		Currency:    "usd",
		AffiliateId: "aff_2",
		PartnerId:   "part_2",
		FeeId:       "fee_2",
	}
	tr.Destination = "ba_dest"
	tr.Description = "Payout test"
	tr.Id_ = "po_new"

	_, err := c.Payout(tr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_Payout_Error(t *testing.T) {
	c, server := makeTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{"message": "Invalid destination"},
		})
	}))
	defer server.Close()

	tr := &transfer.Transfer{
		Amount:   currency.Cents(1000),
		Currency: "usd",
	}
	tr.Destination = "ba_bad"
	tr.Id_ = "po_err"

	_, err := c.Payout(tr)
	if err == nil {
		t.Fatal("expected error")
	}
}
