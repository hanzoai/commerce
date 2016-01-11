package migrations

import (
	"time"

	"github.com/gin-gonic/gin"

	"crowdstart.com/datastore"
	"crowdstart.com/models/order"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/user"
	"crowdstart.com/util/emails"

	ds "crowdstart.com/datastore"
)

var _ = New("send-confirmations-for-jan",
	func(c *gin.Context) []interface{} {
		c.Set("namespace", "kanoa")

		db := datastore.New(c)
		org := organization.New(db)
		org.GetById("kanoa")

		return []interface{}{org.Mandrill.APIKey}
	},
	func(db *ds.Datastore, ord *order.Order, apiKey string) {
		// Fix issue with improperly set up orders
		sendMail := false
		if ord.CreatedAt.IsZero() {
			ord.MustCreate()
			sendMail = true
		}

		t1, err := time.Parse(time.RFC3339, "2016-01-06T13:30:00-06:00")
		if err != nil {
			panic(err)
		}
		t2, err := time.Parse(time.RFC3339, "2016-01-09T14:00:00-06:00")
		if err != nil {
			panic(err)
		}

		if ord.CreatedAt.After(t1) && ord.CreatedAt.Before(t2) {
			sendMail = true
		}

		if !sendMail {
			return
		}

		usr := user.New(ord.Db)
		usr.GetById(ord.UserId)

		org := organization.New(ord.Db)
		org.Mandrill.APIKey = apiKey

		emails.SendOrderConfirmationEmail(ord.Db.Context, org, ord, usr)
	},
)
