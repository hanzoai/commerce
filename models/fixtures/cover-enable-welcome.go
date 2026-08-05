package fixtures

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/organization"
)

var _ = New("cover-enable-welcome", func(c *zip.Ctx) *organization.Organization {
	db := datastore.New(c.Context())

	org := organization.New(db)
	org.MustGetById("cover")
	org.Email.Subscriber.Welcome.Enabled = true
	return org
})
