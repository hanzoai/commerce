package fixtures

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/models/form"
	"hanzo.io/models/organization"
)

var _ = New("damon-forms", func(c *gin.Context) *form.Form {
	db := datastore.New(c)

	org := organization.New(db)
	org.Name = "damon"
	org.GetOrCreate("Name=", org.Name)

	nsdb := datastore.New(org.Namespaced(db.Context))

	// Create mailinglist
	f := form.New(nsdb)
	f.Name = "Preorders"
	f.GetOrCreate("Name=", f.Name)
	f.Mailchimp.APIKey = ""
	f.MustUpdate()

	// Create mailinglist
	f2 := form.New(nsdb)
	f2.Name = "Newsletter"
	f2.GetOrCreate("Name=", f.Name)
	f2.Mailchimp.APIKey = ""
	f2.Mailchimp.ListId = "aacc13e678"
	f2.MustUpdate()

	return f
})
