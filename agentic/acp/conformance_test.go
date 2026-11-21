// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package acp

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// The spec's own examples (testdata, see testdata/SOURCE) are the conformance corpus: each is
// judged by the spec's JSON Schema, and each decodes into the wire types with unknown fields
// refused and encodes back to the same JSON.

type example struct {
	name string // file and key
	def  string // the schema definition the example is an instance of
	typ  func() any
	raw  []byte
}

// defOf names the definition and Go type of an example of examples.agentic_checkout.json by
// the key it has there.
func defOf(key string) (string, func() any) {
	switch {
	case strings.HasPrefix(key, "authentication_result_"):
		return "AuthenticationResult", func() any { return new(AuthenticationResult) }
	case strings.HasPrefix(key, "cancel_checkout_session_request"):
		return "CancelSessionRequest", func() any { return new(CancelRequest) }
	case strings.HasPrefix(key, "complete_") && strings.HasSuffix(key, "request"), strings.HasPrefix(key, "complete_checkout_session_request"):
		return "CheckoutSessionCompleteRequest", func() any { return new(CompleteRequest) }
	case key == "complete_checkout_session_response":
		return "CheckoutSessionWithOrder", func() any { return new(CheckoutSession) }
	case strings.HasPrefix(key, "create_") && strings.Contains(key, "request"):
		return "CheckoutSessionCreateRequest", func() any { return new(CreateRequest) }
	case key == "update_checkout_session_request":
		return "CheckoutSessionUpdateRequest", func() any { return new(UpdateRequest) }
	case strings.HasPrefix(key, "discovery_response"):
		return "DiscoveryResponse", func() any { return new(Discovery) }
	case strings.HasPrefix(key, "error_"):
		return "Error", func() any { return new(Error) }
	case strings.HasPrefix(key, "webhook_"):
		return "", func() any { return new(Event) }
	}
	return "CheckoutSession", func() any { return new(CheckoutSession) }
}

// knownInvalid are examples of the 2026-04-17 release that the release's own schema rejects,
// with the reason. The test requires them to stay invalid: when the spec corrects one, the
// entry is removed and the example joins the corpus.
var knownInvalid = map[string]string{
	"examples.agentic_checkout.json#webhook_order_create_event":             "carries refunds[], which the webhook schema removed, and no order id",
	"examples.agentic_checkout.json#webhook_order_update_event":             "carries refunds[], which the webhook schema removed, and no order id",
	"examples.agentic_checkout.json#authentication_result_denied_example":   "outcome_details lacks the cryptogram and indicator the schema requires of every outcome_details",
	"examples.multi_item_checkout.json#create_multi_item_checkout_request":  "lacks the capabilities the schema requires of a create request",
	"examples.multi_item_checkout.json#create_multi_item_checkout_response": "lacks capabilities, nests capabilities in protocol and uses the total type base_amount, which the schema does not define",
	"discount-extension/automatic-discount.json":                            "request items carry quantity and the response carries capabilities.payment_methods, neither in the released schema",
	"discount-extension/order-level-discount.json":                          "request items carry quantity and the response carries capabilities.payment_methods, neither in the released schema",
	"discount-extension/percentage-discount-with-allocations.json":          "request items carry quantity and the response carries capabilities.payment_methods, neither in the released schema",
	"discount-extension/rejected-discount-code.json":                        "request items carry quantity and the response carries capabilities.payment_methods, neither in the released schema",
	"discount-extension/stacked-discounts.json":                             "request items carry quantity and the response carries capabilities.payment_methods, neither in the released schema",
}

// decodable are known-invalid examples that the wire types still carry.
var decodable = map[string]bool{
	"examples.agentic_checkout.json#authentication_result_denied_example":  true,
	"examples.multi_item_checkout.json#create_multi_item_checkout_request": true,
}

func corpus(t testing.TB) []example {
	t.Helper()
	var out []example
	for _, f := range []string{"examples.agentic_checkout.json", "examples.multi_item_checkout.json"} {
		b, err := os.ReadFile(filepath.Join("testdata", f))
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]json.RawMessage
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatal(err)
		}
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			def, typ := defOf(k)
			out = append(out, example{name: f + "#" + k, def: def, typ: typ, raw: m[k]})
		}
	}
	orders, _ := filepath.Glob("testdata/orders/*.json")
	for _, f := range orders {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]json.RawMessage
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatal(err)
		}
		delete(m, "$schema")
		delete(m, "_description")
		raw, _ := json.Marshal(m)
		out = append(out, example{name: "orders/" + filepath.Base(f), def: "Order", typ: func() any { return new(Order) }, raw: raw})
	}
	return out
}

func TestExamplesAreValidAgainstTheSchema(t *testing.T) {
	sc := loadSchema(t)
	n := 0
	for _, ex := range corpus(t) {
		if ex.def == "" {
			continue
		}
		var doc any
		if err := json.Unmarshal(ex.raw, &doc); err != nil {
			t.Fatal(err)
		}
		errs := sc.check(ex.def, doc)
		_, known := knownInvalid[ex.name]
		switch {
		case known && len(errs) == 0:
			t.Errorf("%s is valid now: remove it from knownInvalid", ex.name)
		case !known && len(errs) > 0:
			t.Errorf("%s (%s): %s", ex.name, ex.def, strings.Join(errs, "; "))
		}
		n++
	}
	if n != 35 {
		t.Fatalf("%d examples were judged; the corpus has 35", n)
	}
}

func TestKnownInvalidDiscountExamplesAreStillInvalid(t *testing.T) {
	sc := loadSchema(t)
	files, _ := filepath.Glob("testdata/discount-extension/*.json")
	if len(files) != 5 {
		t.Fatalf("%d discount examples; want 5", len(files))
	}
	for _, f := range files {
		b, _ := os.ReadFile(f)
		var m struct {
			Request  json.RawMessage `json:"request"`
			Response json.RawMessage `json:"response"`
		}
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatal(err)
		}
		if _, ok := knownInvalid["discount-extension/"+filepath.Base(f)]; !ok {
			t.Errorf("%s has no knownInvalid entry", f)
		}
		var req, res any
		_ = json.Unmarshal(m.Request, &req)
		_ = json.Unmarshal(m.Response, &res)
		if len(sc.check("CheckoutSessionCreateRequest", req))+len(sc.check("CheckoutSession", res)) == 0 {
			t.Errorf("%s is valid now: remove it from knownInvalid", f)
		}
	}
}

func TestExamplesRoundTripThroughTheWireTypes(t *testing.T) {
	n := 0
	for _, ex := range corpus(t) {
		if _, known := knownInvalid[ex.name]; known && !decodable[ex.name] {
			continue
		}
		v := ex.typ()
		dec := json.NewDecoder(bytes.NewReader(ex.raw))
		dec.DisallowUnknownFields()
		if err := dec.Decode(v); err != nil {
			t.Errorf("%s: decode: %v", ex.name, err)
			continue
		}
		back, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		var want, got any
		_ = json.Unmarshal(ex.raw, &want)
		_ = json.Unmarshal(back, &got)
		if !equal(want, got) {
			t.Errorf("%s: round trip differs\nwant %s\n got %s", ex.name, ex.raw, back)
		}
		n++
	}
	if n != 34 {
		t.Fatalf("%d examples round-tripped; the corpus has 34", n)
	}
}

func TestWebhookExamplesAreValidAndRoundTrip(t *testing.T) {
	sc := loadSchema(t)
	b, err := os.ReadFile("testdata/webhook_events.json")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if len(m) != 3 {
		t.Fatalf("%d webhook examples; want 3", len(m))
	}
	for name, raw := range m {
		var ev struct {
			Type string          `json:"type"`
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(raw, &ev); err != nil {
			t.Fatal(err)
		}
		// EventDataOrder is Order with type, checkout_session_id, permalink_url and status required.
		sc.valid(t, "Order", ev.Data)
		var data map[string]any
		_ = json.Unmarshal(ev.Data, &data)
		for _, k := range []string{"type", "checkout_session_id", "permalink_url", "status"} {
			if _, ok := data[k]; !ok {
				t.Errorf("%s: data lacks %s", name, k)
			}
		}
		var e Event
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&e); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		back, _ := json.Marshal(e)
		var want, got any
		_ = json.Unmarshal(raw, &want)
		_ = json.Unmarshal(back, &got)
		if !equal(want, got) {
			t.Errorf("%s: round trip differs\nwant %s\n got %s", name, raw, back)
		}
	}
}
