// Deposit follow-up handlers: confirm, status, webhooks.
//
// Shape mirrors Deposits() — tenant resolved from Host, forwarded to
// the tenant's Backend.URL. Each handler pins its own upstream sub-path
// so the proxy layer is the single enforcement point for path
// validation. IDs are taken from route params (not the request body)
// and re-escaped for path safety.

package checkout

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/zap-proto/zip"
)

// DepositConfirm handles POST /v1/commerce/deposits/:id/confirm. The SPA
// posts the provider-minted token (e.g. Square nonce) back here so BD
// can complete the pre-auth → capture flow. We never touch the provider
// directly from commerce — BD owns that call path and the audit record.
func DepositConfirm(r Resolver, fwd Forwarder) zip.Handler {
	return func(c *zip.Ctx) error {
		tenant, ok := resolveOr404(c, r)
		if !ok {
			return nil
		}
		if c.Header("Authorization") == "" {
			return writeJSONError(c, http.StatusUnauthorized, "authorization required")
		}
		id := c.Param("id")
		if id == "" {
			return writeJSONError(c, http.StatusBadRequest, "missing deposit id")
		}
		return proxyToBackend(c, tenant, fwd,
			http.MethodPost,
			"/v1/bd/deposits/"+url.PathEscape(id)+"/confirm",
		)
	}
}

// DepositStatus handles GET /v1/commerce/deposits/:id/status. Returns
// the BD-owned state machine (pending, processing, settled, failed). The
// SPA polls this until terminal or timeout.
func DepositStatus(r Resolver, fwd Forwarder) zip.Handler {
	return func(c *zip.Ctx) error {
		tenant, ok := resolveOr404(c, r)
		if !ok {
			return nil
		}
		if c.Header("Authorization") == "" {
			return writeJSONError(c, http.StatusUnauthorized, "authorization required")
		}
		id := c.Param("id")
		if id == "" {
			return writeJSONError(c, http.StatusBadRequest, "missing deposit id")
		}
		return proxyToBackend(c, tenant, fwd,
			http.MethodGet,
			"/v1/bd/deposits/"+url.PathEscape(id)+"/status",
		)
	}
}

// WebhookIntake handles POST /v1/commerce/webhooks/:provider. The
// provider (Square, Braintree, etc.) posts settlement/dispute events
// here. We DO NOT verify the provider's signature in commerce —
// signature keys live in BD + the tenant-scoped KMS secret that BD
// already owns, so we forward the payload + original signature headers
// verbatim so BD can verify with its own tenant-scoped key.
//
// Why not verify here: key rotation races. If commerce cached a stale
// signing key it would reject live webhooks. BD is the only source of
// truth for provider keys; having commerce also hold them would be two
// places to rotate, and two places to forget.
func WebhookIntake(r Resolver) zip.Handler {
	fwd := NewHTTPForwarder()
	return func(c *zip.Ctx) error {
		tenant, ok := resolveOr404(c, r)
		if !ok {
			return nil
		}
		provider := c.Param("provider")
		if provider == "" || !isKnownProvider(provider) {
			return writeJSONError(c, http.StatusNotFound, "unknown provider")
		}
		// Forward to BD's provider-specific webhook intake.
		// URL-escape the provider segment even though we validated it
		// against an allowlist — defense in depth.
		sub := "/v1/bd/webhooks/" + url.PathEscape(provider)
		return proxyWebhook(c, tenant, fwd, sub)
	}
}

// ─── helpers ────────────────────────────────────────────────────────────

// resolveOr404 is the common tenant-resolve preamble. On failure it
// writes a 404 with no Host echo and returns ok=false.
func resolveOr404(c *zip.Ctx, r Resolver) (Tenant, bool) {
	t, err := r.Resolve(RequestHost(c))
	if err != nil {
		_ = writeJSONError(c, http.StatusNotFound, "unknown tenant")
		return Tenant{}, false
	}
	return t, true
}

// proxyToBackend forwards the request to tenant.Backend.URL + sub with
// Authorization + Content-Type preserved, body copied, and an
// X-Commerce-Tenant header set so BD logs can attribute correctly.
func proxyToBackend(
	c *zip.Ctx,
	tenant Tenant,
	fwd Forwarder,
	method, sub string,
) error {
	if tenant.Backend.URL == "" {
		return writeJSONError(c, http.StatusServiceUnavailable, "tenant backend not configured")
	}

	upstreamURL := strings.TrimRight(tenant.Backend.URL, "/") + sub

	var body io.Reader
	if b := c.Body(); len(b) > 0 {
		// c.Body() is the request buffer (zero-copy); the reader is consumed
		// before the handler returns, so no copy is needed.
		body = bytes.NewReader(b)
	}

	up, err := http.NewRequestWithContext(c.Context(), method, upstreamURL, body)
	if err != nil {
		return writeJSONError(c, http.StatusInternalServerError, "build upstream")
	}
	up.Header.Set("Authorization", c.Header("Authorization"))
	if ct := c.Header("Content-Type"); ct != "" {
		up.Header.Set("Content-Type", ct)
	}
	up.Header.Set("X-Commerce-Tenant", tenant.Name)

	resp, err := fwd.Forward(up, tenant)
	if err != nil {
		return writeJSONError(c, http.StatusBadGateway, "upstream error")
	}
	defer resp.Body.Close()

	// Buffer the upstream response fully before writing. fiber's SendStream
	// is lazy (SetBodyStream) — it reads resp.Body when the response flushes,
	// which is AFTER this function's deferred Close has already run, so
	// streaming would race the close and truncate the body. Read-then-Bytes
	// matches Deposits(): one proxy-response path, no lazy-close footgun.
	respBody, _ := io.ReadAll(resp.Body)
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		c.SetHeader("Content-Type", ct)
	}
	return c.Bytes(resp.StatusCode, respBody)
}

// proxyWebhook forwards a provider webhook to BD verbatim, preserving
// every header the provider sent (signatures live there). This is the
// one case where we pass headers through un-scrubbed because the
// upstream needs them all for signature verification.
func proxyWebhook(
	c *zip.Ctx,
	tenant Tenant,
	fwd Forwarder,
	sub string,
) error {
	if tenant.Backend.URL == "" {
		return writeJSONError(c, http.StatusServiceUnavailable, "tenant backend not configured")
	}

	upstreamURL := strings.TrimRight(tenant.Backend.URL, "/") + sub

	up, err := http.NewRequestWithContext(c.Context(), http.MethodPost, upstreamURL, bytes.NewReader(c.Body()))
	if err != nil {
		return writeJSONError(c, http.StatusInternalServerError, "build upstream")
	}
	// Forward ALL original headers — provider signatures are in there.
	// The Authorization header (if any) is provider-specific, not a
	// user bearer, so forwarding it is correct.
	c.Fiber().Request().Header.VisitAll(func(kb, vb []byte) {
		k := string(kb)
		// Drop hop-by-hop headers that don't belong on the upstream
		// request (connection state, not application payload).
		switch strings.ToLower(k) {
		case "connection", "keep-alive", "proxy-authenticate",
			"proxy-authorization", "te", "trailer", "transfer-encoding",
			"upgrade", "host", "content-length":
			return
		}
		up.Header.Add(k, string(vb))
	})
	up.Header.Set("X-Commerce-Tenant", tenant.Name)

	resp, err := fwd.Forward(up, tenant)
	if err != nil {
		return writeJSONError(c, http.StatusBadGateway, "upstream error")
	}
	defer resp.Body.Close()
	// Buffer before writing — see proxyToBackend on fiber SendStream's
	// lazy-flush vs. deferred Close.
	respBody, _ := io.ReadAll(resp.Body)
	return c.Bytes(resp.StatusCode, respBody)
}

// isKnownProvider bounds the :provider URL param to an allowlist so
// path traversal, unicode chicanery, and provider-name typos return a
// clean 404 before we forward anywhere. Extend when a new provider is
// onboarded; an allowlist is the boring, auditable control.
func isKnownProvider(name string) bool {
	switch name {
	case "square", "braintree", "stripe", "paypal":
		return true
	}
	return false
}
