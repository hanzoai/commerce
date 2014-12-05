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
	// Temporary redirect while store is under construction.
	if c.MustGet("host") == "store.skullysystems.com" {
		c.Redirect(301, "http://www.skullysystems.com/store")
		return
	}

	db := datastore.New(c)
	var products []models.Product

	_, err := db.Query("product").GetAll(db.Context, &products)
	if err != nil {
		// Something is seriously wrong. i.e. products not loaded into db
		log.Panic("Unable to fetch all products from database: %v", err)
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
	// Redirect to store cuz SKULLY.
	c.Redirect(301, config.UrlFor("store"))
	return

	db := datastore.New(c)
	slug := c.Params.ByName("slug")

	product := new(models.Product)
	if err := db.GetKey("product", slug, product); err != nil {
		log.Error("Invalid slug: %v", err)
		template.Render(c, "error/404.html")
		return
	}

	template.Render(c, "product.html", "product", product)
}
