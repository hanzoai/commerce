package migrations

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/models/mailinglist"
	"crowdstart.com/util/log"

	ds "crowdstart.com/datastore"
)

var _ = New("mailinglist-enabled",
	func(c *gin.Context) []interface{} {
		return NoArgs
	},
	func(db *ds.Datastore, ml *mailinglist.MailingList) {
		ml.Mailchimp.Enabled = true
		if err := ml.Put(); err != nil {
			log.Error(err, db.Context)
		}
	},
)
