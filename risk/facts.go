// Copyright © 2026 Hanzo AI. MIT License.

package risk

import (
	"errors"
	"strings"
)

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

// Facts projects caller-supplied signals onto what may travel — an allowlisted
// key, lowercased so one fact has one spelling, carrying a non-empty value of at
// most [factMax] bytes — and REPORTS HOW MANY IT DROPPED.
//
// The drop is not an error: a merchant's metadata is not a request to this
// plane, and refusing a payment over a stray field would be a worse failure than
// ignoring it. But it is not silent either. A dropped signal is a fact the model
// then scores WITHOUT, so the judgement records the count ([screen.Screen]'s
// Detail["dropped"]) and an operator can see a caller whose signals never arrive
// instead of wondering why its scores are flat. A bound that binds invisibly is
// a control switching itself off.
//
// The result is BOUNDED by construction: at most len(factKeys) keys, each at
// most factMax bytes. That bound is the point — these values are written to a
// durable record, and a store a caller can grow without limit is a store a
// caller can fill.
func Facts(in map[string]string) (map[string]string, int) {
	if len(in) == 0 {
		return nil, 0
	}
	out := map[string]string{}
	for k, v := range in {
		key := strings.ToLower(strings.TrimSpace(k))
		if !factKeys[key] || v == "" || len(v) > factMax {
			continue
		}
		out[key] = v
	}
	if len(out) == 0 {
		return nil, len(in)
	}
	return out, len(in) - len(out)
}

// Text is the most bytes ANY caller-supplied string this plane stores may
// carry: a reason, a reference, a processor name, a note, an idempotency key, a
// subject id.
//
// A COUNT IS NOT A BOUND WHEN THE VALUE IS THE CALLER'S. Every read here is
// capped at a number of ROWS — 500 controls, 200 screens, 200 outcomes — and a
// row cap over caller-sized strings bounds nothing at all in bytes: one tenant
// puts a megabyte in a reason and 500 of them is half a gigabyte materialised
// in a process shared with every other tenant. So the bound that matters is on
// the VALUE, here, at the door, and the row cap times this constant IS the byte
// ceiling a read can cost. [screen.Bytes] states that product, and a test
// measures a real worst-case row against it.
const Text = 256

// Hits is the most rule names one judgement records. The scoring plane is ours,
// but it is reached at an address an operator configures, and a bound that only
// holds while the far end behaves is not a bound.
const Hits = 32

// ErrText refuses a caller string too long to store. It REFUSES rather than
// truncating: a silently shortened reason is evidence that has been altered
// without anyone being told, and a bound that binds quietly is a bound nobody
// can operate.
var ErrText = errors.New("risk: a caller-supplied field is at most 256 bytes")

// Fits reports whether v is short enough for this plane to keep.
func Fits(v string) bool { return len(v) <= Text }

// Bound refuses the first caller string that will not fit. It takes the values
// in one call so a new field on a caller-shaped value is added to the bound in
// the same edit that adds it to the type.
func Bound(values ...string) error {
	for _, v := range values {
		if !Fits(v) {
			return ErrText
		}
	}
	return nil
}

// hits is the projection of a decision's rule names onto what may be stored —
// at most [Hits] of them, each at most [Text] bytes — and how many it dropped.
//
// The count matters for the same reason the signals' does: the rule names are
// WHY a move was scored the way it was, and a judgement that quietly kept 32 of
// 400 of them is evidence with the reasoning cut off at an arbitrary point that
// nothing records.
func hits(in []string) ([]string, int) {
	kept := in
	if len(kept) > Hits {
		kept = kept[:Hits]
	}
	out := make([]string, 0, len(kept))
	for _, h := range kept {
		if h != "" && Fits(h) {
			out = append(out, h)
		}
	}
	if len(out) == 0 {
		return nil, len(in)
	}
	return out, len(in) - len(out)
}

// Signals projects a payment request's metadata onto the same facts. It is the
// TYPE conversion the processor seam needs — metadata is map[string]any — and
// nothing else: the allowlist and the bounds are [Facts]', so the two doors
// cannot drift into disagreeing about what a caller may send. What it drops is
// counted where every other door's drops are counted, inside [Screener.Screen].
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
	kept, _ := Facts(str)
	return kept
}
