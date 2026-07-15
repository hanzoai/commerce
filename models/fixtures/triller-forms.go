package fixtures

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/form"
	"github.com/hanzoai/commerce/models/organization"
)

var _ = New("triller-forms", func(c *zip.Ctx) *form.Form {
	db := datastore.New(c.Context())

	org := organization.New(db)
	org.Name = "triller"
	org.GetOrCreate("Name=", org.Name)

	nsdb := datastore.New(org.Namespaced(db.Context))

	// Create mailinglist
	// f := form.New(nsdb)
	// f.Name = "Preorders"
	// f.GetOrCreate("Name=", f.Name)
	// f.Mailchimp.APIKey = ""
	// f.MustUpdate()

	// Create mailinglist
	f2 := form.New(nsdb)
	f2.Name = "Newsletter"
	f2.GetOrCreate("Name=", f2.Name)
	// f2.Mailchimp.APIKey = ""
	// f2.Mailchimp.ListId = "aacc13e678"
	f2.MustUpdate()

	// Create mailinglist
	f3 := form.New(nsdb)
	f3.Name = "StepUp Newsletter"
	f3.GetOrCreate("Name=", f3.Name)
	// f2.Mailchimp.APIKey = ""
	// f2.Mailchimp.ListId = "aacc13e678"
	f3.MustUpdate()

	return f2
})
