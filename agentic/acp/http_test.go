// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package acp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// spec returns one example of examples.agentic_checkout.json as the bytes the spec gives.
func spec(t testing.TB, key string) string {
	t.Helper()
	b, err := os.ReadFile("testdata/examples.agentic_checkout.json")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	raw, ok := m[key]
	if !ok {
		t.Fatalf("no spec example %s", key)
	}
	return string(raw)
}

// every 2xx session the service writes is judged by the spec's schema.
func validSession(t testing.TB, w res, def string) {
	t.Helper()
	loadSchema(t).valid(t, def, w.Body.Bytes())
}

func validError(t testing.TB, w res) Error {
	t.Helper()
	loadSchema(t).valid(t, "Error", w.Body.Bytes())
	var e Error
	if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil {
		t.Fatal(err)
	}
	return e
}

// wantErr asserts a protocol error: status, type, code, and param when it is not empty.
func wantErr(t testing.TB, w res, status int, typ, code, param string) Error {
	t.Helper()
	if w.Code != status {
		t.Fatalf("status = %d; want %d: %s", w.Code, status, w.Body)
	}
	e := validError(t, w)
	if e.Type != typ || e.Code != code || e.Param != param {
		t.Fatalf("error = %+v; want %s/%s %q", e, typ, code, param)
	}
	return e
}

// ---- the five operations, driven by the spec's examples ----

func TestCreateAnswersTheSpecExampleRequestWithASession(t *testing.T) {
	r := newRig(t)
	w := r.post("/checkout_sessions", spec(t, "create_checkout_session_request"), map[string]string{"Request-Id": "request_id_123", "Idempotency-Key": "550e8400-e29b-41d4-a716-446655440000"})
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d: %s", w.Code, w.Body)
	}
	validSession(t, w, "CheckoutSession")
	if got := w.Header().Get("Idempotency-Key"); got != "550e8400-e29b-41d4-a716-446655440000" {
		t.Fatalf("Idempotency-Key not echoed: %q", got)
	}
	if got := w.Header().Get("Request-Id"); got != "request_id_123" {
		t.Fatalf("Request-Id not echoed: %q", got)
	}
	if w.Header().Get("Idempotent-Replayed") != "" || w.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("headers = %v", w.Header())
	}

	// What the protocol fixes is what the spec's example response says; what the seller
	// chooses (its ids, prices, tax, shipping, handlers, links, interventions) is not compared.
	var want, got map[string]any
	_ = json.Unmarshal([]byte(spec(t, "create_checkout_session_response")), &want)
	got = w.json()
	for _, k := range []string{"protocol", "status", "currency", "messages"} {
		if !equal(want[k], got[k]) {
			t.Errorf("%s = %v; the spec's example has %v", k, got[k], want[k])
		}
	}
	wl, gl := want["line_items"].([]any)[0].(map[string]any), got["line_items"].([]any)[0].(map[string]any)
	if !equal(wl["item"], gl["item"]) || !equal(wl["quantity"], gl["quantity"]) || wl["unit_amount"] != gl["unit_amount"] {
		t.Errorf("line item = %v; the spec's example has %v", gl, wl)
	}
	// The negotiated interventions are the intersection with what the seller runs: none.
	iv := got["capabilities"].(map[string]any)["interventions"].(map[string]any)
	if s := iv["supported"].([]any); len(s) != 0 {
		t.Errorf("interventions.supported = %v; the seller runs none in-band", s)
	}
	h := got["capabilities"].(map[string]any)["payment"].(map[string]any)["handlers"].([]any)[0].(map[string]any)
	if h["id"] != "card_tokenized" || h["name"] != "dev.acp.tokenized.card" || h["psp"] != "stripe" {
		t.Errorf("handler = %v", h)
	}
	if r.be.opens != 1 {
		t.Fatalf("opens = %d", r.be.opens)
	}
}

func TestCreateWithTheMultiItemAndAttributionExamples(t *testing.T) {
	r := newRig(t)
	// Two items cannot be honoured: the seller sells one item per checkout.
	two := `{"currency":"usd","line_items":[{"id":"item_123","unit_amount":300},{"id":"item_456","unit_amount":500}],"capabilities":{}}`
	wantErr(t, r.post("/checkout_sessions", two), 400, "invalid_request", "unsupported", "$.line_items")

	// First-touch attribution is accepted and not stored: it is validated, not applied.
	w := r.post("/checkout_sessions", spec(t, "create_checkout_session_request_with_first_touch_attribution"))
	if w.Code != 201 {
		t.Fatalf("status = %d: %s", w.Code, w.Body)
	}
	validSession(t, w, "CheckoutSession")
}

func TestUpdateRetrieveCompleteCancelFollowTheSessionThroughItsStates(t *testing.T) {
	r := newRig(t)
	id := r.create()
	path := "/checkout_sessions/" + id

	w := r.get(path)
	if w.Code != 200 || w.get("status") != "ready_for_payment" {
		t.Fatalf("get = %d %s", w.Code, w.Body)
	}
	validSession(t, w, "CheckoutSession")

	// Update replaces the item and answers the new authoritative state.
	w = r.post(path, `{"line_items":[{"id":"item_456","unit_amount":500}]}`)
	if w.Code != 200 {
		t.Fatalf("update = %d %s", w.Code, w.Body)
	}
	validSession(t, w, "CheckoutSession")
	if w.get("totals").([]any)[2].(map[string]any)["amount"] != float64(500) || len(w.get("messages").([]any)) != 0 {
		t.Fatalf("update = %s", w.Body)
	}

	// Complete creates the order and answers completed with it.
	w = r.post(path+"/complete", spec(t, "complete_checkout_session_request"))
	if w.Code != 200 {
		t.Fatalf("complete = %d %s", w.Code, w.Body)
	}
	validSession(t, w, "CheckoutSessionWithOrder")
	if w.get("status") != "completed" {
		t.Fatalf("status = %v", w.get("status"))
	}
	o := w.get("order").(map[string]any)
	if o["id"] != "ord_"+id || o["checkout_session_id"] != id || o["permalink_url"] != "https://www.testshop.com/orders/ord_"+id {
		t.Fatalf("order = %v", o)
	}

	// A completed session is read as completed with its order, and can no longer change or cancel.
	if w = r.get(path); w.get("status") != "completed" || w.get("order", "id") != "ord_"+id {
		t.Fatalf("get after complete = %s", w.Body)
	}
	wantErr(t, r.post(path, `{"line_items":[{"id":"item_123"}]}`), 409, "invalid_request", "conflict", "")
	wantErr(t, r.post(path+"/cancel", ``), 405, "invalid_request", "not_cancelable", "")
}

func TestCancelAnswersCanceledAndAgainIs405(t *testing.T) {
	r := newRig(t)
	id := r.create()
	w := r.post("/checkout_sessions/"+id+"/cancel", spec(t, "cancel_checkout_session_request"))
	if w.Code != 200 || w.get("status") != "canceled" {
		t.Fatalf("cancel = %d %s", w.Code, w.Body)
	}
	validSession(t, w, "CheckoutSession")
	m := w.get("messages").([]any)[0].(map[string]any)
	want := map[string]any{}
	_ = json.Unmarshal([]byte(spec(t, "cancel_checkout_session_response")), &want)
	if !equal(m, want["messages"].([]any)[0]) {
		t.Fatalf("message = %v; the spec's example has %v", m, want["messages"])
	}
	wantErr(t, r.post("/checkout_sessions/"+id+"/cancel", spec(t, "cancel_checkout_session_request_timing_deferred")), 405, "invalid_request", "not_cancelable", "")
	// A canceled session can be neither updated nor completed.
	wantErr(t, r.post("/checkout_sessions/"+id, `{"line_items":[{"id":"item_456"}]}`), 409, "invalid_request", "conflict", "")
	wantErr(t, r.post("/checkout_sessions/"+id+"/complete", completeBody), 409, "invalid_request", "conflict", "")
}

func TestCancelWithoutABodyAndWithABadIntentTrace(t *testing.T) {
	r := newRig(t)
	id := r.create()
	wantErr(t, r.post("/checkout_sessions/"+id+"/cancel", `{"intent_trace":{"reason_code":"vibes"}}`), 400, "invalid_request", "invalid", "$.intent_trace.reason_code")
	wantErr(t, r.post("/checkout_sessions/"+id+"/cancel", `{"intent_trace":{"trace_summary":"x"}}`), 400, "invalid_request", "missing_required_field", "$.intent_trace.reason_code")
	wantErr(t, r.post("/checkout_sessions/"+id+"/cancel", `{"intent_trace":{"reason_code":"other","metadata":{"k":[1]}}}`), 400, "invalid_request", "invalid", "$.intent_trace.metadata.k")
	if w := r.post("/checkout_sessions/"+id+"/cancel", ``); w.Code != 200 || w.get("status") != "canceled" {
		t.Fatalf("cancel without a body = %d %s", w.Code, w.Body)
	}
}

func TestSessionsOfAnotherCallerAreNotFound(t *testing.T) {
	r := newRig(t)
	id := r.create()
	other := map[string]string{"Authorization": "Bearer other_key"}
	wantErr(t, r.get("/checkout_sessions/"+id, other), 404, "invalid_request", "not_found", "")
	wantErr(t, r.post("/checkout_sessions/"+id, `{"line_items":[{"id":"item_456"}]}`, other), 404, "invalid_request", "not_found", "")
	wantErr(t, r.post("/checkout_sessions/"+id+"/complete", completeBody, other), 404, "invalid_request", "not_found", "")
	wantErr(t, r.post("/checkout_sessions/"+id+"/cancel", ``, other), 404, "invalid_request", "not_found", "")
	wantErr(t, r.get("/checkout_sessions/nope"), 404, "invalid_request", "not_found", "")
}

func TestAnItemTheSellerDoesNotSellIsTheSpecsInvalidItemError(t *testing.T) {
	r := newRig(t)
	body := strings.Replace(createBody, "item_123", "invalid_item", 1)
	w := r.post("/checkout_sessions", body)
	e := wantErr(t, w, 400, "invalid_request", "invalid_item_id", "$.line_items[0].item.id")
	var want Error
	if err := json.Unmarshal([]byte(spec(t, "error_400_invalid_item")), &want); err != nil {
		t.Fatal(err)
	}
	if !equal(e, want) {
		t.Fatalf("error = %+v; the spec's example is %+v", e, want)
	}
	// The same on update, which opens nothing.
	id := r.create()
	wantErr(t, r.post("/checkout_sessions/"+id, `{"line_items":[{"id":"invalid_item"}]}`), 400, "invalid_request", "invalid_item_id", "$.line_items[0].item.id")
}

// ---- headers: version, authorization, request id ----

func TestAPIVersionIsRequiredAndCheckedWithTheSupportedVersions(t *testing.T) {
	r := newRig(t)
	for _, c := range []call{
		{method: "POST", path: "/checkout_sessions", body: createBody},
		{method: "GET", path: "/checkout_sessions/checkout_session_1"},
		{method: "POST", path: "/checkout_sessions/checkout_session_1/complete", body: completeBody},
	} {
		c.hdr = map[string]string{"API-Version": ""}
		e := wantErr(t, r.send(c), 400, "invalid_request", "missing_api_version", "")
		if len(e.SupportedVersions) != 1 || e.SupportedVersions[0] != "2026-04-17" {
			t.Fatalf("supported_versions = %v", e.SupportedVersions)
		}
		c.hdr = map[string]string{"API-Version": "2025-01-01"}
		e = wantErr(t, r.send(c), 400, "invalid_request", "unsupported_api_version", "")
		if e.Message != "API version '2025-01-01' is not supported" || len(e.SupportedVersions) != 1 {
			t.Fatalf("error = %+v", e)
		}
	}
	if r.be.opens != 0 {
		t.Fatal("a request with a bad version reached the backend")
	}
}

func TestAuthorizationIsRequiredOnEveryEndpoint(t *testing.T) {
	r := newRig(t)
	for _, h := range []string{"", "Basic abc", "Bearer ", "Bearer wrong_key", "agent_key"} {
		for _, c := range []call{
			{method: "POST", path: "/checkout_sessions", body: createBody},
			{method: "POST", path: "/checkout_sessions/x", body: `{}`},
			{method: "GET", path: "/checkout_sessions/x"},
			{method: "POST", path: "/checkout_sessions/x/complete", body: completeBody},
			{method: "POST", path: "/checkout_sessions/x/cancel"},
		} {
			c.hdr = map[string]string{"Authorization": h}
			wantErr(t, r.send(c), 401, "invalid_request", "unauthorized", "")
		}
	}
	if r.be.opens != 0 {
		t.Fatal("an unauthenticated request reached the backend")
	}
}

func TestRequestIdIsEchoedOnErrorsToo(t *testing.T) {
	r := newRig(t)
	w := r.get("/checkout_sessions/nope", map[string]string{"Request-Id": "req-9"})
	if w.Code != 404 || w.Header().Get("Request-Id") != "req-9" {
		t.Fatalf("%d %v", w.Code, w.Header())
	}
	w = r.post("/checkout_sessions", createBody, map[string]string{"Request-Id": "req-10", "API-Version": ""})
	if w.Code != 400 || w.Header().Get("Request-Id") != "req-10" {
		t.Fatalf("%d %v", w.Code, w.Header())
	}
	if w = r.get("/checkout_sessions/nope"); w.Header().Get("Request-Id") != "" {
		t.Fatal("a Request-Id was made up")
	}
}

func TestRoutesAndMethods(t *testing.T) {
	r := newRig(t)
	wantErr(t, r.send(call{method: "GET", path: "/elsewhere"}), 404, "invalid_request", "not_found", "")
	wantErr(t, r.send(call{method: "GET", path: "/checkout_sessions/a/b/c"}), 404, "invalid_request", "not_found", "")
	wantErr(t, r.send(call{method: "GET", path: "/checkout_sessions/a/refund"}), 404, "invalid_request", "not_found", "")
	w := r.send(call{method: "GET", path: "/checkout_sessions"})
	wantErr(t, w, 405, "invalid_request", "method_not_allowed", "")
	if w.Header().Get("Allow") != "POST" {
		t.Fatalf("Allow = %q", w.Header().Get("Allow"))
	}
	w = r.send(call{method: "DELETE", path: "/checkout_sessions/a"})
	wantErr(t, w, 405, "invalid_request", "method_not_allowed", "")
	if w.Header().Get("Allow") != "GET, POST" {
		t.Fatalf("Allow = %q", w.Header().Get("Allow"))
	}
	wantErr(t, r.send(call{method: "GET", path: "/checkout_sessions/a/complete"}), 405, "invalid_request", "method_not_allowed", "")
}

// ---- request validation ----

func TestRequestsAreValidatedAtTheBoundary(t *testing.T) {
	r := newRig(t)
	for _, tc := range []struct {
		name, path, body string
		status           int
		code, param      string
	}{
		{"no line items", "/checkout_sessions", `{"currency":"usd","capabilities":{}}`, 400, "missing_required_field", "$.line_items"},
		{"empty line items", "/checkout_sessions", `{"currency":"usd","line_items":[],"capabilities":{}}`, 400, "missing_required_field", "$.line_items"},
		{"no currency", "/checkout_sessions", `{"line_items":[{"id":"item_123"}],"capabilities":{}}`, 400, "missing_required_field", "$.currency"},
		{"no capabilities", "/checkout_sessions", `{"currency":"usd","line_items":[{"id":"item_123"}]}`, 400, "missing_required_field", "$.capabilities"},
		{"null capabilities", "/checkout_sessions", `{"currency":"usd","line_items":[{"id":"item_123"}],"capabilities":null}`, 400, "missing_required_field", "$.capabilities"},
		{"item without id", "/checkout_sessions", `{"currency":"usd","line_items":[{"name":"x"}],"capabilities":{}}`, 400, "missing_required_field", "$.line_items[0].id"},
		{"another currency", "/checkout_sessions", `{"currency":"eur","line_items":[{"id":"item_123"}],"capabilities":{}}`, 400, "unsupported", "$.currency"},
		{"a quantity the schema does not define", "/checkout_sessions", `{"currency":"usd","line_items":[{"id":"item_123","quantity":2}],"capabilities":{}}`, 400, "invalid", ""},
		{"an unknown field", "/checkout_sessions", `{"currency":"usd","line_items":[{"id":"item_123"}],"capabilities":{},"surprise":1}`, 400, "invalid", ""},
		{"a wrong type", "/checkout_sessions", `{"currency":"usd","line_items":[{"id":"item_123","unit_amount":"300"}],"capabilities":{}}`, 400, "invalid", "$.line_items[0].unit_amount"},
		{"buyer without email", "/checkout_sessions", `{"currency":"usd","line_items":[{"id":"item_123"}],"capabilities":{},"buyer":{"first_name":"A"}}`, 400, "missing_required_field", "$.buyer.email"},
		{"address without a city", "/checkout_sessions", `{"currency":"usd","line_items":[{"id":"item_123"}],"capabilities":{},"fulfillment_details":{"address":{"name":"a","line_one":"b","state":"c","country":"US","postal_code":"1"}}}`, 400, "missing_required_field", "$.fulfillment_details.address.city"},
		{"order notes too long", "/checkout_sessions", `{"currency":"usd","line_items":[{"id":"item_123"}],"capabilities":{},"order_notes":"` + strings.Repeat("x", 5001) + `"}`, 400, "invalid", "$.order_notes"},
		{"data after the document", "/checkout_sessions", createBody + ` {}`, 400, "invalid_json", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wantErr(t, r.post(tc.path, tc.body), tc.status, "invalid_request", tc.code, tc.param)
		})
	}
	if r.be.opens != 0 {
		t.Fatal("an invalid request reached the backend")
	}
}

func TestUpdateIsValidated(t *testing.T) {
	r := newRig(t)
	id := r.create()
	p := "/checkout_sessions/" + id
	wantErr(t, r.post(p, `{"line_items":null}`), 400, "invalid_request", "invalid", "$.line_items")
	wantErr(t, r.post(p, `{"line_items":[{"id":"item_123"},{"id":"item_456"}]}`), 400, "invalid_request", "unsupported", "$.line_items")
	wantErr(t, r.post(p, `{"line_items":[{"id":""}]}`), 400, "invalid_request", "missing_required_field", "$.line_items[0].id")
	// Only the digital option the session offers can be selected (rfc §8: option_id MUST match).
	wantErr(t, r.post(p, spec(t, "update_checkout_session_request")), 400, "invalid_request", "invalid", "$.selected_fulfillment_options[0].type")
	wantErr(t, r.post(p, `{"selected_fulfillment_options":[{"type":"digital","option_id":"fulfillment_option_9","item_ids":["item_123"]}]}`), 400, "invalid_request", "invalid", "$.selected_fulfillment_options[0].option_id")
	wantErr(t, r.post(p, `{"selected_fulfillment_options":[{"type":"digital","option_id":"digital","item_ids":["item_999"]}]}`), 400, "invalid_request", "invalid", "$.selected_fulfillment_options[0].item_ids[0]")
	w := r.post(p, `{"selected_fulfillment_options":[{"type":"digital","option_id":"digital","item_ids":["item_123"]}],"order_notes":"hi"}`)
	if w.Code != 200 {
		t.Fatalf("selecting the offered option = %d %s", w.Code, w.Body)
	}
	validSession(t, w, "CheckoutSession")
	// An update that changes nothing answers the state as it is.
	if w = r.post(p, `{"buyer":{"email":"a@b.co"}}`); w.Code != 200 || w.get("line_items").([]any)[0].(map[string]any)["item"].(map[string]any)["id"] != "item_123" {
		t.Fatalf("update = %d %s", w.Code, w.Body)
	}
}

func TestMalformedAndMisTypedBodiesAreRefused(t *testing.T) {
	r := newRig(t)
	wantErr(t, r.post("/checkout_sessions", `{not json`), 400, "invalid_request", "invalid_json", "")
	wantErr(t, r.post("/checkout_sessions", ``), 400, "invalid_request", "invalid_json", "")
	wantErr(t, r.post("/checkout_sessions", createBody, map[string]string{"Content-Type": "text/plain"}), 415, "invalid_request", "unsupported_media_type", "")
	if w := r.post("/checkout_sessions", createBody, map[string]string{"Content-Type": "application/json; charset=utf-8"}); w.Code != 201 {
		t.Fatalf("a charset parameter is refused: %d %s", w.Code, w.Body)
	}
	big := `{"metadata":{"x":"` + strings.Repeat("a", maxBody) + `"}}`
	wantErr(t, r.post("/checkout_sessions", big), 413, "invalid_request", "request_too_large", "")
}

func TestMalformedJSONDoesNotUseTheIdempotencyKey(t *testing.T) {
	r := newRig(t)
	k := map[string]string{"Idempotency-Key": "k-malformed"}
	wantErr(t, r.post("/checkout_sessions", `{not json`, k), 400, "invalid_request", "invalid_json", "")
	if w := r.post("/checkout_sessions", createBody, k); w.Code != 201 {
		t.Fatalf("the key was used by a request that was not JSON: %d %s", w.Code, w.Body)
	}
}

// ---- warnings ----

func TestAPriceThatIsNotTheOneExpectedIsAWarning(t *testing.T) {
	r := newRig(t)
	w := r.post("/checkout_sessions", `{"currency":"usd","line_items":[{"id":"item_123","unit_amount":250}],"capabilities":{}}`)
	if w.Code != 201 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	validSession(t, w, "CheckoutSession")
	m := w.get("messages").([]any)
	if len(m) != 1 {
		t.Fatalf("messages = %v", m)
	}
	got := m[0].(map[string]any)
	if got["type"] != "warning" || got["code"] != "price_change" || got["param"] != "$.line_items[0].unit_amount" || got["resolution"] != "requires_buyer_review" {
		t.Fatalf("message = %v", got)
	}
	// The price the agent expected is the price: no message.
	if w = r.post("/checkout_sessions", `{"currency":"usd","line_items":[{"id":"item_123","unit_amount":300}],"capabilities":{}}`); len(w.get("messages").([]any)) != 0 {
		t.Fatalf("messages = %v", w.get("messages"))
	}
}

func TestDiscountCodesAreNotAcceptedInSilence(t *testing.T) {
	r := newRig(t)
	w := r.post("/checkout_sessions", `{"currency":"usd","line_items":[{"id":"item_123"}],"capabilities":{},"coupons":["OLD"],"discounts":{"codes":["SAVE10","EXPIRED50"]}}`)
	if w.Code != 201 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	validSession(t, w, "CheckoutSession")
	var params []string
	for _, m := range w.get("messages").([]any) {
		mm := m.(map[string]any)
		if mm["type"] != "warning" || mm["code"] != "discount_code_invalid" {
			t.Fatalf("message = %v", mm)
		}
		params = append(params, mm["param"].(string))
	}
	if strings.Join(params, " ") != "$.coupons[0] $.discounts.codes[0] $.discounts.codes[1]" {
		t.Fatalf("params = %v", params)
	}
	if _, ok := w.json()["discounts"]; ok {
		t.Fatal("a discounts object was returned by a seller that runs no discount extension")
	}
}

// ---- complete: payment_data ----

func TestCompleteRefusesPaymentDataTheSellerCannotHonour(t *testing.T) {
	r := newRig(t)
	id := r.create()
	p := "/checkout_sessions/" + id + "/complete"
	pd := func(extra string) string {
		return `{"payment_data":{"handler_id":"card_tokenized","instrument":{"type":"card","credential":{"type":"spt","token":"spt_1"}}` + extra + `}}`
	}
	for _, tc := range []struct {
		name, body, code, param string
	}{
		{"no payment_data", `{}`, "missing_required_field", "$.payment_data"},
		{"a purchase order only", `{"payment_data":{"purchase_order_number":"PO-1"}}`, "unsupported", "$.payment_data.purchase_order_number"},
		{"payment terms", pd(`,"payment_terms":"net_30"`), "unsupported", "$.payment_data.payment_terms"},
		{"a due date", pd(`,"due_date":"2026-05-01T00:00:00Z"`), "unsupported", "$.payment_data.due_date"},
		{"approval", pd(`,"approval_required":false`), "unsupported", "$.payment_data.approval_required"},
		{"no handler", `{"payment_data":{"instrument":{"type":"card","credential":{"type":"spt","token":"spt_1"}}}}`, "missing_required_field", "$.payment_data.handler_id"},
		{"no instrument", `{"payment_data":{"handler_id":"card_tokenized"}}`, "missing_required_field", "$.payment_data.instrument"},
		{"no instrument type", `{"payment_data":{"handler_id":"card_tokenized","instrument":{"credential":{"type":"spt","token":"spt_1"}}}}`, "missing_required_field", "$.payment_data.instrument.type"},
		{"no credential type", `{"payment_data":{"handler_id":"card_tokenized","instrument":{"type":"card","credential":{"token":"spt_1"}}}}`, "missing_required_field", "$.payment_data.instrument.credential.type"},
		{"no token", `{"payment_data":{"handler_id":"card_tokenized","instrument":{"type":"card","credential":{"type":"spt"}}}}`, "missing_required_field", "$.payment_data.instrument.credential.token"},
		{"a handler the session does not offer", spec(t, "complete_checkout_session_request_seller_backed"), "invalid", "$.payment_data.handler_id"},
		{"an instrument of another handler", `{"payment_data":{"handler_id":"card_tokenized","instrument":{"handler_id":"seller_pm_123","type":"card","credential":{"type":"spt","token":"spt_1"}}}}`, "invalid", "$.payment_data.instrument.handler_id"},
		{"an authentication result", spec(t, "complete_session_with_authentication_result_request"), "unsupported", "$.authentication_result"},
		{"a credential that is not a shared payment token", `{"payment_data":{"handler_id":"card_tokenized","instrument":{"type":"card","credential":{"type":"card_number","token":"4242424242424242"}}}}`, "unsupported", "$.payment_data.instrument"},
		{"an incomplete attribution", `{"payment_data":{"handler_id":"card_tokenized","instrument":{"type":"card","credential":{"type":"spt","token":"spt_1"}}},"affiliate_attribution":{"touchpoint":"last"}}`, "invalid", "$.affiliate_attribution"},
		{"a buyer without email", `{"buyer":{"first_name":"J"},"payment_data":{"handler_id":"card_tokenized","instrument":{"type":"card","credential":{"type":"spt","token":"spt_1"}}}}`, "missing_required_field", "$.buyer.email"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wantErr(t, r.post(p, tc.body), 400, "invalid_request", tc.code, tc.param)
		})
	}
	if r.be.pays != 0 {
		t.Fatal("a payment the seller cannot honour reached the backend")
	}
	// The instrument may carry the base fields of the payment handler RFC and properties
	// the schema leaves open; they do not stop a payment the seller can take.
	open := `{"payment_data":{"handler_id":"card_tokenized","instrument":{"id":"inst_1","handler_id":"card_tokenized","type":"card","credential":{"type":"spt","token":"spt_1","network":"visa"},"last4":"4242"},"billing_address":{"name":"a","line_one":"b","city":"c","state":"d","country":"US","postal_code":"1"}},"risk_signals":{"ip_address":"203.0.113.9"},"marketing_consents":[{"channel":"email","opted_in":true}],"order_notes":"gift"}`
	if w := r.post(p, open); w.Code != 200 || w.get("status") != "completed" {
		t.Fatalf("complete = %d %s", w.Code, w.Body)
	}
}

func TestACompleteTheSpecShowsWithAttribution(t *testing.T) {
	r := newRig(t)
	id := r.create()
	w := r.post("/checkout_sessions/"+id+"/complete", spec(t, "complete_checkout_session_request_with_last_touch_attribution"))
	if w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	validSession(t, w, "CheckoutSessionWithOrder")
	if _, ok := w.json()["affiliate_attribution"]; ok {
		t.Fatal("attribution is write-only and must not be returned")
	}
}

func TestADeclinedPaymentLeavesTheSessionUsableAndSaysWhy(t *testing.T) {
	r := newRig(t)
	id := r.create()
	r.be.refuse = &Refusal{Code: "payment_declined", Resolution: "recoverable", Message: "Your payment was declined. Please try a different payment method."}
	w := r.post("/checkout_sessions/"+id+"/complete", completeBody)
	if w.Code != 200 {
		t.Fatalf("complete = %d %s", w.Code, w.Body)
	}
	// A session, not a protocol error, as the spec's checkout_session_with_payment_declined example.
	validSession(t, w, "CheckoutSession")
	if w.get("status") != "ready_for_payment" {
		t.Fatalf("status = %v", w.get("status"))
	}
	if _, ok := w.json()["order"]; ok {
		t.Fatal("a declined payment returned an order")
	}
	var want map[string]any
	_ = json.Unmarshal([]byte(spec(t, "checkout_session_with_payment_declined")), &want)
	got := w.get("messages").([]any)[0].(map[string]any)
	wm := want["messages"].([]any)[0].(map[string]any)
	for _, k := range []string{"type", "code", "content_type", "content"} {
		if got[k] != wm[k] {
			t.Errorf("message %s = %v; the spec's example has %v", k, got[k], wm[k])
		}
	}
	if len(r.ev.got) != 0 {
		t.Fatal("an order event was sent for a declined payment")
	}
	// The agent tries again, with another token and key, and the session completes.
	w = r.post("/checkout_sessions/"+id+"/complete", strings.Replace(completeBody, "spt_123", "spt_456", 1))
	if w.Code != 200 || w.get("status") != "completed" {
		t.Fatalf("retry = %d %s", w.Code, w.Body)
	}
	// The same session completed under another key is refused, not charged again.
	wantErr(t, r.post("/checkout_sessions/"+id+"/complete", strings.Replace(completeBody, "spt_123", "spt_789", 1)), 409, "invalid_request", "conflict", "")
	if r.be.pays != 2 {
		t.Fatalf("pays = %d; want the declined attempt and the paid one", r.be.pays)
	}
}

// ---- the order webhook on completion ----

func TestCompletionSendsOrderCreateOnceWithTheFullOrder(t *testing.T) {
	r := newRig(t)
	id := r.create()
	k := map[string]string{"Idempotency-Key": "k-complete"}
	r.post("/checkout_sessions/"+id+"/complete", completeBody, k)
	r.post("/checkout_sessions/"+id+"/complete", completeBody, k) // a replay: no second event
	if len(r.ev.got) != 1 {
		t.Fatalf("%d events", len(r.ev.got))
	}
	ev := r.ev.got[0]
	if ev.Type != EventOrderCreate || ev.Data.Type != "order" || ev.Data.ID != "ord_"+id || ev.Data.CheckoutSessionID != id ||
		ev.Data.PermalinkURL != "https://www.testshop.com/orders/ord_"+id || ev.Data.Status != "confirmed" {
		t.Fatalf("event = %+v", ev)
	}
	raw, _ := json.Marshal(ev.Data)
	loadSchema(t).valid(t, "Order", raw)
	if len(ev.Data.Totals) != 1 || ev.Data.Totals[0].Type != "total" || ev.Data.Totals[0].Amount != 300 {
		t.Fatalf("totals = %+v", ev.Data.Totals)
	}
}

func TestAWebhookThatFailsIsReportedAndTheOrderStands(t *testing.T) {
	r := newRig(t)
	r.ev.err = fmt.Errorf("receiver down")
	id := r.create()
	w := r.post("/checkout_sessions/"+id+"/complete", completeBody)
	if w.Code != 200 || w.get("status") != "completed" {
		t.Fatalf("complete = %d %s", w.Code, w.Body)
	}
	if len(r.rep.errs) != 1 || !strings.Contains(r.rep.errs[0].Error(), "order_create for order ord_"+id) {
		t.Fatalf("reports = %v", r.rep.errs)
	}
}

// ---- idempotency (rfc.agentic_checkout.md §6) ----

func TestEveryPostRequiresAnIdempotencyKey(t *testing.T) {
	r := newRig(t)
	id := r.create()
	for _, c := range []call{
		{method: "POST", path: "/checkout_sessions", body: createBody},
		{method: "POST", path: "/checkout_sessions/" + id, body: `{}`},
		{method: "POST", path: "/checkout_sessions/" + id + "/complete", body: completeBody},
		{method: "POST", path: "/checkout_sessions/" + id + "/cancel"},
	} {
		c.hdr = map[string]string{"Idempotency-Key": ""}
		e := wantErr(t, r.send(c), 400, "invalid_request", "idempotency_key_required", "")
		if e.Message != "Idempotency-Key header is required on all POST requests" {
			t.Fatalf("message = %q", e.Message)
		}
		if got := r.send(c).Header().Get("Idempotency-Key"); got != "" {
			t.Fatalf("an absent key was echoed as %q", got)
		}
	}
	// The spec's example of the error has the type and code of ours.
	var want Error
	_ = json.Unmarshal([]byte(spec(t, "error_400_idempotency_key_required")), &want)
	if want.Type != "invalid_request" || want.Code != "idempotency_key_required" {
		t.Fatalf("example = %+v", want)
	}
	// A key over 255 characters is refused; one of 255 is taken.
	wantErr(t, r.post("/checkout_sessions", createBody, map[string]string{"Idempotency-Key": strings.Repeat("k", 256)}), 400, "invalid_request", "invalid_idempotency_key", "")
	if w := r.post("/checkout_sessions", createBody, map[string]string{"Idempotency-Key": strings.Repeat("k", 255)}); w.Code != 201 {
		t.Fatalf("a 255-character key = %d %s", w.Code, w.Body)
	}
	// A GET needs none.
	if w := r.get("/checkout_sessions/" + id); w.Code != 200 {
		t.Fatalf("get = %d", w.Code)
	}
}

func TestTheSameKeyAndBodyReplaysTheOriginalResponse(t *testing.T) {
	r := newRig(t)
	k := map[string]string{"Idempotency-Key": "k-replay", "Request-Id": "r-1"}
	first := r.post("/checkout_sessions", createBody, k)
	second := r.post("/checkout_sessions", createBody, map[string]string{"Idempotency-Key": "k-replay", "Request-Id": "r-2"})
	if first.Code != 201 || second.Code != 201 {
		t.Fatalf("%d %d", first.Code, second.Code)
	}
	if !bytes.Equal(first.Body.Bytes(), second.Body.Bytes()) {
		t.Fatalf("replay differs:\n%s\n%s", first.Body, second.Body)
	}
	if second.Header().Get("Idempotent-Replayed") != "true" || first.Header().Get("Idempotent-Replayed") != "" {
		t.Fatalf("Idempotent-Replayed = %q, %q", first.Header().Get("Idempotent-Replayed"), second.Header().Get("Idempotent-Replayed"))
	}
	if second.Header().Get("Idempotency-Key") != "k-replay" || second.Header().Get("Request-Id") != "r-2" {
		t.Fatalf("replay headers = %v", second.Header())
	}
	if r.be.opens != 1 {
		t.Fatalf("opens = %d; a replay must not run again", r.be.opens)
	}
	// Update and complete replay the same way, and complete does not charge twice.
	id := first.get("id").(string)
	u1 := r.post("/checkout_sessions/"+id, `{"line_items":[{"id":"item_456"}]}`, map[string]string{"Idempotency-Key": "k-update"})
	u2 := r.post("/checkout_sessions/"+id, `{"line_items":[{"id":"item_456"}]}`, map[string]string{"Idempotency-Key": "k-update"})
	if u1.Code != 200 || !bytes.Equal(u1.Body.Bytes(), u2.Body.Bytes()) || u2.Header().Get("Idempotent-Replayed") != "true" {
		t.Fatalf("update replay: %d %s | %d %s", u1.Code, u1.Body, u2.Code, u2.Body)
	}
	c1 := r.post("/checkout_sessions/"+id+"/complete", completeBody, map[string]string{"Idempotency-Key": "k-pay"})
	c2 := r.post("/checkout_sessions/"+id+"/complete", completeBody, map[string]string{"Idempotency-Key": "k-pay"})
	if c1.Code != 200 || !bytes.Equal(c1.Body.Bytes(), c2.Body.Bytes()) || r.be.pays != 1 {
		t.Fatalf("complete replay: %d %d pays=%d", c1.Code, c2.Code, r.be.pays)
	}
}

func TestEquivalentBodiesReplayAndDifferentOnesConflict(t *testing.T) {
	r := newRig(t)
	k := map[string]string{"Idempotency-Key": "k-equal"}
	first := r.post("/checkout_sessions", `{"currency":"usd","line_items":[{"id":"item_123"}],"capabilities":{"interventions":{"supported":["3ds"],"max_redirects":1}}}`, k)
	if first.Code != 201 {
		t.Fatalf("%d %s", first.Code, first.Body)
	}
	for name, body := range map[string]string{
		"another key order":           `{"capabilities":{"interventions":{"max_redirects":1,"supported":["3ds"]}},"line_items":[{"id":"item_123"}],"currency":"usd"}`,
		"the same number written 1.0": `{"currency":"usd","line_items":[{"id":"item_123"}],"capabilities":{"interventions":{"supported":["3ds"],"max_redirects":1.0}}}`,
		"the same number as 10e-1":    `{"currency":"usd","line_items":[{"id":"item_123"}],"capabilities":{"interventions":{"supported":["3ds"],"max_redirects":10e-1}}}`,
		"other whitespace":            "{ \"currency\" : \"usd\",\n\"line_items\":[ {\"id\":\"item_123\"} ],\"capabilities\":{\"interventions\":{\"supported\":[\"3ds\"],\"max_redirects\":1}} }",
	} {
		w := r.post("/checkout_sessions", body, k)
		if w.Code != 201 || w.Header().Get("Idempotent-Replayed") != "true" || !bytes.Equal(w.Body.Bytes(), first.Body.Bytes()) {
			t.Errorf("%s: %d replayed=%q", name, w.Code, w.Header().Get("Idempotent-Replayed"))
		}
	}
	for name, body := range map[string]string{
		"another value":                   `{"currency":"usd","line_items":[{"id":"item_456"}],"capabilities":{"interventions":{"supported":["3ds"],"max_redirects":1}}}`,
		"an array in another order":       `{"currency":"usd","line_items":[{"id":"item_123"}],"capabilities":{"interventions":{"supported":["3ds","biometric"],"max_redirects":1}}}`,
		"a null where the key was absent": `{"currency":"usd","line_items":[{"id":"item_123"}],"capabilities":{"interventions":{"supported":["3ds"],"max_redirects":1}},"order_notes":null}`,
		"an added field":                  `{"currency":"usd","line_items":[{"id":"item_123"}],"capabilities":{"interventions":{"supported":["3ds"],"max_redirects":1}},"locale":"en-US"}`,
		"a different number":              `{"currency":"usd","line_items":[{"id":"item_123"}],"capabilities":{"interventions":{"supported":["3ds"],"max_redirects":2}}}`,
	} {
		e := wantErr(t, r.post("/checkout_sessions", body, k), 422, "invalid_request", "idempotency_conflict", "")
		if e.Message != "Idempotency-Key has already been used with a different request body" {
			t.Errorf("%s: message = %q", name, e.Message)
		}
	}
	// The spec's example of the error is ours.
	var want Error
	_ = json.Unmarshal([]byte(spec(t, "error_422_idempotency_conflict")), &want)
	got := wantErr(t, r.post("/checkout_sessions", `{"currency":"usd","line_items":[{"id":"item_456"}],"capabilities":{}}`, k), 422, "invalid_request", "idempotency_conflict", "")
	if !equal(got, want) {
		t.Fatalf("error = %+v; the spec's example is %+v", got, want)
	}
	if r.be.opens != 1 {
		t.Fatalf("opens = %d", r.be.opens)
	}
}

func TestAKeyIsScopedToTheCallerAndTheEndpoint(t *testing.T) {
	r := newRig(t)
	k := map[string]string{"Idempotency-Key": "k-shared"}
	a := r.post("/checkout_sessions", createBody, k)
	// Another caller using the same key is another request.
	b := r.post("/checkout_sessions", createBody, map[string]string{"Idempotency-Key": "k-shared", "Authorization": "Bearer other_key"})
	if a.Code != 201 || b.Code != 201 || a.get("id") == b.get("id") || b.Header().Get("Idempotent-Replayed") != "" {
		t.Fatalf("callers share a key: %s | %s", a.Body, b.Body)
	}
	// The same key on another endpoint is another request.
	id := a.get("id").(string)
	for _, c := range []call{
		{method: "POST", path: "/checkout_sessions/" + id, body: `{}`},
		{method: "POST", path: "/checkout_sessions/" + id + "/complete", body: completeBody},
	} {
		c.hdr = k
		if w := r.send(c); w.Code != 200 || w.Header().Get("Idempotent-Replayed") != "" {
			t.Fatalf("%s = %d replayed=%q %s", c.path, w.Code, w.Header().Get("Idempotent-Replayed"), w.Body)
		}
	}
	// And the same key on the same path with another session is another request.
	if r.be.opens != 2 {
		t.Fatalf("opens = %d", r.be.opens)
	}
}

func TestARequestThatIsStillRunningAnswers409WithRetryAfter(t *testing.T) {
	r := newRig(t)
	r.be.gate, r.be.entered = make(chan struct{}), make(chan struct{})
	k := map[string]string{"Idempotency-Key": "k-flight"}
	done := make(chan res)
	go func() { done <- r.post("/checkout_sessions", createBody, k) }()
	<-r.be.entered
	w := r.post("/checkout_sessions", createBody, k)
	e := wantErr(t, w, 409, "invalid_request", "idempotency_in_flight", "")
	if w.Header().Get("Retry-After") != "1" {
		t.Fatalf("Retry-After = %q", w.Header().Get("Retry-After"))
	}
	var want Error
	_ = json.Unmarshal([]byte(spec(t, "error_409_idempotency_in_flight")), &want)
	if !equal(e, want) {
		t.Fatalf("error = %+v; the spec's example is %+v", e, want)
	}
	// A different body while it runs is in flight too, not a conflict.
	wantErr(t, r.post("/checkout_sessions", `{"currency":"usd","line_items":[{"id":"item_456"}],"capabilities":{}}`, k), 409, "invalid_request", "idempotency_in_flight", "")
	close(r.be.gate)
	first := <-done
	if first.Code != 201 {
		t.Fatalf("first = %d %s", first.Code, first.Body)
	}
	if w = r.post("/checkout_sessions", createBody, k); w.Code != 201 || w.Header().Get("Idempotent-Replayed") != "true" {
		t.Fatalf("after it finished = %d %s", w.Code, w.Body)
	}
}

func TestAServerErrorIsNotCachedAgainstTheKey(t *testing.T) {
	r := newRig(t)
	k := map[string]string{"Idempotency-Key": "k-5xx"}
	r.be.openErr = Unavailable("billing is temporarily unavailable")
	w := r.post("/checkout_sessions", createBody, k)
	wantErr(t, w, 503, "service_unavailable", "service_unavailable", "")
	if w.Header().Get("Idempotency-Key") != "k-5xx" {
		t.Fatal("the key was not echoed on a 5xx")
	}
	// The retry with the same key is a fresh request.
	w = r.post("/checkout_sessions", createBody, k)
	if w.Code != 201 || w.Header().Get("Idempotent-Replayed") != "" {
		t.Fatalf("retry = %d %s", w.Code, w.Body)
	}
	// An unexpected backend error is a 500 processing_error that carries none of its text,
	// is reported, and is not cached either.
	r.be.openErr = fmt.Errorf("connection to 10.0.0.5:5432 refused")
	k = map[string]string{"Idempotency-Key": "k-500"}
	w = r.post("/checkout_sessions", createBody, k)
	e := wantErr(t, w, 500, "processing_error", "internal_error", "")
	if strings.Contains(w.Body.String(), "10.0.0.5") || strings.Contains(e.Message, "refused") {
		t.Fatalf("the cause leaked: %s", w.Body)
	}
	if len(r.rep.errs) != 1 || !strings.Contains(r.rep.errs[0].Error(), "10.0.0.5") {
		t.Fatalf("reports = %v", r.rep.errs)
	}
	if w = r.post("/checkout_sessions", createBody, k); w.Code != 201 {
		t.Fatalf("retry after a 500 = %d %s", w.Code, w.Body)
	}
}

func TestAClientErrorIsCachedAgainstTheKey(t *testing.T) {
	r := newRig(t)
	k := map[string]string{"Idempotency-Key": "k-4xx"}
	body := strings.Replace(createBody, "item_123", "invalid_item", 1)
	wantErr(t, r.post("/checkout_sessions", body, k), 400, "invalid_request", "invalid_item_id", "$.line_items[0].item.id")
	w := r.post("/checkout_sessions", body, k)
	wantErr(t, w, 400, "invalid_request", "invalid_item_id", "$.line_items[0].item.id")
	if w.Header().Get("Idempotent-Replayed") != "true" || r.be.opens != 1 {
		t.Fatalf("replayed=%q opens=%d", w.Header().Get("Idempotent-Replayed"), r.be.opens)
	}
}

func TestAKeyIsForgottenAfterRetention(t *testing.T) {
	mem := NewMemory()
	clock := time.Date(2026, 4, 17, 10, 0, 0, 0, time.UTC)
	mem.now = func() time.Time { return clock }
	r := newRig(t, func(c *Config) { c.Ledger = mem })
	k := map[string]string{"Idempotency-Key": "k-old"}
	first := r.post("/checkout_sessions", createBody, k)
	clock = clock.Add(Retention - time.Second)
	if w := r.post("/checkout_sessions", createBody, k); w.Header().Get("Idempotent-Replayed") != "true" || !bytes.Equal(w.Body.Bytes(), first.Body.Bytes()) {
		t.Fatalf("a key within 24 hours was forgotten: %d", w.Code)
	}
	clock = clock.Add(2 * time.Second)
	// After 24 hours a reused key is a new request: no error, even with another body.
	w := r.post("/checkout_sessions", `{"currency":"usd","line_items":[{"id":"item_456"}],"capabilities":{}}`, k)
	if w.Code != 201 || w.Header().Get("Idempotent-Replayed") != "" {
		t.Fatalf("after retention = %d %s", w.Code, w.Body)
	}
}

func TestConcurrentRequestsWithOneKeyRunOnce(t *testing.T) {
	r := newRig(t)
	k := map[string]string{"Idempotency-Key": "k-race"}
	var wg sync.WaitGroup
	codes := make([]int, 20)
	for i := range codes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes[i] = r.post("/checkout_sessions", createBody, k).Code
		}()
	}
	wg.Wait()
	created := 0
	for _, c := range codes {
		switch c {
		case 201:
			created++
		case 409:
		default:
			t.Fatalf("code %d", c)
		}
	}
	if r.be.opens != 1 || created < 1 {
		t.Fatalf("opens = %d created = %d", r.be.opens, created)
	}
}

// ---- request signatures ----

type verifier struct {
	want string
	seen []byte
}

func (v *verifier) Verify(_ context.Context, _ Caller, sig string, body []byte) error {
	v.seen = body
	if sig != v.want {
		return fmt.Errorf("bad signature")
	}
	return nil
}

func TestSignedRequestsAreVerifiedWhenAVerifierIsConfigured(t *testing.T) {
	v := &verifier{want: "c2ln"}
	r := newRig(t, func(c *Config) { c.Verify = v })
	ts := r.now.Format(time.RFC3339)
	ok := map[string]string{"Signature": "c2ln", "Timestamp": ts}
	if w := r.post("/checkout_sessions", createBody, ok); w.Code != 201 {
		t.Fatalf("a signed request = %d %s", w.Code, w.Body)
	}
	if string(v.seen) != createBody {
		t.Fatalf("the verifier saw %q", v.seen)
	}
	id := "checkout_session_1"
	if w := r.get("/checkout_sessions/"+id, ok); w.Code != 200 {
		t.Fatalf("a signed GET = %d %s", w.Code, w.Body)
	}
	for name, h := range map[string]map[string]string{
		"no signature":            {"Timestamp": ts},
		"no timestamp":            {"Signature": "c2ln"},
		"a bad signature":         {"Signature": "xxxx", "Timestamp": ts},
		"a timestamp not RFC3339": {"Signature": "c2ln", "Timestamp": "yesterday"},
		"a stale timestamp":       {"Signature": "c2ln", "Timestamp": r.now.Add(-301 * time.Second).Format(time.RFC3339)},
		"a future timestamp":      {"Signature": "c2ln", "Timestamp": r.now.Add(301 * time.Second).Format(time.RFC3339)},
	} {
		h["Idempotency-Key"] = "k-" + name
		wantErr(t, r.post("/checkout_sessions", createBody, h), 401, "invalid_request", "invalid_signature", "")
	}
	// The edge of the window is inside it.
	edge := map[string]string{"Signature": "c2ln", "Timestamp": r.now.Add(-300 * time.Second).Format(time.RFC3339), "Idempotency-Key": "k-edge"}
	if w := r.post("/checkout_sessions", createBody, edge); w.Code != 201 {
		t.Fatalf("a timestamp at the edge = %d %s", w.Code, w.Body)
	}
	if r.be.opens != 2 {
		t.Fatalf("opens = %d; an unverified request reached the backend", r.be.opens)
	}
}

func TestRequestsAreNotVerifiedWithoutAVerifier(t *testing.T) {
	r := newRig(t)
	if w := r.post("/checkout_sessions", createBody, map[string]string{"Signature": "garbage", "Timestamp": "garbage"}); w.Code != 201 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
}

func TestNewRefusesAConfigThatLacksWhatASessionNeeds(t *testing.T) {
	base := newRig(t).cfg
	for name, mut := range map[string]func(*Config){
		"backend":  func(c *Config) { c.Backend = nil },
		"identify": func(c *Config) { c.Identify = nil },
		"handlers": func(c *Config) { c.Handlers = nil },
		"order":    func(c *Config) { c.Order = nil },
	} {
		c := base
		mut(&c)
		if _, err := New(c); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

func TestAFailedHandlerLookupIs503(t *testing.T) {
	r := newRig(t, func(c *Config) {
		c.Handlers = func(context.Context, Caller) ([]PaymentHandler, error) { return nil, fmt.Errorf("kms down") }
	})
	wantErr(t, r.post("/checkout_sessions", createBody), 503, "service_unavailable", "service_unavailable", "")
}
