package migrations

import (
	"appengine"
	"appengine/delay"

	. "appengine/datastore"
	"crowdstart.io/datastore"
	"crowdstart.io/util/log"

	. "crowdstart.io/models"
)

var AddEmailToOrders = delay.Func("add-email-to-orders-migration", func(c appengine.Context) {
	log.Debug("Migrating orders")
	db := datastore.New(c)
	q := db.Query("order")
	t := q.Run(c)
	for {
		var o Order
		k, err := t.Next(&o)
		if err == Done {
			break // No further entities match the query.
		}

		if err != nil {
			log.Error("Error fetching order")
		}

		log.Debug("key: %v", k.StringID())
		o.Email = k.StringID()
		if _, err := db.PutKey("order", k, &o); err != nil {
			log.Debug("Error savin order: %v")
		}
	}
})
