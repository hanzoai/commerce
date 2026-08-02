// Copyright © 2026 Hanzo AI. MIT License.

package risk

import "strings"

// facts.go is the CLOSED boundary between what a caller says and what the money
// plane sends and STORES.
//
// One list, one place. A signal reaches the scoring plane and the durable screen
// record through exactly this projection, whichever door it came in by — a
// payment request's metadata, a typed op's body, a merchant's own integration.
// A rule enforced at one door is a rule the second door does not have.
//
// It is an ALLOWLIST and not a denylist for the reason every such boundary is:
// the payload is caller-shaped. A merchant who puts a card number in a metadata
// field must not thereby send it to a scoring plane and have it written into an
// evidence record we keep for years, and no list of forbidden key names can be
// relied on to catch that.

// factKeys is the CLOSED set of facts a CALLER may state. Counted facts the
// money plane derives itself do not travel this way — see [Move.Standing].
var factKeys = map[string]bool{
	"ip":          true,
	"asn":         true,
	"country":     true,
	"email":       true,
	"phone":       true,
	"device":      true,
	"fingerprint": true,
	"ua":          true,
	"bin":         true,
	"funding":     true,
	"brand":       true,
	"last4":       true,
	"channel":     true,
	"agent":       true,
	"session":     true,
}

// factMax bounds ONE fact's value in bytes. It is generous for every fact on
// the list above — the longest of them is a user agent — and small enough that
// the whole projection cannot exceed len(factKeys)*factMax however hard a
// caller pushes. A value over it is not a long fact, it is a different thing
// wearing a fact's name, so it does not travel at all.
const factMax = 256

// Facts projects caller-supplied signals onto what may travel: an allowlisted
// key, lowercased so one fact has one spelling, carrying a non-empty value of at
// most [factMax] bytes. Everything else is dropped — silently, because a
// merchant's metadata is not a request to this plane and refusing a payment over
// a stray field would be a worse failure than ignoring it.
//
// The result is BOUNDED by construction: at most len(factKeys) keys, each at
// most factMax bytes. That bound is the point — these values are written to a
// durable record, and a store a caller can grow without limit is a store a
// caller can fill.
func Facts(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := map[string]string{}
	for k, v := range in {
		key := strings.ToLower(strings.TrimSpace(k))
		if !factKeys[key] {
			continue
		}
		if v == "" || len(v) > factMax {
			continue
		}
		out[key] = v
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// Signals projects a payment request's metadata onto the same facts. It is the
// TYPE conversion the processor seam needs — metadata is map[string]any — and
// nothing else: the allowlist and the bounds are [Facts]', so the two doors
// cannot drift into disagreeing about what a caller may send.
func Signals(meta map[string]any) map[string]string {
	if len(meta) == 0 {
		return nil
	}
	str := make(map[string]string, len(meta))
	for k, v := range meta {
		if s, ok := v.(string); ok {
			str[k] = s
		}
	}
	return Facts(str)
}
