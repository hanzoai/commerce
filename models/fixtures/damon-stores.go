package fixtures

import (
	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/store"

	"github.com/hanzoai/commerce/models/types/currency"
)

var _ = New("damon-stores", func(c *gin.Context) *store.Store {
	db := datastore.New(c)

	org := organization.New(db)
	org.Name = "damon"
	org.GetOrCreate("Name=", org.Name)

	nsdb := datastore.New(org.Namespaced(db.Context))

	{
		stor := store.New(nsdb)
		stor.Slug = "eur-store"
		stor.GetOrCreate("Slug=", stor.Slug)

		stor.Name = "EUR Store"
		stor.Currency = currency.EUR

		stor.MustUpdate()
	}

	{
		stor := store.New(nsdb)
		stor.Slug = "gbp-store"
		stor.GetOrCreate("Slug=", stor.Slug)

		stor.Name = "GBP Store"
		stor.Currency = currency.GBP

		stor.MustUpdate()
		return stor
	}
})
