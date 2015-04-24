package store

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/middleware"
	"crowdstart.io/models/store"
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
	api.POST("/:storeid/authorize", publishedRequired, namespaced, authorize)
	api.POST("/:storeid/charge", publishedRequired, namespaced, charge)

	// API for getting a full product/variant/bundle for a specific store
	api.GET("/:storeid/bundle/:key", publishedRequired, namespaced, getItem("bundle"))
	api.GET("/:storeid/product/:key", publishedRequired, namespaced, getItem("product"))
	api.GET("/:storeid/variant/:key", publishedRequired, namespaced, getItem("variant"))

	// API for working with listings directly
	api.GET("/:storeid/listing", adminRequired, namespaced, listListing)
	api.GET("/:storeid/listing/:key", adminRequired, namespaced, getListing)
	api.POST("/:storeid/listing/:key", adminRequired, namespaced, createListing)
	api.PUT("/:storeid/listing/:key", adminRequired, namespaced, updateListing)
	api.PATCH("/:storeid/listing/:key", adminRequired, namespaced, patchListing)
	api.DELETE("/:storeid/listing/:key", adminRequired, namespaced, deleteListing)

	api.Route(router, args...)
}
