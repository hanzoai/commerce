// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package billing

import (
	"testing"

	"github.com/hanzoai/commerce/billing/tier"
)

// EVERY PLAN THE CATALOG SELLS MUST CONFER A PAID TIER.
//
// The registry holds tier names (free/starter/pro/enterprise); the catalog sells plan
// slugs (go/dev/pro/max/team/enterprise). `tier.Parse` knows only the first, so four
// of the six sold plans resolved to Free — one agent, two models. Measured live
// against the running pod before the fix:
//
//	?tier=go -> Free   ?tier=dev -> Free   ?tier=team -> Free   ?tier=max -> Free
//
// `max` is $99/mo. A customer on the most expensive plan got the most restrictive
// configuration, with no error anywhere to find it by.
//
// This drives off the CATALOG rather than a list written here, so a plan added
// tomorrow is covered without anyone remembering to update a test. That is the
// property worth holding: not "these four slugs work" but "nothing we sell parses to
// Free".
func TestEverySoldPlanConfersAPaidTier(t *testing.T) {
	plans := hanzoPlans
	if len(plans) == 0 {
		t.Fatal("no plans in the catalog — this guard would pass over nothing")
	}

	sold := 0
	for _, p := range plans {
		if !paidTier(p.Slug) {
			continue // genuinely free/$0 rows (dns-free) are not the subject
		}
		sold++
		got := tierOfName(p.Slug)
		if got == tier.Free {
			t.Errorf("plan %q (category=%q, price=%d) resolves to Free — a sold plan "+
				"must never confer the most restrictive tier", p.Slug, p.Category, p.Price)
		}
		if p.Category == "enterprise" || p.ContactSales {
			if got != tier.Enterprise {
				t.Errorf("plan %q is enterprise-category but resolves to %q", p.Slug, got)
			}
		}
	}
	if sold == 0 {
		t.Fatal("catalog has no paid plans — the guard covered nothing")
	}
}

// A tier NAME still resolves to itself: the catalog lookup is a fallback for slugs,
// not a replacement, so the registry keeps priority.
func TestTierNamesStillResolveToThemselves(t *testing.T) {
	for _, n := range []tier.Name{tier.Free, tier.Starter, tier.Pro, tier.Enterprise} {
		if got := tierOfName(string(n)); got != n {
			t.Errorf("tierOfName(%q) = %q, want %q — a registered tier name must win "+
				"over any catalog lookup", n, got, n)
		}
	}
}

// And a string that is neither a tier nor a sold plan is still Free. Unknown input
// must not be promoted into a paid tier by accident — the failure this fixes runs one
// way only.
func TestUnknownNameIsStillFree(t *testing.T) {
	for _, raw := range []string{"", "nope", "zen-ultra", "pro-plus", "../pro"} {
		if got := tierOfName(raw); got != tier.Free {
			t.Errorf("tierOfName(%q) = %q, want free — an unrecognized name must never "+
				"confer a paid tier", raw, got)
		}
	}
}

// ParseOK reports RECOGNITION, which is the distinction Parse cannot express and the
// reason the slugs fell through silently.
func TestParseOKSeparatesUnknownFromFree(t *testing.T) {
	if n, ok := tier.ParseOK("free"); !ok || n != tier.Free {
		t.Errorf(`ParseOK("free") = (%q, %v), want (free, true)`, n, ok)
	}
	if n, ok := tier.ParseOK("max"); ok || n != tier.Free {
		t.Errorf(`ParseOK("max") = (%q, %v), want (free, false) — "max" is a plan slug, `+
			`not a registered tier`, n, ok)
	}
}
