package discount

import (
	"fmt"

	"appengine"
	"appengine/memcache"

	aeds "appengine/datastore"

	"crowdstart.com/datastore"
	"crowdstart.com/models/discount/scope"

	"crowdstart.com/util/log"
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
func cacheDiscounts(ctx appengine.Context, key string, keys []*aeds.Key) error {
	return memcache.Gob.Set(ctx,
		&memcache.Item{
			Key:    key,
			Object: keys,
		})
}

// Get cached discount keys
func getCachedDiscounts(ctx appengine.Context, key string) ([]*aeds.Key, error) {
	keys := make([]*aeds.Key, 0)
	_, err := memcache.Gob.Get(ctx, key, keys)
	if err != nil {
		return keys, err
	}

	return keys, nil
}

func GetScopedDiscounts(ctx appengine.Context, sc scope.Type, id string, keyc chan []*aeds.Key, errc chan error) {
	// Check memcache for keys
	key := keyForScope(sc, id)
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

		query := Query(datastore.New(ctx)).Filter("Scope.Type=", string(sc))

		if filter != "" {
			query = query.Filter(filter, id)
		}

		keys, err = query.
			Filter("Enabled=", true).
			KeysOnly().
			GetAll(nil)

		// Cache keys for later
		if err != nil {
			err = cacheDiscounts(ctx, key, keys)
		}

		log.Error("sc = %v, id = %v, keys = %v", sc, id, keys)
	}

	// Return with keys
	errc <- err
	keyc <- keys
}
