package discount

import (
	"fmt"

	"appengine/memcache"

	"crowdstart.com/models/discount/scope"
)

// Computes memcache key, using format:
//	discount-keys-organization
//  discount-keys-store-storeId
//  ..etc
func KeyForScope(scopeType scope.Type, id string) string {
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
	key := KeyForScope(d.Scope.Type, d.ScopeId())
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
