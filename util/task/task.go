package task

import (
	"reflect"

	"appengine"
	"appengine/delay"

	"github.com/gin-gonic/gin"

	"crowdstart.io/middleware"
	"crowdstart.io/util/fakecontext"
	"crowdstart.io/util/log"
)

var (
	registry    = make(map[string][]interface{})
	contextType = reflect.TypeOf((**gin.Context)(nil)).Elem()
)

type Task struct {
	ExpectsQuery bool
	Fn           interface{}
	Value        reflect.Value
	DelayFn      *delay.Function
}

func NewTask(fn interface{}) *Task {
	task := new(Task)

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
	task.Value = reflect.ValueOf(fn)
	return task
}

// Register a new task
func Register(name string, tasks ...interface{}) {
	// Create slice for task set
	_tasks, ok := registry[name]

	if !ok {
		_tasks = make([]interface{}, 0)
	}

	// Append tasks
	_tasks = append(_tasks, tasks...)

	registry[name] = _tasks
}

// Run task
func Run(ctx *gin.Context, name string, args ...interface{}) {
	tasks, ok := registry[name]

	if !ok {
		log.Panic("Unknown task: %v", name, ctx)
	}

	for _, task := range tasks {
		switch v := task.(type) {
		case *delay.Function:
			v.Call(middleware.GetAppEngine(ctx), args...)
		case func(*gin.Context, ...interface{}):
			v(ctx, args...)
		case func(*gin.Context):
			v(ctx)
		case func(appengine.Context):
			v(middleware.GetAppEngine(ctx)) // TODO: Remove after updating older tasks.
		case *Task:
			v.DelayFn.Call(middleware.GetAppEngine(ctx), fakecontext.NewContext(ctx))
		default:
			log.Panic("Don't know how to call %v", reflect.ValueOf(v).Type(), ctx)
		}
	}
}

// Creates a new parallel datastore worker task, which will operate on a single
// entity of a given kind at a time (but all of them eventually, in parallel).
func Func(name string, fn interface{}) *delay.Function {
	task := NewTask(fn)

	// Create actual delay func
	delayFn := delay.Func(name, func(c appengine.Context, args ...interface{}) {
		ctx := new(gin.Context)

		// If passed a context, use that
		if len(args) > 0 {
			if _ctx, ok := args[0].(*fakecontext.Context); ok {
				ctx, _ = _ctx.Context()
			}

			// Remove context from args
			args = args[:len(args)-1]
		}

		// Ensure App Engine context is set on gin Context
		ctx.Set("appengine", c)

		// Build arguments for fn
		in := []reflect.Value{reflect.ValueOf(ctx)}

		// Append variadic args
		for _, arg := range args {
			in = append(in, reflect.ValueOf(arg))
		}

		// Call fn
		task.Value.Call(in)
	})

	// Save reference to delay function for easy access from Task
	task.DelayFn = delayFn

	// Auto-register with HTTP handler
	Register(name, task)

	return delayFn
}
