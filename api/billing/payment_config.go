package billing

import (
	"context"
	"errors"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/payment"
	jsonhttp "github.com/hanzoai/commerce/util/json/http"
)

// PaymentConfig is the PUBLIC payment configuration a browser needs to tokenize a
// card for an org: which provider, which application and location it tokenizes
// against, which environment that account lives in, and whether the money is
// real. Every field is safe to hand a client.
type PaymentConfig struct {
	Provider      string `json:"provider"`
	ApplicationId string `json:"applicationId"`
	LocationId    string `json:"locationId"`
	Environment   string `json:"environment"`
	Live          bool   `json:"live"`
}

// ReadPaymentConfig resolves an org's public payment configuration.
//
// It takes the org as a value rather than reading it off a request because the
// browser is not the only asker: the public host→org tenant projection asks it,
// and a peer asks it over the internal plane. Two derivations of "which Square
// account is this" is a card tokenized against an account we cannot charge.
//
// Sandbox-vs-production comes from the SAME single authority as the charge path
// (org.TestMode / SQUARE_ENVIRONMENT) with the same KMS-then-env fallback, so the
// app id the browser tokenizes with always matches the env and access token
// commerce will vault and charge with.
//
// A nil org is not a failure — it is an org with no credentials of its own, which
// resolves to the deployment's public config. Nothing here can fail, so the
// answer is a value and no endpoint has an error to invent a status for.
func ReadPaymentConfig(ctx context.Context, org *organization.Organization) *PaymentConfig {
	sq := payment.SquarePublicConfig(org)
	return &PaymentConfig{
		Provider:      "square",
		ApplicationId: sq.ApplicationID,
		LocationId:    sq.LocationID,
		Environment:   sq.Environment,
		Live:          sq.Live,
	}
}

// GetPaymentConfig returns the PUBLIC Square config (application id, location id,
// environment) the browser's Web Payments SDK must use to tokenize a card for
// THIS org. All values are public (safe to expose to the client).
//
//	GET /v1/billing/settings
func GetPaymentConfig(c *zip.Ctx) error {
	// #146 class: resolve the org nil-safely. On the co-resident embed path there may be
	// no "organization" local; ReadPaymentConfig handles a nil org (env-fallback public
	// config), so a missing org yields the honest public config — never a nil-deref panic (502).
	org, _ := middleware.GetOrganizationOK(c)

	// NOTE: do NOT KMS-hydrate here. payment-config is called on dialog-open and
	// gates the card field mount, so it must be fast; a KMS round-trip can hang
	// when KMS is slow/unavailable and freeze the UI. Cloud orgs have no per-org
	// Square creds anyway — the env fallback supplies the public app id.
	return c.JSON(200, ReadPaymentConfig(c.Context(), org))
}

type testModeRequest struct {
	TestMode bool `json:"testMode"`
}

// Mode is which money an org is transacting: the org it names, and whether its
// transactions are test or live.
type Mode struct {
	OrgId    string `json:"orgId"`
	OrgName  string `json:"orgName"`
	Live     bool   `json:"live"`
	TestMode bool   `json:"testMode"`
}

// SetTestMode moves an org between test and live money and answers with the mode
// it now holds.
//
// It takes the org and the flag as values rather than reading them off a request
// because the mode is read back by everything that charges — a peer on the
// internal plane asks the same question — and a mode set one way and read another
// is how a sandbox charge lands on a live customer.
//
// WHO may move it is not decided here: the endpoint sits behind the money-mint bar,
// because a user who could put their own org in sandbox could dodge real charges.
func SetTestMode(ctx context.Context, org *organization.Organization, testMode bool) (*Mode, error) {
	if org == nil {
		return nil, errors.New("billing mode: no organization")
	}
	org.Live = !testMode
	if err := org.Update(); err != nil {
		return nil, err
	}
	return &Mode{OrgId: org.Id(), OrgName: org.Name, Live: org.Live, TestMode: !org.Live}, nil
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

	mode, err := SetTestMode(c.Context(), org, req.TestMode)
	if err != nil {
		log.Error("Failed to set test mode for org %q: %v", org.Name, err, c)
		return jsonhttp.Fail(c, 500, "failed to set test mode", err)
	}

	return c.JSON(200, mode)
}
