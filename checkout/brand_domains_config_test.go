package checkout

import (
	"os"
	"testing"
)

// Adding a paying host used to cost a module tag, a dependency bump, an image
// build and a pin — to add one line to a compiled table. While that ran, the host
// answered with the fallback brand: somebody else's name and mark on a checkout.
//
// The env cannot be re-read (sync.OnceValue, deliberately — this runs on a public
// unauthenticated path), so this exercises the parse directly.
func TestConfiguredDomainsParse(t *testing.T) {
	t.Setenv("COMMERCE_BRAND_DOMAINS", "  example.coop=zoo, .lux.example=LUX  bogus.test=nosuchbrand  malformed  ")

	got := parseBrandDomains(os.Getenv("COMMERCE_BRAND_DOMAINS"))

	want := map[string]string{"example.coop": "zoo", "lux.example": "lux"}
	if len(got) != len(want) {
		t.Fatalf("parsed %d entries, want %d: %+v", len(got), len(want), got)
	}
	for _, e := range got {
		if want[e.domain] != e.brand.slug {
			t.Errorf("%q resolved to %q, want %q", e.domain, e.brand.slug, want[e.domain])
		}
	}
}

// An unknown brand is skipped, never minted: naming a brand that does not exist
// must not create one, or a typo becomes a tenant.
func TestUnknownBrandIsSkippedNotMinted(t *testing.T) {
	for _, in := range []string{"a.test=ghost", "b.test=", "=lux", "no-equals"} {
		if got := parseBrandDomains(in); len(got) != 0 {
			t.Errorf("%q parsed to %+v, want nothing", in, got)
		}
	}
}
