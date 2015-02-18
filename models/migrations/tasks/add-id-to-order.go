package tasks

import (
	"crowdstart.io/datastore"
	"crowdstart.io/datastore/parallel"
	"crowdstart.io/models"
)

var AddIdToOrder = parallel.Task("add-id-to-order", func(db *datastore.Datastore, key datastore.Key, order models.Order) {
	order.Id = key.Encode()
	db.Put(key, &order)
})
