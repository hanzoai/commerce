package billing

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/paymentmethod"
	storemodel "github.com/hanzoai/commerce/models/store"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/util/test/ae"
)

// TRUE card-on-file monthly subscription (POST /v1/billing/subscribe/card).
// These pin the money invariants Red verifies:
//
//	nonce is VAULTED, the SAVED card (card-on-file id) is what's charged
//	the charge amount is the SERVER-AUTHORITATIVE catalog price (never a client value)
//	the subscription is created + a PAID first invoice references the processor ref
//	the plan fee is NOT credited to the spendable AI-credit wallet
//	a retry / double-submit charges + subscribes AT MOST once (idempotent)
//	a declined card creates NO subscription and NO saved payment method
//	renewal re-charges the SAME vaulted card; a decline leaves the invoice open
//	                                            and never double-charges

// withFakeSquare points the saved-card processor seam (processorsForOrg) at a
// registry holding m for the duration of the test — Square with real behavior,
// no network. DefaultConfig prefers Square (and disables Stripe), so SelectProcessor
// picks m for a USD card charge and squareCustomerProcessorFrom(reg) resolves it.
func withFakeSquare(t *testing.T, m *mockSquareProcessor) {
	t.Helper()
	old := processorsForOrg
	processorsForOrg = func(*organization.Organization) *processor.Registry {
		reg := processor.NewRegistry(processor.DefaultConfig())
		reg.Register(m)
		return reg
	}
	t.Cleanup(func() { processorsForOrg = old })
}

// squareMock builds a Square mock that vaults to (customerID, cardID) and charges
// to chargeRef.
func squareMock(customerID, cardID, chargeRef string) *mockSquareProcessor {
	m := newMockSquare(nil, "", nil)
	m.createdCustomerID = customerID
	m.addedCardID = cardID
	m.chargeRef = chargeRef
	return m
}

func invokeSubscribeCard(org *organization.Organization, ctx context.Context, body string, headers map[string]string) *http.Response {
	req := httptest.NewRequest(http.MethodPost, "/v1/billing/subscribe/card", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return driveSeeded(func(c *zip.Ctx) {
		c.Locals("organization", org)
		c.SetContext(ctx)
		c.Locals("iam_email", "buyer@acme.test")
	}, "/v1/billing/subscribe/card", req, SubscribeWithCard)
}

func jsonBody(t *testing.T, r *http.Response) map[string]any {
	t.Helper()
	raw, _ := io.ReadAll(r.Body)
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("bad response json: %v (%s)", err, string(raw))
	}
	return out
}

// parentSub returns the non-bundle subscription for subject on planId (the paid
// parent, not a $0 bundle child).
func parentSub(t *testing.T, db *datastore.Datastore, subject, planId string) *subscription.Subscription {
	t.Helper()
	subs := make([]*subscription.Subscription, 0)
	if _, err := subscription.Query(db).Filter("UserId=", subject).GetAll(&subs); err != nil {
		t.Fatalf("query subscriptions: %v", err)
	}
	for _, s := range subs {
		if s.ProviderType != "bundle" && (s.PlanId == planId || s.Plan.Slug == planId) {
			return s
		}
	}
	return nil
}

func pmsFor(t *testing.T, db *datastore.Datastore, subject string) []*paymentmethod.PaymentMethod {
	t.Helper()
	rootKey := db.NewKey("synckey", "", 1, nil)
	out := make([]*paymentmethod.PaymentMethod, 0)
	if _, err := paymentmethod.Query(db).Ancestor(rootKey).Filter("CustomerId=", subject).GetAll(&out); err != nil {
		t.Fatalf("query payment methods: %v", err)
	}
	return out
}

func invoicesForSub(t *testing.T, db *datastore.Datastore, subID string) []*billinginvoice.BillingInvoice {
	t.Helper()
	rootKey := db.NewKey("synckey", "", 1, nil)
	out := make([]*billinginvoice.BillingInvoice, 0)
	if _, err := billinginvoice.Query(db).Ancestor(rootKey).Filter("SubscriptionId=", subID).GetAll(&out); err != nil {
		t.Fatalf("query invoices: %v", err)
	}
	return out
}

// TestSubscribeWithCard_VaultChargeCreateInvoice is the end-to-end happy path:
// vault the nonce, charge the SAVED card at the catalog price, create the
// subscription, record a PAID first invoice — and leave the spendable wallet
// untouched.
func TestSubscribeWithCard_VaultChargeCreateInvoice(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("subcard")
	m := squareMock("cust_1", "ccof_1", "sqpay_1")
	withFakeSquare(t, m)

	// pro = $20/mo (2000 cents), flat. Body carries no amount — the price is the
	// catalog's, always.
	resp := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:ok","planId":"pro"}`, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status=%d body=%s, want 201", resp.StatusCode, func() string { b, _ := io.ReadAll(resp.Body); return string(b) }())
	}
	out := jsonBody(t, resp)
	if out["amountCents"].(float64) != 2000 {
		t.Fatalf("response amountCents=%v, want 2000 (catalog pro price)", out["amountCents"])
	}
	if out["subscriptionId"] == "" || out["invoiceId"] == "" {
		t.Fatalf("response missing ids: %+v", out)
	}

	// The NONCE was vaulted, the SAVED card-on-file id was charged (never the nonce),
	// in the Square customer's context, for the catalog price — exactly once.
	if m.createCustomerCalls != 1 {
		t.Fatalf("CreateCustomer calls=%d, want 1", m.createCustomerCalls)
	}
	if m.addCardNonce != "cnon:ok" {
		t.Fatalf("vaulted nonce=%q, want cnon:ok", m.addCardNonce)
	}
	if m.chargeCalls != 1 {
		t.Fatalf("charge calls=%d, want 1", m.chargeCalls)
	}
	if m.lastChargeToken != "ccof_1" {
		t.Fatalf("charged token=%q, want the card-on-file id ccof_1 (NOT the nonce)", m.lastChargeToken)
	}
	if m.lastChargeCustomer != "cust_1" {
		t.Fatalf("charge customer=%q, want cust_1", m.lastChargeCustomer)
	}
	if m.lastChargeAmount != 2000 {
		t.Fatalf("charged amount=%d, want 2000 (server-authoritative)", m.lastChargeAmount)
	}

	db := datastore.New(org.Namespaced(ctx))

	// A vaulted payment method persisted: ProviderRef = card-on-file id, Square
	// customer id in metadata (so renewals can re-charge it), default.
	pms := pmsFor(t, db, "subcard")
	if len(pms) != 1 {
		t.Fatalf("payment methods for subject=%d, want 1", len(pms))
	}
	if pms[0].ProviderRef != "ccof_1" {
		t.Fatalf("pm.ProviderRef=%q, want ccof_1 (card-on-file id, not the nonce)", pms[0].ProviderRef)
	}
	if squareCustomerIDOf(pms[0]) != "cust_1" {
		t.Fatalf("pm squareCustomerId=%q, want cust_1", squareCustomerIDOf(pms[0]))
	}
	if !pms[0].IsDefault {
		t.Fatal("vaulted card should be the default payment method")
	}

	// The subscription: active, Square-backed, pointing at the vaulted card.
	sub := parentSub(t, db, "subcard", "pro")
	if sub == nil {
		t.Fatal("no 'pro' subscription created for subject")
	}
	if sub.Status != subscription.Active {
		t.Fatalf("subscription status=%s, want active", sub.Status)
	}
	if sub.ProviderType != string(processor.Square) {
		t.Fatalf("subscription providerType=%q, want square (payment-backed → allotment flows)", sub.ProviderType)
	}
	if sub.DefaultPaymentMethod != pms[0].Id() {
		t.Fatalf("subscription DefaultPaymentMethod=%q, want the vaulted pm id %q", sub.DefaultPaymentMethod, pms[0].Id())
	}
	if sub.CurrentInvoiceId == "" {
		t.Fatal("subscription CurrentInvoiceId empty — first invoice not linked (would clamp the allotment)")
	}

	// A PAID first invoice referencing the processor ref.
	invs := invoicesForSub(t, db, sub.Id())
	if len(invs) != 1 {
		t.Fatalf("invoices for subscription=%d, want exactly 1", len(invs))
	}
	inv := invs[0]
	if inv.Status != billinginvoice.Paid {
		t.Fatalf("first invoice status=%s, want paid", inv.Status)
	}
	if inv.PaymentMethod != "card" {
		t.Fatalf("first invoice paymentMethod=%q, want card", inv.PaymentMethod)
	}
	if inv.PaymentRef != "sqpay_1" {
		t.Fatalf("first invoice paymentRef=%q, want the charge ref sqpay_1", inv.PaymentRef)
	}
	if inv.AmountPaid != 2000 || inv.AmountDue != 2000 {
		t.Fatalf("first invoice amountDue=%d amountPaid=%d, want 2000/2000", inv.AmountDue, inv.AmountPaid)
	}

	// The plan FEE must NOT have been credited to the spendable AI-credit wallet —
	// a subscription is not a top-up. The wallet the LLM gate reads stays $0.
	if bal := balanceOf(t, ctx, org, "subcard"); bal != 0 {
		t.Fatalf("spendable wallet balance=%d, want 0 — the plan fee must NEVER credit the AI-credit wallet", bal)
	}
}

func TestSubscribeWithCardBindsStore(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("substore")
	db := datastore.New(org.Namespaced(ctx))
	store := storemodel.New(db)
	store.Name = "Store A"
	store.Slug = "store-a"
	if err := store.Create(); err != nil {
		t.Fatalf("create store: %v", err)
	}

	m := squareMock("cust_store", "ccof_store", "sqpay_store")
	withFakeSquare(t, m)
	body := fmt.Sprintf(`{"sourceId":"cnon:store","planId":"pro","storeId":%q}`, store.Id())
	resp := invokeSubscribeCard(org, ctx, body, map[string]string{"X-Idempotency-Key": "store-a-first-period"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status=%d body=%s", resp.StatusCode, func() string { b, _ := io.ReadAll(resp.Body); return string(b) }())
	}
	sub := parentSub(t, db, "substore", "pro")
	if sub == nil || sub.StoreId != store.Id() {
		t.Fatalf("subscription store = %v, want %q", sub, store.Id())
	}
}

// TestSubscribeWithCard_ServerAuthoritativePrice proves the charge is the catalog
// price and a client-supplied amount is ignored (the request has no amount field,
// so a hostile amountCents in the body cannot underpay/overcharge).
func TestSubscribeWithCard_ServerAuthoritativePrice(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("sc-price")
	m := squareMock("cust_p", "ccof_p", "sqpay_p")
	withFakeSquare(t, m)

	// A hostile client tries to pay 1 cent for the $200 "max" plan. The amount is
	// dropped (unknown field) and the server charges the catalog price.
	resp := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:ok","planId":"max","amountCents":1,"amount":"0.01","price":1}`, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status=%d body=%s, want 201", resp.StatusCode, func() string { b, _ := io.ReadAll(resp.Body); return string(b) }())
	}
	if m.lastChargeAmount != 20000 {
		t.Fatalf("charged amount=%d, want 20000 (catalog 'max' price) — a client amount must NEVER set the charge", m.lastChargeAmount)
	}
	out := jsonBody(t, resp)
	if out["amountCents"].(float64) != 20000 {
		t.Fatalf("response amountCents=%v, want 20000", out["amountCents"])
	}
}

// TestSubscribeWithCard_ForeignCurrencyRejected proves the currency is
// SERVER-authoritative: a client that swaps the currency to a weaker unit
// ("jpy" for the USD-priced $20 pro plan) is rejected 400 with NO charge and NO
// subscription — otherwise 2000 JPY (~$13) would settle the $20 plan (underpay).
func TestSubscribeWithCard_ForeignCurrencyRejected(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("sc-jpy")
	m := squareMock("cust_j", "ccof_j", "sqpay_j")
	withFakeSquare(t, m)

	resp := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:ok","planId":"pro","currency":"jpy"}`, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s, want 400 (foreign currency rejected)", resp.StatusCode, func() string { b, _ := io.ReadAll(resp.Body); return string(b) }())
	}
	if m.chargeCalls != 0 {
		t.Fatalf("charge calls=%d for a rejected currency, want 0 — no money must move", m.chargeCalls)
	}
	if m.createCustomerCalls != 0 {
		t.Fatalf("CreateCustomer calls=%d for a rejected currency, want 0 (reject BEFORE vaulting)", m.createCustomerCalls)
	}
	db := datastore.New(org.Namespaced(ctx))
	if sub := parentSub(t, db, "sc-jpy", "pro"); sub != nil {
		t.Fatal("a rejected-currency request must NOT create a subscription")
	}
}

// TestSubscribeWithCard_Idempotent proves a retry with the same idempotency key
// charges + subscribes AT MOST once: the second call replays the first response and
// the card is charged exactly once.
func TestSubscribeWithCard_Idempotent(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("sc-idem")
	m := squareMock("cust_i", "ccof_i", "sqpay_i")
	withFakeSquare(t, m)

	hdr := map[string]string{"X-Idempotency-Key": "sub-key-1"}
	body := `{"sourceId":"cnon:ok","planId":"pro"}`

	r1 := invokeSubscribeCard(org, ctx, body, hdr)
	if r1.StatusCode != http.StatusCreated {
		t.Fatalf("first status=%d, want 201", r1.StatusCode)
	}
	b1, _ := io.ReadAll(r1.Body)

	// Retry (same key) — must replay, not re-charge.
	r2 := invokeSubscribeCard(org, ctx, body, hdr)
	if r2.StatusCode != http.StatusOK {
		t.Fatalf("replay status=%d, want 200 (idempotent replay)", r2.StatusCode)
	}
	b2, _ := io.ReadAll(r2.Body)
	if string(b1) != string(b2) {
		t.Fatalf("replay body differs:\n first=%s\n second=%s", b1, b2)
	}

	if m.chargeCalls != 1 {
		t.Fatalf("charge calls=%d after retry, want 1 — DOUBLE-CHARGE on idempotent retry", m.chargeCalls)
	}
	if m.createCustomerCalls != 1 {
		t.Fatalf("CreateCustomer calls=%d after retry, want 1 (no second vault)", m.createCustomerCalls)
	}

	// Exactly one paid invoice for the subject's subscription (no duplicate).
	db := datastore.New(org.Namespaced(ctx))
	sub := parentSub(t, db, "sc-idem", "pro")
	if sub == nil {
		t.Fatal("subscription not created")
	}
	if invs := invoicesForSub(t, db, sub.Id()); len(invs) != 1 {
		t.Fatalf("invoices=%d after retry, want 1", len(invs))
	}
}

// TestSubscribeWithCard_DeclinedNoSubscription proves a declined card creates NO
// subscription and NO saved payment method — money-in fails closed.
func TestSubscribeWithCard_DeclinedNoSubscription(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("sc-decline")
	m := squareMock("cust_d", "ccof_d", "")
	m.chargeErr = errors.New("CARD_DECLINED")
	withFakeSquare(t, m)

	resp := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:bad","planId":"pro"}`, nil)
	if resp.StatusCode != http.StatusPaymentRequired {
		t.Fatalf("status=%d, want 402 (declined)", resp.StatusCode)
	}
	if m.chargeCalls != 1 {
		t.Fatalf("charge calls=%d, want 1", m.chargeCalls)
	}

	db := datastore.New(org.Namespaced(ctx))
	if sub := parentSub(t, db, "sc-decline", "pro"); sub != nil {
		t.Fatal("a declined card must NOT create a subscription")
	}
	if pms := pmsFor(t, db, "sc-decline"); len(pms) != 0 {
		t.Fatalf("declined card persisted %d payment methods, want 0", len(pms))
	}
}

// TestSubscribeWithCard_FreePlanRejected proves the endpoint refuses a $0 plan (no
// card charge to make — free tiers are self-serve via POST /v1/billing/subscriptions).
func TestSubscribeWithCard_FreePlanRejected(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("sc-free")
	m := squareMock("cust_f", "ccof_f", "sqpay_f")
	withFakeSquare(t, m)

	resp := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:ok","planId":"developer"}`, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400 (free plan needs no card)", resp.StatusCode)
	}
	if m.chargeCalls != 0 {
		t.Fatalf("charge calls=%d for a free plan, want 0", m.chargeCalls)
	}
}

// seedCardBackedSub creates a vaulted card + an active subscription (its plan from
// the real catalog) pointing at it — the setup a renewal collects against. It uses
// the SAME plan.New + StartSubscription path production does, so sub.Plan is a fully
// initialized entity (subscriptionResponse reads sub.Plan.Id()).
func seedCardBackedSub(t *testing.T, db *datastore.Datastore, subject, planSlug, cardID, customerID string) *subscription.Subscription {
	t.Helper()
	pm := paymentmethod.New(db)
	pm.CustomerId = subject
	pm.UserId = subject
	pm.Type = "card"
	pm.ProviderRef = cardID
	pm.ProviderType = string(processor.Square)
	pm.IsDefault = true
	pm.Metadata = map[string]interface{}{"squareCustomerId": customerID, "squareCardId": cardID}
	if err := pm.Create(); err != nil {
		t.Fatalf("seed payment method: %v", err)
	}

	p, err := resolveSubscriptionPlan(db, planSlug)
	if err != nil {
		t.Fatalf("resolve plan %q: %v", planSlug, err)
	}
	s := subscription.New(db)
	s.UserId = subject
	s.DefaultPaymentMethod = pm.Id()
	s.ProviderType = string(processor.Square)
	s.Quantity = 1
	engine.StartSubscription(s, p) // sets s.Plan, s.PlanId, Status=Active, current period
	// Make the current period DUE (already ended) so a renewal actually collects —
	// the is-due gate only bills an elapsed period (never a future one).
	s.PeriodStart = time.Now().AddDate(0, -2, 0)
	s.PeriodEnd = time.Now().AddDate(0, -1, 0)
	if err := s.Create(); err != nil {
		t.Fatalf("seed subscription: %v", err)
	}
	return s
}

func invokeRenew(org *organization.Organization, ctx context.Context, subID string) *http.Response {
	req := httptest.NewRequest(http.MethodPost, "/v1/billing/subscriptions/"+subID+"/renew", nil)
	return driveSeeded(func(c *zip.Ctx) {
		c.Locals("organization", org)
		c.SetContext(ctx)
	}, "/v1/billing/subscriptions/:id/renew", req, RenewBillingSubscription)
}

func invokePay(org *organization.Organization, ctx context.Context, invID string) *http.Response {
	req := httptest.NewRequest(http.MethodPost, "/v1/billing/invoices/"+invID+"/pay", nil)
	return driveSeeded(func(c *zip.Ctx) {
		c.Locals("organization", org)
		c.SetContext(ctx)
	}, "/v1/billing/invoices/:id/pay", req, PayInvoice)
}

// seedOpenInvoice persists one OPEN invoice (a single subscription line item) for
// the subscription's period — what a PayInvoice / dunning retry collects against.
func seedOpenInvoice(t *testing.T, db *datastore.Datastore, sub *subscription.Subscription, amountCents int64) *billinginvoice.BillingInvoice {
	t.Helper()
	inv := billinginvoice.New(db)
	inv.UserId = sub.UserId
	inv.SubscriptionId = sub.Id()
	inv.PeriodStart = sub.PeriodStart
	inv.PeriodEnd = sub.PeriodEnd
	inv.Currency = currency.USD
	inv.LineItems = []billinginvoice.LineItem{{
		Id: "li_plan", Type: billinginvoice.LineSubscription, Amount: amountCents, Currency: currency.USD,
	}}
	inv.RecalculateSubtotal()
	inv.SetNumber(1)
	if err := inv.Finalize(); err != nil {
		t.Fatalf("finalize invoice: %v", err)
	}
	if err := inv.Create(); err != nil {
		t.Fatalf("create invoice: %v", err)
	}
	return inv
}

// TestRenewSubscription_ChargesVaultedCard proves a manual renewal collects the
// period fee from the subscription's SAVED card and marks the invoice paid.
func TestRenewSubscription_ChargesVaultedCard(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("sc-renew")
	m := squareMock("", "", "sqpay_renew")
	withFakeSquare(t, m)

	db := datastore.New(org.Namespaced(ctx))
	sub := seedCardBackedSub(t, db, "sc-renew", "pro", "ccof_r", "cust_r")

	resp := invokeRenew(org, ctx, sub.Id())
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("renew status=%d body=%s, want 200", resp.StatusCode, func() string { b, _ := io.ReadAll(resp.Body); return string(b) }())
	}
	if m.chargeCalls != 1 {
		t.Fatalf("charge calls=%d, want 1", m.chargeCalls)
	}
	if m.lastChargeToken != "ccof_r" {
		t.Fatalf("renewal charged token=%q, want the vaulted card ccof_r", m.lastChargeToken)
	}
	if m.lastChargeCustomer != "cust_r" {
		t.Fatalf("renewal charge customer=%q, want cust_r", m.lastChargeCustomer)
	}
	if m.lastChargeAmount != 2000 {
		t.Fatalf("renewal charged amount=%d, want 2000", m.lastChargeAmount)
	}

	invs := invoicesForSub(t, db, sub.Id())
	if len(invs) != 1 {
		t.Fatalf("invoices=%d, want 1", len(invs))
	}
	if invs[0].Status != billinginvoice.Paid || invs[0].PaymentMethod != "card" {
		t.Fatalf("renewal invoice status=%s method=%q, want paid/card", invs[0].Status, invs[0].PaymentMethod)
	}
}

// TestRenewSubscription_DeclineLeavesOpenNoDoubleCharge proves a declined renewal
// leaves the invoice OPEN (subscription PastDue) and a repeated renew does NOT
// re-charge (the existing open invoice is returned; dunning, not renew, retries).
func TestRenewSubscription_DeclineLeavesOpenNoDoubleCharge(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("sc-renew-decline")
	m := squareMock("", "", "")
	m.chargeErr = errors.New("CARD_DECLINED")
	withFakeSquare(t, m)

	db := datastore.New(org.Namespaced(ctx))
	sub := seedCardBackedSub(t, db, "sc-renew-decline", "pro", "ccof_rd", "cust_rd")

	// First renew: declines → invoice OPEN, subscription PastDue, charged once.
	if resp := invokeRenew(org, ctx, sub.Id()); resp.StatusCode != http.StatusOK {
		t.Fatalf("first renew status=%d, want 200 (a decline is a handled outcome, not a 500)", resp.StatusCode)
	}
	if m.chargeCalls != 1 {
		t.Fatalf("charge calls after first renew=%d, want 1", m.chargeCalls)
	}
	invs := invoicesForSub(t, db, sub.Id())
	if len(invs) != 1 || invs[0].Status != billinginvoice.Open {
		t.Fatalf("after decline: %d invoices, status=%v; want 1 OPEN invoice", len(invs), func() billinginvoice.Status {
			if len(invs) > 0 {
				return invs[0].Status
			}
			return ""
		}())
	}
	got := subscription.New(db)
	if err := got.GetById(sub.Id()); err != nil {
		t.Fatalf("reload subscription: %v", err)
	}
	if got.Status != subscription.PastDue {
		t.Fatalf("subscription status after decline=%s, want past_due", got.Status)
	}

	// Second renew on the SAME (unchanged) period: the existing open invoice is
	// returned WITHOUT a second charge — no double-charge.
	if resp := invokeRenew(org, ctx, got.Id()); resp.StatusCode != http.StatusOK {
		t.Fatalf("second renew status=%d, want 200", resp.StatusCode)
	}
	if m.chargeCalls != 1 {
		t.Fatalf("charge calls after second renew=%d, want STILL 1 — a repeated renew must NOT re-charge the open period", m.chargeCalls)
	}
	if invs := invoicesForSub(t, db, got.Id()); len(invs) != 1 {
		t.Fatalf("invoices after second renew=%d, want 1 (no duplicate)", len(invs))
	}
}

// TestRenewSubscription_ParallelExactlyOneCharge drives N concurrent renewals of
// the SAME due subscription and proves the vaulted card is charged EXACTLY once —
// the per-period guard + the stable per-period Square idempotency key together
// defeat the TOCTOU double-charge.
func TestRenewSubscription_ParallelExactlyOneCharge(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("sc-par-renew")
	m := squareMock("", "", "sqpay_parren")
	withFakeSquare(t, m)

	db := datastore.New(org.Namespaced(ctx))
	sub := seedCardBackedSub(t, db, "sc-par-renew", "pro", "ccof_pr", "cust_pr")
	charger := chargeProviderForOrg(org)

	const N = 8
	var wg sync.WaitGroup
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Each goroutine renews its OWN loaded instance (no shared struct races).
			s := subscription.New(db)
			if err := s.GetById(sub.Id()); err != nil {
				return
			}
			_, _, _ = engine.RenewSubscription(ctx, db, s, BurnCredits, charger)
		}()
	}
	wg.Wait()

	if m.chargeCalls != 1 {
		t.Fatalf("parallel renew charged the card %d times, want exactly 1 (double-charge under concurrency)", m.chargeCalls)
	}
	// Books integrity: exactly ONE invoice row for the period — the deterministic
	// per-period id collapses concurrent builds, so the invoice ledger / MRR can't
	// over-count by (N-1)×Price even though several racers passed the local guard.
	if invs := invoicesForSub(t, db, sub.Id()); len(invs) != 1 {
		t.Fatalf("parallel renew created %d invoice rows for the period, want exactly 1 (ledger over-count)", len(invs))
	}
}

// TestPayInvoice_ConcurrentCollect_OneCharge seeds an OPEN invoice for a
// card-backed subscription and runs N concurrent collections; the stable
// per-period Square idempotency key makes the vaulted-card charge exactly once.
func TestPayInvoice_ConcurrentCollect_OneCharge(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("sc-par-pay")
	m := squareMock("", "", "sqpay_parpay")
	withFakeSquare(t, m)

	db := datastore.New(org.Namespaced(ctx))
	sub := seedCardBackedSub(t, db, "sc-par-pay", "pro", "ccof_pp", "cust_pp")
	inv := seedOpenInvoice(t, db, sub, 2000)

	charger := chargeProviderForOrg(org)
	const N = 8
	var wg sync.WaitGroup
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			i2 := billinginvoice.New(db)
			if err := i2.GetById(inv.Id()); err != nil {
				return
			}
			if i2.Status != billinginvoice.Open {
				return
			}
			_, _ = engine.CollectInvoice(ctx, db, i2, BurnCredits, charger)
		}()
	}
	wg.Wait()

	if m.chargeCalls != 1 {
		t.Fatalf("concurrent invoice collection charged the card %d times, want exactly 1", m.chargeCalls)
	}
}

// TestSubscribeWithCard_NewNonceRetry_OneCharge proves the server-side (subject,
// planId) fallback guard: a retry with a DIFFERENT single-use nonce and NO
// X-Idempotency-Key (the exact SPA "Start over → re-tokenize" shape) charges +
// subscribes exactly once, not twice.
func TestSubscribeWithCard_NewNonceRetry_OneCharge(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("sc-newnonce")
	m := squareMock("cust_nn", "ccof_nn", "sqpay_nn")
	withFakeSquare(t, m)

	// First attempt (nonce A), no X-Idempotency-Key → guard falls back to (subject, plan).
	r1 := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:A","planId":"pro"}`, nil)
	if r1.StatusCode != http.StatusCreated {
		t.Fatalf("first status=%d, want 201", r1.StatusCode)
	}
	// Retry with a FRESH nonce B (re-tokenized), still no header.
	r2 := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:B","planId":"pro"}`, nil)
	if r2.StatusCode != http.StatusOK {
		t.Fatalf("new-nonce retry status=%d, want 200 (idempotent replay on (subject,planId))", r2.StatusCode)
	}

	if m.chargeCalls != 1 {
		t.Fatalf("new-nonce retry charged %d times, want 1 — a re-tokenized retry double-charged", m.chargeCalls)
	}
	db := datastore.New(org.Namespaced(ctx))
	subs := make([]*subscription.Subscription, 0)
	if _, err := subscription.Query(db).Filter("UserId=", "sc-newnonce").GetAll(&subs); err != nil {
		t.Fatalf("query subs: %v", err)
	}
	parents := 0
	for _, s := range subs {
		if s.ProviderType != "bundle" {
			parents++
		}
	}
	if parents != 1 {
		t.Fatalf("new-nonce retry created %d subscriptions, want 1", parents)
	}
}

// TestSubscribeWithCard_PerSeatQuantity proves per-seat first-period collection is
// exact: a per-seat plan charges Price × quantity and the recorded invoice's
// AmountDue == AmountPaid == that amount (never charge one seat yet record N).
func TestSubscribeWithCard_PerSeatQuantity(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("sc-seats")
	m := squareMock("cust_s", "ccof_s", "sqpay_s")
	withFakeSquare(t, m)

	// team = $25/mo (2500c) per seat, minSeats 2. quantity 2 → charge 5000c.
	resp := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:ok","planId":"team","quantity":2}`, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status=%d body=%s, want 201", resp.StatusCode, func() string { b, _ := io.ReadAll(resp.Body); return string(b) }())
	}
	out := jsonBody(t, resp)
	if out["amountCents"].(float64) != 5000 {
		t.Fatalf("response amountCents=%v, want 5000 (2500 x 2 seats)", out["amountCents"])
	}
	if m.lastChargeAmount != 5000 {
		t.Fatalf("charged amount=%d, want 5000 (Price x quantity)", m.lastChargeAmount)
	}

	db := datastore.New(org.Namespaced(ctx))
	sub := parentSub(t, db, "sc-seats", "team")
	if sub == nil {
		t.Fatal("no team subscription created")
	}
	invs := invoicesForSub(t, db, sub.Id())
	if len(invs) != 1 {
		t.Fatalf("invoices=%d, want 1", len(invs))
	}
	if invs[0].AmountDue != 5000 || invs[0].AmountPaid != 5000 {
		t.Fatalf("invoice amountDue=%d amountPaid=%d, want 5000/5000 (charge must equal the recorded paid amount)", invs[0].AmountDue, invs[0].AmountPaid)
	}
}

// TestPayInvoice_DeclineThenRetryReCollects proves fix A: a declined PayInvoice
// leaves the invoice OPEN, does NOT seal the guard, and a later PayInvoice (once the
// card works) actually RE-COLLECTS — dunning is not wedged by a sealed decline. The
// re-collection runs at a higher AttemptCount → a FRESH gateway key → the processor
// lets it through instead of replaying the first decline.
func TestPayInvoice_DeclineThenRetryReCollects(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("sc-pay-dun")
	m := squareMock("", "", "sqpay_dun")
	m.chargeErr = errors.New("CARD_DECLINED")
	withFakeSquare(t, m)

	db := datastore.New(org.Namespaced(ctx))
	sub := seedCardBackedSub(t, db, "sc-pay-dun", "pro", "ccof_dun", "cust_dun")
	inv := seedOpenInvoice(t, db, sub, 2000)

	// First pay: declines → invoice stays OPEN, guard released (not sealed).
	r1 := invokePay(org, ctx, inv.Id())
	if r1.StatusCode != http.StatusOK {
		t.Fatalf("declined pay status=%d, want 200 (a decline is a handled outcome)", r1.StatusCode)
	}
	if m.chargeCalls != 1 {
		t.Fatalf("charge calls after decline=%d, want 1", m.chargeCalls)
	}
	got := billinginvoice.New(db)
	if err := got.GetById(inv.Id()); err != nil {
		t.Fatalf("reload invoice: %v", err)
	}
	if got.Status != billinginvoice.Open {
		t.Fatalf("invoice status after decline=%s, want OPEN (a declined pay must not close the invoice)", got.Status)
	}

	// Card now works: a subsequent pay must actually RE-COLLECT (not replay the sealed
	// decline) and mark the invoice paid.
	m.chargeErr = nil
	r2 := invokePay(org, ctx, inv.Id())
	if r2.StatusCode != http.StatusOK {
		t.Fatalf("retry pay status=%d, want 200", r2.StatusCode)
	}
	if m.chargeCalls != 2 {
		t.Fatalf("charge calls after retry=%d, want 2 — the retry did NOT re-collect (dunning wedged by a sealed decline)", m.chargeCalls)
	}
	paid := billinginvoice.New(db)
	if err := paid.GetById(inv.Id()); err != nil {
		t.Fatalf("reload invoice: %v", err)
	}
	if paid.Status != billinginvoice.Paid || paid.PaymentMethod != "card" {
		t.Fatalf("invoice after retry status=%s method=%q, want paid/card", paid.Status, paid.PaymentMethod)
	}
}

// TestSubscribeWithCard_DistinctKeyReSubscribes proves fix B: a genuine second
// purchase with a DIFFERENT idempotency key is NOT replayed as the first — it creates
// a new subscription and charges again (the guard is not a permanent per-plan lock).
// The SPA reaches this by clearing its per-attempt sessionStorage key on success.
func TestSubscribeWithCard_DistinctKeyReSubscribes(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("sc-resub")
	m := squareMock("cust_rs", "ccof_rs", "sqpay_rs")
	withFakeSquare(t, m)

	r1 := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:A","planId":"pro"}`, map[string]string{"X-Idempotency-Key": "attempt-1"})
	if r1.StatusCode != http.StatusCreated {
		t.Fatalf("first subscribe status=%d, want 201", r1.StatusCode)
	}
	// New checkout attempt (fresh key) → a NEW subscription, not a replay.
	r2 := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:B","planId":"pro"}`, map[string]string{"X-Idempotency-Key": "attempt-2"})
	if r2.StatusCode != http.StatusCreated {
		t.Fatalf("re-subscribe status=%d, want 201 (a fresh key must NOT replay the first purchase)", r2.StatusCode)
	}
	if m.chargeCalls != 2 {
		t.Fatalf("charge calls=%d across two distinct-key subscribes, want 2", m.chargeCalls)
	}
	db := datastore.New(org.Namespaced(ctx))
	subs := make([]*subscription.Subscription, 0)
	if _, err := subscription.Query(db).Filter("UserId=", "sc-resub").GetAll(&subs); err != nil {
		t.Fatalf("query subs: %v", err)
	}
	parents := 0
	for _, s := range subs {
		if s.ProviderType != "bundle" {
			parents++
		}
	}
	if parents != 2 {
		t.Fatalf("distinct-key re-subscribe created %d parent subscriptions, want 2 (guard wrongly blocked the second)", parents)
	}
}
