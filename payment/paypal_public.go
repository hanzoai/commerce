package payment

import (
	"strings"

	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/payment/providers/paypal"
)

// PayPalPublic is the PUBLIC PayPal JS SDK configuration the browser needs to
// mount PayPal Smart Buttons for an org: the OAuth2 client ID and environment.
// Every field is public (PayPal exposes client-id in the browser SDK query
// string) — no client secret and no webhook ID.
type PayPalPublic struct {
	ClientID    string `json:"clientId"`
	Environment string `json:"environment"` // "sandbox" | "live"
	Live        bool   `json:"live"`
}

// PayPalPublicConfig resolves the PUBLIC PayPal config for an org through the
// SINGLE test-mode authority (org.TestMode) and the same KMS-then-env fallback
// the charge path uses, so the client ID the browser mounts PayPal Buttons with
// always matches the environment + client secret commerce will capture/authorize
// with.
func PayPalPublicConfig(org *organization.Organization) PayPalPublic {
	if org == nil {
		org = &organization.Organization{}
	}
	useSandbox := org.TestMode()
	env := org.PayPalEnvironment()

	pp := org.PayPalConfig(useSandbox)
	clientID := strings.TrimSpace(pp.ClientId)
	if clientID == "" {
		creds := paypal.Deployment(useSandbox)
		clientID = creds.ClientID
	}

	return PayPalPublic{
		ClientID:    clientID,
		Environment: env,
		Live:        !useSandbox,
	}
}

// PayPalOwned reports whether the org holds PayPal REST v2 credentials of its
// own — the same test PayPalPublicConfig makes before falling back to the
// deployment's env credentials, which belong to the deployment's own brand.
func PayPalOwned(org *organization.Organization) bool {
	if org == nil {
		return false
	}
	pp := org.PayPalConfig(org.TestMode())
	return strings.TrimSpace(pp.ClientId) != ""
}

// IsPayPalCountry reports whether PayPal operates in the given ISO 3166-1
// alpha-2 country/region code. PayPal operates in ~200 countries/regions; in
// the unsupported or OFAC-restricted jurisdictions listed below, checkout
// disables PayPal so customers fall back to a card form, Crypto (or Wire).
func IsPayPalCountry(code string) bool {
	c := strings.ToUpper(strings.TrimSpace(code))
	if c == "" {
		return true
	}
	switch c {
	case "AF", "BD", "BY", "CF", "CI", "CM", "CU", "ER", "GH", "HT",
		"IQ", "IR", "KP", "LB", "LR", "LY", "MM", "PK", "PS", "RU",
		"SD", "SS", "SY", "TL", "TR", "UZ", "YE", "ZW":
		return false
	default:
		return len(c) == 2
	}
}
