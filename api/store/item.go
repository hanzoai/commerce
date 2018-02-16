package store

import (
	"fmt"
	"reflect"

	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/middleware"
	"hanzo.io/models/bundle"
	"hanzo.io/models/mixin"
	"hanzo.io/models/product"
	"hanzo.io/models/store"
	"hanzo.io/models/variant"
	"hanzo.io/util/json/http"
)

var types = map[string]reflect.Type{
	"bundle":  reflect.ValueOf(bundle.Bundle{}).Type(),
	"product": reflect.ValueOf(product.Product{}).Type(),
	"variant": reflect.ValueOf(variant.Variant{}).Type(),
}

// Return product/variant updated against store listing
func getItem(itemType string) gin.HandlerFunc {

	// Get underlying type
	typ, ok := types[itemType]
	if !ok {
		panic("Unable to get listing with item of that type.")
	}

	return func(c *context.Context) {
		ctx := middleware.GetAppEngine(c)
		db := datastore.New(ctx)
		id := c.Params.ByName("storeid")
		key := c.Params.ByName("key")

		// Get store
		stor := store.New(db)
		if err := stor.GetById(id); err != nil {
			http.Fail(c, 404, fmt.Sprintf("Failed to retrieve store '%v': %v", id, err), err)
			return
		}

		// Create new entity instance
		entity := reflect.New(typ).Interface().(mixin.Entity)
		model := mixin.Model{Db: db, Entity: entity}
		field := reflect.Indirect(reflect.ValueOf(entity)).FieldByName("Model")
		field.Set(reflect.ValueOf(model))

		// Try to get entity using key
		if err := entity.GetById(key); err != nil {
			http.Fail(c, 404, fmt.Sprintf("Failed to retrieve '%s' using '%s': %v", itemType, key, err), err)
			return
		}

		// Update product/variant using listing for said item
		stor.UpdateFromListing(entity)

		http.Render(c, 200, entity)
	}
}
