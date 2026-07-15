package fixtures

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/form"
	"github.com/hanzoai/commerce/models/organization"
)

var _ = New("karma-forms", func(c *zip.Ctx) *form.Form {
	db := datastore.New(c.Context())

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
