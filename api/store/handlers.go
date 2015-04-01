package store

import (
	"crowdstart.io/util/json"
	"github.com/gin-gonic/gin"

	"crowdstart.io/models2/product"
	"crowdstart.io/models2/variant"
)

func GetStorePrice(c *gin.Context) {
	storeId := c.Params.ByName("storeid")
	id := c.Params.ByName("id")
	entity := c.Params.ByName("entity")

	switch entity {
	case product.Product{}.Kind():
		product := productPriceOverride(c, storeId, id)
		c.JSON(200, product)
		return
	case variant.Variant{}.Kind():
		variant := variantPriceOverride(c, storeId, id)
		c.JSON(200, variant)
		return
	}
	json.Fail(c, 500, "Invalid store lookup", nil)
}

func PostStorePrice(c *gin.Context) {
}

func DeleteStorePrice(c *gin.Context) {
}
