package tasks

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/datastore/parallel"
)

// Define a new worker with parallel.Task
var TaskPlus1 = parallel.New("test-worker", func(db *datastore.Datastore, model *Model) {
	model.Count = model.Count + 1
	model.MustPut()
})

// Define a new worker with parallel.Task
var TaskSetVal = parallel.New("test-worker2", func(db *datastore.Datastore, model *Model2, v int) {
	model.Count = v
	model.MustPut()
})
