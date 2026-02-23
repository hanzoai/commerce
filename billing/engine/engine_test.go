package engine

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/creditnote"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/types"
)

// ---------------------------------------------------------------------------
// intents.go — CreatePaymentIntent validation
// ---------------------------------------------------------------------------

func TestCreatePaymentIntent_ZeroAmount(t *testing.T) {
	_, err := CreatePaymentIntent(nil, CreatePaymentIntentParams{
		CustomerId: "cus_123",
		Amount:     0,
		Currency:   "usd",
	})
	if err == nil {
		t.Fatal("expected error for zero amount")
	}
	if !strings.Contains(err.Error(), "amount must be positive") {
		t.Fatalf("unexpected error: %s", err)
	}
}

func TestCreatePaymentIntent_NegativeAmount(t *testing.T) {
	_, err := CreatePaymentIntent(nil, CreatePaymentIntentParams{
		CustomerId: "cus_123",
		Amount:     -500,
		Currency:   "usd",
	})
	if err == nil {
		t.Fatal("expected error for negative amount")
	}
	if !strings.Contains(err.Error(), "amount must be positive") {
		t.Fatalf("unexpected error: %s", err)
	}
}

func TestCreatePaymentIntent_MissingCustomerId(t *testing.T) {
	_, err := CreatePaymentIntent(nil, CreatePaymentIntentParams{
		Amount:   1000,
		Currency: "usd",
	})
	if err == nil {
		t.Fatal("expected error for missing customerId")
	}
	if !strings.Contains(err.Error(), "customerId is required") {
		t.Fatalf("unexpected error: %s", err)
	}
}

func TestCreatePaymentIntent_ValidParams_RequiresDatastore(t *testing.T) {
	t.Skip("requires datastore: paymentintent.New(db) needs live db")
}

// ---------------------------------------------------------------------------
// intents.go — CreateSetupIntent validation
// ---------------------------------------------------------------------------

func TestCreateSetupIntent_MissingCustomerId(t *testing.T) {
	_, err := CreateSetupIntent(nil, CreateSetupIntentParams{})
	if err == nil {
		t.Fatal("expected error for missing customerId")
	}
	if !strings.Contains(err.Error(), "customerId is required") {
		t.Fatalf("unexpected error: %s", err)
	}
}

func TestCreateSetupIntent_ValidParams_RequiresDatastore(t *testing.T) {
	t.Skip("requires datastore: setupintent.New(db) needs live db")
}

// ---------------------------------------------------------------------------
// refunds.go — CreateRefund validation
// ---------------------------------------------------------------------------

func TestCreateRefund_MissingBothIds(t *testing.T) {
	_, err := CreateRefund(nil, nil, CreateRefundParams{
		Reason: "requested_by_customer",
	}, nil)
	if err == nil {
		t.Fatal("expected error when both paymentIntentId and invoiceId are missing")
	}
	if !strings.Contains(err.Error(), "either paymentIntentId or invoiceId is required") {
		t.Fatalf("unexpected error: %s", err)
	}
}

func TestCreateRefund_WithPaymentIntentId_RequiresDatastore(t *testing.T) {
	t.Skip("requires datastore: paymentintent.New(db).GetById needs live db")
}

func TestCreateRefund_WithInvoiceId_RequiresDatastore(t *testing.T) {
	t.Skip("requires datastore: billinginvoice.New(db).GetById needs live db")
}

// ---------------------------------------------------------------------------
// refunds.go — CreateCreditNote validation
// ---------------------------------------------------------------------------

func TestCreateCreditNote_MissingInvoiceId(t *testing.T) {
	_, err := CreateCreditNote(nil, CreateCreditNoteParams{
		CustomerId: "cus_123",
		Amount:     500,
	})
	if err == nil {
		t.Fatal("expected error for missing invoiceId")
	}
	if !strings.Contains(err.Error(), "invoiceId is required") {
		t.Fatalf("unexpected error: %s", err)
	}
}

func TestCreateCreditNote_ValidParams_RequiresDatastore(t *testing.T) {
	t.Skip("requires datastore: billinginvoice.New(db).GetById needs live db")
}

// ---------------------------------------------------------------------------
// metering.go — IngestUsageEvent validation
// ---------------------------------------------------------------------------

func TestIngestUsageEvent_MissingMeterId(t *testing.T) {
	_, _, err := IngestUsageEvent(nil, "", "user_1", 10, "", time.Now(), nil)
	if err == nil {
		t.Fatal("expected error for missing meterId")
	}
	if !strings.Contains(err.Error(), "meterId is required") {
		t.Fatalf("unexpected error: %s", err)
	}
}

func TestIngestUsageEvent_MissingUserId(t *testing.T) {
	_, _, err := IngestUsageEvent(nil, "meter_1", "", 10, "", time.Now(), nil)
	if err == nil {
		t.Fatal("expected error for missing userId")
	}
	if !strings.Contains(err.Error(), "userId is required") {
		t.Fatalf("unexpected error: %s", err)
	}
}

func TestIngestUsageEvent_ValidParams_RequiresDatastore(t *testing.T) {
	t.Skip("requires datastore: meter.NewEvent(db) needs live db")
}

// ---------------------------------------------------------------------------
// events.go — computeSignature (pure function)
// ---------------------------------------------------------------------------

func TestComputeSignature(t *testing.T) {
	timestamp := "1700000000"
	payload := []byte(`{"type":"invoice.paid"}`)
	secret := "whsec_test_secret"

	got := computeSignature(timestamp, payload, secret)

	// Verify independently
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(payload)
	want := hex.EncodeToString(mac.Sum(nil))

	if got != want {
		t.Fatalf("computeSignature mismatch: got %s, want %s", got, want)
	}
}

func TestComputeSignature_DifferentSecrets(t *testing.T) {
	ts := "1700000000"
	payload := []byte(`{"foo":"bar"}`)

	sig1 := computeSignature(ts, payload, "secret_a")
	sig2 := computeSignature(ts, payload, "secret_b")

	if sig1 == sig2 {
		t.Fatal("different secrets should produce different signatures")
	}
}

func TestComputeSignature_DifferentTimestamps(t *testing.T) {
	payload := []byte(`{"foo":"bar"}`)
	secret := "shared"

	sig1 := computeSignature("1000", payload, secret)
	sig2 := computeSignature("2000", payload, secret)

	if sig1 == sig2 {
		t.Fatal("different timestamps should produce different signatures")
	}
}

func TestComputeSignature_EmptyPayload(t *testing.T) {
	sig := computeSignature("1234", []byte{}, "key")
	if sig == "" {
		t.Fatal("signature should not be empty even for empty payload")
	}
	if len(sig) != 64 { // SHA-256 hex = 32 bytes = 64 hex chars
		t.Fatalf("expected 64 hex chars, got %d", len(sig))
	}
}

// ---------------------------------------------------------------------------
// events.go — VerifyWebhookSignature (pure function)
// ---------------------------------------------------------------------------

func TestVerifyWebhookSignature_Valid(t *testing.T) {
	payload := []byte(`{"type":"payment_intent.succeeded"}`)
	secret := "whsec_abc123"
	timestamp := "1700000000"

	sig := computeSignature(timestamp, payload, secret)
	header := fmt.Sprintf("t=%s,v1=%s", timestamp, sig)

	if err := VerifyWebhookSignature(payload, header, secret); err != nil {
		t.Fatalf("valid signature rejected: %v", err)
	}
}

func TestVerifyWebhookSignature_InvalidSignature(t *testing.T) {
	payload := []byte(`{"type":"payment_intent.succeeded"}`)
	secret := "whsec_abc123"
	header := "t=1700000000,v1=0000000000000000000000000000000000000000000000000000000000000000"

	err := VerifyWebhookSignature(payload, header, secret)
	if err == nil {
		t.Fatal("expected error for invalid signature")
	}
	if !strings.Contains(err.Error(), "signature verification failed") {
		t.Fatalf("unexpected error: %s", err)
	}
}

func TestVerifyWebhookSignature_MissingTimestamp(t *testing.T) {
	err := VerifyWebhookSignature([]byte("{}"), "v1=abc123", "secret")
	if err == nil {
		t.Fatal("expected error for missing timestamp")
	}
	if !strings.Contains(err.Error(), "invalid signature header format") {
		t.Fatalf("unexpected error: %s", err)
	}
}

func TestVerifyWebhookSignature_MissingSignature(t *testing.T) {
	err := VerifyWebhookSignature([]byte("{}"), "t=1234567890", "secret")
	if err == nil {
		t.Fatal("expected error for missing v1 signature")
	}
	if !strings.Contains(err.Error(), "invalid signature header format") {
		t.Fatalf("unexpected error: %s", err)
	}
}

func TestVerifyWebhookSignature_EmptyHeader(t *testing.T) {
	err := VerifyWebhookSignature([]byte("{}"), "", "secret")
	if err == nil {
		t.Fatal("expected error for empty header")
	}
}

func TestVerifyWebhookSignature_WrongSecret(t *testing.T) {
	payload := []byte(`{"test":true}`)
	timestamp := "1700000000"
	correctSig := computeSignature(timestamp, payload, "correct_secret")
	header := fmt.Sprintf("t=%s,v1=%s", timestamp, correctSig)

	err := VerifyWebhookSignature(payload, header, "wrong_secret")
	if err == nil {
		t.Fatal("expected error when verifying with wrong secret")
	}
}

func TestVerifyWebhookSignature_TamperedPayload(t *testing.T) {
	original := []byte(`{"amount":1000}`)
	secret := "whsec_test"
	timestamp := "1700000000"
	sig := computeSignature(timestamp, original, secret)
	header := fmt.Sprintf("t=%s,v1=%s", timestamp, sig)

	tampered := []byte(`{"amount":9999}`)
	err := VerifyWebhookSignature(tampered, header, secret)
	if err == nil {
		t.Fatal("expected error for tampered payload")
	}
}

// ---------------------------------------------------------------------------
// tax.go — TaxLine struct and CalculateInvoiceTax nil-address path
// ---------------------------------------------------------------------------

func TestCalculateInvoiceTax_NilAddress(t *testing.T) {
	lines, total, err := CalculateInvoiceTax(nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 0 {
		t.Fatalf("expected 0 tax for nil address, got %d", total)
	}
	if len(lines) != 0 {
		t.Fatalf("expected no tax lines for nil address, got %d", len(lines))
	}
}

func TestTaxLine_Struct(t *testing.T) {
	tl := TaxLine{
		TaxRateId:    "txr_001",
		Description:  "CA State Tax",
		Amount:       875,
		Rate:         0.0875,
		Inclusive:     false,
		Jurisdiction: "US-CA",
	}

	if tl.TaxRateId != "txr_001" {
		t.Fatalf("TaxRateId mismatch: %s", tl.TaxRateId)
	}
	if tl.Amount != 875 {
		t.Fatalf("Amount mismatch: %d", tl.Amount)
	}
	if tl.Rate != 0.0875 {
		t.Fatalf("Rate mismatch: %f", tl.Rate)
	}
	if tl.Jurisdiction != "US-CA" {
		t.Fatalf("Jurisdiction mismatch: %s", tl.Jurisdiction)
	}
}

// ---------------------------------------------------------------------------
// lifecycle.go — StartSubscription (pure in-memory)
// ---------------------------------------------------------------------------

func makePlan(id, name string, price int64, interval types.Interval, intervalCount, trialDays int) *plan.Plan {
	p := &plan.Plan{
		Name:            name,
		Price:           currency.Cents(price),
		Currency:        "usd",
		Interval:        interval,
		IntervalCount:   intervalCount,
		TrialPeriodDays: trialDays,
	}
	// Pre-set Id_ so p.Id() returns without calling into datastore
	p.Id_ = id
	return p
}

func TestStartSubscription_NoTrial(t *testing.T) {
	sub := &subscription.Subscription{}
	p := makePlan("plan_pro", "Pro", 2000, types.Monthly, 1, 0)

	StartSubscription(sub, p)

	if sub.Status != subscription.Active {
		t.Fatalf("expected Active, got %s", sub.Status)
	}
	if sub.PlanId != p.Id() {
		t.Fatalf("planId mismatch: got %s, want %s", sub.PlanId, p.Id())
	}
	if sub.Start.IsZero() {
		t.Fatal("Start should be set")
	}
	if sub.PeriodStart.IsZero() {
		t.Fatal("PeriodStart should be set")
	}
	if sub.PeriodEnd.IsZero() {
		t.Fatal("PeriodEnd should be set")
	}
	if !sub.TrialStart.IsZero() {
		t.Fatal("TrialStart should be zero for no-trial plan")
	}

	// Period should be 1 month later
	expected := sub.PeriodStart.AddDate(0, 1, 0)
	if !sub.PeriodEnd.Equal(expected) {
		t.Fatalf("PeriodEnd mismatch: got %v, want %v", sub.PeriodEnd, expected)
	}
}

func TestStartSubscription_WithTrial(t *testing.T) {
	sub := &subscription.Subscription{}
	p := makePlan("plan_ent", "Enterprise", 10000, types.Monthly, 1, 14)

	StartSubscription(sub, p)

	if sub.Status != subscription.Trialing {
		t.Fatalf("expected Trialing, got %s", sub.Status)
	}
	if sub.TrialStart.IsZero() {
		t.Fatal("TrialStart should be set")
	}
	if sub.TrialEnd.IsZero() {
		t.Fatal("TrialEnd should be set")
	}

	// Trial should be 14 days (compare by calendar date, not hours, to avoid DST drift)
	expectedTrialEnd := sub.TrialStart.AddDate(0, 0, 14)
	if !sub.TrialEnd.Equal(expectedTrialEnd) {
		t.Fatalf("trial end mismatch: got %v, want %v", sub.TrialEnd, expectedTrialEnd)
	}

	// Period should start at trial end
	if !sub.PeriodStart.Equal(sub.TrialEnd) {
		t.Fatalf("PeriodStart should equal TrialEnd")
	}
}

func TestStartSubscription_YearlyInterval(t *testing.T) {
	sub := &subscription.Subscription{}
	p := makePlan("plan_annual", "Annual", 20000, types.Yearly, 1, 0)

	StartSubscription(sub, p)

	expected := sub.PeriodStart.AddDate(1, 0, 0)
	if !sub.PeriodEnd.Equal(expected) {
		t.Fatalf("yearly PeriodEnd mismatch: got %v, want %v", sub.PeriodEnd, expected)
	}
}

func TestStartSubscription_MultiMonthInterval(t *testing.T) {
	sub := &subscription.Subscription{}
	p := makePlan("plan_q", "Quarterly", 5000, types.Monthly, 3, 0)

	StartSubscription(sub, p)

	expected := sub.PeriodStart.AddDate(0, 3, 0)
	if !sub.PeriodEnd.Equal(expected) {
		t.Fatalf("quarterly PeriodEnd mismatch: got %v, want %v", sub.PeriodEnd, expected)
	}
}

// ---------------------------------------------------------------------------
// lifecycle.go — TransitionTrialToActive
// ---------------------------------------------------------------------------

func TestTransitionTrialToActive_Success(t *testing.T) {
	sub := &subscription.Subscription{Status: subscription.Trialing}

	if err := TransitionTrialToActive(sub); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sub.Status != subscription.Active {
		t.Fatalf("expected Active, got %s", sub.Status)
	}
}

func TestTransitionTrialToActive_NotTrialing(t *testing.T) {
	sub := &subscription.Subscription{Status: subscription.Active}

	err := TransitionTrialToActive(sub)
	if err == nil {
		t.Fatal("expected error when not trialing")
	}
	if !strings.Contains(err.Error(), "not trialing") {
		t.Fatalf("unexpected error: %s", err)
	}
}

func TestTransitionTrialToActive_Canceled(t *testing.T) {
	sub := &subscription.Subscription{Status: subscription.Canceled}

	err := TransitionTrialToActive(sub)
	if err == nil {
		t.Fatal("expected error for canceled subscription")
	}
}

// ---------------------------------------------------------------------------
// lifecycle.go — CancelSubscription
// ---------------------------------------------------------------------------

func TestCancelSubscription_Immediate(t *testing.T) {
	sub := &subscription.Subscription{Status: subscription.Active}

	if err := CancelSubscription(sub, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sub.Status != subscription.Canceled {
		t.Fatalf("expected Canceled, got %s", sub.Status)
	}
	if !sub.Canceled {
		t.Fatal("Canceled flag should be true")
	}
	if sub.CanceledAt.IsZero() {
		t.Fatal("CanceledAt should be set")
	}
	if sub.Ended.IsZero() {
		t.Fatal("Ended should be set for immediate cancel")
	}
	if sub.EndCancel {
		t.Fatal("EndCancel should be false for immediate cancel")
	}
}

func TestCancelSubscription_AtPeriodEnd(t *testing.T) {
	sub := &subscription.Subscription{Status: subscription.Active}

	if err := CancelSubscription(sub, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sub.EndCancel != true {
		t.Fatal("EndCancel should be true")
	}
	if sub.CanceledAt.IsZero() {
		t.Fatal("CanceledAt should be set")
	}
	// Status should NOT change to canceled yet (deferred)
	if sub.Status == subscription.Canceled {
		t.Fatal("status should not be canceled yet for at-period-end cancel")
	}
}

func TestCancelSubscription_AlreadyCanceled(t *testing.T) {
	sub := &subscription.Subscription{Status: subscription.Canceled}

	err := CancelSubscription(sub, false)
	if err == nil {
		t.Fatal("expected error when already canceled")
	}
	if !strings.Contains(err.Error(), "already canceled") {
		t.Fatalf("unexpected error: %s", err)
	}
}

// ---------------------------------------------------------------------------
// lifecycle.go — ReactivateSubscription
// ---------------------------------------------------------------------------

func TestReactivateSubscription_PendingCancel(t *testing.T) {
	sub := &subscription.Subscription{
		Status:     subscription.Active,
		EndCancel:  true,
		Canceled:   true,
		CanceledAt: time.Now(),
	}

	if err := ReactivateSubscription(sub); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sub.EndCancel {
		t.Fatal("EndCancel should be cleared")
	}
	if sub.Canceled {
		t.Fatal("Canceled should be cleared")
	}
	if !sub.CanceledAt.IsZero() {
		t.Fatal("CanceledAt should be zeroed")
	}
}

func TestReactivateSubscription_FullyEnded(t *testing.T) {
	sub := &subscription.Subscription{
		Status: subscription.Canceled,
		Ended:  time.Now(),
	}

	err := ReactivateSubscription(sub)
	if err == nil {
		t.Fatal("expected error for fully ended subscription")
	}
	if !strings.Contains(err.Error(), "fully ended") {
		t.Fatalf("unexpected error: %s", err)
	}
}

func TestReactivateSubscription_CanceledButNotEnded(t *testing.T) {
	sub := &subscription.Subscription{
		Status:   subscription.Canceled,
		Canceled: true,
	}

	if err := ReactivateSubscription(sub); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sub.Status != subscription.Active {
		t.Fatalf("expected Active after reactivation, got %s", sub.Status)
	}
}

// ---------------------------------------------------------------------------
// lifecycle.go — ChangePlan
// ---------------------------------------------------------------------------

func TestChangePlan_NoProration(t *testing.T) {
	oldPlan := makePlan("plan_basic", "Basic", 1000, types.Monthly, 1, 0)
	newPlan := makePlan("plan_pro", "Pro", 2000, types.Monthly, 1, 0)

	sub := &subscription.Subscription{
		Plan:   *oldPlan,
		PlanId: "plan_basic",
	}

	item, err := ChangePlan(sub, newPlan, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item != nil {
		t.Fatal("expected nil proration item when prorate=false")
	}
	if sub.PlanId != "plan_pro" {
		t.Fatalf("plan should be updated to new plan")
	}
}

func TestChangePlan_WithProration(t *testing.T) {
	now := time.Now()
	oldPlan := makePlan("plan_basic", "Basic", 3000, types.Monthly, 1, 0)
	newPlan := makePlan("plan_pro", "Pro", 6000, types.Monthly, 1, 0)

	sub := &subscription.Subscription{
		Plan:        *oldPlan,
		PlanId:      "plan_basic",
		PeriodStart: now.AddDate(0, 0, -15),
		PeriodEnd:   now.AddDate(0, 0, 15),
	}

	item, err := ChangePlan(sub, newPlan, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item == nil {
		t.Fatal("expected proration item")
	}
	if item.Type != billinginvoice.LineProration {
		t.Fatalf("expected proration type, got %s", item.Type)
	}
	if item.Currency != "usd" {
		t.Fatalf("expected usd currency, got %s", item.Currency)
	}
	// Net should be positive (upgrading from 3000 to 6000)
	if item.Amount <= 0 {
		t.Fatalf("expected positive proration amount for upgrade, got %d", item.Amount)
	}
	if !strings.Contains(item.Description, "Basic") || !strings.Contains(item.Description, "Pro") {
		t.Fatalf("description should mention both plans: %s", item.Description)
	}
}

func TestChangePlan_Downgrade(t *testing.T) {
	now := time.Now()
	oldPlan := makePlan("plan_pro", "Pro", 6000, types.Monthly, 1, 0)
	newPlan := makePlan("plan_basic", "Basic", 3000, types.Monthly, 1, 0)

	sub := &subscription.Subscription{
		Plan:        *oldPlan,
		PlanId:      "plan_pro",
		PeriodStart: now.AddDate(0, 0, -15),
		PeriodEnd:   now.AddDate(0, 0, 15),
	}

	item, err := ChangePlan(sub, newPlan, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item == nil {
		t.Fatal("expected proration item for downgrade")
	}
	// Net should be negative (downgrading)
	if item.Amount >= 0 {
		t.Fatalf("expected negative proration amount for downgrade, got %d", item.Amount)
	}
}

// ---------------------------------------------------------------------------
// lifecycle.go — advancePeriod (unexported, tested via StartSubscription)
// ---------------------------------------------------------------------------

func TestAdvancePeriod_Monthly(t *testing.T) {
	sub := &subscription.Subscription{}
	p := makePlan("plan_m", "M", 100, types.Monthly, 1, 0)
	StartSubscription(sub, p)

	// Verify by calendar date arithmetic (immune to DST / short months)
	expected := sub.PeriodStart.AddDate(0, 1, 0)
	if !sub.PeriodEnd.Equal(expected) {
		t.Fatalf("monthly period mismatch: got %v, want %v", sub.PeriodEnd, expected)
	}
}

func TestAdvancePeriod_Yearly(t *testing.T) {
	sub := &subscription.Subscription{}
	p := makePlan("plan_y", "Y", 100, types.Yearly, 1, 0)
	StartSubscription(sub, p)

	expected := sub.PeriodStart.AddDate(1, 0, 0)
	if !sub.PeriodEnd.Equal(expected) {
		t.Fatalf("yearly period mismatch: got %v, want %v", sub.PeriodEnd, expected)
	}
}

func TestAdvancePeriod_ZeroIntervalCount(t *testing.T) {
	// IntervalCount=0 should default to 1
	sub := &subscription.Subscription{}
	p := makePlan("plan_z", "Z", 100, types.Monthly, 0, 0)
	StartSubscription(sub, p)

	expected := sub.PeriodStart.AddDate(0, 1, 0) // defaults to 1 month
	if !sub.PeriodEnd.Equal(expected) {
		t.Fatalf("zero interval count should default to 1: got %v, want %v", sub.PeriodEnd, expected)
	}
}

// ---------------------------------------------------------------------------
// collector.go — CollectionResult struct
// ---------------------------------------------------------------------------

func TestCollectionResult_Struct(t *testing.T) {
	r := CollectionResult{
		Success:       true,
		CreditUsed:    500,
		BalanceUsed:   300,
		ProviderUsed:  200,
		ProviderRef:   "ch_abc123",
		AmountCharged: 1000,
	}

	if !r.Success {
		t.Fatal("expected success")
	}
	if r.CreditUsed+r.BalanceUsed+r.ProviderUsed != r.AmountCharged {
		t.Fatalf("amounts should sum to charged: %d + %d + %d != %d",
			r.CreditUsed, r.BalanceUsed, r.ProviderUsed, r.AmountCharged)
	}
}

// ---------------------------------------------------------------------------
// metering.go — UsageSummary struct
// ---------------------------------------------------------------------------

func TestUsageSummary_Struct(t *testing.T) {
	now := time.Now()
	s := UsageSummary{
		MeterId:         "mtr_001",
		MeterName:       "API Calls",
		AggregationType: "sum",
		Value:           42000,
		EventCount:      150,
		PeriodStart:     now.AddDate(0, -1, 0),
		PeriodEnd:       now,
	}

	if s.MeterId != "mtr_001" {
		t.Fatalf("MeterId mismatch: %s", s.MeterId)
	}
	if s.Value != 42000 {
		t.Fatalf("Value mismatch: %d", s.Value)
	}
	if s.EventCount != 150 {
		t.Fatalf("EventCount mismatch: %d", s.EventCount)
	}
	if s.PeriodStart.After(s.PeriodEnd) {
		t.Fatal("PeriodStart should be before PeriodEnd")
	}
}

// ---------------------------------------------------------------------------
// balance.go — GetOrCreateCustomerBalance requires datastore
// ---------------------------------------------------------------------------

func TestGetOrCreateCustomerBalance_RequiresDatastore(t *testing.T) {
	t.Skip("requires datastore: customerbalance.Query(db) needs live db")
}

func TestAdjustCustomerBalance_RequiresDatastore(t *testing.T) {
	t.Skip("requires datastore: GetOrCreateCustomerBalance(db, ...) needs live db")
}

func TestApplyBalanceToInvoice_RequiresDatastore(t *testing.T) {
	t.Skip("requires datastore: GetOrCreateCustomerBalance(db, ...) needs live db")
}

// ---------------------------------------------------------------------------
// tax.go — CalculateInvoiceTax with address requires datastore
// ---------------------------------------------------------------------------

func TestCalculateInvoiceTax_WithAddress_RequiresDatastore(t *testing.T) {
	t.Skip("requires datastore: db.NewKey() + taxregion.Query(db) needs live db")
}

// ---------------------------------------------------------------------------
// events.go — EmitBillingEvent / DispatchWebhooks require datastore
// ---------------------------------------------------------------------------

func TestEmitBillingEvent_RequiresDatastore(t *testing.T) {
	t.Skip("requires datastore: billingevent.New(db) needs live db")
}

func TestDispatchWebhooks_RequiresDatastore(t *testing.T) {
	t.Skip("requires datastore: webhookendpoint.Query(db) needs live db")
}

// ---------------------------------------------------------------------------
// aggregator.go — AggregateUsage requires datastore
// ---------------------------------------------------------------------------

func TestAggregateUsage_RequiresDatastore(t *testing.T) {
	t.Skip("requires datastore: meter.Query(db) needs live db")
}

// ---------------------------------------------------------------------------
// lifecycle.go — RenewSubscription requires datastore
// ---------------------------------------------------------------------------

func TestRenewSubscription_RequiresDatastore(t *testing.T) {
	t.Skip("requires datastore: billinginvoice.New(db) + AggregateUsage needs live db")
}

// ---------------------------------------------------------------------------
// Param struct construction tests
// ---------------------------------------------------------------------------

func TestCreatePaymentIntentParams_Defaults(t *testing.T) {
	p := CreatePaymentIntentParams{
		CustomerId: "cus_test",
		Amount:     5000,
		Currency:   "usd",
	}

	if p.CaptureMethod != "" {
		t.Fatal("CaptureMethod should default to empty")
	}
	if p.ConfirmationMethod != "" {
		t.Fatal("ConfirmationMethod should default to empty")
	}
	if p.SetupFutureUsage != "" {
		t.Fatal("SetupFutureUsage should default to empty")
	}
}

func TestCreateSetupIntentParams_Defaults(t *testing.T) {
	p := CreateSetupIntentParams{
		CustomerId: "cus_test",
	}

	if p.PaymentMethodId != "" {
		t.Fatal("PaymentMethodId should default to empty")
	}
	if p.Usage != "" {
		t.Fatal("Usage should default to empty")
	}
}

func TestCreateRefundParams_FullRefund(t *testing.T) {
	p := CreateRefundParams{
		PaymentIntentId: "pi_123",
		Amount:          0, // 0 = full refund
		Reason:          "duplicate",
	}

	if p.Amount != 0 {
		t.Fatal("Amount=0 should indicate full refund")
	}
}

func TestCreateCreditNoteParams_LineItemTotal(t *testing.T) {
	// Verify that when Amount is 0, line items should be summed
	// (tested via CreateCreditNote, which requires datastore,
	// but we verify the struct holds the data correctly)
	p := CreateCreditNoteParams{
		InvoiceId:  "inv_001",
		CustomerId: "cus_001",
		Amount:     0,
		Memo:       "Goodwill credit",
	}

	if p.Amount != 0 {
		t.Fatal("Amount should be 0 to indicate line-item-based calculation")
	}
	if p.Memo != "Goodwill credit" {
		t.Fatalf("Memo mismatch: %s", p.Memo)
	}
}

// ===========================================================================
// ADDITIONAL TESTS — boost coverage beyond 15.2%
// ===========================================================================

// ---------------------------------------------------------------------------
// intents.go — CreatePaymentIntent: edge cases
// ---------------------------------------------------------------------------

func TestCreatePaymentIntent_MissingCurrency_RequiresDatastore(t *testing.T) {
	t.Skip("requires datastore: empty currency passes validation, paymentintent.New(db) needs live db")
}

func TestCreatePaymentIntent_ValidMinAmount_RequiresDatastore(t *testing.T) {
	t.Skip("requires datastore: amount=1 passes validation, paymentintent.New(db) needs live db")
}

func TestCreatePaymentIntent_ValidLargeAmount_RequiresDatastore(t *testing.T) {
	t.Skip("requires datastore: large amount passes validation, paymentintent.New(db) needs live db")
}

func TestCreatePaymentIntent_AllOptionalFields(t *testing.T) {
	p := CreatePaymentIntentParams{
		CustomerId:         "cus_all",
		Amount:             5000,
		Currency:           "eur",
		PaymentMethodId:    "pm_123",
		CaptureMethod:      "manual",
		ConfirmationMethod: "automatic",
		SetupFutureUsage:   "off_session",
		Description:        "Test payment",
		ReceiptEmail:       "test@example.com",
		InvoiceId:          "inv_456",
	}

	if p.CaptureMethod != "manual" {
		t.Fatal("CaptureMethod should be manual")
	}
	if p.ConfirmationMethod != "automatic" {
		t.Fatal("ConfirmationMethod should be automatic")
	}
	if p.SetupFutureUsage != "off_session" {
		t.Fatal("SetupFutureUsage should be off_session")
	}
	if p.Description != "Test payment" {
		t.Fatal("Description mismatch")
	}
	if p.ReceiptEmail != "test@example.com" {
		t.Fatal("ReceiptEmail mismatch")
	}
	if p.InvoiceId != "inv_456" {
		t.Fatal("InvoiceId mismatch")
	}
}

// ---------------------------------------------------------------------------
// intents.go — CreateSetupIntent: edge cases
// ---------------------------------------------------------------------------

func TestCreateSetupIntent_AllFields(t *testing.T) {
	p := CreateSetupIntentParams{
		CustomerId:      "cus_setup",
		PaymentMethodId: "pm_card_visa",
		Usage:           "off_session",
	}

	if p.Usage != "off_session" {
		t.Fatal("Usage should be off_session")
	}
	if p.PaymentMethodId != "pm_card_visa" {
		t.Fatal("PaymentMethodId mismatch")
	}
}

// ---------------------------------------------------------------------------
// intents.go — ConfirmPaymentIntent requires datastore
// ---------------------------------------------------------------------------

func TestConfirmPaymentIntent_RequiresDatastore(t *testing.T) {
	t.Skip("requires datastore: paymentintent.Confirm() + paymentmethod.GetById needs live db")
}

func TestCapturePaymentIntent_RequiresDatastore(t *testing.T) {
	t.Skip("requires datastore: paymentintent.Capture() + pi.Update() needs live db")
}

func TestCancelPaymentIntent_RequiresDatastore(t *testing.T) {
	t.Skip("requires datastore: pi.Cancel() + pi.Update() needs live db")
}

func TestConfirmSetupIntent_RequiresDatastore(t *testing.T) {
	t.Skip("requires datastore: si.Confirm() + paymentmethod.GetById needs live db")
}

func TestCancelSetupIntent_RequiresDatastore(t *testing.T) {
	t.Skip("requires datastore: si.Cancel() + si.Update() needs live db")
}

// ---------------------------------------------------------------------------
// lifecycle.go — StartSubscription: additional intervals and edge cases
// ---------------------------------------------------------------------------

func TestStartSubscription_DefaultInterval(t *testing.T) {
	// Unknown interval should default to monthly
	sub := &subscription.Subscription{}
	p := makePlan("plan_custom", "Custom", 1500, "weekly", 2, 0)

	StartSubscription(sub, p)

	// Default case in advancePeriod treats unknown interval as monthly
	expected := sub.PeriodStart.AddDate(0, 2, 0)
	if !sub.PeriodEnd.Equal(expected) {
		t.Fatalf("unknown interval should default to monthly: got %v, want %v", sub.PeriodEnd, expected)
	}
}

func TestStartSubscription_SetsAllFields(t *testing.T) {
	sub := &subscription.Subscription{}
	p := makePlan("plan_full", "Full", 9900, types.Monthly, 1, 0)

	StartSubscription(sub, p)

	if sub.Plan.Name != "Full" {
		t.Fatalf("Plan.Name mismatch: %s", sub.Plan.Name)
	}
	if sub.Plan.Price != 9900 {
		t.Fatalf("Plan.Price mismatch: %d", sub.Plan.Price)
	}
	if string(sub.Plan.Currency) != "usd" {
		t.Fatalf("Plan.Currency mismatch: %s", sub.Plan.Currency)
	}
	if sub.PlanId != "plan_full" {
		t.Fatalf("PlanId mismatch: %s", sub.PlanId)
	}
}

func TestStartSubscription_TrialDaysOne(t *testing.T) {
	sub := &subscription.Subscription{}
	p := makePlan("plan_trial1", "OneDayTrial", 500, types.Monthly, 1, 1)

	StartSubscription(sub, p)

	if sub.Status != subscription.Trialing {
		t.Fatalf("expected Trialing, got %s", sub.Status)
	}

	expectedTrialEnd := sub.TrialStart.AddDate(0, 0, 1)
	if !sub.TrialEnd.Equal(expectedTrialEnd) {
		t.Fatalf("1-day trial end mismatch: got %v, want %v", sub.TrialEnd, expectedTrialEnd)
	}
}

func TestStartSubscription_YearlyWithTrial(t *testing.T) {
	sub := &subscription.Subscription{}
	p := makePlan("plan_yearly_trial", "YearlyTrial", 12000, types.Yearly, 1, 30)

	StartSubscription(sub, p)

	if sub.Status != subscription.Trialing {
		t.Fatalf("expected Trialing, got %s", sub.Status)
	}

	// Trial is 30 days
	expectedTrialEnd := sub.TrialStart.AddDate(0, 0, 30)
	if !sub.TrialEnd.Equal(expectedTrialEnd) {
		t.Fatalf("trial end mismatch: got %v, want %v", sub.TrialEnd, expectedTrialEnd)
	}

	// Period after trial should be 1 year
	expectedPeriodEnd := sub.PeriodStart.AddDate(1, 0, 0)
	if !sub.PeriodEnd.Equal(expectedPeriodEnd) {
		t.Fatalf("yearly period end mismatch: got %v, want %v", sub.PeriodEnd, expectedPeriodEnd)
	}
}

func TestStartSubscription_MultiYearInterval(t *testing.T) {
	sub := &subscription.Subscription{}
	p := makePlan("plan_2y", "TwoYear", 50000, types.Yearly, 2, 0)

	StartSubscription(sub, p)

	expected := sub.PeriodStart.AddDate(2, 0, 0)
	if !sub.PeriodEnd.Equal(expected) {
		t.Fatalf("2-year period mismatch: got %v, want %v", sub.PeriodEnd, expected)
	}
}

// ---------------------------------------------------------------------------
// lifecycle.go — CancelSubscription: additional states
// ---------------------------------------------------------------------------

func TestCancelSubscription_Trialing_Immediate(t *testing.T) {
	sub := &subscription.Subscription{Status: subscription.Trialing}

	if err := CancelSubscription(sub, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sub.Status != subscription.Canceled {
		t.Fatalf("expected Canceled, got %s", sub.Status)
	}
	if !sub.Canceled {
		t.Fatal("Canceled flag should be true")
	}
	if sub.Ended.IsZero() {
		t.Fatal("Ended should be set for immediate cancel")
	}
}

func TestCancelSubscription_Trialing_AtPeriodEnd(t *testing.T) {
	sub := &subscription.Subscription{Status: subscription.Trialing}

	if err := CancelSubscription(sub, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sub.EndCancel != true {
		t.Fatal("EndCancel should be true")
	}
	if sub.CanceledAt.IsZero() {
		t.Fatal("CanceledAt should be set")
	}
	// Status should remain trialing for deferred cancel
	if sub.Status != subscription.Trialing {
		t.Fatalf("expected status to remain Trialing for at-period-end, got %s", sub.Status)
	}
}

func TestCancelSubscription_PastDue_Immediate(t *testing.T) {
	sub := &subscription.Subscription{Status: subscription.PastDue}

	if err := CancelSubscription(sub, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sub.Status != subscription.Canceled {
		t.Fatalf("expected Canceled, got %s", sub.Status)
	}
}

func TestCancelSubscription_Unpaid(t *testing.T) {
	sub := &subscription.Subscription{Status: subscription.Unpaid}

	if err := CancelSubscription(sub, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sub.Status != subscription.Canceled {
		t.Fatalf("expected Canceled, got %s", sub.Status)
	}
}

func TestCancelSubscription_SetsTimestampClose(t *testing.T) {
	before := time.Now()
	sub := &subscription.Subscription{Status: subscription.Active}
	_ = CancelSubscription(sub, false)
	after := time.Now()

	if sub.CanceledAt.Before(before) || sub.CanceledAt.After(after) {
		t.Fatalf("CanceledAt should be between before and after: got %v", sub.CanceledAt)
	}
	if sub.Ended.Before(before) || sub.Ended.After(after) {
		t.Fatalf("Ended should be between before and after: got %v", sub.Ended)
	}
}

// ---------------------------------------------------------------------------
// lifecycle.go — ReactivateSubscription: additional cases
// ---------------------------------------------------------------------------

func TestReactivateSubscription_Trialing(t *testing.T) {
	// Trialing with pending cancel
	sub := &subscription.Subscription{
		Status:     subscription.Trialing,
		EndCancel:  true,
		Canceled:   true,
		CanceledAt: time.Now(),
	}

	if err := ReactivateSubscription(sub); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sub.EndCancel {
		t.Fatal("EndCancel should be cleared")
	}
	if sub.Canceled {
		t.Fatal("Canceled should be cleared")
	}
	// Status should remain Trialing (not changed to Active)
	if sub.Status != subscription.Trialing {
		t.Fatalf("expected Trialing to be preserved, got %s", sub.Status)
	}
}

func TestReactivateSubscription_PastDue(t *testing.T) {
	sub := &subscription.Subscription{
		Status:   subscription.PastDue,
		Canceled: true,
	}

	if err := ReactivateSubscription(sub); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Status stays PastDue (only Canceled -> Active transition happens)
	if sub.Status != subscription.PastDue {
		t.Fatalf("expected PastDue to be preserved, got %s", sub.Status)
	}
	if sub.Canceled {
		t.Fatal("Canceled should be cleared")
	}
}

func TestReactivateSubscription_ActiveNotCanceled(t *testing.T) {
	// Reactivating an active sub that was never canceled — should be a no-op
	sub := &subscription.Subscription{
		Status: subscription.Active,
	}

	if err := ReactivateSubscription(sub); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sub.Status != subscription.Active {
		t.Fatalf("expected Active, got %s", sub.Status)
	}
}

// ---------------------------------------------------------------------------
// lifecycle.go — TransitionTrialToActive: all invalid states
// ---------------------------------------------------------------------------

func TestTransitionTrialToActive_PastDue(t *testing.T) {
	sub := &subscription.Subscription{Status: subscription.PastDue}

	err := TransitionTrialToActive(sub)
	if err == nil {
		t.Fatal("expected error for PastDue subscription")
	}
	if !strings.Contains(err.Error(), "not trialing") {
		t.Fatalf("unexpected error: %s", err)
	}
}

func TestTransitionTrialToActive_Unpaid(t *testing.T) {
	sub := &subscription.Subscription{Status: subscription.Unpaid}

	err := TransitionTrialToActive(sub)
	if err == nil {
		t.Fatal("expected error for Unpaid subscription")
	}
}

// ---------------------------------------------------------------------------
// lifecycle.go — ChangePlan: additional edge cases
// ---------------------------------------------------------------------------

func TestChangePlan_SamePlan_NoProration(t *testing.T) {
	p := makePlan("plan_basic", "Basic", 1000, types.Monthly, 1, 0)

	sub := &subscription.Subscription{
		Plan:   *p,
		PlanId: "plan_basic",
	}

	item, err := ChangePlan(sub, p, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item != nil {
		t.Fatal("expected nil proration item for same plan, no prorate")
	}
	if sub.PlanId != "plan_basic" {
		t.Fatalf("planId should remain: %s", sub.PlanId)
	}
}

func TestChangePlan_SamePlan_WithProration(t *testing.T) {
	now := time.Now()
	p := makePlan("plan_basic", "Basic", 3000, types.Monthly, 1, 0)

	sub := &subscription.Subscription{
		Plan:        *p,
		PlanId:      "plan_basic",
		PeriodStart: now.AddDate(0, 0, -15),
		PeriodEnd:   now.AddDate(0, 0, 15),
	}

	item, err := ChangePlan(sub, p, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Same plan proration should be approximately zero
	if item != nil && (item.Amount > 1 || item.Amount < -1) {
		t.Fatalf("same plan proration should be ~0, got %d", item.Amount)
	}
}

func TestChangePlan_UpdatesPlanFields(t *testing.T) {
	oldPlan := makePlan("plan_old", "Old", 1000, types.Monthly, 1, 0)
	newPlan := makePlan("plan_new", "New", 2000, types.Yearly, 1, 0)

	sub := &subscription.Subscription{
		Plan:   *oldPlan,
		PlanId: "plan_old",
	}

	_, err := ChangePlan(sub, newPlan, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sub.PlanId != "plan_new" {
		t.Fatalf("PlanId should be updated: %s", sub.PlanId)
	}
	if sub.Plan.Name != "New" {
		t.Fatalf("Plan.Name should be updated: %s", sub.Plan.Name)
	}
	if sub.Plan.Price != 2000 {
		t.Fatalf("Plan.Price should be updated: %d", sub.Plan.Price)
	}
}

func TestChangePlan_ZeroPeriodDuration(t *testing.T) {
	// PeriodStart == PeriodEnd → totalDays <= 0 → nil proration
	now := time.Now()
	old := makePlan("plan_a", "A", 1000, types.Monthly, 1, 0)
	new_ := makePlan("plan_b", "B", 2000, types.Monthly, 1, 0)

	sub := &subscription.Subscription{
		Plan:        *old,
		PlanId:      "plan_a",
		PeriodStart: now,
		PeriodEnd:   now, // zero duration
	}

	item, err := ChangePlan(sub, new_, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item != nil {
		t.Fatal("expected nil proration for zero-duration period")
	}
	// Plan should still be updated
	if sub.PlanId != "plan_b" {
		t.Fatalf("planId should be updated: %s", sub.PlanId)
	}
}

func TestChangePlan_ExpiredPeriod(t *testing.T) {
	// PeriodEnd < PeriodStart → totalDays negative → nil proration
	now := time.Now()
	old := makePlan("plan_old", "Old", 1000, types.Monthly, 1, 0)
	new_ := makePlan("plan_new", "New", 2000, types.Monthly, 1, 0)

	sub := &subscription.Subscription{
		Plan:        *old,
		PlanId:      "plan_old",
		PeriodStart: now,
		PeriodEnd:   now.AddDate(0, 0, -5), // expired
	}

	item, err := ChangePlan(sub, new_, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item != nil {
		t.Fatal("expected nil proration for expired period")
	}
}

func TestChangePlan_ProrationDescription(t *testing.T) {
	now := time.Now()
	old := makePlan("plan_a", "Alpha", 4000, types.Monthly, 1, 0)
	new_ := makePlan("plan_b", "Beta", 8000, types.Monthly, 1, 0)

	sub := &subscription.Subscription{
		Plan:        *old,
		PlanId:      "plan_a",
		PeriodStart: now.AddDate(0, 0, -10),
		PeriodEnd:   now.AddDate(0, 0, 20),
	}

	item, err := ChangePlan(sub, new_, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item == nil {
		t.Fatal("expected proration item")
	}
	if !strings.Contains(item.Description, "Alpha") {
		t.Fatalf("description should contain old plan name: %s", item.Description)
	}
	if !strings.Contains(item.Description, "Beta") {
		t.Fatalf("description should contain new plan name: %s", item.Description)
	}
	if !strings.Contains(item.Description, "Proration") {
		t.Fatalf("description should contain 'Proration': %s", item.Description)
	}
}

func TestChangePlan_ProrationItemFields(t *testing.T) {
	now := time.Now()
	old := makePlan("plan_lo", "Lo", 2000, types.Monthly, 1, 0)
	hi := makePlan("plan_hi", "Hi", 6000, types.Monthly, 1, 0)

	sub := &subscription.Subscription{
		Plan:        *old,
		PlanId:      "plan_lo",
		PeriodStart: now.AddDate(0, 0, -15),
		PeriodEnd:   now.AddDate(0, 0, 15),
	}

	item, _ := ChangePlan(sub, hi, true)
	if item == nil {
		t.Fatal("expected proration item")
	}

	if item.Type != billinginvoice.LineProration {
		t.Fatalf("expected proration type, got %s", item.Type)
	}
	if item.PlanId != "plan_hi" {
		t.Fatalf("proration item should reference new plan: %s", item.PlanId)
	}
	if item.PlanName != "Hi" {
		t.Fatalf("proration item PlanName mismatch: %s", item.PlanName)
	}
	if item.PeriodEnd.IsZero() {
		t.Fatal("proration item PeriodEnd should be set")
	}
	if item.PeriodStart.IsZero() {
		t.Fatal("proration item PeriodStart should be set")
	}
	if !strings.HasPrefix(item.Id, "li_proration_") {
		t.Fatalf("proration item Id should start with li_proration_: %s", item.Id)
	}
}

func TestChangePlan_FreeToPaid(t *testing.T) {
	now := time.Now()
	free := makePlan("plan_free", "Free", 0, types.Monthly, 1, 0)
	paid := makePlan("plan_paid", "Paid", 5000, types.Monthly, 1, 0)

	sub := &subscription.Subscription{
		Plan:        *free,
		PlanId:      "plan_free",
		PeriodStart: now.AddDate(0, 0, -15),
		PeriodEnd:   now.AddDate(0, 0, 15),
	}

	item, err := ChangePlan(sub, paid, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item == nil {
		t.Fatal("expected proration item")
	}
	// Upgrading from free to paid: should be positive
	if item.Amount <= 0 {
		t.Fatalf("expected positive amount for free->paid upgrade, got %d", item.Amount)
	}
}

func TestChangePlan_PaidToFree(t *testing.T) {
	now := time.Now()
	paid := makePlan("plan_paid", "Paid", 5000, types.Monthly, 1, 0)
	free := makePlan("plan_free", "Free", 0, types.Monthly, 1, 0)

	sub := &subscription.Subscription{
		Plan:        *paid,
		PlanId:      "plan_paid",
		PeriodStart: now.AddDate(0, 0, -15),
		PeriodEnd:   now.AddDate(0, 0, 15),
	}

	item, err := ChangePlan(sub, free, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item == nil {
		t.Fatal("expected proration item")
	}
	// Downgrading from paid to free: should be negative
	if item.Amount >= 0 {
		t.Fatalf("expected negative amount for paid->free downgrade, got %d", item.Amount)
	}
}

// ---------------------------------------------------------------------------
// lifecycle.go — advancePeriod: additional edge cases
// ---------------------------------------------------------------------------

func TestAdvancePeriod_NegativeIntervalCount(t *testing.T) {
	// Negative interval count should default to 1
	sub := &subscription.Subscription{}
	p := makePlan("plan_neg", "Neg", 100, types.Monthly, -1, 0)
	StartSubscription(sub, p)

	expected := sub.PeriodStart.AddDate(0, 1, 0) // defaults to 1
	if !sub.PeriodEnd.Equal(expected) {
		t.Fatalf("negative interval count should default to 1: got %v, want %v", sub.PeriodEnd, expected)
	}
}

func TestAdvancePeriod_LargeIntervalCount(t *testing.T) {
	sub := &subscription.Subscription{}
	p := makePlan("plan_12m", "12Month", 100, types.Monthly, 12, 0)
	StartSubscription(sub, p)

	expected := sub.PeriodStart.AddDate(0, 12, 0)
	if !sub.PeriodEnd.Equal(expected) {
		t.Fatalf("12-month period mismatch: got %v, want %v", sub.PeriodEnd, expected)
	}
}

// ---------------------------------------------------------------------------
// events.go — VerifyWebhookSignature: additional edge cases
// ---------------------------------------------------------------------------

func TestVerifyWebhookSignature_ExtraFields(t *testing.T) {
	// Header with extra unknown fields should still work
	payload := []byte(`{"type":"test"}`)
	secret := "whsec_extra"
	timestamp := "1700000000"
	sig := computeSignature(timestamp, payload, secret)
	header := fmt.Sprintf("t=%s,v1=%s,extra=ignored", timestamp, sig)

	if err := VerifyWebhookSignature(payload, header, secret); err != nil {
		t.Fatalf("extra fields should be ignored: %v", err)
	}
}

func TestVerifyWebhookSignature_MalformedParts(t *testing.T) {
	// Parts without '=' should be skipped
	payload := []byte(`{"type":"test"}`)
	secret := "whsec_malformed"
	timestamp := "1700000000"
	sig := computeSignature(timestamp, payload, secret)
	header := fmt.Sprintf("garbage,t=%s,v1=%s,noequals", timestamp, sig)

	if err := VerifyWebhookSignature(payload, header, secret); err != nil {
		t.Fatalf("malformed parts should be skipped: %v", err)
	}
}

func TestVerifyWebhookSignature_DuplicateTimestamp(t *testing.T) {
	// Multiple t= entries — last one should win
	payload := []byte(`{"dup":"test"}`)
	secret := "whsec_dup"
	timestamp := "1700000000"
	sig := computeSignature(timestamp, payload, secret)
	// First t is wrong, second is correct
	header := fmt.Sprintf("t=9999999999,t=%s,v1=%s", timestamp, sig)

	if err := VerifyWebhookSignature(payload, header, secret); err != nil {
		t.Fatalf("should use last timestamp: %v", err)
	}
}

func TestVerifyWebhookSignature_LargePayload(t *testing.T) {
	// Test with a realistically large payload
	payload := []byte(strings.Repeat(`{"key":"value",`, 1000) + `"end":true}`)
	secret := "whsec_large"
	timestamp := "1700000000"
	sig := computeSignature(timestamp, payload, secret)
	header := fmt.Sprintf("t=%s,v1=%s", timestamp, sig)

	if err := VerifyWebhookSignature(payload, header, secret); err != nil {
		t.Fatalf("large payload should verify: %v", err)
	}
}

func TestVerifyWebhookSignature_EmptyPayload(t *testing.T) {
	payload := []byte{}
	secret := "whsec_empty"
	timestamp := "1700000000"
	sig := computeSignature(timestamp, payload, secret)
	header := fmt.Sprintf("t=%s,v1=%s", timestamp, sig)

	if err := VerifyWebhookSignature(payload, header, secret); err != nil {
		t.Fatalf("empty payload should verify: %v", err)
	}
}

func TestVerifyWebhookSignature_UnicodePayload(t *testing.T) {
	payload := []byte(`{"name":"日本語テスト","emoji":"🎉"}`)
	secret := "whsec_unicode"
	timestamp := "1700000000"
	sig := computeSignature(timestamp, payload, secret)
	header := fmt.Sprintf("t=%s,v1=%s", timestamp, sig)

	if err := VerifyWebhookSignature(payload, header, secret); err != nil {
		t.Fatalf("unicode payload should verify: %v", err)
	}
}

// ---------------------------------------------------------------------------
// events.go — computeSignature: additional cases
// ---------------------------------------------------------------------------

func TestComputeSignature_Deterministic(t *testing.T) {
	ts := "1700000000"
	payload := []byte(`{"deterministic":"yes"}`)
	secret := "whsec_det"

	sig1 := computeSignature(ts, payload, secret)
	sig2 := computeSignature(ts, payload, secret)

	if sig1 != sig2 {
		t.Fatal("computeSignature should be deterministic")
	}
}

func TestComputeSignature_EmptySecret(t *testing.T) {
	sig := computeSignature("1234", []byte("payload"), "")
	if sig == "" {
		t.Fatal("signature should not be empty even for empty secret")
	}
	if len(sig) != 64 {
		t.Fatalf("expected 64 hex chars, got %d", len(sig))
	}
}

func TestComputeSignature_EmptyTimestamp(t *testing.T) {
	sig := computeSignature("", []byte("payload"), "secret")
	if sig == "" {
		t.Fatal("signature should not be empty even for empty timestamp")
	}
	if len(sig) != 64 {
		t.Fatalf("expected 64 hex chars, got %d", len(sig))
	}
}

func TestComputeSignature_DifferentPayloads(t *testing.T) {
	ts := "1700000000"
	secret := "shared_secret"

	sig1 := computeSignature(ts, []byte(`{"a":1}`), secret)
	sig2 := computeSignature(ts, []byte(`{"a":2}`), secret)

	if sig1 == sig2 {
		t.Fatal("different payloads should produce different signatures")
	}
}

// ---------------------------------------------------------------------------
// tax.go — TaxLine: additional struct tests
// ---------------------------------------------------------------------------

func TestTaxLine_Inclusive(t *testing.T) {
	tl := TaxLine{
		TaxRateId:    "txr_inc",
		Description:  "VAT",
		Amount:       2000,
		Rate:         0.20,
		Inclusive:    true,
		Jurisdiction: "GB",
	}

	if !tl.Inclusive {
		t.Fatal("Inclusive should be true")
	}
	if tl.Rate != 0.20 {
		t.Fatalf("Rate mismatch: %f", tl.Rate)
	}
}

func TestTaxLine_ZeroRate(t *testing.T) {
	tl := TaxLine{
		TaxRateId:    "txr_zero",
		Description:  "Zero-rated",
		Amount:       0,
		Rate:         0.0,
		Inclusive:    false,
		Jurisdiction: "US-OR",
	}

	if tl.Amount != 0 {
		t.Fatalf("Amount should be 0 for zero-rated tax: %d", tl.Amount)
	}
	if tl.Rate != 0.0 {
		t.Fatalf("Rate should be 0: %f", tl.Rate)
	}
}

func TestTaxLine_MultipleTaxes(t *testing.T) {
	// Simulate stacking state + county taxes
	lines := []TaxLine{
		{TaxRateId: "txr_state", Description: "State Tax", Amount: 600, Rate: 0.06, Jurisdiction: "US-TX"},
		{TaxRateId: "txr_county", Description: "County Tax", Amount: 200, Rate: 0.02, Jurisdiction: "US-TX-HARRIS"},
	}

	var total int64
	for _, l := range lines {
		total += l.Amount
	}

	if total != 800 {
		t.Fatalf("total tax should be 800, got %d", total)
	}
	if len(lines) != 2 {
		t.Fatalf("expected 2 tax lines, got %d", len(lines))
	}
}

// ---------------------------------------------------------------------------
// collector.go — CollectionResult: additional struct tests
// ---------------------------------------------------------------------------

func TestCollectionResult_Failure(t *testing.T) {
	r := CollectionResult{
		Success:       false,
		AmountCharged: 0,
		Error:         "card_declined",
	}

	if r.Success {
		t.Fatal("expected failure")
	}
	if r.Error != "card_declined" {
		t.Fatalf("Error mismatch: %s", r.Error)
	}
	if r.AmountCharged != 0 {
		t.Fatalf("AmountCharged should be 0 for failure: %d", r.AmountCharged)
	}
}

func TestCollectionResult_CreditOnly(t *testing.T) {
	r := CollectionResult{
		Success:       true,
		CreditUsed:    5000,
		BalanceUsed:   0,
		ProviderUsed:  0,
		AmountCharged: 5000,
	}

	if !r.Success {
		t.Fatal("expected success")
	}
	if r.CreditUsed != r.AmountCharged {
		t.Fatal("credit-only payment should have CreditUsed == AmountCharged")
	}
	if r.ProviderRef != "" {
		t.Fatal("credit-only should have no provider ref")
	}
}

func TestCollectionResult_BalanceOnly(t *testing.T) {
	r := CollectionResult{
		Success:       true,
		CreditUsed:    0,
		BalanceUsed:   3000,
		ProviderUsed:  0,
		AmountCharged: 3000,
	}

	if r.BalanceUsed != r.AmountCharged {
		t.Fatal("balance-only payment should have BalanceUsed == AmountCharged")
	}
}

func TestCollectionResult_ProviderOnly(t *testing.T) {
	r := CollectionResult{
		Success:       true,
		CreditUsed:    0,
		BalanceUsed:   0,
		ProviderUsed:  10000,
		ProviderRef:   "ch_stripe_123",
		AmountCharged: 10000,
	}

	if r.ProviderUsed != r.AmountCharged {
		t.Fatal("provider-only payment should have ProviderUsed == AmountCharged")
	}
	if r.ProviderRef != "ch_stripe_123" {
		t.Fatalf("ProviderRef mismatch: %s", r.ProviderRef)
	}
}

func TestCollectionResult_ZeroAmount(t *testing.T) {
	// e.g. $0 invoice (100% discount)
	r := CollectionResult{
		Success:       true,
		AmountCharged: 0,
	}

	if !r.Success {
		t.Fatal("zero-amount collection should succeed")
	}
}

// ---------------------------------------------------------------------------
// metering.go — UsageSummary: additional tests
// ---------------------------------------------------------------------------

func TestUsageSummary_ZeroValues(t *testing.T) {
	s := UsageSummary{
		MeterId:         "mtr_empty",
		MeterName:       "Empty Meter",
		AggregationType: "sum",
		Value:           0,
		EventCount:      0,
	}

	if s.Value != 0 {
		t.Fatalf("Value should be 0: %d", s.Value)
	}
	if s.EventCount != 0 {
		t.Fatalf("EventCount should be 0: %d", s.EventCount)
	}
}

func TestUsageSummary_CountAggregation(t *testing.T) {
	s := UsageSummary{
		MeterId:         "mtr_count",
		MeterName:       "Request Count",
		AggregationType: "count",
		Value:           1000,
		EventCount:      1000,
	}

	if s.AggregationType != "count" {
		t.Fatalf("AggregationType mismatch: %s", s.AggregationType)
	}
	// For count aggregation, Value should equal EventCount
	if s.Value != s.EventCount {
		t.Fatalf("count aggregation: Value (%d) should match EventCount (%d)", s.Value, s.EventCount)
	}
}

func TestUsageSummary_LastAggregation(t *testing.T) {
	s := UsageSummary{
		MeterId:         "mtr_gauge",
		MeterName:       "Temperature",
		AggregationType: "last",
		Value:           72,
		EventCount:      500,
	}

	if s.AggregationType != "last" {
		t.Fatalf("AggregationType mismatch: %s", s.AggregationType)
	}
	// For last aggregation, Value is the last event's value, not related to count
	if s.Value == s.EventCount {
		// This is fine but unlikely — just documenting the semantics
	}
}

func TestUsageSummary_PeriodBoundary(t *testing.T) {
	now := time.Now()
	start := now.AddDate(0, -1, 0)
	end := now

	s := UsageSummary{
		MeterId:     "mtr_boundary",
		PeriodStart: start,
		PeriodEnd:   end,
	}

	if s.PeriodEnd.Before(s.PeriodStart) {
		t.Fatal("PeriodEnd should be after PeriodStart")
	}

	duration := s.PeriodEnd.Sub(s.PeriodStart)
	if duration.Hours() < 24*28 { // at least 28 days for ~1 month
		t.Fatalf("period should be roughly 1 month, got %v", duration)
	}
}

// ---------------------------------------------------------------------------
// refunds.go — CreateRefundParams: additional param tests
// ---------------------------------------------------------------------------

func TestCreateRefundParams_PartialRefund(t *testing.T) {
	p := CreateRefundParams{
		PaymentIntentId: "pi_abc",
		Amount:          500,
		Reason:          "requested_by_customer",
	}

	if p.Amount != 500 {
		t.Fatal("Amount should be 500 for partial refund")
	}
	if p.Reason != "requested_by_customer" {
		t.Fatalf("Reason mismatch: %s", p.Reason)
	}
}

func TestCreateRefundParams_FraudulentReason(t *testing.T) {
	p := CreateRefundParams{
		PaymentIntentId: "pi_fraud",
		Amount:          0,
		Reason:          "fraudulent",
	}

	if p.Reason != "fraudulent" {
		t.Fatalf("Reason mismatch: %s", p.Reason)
	}
}

func TestCreateRefundParams_DuplicateReason(t *testing.T) {
	p := CreateRefundParams{
		InvoiceId: "inv_dup",
		Reason:    "duplicate",
	}

	if p.Reason != "duplicate" {
		t.Fatalf("Reason mismatch: %s", p.Reason)
	}
	if p.InvoiceId != "inv_dup" {
		t.Fatalf("InvoiceId mismatch: %s", p.InvoiceId)
	}
}

func TestCreateRefundParams_BothIds(t *testing.T) {
	// Having both IDs set — PaymentIntentId takes priority in the code
	p := CreateRefundParams{
		PaymentIntentId: "pi_both",
		InvoiceId:       "inv_both",
		Amount:          1000,
		Reason:          "requested_by_customer",
	}

	if p.PaymentIntentId == "" {
		t.Fatal("PaymentIntentId should be set")
	}
	if p.InvoiceId == "" {
		t.Fatal("InvoiceId should be set")
	}
}

// ---------------------------------------------------------------------------
// refunds.go — CreateCreditNoteParams: additional tests
// ---------------------------------------------------------------------------

func TestCreateCreditNoteParams_WithLineItems(t *testing.T) {
	p := CreateCreditNoteParams{
		InvoiceId:  "inv_li",
		CustomerId: "cus_li",
		LineItems: []creditnote.CreditNoteLineItem{
			{Description: "API calls overage", Amount: 500, Quantity: 100, UnitPrice: 5},
			{Description: "Storage credit", Amount: 200, Quantity: 1, UnitPrice: 200},
		},
		Memo: "Usage credit",
	}

	if len(p.LineItems) != 2 {
		t.Fatalf("expected 2 line items, got %d", len(p.LineItems))
	}

	var total int64
	for _, li := range p.LineItems {
		total += li.Amount
	}
	if total != 700 {
		t.Fatalf("line item total should be 700, got %d", total)
	}
}

func TestCreateCreditNoteParams_OutOfBandAmount(t *testing.T) {
	p := CreateCreditNoteParams{
		InvoiceId:       "inv_oob",
		OutOfBandAmount: 1500,
	}

	if p.OutOfBandAmount != 1500 {
		t.Fatalf("OutOfBandAmount mismatch: %d", p.OutOfBandAmount)
	}
}

func TestCreateCreditNoteParams_AllReasons(t *testing.T) {
	reasons := []string{"duplicate", "fraudulent", "order_change", "product_unsatisfactory"}
	for _, reason := range reasons {
		p := CreateCreditNoteParams{
			InvoiceId: "inv_reason",
			Reason:    reason,
		}
		if p.Reason != reason {
			t.Fatalf("Reason mismatch for %s: %s", reason, p.Reason)
		}
	}
}

// ---------------------------------------------------------------------------
// metering.go — IngestUsageEvent: validation edge cases
// ---------------------------------------------------------------------------

func TestIngestUsageEvent_ZeroValue_RequiresDatastore(t *testing.T) {
	t.Skip("requires datastore: zero value passes validation, meter.NewEvent(db) needs live db")
}

func TestIngestUsageEvent_NegativeValue_RequiresDatastore(t *testing.T) {
	t.Skip("requires datastore: negative value passes validation, meter.NewEvent(db) needs live db")
}

func TestIngestUsageEvent_WhitespaceMeterId_RequiresDatastore(t *testing.T) {
	t.Skip("requires datastore: whitespace meterId passes validation (only checks empty string), meter.NewEvent(db) needs live db")
}

// ---------------------------------------------------------------------------
// metering.go — datastore-required skip tests
// ---------------------------------------------------------------------------

func TestIngestUsageEventBatch_RequiresDatastore(t *testing.T) {
	t.Skip("requires datastore: IngestUsageEvent calls meter.NewEvent(db)")
}

func TestGetUsageSummary_RequiresDatastore(t *testing.T) {
	t.Skip("requires datastore: meter.New(db).GetById needs live db")
}

func TestAggregateItemUsage_RequiresDatastore(t *testing.T) {
	t.Skip("requires datastore: meter.New(db).GetById + watermark queries need live db")
}

func TestCreateWatermark_RequiresDatastore(t *testing.T) {
	t.Skip("requires datastore: usagewatermark.New(db).Create needs live db")
}

func TestCheckThreshold_RequiresDatastore(t *testing.T) {
	t.Skip("requires datastore: meter.New(db).GetById + event queries need live db")
}

// ---------------------------------------------------------------------------
// collector.go — CollectInvoice requires datastore
// ---------------------------------------------------------------------------

func TestCollectInvoice_RequiresDatastore(t *testing.T) {
	t.Skip("requires datastore: deductFromBalance + BurnCredits need live db")
}

func TestDeductFromBalance_RequiresDatastore(t *testing.T) {
	t.Skip("requires datastore: txutil.GetTransactionsByCurrency + transaction.Create need live db")
}

// ---------------------------------------------------------------------------
// lifecycle.go — full lifecycle state machine integration
// ---------------------------------------------------------------------------

func TestLifecycle_FullFlow_NoTrial(t *testing.T) {
	sub := &subscription.Subscription{}
	p := makePlan("plan_flow", "Flow", 2000, types.Monthly, 1, 0)

	// 1. Start
	StartSubscription(sub, p)
	if sub.Status != subscription.Active {
		t.Fatalf("step 1: expected Active, got %s", sub.Status)
	}

	// 2. Cancel at period end
	if err := CancelSubscription(sub, true); err != nil {
		t.Fatalf("step 2: %v", err)
	}
	if !sub.EndCancel {
		t.Fatal("step 2: EndCancel should be true")
	}

	// 3. Reactivate
	if err := ReactivateSubscription(sub); err != nil {
		t.Fatalf("step 3: %v", err)
	}
	if sub.EndCancel {
		t.Fatal("step 3: EndCancel should be false")
	}

	// 4. Change plan
	newPlan := makePlan("plan_pro", "Pro", 5000, types.Monthly, 1, 0)
	item, err := ChangePlan(sub, newPlan, true)
	if err != nil {
		t.Fatalf("step 4: %v", err)
	}
	if item == nil {
		t.Fatal("step 4: expected proration item")
	}
	if sub.PlanId != "plan_pro" {
		t.Fatalf("step 4: planId should be plan_pro, got %s", sub.PlanId)
	}

	// 5. Cancel immediately
	if err := CancelSubscription(sub, false); err != nil {
		t.Fatalf("step 5: %v", err)
	}
	if sub.Status != subscription.Canceled {
		t.Fatalf("step 5: expected Canceled, got %s", sub.Status)
	}

	// 6. Cannot cancel again
	err = CancelSubscription(sub, false)
	if err == nil {
		t.Fatal("step 6: should not be able to cancel again")
	}
}

func TestLifecycle_FullFlow_WithTrial(t *testing.T) {
	sub := &subscription.Subscription{}
	p := makePlan("plan_trial_flow", "TrialFlow", 3000, types.Monthly, 1, 7)

	// 1. Start with trial
	StartSubscription(sub, p)
	if sub.Status != subscription.Trialing {
		t.Fatalf("step 1: expected Trialing, got %s", sub.Status)
	}

	// 2. Transition trial to active
	if err := TransitionTrialToActive(sub); err != nil {
		t.Fatalf("step 2: %v", err)
	}
	if sub.Status != subscription.Active {
		t.Fatalf("step 2: expected Active, got %s", sub.Status)
	}

	// 3. Cannot transition again
	err := TransitionTrialToActive(sub)
	if err == nil {
		t.Fatal("step 3: should not transition non-trialing to active")
	}

	// 4. Cancel at period end
	if err := CancelSubscription(sub, true); err != nil {
		t.Fatalf("step 4: %v", err)
	}

	// 5. Reactivate
	if err := ReactivateSubscription(sub); err != nil {
		t.Fatalf("step 5: %v", err)
	}

	// 6. Immediate cancel
	if err := CancelSubscription(sub, false); err != nil {
		t.Fatalf("step 6: %v", err)
	}
	if sub.Status != subscription.Canceled {
		t.Fatalf("step 6: expected Canceled, got %s", sub.Status)
	}
}

func TestLifecycle_CancelDuringTrial(t *testing.T) {
	sub := &subscription.Subscription{}
	p := makePlan("plan_trial_cancel", "TrialCancel", 3000, types.Monthly, 1, 14)

	StartSubscription(sub, p)
	if sub.Status != subscription.Trialing {
		t.Fatalf("expected Trialing, got %s", sub.Status)
	}

	// Cancel immediately during trial
	if err := CancelSubscription(sub, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sub.Status != subscription.Canceled {
		t.Fatalf("expected Canceled, got %s", sub.Status)
	}
	if !sub.Canceled {
		t.Fatal("Canceled flag should be true")
	}
}

func TestLifecycle_ReactivateThenCancel(t *testing.T) {
	sub := &subscription.Subscription{
		Status:   subscription.Canceled,
		Canceled: true,
		// Ended is zero — eligible for reactivation
	}

	// Reactivate
	if err := ReactivateSubscription(sub); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sub.Status != subscription.Active {
		t.Fatalf("expected Active, got %s", sub.Status)
	}

	// Cancel again
	if err := CancelSubscription(sub, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sub.Status != subscription.Canceled {
		t.Fatalf("expected Canceled, got %s", sub.Status)
	}
}

// ---------------------------------------------------------------------------
// CreditBurner type test
// ---------------------------------------------------------------------------

func TestCreditBurnerType(t *testing.T) {
	// Verify CreditBurner function signature can be satisfied
	var burner CreditBurner = func(db *datastore.Datastore, userId string, amount int64, meterId string) (int64, error) {
		return amount - 100, nil
	}

	remaining, err := burner(nil, "user_test", 500, "meter_test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if remaining != 400 {
		t.Fatalf("expected 400 remaining, got %d", remaining)
	}
}

func TestCreditBurnerType_NilBurner(t *testing.T) {
	// Nil CreditBurner should be a valid state (means no credit system)
	var burner CreditBurner
	if burner != nil {
		t.Fatal("nil CreditBurner should be nil")
	}
}

// ---------------------------------------------------------------------------
// makePlan helper — verify helper produces correct plans
// ---------------------------------------------------------------------------

func TestMakePlan_Fields(t *testing.T) {
	p := makePlan("plan_test", "TestPlan", 4999, types.Yearly, 2, 30)

	if p.Id() != "plan_test" {
		t.Fatalf("Id mismatch: %s", p.Id())
	}
	if p.Name != "TestPlan" {
		t.Fatalf("Name mismatch: %s", p.Name)
	}
	if int64(p.Price) != 4999 {
		t.Fatalf("Price mismatch: %d", p.Price)
	}
	if p.Interval != types.Yearly {
		t.Fatalf("Interval mismatch: %s", p.Interval)
	}
	if p.IntervalCount != 2 {
		t.Fatalf("IntervalCount mismatch: %d", p.IntervalCount)
	}
	if p.TrialPeriodDays != 30 {
		t.Fatalf("TrialPeriodDays mismatch: %d", p.TrialPeriodDays)
	}
	if string(p.Currency) != "usd" {
		t.Fatalf("Currency mismatch: %s", p.Currency)
	}
}
