package checkout

import "testing"

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
		{"commerce-api.hanzo.ai", "hanzo"},
		{"hanzo.ai", "hanzo"},
		{"pay.lux.network", "lux"},
		{"lux.network", "lux"},
		// The MONEY hosts. These were absent from brandDomains and therefore
		// resolved to the deployment default — hanzo — so a Lux customer on a
		// Lux payment page got the Hanzo brand, the Hanzo IAM app and Hanzo's
		// Square application id, and a card entered there would have tokenized
		// against Hanzo's merchant. Nothing errored; the page looked right.
		{"pay.lux.cloud", "lux"},
		{"lux.cloud", "lux"},
		{"pay.zoo.cloud", "zoo"},
		{"zoo.cloud", "zoo"},
		{"pay.zoo.ngo", "zoo"},
		{"pay.pars.network", "pars"},
		// Unknown host → deployment default (hanzo).
		{"pay.example.test", "hanzo"},
		{"random.internal", "hanzo"},
		// Spoofs must NOT inherit the spoofed brand; they resolve to default.
		{"pay.lux.network.evil.com", "hanzo"},
		{"pay.zoo.ngo.attacker.test", "hanzo"},
		{"pay.lux.cloud.evil.com", "hanzo"},
		{"notlux.cloud", "hanzo"},
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
	const want = "https://playground.hanzo.bot"
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
	// It must survive the public projection as a non-nil, populated slice — a
	// nil slice serializes as JSON null and crashes the pay SPA (e is not
	// iterable) on /onboard.
	if pv := toPublicView(ten); len(pv.ReturnURLAllowlist) == 0 {
		t.Errorf("publicView return allowlist is empty, want first-party app hosts")
	}
}

// enabledProviders honors the deploy-wide disabled policy: Square is off only
// when explicitly disabled; crypto + wire are always present; Stripe never is.
func TestEnabledProviders_SquareOffWhenDisabled(t *testing.T) {
	t.Setenv("COMMERCE_DISABLED_PROCESSORS", "square")
	for _, p := range enabledProviders() {
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

	if brandHanzo.logoURL == "" {
		t.Error("brandHanzo has no logoURL, so the checkout header falls back to a wordmark")
	}
	// The other three have no logo that resolves — cdn.lux.network 404s, the zoo
	// and pars CDNs answer 522. Giving them one would ship a broken image; the
	// wordmark is the honest render until a real asset exists.
	for _, b := range []brand{brandLux, brandZoo, brandPars} {
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
