package worker

import (
	"crowdstart.io/datastore"
	"crowdstart.io/datastore/parallel"
)

type Model struct {
	Count int
}

// Define a new worker with parallel.Task
var Task = parallel.Task("test-worker", func(db *datastore.Datastore, k datastore.Key, model Model) {
	model.Count = model.Count + 1
	db.PutKey("test-model", k, model)
})
