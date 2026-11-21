// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ucp

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// The table of the overview, "Derivation algorithm".
func TestAuthorityBindingTable(t *testing.T) {
	for _, tc := range []struct {
		name, schema string
		ok           bool
	}{
		{"dev.ucp.shopping.checkout", "https://ucp.dev/schemas/shopping/checkout.json", true},
		{"dev.ucp.shopping.checkout", "https://shopping.ucp.dev/x.json", true},
		{"com.example.payments.installments", "https://example.com/x.json", true},
		{"com.example.pay", "https://pay.example.com/x.json", true},
		{"com.example.pay", "https://example.com/x.json", true},
		{"com.example.pay", "https://evil.example/x.json", false},
		{"dev.ucp.shopping.checkout", "https://evil.example/x.json", false},
		{"com.examplecorp.pay", "https://example.com/x.json", false},
		{"com.example.pay", "https://cdn.example.com/x.json", false},
		// substring matching on the raw URL is not permitted
		{"dev.ucp.shopping.checkout", "https://ucp.dev@evil.example/x.json", false},
		{"dev.ucp.shopping.checkout", "http://ucp.dev/x.json", false},
		{"dev.ucp.shopping.checkout", "https://user:pw@ucp.dev/x.json", false},
		{"dev.ucp.shopping.checkout", "https://203.0.113.10/x.json", false},
		{"dev.ucp.shopping.checkout", "https://[::1]/x.json", false},
		{"dev.ucp.shopping.checkout", "https://localhost/x.json", false},
		{"dev.ucp.shopping.checkout", "https://UCP.DEV./x.json", true},
		{"dev.ucp.shopping.checkout", "https://ucp.dev:8443/x.json", true},
		{"dev.ucp.shopping.checkout", "https://ucp.dev.evil.example/x.json", false},
		{"dev.ucp", "https://ucp.dev/x.json", true},
		{"dev.ucpx.shopping", "https://ucp.dev/x.json", false},
		{"dev.ucp.shopping.checkout", "https://ünïcode.dev/x.json", false},
		{"dev.ucp.shopping.checkout", "not a url", false},
		{"dev.ucp.shopping.checkout", "", false},
	} {
		if err := Bind(tc.name, tc.schema); (err == nil) != tc.ok {
			t.Errorf("Bind(%q, %q) = %v; want ok=%v", tc.name, tc.schema, err, tc.ok)
		}
	}
}

func goodBusiness(t *testing.T) *Profile {
	t.Helper()
	p, err := NewProfile(ProfileConfig{Endpoint: "https://merchant.example/ucp/v1", Order: true,
		Handlers: shopPay(), Keys: nil})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func clone(t *testing.T, p *Profile) *Profile {
	t.Helper()
	b, _ := json.Marshal(p)
	var out Profile
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return &out
}

func TestNewProfileIsTheSpecifiedBusinessProfile(t *testing.T) {
	p := goodBusiness(t)
	svc := p.UCP.Services[ServiceShopping]
	if len(svc) != 1 || svc[0].Transport != "rest" || svc[0].Version != Version || svc[0].Endpoint != "https://merchant.example/ucp/v1" ||
		svc[0].Schema != "https://ucp.dev/"+Version+"/services/shopping/rest.openapi.json" {
		t.Fatalf("services = %+v", svc)
	}
	co := p.UCP.Capabilities[CapCheckout][0]
	if co.Schema != "https://ucp.dev/"+Version+"/schemas/shopping/checkout.json" || co.Spec != "https://ucp.dev/"+Version+"/specification/shopping/checkout" || co.Version != Version {
		t.Fatalf("checkout = %+v", co)
	}
	if _, ok := p.UCP.Capabilities[CapOrder]; !ok {
		t.Fatal("order not advertised")
	}
	if err := p.Validate(Business); err != nil {
		t.Fatal(err)
	}
	// A profile with no handlers states payment_handlers as an empty registry.
	bare, err := NewProfile(ProfileConfig{Endpoint: "https://merchant.example/ucp/v1"})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(bare)
	if !strings.Contains(string(b), `"payment_handlers":{}`) || !strings.Contains(string(b), `"services":{`) {
		t.Fatalf("services and payment_handlers must be present even when empty: %s", b)
	}
	if _, err := NewProfile(ProfileConfig{Endpoint: "http://merchant.example/ucp/v1"}); err == nil {
		t.Fatal("an http endpoint was accepted")
	}
	if _, err := NewProfile(ProfileConfig{Endpoint: "https://merchant.example/ucp/v1/"}); err == nil {
		t.Fatal("a trailing slash was accepted")
	}
}

func TestMetaDistinguishesAbsentFromEmpty(t *testing.T) {
	var m Meta
	if err := json.Unmarshal([]byte(`{"version":"2026-08-25","services":{},"payment_handlers":{}}`), &m); err != nil {
		t.Fatal(err)
	}
	if m.Services == nil || m.PaymentHandlers == nil || m.Capabilities != nil {
		t.Fatalf("meta = %+v", m)
	}
	b, _ := json.Marshal(m)
	if string(b) != `{"version":"2026-08-25","services":{},"payment_handlers":{}}` {
		t.Fatalf("encoded = %s", b)
	}
	// Unrecognised members of the open ucp object are ignored.
	var o Meta
	if err := json.Unmarshal([]byte(`{"version":"2026-08-25","from_the_future":{"x":1},"ucp":{"ucp":1}}`), &o); err != nil || o.Version != "2026-08-25" {
		t.Fatalf("open ucp member: %v %+v", err, o)
	}
}

func TestValidateRejectsWhatTheSchemaForbids(t *testing.T) {
	for name, tc := range map[string]struct {
		kind   Kind
		mutate func(*Profile)
		want   string
	}{
		"missing services": {Business, func(p *Profile) { p.UCP.Services = nil }, "ucp.services"},
		"missing handlers": {Business, func(p *Profile) { p.UCP.PaymentHandlers = nil }, "ucp.payment_handlers"},
		"bad version":      {Business, func(p *Profile) { p.UCP.Version = "draft" }, "ucp.version"},
		"impossible date":  {Business, func(p *Profile) { p.UCP.Version = "2026-13-45" }, "ucp.version"},
		"registry key":     {Business, func(p *Profile) { p.UCP.Capabilities["Checkout"] = p.UCP.Capabilities[CapCheckout] }, "not a reverse-domain name"},
		"service version": {Business, func(p *Profile) {
			s := p.UCP.Services[ServiceShopping]
			s[0].Version = "2026-01-11"
		}, "dev.ucp.* entry must declare"},
		"capability version": {Business, func(p *Profile) {
			c := p.UCP.Capabilities[CapCheckout]
			c[0].Version = "2026-01-11"
		}, "dev.ucp.* entry must declare"},
		"capability schema": {Business, func(p *Profile) {
			c := p.UCP.Capabilities[CapCheckout]
			c[0].Schema = "https://evil.example/checkout.json"
		}, "does not bind"},
		"capability without schema": {Business, func(p *Profile) {
			c := p.UCP.Capabilities[CapCheckout]
			c[0].Schema = ""
		}, "schema"},
		"service without endpoint": {Business, func(p *Profile) {
			s := p.UCP.Services[ServiceShopping]
			s[0].Endpoint = ""
		}, "endpoint"},
		"service transport": {Business, func(p *Profile) {
			s := p.UCP.Services[ServiceShopping]
			s[0].Transport = "carrier-pigeon"
		}, "transport"},
		"endpoint with slash": {Business, func(p *Profile) {
			s := p.UCP.Services[ServiceShopping]
			s[0].Endpoint = "https://merchant.example/ucp/"
		}, "slash"},
		"endpoint http": {Business, func(p *Profile) {
			s := p.UCP.Services[ServiceShopping]
			s[0].Endpoint = "http://merchant.example/ucp"
		}, "https"},
		"spec not https": {Business, func(p *Profile) {
			c := p.UCP.Capabilities[CapCheckout]
			c[0].Spec = "http://ucp.dev/spec"
		}, "spec"},
		"extends not a name": {Business, func(p *Profile) {
			p.UCP.Capabilities["com.example.ext"] = []Capability{{Entity: Entity{Version: Version, Schema: "https://example.com/e.json"}, Extends: Extends("Not A Name")}}
		}, "extends"},
		"extends empty array": {Business, func(p *Profile) {
			p.UCP.Capabilities["com.example.ext"] = []Capability{{Entity: Entity{Version: Version, Schema: "https://example.com/e.json"}, Extends: json.RawMessage(`[]`)}}
		}, "extends"},
		"handler without id": {Business, func(p *Profile) {
			h := p.UCP.PaymentHandlers["com.shopify.shop_pay"]
			h[0].ID = ""
		}, "id"},
		"handler schema binding": {Business, func(p *Profile) {
			h := p.UCP.PaymentHandlers["com.shopify.shop_pay"]
			h[0].Schema = "https://evil.example/h.json"
		}, "does not bind"},
		"empty available_instruments": {Business, func(p *Profile) {
			h := p.UCP.PaymentHandlers["com.shopify.shop_pay"]
			h[0].AvailableInstruments = []AvailableInstrument{}
		}, "available_instruments"},
		"private key": {Business, func(p *Profile) {
			p.Keys = []Key{{KID: "k", KTY: "OKP", CRV: "Ed25519", X: rfcX, Extra: map[string]json.RawMessage{"d": json.RawMessage(`"secret"`)}}}
		}, "private member"},
		"duplicate kid": {Business, func(p *Profile) {
			k := Key{KID: "k", KTY: "OKP", CRV: "Ed25519", X: rfcX}
			p.Keys = []Key{k, k}
		}, "appears twice"},
		"alg does not match crv": {Business, func(p *Profile) {
			p.Keys = []Key{{KID: "k", KTY: "OKP", CRV: "Ed25519", X: rfcX, Alg: "ES256"}}
		}, "does not match"},
		"EC key without y": {Business, func(p *Profile) { p.Keys = []Key{{KID: "k", KTY: "EC", CRV: "P-256", X: "x"}} }, "needs crv, x and y"},
		"supported_versions not a date": {Business, func(p *Profile) {
			p.UCP.SupportedVersions = map[string]string{"old": "https://merchant.example/old"}
		}, "supported_versions"},
		"supported_versions lists itself": {Business, func(p *Profile) {
			p.UCP.SupportedVersions = map[string]string{Version: "https://merchant.example/old"}
		}, "own version"},
		"supported_versions not https": {Business, func(p *Profile) {
			p.UCP.SupportedVersions = map[string]string{"2026-01-11": "http://merchant.example/old"}
		}, "https"},
		"platform without spec": {Platform, func(p *Profile) {
			c := p.UCP.Capabilities[CapCheckout]
			c[0].Spec = ""
		}, "spec"},
		"platform handler without schema": {Platform, func(p *Profile) {
			p.UCP.PaymentHandlers["com.example.pay"] = []Handler{{Entity: Entity{ID: "p", Version: Version, Spec: "https://example.com/s"}}}
		}, "schema"},
	} {
		t.Run(name, func(t *testing.T) {
			var p *Profile
			if tc.kind == Business {
				p = clone(t, goodBusiness(t))
			} else {
				p = platformWithSpec(t)
			}
			tc.mutate(p)
			err := p.Validate(tc.kind)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v; want it to mention %q", err, tc.want)
			}
		})
	}
}

func platformWithSpec(t *testing.T) *Profile {
	t.Helper()
	p := platform(Version, CapCheckout)
	p.UCP.Services = map[string][]Service{ServiceShopping: {{Entity: Entity{Version: Version, Spec: "https://ucp.dev/s", Schema: "https://ucp.dev/s.json"}, Transport: "rest", Endpoint: "https://platform.example/ucp"}}}
	if err := p.Validate(Platform); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestAdmitRemovesEntitiesTheSpecificationRejects(t *testing.T) {
	p := clone(t, goodBusiness(t))
	p.UCP.Capabilities["com.example.loyalty"] = []Capability{{Entity: Entity{Version: "2026-06-15", Schema: "https://evil.example/loyalty.json"}}}
	p.UCP.Capabilities["com.example.ok"] = []Capability{{Entity: Entity{Version: "2026-06-15", Schema: "https://example.com/ok.json"}}}
	p.UCP.Capabilities[CapOrder] = []Capability{{Entity: Entity{Version: "2026-01-11", Schema: "https://ucp.dev/order.json"}}}
	p.UCP.PaymentHandlers["com.evil.pay"] = []Handler{{Entity: Entity{ID: "x", Version: Version, Schema: "https://other.example/h.json"}}}
	dropped := p.Admit()
	if len(dropped) != 3 {
		t.Fatalf("dropped = %v", dropped)
	}
	if _, ok := p.UCP.Capabilities["com.example.loyalty"]; ok {
		t.Error("a capability whose schema does not bind was kept")
	}
	if _, ok := p.UCP.Capabilities[CapOrder]; ok {
		t.Error("a dev.ucp.* capability at another version was kept")
	}
	if _, ok := p.UCP.PaymentHandlers["com.evil.pay"]; ok {
		t.Error("a handler whose schema does not bind was kept")
	}
	if _, ok := p.UCP.Capabilities["com.example.ok"]; !ok {
		t.Error("a valid third-party capability at its own version was dropped")
	}
	if _, ok := p.UCP.Capabilities[CapCheckout]; !ok {
		t.Error("checkout was dropped")
	}
}

func TestSupportedVersions(t *testing.T) {
	m := Meta{Version: "2026-08-25", SupportedVersions: map[string]string{"2026-04-08": "https://m.example/old", "2026-01-23": "https://m.example/older"}}
	if !m.Supports("2026-08-25") || !m.Supports("2026-04-08") || m.Supports("2026-01-12") {
		t.Fatal("Supports")
	}
}
