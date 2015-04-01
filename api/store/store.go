package store

import (
	"fmt"
	"reflect"

	"github.com/gin-gonic/gin"

	"crowdstart.io/datastore"
	"crowdstart.io/middleware"
	"crowdstart.io/models/mixin"
	"crowdstart.io/models2/product"
	"crowdstart.io/models2/store"
	"crowdstart.io/models2/variant"
	"crowdstart.io/util/json"
	"crowdstart.io/util/permission"
	"crowdstart.io/util/rest"
	"crowdstart.io/util/router"
)

var entityTypes = map[string]reflect.Type{
	"product": reflect.ValueOf(product.Product{}).Type(),
	"variant": reflect.ValueOf(variant.Variant{}).Type(),
}

func getOverride(c *gin.Context) {
	storeid := c.Params.ByName("id")
	entityId := c.Params.ByName("entityid")
	kind := c.Params.ByName("kind")

	db := datastore.New(c)

	stor := store.New(db)
	if err := stor.Get(storeid); err != nil {
		json.Fail(c, 500, fmt.Sprintf("Failed to retrieve store '%v': %v", storeid, err), err)
		return
	}

	entity := reflect.New(entityTypes[kind]).Interface().(mixin.Entity)
	model := mixin.Model{Db: db, Entity: entity}
	field := reflect.Indirect(reflect.ValueOf(entity)).FieldByName("Model")
	field.Set(reflect.ValueOf(model))

	if err := entity.Get(entityId); err != nil {
		json.Fail(c, 500, fmt.Sprintf("Failed to retrieve %s %s: %v", kind, entityId, err), err)
		return
	}

	// Override product with store customizations
	stor.Override(entity)

	c.JSON(200, entity)
}

func Route(router router.Router, args ...gin.HandlerFunc) {
	publishedRequired := middleware.TokenRequired(permission.Admin, permission.Published)

	api := rest.New(store.Store{})

	api.GET("/:id/:kind/:entityid", publishedRequired, rest.NamespacedMiddleware, getOverride)
	// api.GET("/:id/bundle/:bundleid", adminRequired, store.GetStorePrice)

	// api.GET("/:id/listings", adminRequired, store.GetStorePrice)
	// api.POST("/:id/listings", adminRequired, store.GetStorePrice)
	// api.PUT("/:id/listings", adminRequired, store.GetStorePrice)
	// api.PATCH("/:id/listings", adminRequired, store.GetStorePrice)
	// api.DELETE("/:id/listings", adminRequired, store.GetStorePrice)

	api.Route(router, args...)
}
