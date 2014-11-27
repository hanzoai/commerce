package products

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/config"
	"crowdstart.io/datastore"
	"crowdstart.io/models"
	"crowdstart.io/util/json"
	"crowdstart.io/util/log"
	"crowdstart.io/util/template"
)

func List(c *gin.Context) {
	db := datastore.New(c)
	var products []models.Product

	_, err := db.Query("product").GetAll(db.Context, &products)
	if err != nil {
		// Something is seriously wrong. i.e. products not loaded into db
		log.Panic(err.Error())
		return
	}

	// Create map of slug -> product
	productsMap := make(map[string]models.Product)
	for _, product := range products {
		productsMap[product.Slug] = product
	}
	productsJSON := json.Encode(productsMap)

	template.Render(c, "list.html",
		"products", products,
		"productsJSON", productsJSON,
	)
}

func Get(c *gin.Context) {
	c.Redirect(301, config.ModuleUrl("store"))

	// Redirec to store cuz SKULLY.
	// db := datastore.New(c)
	// slug := c.Params.ByName("slug")

	// product := new(models.Product)
	// err := db.GetKey("product", slug, product)
	// if err != nil { // Invalid slug
	// 	log.Error(err.Error())
	// 	template.Render(c, "error/404.html")
	// 	return
	// }

	// template.Render(c, "product.html", "product", product)
}
