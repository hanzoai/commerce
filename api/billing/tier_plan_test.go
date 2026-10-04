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
// slugs (go/dev/pro/max/team/enterprise). An active subscription is tiered by its
// slug through the catalog (tierForActivePaidSlug), so this drives off the CATALOG
// rather than a list written here: a plan added tomorrow is covered without anyone
// remembering to update a test, and nothing we sell confers Free.
func TestEverySoldPlanConfersAPaidTier(t *testing.T) {
	plans := catalog
	if len(plans) == 0 {
		t.Fatal("no plans in the catalog — this guard would pass over nothing")
	}

	sold := 0
	for _, p := range plans {
		if !paidTier(p.Slug) {
			continue // genuinely free/$0 rows (dns-free) are not the subject
		}
		sold++
		got := tierForActivePaidSlug(p.Slug)
		if got == tier.Free {
			t.Errorf("plan %q (category=%q, price=%d) confers Free — a sold plan "+
				"must never confer the most restrictive tier", p.Slug, p.Category, p.Price)
		}
		if p.Category == "enterprise" && got != tier.Enterprise {
			t.Errorf("plan %q is enterprise-category but confers %q", p.Slug, got)
		}
	}
	if sold == 0 {
		t.Fatal("catalog has no paid plans — the guard covered nothing")
	}
}

// A slug the catalog does not sell confers Free. Unknown input must not be promoted
// into a paid tier by accident.
func TestUnknownSlugConfersFree(t *testing.T) {
	for _, raw := range []string{"", "nope", "zen-ultra", "pro-plus", "../pro"} {
		if got := tierForActivePaidSlug(raw); got != tier.Free {
			t.Errorf("tierForActivePaidSlug(%q) = %q, want free", raw, got)
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
