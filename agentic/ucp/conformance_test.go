// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ucp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// The examples in testdata/examples are the JSON blocks of the specification
// that its own CI validates against the schemas (see testdata/SOURCE). Each is
// decoded into this package's wire types and encoded again; the result must
// equal the example. A member the types do not declare would be dropped by the
// decode and fail the comparison, and the members of open objects that land in
// Extra are listed explicitly below, never ignored.

type example struct {
	File      string `json:"file"`
	Source    string `json:"source"`
	Schema    string `json:"schema"`
	Op        string `json:"op"`
	Direction string `json:"direction"`
	Def       string `json:"def"`
	Target    string `json:"target"`
	HTTP      string `json:"http"`
}

func examples(t *testing.T) []example {
	t.Helper()
	b, err := os.ReadFile("testdata/examples/index.json")
	if err != nil {
		t.Fatal(err)
	}
	var out []example
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func (e example) body(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata/examples", e.File))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// extension members the examples carry that this package keeps opaque: the
// Fulfillment extension composes `fulfillment` onto checkout.
var allowedExtras = map[string][]string{
	"shopping-checkout-rest-380.json":  {"$.fulfillment"},
	"shopping-checkout-rest-423.json":  {"$.fulfillment"},
	"shopping-checkout-rest-583.json":  {"$.fulfillment"},
	"shopping-checkout-rest-635.json":  {"$.fulfillment"},
	"shopping-checkout-rest-829.json":  {"$.fulfillment"},
	"shopping-checkout-rest-993.json":  {"$.fulfillment"},
	"shopping-checkout-rest-1151.json": {"$.fulfillment"},
	// The AP2 Mandates extension composes `ap2` onto complete requests.
	"overview-2062.json": {"$.ap2"},
}

// collect lists the Extra members of every open object in v.
func collect(v reflect.Value, path string, out *[]string) {
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		if !v.IsNil() {
			collect(v.Elem(), path, out)
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			f := v.Type().Field(i)
			if f.Name == "Extra" {
				for _, k := range sortedKeys(v.Field(i).Interface().(map[string]json.RawMessage)) {
					*out = append(*out, path+"."+k)
				}
				continue
			}
			name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
			if f.Anonymous && name == "" {
				collect(v.Field(i), path, out)
				continue
			}
			if name == "" || name == "-" {
				continue
			}
			collect(v.Field(i), path+"."+name, out)
		}
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			collect(v.Index(i), path+"[]", out)
		}
	case reflect.Map:
		for _, k := range v.MapKeys() {
			collect(v.MapIndex(k), path+"."+k.String(), out)
		}
	}
}

func normalize(t *testing.T, b []byte) any {
	t.Helper()
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

// roundTrip decodes body into v, encodes it, and requires the same JSON.
func roundTrip(t *testing.T, e example, v any) {
	t.Helper()
	body := e.body(t)
	if err := json.Unmarshal(body, v); err != nil {
		t.Fatalf("%s: decode: %v", e.Source, err)
	}
	out, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("%s: encode: %v", e.Source, err)
	}
	if !reflect.DeepEqual(normalize(t, body), normalize(t, out)) {
		t.Fatalf("%s: round trip changed the example\n got %s\nwant %s", e.Source, out, body)
	}
	var extra []string
	collect(reflect.ValueOf(v), "$", &extra)
	sort.Strings(extra)
	want := append([]string(nil), allowedExtras[e.File]...)
	sort.Strings(want)
	if !reflect.DeepEqual(extra, want) && !(len(extra) == 0 && len(want) == 0) {
		t.Fatalf("%s: members kept opaque = %v; want %v", e.Source, extra, want)
	}
}

// fragments decode the examples that are a part of a document, named by target.
func fragment(t *testing.T, e example) {
	t.Helper()
	switch e.Target {
	case "$.totals":
		var v []Total
		roundTrip(t, e, &v)
		if err := CheckTotals(v); err != nil {
			t.Fatalf("%s: %v", e.Source, err)
		}
	case "$.messages":
		var v []Message
		roundTrip(t, e, &v)
		for _, m := range v {
			if err := m.Check(); err != nil {
				t.Fatalf("%s: %v", e.Source, err)
			}
		}
	case "$.ucp":
		var v Meta
		roundTrip(t, e, &v)
	case "$.attribution":
		var v map[string]string
		roundTrip(t, e, &v)
	case "$.keys[0]":
		var v Key
		roundTrip(t, e, &v)
		if err := v.check(); err != nil {
			t.Fatalf("%s: %v", e.Source, err)
		}
	case "$.ucp.capabilities":
		var v map[string][]Capability
		roundTrip(t, e, &v)
	case "$.policies":
		// Policies are not produced by this package; the example is kept opaque.
		var v []json.RawMessage
		roundTrip(t, e, &v)
	default:
		t.Fatalf("%s: unhandled target %s", e.Source, e.Target)
	}
}

func TestSpecificationExamplesRoundTrip(t *testing.T) {
	exs := examples(t)
	if len(exs) < 60 {
		t.Fatalf("only %d examples; testdata is incomplete", len(exs))
	}
	for _, e := range exs {
		e := e
		t.Run(e.File, func(t *testing.T) {
			if e.Target != "" {
				fragment(t, e)
				return
			}
			switch {
			case e.Schema == "profile" && e.Def == "business_schema":
				var p Profile
				roundTrip(t, e, &p)
				if err := p.Validate(Business); err != nil {
					t.Fatalf("%s: %v", e.Source, err)
				}
			case e.Schema == "profile" && e.Def == "platform_schema":
				var p Profile
				roundTrip(t, e, &p)
				if err := p.Validate(Platform); err != nil {
					t.Fatalf("%s: %v", e.Source, err)
				}
			case e.Schema == "payment_handler":
				var h Handler
				roundTrip(t, e, &h)
			case e.Schema == "common/types/error_response":
				var r ErrorResponse
				roundTrip(t, e, &r)
				if r.UCP.Status != "error" || len(r.Messages) == 0 {
					t.Fatalf("%s: not an error response", e.Source)
				}
				for _, m := range r.Messages {
					if err := m.Check(); err != nil {
						t.Fatalf("%s: %v", e.Source, err)
					}
				}
			case e.Schema == "shopping/checkout" && e.Direction == "request":
				switch e.Op {
				case "create":
					var r CreateRequest
					roundTrip(t, e, &r)
				case "update":
					var r UpdateRequest
					roundTrip(t, e, &r)
				case "complete":
					var r CompleteRequest
					roundTrip(t, e, &r)
				default: // read and cancel have no body
					var r map[string]json.RawMessage
					roundTrip(t, e, &r)
					if len(r) != 0 {
						t.Fatalf("%s: a %s request has no body", e.Source, e.Op)
					}
				}
			case e.Schema == "shopping/checkout":
				var c Checkout
				roundTrip(t, e, &c)
				if err := c.Check(); err != nil {
					t.Fatalf("%s: %v", e.Source, err)
				}
			case e.Schema == "shopping/order":
				var o Order
				roundTrip(t, e, &o)
			default:
				t.Fatalf("%s: unhandled schema %s", e.Source, e.Schema)
			}
		})
	}
}
