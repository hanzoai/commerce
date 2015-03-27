package fixtures

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/models2/product"
	"crowdstart.io/models2/types/currency"
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

	// Add options
	opt := product.Option{
		Name:   "Size",
		Values: []string{"Much", "Wow"},
	}
	prod.Options = append(prod.Options, &opt)
	prod.Price = 2000
	prod.Currency = currency.USD

	prod.MustPut()

	return prod
}
