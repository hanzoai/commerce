// Copyright © 2026 Hanzo AI. MIT License.

// Package org provides KV-cached org resolution from gateway-supplied
// X-Org-Id headers. The trust boundary lives in pkg/auth — this package
// is the lookup-side cache, separate so callers can bind a KV without
// pulling in the auth middleware.
package org

import (
	"context"
	"sync"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/organization"
)

// KVCache is the minimal interface required for org-id caching.
// *infra.KVClient satisfies it.
type KVCache interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
	Delete(ctx context.Context, keys ...string) error
}

var (
	mu      sync.RWMutex
	kvCache KVCache
	ttl     = 5 * time.Minute
)

// Bind wires the KV client. nil disables caching.
func Bind(kv KVCache) {
	mu.Lock()
	defer mu.Unlock()
	kvCache = kv
}

func cacheKey(name string) string { return "iam:org_by_name:" + name }

// Resolve loads an organization from a gateway-supplied owner name. It
// reads-through the KV cache and falls back to GetOrCreate, which auto-
// provisions the org record on first encounter.
//
// Caller is responsible for providing the right context — typically
// a request-scoped context.WithTimeout from the handler.
func Resolve(ctx context.Context, name string) (*organization.Organization, error) {
	// Never provision an org from a bearer-shaped identifier. `name` is the
	// gateway-supplied X-Org-Id; a caller who presents a raw API key as their
	// bearer would otherwise cause GetOrCreate below to persist the key as an
	// org name and tenant id, leaking the secret (incident 2026-07-02).
	if organization.IsSecretLikeName(name) {
		return nil, organization.ErrSecretLikeName
	}

	mu.RLock()
	kv := kvCache
	mu.RUnlock()

	db := datastore.New(ctx)
	o := organization.New(db)
	o.Name = name
	o.Enabled = true

	if kv != nil {
		// A legacy pre-hashid GAE numeric id (e.g. IAM's Valkey still holds
		// "1772587477" for org "hanzo") can never match the org's real hashid
		// key, so GetById is a guaranteed miss — and on Postgres an intID-keyed
		// lookup no schema expects. Skip it and resolve by name instead, which
		// re-caches the correct hashid below (self-heal). This hardens the exact
		// path behind the 2026-07-18 SEV1 (api.hanzo.ai default model down).
		if id, err := kv.Get(ctx, cacheKey(name)); err == nil && id != "" && !isLegacyNumericID(id) {
			if err := o.GetById(id); err == nil {
				return o, nil
			}
		}
	}

	if err := o.GetOrCreate("Name=", name); err != nil {
		return nil, err
	}
	if kv != nil {
		_ = kv.Set(ctx, cacheKey(name), o.Id(), ttl)
	}
	return o, nil
}

// isLegacyNumericID reports whether id is an all-ASCII-digit string. Commerce
// hashid ids are alphanumeric and effectively always contain letters, so a
// purely numeric cached id is a stale legacy (pre-hashid) id that by-id lookup
// can never resolve. Callers use this to route such ids to name-based
// resolution instead of a doomed GetById.
func isLegacyNumericID(id string) bool {
	if id == "" {
		return false
	}
	for _, r := range id {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
