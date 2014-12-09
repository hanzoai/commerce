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

		// Done
		if err == Done {
			break
		}

		if err != nil {
			continue
		}

		// Error, ignore field mismatch
		if _, ok := err.(*ErrFieldMismatch); !ok {
			log.Error("Error fetching order: %v", err, c)
		}

		// Update user
		if o.Email == "" {
			o.Email = k.StringID()
			if _, err := db.PutKey("order", k, &o); err != nil {
				log.Error("Failed to update order: %v", err, c)
			}
		}
	}
})
