package tasks

import (
	"hanzo.io/datastore"
	"hanzo.io/datastore/parallel"
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

var TaskSetFilter = parallel.New("test-worker3", func(db *datastore.Datastore, model *user.User, v int) {
	// log.Warn("TaskSetFilter\n %v\n %v\nv %v", model.Count, model.Count2, v)
	model.Count2 = v
	if err := model.Put(); err != nil {
		panic(err)
	}
},
	parallel.Filter{
		FilterStr: "Count<",
		Value:     5,
	})
