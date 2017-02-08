package parallel

import (
	"reflect"
	"time"

	"github.com/gin-gonic/gin"

	"appengine"
	"appengine/delay"

	"hanzo.io/datastore"
	"hanzo.io/models"
	"hanzo.io/models/mixin"
	"hanzo.io/util/log"
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

	// fn should be a function that takes at least two arguments
	argNum := typ.NumIn()
	if argNum < 2 {
		log.Panic("Function requires at least two arguments")
	}

	// Check fn's first argument
	if typ.In(0) != datastoreType {
		log.Panic("First argument must be datastore.Datastore: %v", typ)
	}

	// Get entity type & kind
	entityType := typ.In(1).Elem()
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
		nsCtx := ctx
		if namespace != "" {
			var err error
			nsCtx, err = appengine.Namespace(ctx, namespace)
			if err != nil {
				panic(err)
			}
		}

		// Increase Timeout
		nsCtx = appengine.Timeout(nsCtx, 30*time.Second)

		// Run query to get results for this batch of entities
		db := datastore.New(nsCtx)

		// Construct query
		q := db.Query(fn.Kind).Offset(offset).Limit(batchSize)

		// Run query
		t := q.Run()

		// Loop over entities passing them into workerFunc one at a time
		for {
			entity := newEntity(db, fn.EntityType)
			key, err := t.Next(entity)

			// Done iterating
			if err == datastore.Done {
				break
			}

			// Skip field mismatch errors
			if err := datastore.IgnoreFieldMismatch(err); err != nil {
				log.Error("Failed to fetch next entity: %v", err, ctx)
				break
			}

			if err := entity.SetKey(key); err != nil {
				log.Error("Failed to set key: %v", err, ctx)
				break
			}

			// Build arguments for workerFunc
			numArgs := len(args)
			in := make([]reflect.Value, numArgs+2, numArgs+2)
			in[0] = reflect.ValueOf(db)
			in[1] = reflect.ValueOf(entity)

			// Append variadic args
			for i := 0; i < numArgs; i++ {
				in[i+2] = reflect.ValueOf(args[i])
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
	v, ok := c.Get("namespace")
	if ok {
		namespace, ok := v.(string)
		if ok {
			namespaces = append(namespaces, namespace)
		}
	}

	// Use all namespaces
	if len(namespaces) == 0 {
		namespaces = models.GetNamespaces(ctx)
	}

	log.Debug("Migrating namespaces: %v", namespaces)

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
	nsCtx := ctx
	if namespace != "" {
		var err error
		nsCtx, err = appengine.Namespace(ctx, namespace)
		if err != nil {
			panic(err)
		}
	}

	db := datastore.New(nsCtx)

	// Get relevant ParallelFn
	fn := parallelFns[fnName]

	total, _ := db.Query(fn.Kind).Count()

	// Start all workers
	for offset := 0; offset < total; offset += batchSize {
		// Append variadic arguments after required args
		args := append([]interface{}{namespace, offset, batchSize}, args...)

		// Call delay.Function
		fn.DelayFn.Call(ctx, args...)
	}
})
