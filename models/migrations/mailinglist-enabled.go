package migrations

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/models/mailinglist"
	"hanzo.io/util/log"

	ds "hanzo.io/datastore"
)

var _ = New("mailinglist-enabled",
	func(c *context.Context) []interface{} {
		return NoArgs
	},
	func(db *ds.Datastore, ml *mailinglist.MailingList) {
		ml.Mailchimp.Enabled = true
		if err := ml.Put(); err != nil {
			log.Error(err, db.Context)
		}
	},
)
