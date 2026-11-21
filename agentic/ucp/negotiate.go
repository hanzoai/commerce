// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ucp

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

// Supports reports whether the business speaks protocol version v: its version
// or a key of supported_versions (overview, "Request-Time Validation").
func (m Meta) Supports(v string) bool {
	if v == m.Version {
		return true
	}
	_, ok := m.SupportedVersions[v]
	return ok
}

// Negotiate computes the active capabilities by the intersection algorithm of
// the overview ("Intersection Algorithm"): a business capability is a
// candidate if the platform declares the same name; each candidate is selected
// at the highest version present in both arrays and excluded when there is none;
// an extension none of whose parents remains is removed, repeatedly. The
// business's entry for the selected version is returned.
func Negotiate(business, platform *Meta) *Active {
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
	for removed := true; removed; {
		removed = false
		for name, c := range out.Capabilities {
			if ps := c.Parents(); len(ps) > 0 && !anyActive(out.Capabilities, ps) {
				delete(out.Capabilities, name)
				removed = true
			}
		}
	}
	return out
}

func anyActive(caps map[string]Capability, names []string) bool {
	for _, n := range names {
		if _, ok := caps[n]; ok {
			return true
		}
	}
	return false
}

// Relevant selects the capabilities a response for an operation on root
// carries: the root, if negotiated, and every negotiated extension that extends
// it ("Response Capability Selection").
func (a *Active) Relevant(root string) map[string][]Capability {
	out := map[string][]Capability{}
	if _, ok := a.Capabilities[root]; !ok {
		return out
	}
	for _, n := range sortedKeys(a.Capabilities) {
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
