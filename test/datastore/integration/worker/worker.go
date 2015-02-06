package worker

import (
	"crowdstart.io/datastore"
	"crowdstart.io/datastore/parallel"
	"crowdstart.io/util/log"
)

type Model struct {
	Count int
}

// Define a new worker with parallel.Task
var TaskPlus1 = parallel.Task("test-worker", func(db *datastore.Datastore, k datastore.Key, model Model) {
	model.Count = model.Count + 1
	log.Warn("Working On Object %v, %v", k, model)
	db.PutKey("test-model", k, &model)
	log.Warn("Inserted")
})

// Define a new worker with parallel.Task
var TaskSetVal = parallel.Task("test-worker2", func(db *datastore.Datastore, k datastore.Key, model Model, v int) {
	model.Count = v
	db.PutKey("test-model", k, &model)
})
