package discount

import (
	"fmt"

	"google.golang.org/appengine"
	"google.golang.org/appengine/memcache"

	aeds "google.golang.org/appengine/datastore"

	"hanzo.io/datastore"
	"hanzo.io/models/discount/scope"

	"hanzo.io/util/log"
)

// Computes memcache key, using format:
//	discount-keys-organization
//  discount-keys-store-storeId
//  ..etc
func keyForScope(scopeType scope.Type, id string) string {
	key := "discount-keys-"
	keyFmt := key + "%s-%s"
	scopeName := string(scopeType)

	switch scopeType {
	case scope.Organization:
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

	return key
}

// Invalidate cache for all keys in matching scope
func (d *Discount) invalidateCache() error {
	key := keyForScope(d.Scope.Type, d.ScopeId())
	err := memcache.Delete(d.Context(), key)
	if err == memcache.ErrCacheMiss {
		err = nil
	}
	return err
}

// Cache discount keys
func cacheDiscounts(ctx context.Context, key string, keys []*aeds.Key) error {
	return memcache.Gob.Set(ctx,
		&memcache.Item{
			Key:    key,
			Object: keys,
		})
}

// Get cached discount keys
func getCachedDiscounts(ctx context.Context, key string) ([]*aeds.Key, error) {
	keys := make([]*aeds.Key, 0)
	_, err := memcache.Gob.Get(ctx, key, keys)
	if err != nil {
		return keys, err
	}

	return keys, nil
}

func GetScopedDiscounts(ctx context.Context, sc scope.Type, id string, keyc chan []*aeds.Key, errc chan error) {
	// Id required for all scopes except organization
	if id == "" && sc != scope.Organization {
		// TODO: Prevent this from happening. Usually due to store id missing on order.
		errc <- nil
		keyc <- make([]*aeds.Key, 0)
		return
	}

	// Check memcache for keys
	key := keyForScope(sc, id)

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
