package cart

import (
	"crowdstart.io/datastore"
	"crowdstart.io/models"
	"crowdstart.io/util/template"
	"github.com/gin-gonic/gin"
)

func Get(c *gin.Context) {
	// Just for testing
	db := datastore.New(c)

	product := new(models.Product)
	db.GetKey("product", "ar-1", product)

	variant := new(models.ProductVariant)
	db.GetKey("variant", "AR1-BLACK-S", variant)

	item := models.LineItem{
		Product: *product,
		Variant: *variant,
	}

	template.Render(c, "store/cart.html", "item", item)
}
