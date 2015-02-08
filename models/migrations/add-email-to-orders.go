package migrations

import (
	"appengine"
	. "appengine/datastore"
	"appengine/delay"

	"crowdstart.io/datastore"
	. "crowdstart.io/models"
	"crowdstart.io/util/log"
)

// Originally referenced User by  Email, now uses User ID,
var addEmailToOrders = delay.Func("migrate-add-userid-to-orders", func(c appengine.Context) {
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
		if o.UserId == "" {
			o.UserId = db.EncodeId("user", k.IntID())
			if _, err := db.PutKind("order", k, &o); err != nil {
				log.Error("Failed to update order: %v", err, c)
			}
		}
	}
})
