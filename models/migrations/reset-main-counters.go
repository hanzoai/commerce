package migrations

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/models/order"
	"hanzo.io/models/return"
	"hanzo.io/models/subscriber"
	"hanzo.io/models/user"
	"hanzo.io/util/counter"
	"hanzo.io/util/log"

	ds "hanzo.io/datastore"
)

var _ = New("reset-main-counters",
	func(c *gin.Context) []interface{} {
		c.Set("namespace", "bellabeat")
		return NoArgs
	},
	func(db *ds.Datastore, usr *user.User) {
		if err := counter.IncrUser(db.Context, usr.CreatedAt); err != nil {
			log.Error("IncrUser Error %v", err, db.Context)
		}
	},
	func(db *ds.Datastore, usr *subscriber.Subscriber) {
		if err := counter.IncrSubscriber(db.Context, usr.CreatedAt); err != nil {
			log.Error("IncrSubscriber Error %v", err, db.Context)
		}
	},
	func(db *ds.Datastore, rtn *return_.Return) {
		items := rtn.Items
		if len(items) == 0 {
			ord := order.New(db)
			if err := ord.GetById(rtn.OrderId); err != nil {
				log.Error("Could not get order %v", err, db.Context)
				return
			}
			items = ord.Items
		}
		if err := counter.IncrOrderReturn(db.Context, items, rtn); err != nil {
			log.Error("IncrOrderReturn Error %v", err, db.Context)
		}
	},
	func(db *ds.Datastore, ord *order.Order) {
		if err := counter.IncrOrder(db.Context, ord); err != nil {
			log.Error("IncrOrder Error %v", err, db.Context)
		}
		if err := counter.IncrOrderRefund(db.Context, ord, int(ord.Refunded), ord.UpdatedAt); err != nil {
			log.Error("IncrOrderRefund Error %v", err, db.Context)
		}
	},
)
