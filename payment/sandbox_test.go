package payment

import (
	"testing"

	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/payment/processor"
)

// hostileProduction sets the deployment's production Square account, which no
// sandbox path may read.
func hostileProduction(t *testing.T) {
	t.Helper()
	t.Setenv("SQUARE_ENVIRONMENT", "production")
	t.Setenv("SQUARE_ACCESS_TOKEN", "EAAA-prod-hostile")
	t.Setenv("SQUARE_LOCATION_ID", "PRODLOC-hostile")
	t.Setenv("SQUARE_APPLICATION_ID", "sq0idp-hostile")
	t.Setenv("SQUARE_SANDBOX_ACCESS_TOKEN", "")
	t.Setenv("SQUARE_SANDBOX_LOCATION_ID", "")
	t.Setenv("SQUARE_SANDBOX_APPLICATION_ID", "")
}

// ceiling caps every org at k for one test.
func ceiling(t *testing.T, k organization.Kind) {
	t.Helper()
	prev := organization.Ceiling
	organization.Ceiling = k
	t.Cleanup(func() { organization.Ceiling = prev })
}

// A test-mode org on a deployment with no sandbox account publishes no card,
// never the production application.
func TestSquarePublicConfig_TestOrgNeverShowsProductionApp(t *testing.T) {
	hostileProduction(t)

	got := SquarePublicConfig(&organization.Organization{})
	if got.ApplicationID != "" || got.LocationID != "" {
		t.Fatalf("test-mode org shown %q/%q, want no application (production's is not its own)", got.ApplicationID, got.LocationID)
	}
	if got.Environment != "sandbox" || got.Live {
		t.Fatalf("environment = %q live=%v, want sandbox", got.Environment, got.Live)
	}
}

// A test-mode org with no Square credentials of its own, on a deployment with
// no sandbox account, gets no Square processor: the production account is not
// lent to it.
func TestProcessorsForOrg_SandboxOrgNeverBorrowsProductionAccount(t *testing.T) {
	ensureDefaultProcessorPolicy(t)
	hostileProduction(t)

	if _, err := ProcessorsForOrg(&organization.Organization{}).Get(processor.Square); err == nil {
		t.Fatal("a sandbox org was given a Square processor built from production names")
	}

	// A sandbox token with no sandbox location is not completed by production's.
	t.Setenv("SQUARE_SANDBOX_ACCESS_TOKEN", "EAAA-sandbox")
	if _, err := ProcessorsForOrg(&organization.Organization{}).Get(processor.Square); err == nil {
		t.Fatal("a sandbox token was paired with the production location")
	}
}

// Under a Test ceiling an org whose record says Live transacts in sandbox: the
// sandbox credential set, the sandbox host, the test books.
func TestCeiling_LiveOrgChargesSandbox(t *testing.T) {
	ensureDefaultProcessorPolicy(t)
	hostileProduction(t)
	ceiling(t, organization.Test)

	org := orgWithSquareCreds()
	org.Live = true

	sp := squareProcFromReg(t, ProcessorsForOrg(org))
	if sp.Environment() != "sandbox" || sp.LocationID() != "sandbox-LOC" {
		t.Fatalf("capped live org charges %q at %q, want sandbox at sandbox-LOC", sp.Environment(), sp.LocationID())
	}
	pub := SquarePublicConfig(org)
	if pub.ApplicationID != "sandbox-APP" || pub.Environment != "sandbox" || pub.Live {
		t.Fatalf("capped live org publishes %+v, want the sandbox application, not live", pub)
	}
}

// With no ceiling declared the record decides, as before.
func TestCeiling_DefaultLeavesLiveOrgLive(t *testing.T) {
	ensureDefaultProcessorPolicy(t)
	hostileProduction(t)

	org := orgWithSquareCreds()
	org.Live = true

	sp := squareProcFromReg(t, ProcessorsForOrg(org))
	if sp.Environment() != "production" || sp.LocationID() != "prod-LOC" {
		t.Fatalf("live org charges %q at %q, want production at prod-LOC", sp.Environment(), sp.LocationID())
	}
}
