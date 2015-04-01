package fixtures

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/models2/store"
	"crowdstart.io/models2/types/currency"
)

func Store(c *gin.Context) *store.Store {
	// Get namespaced db
	db := getDb(c)

	stor := store.New(db)
	stor.Name = "default"
	stor.Slug = "suchtees"
	stor.Hostname = "www.suchtees.com"
	stor.Prefix = "/"
	stor.Currency = currency.USD

	// Fetch first product

	prod := Store(c)
	stor.Listings[prod.Id()] = store.Listing{
		ProductId: prod.Id(),
		Price:     currency.Cents(30000),
	}

	stor.MustPut()
	return stor
}
