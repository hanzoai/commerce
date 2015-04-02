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
	adminRequired := middleware.TokenRequired(permission.Admin)
	publishedRequired := middleware.TokenRequired(permission.Admin, permission.Published)

	api := rest.New(store.Store{})

	// API for getting a full product/variant/bundle for a specific store
	api.GET("/:id/authorize", publishedRequired, rest.NamespacedMiddleware, authorize)
	api.GET("/:id/charge", publishedRequired, rest.NamespacedMiddleware, charge)

	// API for getting a full product/variant/bundle for a specific store
	api.GET("/:id/product/:key", publishedRequired, rest.NamespacedMiddleware, getItem)
	api.GET("/:id/variant/:key", publishedRequired, rest.NamespacedMiddleware, getItem)
	api.GET("/:id/bundle/:key", publishedRequired, rest.NamespacedMiddleware, getItem)

	// API for working with listings directly
	api.GET("/:id/listing", adminRequired, rest.NamespacedMiddleware, listListing)
	api.GET("/:id/listing/:key", adminRequired, rest.NamespacedMiddleware, getListing)
	api.POST("/:id/listing/:key", adminRequired, rest.NamespacedMiddleware, createListing)
	api.PUT("/:id/listing/:key", adminRequired, rest.NamespacedMiddleware, updateListing)
	api.PATCH("/:id/listing/:key", adminRequired, rest.NamespacedMiddleware, patchListing)
	api.DELETE("/:id/listing/:key", adminRequired, rest.NamespacedMiddleware, deleteListing)

	api.Route(router, args...)
}
