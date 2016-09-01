package fixtures

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/models"
	"crowdstart.com/models/product"
	"crowdstart.com/models/store"
	"crowdstart.com/models/types/currency"
)

var Store = New("store", func(c *gin.Context) *store.Store {
	// Get namespaced db
	db := getNamespaceDb(c)

	stor := store.New(db)
	stor.Slug = "suchtees"
	stor.GetOrCreate("Slug=", stor.Slug)

	stor.Name = "default"
	stor.Domain = "suchtees.com"
	stor.Prefix = "/"
	stor.Currency = currency.USD
	stor.TaxNexus = []models.Address{models.Address{Line1: "123 Such St", City: "Tee City"}, models.Address{Line1: "456 Noo Ln", City: "Memetown"}}

	// Fetch first product
	prod := Product(c).(*product.Product)
	price := currency.Cents(30000)
	stor.Listings[prod.Id()] = store.Listing{
		ProductId: prod.Id(),
		Price:     &price,
	}

	stor.MustPut()
	return stor
})
