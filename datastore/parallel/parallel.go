package parallel

import (
	"reflect"

	"github.com/gin-gonic/gin"

	"appengine"
	"appengine/delay"

	"crowdstart.io/datastore"
	"crowdstart.io/models"
	"crowdstart.io/models/mixin"
	"crowdstart.io/util/log"
)

type ParallelFn struct {
	Kind       string
	Name       string
	EntityType reflect.Type
	Value      reflect.Value
	DelayFn    *delay.Function
}

var parallelFns = make(map[string]*ParallelFn)

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

	parallelFns[p.Name] = p

	return p
}

// Creates a new parallel datastore worker task, which will operate on a single
// entity of a given kind at a time (but all of them eventually, in parallel).
func (fn *ParallelFn) createDelayFn(name string) {
	fn.DelayFn = delay.Func("parallel-fn-"+name, func(ctx appengine.Context, namespace string, offset int, batchSize int, args ...interface{}) {
		// Explicitly switch namespace. TODO: this should not be necessary, bug?
		ctx, _ = appengine.Namespace(ctx, namespace)

		// Run query to get results for this batch of entities
		db := datastore.New(ctx)

		// Construct query
		q := db.Query2(fn.Kind).Offset(offset).Limit(batchSize)

		total, _ := db.Query2("product").Count()
		log.Debug("total: %v", total)

		// Run query
		t := q.Run()

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
					log.Error("datastore.parallel worker encountered error: %v", err, ctx)
					continue
				}

				// Ignore field mismatch
				log.Warn("Field mismatch when getting %v: %v", key, err, ctx)
				err = nil
			}

			err = entity.SetKey(key)
			if err != nil {
				log.Error("Failed to set key: %v", err, ctx)
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

// Run fn in parallel across all entities
func (fn *ParallelFn) Run(c *gin.Context, batchSize int, args ...interface{}) error {
	// Limit results in test mode
	if c.MustGet("test").(bool) {
		batchSize = 1
	}

	ctx := c.MustGet("appengine").(appengine.Context)

	namespaces := make([]string, 0)

	// Check if namespace is set explicitly
	v, err := c.Get("namespace")
	if err == nil {
		namespace, ok := v.(string)
		if ok {
			namespaces = append(namespaces, namespace)
		}
	}

	// Use all namespaces
	if len(namespaces) == 0 {
		namespaces = models.GetNamespaces(ctx)
	}

	// Iterate through namespaces and initialize workers to run in each
	for _, ns := range namespaces {
		args := append([]interface{}{fn.Name, ns, batchSize}, args...)
		initNamespace.Call(ctx, args...)
	}

	return nil
}

// Start individual runs in a given namespace
var initNamespace = delay.Func("parallel-init", func(ctx appengine.Context, fnName string, namespace string, batchSize int, args ...interface{}) {
	// Set namespace explicitly
	ctx, _ = appengine.Namespace(ctx, namespace)
	db := datastore.New(ctx)

	// Get relevant ParallelFn
	fn := parallelFns[fnName]

	total, _ := db.Query2(fn.Kind).Count()
	log.Debug("total: %v", total)

	// Start all workers
	for offset := 0; offset < total; offset += batchSize {
		// Append variadic arguments after required args
		args := append([]interface{}{namespace, offset, batchSize}, args...)

		// Check context here
		log.Debug("context to delay fn: %v", ctx)

		// Call delay.Function
		fn.DelayFn.Call(ctx, args...)
	}
})
