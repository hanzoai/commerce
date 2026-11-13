package payment

import (
	"context"
	"testing"

	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/payment/providers/square"
)

// ensureDefaultProcessorPolicy lets every processor take a new charge for the
// test (COMMERCE_CHARGE_PROCESSORS=*); the deployment default is Stripe alone.
func ensureDefaultProcessorPolicy(t *testing.T) {
	t.Helper()
	t.Setenv("COMMERCE_CHARGE_PROCESSORS", "*")
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

func squareProcFromReg(t *testing.T, reg *processor.Registry) *square.Provider {
	t.Helper()
	p, err := reg.Get(processor.Square)
	if err != nil {
		t.Fatalf("Square not registered/selectable: %v", err)
	}
	sp, ok := p.(*square.Provider)
	if !ok {
		t.Fatalf("registered Square is %T, want *square.Provider", p)
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

// A hydrated Stripe token registers Stripe and leaves Square the default: a USD
// charge selects Square, a currency Square does not take selects Stripe, and a
// caller naming Stripe reaches it.
func TestProcessorsForOrg_StripeHydratedSquareStillDefault(t *testing.T) {
	ensureDefaultProcessorPolicy(t)
	t.Setenv("SQUARE_ENVIRONMENT", "production")

	org := orgWithSquareCreds()
	org.Live = true
	org.Stripe.Live.AccessToken = "sk_live_HYDRATED" // org HAS a Stripe token

	reg := ProcessorsForOrg(org)

	if _, err := reg.Get(processor.Stripe); err != nil {
		t.Fatalf("Get(Stripe) with a hydrated Stripe token: %v", err)
	}
	proc, err := reg.SelectProcessor(context.Background(), processor.PaymentRequest{Amount: 500, Currency: currency.USD})
	if err != nil {
		t.Fatalf("SelectProcessor(USD): %v", err)
	}
	if proc.Type() != processor.Square {
		t.Fatalf("USD selected %q; want square (Square takes every payment it can)", proc.Type())
	}
	proc, err = reg.SelectProcessor(context.Background(), processor.PaymentRequest{Amount: 500, Currency: currency.CHF})
	if err != nil {
		t.Fatalf("SelectProcessor(CHF): %v", err)
	}
	if proc.Type() != processor.Stripe {
		t.Fatalf("CHF selected %q; want stripe (Square does not take CHF)", proc.Type())
	}
}

// With no Stripe keys of its own, an org charges the deployment's Stripe
// account of its own kind — and never a key of the other kind.
func TestStripeKeys_EnvFallbackByKind(t *testing.T) {
	t.Setenv("STRIPE_SECRET_KEY", "sk_live_DEPLOY")
	t.Setenv("STRIPE_PUBLISHABLE_KEY", "pk_live_DEPLOY")
	t.Setenv("STRIPE_TEST_SECRET_KEY", "sk_test_DEPLOY")
	t.Setenv("STRIPE_TEST_PUBLISHABLE_KEY", "pk_test_DEPLOY")

	live := &organization.Organization{Live: true}
	if k := StripeKeys(live); k.SecretKey != "sk_live_DEPLOY" || k.PublishableKey != "pk_live_DEPLOY" {
		t.Fatalf("live org keys = %+v; want the deployment's live keys", k)
	}
	test := &organization.Organization{}
	if k := StripeKeys(test); k.SecretKey != "sk_test_DEPLOY" || k.PublishableKey != "pk_test_DEPLOY" {
		t.Fatalf("test org keys = %+v; want the deployment's test keys", k)
	}
	if pub := StripePublicConfig(test); pub.PublishableKey != "pk_test_DEPLOY" || pub.Environment != "test" || pub.Live {
		t.Fatalf("test org public = %+v", pub)
	}

	// A secret key misfiled as the publishable key is never published.
	t.Setenv("STRIPE_TEST_PUBLISHABLE_KEY", "sk_test_MISFILED")
	if k := StripeKeys(test); k.PublishableKey != "" {
		t.Fatalf("a secret key reached the publishable slot: %+v", k)
	}
	t.Setenv("STRIPE_TEST_PUBLISHABLE_KEY", "pk_test_DEPLOY")

	// A key of the wrong kind under a name is dropped, never used.
	t.Setenv("STRIPE_SECRET_KEY", "sk_test_MISFILED")
	if k := StripeKeys(live); k.SecretKey != "" {
		t.Fatalf("live org took a test key: %+v", k)
	}
	if reg := ProcessorsForOrg(live); func() bool { _, err := reg.Get(processor.Stripe); return err == nil }() {
		t.Fatal("Stripe registered for a live org from a test key")
	}

	// The org's own keys win over the deployment's.
	own := &organization.Organization{Live: true}
	own.Stripe.Live.AccessToken = "sk_live_OWN"
	own.Stripe.Live.PublishableKey = "pk_live_OWN"
	if k := StripeKeys(own); k.SecretKey != "sk_live_OWN" || k.PublishableKey != "pk_live_OWN" || !StripeOwned(own) {
		t.Fatalf("own keys = %+v owned=%v", k, StripeOwned(own))
	}
}

// An org's OWN Stripe keys are held to the deployment's rule: a secret key of the
// org's mode, a publishable key of that mode. A key of the other mode, or of the
// wrong kind, is no key at all.
func TestStripeKeys_OwnKeysAreCheckedByMode(t *testing.T) {
	t.Setenv("STRIPE_SECRET_KEY", "")
	t.Setenv("STRIPE_TEST_SECRET_KEY", "")
	t.Setenv("STRIPE_PUBLISHABLE_KEY", "")
	t.Setenv("STRIPE_TEST_PUBLISHABLE_KEY", "")
	cases := []struct {
		name           string
		live           bool
		sk, pk         string
		wantSK, wantPK string
	}{
		{"live org, live keys", true, "sk_live_a", "pk_live_a", "sk_live_a", "pk_live_a"},
		{"restricted key", true, "rk_live_a", "pk_live_a", "rk_live_a", "pk_live_a"},
		{"live org, a test secret key", true, "sk_test_a", "pk_live_a", "", ""},
		{"test org, a live secret key", false, "sk_live_a", "pk_test_a", "", ""},
		{"a publishable key in the secret slot", true, "pk_live_a", "pk_live_a", "", ""},
		{"a secret key in the publishable slot", true, "sk_live_a", "sk_live_b", "sk_live_a", ""},
		{"live org, a test publishable key", true, "sk_live_a", "pk_test_a", "sk_live_a", ""},
	}
	for _, c := range cases {
		org := &organization.Organization{Live: c.live}
		if c.live {
			org.Stripe.Live.AccessToken, org.Stripe.Live.PublishableKey = c.sk, c.pk
		} else {
			org.Stripe.Test.AccessToken, org.Stripe.Test.PublishableKey = c.sk, c.pk
		}
		k := StripeKeys(org)
		if c.live && organization.Ceiling != organization.Live {
			continue
		}
		if k.SecretKey != c.wantSK || k.PublishableKey != c.wantPK || OwnStripeSecret(org) != c.wantSK {
			t.Errorf("%s: keys = %+v own = %q; want %q / %q", c.name, k, OwnStripeSecret(org), c.wantSK, c.wantPK)
		}
	}
}
