// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

// Package org resolves — and memoizes — the organization a request acts on
// behalf of. It is the ONE org-resolution path in commerce: the IAM/gateway
// identity middleware and the verified-service-token branch both land here.
//
// WHY THE CACHE EXISTS (money-critical, incident 2026-07-04):
//
// Resolution used to call GetOrCreate("Name=", name) on EVERY request, on the
// caller's cancelable request context. Under concurrent load (many orgs, e2e
// first-touch of NEW orgs, and the auto-recharge cron all sharing commerce's
// single writer) that read-then-maybe-create serialized behind the writer; when
// the wait exceeded the caller's deadline the query returned "context
// canceled", auth fell through to the legacy per-org-token path, tried to Peek
// a service token as a JWT, and 401'd — so cloud-api's balance gate
// fail-secured to 402 and real customers could not run paid inference.
//
//   - CACHE: an LRU keyed by org name with a short TTL. A hit does ZERO
//     datastore work, so ORG RESOLUTION touches no SQL in the steady state.
//     That is a claim about this package only — the IAM middleware still fires
//     its trial on-ramp per authenticated request, which does query; what bounds
//     the store there is that path's own concurrency cap, not this cache.
//   - SINGLEFLIGHT: concurrent misses for one name collapse into ONE
//     GetOrCreate, so a burst of first requests for a brand-new org performs
//     exactly one create, not N contending writes.
//   - DETACHED + BOUNDED: resolution runs on a fresh context.Background() with
//     its OWN timeout, so a slow resolve fast-fails inside commerce instead of
//     being canceled mid-create by an upstream deadline.
//
// WHY THE HOT PATH MUST NOT TOUCH THE STORE (2026-07-18 SEV1, heap profile):
//
// Resolve allocates the Organization BEFORE it issues its query. A request
// blocked in that query therefore keeps a live 5,256-byte struct reachable from
// its goroutine stack for as long as it waits. When the connection pool starved
// (unclosed single-row iterators, fixed in 24ff20a68) thousands of requests
// blocked here for their full 10s deadline at once, and the heap filled with
// tens of thousands of simultaneously-live Organizations — 40% of the process
// heap in the v1.801.88 profile. Serving the steady state from memory removes
// the blocking I/O from the hot path, so the pile-up cannot form.
//
// WHY RESOLVE RETURNS A COPY:
//
// The cached Organization is a 5,256-byte struct that callers MUTATE per
// request — notably o.Live, the live/test switch deciding whether a charge hits
// a sandbox or a real processor. Handing the same pointer to concurrent
// requests would let a test-mode request flip Live for a live one: a data race
// and a money bug. The cached entry is therefore IMMUTABLE and every caller
// gets its own copy bound to that request's datastore. Callers mutate freely;
// the cache can never be corrupted, and two requests for the same org can never
// observe each other's writes.
package org

import (
	"context"
	"errors"
	"slices"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"golang.org/x/sync/singleflight"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/secret"
)

const (
	// cacheSize bounds the LRU. Commerce serves a few thousand orgs at most;
	// 4096 hot entries covers the working set and evicts the cold tail. The
	// bound is the point: an unbounded org cache would relocate the heap growth
	// it exists to remove. Overflow is correctness-neutral — an evicted name
	// just re-resolves.
	cacheSize = 4096

	// ttl bounds staleness. Org identity fields (Name/Live/Enabled) change
	// rarely; 60s picks up a mutation within a minute while steady-state
	// requests stay store-free. Short enough that a disabled org can't be served
	// stale for long, long enough to absorb a full load burst.
	ttl = 60 * time.Second

	// resolveTimeout is commerce's OWN deadline for a cache-miss resolve,
	// INDEPENDENT of the caller's context, so a busy writer fast-fails here
	// rather than hanging until the upstream cancels — which previously wedged
	// the create half-done and cascaded to a bogus 401.
	resolveTimeout = 4 * time.Second
)

// ErrResolveFailed wraps any failure to resolve/provision an org for an
// already-authenticated caller. Treat it as retryable (HTTP 503), never as a
// bad credential — the credential was verified; only the backing store
// hiccuped.
var ErrResolveFailed = errors.New("org: could not resolve organization")

// entry is an immutable resolved org plus its expiry. The pointer is never
// handed to a caller — Resolve copies it — so nothing outside this package can
// mutate a cached org.
type entry struct {
	org     *organization.Organization
	expires time.Time
}

var (
	cache *lru.Cache[string, entry]
	group singleflight.Group

	// now is overridable in tests to drive TTL expiry deterministically.
	now = time.Now
)

func init() {
	// lru.New only errors on a non-positive size, which cacheSize is not, so a
	// failure here is a programming bug worth surfacing loudly at startup.
	c, err := lru.New[string, entry](cacheSize)
	if err != nil {
		panic("org: LRU init: " + err.Error())
	}
	cache = c

	// Drop a cached org as soon as it is persisted, so an identity change
	// (Name/Enabled/Live/fees) is not masked for up to a full TTL.
	organization.OnSaved = Invalidate
}

// Resolve returns the organization named name, creating it if it does not yet
// exist. The returned value is owned by the caller and safe to mutate.
//
// Steady state (name cached, unexpired) does NO datastore work. A miss resolves
// under singleflight on a detached, bounded context, so concurrent misses for
// the same name share one GetOrCreate and a slow store fast-fails with
// ErrResolveFailed instead of blocking on the caller's deadline.
func Resolve(ctx context.Context, name string) (*organization.Organization, error) {
	// Never provision an org from a bearer-shaped identifier. `name` is the
	// gateway-supplied X-Org-Id; a caller who presents a raw API key as their
	// bearer would otherwise cause GetOrCreate below to persist the key as an
	// org name and tenant id, leaking the secret (incident 2026-07-02). This
	// runs before any cache read or store write, so a secret-shaped name can
	// neither be looked up nor ever enter the cache.
	if secret.Like(name) {
		return nil, organization.ErrSecretLikeName
	}

	if e, ok := cache.Get(name); ok && now().Before(e.expires) {
		return bind(e.org, ctx), nil
	}

	// Miss (or expired): one resolver per name across all concurrent callers.
	v, err, _ := group.Do(name, func() (interface{}, error) {
		// Re-check under the flight: a racer that resolved while we queued
		// populated the cache, so don't hit the store again.
		if e, ok := cache.Get(name); ok && now().Before(e.expires) {
			return e.org, nil
		}

		rctx, cancel := context.WithTimeout(context.Background(), resolveTimeout)
		defer cancel()

		o := organization.New(datastore.New(rctx))
		o.Name = name
		o.Enabled = true

		// GetOrCreate: find by name (read) or create (write). Resolving by NAME
		// is deliberate — it is the only lookup correct for every org. A by-id
		// fast path off a shared external cache is what produced the 2026-07-18
		// SEV1: the cached id was a pre-hashid legacy value that could never
		// match the org's real key, so every lookup missed.
		if err := o.GetOrCreate("Name=", name); err != nil {
			log.Warn("org: resolve %q failed: %v", name, err)
			return nil, err
		}

		cache.Add(name, entry{org: o, expires: now().Add(ttl)})
		return o, nil
	})
	if err != nil {
		return nil, errors.Join(ErrResolveFailed, err)
	}
	return bind(v.(*organization.Organization), ctx), nil
}

// bind returns a request-owned copy of an immutable cached org, wired to ctx's
// datastore and sharing no mutable state with the cache.
//
// Rebind, NOT Init: Init ends in orm.ApplyDefaults, whose Defaulter tail calls
// Organization.Defaults() unconditionally. On a zero struct (New) that is the
// point; on an entity already loaded from the store it silently replaces stored
// values with platform defaults — Enabled=true over a suspended org, Fees.Card
// back to 5%/50c, Admins/Moderators/Partners emptied. Handlers persist the org
// they are handed with full-entity writes, so binding through Init would write
// those defaults back permanently.
//
// The struct copy above is shallow, so every reference-typed field must be
// cloned or the cache and each request would share one backing array — a
// concurrent AddToken append or an Owners edit would be visible to the cached
// entry and to other in-flight requests for the same org. bind_test.go walks
// the struct by reflection and fails if a new reference field is added without
// being cloned here.
func bind(cached *organization.Organization, ctx context.Context) *organization.Organization {
	o := new(organization.Organization)
	*o = *cached

	o.Owners = slices.Clone(cached.Owners)
	o.Admins = slices.Clone(cached.Admins)
	o.Moderators = slices.Clone(cached.Moderators)
	o.Websites = slices.Clone(cached.Websites)
	o.Partners = slices.Clone(cached.Partners)
	o.SecretKey = slices.Clone(cached.SecretKey)
	o.Tokens = slices.Clone(cached.Tokens)
	o.Integrations = slices.Clone(cached.Integrations)

	// Wallet is a lazily-loaded, non-persisted (datastore:"-") handle. Sharing
	// the pointer would hand two requests the same mutable wallet; each loads
	// its own via GetOrCreateWallet.
	o.Wallet = nil

	o.Rebind(datastore.New(ctx))
	o.AccessTokens.Init(o)
	return o
}

// Invalidate drops a name so the next Resolve re-reads it. Call after mutating
// an org's identity fields (Name/Enabled/Live) so a change is not masked by the
// TTL. Safe to call for an absent name.
func Invalidate(name string) { cache.Remove(name) }
