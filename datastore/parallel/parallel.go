package parallel

import (
	"reflect"

	"appengine"
	"appengine/delay"

	aeds "appengine/datastore"

	"crowdstart.io/datastore"
	"crowdstart.io/util/log"
)

// Precompute a few common types
var (
	datastoreType = reflect.TypeOf((**datastore.Datastore)(nil)).Elem()
	keyType       = reflect.TypeOf((*datastore.Key)(nil)).Elem()
)

// Creates a new parallel datastore worker task, which will operate on a single
// entity of a given kind at a time (but all of them eventually, in parallel).
func Task(name string, workerFunc interface{}) *delay.Function {
	// Check type of worker func to ensure it matches required signature.
	t := reflect.TypeOf(workerFunc)

	// Ensure that workerFunc is actually a func
	if t.Kind() != reflect.Func {
		log.Panic("Function is required for second parameter")
	}

	// workerFunc should be a function that takes at least three arguments
	argNum := t.NumIn()
	if argNum < 3 {
		log.Panic("Function requires at least three arguments")
	}

	// check workerFunc's first argument
	if t.In(0) != datastoreType {
		log.Panic("First argument must be datastore.Datastore: %v", t)
	}

	// check workerFunc's second argument
	if t.In(1) != keyType {
		log.Panic("Second argument must be datastore.Key")
	}

	entityType := t.In(2)
	workerFuncValue := reflect.ValueOf(workerFunc)

	return delay.Func(name, func(c appengine.Context, kind string, offset, limit int, args ...interface{}) {
		var k *aeds.Key
		var err error

		// Run query to get results for this batch of entities
		db := datastore.New(c)
		t := db.Query(kind).Offset(offset).Limit(limit).Run(c)

		// Loop over entities passing them into workerFunc one at a time
		for {
			entityPtr := reflect.New(entityType).Interface()
			if _, err = t.Next(entityPtr); err != nil {
				// Done iterating
				if err == datastore.Done {
					break
				}

				log.Error("datastore.parallel worker encountered error: %v", err, c)
				continue
			}

			// Build arguments for workerFunc
			in := []reflect.Value{reflect.ValueOf(db), reflect.ValueOf(k), reflect.Indirect(reflect.ValueOf(entityPtr)), reflect.ValueOf(args)}

			// Run our worker func with this entity
			workerFuncValue.Call(in)
		}
	})
}

// Executes parallel task
func Run(c appengine.Context, kind string, batchSize int, fn *delay.Function, args ...interface{}) error {
	var total int
	var err error

	if total, err = aeds.NewQuery(kind).Count(c); err != nil {
		log.Error("Could not get count of %v because %v", kind, err, c)
		return err
	}

	for offset := 0; offset < total; offset += batchSize {
		// prepend variadic arguments for `delay.Function.Call` with `kind`, `offset`, `batchSize`.
		args := append([]interface{}{kind, offset, batchSize}, args)

		fn.Call(c, args...)
	}

	return nil
}
