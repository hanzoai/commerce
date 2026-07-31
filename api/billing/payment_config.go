package billing

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/payment"
	jsonhttp "github.com/hanzoai/commerce/util/json/http"
)

// GetPaymentConfig returns the PUBLIC Square config (application id, location id,
// environment) the browser's Web Payments SDK must use to tokenize a card for
// THIS org. It resolves sandbox-vs-production through the SAME single authority
// as the charge path (org.TestMode / SQUARE_ENVIRONMENT) and the same KMS-then-env
// fallback, so the app id the browser tokenizes with always matches the env +
// access token commerce will vault/charge with. All values are public (safe to
// expose to the client).
//
//	GET /v1/billing/settings
func GetPaymentConfig(c *zip.Ctx) error {
	// #146 class: resolve the org nil-safely. On the co-resident embed path there may be
	// no "organization" local; SquarePublicConfig handles a nil org (env-fallback public
	// config), so a missing org yields the honest public config — never a nil-deref panic (502).
	org, _ := middleware.GetOrganizationOK(c)

	// NOTE: do NOT KMS-hydrate here. payment-config is called on dialog-open and
	// gates the card field mount, so it must be fast; a KMS round-trip can hang
	// when KMS is slow/unavailable and freeze the UI. Cloud orgs have no per-org
	// Square creds anyway — the env fallback supplies the public app id.
	//
	// Resolution lives in ONE place (payment.SquarePublicConfig) so this
	// authenticated handler and the public host→org tenant projection
	// (/v1/commerce/tenant) can never hand the browser a different Square app.
	sq := payment.SquarePublicConfig(org)

	return c.JSON(200, map[string]any{
		"provider":      "square",
		"applicationId": sq.ApplicationID,
		"locationId":    sq.LocationID,
		"environment":   sq.Environment,
		"live":          sq.Live,
	})
}

type testModeRequest struct {
	TestMode bool `json:"testMode"`
}

// SetOrgTestMode toggles the org's live flag (org.Live) and its test-mode view.
// org.Live marks transactions Test=true and is the FALLBACK Square-environment
// signal: when the deployment does NOT set SQUARE_ENVIRONMENT, a test org uses
// Square sandbox and a live org uses production (via org.TestMode).
// When the deployment DOES set SQUARE_ENVIRONMENT (the per-env authority:
// mainnet=production, testnet/devnet=sandbox), that env governs which Square
// environment is charged regardless of this flag — on mainnet there is no
// sandbox charge. Admin-only — a user must not be able to move their own org to
// sandbox to dodge real charges.
//
//	POST /v1/billing/mode   { testMode: bool }
func SetOrgTestMode(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)

	var req testModeRequest
	if err := c.Bind(&req); err != nil {
		return jsonhttp.Fail(c, 400, "invalid request body", err)
	}

	org.Live = !req.TestMode
	if err := org.Update(); err != nil {
		log.Error("Failed to set test mode for org %q: %v", org.Name, err, c)
		return jsonhttp.Fail(c, 500, "failed to set test mode", err)
	}

	return c.JSON(200, map[string]any{
		"orgId":    org.Id(),
		"orgName":  org.Name,
		"live":     org.Live,
		"testMode": !org.Live,
	})
}
