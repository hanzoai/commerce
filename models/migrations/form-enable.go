package migrations

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/log"
	"hanzo.io/models/form"

	ds "hanzo.io/datastore"
)

var _ = New("form-enabled",
	func(c *gin.Context) []interface{} {
		return NoArgs
	},
	func(db *ds.Datastore, f *form.Form) {
		f.EmailList.Enabled = true
		if err := f.Put(); err != nil {
			log.Error(err, db.Context)
		}
	},
)
