package payment

import (
	"os"
	"strings"

	"github.com/hanzoai/commerce/models/organization"
)

// SquareUseSandbox resolves whether the given org should charge against Square's
// SANDBOX environment (and use sandbox credentials) rather than production.
//
// SQUARE_ENVIRONMENT is the deployment-level authority and the SINGLE source of
// truth for the 3-env split: mainnet sets SQUARE_ENVIRONMENT=production,
// testnet/devnet set SQUARE_ENVIRONMENT=sandbox. The split is therefore pure
// per-env config, not per-org code forks. When SQUARE_ENVIRONMENT is unset or
// holds an unrecognized value, the org's live flag decides (live→production,
// test→sandbox) so local and un-templated deploys still resolve deterministically.
//
// Every Square call site — direct charge (ProcessorsForOrg), hosted checkout
// (api/checkout), the public Web-Payments config (api/billing/payment-config)
// and authorize/refund/capture (api/checkout/square) — routes the sandbox-vs-
// production decision through THIS function, so the browser tokenizes against
// the SAME environment commerce charges, and a refund hits the SAME environment
// as its original charge. organization.SquareConfig(sandbox) stays a pure cred
// picker; the deployment env policy lives here, in one place.
func SquareUseSandbox(org *organization.Organization) bool {
	if v, ok := os.LookupEnv("SQUARE_ENVIRONMENT"); ok {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "production", "prod", "live":
			return false
		case "sandbox", "test":
			return true
		}
		// Unrecognized value: fall through to the org's live flag rather than
		// guessing — a typo must not silently flip the money environment.
	}
	return !org.Live
}

// SquareEnvironment returns "sandbox" or "production" for the org per
// SquareUseSandbox — the value the Square SDK uses to select its API base URL
// (connect.squareupsandbox.com vs connect.squareup.com).
func SquareEnvironment(org *organization.Organization) string {
	if SquareUseSandbox(org) {
		return "sandbox"
	}
	return "production"
}
