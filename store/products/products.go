package products

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/datastore"
	"crowdstart.io/models"
	"crowdstart.io/util/log"
	"crowdstart.io/util/template"
)

func List(c *gin.Context) {
	db := datastore.New(c)
	var products []models.Product

	_, err := db.Query("product").GetAll(db.Context, &products)
	if err != nil {
		log.Panic(err.Error())
	}

	template.Render(c, "list.html", "products", products)
}

func Get(c *gin.Context) {
	db := datastore.New(c)
	slug := c.Params.ByName("slug")

	product := new(models.Product)
	db.GetKey("product", slug, product)

	template.Render(c, "product.html", "product", product)
}

func Store(c *gin.Context) {
	db := datastore.New(c)
	var products []models.Product
	db.Query("product").GetAll(db.Context, &products)

	template.Render(c, "store.html", "products", products)
}
