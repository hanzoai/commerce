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

	// Something is seriously wrong. i.e. products not loaded into db
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
	err := db.GetKey("product", slug, product)
	if err != nil { // Invalid slug
		log.Error(err.Error())
		template.Render(c, "error/404.html")
		return
	}

	template.Render(c, "product.html", "product", product)
}

func Store(c *gin.Context) {
	db := datastore.New(c)
	var products []models.Product
	db.Query("product").GetAll(db.Context, &products)

	template.Render(c, "store.html", "products", products)
}
