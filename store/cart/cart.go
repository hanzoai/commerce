package cart

import (
	"crowdstart.io/datastore"
	"crowdstart.io/middleware"
	"crowdstart.io/models"
	"crowdstart.io/util/template"
	"github.com/gin-gonic/gin"
)

func Get(c *gin.Context) {
	// Just for testing
	db := datastore.New(c)
	ctx := middleware.GetAppEngine(c)

	product := new(models.Product)
	db.GetKey("product", "ar-1", product)
	ctx.Infof("product: %v", product)

	variant := new(models.ProductVariant)
	db.GetKey("variant", "AR-1-BLACK-S", variant)
	ctx.Infof("variant: %v", variant)

	item := models.LineItem{
		Product: *product,
		Variant: *variant,
		Quantity: 1,
	}

	ctx.Infof("item: %v", item)

	template.Render(c, "store/cart.html", "item", item)
}
