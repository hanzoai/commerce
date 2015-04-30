package tasks

import (
	"crowdstart.io/datastore"
	"crowdstart.io/datastore/parallel"
	"crowdstart.io/models"
)

var SetEstimatedDeliveryDateToLate2015 = parallel.Task("set-estimated-delivery-date-to-late-2015", func(db *datastore.Datastore, key datastore.Key, order models.Order) {
	order.EstimatedDelivery = "Late 2015"
	db.PutKind("order", key, &order)
})
