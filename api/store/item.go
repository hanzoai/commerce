package store

import (
	"fmt"
	"reflect"

	"github.com/gin-gonic/gin"

	"crowdstart.io/datastore"
	"crowdstart.io/middleware"
	"crowdstart.io/models/mixin"
	"crowdstart.io/models2/bundle"
	"crowdstart.io/models2/product"
	"crowdstart.io/models2/store"
	"crowdstart.io/models2/variant"
	"crowdstart.io/util/json"
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

	return func(c *gin.Context) {
		ctx := middleware.GetAppEngine(c)
		db := datastore.New(ctx)
		id := c.Params.ByName("storeid")
		key := c.Params.ByName("key")

		// Get store
		stor := store.New(db)
		if err := stor.GetById(id); err != nil {
			json.Fail(c, 500, fmt.Sprintf("Failed to retrieve store '%v': %v", id, err), err)
			return
		}

		// Create new entity instance
		entity := reflect.New(typ).Interface().(mixin.Entity)
		model := mixin.Model{Db: db, Entity: entity}
		field := reflect.Indirect(reflect.ValueOf(entity)).FieldByName("Model")
		field.Set(reflect.ValueOf(model))

		// Try to get entity using key
		if err := entity.GetById(key); err != nil {
			json.Fail(c, 500, fmt.Sprintf("Failed to retrieve '%s' using '%s': %v", itemType, key, err), err)
			return
		}

		// Update product/variant using listing for said item
		stor.UpdateFromListing(entity)

		c.JSON(200, entity)
	}
}
