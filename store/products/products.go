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

	productListings := make([]*models.ProductListing, 1)
	productListings[0] = new(models.ProductListing)

	if err := db.GetKeyMulti("productlisting", []string{"ar-1-winter2014promo"}, productListings); err != nil {
		// Something is seriously wrong. i.e. products not loaded into db
		log.Panic("Unable to fetch product listing from database: %v", err)
	}

	productListingsMap := make(map[string]*models.ProductListing)
	for _, productListing := range productListings {
		productListingsMap[productListing.Slug] = productListing
	}
	productListingsJSON := json.Encode(productListingsMap)

	slugs := productListings[0].GetProductSlugs()
	products := make([]*models.Product, len(slugs))
	for i, _ := range slugs {
		products[i] = new(models.Product)
	}

	if err := db.GetKeyMulti("product", slugs, products); err != nil {
		// Something is seriously wrong. i.e. products not loaded into db
		log.Panic("Unable to fetch all products from database: %v", err)
	}

	// Create map of slug -> product
	productsMap := make(map[string]*models.Product)
	for _, product := range products {
		productsMap[product.Slug] = product
	}
	productsJSON := json.Encode(productsMap)

	template.Render(c, "list.html",
		"productListings", productListings,
		"productListingsJSON", productListingsJSON,
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
