package payment

import (
	"context"
	"os"
	"testing"

	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
	square "github.com/hanzoai/commerce/thirdparty/square"
)

// ensureDefaultProcessorPolicy guarantees COMMERCE_DISABLED_PROCESSORS is UNSET
// so the default (Stripe disabled) applies. t.Setenv cannot unset.
func ensureDefaultProcessorPolicy(t *testing.T) {
	t.Helper()
	if v, ok := os.LookupEnv("COMMERCE_DISABLED_PROCESSORS"); ok {
		os.Unsetenv("COMMERCE_DISABLED_PROCESSORS")
		t.Cleanup(func() { _ = os.Setenv("COMMERCE_DISABLED_PROCESSORS", v) })
	}
}

func orgWithSquareCreds() *organization.Organization {
	org := &organization.Organization{}
	org.Square.Production.AccessToken = "prod-access"
	org.Square.Production.LocationId = "prod-LOC"
	org.Square.Production.ApplicationId = "prod-APP"
	org.Square.Sandbox.AccessToken = "sandbox-access"
	org.Square.Sandbox.LocationId = "sandbox-LOC"
	org.Square.Sandbox.ApplicationId = "sandbox-APP"
	return org
}

func squareProcFromReg(t *testing.T, reg *processor.Registry) *square.SquareProcessor {
	t.Helper()
	p, err := reg.Get(processor.Square)
	if err != nil {
		t.Fatalf("Square not registered/selectable: %v", err)
	}
	sp, ok := p.(*square.SquareProcessor)
	if !ok {
		t.Fatalf("registered Square is %T, want *square.SquareProcessor", p)
	}
	return sp
}

// A LIVE org selects production creds + the production base URL. The deployment
// env is set to the OPPOSITE on purpose: it must not participate.
func TestProcessorsForOrg_SquareSplit_LiveOrgIsProduction(t *testing.T) {
	ensureDefaultProcessorPolicy(t)
	t.Setenv("SQUARE_ENVIRONMENT", "sandbox") // hostile: the old authority says sandbox

	org := orgWithSquareCreds()
	org.Live = true

	sp := squareProcFromReg(t, ProcessorsForOrg(org))
	if sp.Environment() != "production" {
		t.Fatalf("Environment = %q, want production", sp.Environment())
	}
	if sp.LocationID() != "prod-LOC" {
		t.Fatalf("LocationID = %q, want prod-LOC (production creds)", sp.LocationID())
	}
}

// A TEST-mode org selects sandbox creds + the sandbox base URL, on the very same
// deployment that just served the live org above — that is the multi-tenancy
// property. The env again says the opposite and is again ignored.
func TestProcessorsForOrg_SquareSplit_TestOrgIsSandbox(t *testing.T) {
	ensureDefaultProcessorPolicy(t)
	t.Setenv("SQUARE_ENVIRONMENT", "production") // hostile: the old authority says production

	org := orgWithSquareCreds()
	org.Live = false

	sp := squareProcFromReg(t, ProcessorsForOrg(org))
	if sp.Environment() != "sandbox" {
		t.Fatalf("Environment = %q, want sandbox", sp.Environment())
	}
	if sp.LocationID() != "sandbox-LOC" {
		t.Fatalf("LocationID = %q, want sandbox-LOC (sandbox creds)", sp.LocationID())
	}
}

// The env-var credential fallback (an org with no KMS creds) picks its credential
// set from the ORG's mode, not from SQUARE_ENVIRONMENT. NOTE: this fallback lets a
// tenant transact on the DEPLOYMENT's payment account, which api/checkout refuses
// by design ("per-tenant Square credentials can NEVER borrow the deployment's
// env-configured account"). Retiring it is the next step; this pins that while it
// exists it at least follows the per-org mode.
func TestProcessorsForOrg_SquareEnvVarFallback_FollowsOrgMode(t *testing.T) {
	ensureDefaultProcessorPolicy(t)
	t.Setenv("SQUARE_ENVIRONMENT", "production") // hostile: ignored
	t.Setenv("SQUARE_ACCESS_TOKEN", "prod-env-token")
	t.Setenv("SQUARE_LOCATION_ID", "prod-env-loc")
	t.Setenv("SQUARE_SANDBOX_ACCESS_TOKEN", "sandbox-env-token")
	t.Setenv("SQUARE_SANDBOX_LOCATION_ID", "sandbox-env-loc")

	org := &organization.Organization{} // no KMS creds → env fallback
	org.Live = false                    // test mode ⇒ sandbox credential set

	sp := squareProcFromReg(t, ProcessorsForOrg(org))
	if sp.Environment() != "sandbox" {
		t.Fatalf("Environment = %q, want sandbox", sp.Environment())
	}
	if sp.LocationID() != "sandbox-env-loc" {
		t.Fatalf("LocationID = %q, want sandbox-env-loc (sandbox env vars chosen)", sp.LocationID())
	}
}

// A hydrated Stripe token leaves Stripe REGISTERED (inert, for historical data)
// but a USD charge selects Square and Stripe is never selectable directly.
func TestProcessorsForOrg_StripeHydratedButSquareSelected(t *testing.T) {
	ensureDefaultProcessorPolicy(t)
	t.Setenv("SQUARE_ENVIRONMENT", "production")

	org := orgWithSquareCreds()
	org.Live = true
	org.Stripe.Live.AccessToken = "sk_live_HYDRATED" // org HAS a Stripe token

	reg := ProcessorsForOrg(org)

	// Stripe IS registered (inert historical provider) ...
	registered := false
	for _, tp := range reg.ListTypes() {
		if tp == processor.Stripe {
			registered = true
		}
	}
	if !registered {
		t.Fatal("Stripe should be REGISTERED (inert) when the org has a Stripe token")
	}
	// ... but it is disabled for direct Get ...
	if _, err := reg.Get(processor.Stripe); err == nil {
		t.Fatal("Get(Stripe) must error (disabled) under the default policy")
	}
	// ... and a USD charge selects Square, never Stripe.
	proc, err := reg.SelectProcessor(context.Background(), processor.PaymentRequest{Amount: 500, Currency: currency.USD})
	if err != nil {
		t.Fatalf("SelectProcessor(USD): %v", err)
	}
	if proc.Type() != processor.Square {
		t.Fatalf("USD selected %q; want square (Stripe hydrated but must not be selected)", proc.Type())
	}
}
