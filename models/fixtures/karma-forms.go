package fixtures

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/models/form"
	"hanzo.io/models/organization"
)

var _ = New("karma-forms", func(c *gin.Context) *form.Form {
	db := datastore.New(c)

	org := organization.New(db)
	org.Name = "karma"
	org.GetOrCreate("Name=", org.Name)

	nsdb := datastore.New(org.Namespaced(db.Context))

	// Create mailinglist
	f := form.New(nsdb)
	f.Name = "Preorders"
	f.GetOrCreate("Name=", f.Name)
	// f.Mailchimp.APIKey = ""
	f.MustUpdate()

	return f
})
