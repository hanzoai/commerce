package events

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// capture spins a collector stand-in that records the last /event body, returns a
// client wired to it, and the recorder. Mirrors the real POST /event contract.
func capture(t *testing.T) (*Client, *[]map[string]any, func()) {
	t.Helper()
	var mu sync.Mutex
	got := make([]map[string]any, 0, 8)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/event" || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		body, _ := io.ReadAll(r.Body)
		var m map[string]any
		if err := json.Unmarshal(body, &m); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		mu.Lock()
		got = append(got, m)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	return NewClient(srv.URL), &got, srv.Close
}

// requireEnvelope asserts the {event, distinct_id, organization_id, revenue,
// properties} envelope every customer-activity emit shares (mirrors EmitOrderCompleted).
func requireEnvelope(t *testing.T, m map[string]any, event, distinct, org string, revenue float64) map[string]any {
	t.Helper()
	if m["event"] != event {
		t.Fatalf("event = %v, want %q", m["event"], event)
	}
	if m["distinct_id"] != distinct {
		t.Fatalf("distinct_id = %v, want %q", m["distinct_id"], distinct)
	}
	if m["organization_id"] != org {
		t.Fatalf("organization_id = %v, want %q", m["organization_id"], org)
	}
	rv, ok := m["revenue"].(float64)
	if !ok {
		t.Fatalf("revenue missing/not-number: %v", m["revenue"])
	}
	if rv != revenue {
		t.Fatalf("revenue = %v, want %v", rv, revenue)
	}
	props, ok := m["properties"].(map[string]any)
	if !ok {
		t.Fatalf("properties missing/not-object: %v", m["properties"])
	}
	return props
}

func propNum(t *testing.T, props map[string]any, key string, want float64) {
	t.Helper()
	v, ok := props[key].(float64)
	if !ok {
		t.Fatalf("properties.%s missing/not-number: %v", key, props[key])
	}
	if v != want {
		t.Fatalf("properties.%s = %v, want %v", key, v, want)
	}
}

func propStr(t *testing.T, props map[string]any, key, want string) {
	t.Helper()
	if props[key] != want {
		t.Fatalf("properties.%s = %v, want %q", key, props[key], want)
	}
}

func TestEmitSubscriptionShapes(t *testing.T) {
	c, got, done := capture(t)
	defer done()
	ctx := context.Background()
	sub := &Subscription{
		ID: "sub_1", OrgID: "acme", UserID: "hanzo/alice",
		Plan: "pro", PlanName: "Pro", Category: "cloud", Status: "active",
		Interval: "month", PriceCents: 4900, MRRCents: 4900, Seats: 3,
		Trial: false, PeriodStart: "2026-07-01T00:00:00Z", PeriodEnd: "2026-08-01T00:00:00Z",
	}
	for _, tc := range []struct {
		name  string
		emit  func() error
		event string
	}{
		{"created", func() error { return c.EmitSubscriptionCreated(ctx, sub) }, "subscription_created"},
		{"renewed", func() error { return c.EmitSubscriptionRenewed(ctx, sub) }, "subscription_renewed"},
		{"plan_changed", func() error { return c.EmitSubscriptionPlanChanged(ctx, sub) }, "subscription_plan_changed"},
		{"canceled", func() error { return c.EmitSubscriptionCanceled(ctx, sub) }, "subscription_canceled"},
	} {
		*got = (*got)[:0]
		if err := tc.emit(); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if len(*got) != 1 {
			t.Fatalf("%s: got %d events, want 1", tc.name, len(*got))
		}
		props := requireEnvelope(t, (*got)[0], tc.event, "hanzo/alice", "acme", 0)
		propStr(t, props, "subscription_id", "sub_1")
		propStr(t, props, "plan", "pro")
		propStr(t, props, "plan_name", "Pro")
		propStr(t, props, "category", "cloud")
		propStr(t, props, "status", "active")
		propNum(t, props, "mrr_cents", 4900)
		propNum(t, props, "price_cents", 4900)
		propNum(t, props, "seats", 3)
	}
}

func TestEmitInvoiceShapes(t *testing.T) {
	c, got, done := capture(t)
	defer done()
	ctx := context.Background()
	inv := &Invoice{
		ID: "inv_1", Number: "INV-0042", OrgID: "acme", UserID: "hanzo/alice",
		Status: "paid", AmountCents: 4900, AmountPaidCents: 4900, Currency: "usd",
		SubscriptionID: "sub_1", Issued: "2026-07-01T00:00:00Z", Due: "2026-07-15T00:00:00Z",
	}
	// finalized → revenue 0
	*got = (*got)[:0]
	if err := c.EmitInvoiceFinalized(ctx, inv); err != nil {
		t.Fatal(err)
	}
	p := requireEnvelope(t, (*got)[0], "invoice_finalized", "hanzo/alice", "acme", 0)
	propStr(t, p, "invoice_id", "inv_1")
	propStr(t, p, "number", "INV-0042")
	propNum(t, p, "amount_cents", 4900)
	propStr(t, p, "currency", "usd")

	// paid → revenue = amount_paid/100 = 49.00
	*got = (*got)[:0]
	if err := c.EmitInvoicePaid(ctx, inv); err != nil {
		t.Fatal(err)
	}
	p = requireEnvelope(t, (*got)[0], "invoice_paid", "hanzo/alice", "acme", 49.00)
	propNum(t, p, "amount_paid_cents", 4900)

	// void → revenue 0
	*got = (*got)[:0]
	if err := c.EmitInvoiceVoid(ctx, inv); err != nil {
		t.Fatal(err)
	}
	requireEnvelope(t, (*got)[0], "invoice_void", "hanzo/alice", "acme", 0)
}

func TestEmitAPIUsageDebitShape(t *testing.T) {
	c, got, done := capture(t)
	defer done()
	ctx := context.Background()
	u := &APIUsage{
		OrgID: "acme", UserID: "hanzo/alice", AmountCents: 250, AmountMicros: 2_500_000,
		Model: "zen", Provider: "hanzo", Project: "p1", Service: "chat",
		RequestID: "req_9", TotalTokens: 1234, Status: "ok",
	}
	if err := c.EmitAPIUsageDebit(ctx, u); err != nil {
		t.Fatal(err)
	}
	// revenue = 250 cents / 100 = 2.50
	p := requireEnvelope(t, (*got)[0], "api_usage_debit", "hanzo/alice", "acme", 2.50)
	propNum(t, p, "amount_cents", 250)
	propNum(t, p, "amount_micros", 2_500_000)
	propStr(t, p, "model", "zen")
	propStr(t, p, "provider", "hanzo")
	propNum(t, p, "total_tokens", 1234)
}

// TestEmitNoCollectorNoop verifies an unset endpoint no-ops (never blocks the money path).
func TestEmitNoCollectorNoop(t *testing.T) {
	c := NewClient("")
	if err := c.EmitSubscriptionCreated(context.Background(), &Subscription{ID: "x"}); err != nil {
		t.Fatalf("no-collector emit must be a silent no-op, got %v", err)
	}
}
