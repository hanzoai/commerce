package fixtures

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/models2/product"
)

func Product(c *gin.Context) *product.Product {
	// Get namespaced db
	db := getDb(c)

	prod := product.New(db)
	prod.Slug = "t-shirt"
	prod.GetOrCreate("Slug=", prod.Slug)
	prod.Name = "Such T-shirt"
	prod.Headline = "wow  such shirt  much tee"
	prod.Description = `wow
	　　　　　　such shirt
	much tee

			nice shop

	　so hip

	　　　　so doge
	`

	prod.Options = []product.Option{
		product.Option{
			Name:   "Size",
			Values: []string{"Much", "Wow"},
		},
	}

	for _, variant := range Variant(c) {
		prod.Variants = append(prod.Variants, *variant)
	}
	err := prod.Put()
	if err != nil {
		panic(err)
	}

	return prod
}
