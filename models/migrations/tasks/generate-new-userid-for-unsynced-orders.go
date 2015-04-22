package tasks

import (
	"crowdstart.io/datastore"
	"crowdstart.io/datastore/parallel"
	"crowdstart.io/models"
	"crowdstart.io/util/log"
	"crowdstart.io/util/queries"
)

//If we don't get a sync, then there is some problem with the order id key and it needs to be regenerated
var GenerateNewUserIdForUnsyncedOrders = parallel.Task("generate-new-user-id-for-unsynced-orders", func(db *datastore.Datastore, key datastore.Key, order models.Order) {
	// skip orders that were synced
	if order.PrimarySalesforceId_ != "" || order.UserId != "" {
		return
	}

	log.Debug("Regenerating UserId for order %v", order, db.Context)

	user := models.User{}
	q := queries.New(db.Context)
	if err := q.GetUserByEmail(order.Email, &user); err != nil {
		log.Warn("Error %v", err, db.Context)
	}
	order.UserId = user.Id
	db.PutKind("order", key, &order)
})
