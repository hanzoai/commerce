package discount

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/datastore/iface"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/discount/scope"
	"github.com/hanzoai/commerce/util/nscontext"
)

// In-memory cache for discount keys (replaces appengine memcache).
//
// EVERY key is namespaced. It was not: the organization scope hashed to the bare
// constant "discount-keys-organization" — the id argument was ignored outright —
// so the FIRST org to price an order populated one entry every other org then
// read. The datastore query IS namespaced (datastore.New(ctx) reads the namespace
// from ctx), so only the cache crossed tenants: org B priced its order with org
// A's org-scoped discounts. Wrong prices, real money, and live at a single
// replica — never a multi-replica-only bug.
//
// Entries also never expired, and invalidation is a local map delete, so on N
// replicas a disabled or edited discount stayed live on every other replica
// forever. The query filters Enabled=true, which means a stale entry keeps
// applying a discount the merchant has already switched off. The TTL bounds that
// to ttl rather than "until the pod restarts". It is a mitigation, not the fix —
// a shared invalidation bus is — so keep it short.
const ttl = 30 * time.Second

type cacheEntry struct {
	keys []iface.Key
	at   time.Time
}

var (
	discountCache     = make(map[string]cacheEntry)
	discountCacheLock sync.RWMutex
)

// keyForScope computes the cache key, using format:
//
//	<namespace>|discount-keys-organization
//	<namespace>|discount-keys-store-storeId
//	..etc
//
// The namespace is FIRST and always present. An empty namespace stays distinct
// from a named one rather than silently sharing, so an unnamespaced caller — a
// cron, a test — can never collide with a tenant.
func keyForScope(namespace string, scopeType scope.Type, id string) string {
	key := "discount-keys-"
	keyFmt := key + "%s-%s"
	scopeName := string(scopeType)

	switch scopeType {
	case scope.Organization:
		// No id by definition: the scope IS the org. Before namespacing, that is
		// what made this one global entry shared by every tenant.
		key = key + scopeName
	case scope.Store:
		key = fmt.Sprintf(keyFmt, scopeName, id)
	case scope.Collection:
		key = fmt.Sprintf(keyFmt, scopeName, id)
	case scope.Product:
		key = fmt.Sprintf(keyFmt, scopeName, id)
	case scope.Variant:
		key = fmt.Sprintf(keyFmt, scopeName, id)
	}

	return namespace + "|" + key
}

// Invalidate cache for all keys in matching scope.
//
// The AfterCreate/AfterUpdate/AfterDelete hooks carry no context, so the
// namespace comes from the model. When it is unavailable we cannot compute the
// one exact key, so drop that scope's entry in EVERY namespace: an extra
// datastore read for other tenants, versus continuing to serve a discount the
// merchant just changed. Over-invalidation is a cache miss; under-invalidation is
// a wrong price.
func (d *Discount) invalidateCache() error {
	suffix := keyForScope("", d.Scope.Type, d.ScopeId()) // "|discount-keys-..."

	discountCacheLock.Lock()
	defer discountCacheLock.Unlock()

	if ns := d.Namespace(); ns != "" {
		delete(discountCache, ns+suffix)
		return nil
	}
	for k := range discountCache {
		if strings.HasSuffix(k, suffix) {
			delete(discountCache, k)
		}
	}
	return nil
}

// Cache discount keys under an already-namespaced key.
func cacheDiscounts(ctx context.Context, key string, keys []iface.Key) error {
	discountCacheLock.Lock()
	discountCache[key] = cacheEntry{keys: keys, at: time.Now()}
	discountCacheLock.Unlock()
	return nil
}

// Get cached discount keys, treating an entry older than ttl as a miss.
func getCachedDiscounts(ctx context.Context, key string) ([]iface.Key, error) {
	discountCacheLock.RLock()
	e, ok := discountCache[key]
	discountCacheLock.RUnlock()
	if !ok {
		return nil, fmt.Errorf("cache miss for key: %s", key)
	}
	if time.Since(e.at) > ttl {
		return nil, fmt.Errorf("cache expired for key: %s", key)
	}
	return e.keys, nil
}

func GetScopedDiscounts(ctx context.Context, sc scope.Type, id string, keyc chan []iface.Key, errc chan error) {
	// Id required for all scopes except organization
	if id == "" && sc != scope.Organization {
		// TODO: Prevent this from happening. Usually due to store id missing on order.
		errc <- nil
		keyc <- make([]iface.Key, 0)
		return
	}

	// Check cache for keys. The namespace comes from the SAME ctx the datastore
	// query below reads it from, so a cached value can never describe a different
	// tenant than the query that produced it.
	key := keyForScope(nscontext.GetNamespace(ctx), sc, id)

	log.Debug("Trying to get discounts from cache using key '%s'", key)
	keys, err := getCachedDiscounts(ctx, key)

	// Fetch keys from datastore if that fails
	if err != nil {
		var filter string
		switch sc {
		case scope.Store:
			filter = "Scope.StoreId="
		case scope.Collection:
			filter = "Scope.CollectionId="
		case scope.Product:
			filter = "Scope.ProductId="
		case scope.Variant:
			filter = "Scope.VariantId="
		}

		db := datastore.New(ctx)
		q := Query(db).Filter("Scope.Type=", string(sc))

		if filter != "" {
			q = q.Filter(filter, id)
		}

		if sc == scope.Organization {
			log.Debug("Trying to get discounts from datastore Scope.Type=organization")
		} else {
			log.Debug("Trying to get discounts from datastore Scope.Type=%s, %s%s", sc, filter, id)
		}

		keys, err = q.Filter("Enabled=", true).GetKeys()

		// Cache keys for later
		if err == nil {
			log.Debug("Caching discount keys for later using cache key '%s'", key)
			err = cacheDiscounts(ctx, key, keys)
		}
	}

	// Return with keys
	errc <- err
	keyc <- keys
}
