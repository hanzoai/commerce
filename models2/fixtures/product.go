package fixtures

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/datastore"
	"crowdstart.io/models2/organization"
	"crowdstart.io/models2/product"
	"crowdstart.io/util/task"
)

var _ = task.Func("models2-fixtures-product", func(c *gin.Context) {
	db := datastore.New(c)

	org := organization.New(db)
	org.Name = "suchtees"
	org.GetOrCreate("Name=", org.Name)

	// Use org's namespace
	ctx := org.Namespace(c)
	db = datastore.New(ctx)

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

	variants := createVariants(db, prod)
	for _, variant := range variants {
		prod.Variants = append(prod.Variants, *variant)
	}
	prod.Put()
})
