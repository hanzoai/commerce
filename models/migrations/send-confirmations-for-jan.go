package migrations

import (
	"encoding/gob"
	"time"

	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/models/order"
	"hanzo.io/models/organization"
	"hanzo.io/models/user"
	"hanzo.io/util/emails"
	"hanzo.io/util/log"

	ds "hanzo.io/datastore"
)

func init() {
	gob.Register(organization.Email{})
}

var _ = New("send-confirmations-for-jan",
	func(c *gin.Context) []interface{} {
		c.Set("namespace", "kanoa")

		db := datastore.New(c)
		org := organization.New(db)
		org.GetById("kanoa")

		return []interface{}{org.Mandrill.APIKey, org.Email.Defaults.Enabled, org.Email.Defaults.FromName, org.Email.Defaults.FromEmail, org.Email.OrderConfirmation}
	},
	func(db *ds.Datastore, ord *order.Order, apiKey string, defaultEnabled bool, defaultFromName, defaultFromEmail string, orderConfirmation organization.Email) {
		// Fix issue with improperly set up orders
		sendMail := false
		if ord.CreatedAt.IsZero() {
			ord.MustCreate()
			sendMail = true
			log.Warn("Fixing Uninitialized Order %v", ord.Id(), ord.Db.Context)
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
			log.Warn("NOT SENDING Order %v", ord.Id(), ord.Db.Context)
			return
		}

		log.Warn("SENDING Order %v", ord.Id(), ord.Db.Context)

		usr := user.New(ord.Db)
		usr.GetById(ord.UserId)

		org := organization.New(ord.Db)
		org.Email.Defaults.Enabled = defaultEnabled
		org.Email.Defaults.FromName = defaultFromName
		org.Email.Defaults.FromEmail = defaultFromEmail
		org.Email.OrderConfirmation = orderConfirmation
		org.Mandrill.APIKey = apiKey

		// log.Warn("API email config %v", org.Email, ord.Db.Context)
		// log.Warn("API Key %v", org.Mandrill.APIKey, ord.Db.Context)

		emails.SendOrderConfirmationEmail(ord.Db.Context, org, ord, usr)
	},
)
