package fixtures

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/datastore"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/store"
	"crowdstart.com/models/types/currency"
)

var _ = New("kanoa-mailchimp", func(c *gin.Context) *organization.Organization {
	db := datastore.New(c)

	org := organization.New(db)
	org.Query().Filter("Name=", "kanoa").First()
	org.Mailchimp.APIKey = ""

	nsdb := datastore.New(org.Namespaced(db.Context))

	// Create new store
	stor := store.New(nsdb)
	stor.Name = "default"
	stor.Prefix = "/"
	stor.Currency = currency.USD
	stor.Mailchimp.ListId = "23ad4e4ba4"
	stor.Create()

	org.DefaultStore = stor.Id()
	org.Update()

	// // Fetch earphones
	// prod := product.New(db)
	// prod.Query().Filter("Slug=", "earphone").First()

	// // Create corresponding Mailchimp entities
	// client := mailchimp.New(db.Context, org.Mailchimp.APIKey)
	// client.CreateStore(stor)
	// client.CreateProduct(stor.Id(), prod)

	return org
})
