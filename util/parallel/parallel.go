package parallel

import (
	"reflect"

	"appengine"
	"appengine/datastore"
	"appengine/delay"

	"crowdstart.io/util/log"
)

var (
	// precomputed types
	contextType = reflect.TypeOf((*appengine.Context)(nil)).Elem()
	keyType     = reflect.TypeOf((**datastore.Key)(nil)).Elem()
)

// Continuation stores context information for execution
type SerializableClosure interface {
	// NewObject should return pointer to an instance of the object
	NewObject() interface{}
	Execute(appengine.Context, *datastore.Key, interface{}) error
}

var datastoreWorker = delay.Func("ParallelDatastoreWorker", func(c appengine.Context, kind string, offset, limit int, cont SerializableClosure) {
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
func DatastoreJob(c appengine.Context, kind string, limit int, cont SerializableClosure) error {
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

func DatastoreCall(c appengine.Context, kind string, limit int, fn *delay.Function, metadata interface{}) error {
	var total int
	var err error

	if total, err = datastore.NewQuery(kind).Count(c); err != nil {
		log.Error("Could not get count of %v because %v", kind, err, c)
		return err
	}

	for offset := 0; offset < total; offset += limit {
		fn.Call(c, kind, offset, limit, metadata)
	}

	return nil
}

func DatastoreFunc(name string, w interface{}) *delay.Function {
	t := reflect.TypeOf(w)
	if t.Kind() != reflect.Func {
		log.Panic("Function is required for third parameter")
	}

	argNum := t.NumIn()
	if argNum < 3 {
		log.Panic("Function requires atleast 3 parameters")
	}

	if t.In(0) != contextType {
		log.Panic("First argument must be an appengine.Context")
	}

	if t.In(1) != keyType {
		log.Panic("First argument must be a *datastore.Key")
	}

	objectType := t.In(2)
	v := reflect.ValueOf(w)

	return delay.Func(name, func(c appengine.Context, kind string, offset, limit int, metadata interface{}) {
		var k *datastore.Key
		var err error
		t := datastore.NewQuery(kind).Offset(offset).Limit(limit).Run(c)

		for {
			objectPtr := reflect.New(objectType).Interface()
			if _, err = t.Next(objectPtr); err != nil {
				// Done
				if err == datastore.Done {
					break
				}

				log.Error("Datastore worker encountered error: %v", err, c)
				continue
			}

			in := []reflect.Value{reflect.ValueOf(c), reflect.ValueOf(k), reflect.Indirect(reflect.ValueOf(objectPtr)), reflect.ValueOf(metadata)}
			v.Call(in)
		}
	})
}
