// Copyright © 2026 Hanzo AI. MIT License.

// Package secret recognizes strings that are credentials rather than
// identifiers.
//
// It exists so that exactly one list of credential markers backs every guard in
// the system: the Go check on the provisioning path (models/organization), and
// the SQL backstop trigger below the ORM (db). A credential caught by one gate
// but not the other is the bug this package is shaped to prevent — so the SQL
// is generated from these same values rather than written twice.
package secret

import (
	"slices"
	"strings"
)

// Prefixes are the markers that identify a string as a credential rather than
// an organization slug. Matching is on the normalized form (see Normalize), so
// each entry is lowercase.
//
// Real org identifiers are slugs ("hanzo", "adnexus"), UUIDs, or numeric ids.
// Matching is prefix-based and every marker ends in a separator ("-", "_", " ")
// that a slug cannot contain, so no real name collides: "skunkworks" and
// "hkust" do not start with "sk-"/"hk-". The lone exception is "eyj", the
// base64 of a JWT header '{"' — a bare token pasted as a tenant.
var Prefixes = []string{
	// Hanzo, and the sk- provider family: Anthropic (sk-ant-), OpenAI
	// (sk-proj-), DigitalOcean (sk-do-), Hanzo (sk-hz-).
	"hk-",
	"sk-",
	// Stripe. This is a billing system, so these are the keys most likely to
	// reach a tenant header. The trailing underscore covers both the _live_
	// and _test_ variants of each.
	"sk_",
	"pk_",
	"rk_",
	"whsec_",
	// A whole Authorization header value pasted as a name.
	"bearer ",
	// A bare JWT.
	"eyj",
}

// Blank are runes stripped before matching. They carry no identifier meaning
// but do break a naive prefix test: a zero-width space or a BOM in front of
// "sk-" defeats an anchored match, and a non-breaking space is not removed by
// trimming in SQL (Postgres btrim strips ASCII space only). They are removed
// everywhere in the string, not just at the ends, so an interior insertion
// cannot split a marker either.
//
// Written as escapes, never as literals: these runes are invisible, and source
// that renders identically to different bytes cannot be reviewed.
//
// Stripping is used only to decide; the original string is never rewritten.
var Blank = []rune{
	0x00A0, // no-break space
	0x00AD, // soft hyphen
	0x180E, // mongolian vowel separator
	0x200B, // zero width space
	0x200C, // zero width non-joiner
	0x200D, // zero width joiner
	0x200E, // left-to-right mark
	0x200F, // right-to-left mark
	0x2028, // line separator
	0x2029, // paragraph separator
	0x202F, // narrow no-break space
	0x205F, // medium mathematical space
	0x2060, // word joiner
	0x3000, // ideographic space
	0xFEFF, // zero width no-break space / BOM
}

// Normalize reduces a name to the form the markers are matched against:
// Blank runes removed, surrounding whitespace trimmed, lowercased.
func Normalize(name string) string {
	var b strings.Builder
	b.Grow(len(name))
	for _, r := range name {
		if !slices.Contains(Blank, r) {
			b.WriteRune(r)
		}
	}
	return strings.ToLower(strings.TrimSpace(b.String()))
}

// Like reports whether name is a credential rather than an organization
// identifier.
//
// Any code that provisions a tenant from an untrusted, gateway-supplied name
// MUST reject these first: otherwise a caller who presents a raw key as their
// bearer causes the key to be persisted as an org name and tenant id, leaking
// the secret into the datastore (incident 2026-07-02).
func Like(name string) bool {
	n := Normalize(name)
	for _, p := range Prefixes {
		if strings.HasPrefix(n, p) {
			return true
		}
	}
	return false
}
