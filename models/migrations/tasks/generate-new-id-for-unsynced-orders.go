package tasks

import (
	"time"

	"crowdstart.io/datastore"
	"crowdstart.io/datastore/parallel"
	"crowdstart.io/models"
	"crowdstart.io/util/log"
)

//If we don't get a sync, then there is some problem with the order id key and it needs to be regenerated
var GenerateNewIdForUnsyncedOrders = parallel.Task("generate-new-id-for-unsynced-orders", func(db *datastore.Datastore, key datastore.Key, order models.Order) {
	// skip orders that were synced
	if order.SalesforceId() != "" {
		return
	}

	orders := make([]models.Order, 0)
	var err error

	// Get Contributions
	if _, err = db.Query("order").Filter("Id =", order.Id).GetAll(db.Context, &orders); err != nil {
		log.Error("Task has encountered error: %v", err, db.Context)
		return
	}

	if len(orders) <= 0 {
		log.Error("This should never happen! %v", order, db.Context)
		return
	}

	if len(orders) <= 1 {
		return
	}

	log.Debug("Regenerating Order Id for %v", order, db.Context)

	order.Id = key.Encode()
	order.UpdatedAt = time.Now()
	db.PutKind("order", key, &order)
})
