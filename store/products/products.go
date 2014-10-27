package products

import (
	"github.com/gin-gonic/gin"
	"crowdstart.io/util/template"
	"crowdstart.io/datastore"
	"crowdstart.io/models"
)

func List(c *gin.Context) {
	template.Render(c, "store/list.html")
}

func Get(c *gin.Context) {
	db   := datastore.New(c)
	slug := c.Params.ByName("slug")

	product := new(models.Product)
	db.GetKey("product", slug, product)

	template.Render(c, "store/product.html", "product", product)
}
