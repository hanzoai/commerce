package fixtures

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/form"
)

var _ = New("kanoa-forms", func(c *zip.Ctx) *form.Form {
	db := datastore.New(c.Context())

	f := form.New(db)
	f.MustGetById("3XudPY2SQeXQ3")
	f.Forward.Name = "Cival"
	f.Forward.Email = "dev@hanzo.ai"
	f.Forward.Enabled = true

	return f
})
