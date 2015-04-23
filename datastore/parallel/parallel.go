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

type ParallelFn struct {
	Kind       string
	Name       string
	EntityType reflect.Type
	Value      reflect.Value
	DelayFn    *delay.Function
}

func New(name string, fn interface{}) *ParallelFn {
	// Check type of worker func to ensure it matches required signature.
	typ := reflect.TypeOf(fn)

	// Ensure that fn is actually a func
	if typ.Kind() != reflect.Func {
		log.Panic("Function is required for second parameter")
	}

	// fn should be a function that takes at least three arguments
	argNum := typ.NumIn()
	if argNum < 3 {
		log.Panic("Function requires at least three arguments")
	}

	// Check fn's first argument
	if typ.In(0) != datastoreType {
		log.Panic("First argument must be datastore.Datastore: %v", typ)
	}

	// Check fn's second argument
	if typ.In(1) != keyType {
		log.Panic("Second argument must be datastore.Key")
	}

	// Get entity type & kind
	entityType := typ.In(2).Elem()
	entity := reflect.New(entityType).Interface().(mixin.Kind)
	kind := entity.Kind()

	// Create a new ParallelFn
	p := &ParallelFn{
		Name:       name,
		Kind:       kind,
		EntityType: entityType,
		Value:      reflect.ValueOf(fn),
	}

	// Create delay function
	p.createDelayFn(p.Name)

	return p
}

// Creates a new parallel datastore worker task, which will operate on a single
// entity of a given kind at a time (but all of them eventually, in parallel).
func (fn *ParallelFn) createDelayFn(name string) {
	fn.DelayFn = delay.Func("parallel-fn-"+name, func(c appengine.Context, fc *fakecontext.Context, cursor string, offset int, limit int, args ...interface{}) {
		// Run query to get results for this batch of entities
		db := datastore.New(c)

		// Construct query
		q := db.Query(fn.Kind).Offset(offset).Limit(limit)

		// Run query
		t := q.Run(c)

		// Loop over entities passing them into workerFunc one at a time
		for {
			entity := newEntity(db, fn.EntityType)
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

			err = entity.SetKey(key)
			if err != nil {
				log.Error("Failed to set key: %v", err, c)
				continue
			}

			// Build arguments for workerFunc
			in := []reflect.Value{reflect.ValueOf(db), reflect.ValueOf(key), reflect.ValueOf(entity)}

			// Append variadic args
			for _, arg := range args {
				in = append(in, reflect.ValueOf(arg))
			}

			// Run our worker func with this entity
			fn.Value.Call(in)
		}
	})
}

// Call underlying delay function
func (fn *ParallelFn) Call(ctx appengine.Context, args ...interface{}) {
	fn.DelayFn.Call(ctx, args...)
}

// Return a wrapped query which we can run our workers across
func (fn *ParallelFn) Query(c *gin.Context) *Query {
	return NewQuery(c, fn)
}

// Run fn in parallel across all entities
func (fn *ParallelFn) Run(c *gin.Context, batchSize int, args ...interface{}) error {
	return fn.Query(c).RunAll(batchSize, args...)
}
