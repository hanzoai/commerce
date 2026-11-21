// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package acp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func discover(t *testing.T, base, docs string, locales []string) http.Handler {
	t.Helper()
	h, err := newRig(t).svc.Discover(base, docs, locales)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestDiscoveryDocument(t *testing.T) {
	h := discover(t, "https://api.shop.example/acp", "https://docs.shop.example/acp", []string{"en-US"})
	req := httptest.NewRequest(http.MethodGet, "/.well-known/acp.json", nil) // no Authorization, no API-Version
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 200 || w.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("%d %v", w.Code, w.Header())
	}
	if cc := w.Header().Get("Cache-Control"); cc != "public, max-age=3600" {
		t.Fatalf("Cache-Control = %q", cc)
	}
	loadSchema(t).valid(t, "DiscoveryResponse", w.Body.Bytes())
	var d Discovery
	if err := json.Unmarshal(w.Body.Bytes(), &d); err != nil {
		t.Fatal(err)
	}
	if d.Protocol.Name != "acp" || d.Protocol.Version != "2026-04-17" || len(d.Protocol.SupportedVersions) != 1 || d.Protocol.SupportedVersions[0] != "2026-04-17" ||
		d.Protocol.DocumentationURL != "https://docs.shop.example/acp" || d.APIBaseURL != "https://api.shop.example/acp" {
		t.Fatalf("protocol = %+v", d)
	}
	// MUST include rest; the seller serves checkout only: no orders API, no delegate_payment (the PSP's).
	if len(d.Transports) != 1 || d.Transports[0] != "rest" || len(d.Capabilities.Services) != 1 || d.Capabilities.Services[0] != "checkout" {
		t.Fatalf("transports=%v services=%v", d.Transports, d.Capabilities.Services)
	}
	if len(d.Capabilities.SupportedCurrencies) != 1 || d.Capabilities.SupportedCurrencies[0] != "usd" || len(d.Capabilities.SupportedLocales) != 1 {
		t.Fatalf("capabilities = %+v", d.Capabilities)
	}
	// It names no merchant (rfc.discovery.md §7.4).
	for _, k := range []string{"merchant_id", "merchant"} {
		if strings.Contains(w.Body.String(), k) {
			t.Fatalf("the document names a merchant: %s", w.Body)
		}
	}
}

func TestDiscoveryMinimalHasTheSpecsMinimalShape(t *testing.T) {
	h := discover(t, "https://merchant.example.com/api", "", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/.well-known/acp.json", nil))
	var got, want map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	_ = json.Unmarshal([]byte(spec(t, "discovery_response_minimal")), &want)
	for k := range want {
		if _, ok := got[k]; !ok {
			t.Errorf("the document lacks %s", k)
		}
	}
	p := got["protocol"].(map[string]any)
	if _, ok := p["documentation_url"]; ok {
		t.Fatal("an empty documentation_url was written")
	}
	if _, ok := got["capabilities"].(map[string]any)["supported_locales"]; ok {
		t.Fatal("an empty supported_locales was written")
	}
}

func TestDiscoveryAnswersGetAndHeadOnly(t *testing.T) {
	h := discover(t, "https://merchant.example.com/api", "", nil)
	for _, m := range []string{http.MethodGet, http.MethodHead} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(m, "/.well-known/acp.json", nil))
		if w.Code != 200 {
			t.Fatalf("%s = %d", m, w.Code)
		}
	}
	for _, m := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(m, "/.well-known/acp.json", nil))
		if w.Code != 405 || w.Header().Get("Allow") != "GET, HEAD" {
			t.Fatalf("%s = %d %v", m, w.Code, w.Header())
		}
		loadSchema(t).valid(t, "Error", w.Body.Bytes())
	}
}

func TestDiscoveryRefusesABaseURLAnAgentCouldNotUse(t *testing.T) {
	r := newRig(t)
	for _, base := range []string{"", "/acp", "http://merchant.example.com/api", "https:///api", "merchant.example.com"} {
		if _, err := r.svc.Discover(base, "", nil); err == nil {
			t.Errorf("%q accepted", base)
		}
	}
}
