package payment

import (
	"github.com/hanzoai/commerce/models/organization"
	square "github.com/hanzoai/commerce/thirdparty/square"
)

// SquarePublic is the PUBLIC Square Web Payments config the browser needs to
// tokenize a card for an org: the application id + location id + environment.
// Every field is public (Square exposes them to the client SDK) — no secret,
// no access token, no webhook key.
type SquarePublic struct {
	ApplicationID string `json:"applicationId"`
	LocationID    string `json:"locationId"`
	Environment   string `json:"environment"` // "sandbox" | "production"
	Live          bool   `json:"live"`
}

// SquarePublicConfig resolves the PUBLIC Square config for an org through the
// SINGLE test-mode authority (org.TestMode / SQUARE_ENVIRONMENT) and the same
// KMS-then-env fallback the charge path uses, so the app id the browser
// tokenizes with always matches the environment + access token commerce will
// vault/charge with.
//
// This is the ONE place that resolution lives: both the authenticated
// /v1/billing/settings handler and the public host→org tenant projection
// (/v1/commerce/tenant) call it, so a card form and its charge can never drift
// onto different Square applications.
func SquarePublicConfig(org *organization.Organization) SquarePublic {
	// A nil org (an unauthenticated / no-org caller on the co-resident billing path) means
	// "no per-org Square creds" — resolve from the env fallback below rather than nil-deref.
	// A zero Organization is env-fallback-safe (TestMode reads env/!Live; Square is a value).
	if org == nil {
		org = &organization.Organization{}
	}
	useSandbox := org.TestMode()
	env := org.SquareEnvironment()

	sq := org.SquareConfig(useSandbox)
	appID := sq.ApplicationId
	locationID := sq.LocationId

	// Env fallback (KMS disabled or no per-org Square creds — the common case
	// for cloud orgs). The SAME useSandbox authority picks the sandbox vs
	// production public vars, so the app id always matches `env`; a sandbox
	// org is never shown the production application (square.FromEnv).
	if appID == "" || locationID == "" {
		creds := square.FromEnv(useSandbox)
		appID, locationID = creds.ApplicationID, creds.LocationID
	}

	return SquarePublic{
		ApplicationID: appID,
		LocationID:    locationID,
		Environment:   env,
		Live:          !org.TestMode(),
	}
}

// SquareOwned reports whether the org holds Square credentials of its own —
// the same test SquarePublicConfig makes before it falls back to the
// deployment's env credentials, which belong to the deployment's own brand.
func SquareOwned(org *organization.Organization) bool {
	if org == nil {
		return false
	}
	sq := org.SquareConfig(org.TestMode())
	return sq.ApplicationId != "" && sq.LocationId != ""
}
