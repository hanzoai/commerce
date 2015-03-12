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
	if order.PrimarySalesforceId_ != "" {
		return
	}

	log.Debug("Regenerating Id for order %v", order, db.Context)

	order.Id = key.Encode()
	order.UpdatedAt = time.Now()
	db.PutKind("order", key, &order)
})
