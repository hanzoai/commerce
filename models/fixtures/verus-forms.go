package fixtures

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/form"
)

var _ = New("verus-forms", func(c *zip.Ctx) *form.Form {
	db := datastore.New(c.Context())

	f := form.New(db)
	f.MustGetById("NEu14x75uv0Z6B")
	f.Forward.Name = "Sales"
	f.Forward.Email = "dev@hanzo.ai"
	f.Forward.Enabled = true

	return f
})
