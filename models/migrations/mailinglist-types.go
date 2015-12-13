package migrations

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/models/mailinglist"
	"crowdstart.com/models/types/form"
	"crowdstart.com/util/log"

	ds "crowdstart.com/datastore"
)

var _ = New("mailinglist-types",
	func(c *gin.Context) []interface{} {
		return NoArgs
	},
	func(db *ds.Datastore, ml *mailinglist.MailingList) {
		ml.Type = form.Signup
		if err := ml.Put(); err != nil {
			log.Error(err, db.Context)
		}
	},
)
