package parallel

import (
	"appengine"
	"appengine/datastore"
	"appengine/delay"

	"crowdstart.io/util/log"
)

// Continuation stores context information for execution
type Continuation interface {
	// NewObject should return pointer to an instance of the object
	NewObject() interface{}
	Execute(appengine.Context, *datastore.Key, interface{}) error
}

var datastoreWorker = delay.Func("ParallelDatastoreWorker", func(c appengine.Context, kind string, offset, limit int, cont Continuation) {
	var k *datastore.Key
	var err error
	t := datastore.NewQuery(kind).Offset(offset).Limit(limit).Run(c)

	for {
		object := cont.NewObject()
		if k, err = t.Next(object); err != nil {
			// Done
			if err == datastore.Done {
				break
			}

			log.Error("Datastore worker encountered error: %v", err, c)
			continue
		}

		if err = cont.Execute(c, k, object); err != nil {
			log.Error("Function encountered error: %v", err, c)
		}
	}
})

// NewDatastoreJob initializes Ceiling[Count(kind)/limit] workers.
func DatastoreJob(c appengine.Context, kind string, limit int, cont Continuation) error {
	var total int
	var err error

	if total, err = datastore.NewQuery(kind).Count(c); err != nil {
		log.Error("Could not get count of %v because %v", kind, err, c)
		return err
	}

	for offset := 0; offset < total; offset += limit {
		datastoreWorker.Call(c, kind, offset, limit, cont)
	}

	return nil
}
