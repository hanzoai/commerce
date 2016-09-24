package discount

import (
	"fmt"

	"appengine/memcache"

	"crowdstart.com/models/discount/scope"
)

// Invalidate cache for all keys in matching scope
func (d *Discount) invalidateCache() error {
	// Key format:
	//	discount-keys-organization
	//  discount-keys-store-storeId
	//  ..etc
	key := d.Kind() + "-keys-"
	keyFmt := key + "%s-%s"
	scopeName := string(d.Scope.Type)

	switch d.Scope.Type {
	case scope.Organization:
		key = key + scopeName
	case scope.Store:
		key = fmt.Sprintf(keyFmt, scopeName, d.Scope.StoreId)
	case scope.Collection:
		key = fmt.Sprintf(keyFmt, scopeName, d.Scope.CollectionId)
	case scope.Product:
		key = fmt.Sprintf(keyFmt, scopeName, d.Scope.ProductId)
	case scope.Variant:
		key = fmt.Sprintf(keyFmt, scopeName, d.Scope.VariantId)
	}

	return memcache.Delete(d.Context(), key)
}

// Invalidate cache based on scope
func (d *Discount) AfterCreate() error {
	return d.invalidateCache()
}

func (d *Discount) AfterUpdate(previous *Discount) error {
	return d.invalidateCache()
}

func (d *Discount) AfterDelete() error {
	return d.invalidateCache()
}
