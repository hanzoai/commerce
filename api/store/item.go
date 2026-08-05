package store

import (
	"fmt"
	"reflect"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/bundle"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/models/product"
	"github.com/hanzoai/commerce/models/store"
	"github.com/hanzoai/commerce/models/variant"
	"github.com/hanzoai/commerce/util/json/http"
)

var types = map[string]reflect.Type{
	"bundle":  reflect.ValueOf(bundle.Bundle{}).Type(),
	"product": reflect.ValueOf(product.Product{}).Type(),
	"variant": reflect.ValueOf(variant.Variant{}).Type(),
}

// Return product/variant updated against store listing
func getItem(itemType string) zip.Handler {

	// Get underlying type
	typ, ok := types[itemType]
	if !ok {
		panic("Unable to get listing with item of that type.")
	}

	return func(c *zip.Ctx) error {
		ctx := c.Context()
		db := datastore.New(ctx)
		id := c.Param("storeid")
		key := c.Param("key")

		// Get store
		stor := store.New(db)
		if err := stor.GetById(id); err != nil {
			return http.Fail(c, 404, fmt.Sprintf("Failed to retrieve store '%v': %v", id, err), err)
		}

		// Create new entity instance
		entity := reflect.New(typ).Interface().(mixin.Entity)
		model := mixin.BaseModel{Db: db, Entity: entity}
		field := reflect.Indirect(reflect.ValueOf(entity)).FieldByName("Model")
		field.Set(reflect.ValueOf(model))

		// Try to get entity using key
		if err := entity.GetById(key); err != nil {
			return http.Fail(c, 404, fmt.Sprintf("Failed to retrieve '%s' using '%s': %v", itemType, key, err), err)
		}

		// Update product/variant using listing for said item
		stor.UpdateFromListing(entity)

		return http.Render(c, 200, entity)
	}
}
