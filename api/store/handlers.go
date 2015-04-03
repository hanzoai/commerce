package store

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/middleware"
	"crowdstart.io/models2/store"
	"crowdstart.io/util/permission"
	"crowdstart.io/util/rest"
	"crowdstart.io/util/router"
)

func Route(router router.Router, args ...gin.HandlerFunc) {
	allowAll := middleware.AccessControl("*")
	adminRequired := middleware.TokenRequired(permission.Admin)
	publishedRequired := middleware.TokenRequired(permission.Admin, permission.Published)
	namespaced := middleware.Namespace()

	admin := []gin.HandlerFunc{allowAll, adminRequired, namespaced}
	published := []gin.HandlerFunc{allowAll, publishedRequired, namespaced}

	api := rest.New(store.Store{})

	// API for getting a full product/variant/bundle for a specific store
	api.GET("/:id/authorize", append(published, authorize)...)
	api.GET("/:id/charge", append(published, charge)...)

	// API for getting a full product/variant/bundle for a specific store
	api.GET("/:id/product/:key", append(published, getItem)...)
	api.GET("/:id/variant/:key", append(published, getItem)...)
	api.GET("/:id/bundle/:key", append(published, getItem)...)

	// API for working with listings directly
	api.GET("/:id/listing", append(admin, listListing)...)
	api.GET("/:id/listing/:key", append(admin, getListing)...)
	api.POST("/:id/listing/:key", append(admin, createListing)...)
	api.PUT("/:id/listing/:key", append(admin, updateListing)...)
	api.PATCH("/:id/listing/:key", append(admin, patchListing)...)
	api.DELETE("/:id/listing/:key", append(admin, deleteListing)...)

	api.Route(router, args...)
}
