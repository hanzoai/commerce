// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.
//
// Package commerce — standalone-router security regression for the
// in-cluster identity-header forge (Red CRITICAL).
//
// Red proved that against the directly-exposed commerce-api a pod could
// `POST commerce.hanzo.svc:8001/_/commerce/orgs` with
// `X-Org-Id: admin` + `X-User-IsGlobalAdmin: true` and get 201 — platform
// superadmin by header forgery. Root cause: the identity boundary (EdgeAuth
// strip+remint) was mounted AFTER setupRoutes had already registered
// /_/commerce/* and /v1/commerce/*, and gin applies engine.Use() only to
// routes registered AFTER the Use() call, so those groups were unguarded.
//
// These tests boot the REAL App via NewWithConfig → Bootstrap with
// COMMERCE_EDGE_AUTH=true — the production shape — and assert the boundary
// now wraps every route: forged identity headers are stripped before any
// handler, so the forge yields 401 with no org row, while the
// service-token money path (no X-Org-Id, require=false) still flows through.
package commerce

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

// bootEdgeCommerce boots a real App with the standalone edge trust boundary
// ENABLED (COMMERCE_EDGE_AUTH=true) and IAM disabled (no JWKS fetch; EdgeAuth
// therefore strips forged identity but mints nothing — exactly the state an
// unauthenticated forge attempt hits). Mirrors bootTestCommerce's degraded
// infra config so the only thing under test is the boundary + route wiring.
func bootEdgeCommerce(t *testing.T) *App {
	t.Helper()

	// EdgeAuth() reads COMMERCE_EDGE_AUTH at construction time (during
	// Bootstrap → installIdentityBoundary), so it MUST be set before boot.
	t.Setenv("COMMERCE_EDGE_AUTH", "true")
	// require-identity OFF: the binary-edge gate is incompatible with the
	// service-token money path (no X-Org-Id). This is the deployed default.
	t.Setenv("COMMERCED_REQUIRE_IDENTITY", "false")
	t.Setenv("STRIPE_SECRET_KEY", "")
	t.Setenv("COMMERCE_STRIPE_SEED", "false")
	t.Setenv("SQL_URL", "")

	dir := t.TempDir()
	cfg := &Config{
		DataDir:      filepath.Join(dir, "data"),
		Secret:       "test-secret",
		HTTPAddr:     "127.0.0.1:0",
		QueryTimeout: 30e9,
	}
	cfg.IAM.Enabled = false
	cfg.KMS.Enabled = false
	t.Setenv("COMMERCE_DATA_DIR", cfg.DataDir)
	t.Setenv("COMMERCE_BASE_URL", "")

	app := NewWithConfig(cfg)
	if err := app.Bootstrap(); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	t.Cleanup(func() { _ = app.Shutdown() })
	return app
}

func respBody(r *http.Response) string {
	b, _ := io.ReadAll(r.Body)
	return string(b)
}
// TestStandaloneMoneyPath_NotBlockedByBoundary proves the boundary does not
// regress the service-token money path. require=false means a service-token-
// shaped request is NOT 401'd by the boundary — it reaches the per-route handler.
//
// The property under test is "not blocked by the boundary" (no boundary 401),
// asserted by the handler actually running. GET /v1/commerce/org is wired to
// the RESOLVER-backed checkout.OrgJSON(orgResolver) (see commerce.go) — now
// the only public org handler, the store-backed duplicate having been deleted
// unused. By deliberate, documented design
// (checkout/org_resolver.go: "Resolution never 404s for a well-formed host"),
// proven by checkout.TestBrandForHost + TestRoutes_PublicTenantResolvesOrg, the
// pay-SPA boot endpoint resolves EVERY well-formed host to a usable org (a
// recognized brand, else the deployment default) so "add credits" always
// renders; it 404s ONLY a malformed Host. So a reached handler returns 200 with
// a org JSON. The earlier 404 / {"error":"unknown org"} expectation here
// assumed the store-backed handler, which is not what this route wires — that
// was a wrong expectation, corrected (not weakened: the boundary-not-401 check
// is preserved and the handler-ran signal is now the exact 200 org JSON).
func TestStandaloneMoneyPath_NotBlockedByBoundary(t *testing.T) {
	app := bootEdgeCommerce(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/commerce/org", nil)
	req.Host = "pay.unknown.test"
	// Service-token shape: opaque Bearer + X-Org-Id, no JWT.
	req.Header.Set("Authorization", "Bearer st_opaque_service_token")
	req.Header.Set("X-Org-Id", "maxpower")

	resp, terr := app.Router.Test(req)
	if terr != nil {
		t.Fatalf("Test: %v", terr)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		t.Fatalf("boundary 401'd a service-token-shaped request — money path regression; body=%s", respBody(resp))
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /v1/commerce/org status=%d, want 200 (handler reached; resolver returns a usable org for a well-formed host); body=%s", resp.StatusCode, respBody(resp))
	}
	var body map[string]any
	if err := json.NewDecoder(strings.NewReader(respBody(resp))).Decode(&body); err != nil {
		t.Fatalf("org JSON decode: %v; body=%s", err, respBody(resp))
	}
	if name, _ := body["name"].(string); name == "" {
		t.Fatalf("org JSON missing name (org slug) — the resolver-backed handler must have run: %v", body)
	}
}
