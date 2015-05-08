package tasks

import (
	"crowdstart.com/datastore"
	"crowdstart.com/datastore/parallel"
	"crowdstart.com/util/log"
)

// Define a new worker with parallel.Task
var TaskPlus1 = parallel.New("test-worker", func(db *datastore.Datastore, model *Model) {
	log.Debug("ADSFJKASJDFKASDJFLKASDJFLAKSDJFLASKDJFLSAKDJFALSKDJFLASKDJFLAKSDJFLAKSJDFLASKDJFALSKDFJASLDKJFALSKDFJ")
	model.Count = model.Count + 1
	if err := model.Put(); err != nil {
		panic(err)
	}
})

// Define a new worker with parallel.Task
var TaskSetVal = parallel.New("test-worker2", func(db *datastore.Datastore, model *Model2, v int) {
	log.Debug("ADSFJKASJDFKASDJFLKASDJFLAKSDJFLASKDJFLSAKDJFALSKDJFLASKDJFLAKSDJFLAKSJDFLASKDJFALSKDFJASLDKJFALSKDFJ")
	model.Count = v
	if err := model.Put(); err != nil {
		panic(err)
	}
})
