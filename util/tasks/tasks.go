package tasks

import (
	"reflect"

	"appengine"
	"appengine/delay"

	"github.com/gin-gonic/gin"

	"crowdstart.io/middleware"
	"crowdstart.io/util/log"
)

var (
	tasks       = make(map[string][]interface{})
	contextType = reflect.TypeOf((**gin.Context)(nil)).Elem()
)

// Register a new task
func Register(name string, fns ...interface{}) {
	// Create slice for task set
	if _, ok := tasks[name]; !ok {
		tasks[name] = make([]interface{}, 0)
	}

	// Append task fns
	for _, fn := range fns {
		tasks[name] = append(tasks[name], fn)
	}
}

// Run task
func Run(ctx *gin.Context, name string, args ...interface{}) {
	// c := appengine.NewContext(c.Request)

	fns := tasks[name]
	for _, fn := range fns {
		switch v := fn.(type) {
		case *delay.Function:
			v.Call(middleware.GetAppEngine(ctx), args...)
		case func(*gin.Context, ...interface{}):
			v(ctx, args...)
		case func(*gin.Context):
			v(ctx)
		default:
			log.Error("Don't know how to call %v", reflect.ValueOf(v).Type(), ctx)
		}
	}
}

// Creates a new parallel datastore worker task, which will operate on a single
// entity of a given kind at a time (but all of them eventually, in parallel).
func Func(name string, fn interface{}) *delay.Function {
	// Check type of worker func to ensure it matches required signature.
	t := reflect.TypeOf(fn)

	// Ensure that fn is actually a func
	if t.Kind() != reflect.Func {
		log.Panic("Function is required for second parameter")
	}

	// fn should be a function that takes at least three arguments
	argNum := t.NumIn()
	if argNum < 1 {
		log.Panic("Function requires at least one argument")
	}

	// First argument fn should be gin.Context
	if t.In(0) != contextType {
		log.Panic("First argument must be *gin.Context: %v", t)
	}

	// Get reflect.Value of fn so we can call from delay.Func
	fnValue := reflect.ValueOf(fn)

	// Create actual delay func
	delayFn := delay.Func(name, func(c appengine.Context, args ...interface{}) {
		// Setup gin context
		ctx := new(gin.Context)
		ctx.Set("appengine", c)

		// Build arguments for fn
		in := []reflect.Value{reflect.ValueOf(ctx)}

		// Append variadic args
		for _, arg := range args {
			in = append(in, reflect.ValueOf(arg))
		}

		// Call fn
		fnValue.Call(in)
	})

	// Autoregister with HTTP handler
	Register(name, delayFn)

	return delayFn
}
