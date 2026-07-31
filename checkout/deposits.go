// Deposits is the POST /v1/commerce/deposits handler. The checkout SPA
// POSTs here to create a deposit intent; we resolve the tenant from the
// Host header, then forward to the tenant's configured backend. For
// broker-dealer tenants, Backend.Kind="bd" and Backend.URL points at
// the BD service, which owns the deposit-intent lifecycle.
//
// Why a proxy and not a local implementation: commerce does not own
// deposit state for these tenants — the BD service does, and is the
// source of truth per the on-chain trading architecture. Re-implementing
// here would split the record and invite drift.

package checkout

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/zap-proto/zip"
)

// Forwarder wraps "given this request and this tenant, produce a
// response". The production Forwarder is a net/http.Client wired to the
// tenant's Backend.URL; tests substitute a ForwarderFunc.
type Forwarder interface {
	Forward(req *http.Request, tenant Tenant) (*http.Response, error)
}

// ForwarderFunc adapts a function to the Forwarder interface.
type ForwarderFunc func(req *http.Request, tenant Tenant) (*http.Response, error)

// Forward calls f.
func (f ForwarderFunc) Forward(req *http.Request, tenant Tenant) (*http.Response, error) {
	return f(req, tenant)
}

// Deposits handles POST /v1/commerce/deposits. Preconditions:
//  1. Host resolves to a known tenant (404 if not).
//  2. Authorization header is present (401 if not — IAM middleware at
//     the commerce router will re-validate the JWT; we only enforce
//     presence here to fail fast before forwarding anywhere).
//  3. Tenant.Backend.URL is configured (503 if not — fail closed, never
//     fall back to a default).
//
// On success the upstream response is streamed back to the client
// verbatim so the SPA can consume { id, provider, clientToken, ... }.
func Deposits(r Resolver, fwd Forwarder) zip.Handler {
	return func(c *zip.Ctx) error {
		tenant, err := r.Resolve(RequestHost(c))
		if err != nil {
			return writeJSONError(c, http.StatusNotFound, "unknown tenant")
		}
		if c.Header("Authorization") == "" {
			return writeJSONError(c, http.StatusUnauthorized, "authorization required")
		}
		if tenant.Backend.URL == "" {
			return writeJSONError(c, http.StatusServiceUnavailable, "tenant backend not configured")
		}

		// Build an upstream request. We force the URL to the tenant's
		// Backend.URL — never the Host header — so a spoofed Host cannot
		// redirect the forwarder to an attacker-controlled URL.
		upstreamURL := strings.TrimRight(tenant.Backend.URL, "/") + "/v1/bd/deposits"
		up, err := http.NewRequestWithContext(c.Context(), http.MethodPost, upstreamURL, bytes.NewReader(c.Body()))
		if err != nil {
			return writeJSONError(c, http.StatusInternalServerError, "build upstream")
		}
		// Forward the Authorization header + content type; drop
		// everything else so cookies, custom headers, etc. do not leak
		// to the backend.
		up.Header.Set("Authorization", c.Header("Authorization"))
		up.Header.Set("Content-Type", c.Header("Content-Type"))
		up.Header.Set("X-Commerce-Tenant", tenant.Name)

		resp, err := fwd.Forward(up, tenant)
		if err != nil {
			return writeJSONError(c, http.StatusBadGateway, "upstream error")
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		if ct := resp.Header.Get("Content-Type"); ct != "" {
			c.SetHeader("Content-Type", ct)
		}
		return c.Bytes(resp.StatusCode, body)
	}
}

// writeJSONError emits a compact JSON error. Intentionally does NOT echo
// the Host back so attackers cannot use error responses to fingerprint
// tenant existence.
func writeJSONError(c *zip.Ctx, code int, msg string) error {
	c.SetHeader("Content-Type", "application/json; charset=utf-8")
	c.SetHeader("Cache-Control", "no-store")
	return c.Bytes(code, []byte(fmt.Sprintf(`{"error":%q}`, msg)))
}
