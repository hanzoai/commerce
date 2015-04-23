package products

import "github.com/gin-gonic/gin"

func List(c *gin.Context) {
	// db := datastore.New(c)

	// listings := make([]*models.Listing, 1)
	// listings[0] = new(models.Listing)

	// if err := db.GetKindMulti("listing", []string{"ar-1"}, listings); err != nil {
	// 	// Something is seriously wrong. i.e. products not loaded into db
	// 	log.Panic("Unable to fetch product listing from database: %v", err)
	// }

	// listingsMap := make(map[string]*models.Listing)
	// for _, listing := range listings {
	// 	listingsMap[listing.SKU] = listing
	// }
	// listingsJSON := json.Encode(listingsMap)

	// slugs := listings[0].GetProductSlugs()
	// products := make([]*models.Product, len(slugs))
	// for i, _ := range slugs {
	// 	products[i] = new(models.Product)
	// }

	// if err := db.GetKindMulti("product", slugs, products); err != nil {
	// 	// Something is seriously wrong. i.e. products not loaded into db
	// 	log.Panic("Unable to fetch all products from database: %v", err)
	// }

	// // Create map of slug -> product
	// productsMap := make(map[string]*models.Product)
	// for _, product := range products {
	// 	productsMap[product.Slug] = product
	// }
	// productsJSON := json.Encode(productsMap)

	// template.Render(c, "list.html",
	// 	"listings", listings,
	// 	"listingsJSON", listingsJSON,
	// 	"products", products,
	// 	"productsJSON", productsJSON,
	// )
}

func Get(c *gin.Context) {
	// // Redirect to store cuz SKULLY.
	// c.Redirect(301, config.UrlFor("store"))
	// return

	// db := datastore.New(c)
	// slug := c.Params.ByName("slug")

	// product := new(models.Product)
	// if err := db.GetKind("product", slug, product); err != nil {
	// 	log.Error("Invalid slug: %v", err)
	// 	template.Render(c, "error/404.html")
	// 	return
	// }

	// template.Render(c, "product.html", "product", product)
}
