// Copyright © 2026 Hanzo AI. MIT License.

package risk

import (
	"errors"
	"fmt"
	"strings"
)

// facts.go is the ONE gate every signal passes through on its way to the
// scoring plane and to this org's durable record.
//
// A signal is a FACT ABOUT A MOVE — an ip, a bin, a device — and the set of
// facts this plane understands is CLOSED. It has to be an allowlist and not a
// denylist for two independent reasons, and either one alone would be enough:
//
//   - What arrives is caller-shaped. A merchant who puts a card number in a
//     metadata field must not thereby send it to a scoring plane, and no list
//     of forbidden key names catches that. A name the plane does not read is a
//     name it must not store.
//   - What is accepted is DURABLE. Every accepted key lands in a screen row
//     that is kept as evidence, so an open map is an unbounded write a caller
//     controls: a thousand keys with a megabyte each, per screen, in the org's
//     own store.

// signalKeys is the CLOSED set of facts that may travel to the scoring plane
// and into the record. Adding a key here is a deliberate act.
//
// Two vocabularies, one list, because one list is the only way the gate stays
// the gate. The first group is what a REQUEST knows about itself — who is
// paying, from where, with what. The second is what the ORG'S OWN RECORD knows
// about a merchant, counted by [Standing.signals] for a merchant-stage review:
// derived here, but it travels the same wire into the same durable row, so it
// passes the same gate rather than an exemption beside it.
var signalKeys = map[string]bool{
	// facts about the request
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

	// the counted standing of a merchant
	"screens":     true,
	"refused":     true,
	"disputes":    true,
	"lost":        true,
	"refunds":     true,
	"failed":      true,
	"negative":    true,
	"disputerate": true,
	"refusalrate": true,
	"volumein":    true,
	"volumeout":   true,
	"held":        true,
	"reserved":    true,
	"window":      true,
}

const (
	// maxFact bounds one fact's value. Every fact this plane names is an
	// identifier, a code or a user agent; none of them is prose, and the longest
	// real one (a user agent) is comfortably inside this.
	maxFact = 512
	// maxStated bounds how many keys a caller may STATE before the allowlist is
	// even consulted. The allowlist already bounds what survives, but a caller
	// that sends a million keys has already made us walk a million keys.
	maxStated = 64
)

// ErrFact refuses a stated signal the plane will not carry.
var ErrFact = errors.New("risk: signal")

// Facts is the STATED path: the caller named these facts deliberately, so a
// name this plane does not carry or a value over the bound is the caller's
// mistake and is REFUSED by name. Silently dropping what a caller explicitly
// asked to be scored on would make a screen quietly weaker than the caller
// believes it to be.
func Facts(stated map[string]string) (map[string]string, error) {
	if len(stated) == 0 {
		return nil, nil
	}
	if len(stated) > maxStated {
		return nil, fmt.Errorf("%w: at most %d may be stated on one move", ErrFact, maxStated)
	}
	out := make(map[string]string, len(stated))
	for k, v := range stated {
		key, value, ok := fact(k, v)
		switch {
		case !signalKeys[key]:
			return nil, fmt.Errorf("%w: %q is not a fact this plane reads", ErrFact, key)
		case len(v) > maxFact:
			return nil, fmt.Errorf("%w: %q is longer than %d bytes", ErrFact, key, maxFact)
		case !ok:
			continue
		}
		out[key] = value
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

// Signals is the METADATA path: a payment request's metadata is incidental —
// the merchant put it there for its own reasons, not to be scored — so a name
// this plane does not read is DROPPED rather than refused. Refusing here would
// fail a payment because someone wrote a long note in an unrelated field.
func Signals(meta map[string]any) map[string]string {
	if len(meta) == 0 {
		return nil
	}
	out := map[string]string{}
	for k, v := range meta {
		s, is := v.(string)
		if !is {
			continue
		}
		key, value, ok := fact(k, s)
		if !ok || !signalKeys[key] || len(s) > maxFact {
			continue
		}
		out[key] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// fact normalizes one key/value pair to the single spelling the plane uses: the
// key lowercased and trimmed, the value trimmed. ok is false for a value that
// carries nothing.
func fact(k, v string) (string, string, bool) {
	key := strings.ToLower(strings.TrimSpace(k))
	value := strings.TrimSpace(v)
	return key, value, value != ""
}
