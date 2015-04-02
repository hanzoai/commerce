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
	"crowdstart.io/util/permission"
	"crowdstart.io/util/rest"
	"crowdstart.io/util/router"
)

// Return all listings
func getListings(c *gin.Context) {
	storeid := c.Params.ByName("id")

	db := datastore.New(c)

	stor := store.New(db)
	if err := stor.Get(storeid); err != nil {
		json.Fail(c, 500, fmt.Sprintf("Failed to retrieve store '%v': %v", storeid, err), err)
		return
	}

	c.JSON(200, stor.Listings)
}

func createListings(c *gin.Context) {
	storeid := c.Params.ByName("id")

	db := datastore.New(c)

	stor := store.New(db)
	if err := stor.Get(storeid); err != nil {
		json.Fail(c, 500, fmt.Sprintf("Failed to retrieve store '%v': %v", storeid, err), err)
		return
	}

	// Zero out listings for clean create
	stor.Listings = make(store.Listings)

	// Decode response body to create new listings
	if err := json.Decode(c.Request.Body, stor.Listings); err != nil {
		json.Fail(c, 400, "Failed decode request body", err)
		return
	}

	// Try to save store
	if err := stor.Put(); err != nil {
		json.Fail(c, 500, "Failed to save store listings", err)
	} else {
		c.Writer.Header().Add("Location", c.Request.URL.Path)
		c.JSON(201, stor.Listings)
	}
}

func patchListings(c *gin.Context) {
	storeid := c.Params.ByName("id")

	db := datastore.New(c)

	stor := store.New(db)
	if err := stor.Get(storeid); err != nil {
		json.Fail(c, 500, fmt.Sprintf("Failed to retrieve store '%v': %v", storeid, err), err)
		return
	}

	// Decode response body to update listings
	if err := json.Decode(c.Request.Body, stor.Listings); err != nil {
		json.Fail(c, 400, "Failed decode request body", err)
		return
	}

	// Try to save store
	if err := stor.Put(); err != nil {
		json.Fail(c, 500, "Failed to save store listings", err)
	} else {
		c.JSON(200, stor.Listings)
	}
}

func deleteListings(c *gin.Context) {
	storeid := c.Params.ByName("id")

	db := datastore.New(c)

	stor := store.New(db)
	if err := stor.Get(storeid); err != nil {
		json.Fail(c, 500, fmt.Sprintf("Failed to retrieve store '%v': %v", storeid, err), err)
		return
	}

	// Zero out listings for clean create
	stor.Listings = make(store.Listings)

	// Try to save store
	if err := stor.Put(); err != nil {
		json.Fail(c, 500, "Failed to save store listings", err)
	} else {
		c.Data(204, "application/json", make([]byte, 0))
	}
}

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

func Route(router router.Router, args ...gin.HandlerFunc) {
	adminRequired := middleware.TokenRequired(permission.Admin)
	publishedRequired := middleware.TokenRequired(permission.Admin, permission.Published)

	api := rest.New(store.Store{})

	// API for manipulating listings per store
	api.GET("/:id/listings", adminRequired, getListings)
	api.POST("/:id/listings", adminRequired, createListings)
	api.PUT("/:id/listings", adminRequired, createListings)
	api.PATCH("/:id/listings", adminRequired, patchListings)
	api.DELETE("/:id/listings", adminRequired, deleteListings)

	// API for getting listing per product/variant
	api.GET("/:id/product/:entityid", publishedRequired, rest.NamespacedMiddleware, getListing("product"))
	api.GET("/:id/variant/:entityid", publishedRequired, rest.NamespacedMiddleware, getListing("variant"))
	api.GET("/:id/bundle/:entityid", publishedRequired, rest.NamespacedMiddleware, getListing("bundle"))

	api.Route(router, args...)
}
