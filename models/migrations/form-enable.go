package migrations

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/form"

	ds "github.com/hanzoai/commerce/datastore"
)

var _ = New("form-enabled",
	func(c *zip.Ctx) []interface{} {
		return NoArgs
	},
	func(db *ds.Datastore, f *form.Form) {
		f.EmailList.Enabled = true
		if err := f.Put(); err != nil {
			log.Error(err, db.Context)
		}
	},
)
