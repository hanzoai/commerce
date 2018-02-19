package migrations

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/models/mailinglist"
	"hanzo.io/models/types/form"
	"hanzo.io/log"

	ds "hanzo.io/datastore"
)

var _ = New("mailinglist-types",
	func(c *context.Context) []interface{} {
		return NoArgs
	},
	func(db *ds.Datastore, ml *mailinglist.MailingList) {
		ml.Type = form.Subscribe
		if err := ml.Put(); err != nil {
			log.Error(err, db.Context)
		}
	},
)
