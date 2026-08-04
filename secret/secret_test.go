// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package secret

import (
	"slices"
	"strings"
	"testing"
)

func TestLike(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		// Leaked provider keys (incident 2026-07-02) — the shapes the guard
		// was originally written for.
		{"anthropic", "sk-ant-api03-cPXc0f", true},
		{"openai", "sk-proj-7qGaB9rn", true},
		{"digitalocean", "sk-do-u_6ppglS1w", true},
		{"hanzo", "sk-hz-0a2d6440-c", true},

		// Refused on shape alone. This one authenticates nowhere, but a caller
		// can still type it at the org header, and a name that is not refused
		// is a row.
		{"hk", "hk-feb5b4b27e2c0", true},

		// Stripe. This is a billing system, so these are the likeliest keys
		// to be pasted into a tenant header — and every one of them was
		// missed before.
		{"stripe secret live", "sk_live_51H8xQ2eZvKYlo2C", true},
		{"stripe secret test", "sk_test_51H8xQ2eZvKYlo2C", true},
		{"stripe publishable live", "pk_live_51H8xQ2eZvKYlo2C", true},
		{"stripe publishable test", "pk_test_51H8xQ2eZvKYlo2C", true},
		{"stripe restricted live", "rk_live_51H8xQ2eZvKYlo2C", true},
		{"stripe restricted test", "rk_test_51H8xQ2eZvKYlo2C", true},
		{"stripe webhook secret", "whsec_tW3Xy9pLqR2mN8vB", true},

		// A whole header value, or a bare token, pasted as a name.
		{"bearer header", "Bearer sk-ant-api03-cPXc0f", true},
		{"bearer lowercase", "bearer abc123", true},
		{"bare jwt", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.e30.x", true},

		// Case and whitespace — held before, must keep holding.
		{"upper", "SK-HZ-UPPER", true},
		{"mixed case stripe", "SK_Live_51H8xQ2", true},
		{"leading space", "  sk-hz-trimmed", true},
		{"trailing space", "hk-feb5b4b27e2c0  ", true},
		{"nbsp leading", "\u00a0hk-feb5b4b27e2c0", true},

		// Zero-width and formatting runes — these bypassed BOTH the Go check
		// and the DB trigger's anchor before this change.
		{"zwsp leading", "\u200bsk-ant-api03", true},
		{"bom leading", "\ufeffhk-feb5b4b27e2c0", true},
		{"zwnj leading", "\u200csk-live", true},
		{"zwj leading", "\u200dhk-abc", true},
		{"word joiner leading", "\u2060sk-abc", true},
		{"soft hyphen leading", "\u00adsk-abc", true},
		{"zwsp interior split", "s\u200bk-ant-api03", true},
		{"bom interior split", "hk\ufeff-abc", true},
		{"stacked blanks", "\ufeff\u200b\u00a0sk_live_51H8", true},

		// Real org identifiers must still be allowed. A slug cannot contain
		// the separator every marker ends in, so none of these collide.
		{"slug", "hanzo", false},
		{"slug adnexus", "adnexus", false},
		{"slug maxpower", "maxpower", false},
		{"slug with hyphen", "iam-user", false},
		{"slug system", "system", false},
		{"slug lux", "lux", false},
		{"uuid", "2d4d67ab-30f1-474e-b81f-f60461852259", false},
		{"skunkworks", "skunkworks", false},
		{"hkust", "hkust", false},
		{"empty", "", false},
		{"numeric", "1783182697", false},

		// Near-misses on the new markers: same letters, no separator.
		{"pkg", "pkg", false},
		{"rkelly", "rkelly", false},
		{"whsecurity", "whsecurity", false},
		{"bearerless", "bearershare", false},
		{"eyewear", "eyewear", false},
		{"eyj is a marker but eye is not", "eye", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Like(c.in); got != c.want {
				t.Errorf("Like(%q) = %v, want %v (normalized %q)", c.in, got, c.want, Normalize(c.in))
			}
		})
	}
}

// Markers are matched against the normalized form, so each must already be
// lowercase and free of Blank runes — otherwise it could never match, in Go or
// in the SQL backstop that reproduces Normalize with translate/btrim/lower.
//
// A marker MAY end in a space ("bearer "), which is why this checks those two
// properties directly rather than round-tripping through Normalize: trailing
// space is meaningful in a marker and would be trimmed away.
func TestPrefixesAreMatchable(t *testing.T) {
	for _, p := range Prefixes {
		if p == "" {
			t.Error("empty prefix matches every name")
			continue
		}
		if p != strings.ToLower(p) {
			t.Errorf("prefix %q is not lowercase, so it can never match", p)
		}
		for _, r := range p {
			if slices.Contains(Blank, r) {
				t.Errorf("prefix %q contains blank rune %U, which Normalize strips", p, r)
			}
		}
	}
}
