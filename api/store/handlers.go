package store

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/models2/store"
	"crowdstart.io/util/rest"
	"crowdstart.io/util/router"
)

// func GetStorePrice(c *gin.Context) {
// 	storeId := c.Params.ByName("storeid")
// 	id := c.Params.ByName("id")
// 	entity := c.Params.ByName("entity")

// 	switch entity {
// 	case product.Product{}.Kind():
// 		product := productPriceOverride(c, storeId, id)
// 		c.JSON(200, product)
// 		return
// 	case variant.Variant{}.Kind():
// 		variant := variantPriceOverride(c, storeId, id)
// 		c.JSON(200, variant)
// 		return
// 	}
// 	json.Fail(c, 500, "Invalid store lookup", nil)
// }

// func PostStorePrice(c *gin.Context) {
// }

// func DeleteStorePrice(c *gin.Context) {
// }

func Route(router router.Router, args ...gin.HandlerFunc) {
	api := rest.New(store.Store{})

	// api.GET("/:id/product/:productid", adminRequired, store.GetStorePrice)
	// api.GET("/:id/bundle/:bundleid", adminRequired, store.GetStorePrice)
	// api.GET("/:id/variant/:variantid", adminRequired, store.GetStorePrice)

	// api.GET("/:id/listings", adminRequired, store.GetStorePrice)
	// api.POST("/:id/listings", adminRequired, store.GetStorePrice)
	// api.PUT("/:id/listings", adminRequired, store.GetStorePrice)
	// api.PATCH("/:id/listings", adminRequired, store.GetStorePrice)
	// api.DELETE("/:id/listings", adminRequired, store.GetStorePrice)

	api.Route(router, args...)
}
