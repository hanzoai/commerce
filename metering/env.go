package metering

import (
	"os"
	"strings"
)

// Environment variables a product reads to wire the metering Client. These are
// the canonical names the operator injects (the token from a KMS-backed
// secret); products MUST NOT invent their own. See gateway/auth_middleware.go
// (DefaultAuthConfig) for the same names on the gateway side.
const (
	// EnvBaseURL is the commerce service base URL.
	// Default: http://commerce.hanzo.svc.cluster.local:8001
	EnvBaseURL = "COMMERCE_URL"

	// EnvToken is the commerce service token (admin-scoped S2S). The operator
	// wires this from a KMS-backed secret; it is NEVER stored in plaintext in
	// the repo or image.
	EnvToken = "COMMERCE_SERVICE_TOKEN"

	// EnvOrg is the default tenant org slug (X-Org-Id) for S2S calls when a
	// request carries no org. Default: hanzo.
	EnvOrg = "COMMERCE_SERVICE_ORG"
)

// DefaultBaseURL is the in-cluster commerce address. Matches the gateway's
// AUTH_BILLING_URL default so both gate on the same balance source.
const DefaultBaseURL = "http://commerce.hanzo.svc.cluster.local:8001"

// ConfigFromEnv builds a Config from the canonical environment variables,
// applying the in-cluster commerce default. No variable turns metering off,
// opens it on error, or sends it to a sandbox ledger: there is no such switch.
func ConfigFromEnv() Config {
	base := strings.TrimSpace(os.Getenv(EnvBaseURL))
	if base == "" {
		base = DefaultBaseURL
	}
	org := strings.TrimSpace(os.Getenv(EnvOrg))
	if org == "" {
		org = "hanzo"
	}

	return Config{
		BaseURL: base,
		Token:   strings.TrimSpace(os.Getenv(EnvToken)),
		Org:     org,
	}
}

// FromEnv builds a Client from the canonical environment variables. This is the
// one-liner products use at startup:
//
//	meter, _ := metering.FromEnv()
//	mux.Use(meter.Middleware(metering.MiddlewareConfig{Provider: "search", Price: priceSearch}))
func FromEnv() (*Client, error) {
	return New(ConfigFromEnv())
}
