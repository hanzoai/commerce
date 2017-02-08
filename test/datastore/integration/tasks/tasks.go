package tasks

import (
	"hanzo.io/datastore"
	"hanzo.io/datastore/parallel"
	"hanzo.io/test/fixtures/user"
	// "hanzo.io/util/log"
)

// Define a new worker with parallel.Task
var TaskPlus1 = parallel.New("test-worker", func(db *datastore.Datastore, model *user.User) {
	// log.Warn("TaskPlus1", model)
	model.Count = model.Count + 1
	if err := model.Put(); err != nil {
		panic(err)
	}
})

// Define a new worker with parallel.Task
var TaskSetVal = parallel.New("test-worker2", func(db *datastore.Datastore, model *user.User, v int) {
	// log.Warn("TaskSetVal %v\nv %v", model, v)
	model.Count = v
	if err := model.Put(); err != nil {
		panic(err)
	}
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
