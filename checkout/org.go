// Package checkout is the hosted multi-org checkout SPA embedded into
// commerce. The Vite build lives under ui/ and ships into ui/dist via the
// Dockerfile's pay-dist stage; embed.go exposes it to the Go binary.
//
// Security posture:
//   - Org resolution is exact-match on the Host header after port/case
//     normalization. Suffix-match tricks ("pay.example.com.evil.com") are
//     rejected by design.
//   - The public org JSON endpoint (GET /v1/commerce/org) exposes
//     ONLY branding, public IAM client ID + issuer, return-URL allowlist,
//     and the NAMES of enabled payment providers. No secrets, no KMS
//     paths, no client secrets, no webhook keys.
//   - Writes are scoped to the resolved org; cross-org mutations are
//     handled at the API layer (see deposits.go + admin/org handlers)
//     by cross-checking the IAM claim against the resolved org name.
package checkout

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/payment"
	"github.com/hanzoai/commerce/payment/processor"
)

// ErrUnknownOrg is returned when the incoming Host header does not map
// to a configured org. Callers should respond with 404 (never 500) and
// MUST NOT echo the Host back in the response body — that would be a free
// fingerprinting primitive for attackers.
var ErrUnknownOrg = errors.New("checkout: unknown org")

// Org is the full org config. Only the fields tagged `json:"..."`
// (no `-` suffix) flow to the public GET /v1/commerce/org endpoint via
// the PublicView projection — everything else (secrets, backend creds) is
// dropped before serialization.
type Org struct {
	// Name is the stable org identifier (also the Hanzo IAM org name /
	// commerce organization.Name). Used to scope KMS paths, DB queries,
	// and IAM owner-claim comparisons.
	Name string `json:"name"`

	// Brand controls what the SPA renders.
	Brand Brand `json:"brand"`

	// IAM points the SPA at the correct identity provider and app. Only
	// the public fields (Issuer, ClientID) project to PublicView; the rest
	// stay server-side and are used by the checkout API handlers.
	IAM IAMConfig `json:"iam"`

	// IDV (identity verification) is opaque to the server: the SPA just
	// reads it, renders a redirect/prompt, and trusts the IDV provider's
	// completion webhook (handled by the org's back end, not here).
	IDV IDVConfig `json:"idv"`

	// Providers is the per-org enable/disable list for payment
	// providers. The PublicView projection strips all credential fields
	// before emission.
	Providers []Provider `json:"providers"`

	// ReturnURLAllowlist bounds the ?return= query param the SPA may
	// bounce to. Prevents open-redirect phishing pivots.
	ReturnURLAllowlist []string `json:"returnUrlAllowlist"`

	// Square carries the PUBLIC Square Web Payments config (application id +
	// location id + environment) the SPA's card iframe needs. Resolved from the
	// IAM org via the single test-mode authority (see payment.SquarePublicConfig)
	// so the browser tokenizes with the exact application commerce will charge.
	// Every field is public — no secret ever crosses this boundary.
	Square SquarePublic `json:"square"`

	// PayPal carries the PUBLIC PayPal JS SDK config (client id + environment)
	// so the SPA can mount PayPal Smart Buttons alongside Square in the 8
	// Square-supported countries and replace Square everywhere else. Every field
	// is public — no client secret or webhook ID ever crosses this boundary.
	PayPal PayPalPublic `json:"paypal"`

	// Stripe carries the PUBLIC Stripe.js config (publishable key + environment)
	// for the currencies whose card form is Stripe's (Cards). Every field is
	// public — the secret key never crosses this boundary.
	Stripe StripePublic `json:"stripe"`

	// Cards maps a currency ("usd") to the processor whose card form takes it
	// for this caller ("square" or "stripe"): Square wherever Square can, Stripe
	// where it cannot. It is processor.Registry.Cards over the processors this
	// brand may show, for the caller's country. A currency no card processor
	// takes is absent.
	Cards map[string]string `json:"cards"`

	// Country is the caller's resolved ISO 3166-1 alpha-2 country code when
	// present on the request (?country= or edge geo-IP headers).
	Country string `json:"country,omitempty"`

	// reg is the processors Cards is chosen from. It never serializes.
	reg *processor.Registry

	// Backend tells the checkout API how to proxy deposit intents. For
	// the example org this resolves to BD; other orgs supply
	// their own URL. Kind is an opaque free-form label ("bd", "custom").
	Backend BackendConfig `json:"-"`
}

// SquarePublic is the public Square config surfaced to the checkout SPA. It
// mirrors payment.SquarePublic minus the ledger `live` flag (the SPA drives its
// sandbox-vs-prod script off Environment). All fields are public.
type SquarePublic struct {
	ApplicationID string `json:"applicationId"`
	LocationID    string `json:"locationId"`
	Environment   string `json:"environment"`
}

// PayPalPublic is the public PayPal JS SDK config surfaced to the checkout SPA.
// All fields are public.
type PayPalPublic struct {
	ClientID    string `json:"clientId"`
	Environment string `json:"environment"`
}

// StripePublic is the public Stripe.js config surfaced to the checkout SPA.
// All fields are public.
type StripePublic struct {
	PublishableKey string `json:"publishableKey"`
	Environment    string `json:"environment"`
}

// Brand controls visible white-label surface.
type Brand struct {
	DisplayName  string `json:"displayName"`
	LogoURL      string `json:"logoUrl"`
	PrimaryColor string `json:"primaryColor"`
	// The entity the buyer is contracting with, and the policies it publishes.
	// A checkout is a contract; the footer is where it says whose. All three are
	// omitted when unset so a brand that has not stated them renders nothing
	// rather than a wrong name or a 404.
	LegalName  string `json:"legalName,omitempty"`
	TermsURL   string `json:"termsUrl,omitempty"`
	PrivacyURL string `json:"privacyUrl,omitempty"`
}

// IAMConfig: Issuer + ClientID are OIDC-public (they already ship in the
// well-known discovery doc). ClientSecret and AdminSecret are server-side
// and MUST NOT project to PublicView.
type IAMConfig struct {
	Issuer       string `json:"issuer"`
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"-"`
	AdminSecret  string `json:"-"`
}

// IDVConfig: opaque to commerce. Provider is a label the SPA switches on;
// Endpoint is the URL the SPA opens for the IDV flow. RequiredFields is a
// whitelist of claims the org requires from the IDV provider.
type IDVConfig struct {
	Provider       string   `json:"provider"`
	Endpoint       string   `json:"endpoint"`
	RequiredFields []string `json:"requiredFields,omitempty"`
}

// Provider is a payment provider the org has enabled. All credential
// fields are `json:"-"` so json.Marshal drops them — the PublicView
// projection relies on this.
type Provider struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`

	// The following are server-side only. KMSPath is the KMS folder that
	// holds this provider's credentials; AccessToken et al are optional
	// fallbacks for bootstrap / local dev.
	KMSPath             string `json:"-"`
	ApplicationID       string `json:"-"`
	AccessToken         string `json:"-"`
	PrivateKey          string `json:"-"`
	WebhookSignatureKey string `json:"-"`
}

// BackendConfig describes where the checkout API forwards deposit
// intents. Backend.Kind is org-specific.
// For generic orgs, Kind="custom" and URL is the org's own endpoint.
type BackendConfig struct {
	Kind string
	URL  string
}

// ─── Resolver ────────────────────────────────────────────────────────────

// Resolver resolves a Host header to a Org. The default implementation
// is an in-memory map driven by hostname→Org (tests only); in production the
// resolver is backed by the commerce organization model (hosts stored on
// the organization record).
type Resolver interface {
	Resolve(host string) (Org, error)
}

// normalizeHost strips :port and lowercases. Any malformed input —
// embedded whitespace, control bytes, empty string — is rejected (returns
// ""). We deliberately do NOT trim: a well-formed Host header has none,
// and silently repairing input turns a bug into an attack surface.
// RequestHost is the ONE way commerce learns the customer-facing hostname.
//
// fiber parses the request URI ONCE, so a middleware that rewrites the Host
// HEADER afterwards does not change what Host() returns. That is why lifting
// X-Forwarded-Host into the header (forwardedHostMiddleware) was a silent
// no-op behind the ingress: Host() stayed empty, normalizeHost returned "",
// and Resolve's only error path fired — 404 {"error":"unknown org"} on
// EVERY well-formed host. Measured live 2026-07-30 on pay.hanzo.ai and
// api.hanzo.ai, with the sibling /v1/commerce/catalog on the same group
// answering 200 (it reads ?brand=, never the host, so it could not see this).
//
// Order: the parsed host, then the forwarded host set by the trusted ingress,
// then the raw Host header. brandForHost is exact-suffix and an unknown host
// falls back to the deployment default, so a spoofed value can only ever
// select a brand's ALREADY-PUBLIC config (brand chrome, IAM client id, the
// Square PUBLIC application id). There is no probe oracle and nothing private
// behind it — the 404 body still never echoes the host.
func RequestHost(c *zip.Ctx) string {
	if h := normalizeHost(c.Host()); h != "" {
		return h
	}
	if h := normalizeHost(firstForwarded(c.Header("X-Forwarded-Host"))); h != "" {
		return h
	}
	return normalizeHost(c.Header("Host"))
}

// firstForwarded takes the LEFT-MOST entry of a comma-separated forwarding
// header: each proxy appends, so the left-most is the original client-facing
// value and every later one is an internal hop.
func firstForwarded(v string) string {
	if i := strings.IndexByte(v, ','); i >= 0 {
		v = v[:i]
	}
	return strings.TrimSpace(v)
}

func normalizeHost(host string) string {
	if host == "" {
		return ""
	}
	for i := 0; i < len(host); i++ {
		b := host[i]
		if b <= 0x20 || b == 0x7f {
			return "" // whitespace / DEL / control byte → reject
		}
	}
	// Strip :port. IPv6 literals start with '[' and are not supported as
	// org keys; they'd fall through to ErrUnknownOrg anyway.
	if i := strings.IndexByte(host, ':'); i >= 0 && !strings.HasPrefix(host, "[") {
		host = host[:i]
	}
	return strings.ToLower(host)
}

// ─── Public org JSON ──────────────────────────────────────────────────

// publicView is the JSON shape served at GET /v1/commerce/org. It's a
// deliberate projection — anything not explicitly listed here cannot leak.
type publicView struct {
	Name               string            `json:"name"`
	Brand              Brand             `json:"brand"`
	IAM                publicIAM         `json:"iam"`
	IDV                IDVConfig         `json:"idv"`
	Providers          []publicProvider  `json:"providers"`
	ReturnURLAllowlist []string          `json:"returnUrlAllowlist"`
	Square             SquarePublic      `json:"square"`
	PayPal             PayPalPublic      `json:"paypal"`
	Stripe             StripePublic      `json:"stripe"`
	Cards              map[string]string `json:"cards"`
	Country            string            `json:"country,omitempty"`
}

type publicIAM struct {
	Issuer   string `json:"issuer"`
	ClientID string `json:"clientId"`
}

type publicProvider struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

// toPublicView drops every field that is not safe to expose to an
// anonymous client. This is the single source of truth for "what does the
// SPA get to see" — org_test.go pins this contract.
func toPublicView(t Org) publicView {
	enabled := make([]publicProvider, 0, len(t.Providers))
	for _, p := range t.Providers {
		if !p.Enabled {
			continue
		}
		enabled = append(enabled, publicProvider{Name: p.Name, Enabled: true})
	}
	return publicView{
		Name:      t.Name,
		Brand:     t.Brand,
		IAM:       publicIAM{Issuer: t.IAM.Issuer, ClientID: t.IAM.ClientID},
		IDV:       t.IDV,
		Providers: enabled,
		// Always a non-nil slice so the JSON is `[]` not `null`. A nil
		// allowlist otherwise serializes as null, and the pay SPA iterates it
		// (firstOrigin/isAllowedReturnUrl) → "e is not iterable", which breaks
		// the entire /onboard page ("Could not load org configuration").
		ReturnURLAllowlist: append([]string{}, t.ReturnURLAllowlist...),
		Square:             t.Square,
		PayPal:             t.PayPal,
		Stripe:             t.Stripe,
		Cards:              cards(t.Cards),
		Country:            t.Country,
	}
}

// RequestCountry extracts and validates a 2-letter ISO 3166-1 alpha-2 country
// code from ?country= or edge geo-IP headers (Cloudflare, CloudFront, Vercel).
// Pseudo-codes ("XX", "T1") and malformed values return "".
func RequestCountry(c *zip.Ctx) string {
	for _, raw := range []string{
		c.Query("country"),
		c.Header("CF-IPCountry"),
		c.Header("CloudFront-Viewer-Country"),
		c.Header("X-Vercel-IP-Country"),
		c.Header("X-Country-Code"),
	} {
		code := normalizeCountryCode(firstForwarded(raw))
		if code != "" {
			return code
		}
	}
	return ""
}

func normalizeCountryCode(raw string) string {
	s := strings.ToUpper(strings.TrimSpace(raw))
	if len(s) != 2 {
		return ""
	}
	for i := 0; i < 2; i++ {
		if s[i] < 'A' || s[i] > 'Z' {
			return ""
		}
	}
	if s == "XX" || s == "T1" {
		return ""
	}
	return s
}

// applyCountryPolicy decides which payment rails a caller in country (ISO
// 3166-1 alpha-2, "" when unknown) is offered.
//
// Cards is the selection rule's answer (processor.Registry.Cards): per currency,
// Square's card form wherever Square takes the payment, Stripe's wherever it
// does not. Square and Stripe are each enabled only when they take some
// currency here, and a disabled one's public config is dropped. PayPal runs in
// every country PayPal operates in (payment.IsPayPalCountry); crypto and wire
// stay enabled everywhere.
func applyCountryPolicy(t Org, country string) Org {
	t.Country = country
	t.Cards = map[string]string{}
	if t.reg != nil {
		for cur, p := range t.reg.Cards(context.Background(), country) {
			t.Cards[string(cur)] = string(p)
		}
	}
	takes := func(name string) bool {
		for _, v := range t.Cards {
			if v == name {
				return true
			}
		}
		return false
	}
	paypalOK := payment.IsPayPalCountry(country)
	filtered := make([]Provider, 0, len(t.Providers))
	for _, p := range t.Providers {
		switch p.Name {
		case "square", "stripe":
			p.Enabled = p.Enabled && takes(p.Name)
		case "paypal":
			p.Enabled = p.Enabled && paypalOK
		}
		filtered = append(filtered, p)
	}
	t.Providers = filtered
	if !takes("square") {
		t.Square = SquarePublic{}
	}
	if !takes("stripe") {
		t.Stripe = StripePublic{}
	}
	if !paypalOK {
		t.PayPal = PayPalPublic{}
	}
	return t
}

// cards is m, or an empty map, so the JSON says `{}` rather than `null`.
func cards(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return m
}

// OrgJSON returns a zip.Handler for GET /v1/commerce/org. The
// handler:
//  1. Extracts and normalizes the Host header.
//  2. Resolves to a Org (or 404 with no Host echo on failure).
//  3. Applies the payment rail policy for the caller's country (Square's card
//     form wherever Square takes the currency there, Stripe's where it does
//     not; PayPal wherever PayPal operates).
//  4. Projects through toPublicView and JSON-encodes.
//
// Cache policy: private 60s browser cache (never shared CDN cache, because
// Cloudflare keys shared cache on URL alone while this response varies by
// CF-IPCountry, Host, X-Forwarded-Host, and X-Publishable-Key).
func OrgJSON(r Resolver) zip.Handler {
	return func(c *zip.Ctx) error {
		t, err := r.Resolve(RequestHost(c))
		if err != nil {
			// Do NOT include the Host in the 404 body. Attackers probing
			// for org existence should see a constant response.
			c.SetHeader("Cache-Control", "no-store")
			return c.Bytes(http.StatusNotFound, []byte(`{"error":"unknown org"}`))
		}
		t = applyCountryPolicy(t, RequestCountry(c))
		c.SetHeader("Content-Type", "application/json; charset=utf-8")
		c.SetHeader("Vary", "Host, X-Forwarded-Host, X-Publishable-Key, CF-IPCountry, CloudFront-Viewer-Country, X-Vercel-IP-Country, X-Country-Code")
		if strings.TrimSpace(c.Query("country")) != "" {
			c.SetHeader("Cache-Control", "no-store")
		} else {
			c.SetHeader("Cache-Control", "private, max-age=60")
		}
		b, _ := json.Marshal(toPublicView(t))
		return c.Bytes(http.StatusOK, b)
	}
}
