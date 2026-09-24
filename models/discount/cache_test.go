package discount

import (
	"strings"
	"testing"
	"time"

	"github.com/hanzoai/commerce/datastore/iface"
	"github.com/hanzoai/commerce/models/discount/scope"
	"github.com/hanzoai/commerce/util/nscontext"
)

// The bug this file exists for. The org scope used to key on the bare constant
// "discount-keys-organization" with the id ignored, so two tenants shared ONE
// entry and the second priced its order with the first's discounts.
func TestKeyForScope_OrgScopeIsPerTenant(t *testing.T) {
	a := keyForScope("orga", scope.Organization, "")
	b := keyForScope("orgb", scope.Organization, "")
	if a == b {
		t.Fatalf("two tenants share one org-scope cache key (%q) — cross-tenant discount leak", a)
	}
	if !strings.HasPrefix(a, "orga|") || !strings.HasPrefix(b, "orgb|") {
		t.Errorf("keys are not namespace-prefixed: %q / %q", a, b)
	}
}

// Every scope must be per-tenant, not just the org one: a store/product/variant
// id is not guaranteed distinct across tenants, and relying on that would be an
// invisible dependency.
func TestKeyForScope_EveryScopeIsPerTenant(t *testing.T) {
	for _, tc := range []struct {
		sc scope.Type
		id string
	}{
		{scope.Organization, ""},
		{scope.Store, "store1"},
		{scope.Collection, "coll1"},
		{scope.Product, "prod1"},
		{scope.Variant, "var1"},
	} {
		if keyForScope("orga", tc.sc, tc.id) == keyForScope("orgb", tc.sc, tc.id) {
			t.Errorf("scope %q with id %q collides across tenants", tc.sc, tc.id)
		}
	}
}

// An unnamespaced caller (cron, test) must not land in a tenant's entry.
func TestKeyForScope_EmptyNamespaceIsDistinct(t *testing.T) {
	if keyForScope("", scope.Organization, "") == keyForScope("orga", scope.Organization, "") {
		t.Error("empty namespace shares a key with a named tenant")
	}
}

// The cache read path must derive its namespace from the SAME ctx the datastore
// query uses, so a hit can never describe a different tenant.
func TestCache_ReadIsNamespaceScoped(t *testing.T) {
	ctxA := nscontext.WithNamespace(t.Context(), "orga")
	ctxB := nscontext.WithNamespace(t.Context(), "orgb")

	keyA := keyForScope(nscontext.GetNamespace(ctxA), scope.Organization, "")
	keyB := keyForScope(nscontext.GetNamespace(ctxB), scope.Organization, "")

	aKeys := []iface.Key{nil} // identity only; contents are irrelevant here
	if err := cacheDiscounts(ctxA, keyA, aKeys); err != nil {
		t.Fatalf("cacheDiscounts: %v", err)
	}
	t.Cleanup(func() {
		discountCacheLock.Lock()
		delete(discountCache, keyA)
		delete(discountCache, keyB)
		discountCacheLock.Unlock()
	})

	if _, err := getCachedDiscounts(ctxA, keyA); err != nil {
		t.Fatalf("org A must hit its own entry: %v", err)
	}
	if _, err := getCachedDiscounts(ctxB, keyB); err == nil {
		t.Fatal("org B read org A's cached discounts — cross-tenant leak")
	}
}

// A disabled discount stayed applied forever, because entries never expired and
// the query filters Enabled=true. The TTL bounds that.
func TestCache_EntryExpires(t *testing.T) {
	ctx := nscontext.WithNamespace(t.Context(), "orgttl")
	key := keyForScope("orgttl", scope.Organization, "")

	discountCacheLock.Lock()
	discountCache[key] = cacheEntry{keys: []iface.Key{nil}, at: time.Now().Add(-2 * ttl)}
	discountCacheLock.Unlock()
	t.Cleanup(func() {
		discountCacheLock.Lock()
		delete(discountCache, key)
		discountCacheLock.Unlock()
	})

	if _, err := getCachedDiscounts(ctx, key); err == nil {
		t.Fatal("an entry older than ttl was served — a switched-off discount keeps applying")
	}
}

// Invalidation with no namespace available (the hooks carry no ctx) must clear
// that scope in every namespace. Over-invalidating costs a datastore read;
// under-invalidating serves a price the merchant already changed.
func TestInvalidate_NoNamespaceClearsEveryTenant(t *testing.T) {
	suffix := keyForScope("", scope.Organization, "")
	ka, kb := "orga"+suffix, "orgb"+suffix

	discountCacheLock.Lock()
	discountCache[ka] = cacheEntry{keys: []iface.Key{nil}, at: time.Now()}
	discountCache[kb] = cacheEntry{keys: []iface.Key{nil}, at: time.Now()}
	discountCacheLock.Unlock()
	t.Cleanup(func() {
		discountCacheLock.Lock()
		delete(discountCache, ka)
		delete(discountCache, kb)
		discountCacheLock.Unlock()
	})

	d := &Discount{}
	d.Scope.Type = scope.Organization
	if err := d.invalidateCache(); err != nil {
		t.Fatalf("invalidateCache: %v", err)
	}

	discountCacheLock.RLock()
	_, aOK := discountCache[ka]
	_, bOK := discountCache[kb]
	discountCacheLock.RUnlock()
	if aOK || bOK {
		t.Errorf("stale entries survived invalidation: orga=%v orgb=%v", aOK, bOK)
	}
}
