// Package checkout tests — org resolver must withstand Host-header
// spoofing and never leak secrets through the public org JSON endpoint.
package checkout

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/zap-proto/zip"
)

// serveOrg runs OrgJSON(r) against a synthetic request carrying the
// given Host and returns the status + body. It uses TestCtx (not
// app.Fiber().Test) so malformed hosts — control bytes, empty, bare ":" —
// reach the handler's own normalizeHost guard unfiltered; fiber's HTTP
// parser would otherwise reject them before the handler runs.
func serveOrg(t *testing.T, r Resolver, host string) (int, string) {
	t.Helper()
	app := zip.New(zip.Config{DisableStartupMessage: true})
	c := app.TestCtx(http.MethodGet, "/checkout/v1/org")
	// c.Fiber().Host() reads the URI host (fiber's DefaultReq.Host), so inject
	// there — Header.SetHost would be ignored. Setting it directly also lets
	// malformed hosts (control bytes, bare ":") reach normalizeHost unfiltered.
	c.Fiber().Request().URI().SetHost(host)
	if err := OrgJSON(r)(c); err != nil {
		t.Fatalf("OrgJSON: %v", err)
	}
	return c.Fiber().Response().StatusCode(), string(c.Fiber().Response().Body())
}

// ─── Resolver ────────────────────────────────────────────────────────────

func TestResolveOrg_KnownHostname(t *testing.T) {
	r := newHostOrgs(map[string]Org{
		"pay.example.com":      {Name: "examplecorp", Brand: Brand{DisplayName: "ExampleCorp"}},
		"pay.dev.example.com":  {Name: "examplecorp", Brand: Brand{DisplayName: "ExampleCorp"}},
		"pay.test.example.com": {Name: "examplecorp", Brand: Brand{DisplayName: "ExampleCorp"}},
	})

	cases := []struct {
		host, want string
	}{
		{"pay.example.com", "examplecorp"},
		{"pay.dev.example.com", "examplecorp"},
		{"pay.test.example.com", "examplecorp"},
		// Port suffix must be stripped before lookup.
		{"pay.example.com:443", "examplecorp"},
		// Case-insensitive.
		{"PAY.EXAMPLE.COM", "examplecorp"},
	}

	for _, tc := range cases {
		got, err := r.Resolve(tc.host)
		if err != nil {
			t.Errorf("Resolve(%q) err = %v, want nil", tc.host, err)
			continue
		}
		if got.Name != tc.want {
			t.Errorf("Resolve(%q).Name = %q, want %q", tc.host, got.Name, tc.want)
		}
	}
}

func TestResolveOrg_UnknownHostname(t *testing.T) {
	r := newHostOrgs(map[string]Org{
		"pay.example.com": {Name: "examplecorp"},
	})

	// Arbitrary unrelated hosts MUST NOT match.
	for _, host := range []string{
		"evil.com",
		"pay.evil.com",
		"example.com.evil.com", // suffix-match attack
		"xyzpay.example.com",
		"",
	} {
		if _, err := r.Resolve(host); err != ErrUnknownOrg {
			t.Errorf("Resolve(%q) err = %v, want ErrUnknownOrg", host, err)
		}
	}
}

// An attacker setting a Host header like `pay.example.com.evil.com` must
// not be resolved as examplecorp. Exact-match only (after port/case
// normalization).
func TestResolveOrg_SuffixSpoofing(t *testing.T) {
	r := newHostOrgs(map[string]Org{
		"pay.example.com": {Name: "examplecorp"},
	})
	spoofs := []string{
		"pay.example.com.attacker.test",
		"attacker.pay.example.com",
		" pay.example.com",
		"pay.example.com ",
	}
	for _, s := range spoofs {
		if _, err := r.Resolve(s); err != ErrUnknownOrg {
			t.Errorf("Resolve(%q) was accepted — expected rejection", s)
		}
	}
}

// ─── Public org JSON endpoint ─────────────────────────────────────────

// Public org JSON must include branding + public IAM config + enabled
// payment method NAMES, but MUST NEVER leak secrets: no access tokens, no
// client secrets, no webhook keys, no KMS paths.
func TestOrgJSON_NeverLeaksSecrets(t *testing.T) {
	org := Org{
		Name: "examplecorp",
		Brand: Brand{
			DisplayName:  "ExampleCorp",
			LogoURL:      "https://cdn.example.com/examplecorp.png",
			PrimaryColor: "#0ea5e9",
		},
		IAM: IAMConfig{
			Issuer:   "https://id.example.com",
			ClientID: "examplecorp-exchange-client-id",
			// These MUST NOT leak:
			ClientSecret: "secret-do-not-share",
			AdminSecret:  "even-more-secret",
		},
		Providers: []Provider{
			{Name: "square", Enabled: true, AccessToken: "EAAA-secret-token", WebhookSignatureKey: "whk-secret"},
			{Name: "braintree", Enabled: false, PrivateKey: "bt-secret"},
		},
		ReturnURLAllowlist: []string{"https://exchange.example.com"},
		Backend:            BackendConfig{URL: "https://bd.example.com", Kind: "bd"},
	}
	r := newHostOrgs(map[string]Org{"pay.example.com": org})

	code, body := serveOrg(t, r, "pay.example.com")

	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}

	// These strings MUST be absent from the payload.
	forbidden := []string{
		"secret-do-not-share",
		"even-more-secret",
		"EAAA-secret-token",
		"whk-secret",
		"bt-secret",
		"ClientSecret",
		"AdminSecret",
		"AccessToken",
		"PrivateKey",
		"WebhookSignatureKey",
	}
	for _, s := range forbidden {
		if strings.Contains(body, s) {
			t.Errorf("org JSON leaked %q — body:\n%s", s, body)
		}
	}

	// These fields MUST be present.
	required := []string{
		"examplecorp",
		"ExampleCorp",
		"#0ea5e9",
		"https://id.example.com",
		"examplecorp-exchange-client-id",
		"square",
	}
	for _, s := range required {
		if !strings.Contains(body, s) {
			t.Errorf("org JSON missing %q — body:\n%s", s, body)
		}
	}

	// Disabled providers must not show up in the enabled-methods list.
	var resp map[string]any
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	methods, _ := resp["providers"].([]any)
	for _, m := range methods {
		if p, ok := m.(map[string]any); ok {
			if name, _ := p["name"].(string); name == "braintree" {
				t.Errorf("disabled provider %q surfaced in public org JSON", name)
			}
		}
	}
}

func TestOrgJSON_UnknownHostReturns404(t *testing.T) {
	r := newHostOrgs(map[string]Org{
		"pay.example.com": {Name: "examplecorp"},
	})
	code, body := serveOrg(t, r, "evil.com")

	if code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 for unknown org", code)
	}
	// 404 body MUST NOT echo the Host back — reflection would help
	// attackers fingerprint org existence.
	if strings.Contains(body, "evil.com") {
		t.Errorf("unknown-org 404 echoed Host header — body: %s", body)
	}
}

// serveOrgForwarded drives OrgJSON with an explicit URI host AND an
// X-Forwarded-Host header — the shape a request actually has behind the ingress.
func serveOrgForwarded(t *testing.T, r Resolver, uriHost, forwarded string) (int, string) {
	t.Helper()
	app := zip.New(zip.Config{DisableStartupMessage: true})
	c := app.TestCtx(http.MethodGet, "/checkout/v1/org")
	c.Fiber().Request().URI().SetHost(uriHost)
	if forwarded != "" {
		c.Fiber().Request().Header.Set("X-Forwarded-Host", forwarded)
	}
	if err := OrgJSON(r)(c); err != nil {
		t.Fatalf("OrgJSON: %v", err)
	}
	return c.Fiber().Response().StatusCode(), string(c.Fiber().Response().Body())
}

// THE production regression (measured live 2026-07-30). Behind the ingress the
// parsed URI host is empty and the ONLY carrier of the customer-facing hostname
// is X-Forwarded-Host. forwardedHostMiddleware lifted it into the Host HEADER,
// which fiber's Host() ignores (see serveOrg's note above) — so every
// well-formed host normalized to "" and the public org read 404'd
// {"error":"unknown org"} on pay.hanzo.ai AND api.hanzo.ai, taking the whole
// card/top-up path down. The sibling /v1/commerce/catalog on the same group
// stayed 200 because it reads ?brand=, never the host.
func TestOrgJSON_ResolvesFromForwardedHost(t *testing.T) {
	r := newHostOrgs(map[string]Org{"pay.example.com": {Name: "examplecorp"}})

	code, body := serveOrgForwarded(t, r, "", "pay.example.com")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200 — forwarded host must resolve; body: %s", code, body)
	}
	if !strings.Contains(body, "examplecorp") {
		t.Errorf("org not resolved from X-Forwarded-Host — body: %s", body)
	}
}

// A proxy chain APPENDS, so the left-most entry is the original client-facing
// host and every later one is an internal hop.
func TestOrgJSON_ForwardedHostChainUsesLeftmost(t *testing.T) {
	r := newHostOrgs(map[string]Org{"pay.example.com": {Name: "examplecorp"}})

	code, body := serveOrgForwarded(t, r, "", "pay.example.com, internal.svc")
	if code != http.StatusOK || !strings.Contains(body, "examplecorp") {
		t.Fatalf("status=%d body=%s — want the left-most forwarded host to win", code, body)
	}
}

// A real parsed host is the direct-connection truth and MUST NOT be overridable
// by a client-supplied header. The forwarded value is a fallback, never an
// override — otherwise any caller could select another brand's org at will.
func TestOrgJSON_ParsedHostWinsOverForwarded(t *testing.T) {
	r := newHostOrgs(map[string]Org{
		"pay.example.com": {Name: "examplecorp"},
		"evil.com":        {Name: "attacker"},
	})

	code, body := serveOrgForwarded(t, r, "pay.example.com", "evil.com")
	if code != http.StatusOK || !strings.Contains(body, "examplecorp") {
		t.Fatalf("status=%d body=%s — parsed host must win over X-Forwarded-Host", code, body)
	}
	if strings.Contains(body, "attacker") {
		t.Error("X-Forwarded-Host overrode a real parsed host")
	}
}

// When NOTHING carries a host the 404 is the honest answer and stays.
func TestOrgJSON_NoHostAnywhereStill404(t *testing.T) {
	r := newHostOrgs(map[string]Org{"pay.example.com": {Name: "examplecorp"}})

	code, _ := serveOrgForwarded(t, r, "", "")
	if code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 when no host is present anywhere", code)
	}
}

// Host header with no mapping must not panic and must not 500.
func TestOrgJSON_MalformedHost(t *testing.T) {
	r := newHostOrgs(map[string]Org{})
	for _, h := range []string{"", ":", "::8080", "\x00badhost"} {
		code, _ := serveOrg(t, r, h)
		if code != http.StatusNotFound {
			t.Errorf("Host=%q: status = %d, want 404", h, code)
		}
	}
}
