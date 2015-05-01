package tasks

import (
	"time"

	"crowdstart.io/datastore"
	"crowdstart.io/datastore/parallel"
	"crowdstart.io/models"
)

var FixEstimatedDelivery420 = parallel.Task("fix-estimated-delivery-420", func(db *datastore.Datastore, key datastore.Key, order models.Order) {
	LastMonth := time.Now().Add(-30 * 24 * time.Hour)
	if order.CreatedAt.Before(LastMonth) {
		return
	}

	order.EstimatedDelivery = "December 2015"
	db.PutKind("order", key, &order)
})
