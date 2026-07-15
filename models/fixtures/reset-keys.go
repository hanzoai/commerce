package fixtures

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/organization"
)

var _ = New("reset-keys", func(c *zip.Ctx) *organization.Organization {
	db := datastore.New(c.Context())

	org := organization.New(db)
	org.Name = "sec-demo"
	org.GetOrCreate("Name=", org.Name)

	org.AddDefaultTokens()
	org.MustUpdate()

	return org
})
