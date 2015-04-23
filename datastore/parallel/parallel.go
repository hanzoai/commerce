package parallel

import (
	"reflect"

	"github.com/gin-gonic/gin"

	"appengine"
	"appengine/delay"

	"crowdstart.io/datastore"
	"crowdstart.io/models/mixin"
	"crowdstart.io/util/fakecontext"
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

	entityType := t.In(2).Elem()
	workerFuncValue := reflect.ValueOf(workerFunc)

	return delay.Func(name, func(c appengine.Context, fc *fakecontext.Context, kind string, offset, batchSize int, args ...interface{}) {
		// Run query to get results for this batch of entities
		db := datastore.New(c)
		q := db.Query(kind).Offset(offset).Limit(batchSize)

		// Limit 1 if in test mode
		if gc, err := fc.Context(&c); err == nil && gc.MustGet("test").(bool) {
			q = q.Limit(1)
		}

		t := q.Run(c)

		// Loop over entities passing them into workerFunc one at a time
		for {
			entity := reflect.New(entityType).Interface().(mixin.Entity)
			model := mixin.Model{Db: db, Entity: entity}

			// Set model on entity
			field := reflect.Indirect(reflect.ValueOf(entity)).FieldByName("Model")
			field.Set(reflect.ValueOf(model))

			key, err := t.Next(entity)

			if err != nil {
				// Done iterating
				if err == datastore.Done {
					break
				}

				// Check if genuine error occurred
				if db.SkipFieldMismatch(err) != nil {
					log.Error("datastore.parallel worker encountered error: %v", err, c)
					continue
				}

				// Ignore field mismatch
				log.Warn("Field mismatch when getting %v: %v", key, err, c)
				err = nil
			}

			// Build arguments for workerFunc
			in := []reflect.Value{reflect.ValueOf(db), reflect.ValueOf(key), reflect.ValueOf(entity)}

			// Append variadic args
			for _, arg := range args {
				in = append(in, reflect.ValueOf(arg))
			}

			// Run our worker func with this entity
			workerFuncValue.Call(in)
		}
	})
}

// Executes parallel task
func Run(c *gin.Context, kind string, batchSize int, fn *delay.Function, args ...interface{}) error {
	var total int
	var err error

	db := datastore.New(c)

	if total, err = db.Query(kind).Count(db.Context); err != nil {
		log.Error("Could not get count of %v because %v", kind, err, c)
		return err
	}

	// Launch only 1 worker if in test mode
	if c.MustGet("test").(bool) {
		total = 1
	}

	for offset := 0; offset < total; offset += batchSize {
		// prepend variadic arguments for `delay.Function.Call` with `kind`, `offset`, `batchSize`.
		args := append([]interface{}{fakecontext.NewContext(c), kind, offset, batchSize}, args...)

		fn.Call(db.Context, args...)
	}
	return nil
}
