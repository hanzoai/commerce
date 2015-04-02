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
