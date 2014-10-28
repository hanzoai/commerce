package products

import (
	"crowdstart.io/datastore"
	"crowdstart.io/middleware"
	"crowdstart.io/models"
	"crowdstart.io/util/template"
	"github.com/gin-gonic/gin"
)

func List(c *gin.Context) {
	db := datastore.New(c)
	var products []models.Product
	db.Query("product").GetAll(db.Context, &products)

	ctx := middleware.GetAppEngine(c)
	ctx.Infof("%v", products)

	template.Render(c, "store/list.html", "products", products)
}

func Get(c *gin.Context) {
	db := datastore.New(c)
	slug := c.Params.ByName("slug")

	product := new(models.Product)
	db.GetKey("product", slug, product)

	template.Render(c, "store/product.html", "product", product)
}
