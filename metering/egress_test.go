package metering_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hanzoai/commerce/metering"
)

// Egress is metered as a first-class dimension on the canonical usage path:
// POST /v1/billing/usage with Provider="egress", amount = GB × rate.
func TestRecordEgress_MetersOutboundGB(t *testing.T) {
	fc := &fakeCommerce{status: 201, reply: `{"transactionId":"tx_eg","user":"hanzo","amount":250,"currency":"usd","type":"withdraw"}`}
	srv := httptest.NewServer(fc.handler())
	defer srv.Close()

	c := newClient(t, srv, metering.Config{})
	// 250 GB overage × 1c/GB (cost-recovery default) = 250c.
	res, err := c.RecordEgress(context.Background(), metering.EgressUsage{
		User: "hanzo",
		Org:  "hanzo",
		GB:   250,
	})
	if err != nil {
		t.Fatalf("RecordEgress: %v", err)
	}
	if res == nil || res.Amount != 250 {
		t.Fatalf("unexpected RecordResult: %+v", res)
	}
	if fc.path != "/v1/billing/usage" {
		t.Errorf("path = %s, want /v1/billing/usage", fc.path)
	}

	var body map[string]any
	if err := json.Unmarshal(fc.body, &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["provider"] != metering.ProviderEgress {
		t.Errorf("body.provider = %v, want %q", body["provider"], metering.ProviderEgress)
	}
	if body["amount"].(float64) != 250 {
		t.Errorf("body.amount = %v, want 250 (250GB × 1c)", body["amount"])
	}
	// Org travels via the X-Org-Id header, never the JSON body.
	if _, ok := body["Org"]; ok {
		t.Error("Org leaked into the JSON body; it must be a header only")
	}
}

// A plan may price egress above the cost-recovery default.
func TestRecordEgress_CustomRate(t *testing.T) {
	fc := &fakeCommerce{status: 201, reply: `{"transactionId":"tx_eg2","user":"hanzo","amount":200,"currency":"usd","type":"withdraw"}`}
	srv := httptest.NewServer(fc.handler())
	defer srv.Close()

	c := newClient(t, srv, metering.Config{})
	// 100 GB × 2c/GB override = 200c.
	res, err := c.RecordEgress(context.Background(), metering.EgressUsage{
		User: "hanzo", Org: "hanzo", GB: 100, CentsPerGB: 2,
	})
	if err != nil {
		t.Fatalf("RecordEgress: %v", err)
	}
	if res == nil || res.Amount != 200 {
		t.Fatalf("unexpected RecordResult: %+v", res)
	}
}

// Zero billable GB must not touch commerce (matches Record's zero-amount no-op).
func TestRecordEgress_ZeroGB_IsNoOp(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer srv.Close()

	c := newClient(t, srv, metering.Config{})
	res, err := c.RecordEgress(context.Background(), metering.EgressUsage{User: "hanzo", Org: "hanzo", GB: 0})
	if err != nil || res != nil {
		t.Fatalf("zero-GB RecordEgress should be (nil,nil), got (%v,%v)", res, err)
	}
	if called {
		t.Error("zero-GB RecordEgress must not hit commerce")
	}
}
