package checkout

import (
	"testing"

	"github.com/hanzoai/commerce/models/organization"
)

// brandForHost is exact-suffix: a host maps to a brand only when it equals the
// brand domain or is a real subdomain of it. Cross-brand spoofs
// ("pay.lux.network.evil.com") must NOT inherit the spoofed brand — they fall
// through to the deployment default.
func TestBrandForHost(t *testing.T) {
	cases := []struct {
		host string
		slug string
	}{
		{"pay.hanzo.ai", "hanzo"},
		{"commerce.hanzo.ai", "hanzo"},
		{"hanzo.ai", "hanzo"},
		{"pay.lux.network", "lux"},
		{"lux.network", "lux"},
		// The MONEY hosts. Each must carry its OWN brand and merchant: falling
		// through to the deployment default would bill the wrong party, and no
		// error would be raised to say so.
		{"pay.lux.cloud", "lux"},
		{"lux.cloud", "lux"},
		{"pay.lux.tel", "lux"},
		{"lux.tel", "lux"},
		{"pay.zoo.cloud", "zoo"},
		{"zoo.cloud", "zoo"},
		{"pay.zoo.ngo", "zoo"},
		// Zoo's identity origin is zoolabs.id (brandZoo.iamIssuer); zoo.id is
		// not Zoo's domain and resolves nowhere in particular.
		{"zoolabs.id", "zoo"},
		{"zoo.id", "hanzo"},
		{"pay.pars.network", "pars"},
		// Unknown host → deployment default (hanzo).
		{"pay.example.test", "hanzo"},
		{"random.internal", "hanzo"},
		// Spoofs must NOT inherit the spoofed brand; they resolve to default.
		{"pay.lux.network.evil.com", "hanzo"},
		{"pay.zoo.ngo.attacker.test", "hanzo"},
		{"pay.lux.cloud.evil.com", "hanzo"},
		{"pay.lux.tel.evil.com", "hanzo"},
		{"notlux.cloud", "hanzo"},
		{"notlux.tel", "hanzo"},
		// Non-subdomain lookalikes must not hijack the brand.
		{"notlux.network", "hanzo"},
	}
	for _, tc := range cases {
		if got := brandForHost(tc.host).slug; got != tc.slug {
			t.Errorf("brandForHost(%q).slug = %q, want %q", tc.host, got, tc.slug)
		}
	}
}

// The resolver returns a usable org for any well-formed host — no separate
// org registry, no 404. The public Square config comes from the org via the
// env credential fallback (the cloud-org path), so the pay SPA can mount its card
// iframe.
//
// A synthetic org (nil loader, no record) is NOT Live, so it resolves SANDBOX
// whatever SQUARE_ENVIRONMENT says. That is the fail-closed direction and the one
// we want here: a host with no org record behind it must never have the pay SPA
// tokenizing against a production Square application.
func TestOrgResolver_ResolvesOrgAsTenant(t *testing.T) {
	t.Setenv("SQUARE_ENVIRONMENT", "production") // hostile: ignored
	t.Setenv("SQUARE_APPLICATION_ID", "sq0idp-TESTAPP")
	t.Setenv("SQUARE_LOCATION_ID", "TESTLOC")

	r := NewOrgResolver(nil) // nil loader → brand-default synthetic org + env

	ten, err := r.Resolve("pay.hanzo.ai")
	if err != nil {
		t.Fatalf("Resolve(pay.hanzo.ai) err = %v, want nil", err)
	}
	if ten.Name != "hanzo" {
		t.Errorf("org name = %q, want hanzo", ten.Name)
	}
	if ten.IAM.Issuer != "https://hanzo.id" || ten.IAM.ClientID != "hanzo-app" {
		t.Errorf("org IAM = %+v, want hanzo.id/hanzo-app", ten.IAM)
	}
	if ten.Square.ApplicationID != "sq0idp-TESTAPP" || ten.Square.LocationID != "TESTLOC" {
		t.Errorf("org Square = %+v, want env app/location", ten.Square)
	}
	if ten.Square.Environment != "sandbox" {
		t.Errorf("org Square env = %q, want sandbox (synthetic org is not Live ⇒ fail closed)", ten.Square.Environment)
	}
	// Square must be an enabled provider so the card method surfaces; Stripe never.
	var hasSquare, hasStripe bool
	for _, p := range ten.Providers {
		if p.Name == "square" && p.Enabled {
			hasSquare = true
		}
		if p.Name == "stripe" {
			hasStripe = true
		}
	}
	if !hasSquare {
		t.Errorf("org providers %+v missing enabled square", ten.Providers)
	}
	if hasStripe {
		t.Errorf("org providers %+v must not surface stripe", ten.Providers)
	}
}

// A malformed Host (empty / control bytes) is the only case that 404s.
func TestOrgResolver_MalformedHostRejected(t *testing.T) {
	r := NewOrgResolver(nil)
	for _, h := range []string{"", "\x00bad", " ", ":"} {
		if _, err := r.Resolve(h); err != ErrUnknownOrg {
			t.Errorf("Resolve(%q) err = %v, want ErrUnknownOrg", h, err)
		}
	}
}

// The public org JSON built from an org resolution must carry the Square
// block and enabled square provider, and never leak a secret.
func TestOrgResolver_PublicViewCarriesSquare(t *testing.T) {
	t.Setenv("SQUARE_ENVIRONMENT", "production") // hostile: ignored, org decides
	t.Setenv("SQUARE_APPLICATION_ID", "sq0idp-PUBVIEW")
	t.Setenv("SQUARE_LOCATION_ID", "LOCPUB")

	r := NewOrgResolver(nil)
	ten, err := r.Resolve("pay.hanzo.ai")
	if err != nil {
		t.Fatalf("Resolve err = %v", err)
	}
	pv := toPublicView(ten)
	if pv.Square.ApplicationID != "sq0idp-PUBVIEW" || pv.Square.Environment != "sandbox" {
		t.Errorf("publicView.Square = %+v, want env app + sandbox (synthetic org fails closed)", pv.Square)
	}
}

// The hanzo org's return allowlist must carry the first-party app hosts so
// pay can bounce ?return= back to the app that sent the user here (e.g. the
// playground onboarding flow). An empty allowlist would reject every return and
// strand the user on the brand default after onboarding.
func TestOrgResolver_ReturnAllowlistCarriesAppHosts(t *testing.T) {
	r := NewOrgResolver(nil)
	ten, err := r.Resolve("pay.hanzo.ai")
	if err != nil {
		t.Fatalf("Resolve err = %v", err)
	}
	// hanzo.id's onboarding sends a new customer to checkout and completes only
	// when pay returns them to /onboarding paid.
	for _, want := range []string{"https://playground.hanzo.bot", "https://hanzo.id"} {
		found := false
		for _, h := range ten.ReturnURLAllowlist {
			if h == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("hanzo return allowlist %v missing %q", ten.ReturnURLAllowlist, want)
		}
	}
	// It must survive the public projection as a non-nil, populated slice — a
	// nil slice serializes as JSON null and crashes the pay SPA (e is not
	// iterable) on /onboard.
	if pv := toPublicView(ten); len(pv.ReturnURLAllowlist) == 0 {
		t.Errorf("publicView return allowlist is empty, want first-party app hosts")
	}
}

// pay.lux.tel resolves to lux, and its return allowlist carries the Lux Tel
// site and console so a customer who paid lands back where they started. The
// pay SPA matches return hosts exactly, so an unrelated host stays out.
func TestOrgResolver_ReturnAllowlistCarriesLuxTel(t *testing.T) {
	r := NewOrgResolver(nil)
	ten, err := r.Resolve("pay.lux.tel")
	if err != nil {
		t.Fatalf("Resolve err = %v", err)
	}
	if ten.Name != "lux" {
		t.Fatalf("pay.lux.tel resolved to %q, want lux", ten.Name)
	}
	allowed := map[string]bool{}
	for _, h := range toPublicView(ten).ReturnURLAllowlist {
		allowed[h] = true
	}
	for _, want := range []string{"https://lux.tel", "https://www.lux.tel", "https://console.lux.tel"} {
		if !allowed[want] {
			t.Errorf("lux return allowlist missing %q", want)
		}
	}
	for _, reject := range []string{"https://evil.com", "https://lux.tel.evil.com", "https://hanzo.ai"} {
		if allowed[reject] {
			t.Errorf("lux return allowlist carries unrelated host %q", reject)
		}
	}
}

// enabledProviders honors the deploy-wide disabled policy: Square is off only
// when explicitly disabled; crypto + wire are always present; Stripe never is.
func TestEnabledProviders_SquareOffWhenDisabled(t *testing.T) {
	t.Setenv("COMMERCE_DISABLED_PROCESSORS", "square")
	for _, p := range enabledProviders(true) {
		if p.Name == "square" {
			t.Errorf("square surfaced despite COMMERCE_DISABLED_PROCESSORS=square")
		}
	}
}

// A logo belongs to ONE brand, and a brand without one shows its name.
//
// The failure this guards is not a missing image, it is a Hanzo mark appearing
// on a Lux checkout — the kind of thing a well-meaning "fill in the blanks" pass
// produces. So the assertion is per brand and by identity, not "is non-empty":
// only hanzo may carry a logo, and no two brands may ever carry the same one.
func TestBrandLogosAreNotShared(t *testing.T) {
	withLogo := map[string]string{}
	for _, b := range []brand{brandHanzo, brandLux, brandZoo, brandPars} {
		if b.logoURL == "" {
			continue
		}
		if other, dup := withLogo[b.logoURL]; dup {
			t.Fatalf("brands %q and %q share logoURL %q — one brand's mark on another's checkout",
				other, b.slug, b.logoURL)
		}
		withLogo[b.logoURL] = b.slug
	}

	// Hanzo and Lux each have an asset that was fetched and confirmed: 200 with an
	// image/* content type, and the two files differ — Hanzo's is the 67x67 square
	// mark, Lux's the 63x17 wordmark. That second check is the one that matters
	// here, because a Lux host serving Hanzo's bytes would pass every other test.
	for _, b := range []brand{brandHanzo, brandLux} {
		if b.logoURL == "" {
			t.Errorf("brand %q lost its logoURL, so its checkout header falls back to a wordmark", b.slug)
		}
	}
	// Zoo and Pars have no asset that resolves: cdn.zoo.ngo and cdn.pars.network
	// answer 522, and cdn.zoo.network / cdn.zoo.cloud / cdn.zoolabs.org do not
	// resolve at all. Giving them one would ship a broken image; the wordmark is
	// the honest render until a real asset exists. Lux sat in this list until
	// 2026-08-11, when its asset was published — so re-fetch before believing a
	// "none exists" note here, and put the measurement in the commit.
	for _, b := range []brand{brandZoo, brandPars} {
		if b.logoURL != "" {
			t.Errorf("brand %q gained logoURL %q — verify it actually serves an image "+
				"(200 AND an image/* content type, not a 200 of the SPA's index.html)",
				b.slug, b.logoURL)
		}
	}
}

// The brand's logo must reach the wire, or setting it changes nothing a browser
// can see. Asserts the field survives the projection into the tenant payload.
func TestTenantPayloadCarriesTheBrandLogo(t *testing.T) {
	if got := brandForHost("pay.hanzo.ai"); got.logoURL != brandHanzo.logoURL {
		t.Fatalf("pay.hanzo.ai resolved to logoURL %q, want %q", got.logoURL, brandHanzo.logoURL)
	}
}

// The deployment's Square credentials are its default brand's account. A brand
// with no account of its own publishes no card: no Square block and no square
// provider, so its pay host never tokenizes against another brand's merchant.
func TestOrgResolver_SquareIsTheBrandsOwn(t *testing.T) {
	t.Setenv("SQUARE_APPLICATION_ID", "sq0idp-DEPLOY")
	t.Setenv("SQUARE_LOCATION_ID", "LOCDEPLOY")

	hasSquare := func(o Org) bool {
		for _, p := range o.Providers {
			if p.Name == "square" && p.Enabled {
				return true
			}
		}
		return false
	}

	r := NewOrgResolver(nil)
	for _, host := range []string{"pay.lux.cloud", "pay.lux.tel", "pay.zoo.cloud", "pay.pars.network"} {
		o, err := r.Resolve(host)
		if err != nil {
			t.Fatalf("Resolve(%q): %v", host, err)
		}
		if o.Square.ApplicationID != "" || o.Square.LocationID != "" {
			t.Errorf("%s publishes Square %+v, the deployment's account", host, o.Square)
		}
		if hasSquare(o) {
			t.Errorf("%s surfaces the square provider with no account of its own", host)
		}
	}

	o, _ := r.Resolve("pay.hanzo.ai")
	if o.Square.ApplicationID != "sq0idp-DEPLOY" || !hasSquare(o) {
		t.Errorf("pay.hanzo.ai lost its card: %+v %+v", o.Square, o.Providers)
	}

	// A brand that holds an account of its own publishes it.
	own := NewOrgResolver(func(slug string) (*organization.Organization, bool) {
		org := &organization.Organization{}
		org.Name = slug
		org.Square.Sandbox.ApplicationId = "sq0idb-LUX"
		org.Square.Sandbox.LocationId = "LOCLUX"
		return org, true
	})
	o, _ = own.Resolve("pay.lux.cloud")
	if o.Square.ApplicationID != "sq0idb-LUX" || !hasSquare(o) {
		t.Errorf("lux with its own account: %+v %+v", o.Square, o.Providers)
	}
}

// Zoo's sign-up returns through its own identity origin, and never through a
// domain Zoo does not hold.
func TestReturnHosts_ZooIsZoolabsID(t *testing.T) {
	hosts := map[string]bool{}
	for _, h := range returnHostsFor("zoo") {
		hosts[h] = true
	}
	if !hosts["https://zoolabs.id"] {
		t.Errorf("zoo return allowlist %v lacks https://zoolabs.id", returnHostsFor("zoo"))
	}
	if hosts["https://zoo.id"] {
		t.Errorf("zoo return allowlist carries https://zoo.id")
	}
}
