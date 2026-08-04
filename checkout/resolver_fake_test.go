// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package checkout

// hostTenants is the in-memory Resolver the tests drive live handlers with.
//
// It replaces the exported StaticResolver, which existed in production solely to
// be a test double and a "bootstrap" resolver nothing bootstrapped — a second way
// to resolve a tenant beside the one real OrgResolver. Resolver is a one-method
// interface, so the fake belongs with its consumers, not in the shipped surface.
//
// Same semantics as the type it replaces: keys normalized, EXACT match only (so
// suffix spoofing like "pay.example.com.evil.com" misses), ErrUnknownTenant on a
// miss. No mutex: tests do not reconfigure it concurrently, and the Set method
// that needed one had no callers at all.
type hostTenants map[string]Tenant

func newHostTenants(hosts map[string]Tenant) hostTenants {
	m := make(hostTenants, len(hosts))
	for h, t := range hosts {
		m[normalizeHost(h)] = t
	}
	return m
}

func (f hostTenants) Resolve(host string) (Tenant, error) {
	h := normalizeHost(host)
	if h == "" {
		return Tenant{}, ErrUnknownTenant
	}
	if t, ok := f[h]; ok {
		return t, nil
	}
	return Tenant{}, ErrUnknownTenant
}
