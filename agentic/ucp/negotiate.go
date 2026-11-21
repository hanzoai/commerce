// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ucp

import (
	"sort"
)

// VersionRange is a version constraint: Min is required and inclusive, Max is
// inclusive and absent means no upper bound (schemas/ucp.json, version_constraint).
type VersionRange struct {
	Min string `json:"min"`
	Max string `json:"max,omitempty"`
}

// Has reports whether v lies in the range. Versions are YYYY-MM-DD dates, so
// string order is date order.
func (r VersionRange) Has(v string) bool {
	return v >= r.Min && (r.Max == "" || v <= r.Max)
}

// Requires is the version requirement an extension's schema author declares:
// the range of the selected ucp.version and of the selected versions of the
// capabilities it names (schemas/ucp.json, requires; overview, "Version
// Requirements"). It verifies dependencies after versions are selected; it
// does not select them.
type Requires struct {
	Protocol     *VersionRange           `json:"protocol,omitempty"`
	Capabilities map[string]VersionRange `json:"capabilities,omitempty"`
}

// Active is the outcome of negotiation: the protocol version in force and the
// capabilities both parties support at a version they share.
type Active struct {
	Version      string
	Capabilities map[string]Capability
}

// Has reports whether the capability was negotiated.
func (a *Active) Has(name string) bool {
	_, ok := a.Capabilities[name]
	return ok
}

// Names returns the negotiated capability names in sorted order.
func (a *Active) Names() []string { return sortedKeys(a.Capabilities) }

// Supports reports whether the business can speak protocol version v: its
// current version, or a key of supported_versions (overview, "Request-Time
// Validation").
func (m Meta) Supports(v string) bool {
	if v == m.Version {
		return true
	}
	_, ok := m.SupportedVersions[v]
	return ok
}

// Pick chooses the protocol version a platform that speaks the given versions
// uses with a business: the most recent version the business supports, that the
// platform also speaks ("Initial Service and Capability Discovery", step 2).
// It returns false when there is none.
func (m Meta) Pick(platform []string) (string, bool) {
	best, ok := "", false
	for _, v := range platform {
		if m.Supports(v) && v > best {
			best, ok = v, true
		}
	}
	return best, ok
}

// Negotiate computes the capabilities active for a session by the intersection
// algorithm of the overview ("Intersection Algorithm"):
//
//  1. a business capability is a candidate if the platform declares the same name;
//  2. each candidate is selected at the highest version string present in both
//     parties' arrays, and excluded when there is none;
//  3. an extension none of whose parents is in the result is removed, and this
//     repeats until nothing more is removed.
//
// requires, keyed by capability name, holds the version requirements the
// extension schemas declare; an extension whose requirements the selected
// versions do not meet is excluded and the pruning repeats ("Resolution Flow",
// step 4). The business's entry for the selected version is the one returned.
func Negotiate(business, platform *Meta, requires map[string]Requires) *Active {
	out := &Active{Version: business.Version, Capabilities: map[string]Capability{}}
	for name, bcaps := range business.Capabilities {
		theirs := map[string]bool{}
		for _, pc := range platform.Capabilities[name] {
			theirs[pc.Version] = true
		}
		var best *Capability
		for i := range bcaps {
			if theirs[bcaps[i].Version] && (best == nil || bcaps[i].Version > best.Version) {
				best = &bcaps[i]
			}
		}
		if best != nil {
			out.Capabilities[name] = *best
		}
	}
	for {
		removed := false
		for name, c := range out.Capabilities {
			if ps := c.Parents(); len(ps) > 0 && !anyActive(out.Capabilities, ps) {
				delete(out.Capabilities, name)
				removed = true
				continue
			}
			if r, ok := requires[name]; ok && !met(r, business.Version, out.Capabilities) {
				delete(out.Capabilities, name)
				removed = true
			}
		}
		if !removed {
			return out
		}
	}
}

func anyActive(caps map[string]Capability, names []string) bool {
	for _, n := range names {
		if _, ok := caps[n]; ok {
			return true
		}
	}
	return false
}

func met(r Requires, protocol string, caps map[string]Capability) bool {
	if r.Protocol != nil && !r.Protocol.Has(protocol) {
		return false
	}
	for name, rng := range r.Capabilities {
		c, ok := caps[name]
		if !ok || !rng.Has(c.Version) {
			return false
		}
	}
	return true
}

// Relevant selects the capabilities a response for an operation on root
// carries: the root if negotiated, and every negotiated extension that extends
// it ("Response Capability Selection"). The result is keyed by name.
func (a *Active) Relevant(root string) map[string][]Capability {
	out := map[string][]Capability{}
	if _, ok := a.Capabilities[root]; !ok {
		return out
	}
	names := make([]string, 0, len(a.Capabilities))
	for n := range a.Capabilities {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		c := a.Capabilities[n]
		relevant := n == root
		for _, p := range c.Parents() {
			relevant = relevant || p == root
		}
		if relevant {
			out[n] = []Capability{{Entity: Entity{Version: c.Version}}}
		}
	}
	return out
}
