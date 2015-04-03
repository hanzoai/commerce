package fixtures

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/models2/product"
	"crowdstart.io/models2/store"
	"crowdstart.io/models2/types/currency"
)

var Store = New("store", func(c *gin.Context) *store.Store {
	// Get namespaced db
	db := getNamespaceDb(c)

	stor := store.New(db)
	stor.Slug = "suchtees"
	stor.GetOrCreate("Slug=", stor.Slug)

	stor.Name = "default"
	stor.Hostname = "www.suchtees.com"
	stor.Prefix = "/"
	stor.Currency = currency.USD

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
