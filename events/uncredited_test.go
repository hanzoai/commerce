package events

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The alarm has to LEAVE the process. A money defect that is only written to a
// log is what this event exists to end — a $99 charge settled and sat
// uncredited for about twenty-one hours behind a line nobody read — so the test
// that matters is the one that reads what actually goes out on the wire.
func TestEmitPaymentUncredited_ReachesTheCollector(t *testing.T) {
	var got map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/event" {
			t.Errorf("posted to %q, want /event", r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(b, &got); err != nil {
			t.Errorf("collector received unparseable body %q: %v", string(b), err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	err := NewClient(srv.URL).EmitPaymentUncredited(context.Background(),
		"hanzo", "hanzo/a", "9W48BXbFob6LBHejppWOjr6f9JPZY",
		"the first-period charge settled and no subscription was created", 9900, true)
	if err != nil {
		t.Fatalf("EmitPaymentUncredited: %v", err)
	}

	if got["event"] != EventPaymentUncredited {
		t.Fatalf("event = %v, want %q", got["event"], EventPaymentUncredited)
	}
	if got["organization_id"] != "hanzo" || got["distinct_id"] != "hanzo/a" {
		t.Fatalf("org/subject = %v/%v, want hanzo/hanzo/a", got["organization_id"], got["distinct_id"])
	}
	// revenue stays 0: the cash moved at the PROCESSOR and the ledger did not,
	// so booking it here would report revenue the ledger never recorded — the
	// exact disagreement this event is reporting.
	if rev, _ := got["revenue"].(float64); rev != 0 {
		t.Fatalf("revenue = %v, want 0 — this event reports a defect, it does not book money", rev)
	}

	props, _ := got["properties"].(map[string]interface{})
	if props == nil {
		t.Fatal("no properties — the alarm carries the facts a human needs to act")
	}
	// The settlement id is the whole point: it is what reconciles the alarm
	// against the processor, and what support quotes back to the customer.
	if props["settlement"] != "9W48BXbFob6LBHejppWOjr6f9JPZY" {
		t.Fatalf("settlement = %v, want the processor payment id", props["settlement"])
	}
	if amt, _ := props["amount_cents"].(float64); amt != 9900 {
		t.Fatalf("amount_cents = %v, want 9900", amt)
	}
	// terminal separates "the customer's own retry fixes this" from "a human
	// must grant the balance and refund the charge". Telling the second class to
	// retry takes their card twice.
	if props["terminal"] != true {
		t.Fatalf("terminal = %v, want true", props["terminal"])
	}
	if props["reason"] == "" || props["reason"] == nil {
		t.Fatal("no reason — an alarm that cannot say what happened is a page with no next step")
	}
}

// No collector configured must stay silent rather than fail: the emit is
// best-effort and sits on the money path, so it can never be the thing that
// breaks a charge.
func TestEmitPaymentUncredited_NoCollectorIsNotAnError(t *testing.T) {
	if err := NewClient("").EmitPaymentUncredited(context.Background(),
		"hanzo", "hanzo/a", "ref", "reason", 100, false); err != nil {
		t.Fatalf("with no collector: %v, want nil", err)
	}
}
