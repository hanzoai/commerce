package store

import (
	"fmt"
	"reflect"

	"github.com/gin-gonic/gin"

	"crowdstart.io/datastore"
	"crowdstart.io/models/mixin"
	"crowdstart.io/models2/bundle"
	"crowdstart.io/models2/product"
	"crowdstart.io/models2/store"
	"crowdstart.io/models2/variant"
	"crowdstart.io/util/json"
)

// Return store listing for given product/variant
func getListing(kind string) gin.HandlerFunc {
	var entityType reflect.Type

	switch kind {
	case "product":
		entityType = reflect.ValueOf(product.Product{}).Type()
	case "variant":
		entityType = reflect.ValueOf(variant.Variant{}).Type()
	case "bundle":
		entityType = reflect.ValueOf(bundle.Bundle{}).Type()
	}

	return func(c *gin.Context) {
		storeid := c.Params.ByName("id")
		entityId := c.Params.ByName("entityid")

		db := datastore.New(c)

		stor := store.New(db)
		if err := stor.Get(storeid); err != nil {
			json.Fail(c, 500, fmt.Sprintf("Failed to retrieve store '%v': %v", storeid, err), err)
			return
		}

		entity := reflect.New(entityType).Interface().(mixin.Entity)
		model := mixin.Model{Db: db, Entity: entity}
		field := reflect.Indirect(reflect.ValueOf(entity)).FieldByName("Model")
		field.Set(reflect.ValueOf(model))

		if err := entity.Get(entityId); err != nil {
			json.Fail(c, 500, fmt.Sprintf("Failed to retrieve %s %s: %v", kind, entityId, err), err)
			return
		}

		// Update product/variant using listing for said item
		stor.UpdateFromListing(entity)

		c.JSON(200, entity)
	}
}
