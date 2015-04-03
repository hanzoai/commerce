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
	namespaced := middleware.Namespace()

	api := rest.New(store.Store{})

	// API for getting a full product/variant/bundle for a specific store
	api.GET("/:id/authorize", publishedRequired, namespaced, authorize)
	api.GET("/:id/charge", publishedRequired, namespaced, charge)

	// API for getting a full product/variant/bundle for a specific store
	api.GET("/:id/product/:key", publishedRequired, namespaced, getItem)
	api.GET("/:id/variant/:key", publishedRequired, namespaced, getItem)
	api.GET("/:id/bundle/:key", publishedRequired, namespaced, getItem)

	// API for working with listings directly
	api.GET("/:id/listing", adminRequired, namespaced, listListing)
	api.GET("/:id/listing/:key", adminRequired, namespaced, getListing)
	api.POST("/:id/listing/:key", adminRequired, namespaced, createListing)
	api.PUT("/:id/listing/:key", adminRequired, namespaced, updateListing)
	api.PATCH("/:id/listing/:key", adminRequired, namespaced, patchListing)
	api.DELETE("/:id/listing/:key", adminRequired, namespaced, deleteListing)

	api.Route(router, args...)
}
