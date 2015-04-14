package tasks

import (
	"time"

	"crowdstart.io/datastore"
	"crowdstart.io/datastore/parallel"
	"crowdstart.io/models"
	"crowdstart.io/thirdparty/salesforce"
	"crowdstart.io/util/log"
)

//If we don't get a sync, then there is some problem with the order id key and it needs to be regenerated
var FixPreorderCorruption = parallel.Task("fix-preorder-corruption", func(db *datastore.Datastore, key datastore.Key, order models.Order, campaign models.Campaign) {
	// skip orders that were synced
	if order.PrimarySalesforceId_ != "" {
		return
	}

	log.Debug("Regenerating Id for order %v", order, db.Context)

	client := salesforce.New(db.Context, &campaign, true)
	if err := client.Push(&order); err != nil {
		log.Warn("Warn: %v, '%v'", err, order.Id, db.Context)
	}

	sOrder := salesforce.Order{}
	sOrder.Account = nil
	sOrder.Read(&order)
	sOrder.Delete(client)

	order.Id = key.Encode()
	order.UpdatedAt = time.Now()

	client.Push(&order)

	db.PutKind("order", key, &order)
})
