package kms

import "testing"

// TestSeedWritesWhereHydrateReads pins the one thing that made a stored Square
// credential unreachable: seeding wrote "/orgs/<org>/square" while hydrating
// read "/tenants/<org>/square". Both sides key the same store by (path, name,
// env), so an address only one of them spells is a read that can never hit —
// the org looked configured, every charge said the merchant was not, and
// nothing in between said a word.
func TestSeedWritesWhereHydrateReads(t *testing.T) {
	const org = "hanzo"

	var square string
	for _, m := range mappings(org) {
		if m.name == "SQUARE_SANDBOX_ACCESS_TOKEN" {
			square = m.path
		}
	}
	if square == "" {
		t.Fatal("hydrate no longer maps a Square access token")
	}
	if got := Path(org, "square"); got != square {
		t.Fatalf("seed writes %q, hydrate reads %q", got, square)
	}
}

// TestPathNamesTheTenant holds the address itself. The client scopes every call
// to its own org, so the tenant being charged is what the path has to carry:
// one vault holds many merchants, and they are told apart here or nowhere.
func TestPathNamesTheTenant(t *testing.T) {
	if got, want := Path("acme", "stripe"), "/tenants/acme/stripe"; got != want {
		t.Fatalf("Path = %q, want %q", got, want)
	}
	if Path("acme", "square") == Path("other", "square") {
		t.Fatal("two tenants share one credential address")
	}
}

// TestEveryProviderHasItsOwnPath keeps providers from collapsing onto one
// address, which would let a Stripe rotation overwrite a Square token.
func TestEveryProviderHasItsOwnPath(t *testing.T) {
	seen := map[string]string{}
	for _, m := range mappings("hanzo") {
		if prev, ok := seen[m.name]; ok && prev != m.path {
			t.Fatalf("%s is read from both %q and %q", m.name, prev, m.path)
		}
		seen[m.name] = m.path
	}
	if len(seen) == 0 {
		t.Fatal("no credentials are mapped at all")
	}
}
