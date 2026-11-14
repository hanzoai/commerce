package payment

import (
	"github.com/hanzoai/commerce/payment/processor"
	"testing"

	"github.com/hanzoai/commerce/models/organization"
)

// The env credential fallback (cloud orgs with no per-org Square creds) must
// surface the PRODUCTION app id + location for a LIVE org — the values the browser
// tokenizes against and commerce charges with. The org is the authority;
// SQUARE_ENVIRONMENT is set to the opposite here and must not participate.
func TestSquarePublicConfig_LiveOrgGetsProductionPair(t *testing.T) {
	// SQUARE_ENVIRONMENT: hostile: ignored
	t.Cleanup(processor.EnvSeed{"SQUARE_ENVIRONMENT": "sandbox", "SQUARE_APPLICATION_ID": "sq0idp-PROD", "SQUARE_LOCATION_ID": "PRODLOC"}.Apply())

	org := &organization.Organization{}
	org.Live = true
	got := SquarePublicConfig(org)
	if got.ApplicationID != "sq0idp-PROD" || got.LocationID != "PRODLOC" {
		t.Errorf("app/location = %q/%q, want sq0idp-PROD/PRODLOC", got.ApplicationID, got.LocationID)
	}
	if got.Environment != "production" {
		t.Errorf("environment = %q, want production", got.Environment)
	}
}

// The SAME per-org authority must select the SANDBOX public vars for a test-mode
// org, so the app id never straddles environments — on the same deployment that
// serves the live org above.
func TestSquarePublicConfig_TestOrgGetsSandboxPair(t *testing.T) {
	// SQUARE_ENVIRONMENT: hostile: ignored
	t.Cleanup(processor.EnvSeed{"SQUARE_ENVIRONMENT": "production", "SQUARE_APPLICATION_ID": "sq0idp-PROD", "SQUARE_LOCATION_ID": "PRODLOC", "SQUARE_SANDBOX_APPLICATION_ID": "sandbox-sq0idb-SBX", "SQUARE_SANDBOX_LOCATION_ID": "SBXLOC"}.Apply())

	got := SquarePublicConfig(&organization.Organization{})
	if got.ApplicationID != "sandbox-sq0idb-SBX" || got.LocationID != "SBXLOC" {
		t.Errorf("app/location = %q/%q, want sandbox pair", got.ApplicationID, got.LocationID)
	}
	if got.Environment != "sandbox" {
		t.Errorf("environment = %q, want sandbox", got.Environment)
	}
}

// Fail-closed, per tenant: an org that does not say Live gets sandbox, whatever
// the deployment env says — so a half-configured org can never silently charge
// live cards, and it cannot drag other tenants with it either.
func TestSquarePublicConfig_FailsClosedPerOrg(t *testing.T) {
	// SQUARE_ENVIRONMENT: hostile: ignored
	t.Cleanup(processor.EnvSeed{"SQUARE_ENVIRONMENT": "production"}.Apply())
	got := SquarePublicConfig(&organization.Organization{})
	if got.Environment != "sandbox" {
		t.Errorf("environment = %q, want sandbox (fail-closed)", got.Environment)
	}
}
