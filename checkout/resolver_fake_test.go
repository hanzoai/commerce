// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package checkout

// hostOrgs is the in-memory Resolver the tests drive live handlers with.
//
// It replaces the exported StaticResolver, which existed in production solely to
// be a test double and a "bootstrap" resolver nothing bootstrapped — a second way
// to resolve a org beside the one real OrgResolver. Resolver is a one-method
// interface, so the fake belongs with its consumers, not in the shipped surface.
//
// Same semantics as the type it replaces: keys normalized, EXACT match only (so
// suffix spoofing like "pay.example.com.evil.com" misses), ErrUnknownOrg on a
// miss. No mutex: tests do not reconfigure it concurrently, and the Set method
// that needed one had no callers at all.
type hostOrgs map[string]Org

func newHostOrgs(hosts map[string]Org) hostOrgs {
	m := make(hostOrgs, len(hosts))
	for h, t := range hosts {
		m[normalizeHost(h)] = t
	}
	return m
}

func (f hostOrgs) Resolve(host string) (Org, error) {
	h := normalizeHost(host)
	if h == "" {
		return Org{}, ErrUnknownOrg
	}
	if t, ok := f[h]; ok {
		return t, nil
	}
	return Org{}, ErrUnknownOrg
}
